package biz

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/anthropic/claudecode"
	"github.com/looplj/axonhub/llm/transformer/antigravity"
	"github.com/looplj/axonhub/llm/transformer/bailian"
	"github.com/looplj/axonhub/llm/transformer/deepseek"
	"github.com/looplj/axonhub/llm/transformer/doubao"
	"github.com/looplj/axonhub/llm/transformer/fireworks"
	"github.com/looplj/axonhub/llm/transformer/gemini"
	geminioai "github.com/looplj/axonhub/llm/transformer/gemini/openai"
	"github.com/looplj/axonhub/llm/transformer/jina"
	"github.com/looplj/axonhub/llm/transformer/longcat"
	"github.com/looplj/axonhub/llm/transformer/modelscope"
	"github.com/looplj/axonhub/llm/transformer/moonshot"
	"github.com/looplj/axonhub/llm/transformer/nanogpt"
	"github.com/looplj/axonhub/llm/transformer/ollama"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
	"github.com/looplj/axonhub/llm/transformer/openai/copilot"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
	"github.com/looplj/axonhub/llm/transformer/openrouter"
	"github.com/looplj/axonhub/llm/transformer/xai"
	"github.com/looplj/axonhub/llm/transformer/zai"
)

type AutoRefresher interface {
	StartAutoRefresh(ctx context.Context, opts oauth.AutoRefreshOptions)
	StopAutoRefresh()
}

func setupAutoRefresh(ch *Channel, refresher AutoRefresher, opts oauth.AutoRefreshOptions) {
	ch.startTokenProvider = func() {
		refresher.StartAutoRefresh(context.Background(), opts)
	}
	ch.stopTokenProvider = refresher.StopAutoRefresh
}

func (c *Channel) IsModelSupported(model string) bool {
	entries := c.GetModelEntries()
	_, ok := entries[model]

	return ok
}

// CustomizeExecutor implements pipeline.ChannelCustomizedExecutor interface
// This allows the channel to provide a custom HTTP client with proxy support.
func (c *Channel) CustomizeExecutor(executor pipeline.Executor) pipeline.Executor {
	if c.HTTPClient != nil {
		// Return the HTTP client as the executor for this channel
		return c.HTTPClient
	}
	// Fall back to the default executor if no custom HTTP client is configured
	return executor
}

func (c *Channel) ChooseModel(model string) (string, error) {
	entries := c.GetModelEntries()

	entry, ok := entries[model]
	if !ok {
		return "", fmt.Errorf("model %s not supported in channel %s", model, c.Name)
	}

	return entry.ActualModel, nil
}

// getProxyConfig extracts proxy configuration from channel settings
// Returns nil if no proxy configuration is set (backward compatibility).
func getProxyConfig(channelSettings *objects.ChannelSettings) *httpclient.ProxyConfig {
	if channelSettings == nil || channelSettings.Proxy == nil {
		// Backward compatibility: default to environment proxy type
		return &httpclient.ProxyConfig{
			Type: httpclient.ProxyTypeEnvironment,
		}
	}

	return channelSettings.Proxy
}

// getHttpClient returns the injected default HTTP client when no custom proxy is configured,
// or creates a new one with proxy support (inheriting TLS settings from the default client).
func (svc *ChannelService) getHttpClient(channelSettings *objects.ChannelSettings) *httpclient.HttpClient {
	if channelSettings == nil || channelSettings.Proxy == nil {
		return svc.httpClient
	}

	return svc.httpClient.WithProxy(channelSettings.Proxy)
}

// buildChannel creates a Channel with precomputed caches (transformer is set separately).
func buildChannel(c *ent.Channel, httpClient *httpclient.HttpClient) *Channel {
	// Precompute disabled key set for O(1) lookup
	disabledKeySet := make(map[string]struct{}, len(c.DisabledAPIKeys))
	for _, dk := range c.DisabledAPIKeys {
		if dk.Key != "" {
			disabledKeySet[dk.Key] = struct{}{}
		}
	}

	ch := &Channel{
		Channel:              c,
		HTTPClient:           httpClient,
		cachedDisabledKeySet: disabledKeySet,
		cachedEnabledAPIKeys: c.Credentials.GetEnabledAPIKeys(c.DisabledAPIKeys),
	}

	// Precompute other caches
	entries := ch.GetModelEntries()
	headers := ch.GetHeaderOverrideOperations()
	params := ch.GetBodyOverrideOperations()

	if log.DebugEnabled(context.Background()) {
		log.Debug(context.Background(), "pre cached settings",
			log.String("channel", ch.Name),
			log.Int("entries", len(entries)),
			log.Int("headers", len(headers)),
			log.Int("params", len(params)),
		)
	}

	return ch
}

