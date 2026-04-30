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
	"github.com/looplj/axonhub/llm"
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

// BuildOutboundByAPIFormat returns the outbound transformer for a resolved endpoint API format.
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
// populated from the channel's resolved default endpoints.
//
// A channel always has one primary default endpoint, which is the first item in
// the resolved default endpoint list and backs Channel.Outbound for backward
// compatibility. Additional default endpoints are peer capability surfaces for
// the same channel type, each bound to exactly one API format.
func (svc *ChannelService) buildChannelWithOutbounds(c *ent.Channel) (*Channel, error) {
	ch, err := svc.buildChannelWithTransformer(c)
	if err != nil {
		return nil, err
	}

	defaultEndpoints := DefaultEndpointsForChannelType(c.Type)
	userEndpoints := c.Endpoints

	if len(defaultEndpoints) == 0 && len(userEndpoints) == 0 {
		return ch, nil
	}

	outbounds := make(map[string]transformer.Outbound)
	accountIdentity := strconv.Itoa(c.ID)

	for _, ep := range defaultEndpoints {
		if ep.APIFormat == "" {
			continue
		}

		outbounds[ep.APIFormat] = ch.Outbound
	}

	for _, ep := range userEndpoints {
		if ep.APIFormat == "" {
			continue
		}
		out, err := svc.buildNonDefaultEndpointOutbound(c, ch, accountIdentity, ep)
		if err != nil {
			return nil, fmt.Errorf("failed to build outbound for api_format %q on channel %s: %w", ep.APIFormat, c.Name, err)
		}
		outbounds[ep.APIFormat] = out
	}

	if len(outbounds) == 0 {
		return ch, nil
	}

	ch.Outbounds = outbounds

	return ch, nil
}