// getAPIKeyProvider returns an APIKeyProvider based on the channel.
// If multiple enabled API keys are configured, it returns a TraceStickyKeyProvider for consistent hashing.
// Otherwise, it returns a StaticKeyProvider.
//
// NOTE: This function panics when there is no enabled API key. This is intended as an assertion:
// buildChannelWithTransformer should validate channel credentials before constructing transformers.
func getAPIKeyProvider(ch *Channel) auth.APIKeyProvider {
	enabled := ch.cachedEnabledAPIKeys
	if len(enabled) > 1 {
		return NewTraceStickyKeyProvider(ch)
	}

	if len(enabled) == 1 {
		return auth.NewStaticKeyProvider(enabled[0])
	}

	panic(fmt.Errorf("no enabled api key configured for channel %s", ch.Name))
}

// BuildOutboundByAPIFormat returns the outbound transformer for the given API format.
// If the channel does not support the format, returns an error.
func BuildOutboundByAPIFormat(ch *Channel, apiFormat string) (transformer.Outbound, error) {
	if ch.Outbounds == nil {
		return nil, fmt.Errorf("channel %s has no outbounds configured", ch.Name)
	}

	out, ok := ch.Outbounds[apiFormat]
	if !ok {
		return nil, fmt.Errorf("channel %s does not support api_format %q", ch.Name, apiFormat)
	}

	return out, nil
}

// buildChannelWithOutbounds builds a Channel with its outbound transformers
// populated from the channel's endpoints. Each endpoint gets its own outbound
// transformer, allowing different API formats within the same channel.
func (svc *ChannelService) buildChannelWithOutbounds(c *ent.Channel) (*Channel, error) {
	ch, err := svc.buildChannelWithTransformer(c)
	if err != nil {
		return nil, err
	}

	endpoints := ch.ResolveEndpoints()
	if len(endpoints) == 0 {
		return ch, nil
	}

	defaultEndpoints := DefaultEndpointsForChannelType(c.Type)

	defaultAPIFormat := ""
	if len(defaultEndpoints) > 0 {
		defaultAPIFormat = defaultEndpoints[0].APIFormat
	}

	outbounds := make(map[string]transformer.Outbound, len(endpoints))
	accountIdentity := strconv.Itoa(c.ID)

	for _, ep := range endpoints {
		if ep.APIFormat == "" {
			continue
		}

		if ep.APIFormat == defaultAPIFormat && ep.Path == "" {
			outbounds[ep.APIFormat] = ch.Outbound
			continue
		}

		out, err := svc.buildNonDefaultEndpointOutbound(c, ch, accountIdentity, ep)
		if err != nil {
			return nil, fmt.Errorf("failed to build outbound for api_format %q on channel %s: %w", ep.APIFormat, c.Name, err)
		}

		outbounds[ep.APIFormat] = out
	}

	ch.Outbounds = outbounds

	return ch, nil
}

// buildNonDefaultEndpointOutbound creates a transformer for an endpoint whose
// api_format differs from the channel type's default. This handles the case
// where a channel supports multiple API formats (e.g., an openai channel
// with both chat/completions and responses endpoints).
func (svc *ChannelService) buildNonDefaultEndpointOutbound(
	c *ent.Channel,
	ch *Channel,
	accountIdentity string,
	ep objects.ChannelEndpoint,
) (transformer.Outbound, error) {
	apiKeyProvider := getAPIKeyProvider(ch)

	switch ep.APIFormat {
	case "openai/chat_completions":
		return openai.NewOutboundTransformerWithConfig(&openai.Config{
			PlatformType:    openai.PlatformOpenAI,
			BaseURL:         c.BaseURL,
			AccountIdentity: accountIdentity,
			APIKeyProvider:  apiKeyProvider,
			EndpointPath:    ep.Path,
		})
	case "openai/responses":
		return responses.NewOutboundTransformerWithConfig(&responses.Config{
			BaseURL:         c.BaseURL,
			AccountIdentity: accountIdentity,
			APIKeyProvider:  apiKeyProvider,
			EndpointPath:    ep.Path,
		})
	case "openai/image_generation", "openai/image_edit", "openai/image_variation", "openai/embeddings":
		return openai.NewOutboundTransformerWithConfig(&openai.Config{
			PlatformType:    openai.PlatformOpenAI,
			BaseURL:         c.BaseURL,
			AccountIdentity: accountIdentity,
			APIKeyProvider:  apiKeyProvider,
			EndpointPath:    ep.Path,
		})
	case "anthropic/messages":
		return anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:            anthropic.PlatformDirect,
			BaseURL:         c.BaseURL,
			AccountIdentity: accountIdentity,
			APIKeyProvider:  apiKeyProvider,
			EndpointPath:    ep.Path,
		})
	case "gemini/contents":
		return gemini.NewOutboundTransformerWithConfig(gemini.Config{
			BaseURL:         c.BaseURL,
			AccountIdentity: accountIdentity,
			APIKeyProvider:  apiKeyProvider,
			EndpointPath:    ep.Path,
		})
	case "jina/rerank", "jina/embeddings":
		return jina.NewOutboundTransformerWithConfig(&jina.Config{
			BaseURL:        c.BaseURL,
			APIKeyProvider: apiKeyProvider,
			EndpointPath:   ep.Path,
		})
	case "ollama/chat":
		return ollama.NewOutboundTransformerWithConfig(&ollama.Config{
			BaseURL:        c.BaseURL,
			APIKeyProvider: apiKeyProvider,
			EndpointPath:   ep.Path,
		})
	default:
		return nil, fmt.Errorf("unsupported api_format %q", ep.APIFormat)
	}
}