// buildNonDefaultEndpointOutbound creates a transformer for a user-configured
// endpoint override. Default endpoints are handled by buildChannelWithOutbounds
// and always use the primary outbound.
// Multiple api_format values may share the same provider outbound implementation.
func (svc *ChannelService) buildNonDefaultEndpointOutbound(
	c *ent.Channel,
	ch *Channel,
	accountIdentity string,
	ep objects.ChannelEndpoint,
) (transformer.Outbound, error) {
	apiKeyProvider := getAPIKeyProvider(ch)

	switch ep.APIFormat {
	case llm.APIFormatOpenAIChatCompletion.String():
		return openai.NewOutboundTransformerWithConfig(&openai.Config{
			PlatformType:    openai.PlatformOpenAI,
			BaseURL:         c.BaseURL,
			AccountIdentity: accountIdentity,
			APIKeyProvider:  apiKeyProvider,
			EndpointPath:    ep.Path,
		})
	case llm.APIFormatOpenAIResponse.String(),
		llm.APIFormatOpenAIResponseCompact.String():
		return responses.NewOutboundTransformerWithConfig(&responses.Config{
			BaseURL:         c.BaseURL,
			AccountIdentity: accountIdentity,
			APIKeyProvider:  apiKeyProvider,
			EndpointPath:    ep.Path,
		})
	case llm.APIFormatOpenAIEmbedding.String(),
		llm.APIFormatOpenAIImageGeneration.String(),
		llm.APIFormatOpenAIImageEdit.String(),
		llm.APIFormatOpenAIImageVariation.String(),
		llm.APIFormatOpenAIVideo.String():
		return openai.NewOutboundTransformerWithConfig(&openai.Config{
			PlatformType:    openai.PlatformOpenAI,
			BaseURL:         c.BaseURL,
			AccountIdentity: accountIdentity,
			APIKeyProvider:  apiKeyProvider,
			EndpointPath:    ep.Path,
		})
	case llm.APIFormatAnthropicMessage.String():
		return anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:            anthropic.PlatformDirect,
			BaseURL:         c.BaseURL,
			AccountIdentity: accountIdentity,
			APIKeyProvider:  apiKeyProvider,
			EndpointPath:    ep.Path,
		})
	case llm.APIFormatGeminiContents.String():
		return gemini.NewOutboundTransformerWithConfig(gemini.Config{
			BaseURL:         c.BaseURL,
			AccountIdentity: accountIdentity,
			APIKeyProvider:  apiKeyProvider,
			EndpointPath:    ep.Path,
			PlatformType:    ch.platformTypeForGeminiEndpoint(),
		})
	case llm.APIFormatGeminiEmbedding.String():
		return gemini.NewOutboundTransformerWithConfig(gemini.Config{
			BaseURL:         c.BaseURL,
			AccountIdentity: accountIdentity,
			APIKeyProvider:  apiKeyProvider,
			EndpointPath:    ep.Path,
			PlatformType:    ch.platformTypeForGeminiEndpoint(),
		})
	case llm.APIFormatJinaRerank.String(), llm.APIFormatJinaEmbedding.String():
		return jina.NewOutboundTransformerWithConfig(&jina.Config{
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

var openAICompatibleDefaultEndpoints = []objects.ChannelEndpoint{
	{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
	{APIFormat: llm.APIFormatOpenAIEmbedding.String()},
	{APIFormat: llm.APIFormatOpenAIImageGeneration.String()},
	{APIFormat: llm.APIFormatOpenAIImageEdit.String()},
	{APIFormat: llm.APIFormatOpenAIImageVariation.String()},
	{APIFormat: llm.APIFormatOpenAIVideo.String()},
}

var openAIChatOnlyDefaultEndpoints = []objects.ChannelEndpoint{
	{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
}

// defaultEndpointsForChannelType defines the built-in default endpoints for
// each channel type.
//
// A default endpoint is a first-class built-in capability surface owned by the
// channel type. The first endpoint is the primary endpoint and backs
// Channel.Outbound for backward compatibility. Additional entries are peer
// default endpoints, each mapped to exactly one API format / outbound
// transformer pair.
//
// Only include endpoints that are intentionally part of the channel type's
// built-in contract. User-configured custom endpoints remain external overrides
// and are not modeled here.
var defaultEndpointsForChannelType = map[channel.Type][]objects.ChannelEndpoint{
	channel.TypeOpenai:          openAICompatibleDefaultEndpoints,
	channel.TypeOpenaiResponses: {{APIFormat: llm.APIFormatOpenAIResponse.String()}},
	channel.TypeCodex:           {{APIFormat: llm.APIFormatOpenAIResponse.String()}},
	channel.TypeVercel:          openAICompatibleDefaultEndpoints,
	channel.TypeAnthropic:       {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeAnthropicAWS:    {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeAnthropicGcp:    {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeGeminiOpenai:    {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeGemini: {
		{APIFormat: llm.APIFormatGeminiContents.String()},
		{APIFormat: llm.APIFormatGeminiEmbedding.String()},
	},
	channel.TypeGeminiVertex: {
		{APIFormat: llm.APIFormatGeminiContents.String()},
		{APIFormat: llm.APIFormatGeminiEmbedding.String()},
	},
	channel.TypeDeepseek:          {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeDeepseekAnthropic: {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeDeepinfra:         openAICompatibleDefaultEndpoints,
	channel.TypeFireworks:         {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeDoubao: {
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
		{APIFormat: llm.APIFormatSeedanceVideo.String()},
	},
	channel.TypeDoubaoAnthropic:     {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeMoonshot:            {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeMoonshotAnthropic:   {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeZhipu:               {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeZai:                 {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeZhipuAnthropic:      {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeZaiAnthropic:        {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeAnthropicFake:       {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeOpenaiFake:          {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeOpenrouter:          {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeXiaomi:              openAIChatOnlyDefaultEndpoints,
	channel.TypeXiaomiAnthropic:     {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeXai:                 {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypePpio:                openAICompatibleDefaultEndpoints,
	channel.TypeSiliconflow:         openAICompatibleDefaultEndpoints,
	channel.TypeVolcengine:          {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeVolcengineAnthropic: {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeLongcat:             {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeLongcatAnthropic:    {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeMinimax:             openAIChatOnlyDefaultEndpoints,
	channel.TypeMinimaxAnthropic:    {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeAihubmix:            openAICompatibleDefaultEndpoints,
	channel.TypeAihubmixAnthropic:   {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeBurncloud:           openAICompatibleDefaultEndpoints,
	channel.TypeModelscope:          {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeBailian:             {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeBailianAnthropic:    {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeMoonshotCoding:      {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeJina: {
		{APIFormat: llm.APIFormatJinaRerank.String()},
		{APIFormat: llm.APIFormatJinaEmbedding.String()},
	},
	channel.TypeGithub:           openAICompatibleDefaultEndpoints,
	channel.TypeGithubCopilot:    {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeClaudecode:       {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeCerebras:         {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeAntigravity:      {{APIFormat: llm.APIFormatGeminiContents.String()}},
	channel.TypeNanogpt:          openAICompatibleDefaultEndpoints,
	channel.TypeNanogptResponses: {{APIFormat: llm.APIFormatOpenAIResponse.String()}},
	channel.TypeOllama:           {{APIFormat: llm.APIFormatOllamaChat.String()}},
}

func DefaultEndpointsForChannelType(t channel.Type) []objects.ChannelEndpoint {
	if eps, ok := defaultEndpointsForChannelType[t]; ok {
		return eps
	}

	return nil
}

func mergeEndpoints(defaultEndpoints, userEndpoints []objects.ChannelEndpoint) []objects.ChannelEndpoint {
	if len(defaultEndpoints) == 0 && len(userEndpoints) == 0 {
		return nil
	}

	merged := make([]objects.ChannelEndpoint, 0, len(defaultEndpoints)+len(userEndpoints))

	overrides := make(map[string]objects.ChannelEndpoint, len(userEndpoints))

	for _, ep := range userEndpoints {
		if ep.APIFormat == "" {
			continue
		}

		overrides[ep.APIFormat] = ep
	}

	for _, ep := range defaultEndpoints {
		if ep.APIFormat == "" {
			continue
		}

		if override, ok := overrides[ep.APIFormat]; ok {
			merged = append(merged, override)

			delete(overrides, ep.APIFormat)

			continue
		}

		merged = append(merged, ep)
	}

	for _, ep := range userEndpoints {
		if ep.APIFormat == "" {
			continue
		}

		if _, ok := overrides[ep.APIFormat]; !ok {
			continue
		}

		merged = append(merged, ep)

		delete(overrides, ep.APIFormat)
	}

	return merged
}

// ResolveEndpoints returns the runtime-effective endpoints used for API format
// selection. Built-in default endpoints define the channel's capability
// surface, and user-configured endpoints override matching api_format entries
// or append additional ones.
func (c *Channel) ResolveEndpoints() []objects.ChannelEndpoint {
	if c.Channel == nil {
		return nil
	}

	return mergeEndpoints(DefaultEndpointsForChannelType(c.Type), c.Endpoints)
}

func (c *Channel) platformTypeForGeminiEndpoint() string {
	if c == nil || c.Channel == nil {
		return ""
	}

	if c.Type == channel.TypeGeminiVertex {
		return gemini.PlatformVertex
	}

	return ""
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