//nolint:maintidx // Checked.
func (svc *ChannelService) buildChannelWithTransformer(c *ent.Channel) (*Channel, error) {
	// Validate credentials early so we can fail fast without constructing HTTP clients/transformers.
	//
	// NOTE: "enabled" keys excludes keys that were explicitly disabled for this channel.
	enabledKeys := c.Credentials.GetEnabledAPIKeys(c.DisabledAPIKeys)

	//nolint:exhaustive // Checked.
	switch c.Type {
	case channel.TypeCodex, channel.TypeClaudecode:
		if !c.Credentials.IsOAuth() && len(enabledKeys) == 0 {
			return nil, fmt.Errorf("missing credentials: oauth or api key required for channel %s", c.Name)
		}
	case channel.TypeGithubCopilot:
		// GitHub Copilot requires OAuth credentials with device flow (strict OAuth only)
		if !c.Credentials.IsOAuth() {
			return nil, fmt.Errorf("missing oauth credentials for channel %s", c.Name)
		}
	case channel.TypeAntigravity:
		// Antigravity transformer currently consumes the single legacy APIKey field directly.
		if strings.TrimSpace(c.Credentials.APIKey) == "" {
			return nil, fmt.Errorf("missing api key for channel %s", c.Name)
		}
	case channel.TypeAnthropicGcp, channel.TypeAnthropicFake, channel.TypeOpenaiFake:
		// These channel types don't use API keys:
		// - anthropic_gcp uses GCP credentials JSON
		// - *_fake are test-only
	default:
		if len(enabledKeys) == 0 {
			return nil, fmt.Errorf("missing api key for channel %s", c.Name)
		}
	}

	httpClient := svc.getHttpClient(c.Settings)
	ch := buildChannel(c, httpClient)
	accountIdentity := strconv.Itoa(c.ID)

	switch c.Type {
	case channel.TypeDoubao, channel.TypeVolcengine:
		transformer, err := doubao.NewOutboundTransformerWithConfig(&doubao.Config{
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeFireworks:
		transformer, err := fireworks.NewOutboundTransformerWithConfig(&fireworks.Config{
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeOpenrouter, channel.TypeCerebras:
		transformer, err := openrouter.NewOutboundTransformerWithConfig(&openrouter.Config{
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeNanogpt:
		transformer, err := nanogpt.NewOutboundTransformerWithConfig(&nanogpt.Config{
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeNanogptResponses:
		transformer, err := responses.NewOutboundTransformerWithConfig(&responses.Config{
			BaseURL:         c.BaseURL,
			APIKeyProvider:  getAPIKeyProvider(ch),
			AccountIdentity: accountIdentity,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeZai, channel.TypeZhipu:
		transformer, err := zai.NewOutboundTransformerWithConfig(&zai.Config{
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeDeepseek:
		transformer, err := deepseek.NewOutboundTransformerWithConfig(&deepseek.Config{
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeMoonshot:
		transformer, err := moonshot.NewOutboundTransformerWithConfig(&moonshot.Config{
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeXai:
		transformer, err := xai.NewOutboundTransformerWithConfig(&xai.Config{
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeLongcatAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:           anthropic.PlatformLongCat,
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeAnthropic, channel.TypeMinimaxAnthropic, channel.TypeVolcengineAnthropic, channel.TypeAihubmixAnthropic, channel.TypeXiaomiAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:            anthropic.PlatformDirect,
			BaseURL:         c.BaseURL,
			AccountIdentity: accountIdentity,
			APIKeyProvider:  getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeClaudecode:
		// Check if using OAuth credentials first
		if c.Credentials.IsOAuth() {
			credsJSON := strings.TrimSpace(c.Credentials.APIKey)
			if c.Credentials.OAuth != nil {
				o := c.Credentials.OAuth

				creds, err := (&oauth.OAuthCredentials{
					AccessToken:  o.AccessToken,
					RefreshToken: o.RefreshToken,
					ClientID:     o.ClientID,
					ExpiresAt:    o.ExpiresAt,
					TokenType:    o.TokenType,
					Scopes:       o.Scopes,
				}).ToJSON()
				if err != nil {
					return nil, fmt.Errorf("failed to encode claudecode oauth credentials: %w", err)
				}

				credsJSON = creds
			}

			creds, err := oauth.ParseCredentialsJSON(credsJSON)
			if err != nil {
				return nil, fmt.Errorf("failed to parse claudecode oauth credentials: %w", err)
			}

			tokens := claudecode.NewTokenProvider(oauth.TokenProviderParams{
				Credentials: creds,
				HTTPClient:  httpClient,
				OnRefreshed: svc.onTokenRefreshed(c),
			})

			transformer, err := claudecode.NewOutboundTransformer(claudecode.Params{
				TokenProvider:   tokens,
				BaseURL:         c.BaseURL,
				IsOfficial:      true,
				AccountIdentity: accountIdentity,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create claudecode outbound transformer: %w", err)
			}

			ch.Outbound = transformer
			setupAutoRefresh(ch, tokens, oauth.AutoRefreshOptions{})

			return ch, nil
		}

		// Non-OAuth: use APIKeyProvider for multi-key rotation support
		apiKeyProvider := getAPIKeyProvider(ch)
		tokens := oauth.NewAPIKeyTokenProvider(apiKeyProvider.Get)

		transformer, err := claudecode.NewOutboundTransformer(claudecode.Params{
			TokenProvider:   tokens,
			BaseURL:         c.BaseURL,
			IsOfficial:      false,
			AccountIdentity: accountIdentity,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create claudecode outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeDeepseekAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:           anthropic.PlatformDeepSeek,
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeDoubaoAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:           anthropic.PlatformDoubao,
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeMoonshotAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:           anthropic.PlatformMoonshot,
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeZhipuAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:           anthropic.PlatformZhipu,
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeZaiAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:           anthropic.PlatformZai,
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil

	case channel.TypeAnthropicAWS:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:            anthropic.PlatformBedrock,
			BaseURL:         c.BaseURL,
			AccountIdentity: accountIdentity,
			APIKeyProvider:  getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeAnthropicGcp:
		// For anthropic_vertex, we need to create a VertexTransformer with GCP credentials
		// The transformer will handle Google Vertex AI integration
		if c.Credentials.GCP == nil {
			return nil, errors.New("GCP credentials are required for anthropic_vertex channel")
		}

		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:            anthropic.PlatformVertex,
			Region:          c.Credentials.GCP.Region,
			ProjectID:       c.Credentials.GCP.ProjectID,
			JSONData:        c.Credentials.GCP.JSONData,
			AccountIdentity: accountIdentity,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeAnthropicFake:
		// For anthropic_fake, we use the fake transformer for testing
		ch.Outbound = anthropic.NewFakeTransformer()
		return ch, nil
	case channel.TypeOpenaiFake:
		ch.Outbound = openai.NewFakeTransformer()
		return ch, nil
	case channel.TypeModelscope:
		transformer, err := modelscope.NewOutboundTransformerWithConfig(&modelscope.Config{
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeGeminiOpenai:
		transformer, err := geminioai.NewOutboundTransformerWithConfig(&geminioai.Config{
			BaseURL:         c.BaseURL,
			APIKeyProvider:  getAPIKeyProvider(ch),
			AccountIdentity: accountIdentity,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeLongcat:
		transformer, err := longcat.NewOutboundTransformerWithConfig(&longcat.Config{
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeBailian:
		transformer, err := bailian.NewOutboundTransformerWithConfig(&bailian.Config{
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeBailianAnthropic, channel.TypeMoonshotCoding:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:            anthropic.PlatformDirect,
			BaseURL:         c.BaseURL,
			AccountIdentity: accountIdentity,
			APIKeyProvider:  getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeCodex:
		// Check if using OAuth credentials first
		if c.Credentials.IsOAuth() {
			credsJSON := strings.TrimSpace(c.Credentials.APIKey)
			if c.Credentials.OAuth != nil {
				o := c.Credentials.OAuth

				creds, err := (&oauth.OAuthCredentials{
					AccessToken:  o.AccessToken,
					RefreshToken: o.RefreshToken,
					ClientID:     o.ClientID,
					ExpiresAt:    o.ExpiresAt,
					TokenType:    o.TokenType,
					Scopes:       o.Scopes,
				}).ToJSON()
				if err != nil {
					return nil, fmt.Errorf("failed to encode codex oauth credentials: %w", err)
				}

				credsJSON = creds
			}

			creds, err := oauth.ParseCredentialsJSON(credsJSON)
			if err != nil {
				return nil, fmt.Errorf("failed to parse codex oauth credentials: %w", err)
			}

			p := codex.NewTokenProvider(codex.TokenProviderParams{
				Credentials: creds,
				HTTPClient:  httpClient,
				OnRefreshed: svc.onTokenRefreshed(c),
			})

			transformer, err := codex.NewOutboundTransformer(codex.Params{
				TokenProvider:   p,
				BaseURL:         c.BaseURL,
				AccountIdentity: accountIdentity,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create codex outbound transformer: %w", err)
			}

			ch.Outbound = transformer
			setupAutoRefresh(ch, p, oauth.AutoRefreshOptions{})

			return ch, nil
		}

		// Non-OAuth: use APIKeyProvider for multi-key rotation support
		apiKeyProvider := getAPIKeyProvider(ch)
		tokens := oauth.NewAPIKeyTokenProvider(apiKeyProvider.Get)

		transformer, err := codex.NewOutboundTransformer(codex.Params{
			TokenProvider:   tokens,
			BaseURL:         c.BaseURL,
			AccountIdentity: accountIdentity,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create codex outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeGithubCopilot:
		// GitHub Copilot requires OAuth credentials with device flow
		if !c.Credentials.IsOAuth() {
			return nil, fmt.Errorf("missing oauth credentials for channel %s", c.Name)
		}

		credsJSON := strings.TrimSpace(c.Credentials.APIKey)
		if credsJSON == "" {
			return nil, fmt.Errorf("github_copilot channel %s has no credentials", c.Name)
		}

		if c.Credentials.OAuth != nil {
			o := c.Credentials.OAuth

			creds, err := (&oauth.OAuthCredentials{
				AccessToken:  o.AccessToken,
				RefreshToken: o.RefreshToken,
				ClientID:     o.ClientID,
				ExpiresAt:    o.ExpiresAt,
				TokenType:    o.TokenType,
				Scopes:       o.Scopes,
			}).ToJSON()
			if err != nil {
				return nil, fmt.Errorf("failed to encode github_copilot oauth credentials for channel %s: %w", c.Name, err)
			}

			credsJSON = creds
		}

		creds, err := oauth.ParseCredentialsJSON(credsJSON)
		if err != nil {
			return nil, fmt.Errorf("github_copilot channel %s has invalid credentials: %w", c.Name, err)
		}

		// Create CopilotTokenProvider with the token exchanger
		p, err := copilot.NewTokenProvider(copilot.TokenProviderParams{
			Credentials: creds,
			HTTPClient:  httpClient,
			OnRefreshed: svc.onTokenRefreshed(c),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create CopilotTokenProvider: %w", err)
		}

		// Create the Copilot outbound transformer with LiteLLM headers
		transformer, err := copilot.NewOutboundTransformer(copilot.OutboundTransformerParams{
			TokenProvider: p,
			BaseURL:       c.BaseURL,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create github_copilot outbound transformer: %w", err)
		}
		ch.Outbound = transformer
		setupAutoRefresh(ch, p, oauth.AutoRefreshOptions{
			Interval:      5 * time.Minute,
			RefreshBefore: 5 * time.Minute,
		})

		return ch, nil
	case channel.TypeOpenai, channel.TypeDeepinfra, channel.TypeMinimax, channel.TypeXiaomi,
		channel.TypePpio, channel.TypeSiliconflow,
		channel.TypeVercel, channel.TypeAihubmix, channel.TypeBurncloud, channel.TypeGithub:
		transformer, err := openai.NewOutboundTransformerWithConfig(&openai.Config{
			PlatformType:    openai.PlatformOpenAI,
			BaseURL:         c.BaseURL,
			AccountIdentity: accountIdentity,
			APIKeyProvider:  getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeOpenaiResponses:
		transformer, err := responses.NewOutboundTransformerWithConfig(&responses.Config{
			BaseURL:         c.BaseURL,
			APIKeyProvider:  getAPIKeyProvider(ch),
			AccountIdentity: accountIdentity,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeGemini:
		transformer, err := gemini.NewOutboundTransformerWithConfig(gemini.Config{
			BaseURL:         c.BaseURL,
			APIKeyProvider:  getAPIKeyProvider(ch),
			AccountIdentity: accountIdentity,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeGeminiVertex:
		transformer, err := gemini.NewOutboundTransformerWithConfig(gemini.Config{
			BaseURL:         c.BaseURL,
			APIKeyProvider:  getAPIKeyProvider(ch),
			PlatformType:    gemini.PlatformVertex,
			AccountIdentity: accountIdentity,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeJina:
		transformer, err := jina.NewOutboundTransformerWithConfig(&jina.Config{
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeAntigravity:
		transformer, err := antigravity.NewTransformer(
			antigravity.Config{BaseURL: c.BaseURL, APIKey: c.Credentials.APIKey},
			antigravity.WithHTTPClient(httpClient),
			antigravity.WithOnTokenRefreshed(svc.onTokenRefreshed(c)),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create antigravity outbound transformer: %w", err)
		}

		ch.Outbound = transformer
		tokens := transformer.GetTokenProvider()
		if tokens != nil {
			setupAutoRefresh(ch, tokens, oauth.AutoRefreshOptions{})
		}

		return ch, nil
	case channel.TypeOllama:
		// Ollama is often used locally without API key, but may also be configured with one
		var apiKeyProvider auth.APIKeyProvider
		if len(ch.cachedEnabledAPIKeys) > 0 {
			apiKeyProvider = getAPIKeyProvider(ch)
		}

		transformer, err := ollama.NewOutboundTransformerWithConfig(&ollama.Config{
			BaseURL:        c.BaseURL,
			APIKeyProvider: apiKeyProvider,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create ollama outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	default:
		return nil, errors.New("unknown channel type")
	}
}

func isOAuthJSON(s string) bool {
	trimmed := strings.TrimSpace(s)
	return strings.HasPrefix(trimmed, "{") && strings.Contains(s, "access_token")
}

func extractProjectIDFromAntigravityCreds(apiKey string) (string, error) {
	parts := strings.Split(apiKey, "|")
	if len(parts) >= 2 {
		return parts[1], nil
	}
	return "", errors.New("api key does not contain project ID (expected format: \"<refreshToken>|<projectID>\")")
}

func (svc *ChannelService) refreshOAuthToken(ctx context.Context, ch *ent.Channel, refreshed *oauth.OAuthCredentials) error {
	if refreshed == nil {
		return nil
	}

	updated := ch.Credentials

	if ch.Type == channel.TypeAntigravity {
		projectID, err := extractProjectIDFromAntigravityCreds(ch.Credentials.APIKey)
		if err != nil {
			log.Warn(ctx, "failed to extract project ID from antigravity credentials",
				log.Cause(err),
				log.String("channel", ch.Name))
			return fmt.Errorf("failed to extract project ID from antigravity credentials: %w", err)
		}
		updated.APIKey = fmt.Sprintf("%s|%s", refreshed.RefreshToken, projectID)
	} else {
		credJSON, err := refreshed.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to serialize refreshed credentials: %w", err)
		}
		// NOTE：必须是使用 APIKey 字段，不能使用 API Keys 字段
		updated.APIKey = credJSON
	}

	updated.OAuth = refreshed

	_, err := svc.entFromContext(ctx).Channel.UpdateOneID(ch.ID).SetCredentials(updated).Save(ctx)

	return err
}

// GetModelEntries returns all models this channel can handle, RequestModel -> Entry
// This unifies:
// - SupportedModels (direct models)
// - ExtraModelPrefix (prefixed models)
// - AutoTrimedModelPrefixes (auto-trimmed models)
// - ModelMappings (mapped models)
// The result is cached for performance.
//
// WARNING: The returned map is internal cached state.
// DO NOT modify the returned map or its ChannelModelEntry values.
// Modifications will not persist and may cause data inconsistency.
func (ch *Channel) GetModelEntries() map[string]ChannelModelEntry {
	// Return cached result if available
	if ch.cachedModelEntries != nil {
		return ch.cachedModelEntries
	}

	entries := make(map[string]ChannelModelEntry)

	// 1. Direct models from SupportedModels
	for _, model := range ch.SupportedModels {
		if _, exists := entries[model]; !exists {
			entries[model] = ChannelModelEntry{
				RequestModel: model,
				ActualModel:  model,
				Source:       "direct",
			}
		}
	}

	if ch.Settings == nil {
		ch.cachedModelEntries = entries
		return entries
	}

	// 2. Prefixed models (ExtraModelPrefix)
	if ch.Settings.ExtraModelPrefix != "" {
		prefix := ch.Settings.ExtraModelPrefix
		for _, model := range ch.SupportedModels {
			prefixedModel := prefix + "/" + model
			if _, exists := entries[prefixedModel]; !exists {
				entries[prefixedModel] = ChannelModelEntry{
					RequestModel: prefixedModel,
					ActualModel:  model,
					Source:       "prefix",
				}
			}
		}
	}

	// 3. Auto-trimmed models (AutoTrimedModelPrefixes)
	for _, prefix := range ch.Settings.AutoTrimedModelPrefixes {
		if prefix == "" {
			continue
		}

		prefix += "/"
		for _, model := range ch.SupportedModels {
			// Only process models that have the prefix
			if after, ok := strings.CutPrefix(model, prefix); ok {
				trimmedModel := after
				if _, exists := entries[trimmedModel]; !exists {
					entries[trimmedModel] = ChannelModelEntry{
						RequestModel: trimmedModel,
						ActualModel:  model,
						Source:       "auto_trim",
					}
				}
			}
		}
	}

	// 4. Model mappings
	for _, mapping := range ch.Settings.ModelMappings {
		// Only add if the target model is supported
		if slices.Contains(ch.SupportedModels, mapping.To) {
			if _, exists := entries[mapping.From]; !exists {
				entries[mapping.From] = ChannelModelEntry{
					RequestModel: mapping.From,
					ActualModel:  mapping.To,
					Source:       "mapping",
				}
				// When hideMappedModels is enabled, remove mapped models from the entries
				if ch.Settings.HideMappedModels {
					delete(entries, mapping.To)
				}
			}
		}
	}

	// 5. Hide original models if configured
	// When hideOriginalModels is enabled, remove direct models from the entries
	// This allows only transformed models (prefix, auto_trim, mapping) to be exposed
	if ch.Settings.HideOriginalModels {
		for key, entry := range entries {
			if entry.Source == "direct" {
				delete(entries, key)
			}
		}
	}

	ch.cachedModelEntries = entries

	return entries
}

var defaultEndpointsForChannelType = map[channel.Type][]objects.ChannelEndpoint{
	channel.TypeOpenai:            {{APIFormat: "openai/chat_completions"}},
	channel.TypeOpenaiResponses:   {{APIFormat: "openai/responses"}},
	channel.TypeCodex:             {{APIFormat: "openai/responses"}},
	channel.TypeVercel:            {{APIFormat: "openai/chat_completions"}},
	channel.TypeAnthropic:         {{APIFormat: "anthropic/messages"}},
	channel.TypeAnthropicAWS:      {{APIFormat: "anthropic/messages"}},
	channel.TypeAnthropicGcp:      {{APIFormat: "anthropic/messages"}},
	channel.TypeGeminiOpenai:      {{APIFormat: "openai/chat_completions"}},
	channel.TypeGemini:            {{APIFormat: "gemini/contents"}},
	channel.TypeGeminiVertex:      {{APIFormat: "gemini/contents"}},
	channel.TypeDeepseek:          {{APIFormat: "openai/chat_completions"}},
	channel.TypeDeepseekAnthropic: {{APIFormat: "anthropic/messages"}},
	channel.TypeDeepinfra:         {{APIFormat: "openai/chat_completions"}},
	channel.TypeFireworks:         {{APIFormat: "openai/chat_completions"}},
	channel.TypeDoubao:            {{APIFormat: "openai/chat_completions"}},
	channel.TypeDoubaoAnthropic:   {{APIFormat: "anthropic/messages"}},
	channel.TypeMoonshot:          {{APIFormat: "openai/chat_completions"}},
	channel.TypeMoonshotAnthropic: {{APIFormat: "anthropic/messages"}},
	channel.TypeZhipu:             {{APIFormat: "openai/chat_completions"}},
	channel.TypeZai:               {{APIFormat: "openai/chat_completions"}},
	channel.TypeZhipuAnthropic:    {{APIFormat: "anthropic/messages"}},
	channel.TypeZaiAnthropic:      {{APIFormat: "anthropic/messages"}},
	channel.TypeAnthropicFake:     {{APIFormat: "anthropic/messages"}},
	channel.TypeOpenaiFake:        {{APIFormat: "openai/chat_completions"}},
	channel.TypeOpenrouter:        {{APIFormat: "openai/chat_completions"}},
	channel.TypeXiaomi:            {{APIFormat: "openai/chat_completions"}},
	channel.TypeXai:               {{APIFormat: "openai/chat_completions"}},
	channel.TypePpio:              {{APIFormat: "openai/chat_completions"}},
	channel.TypeSiliconflow:       {{APIFormat: "openai/chat_completions"}},
	channel.TypeVolcengine:        {{APIFormat: "openai/chat_completions"}},
	channel.TypeLongcat:           {{APIFormat: "openai/chat_completions"}},
	channel.TypeLongcatAnthropic:  {{APIFormat: "anthropic/messages"}},
	channel.TypeMinimax:           {{APIFormat: "openai/chat_completions"}},
	channel.TypeMinimaxAnthropic:  {{APIFormat: "anthropic/messages"}},
	channel.TypeAihubmix:          {{APIFormat: "openai/chat_completions"}},
	channel.TypeBurncloud:         {{APIFormat: "openai/chat_completions"}},
	channel.TypeModelscope:        {{APIFormat: "openai/chat_completions"}},
	channel.TypeBailian:           {{APIFormat: "openai/chat_completions"}},
	channel.TypeBailianAnthropic:  {{APIFormat: "anthropic/messages"}},
	channel.TypeMoonshotCoding:    {{APIFormat: "anthropic/messages"}},
	channel.TypeJina: {
		{APIFormat: "jina/rerank"},
		{APIFormat: "jina/embeddings"},
	},
	channel.TypeGithub:           {{APIFormat: "openai/chat_completions"}},
	channel.TypeGithubCopilot:    {{APIFormat: "openai/chat_completions"}},
	channel.TypeClaudecode:       {{APIFormat: "anthropic/messages"}},
	channel.TypeCerebras:         {{APIFormat: "openai/chat_completions"}},
	channel.TypeAntigravity:      {{APIFormat: "gemini/contents"}},
	channel.TypeNanogpt:          {{APIFormat: "openai/chat_completions"}},
	channel.TypeNanogptResponses: {{APIFormat: "openai/responses"}},
	channel.TypeOllama:           {{APIFormat: "ollama/chat"}},
}

func DefaultEndpointsForChannelType(t channel.Type) []objects.ChannelEndpoint {
	if eps, ok := defaultEndpointsForChannelType[t]; ok {
		return eps
	}

	return nil
}

func (c *Channel) ResolveEndpoints() []objects.ChannelEndpoint {
	if c.Channel == nil {
		return nil
	}

	if len(c.Endpoints) > 0 {
		return c.Endpoints
	}

	return DefaultEndpointsForChannelType(c.Type)
}

// GetDirectModelEntries returns the direct models this channel can handle.
// This is used for testing purposes where we need to see all available models
// regardless of the HideOriginalModels setting.
// The difference from GetModelEntries is that this method does NOT filter out
// direct models when HideOriginalModels is enabled.
func (ch *Channel) GetDirectModelEntries() map[string]ChannelModelEntry {
	entries := make(map[string]ChannelModelEntry)

	for _, model := range ch.SupportedModels {
		if _, exists := entries[model]; !exists {
			entries[model] = ChannelModelEntry{
				RequestModel: model,
				ActualModel:  model,
				Source:       "direct",
			}
		}
	}

	return entries
}
