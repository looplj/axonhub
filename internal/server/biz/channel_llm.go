package biz

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channelmodelprice"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/anthropic/claudecode"
	"github.com/looplj/axonhub/llm/transformer/antigravity"
	"github.com/looplj/axonhub/llm/transformer/bailian"
	"github.com/looplj/axonhub/llm/transformer/deepseek"
	"github.com/looplj/axonhub/llm/transformer/doubao"
	"github.com/looplj/axonhub/llm/transformer/gemini"
	geminioai "github.com/looplj/axonhub/llm/transformer/gemini/openai"
	"github.com/looplj/axonhub/llm/transformer/jina"
	"github.com/looplj/axonhub/llm/transformer/longcat"
	"github.com/looplj/axonhub/llm/transformer/modelscope"
	"github.com/looplj/axonhub/llm/transformer/moonshot"
	"github.com/looplj/axonhub/llm/transformer/nanogpt"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
	"github.com/looplj/axonhub/llm/transformer/openrouter"
	"github.com/looplj/axonhub/llm/transformer/xai"
	"github.com/looplj/axonhub/llm/transformer/zai"
)

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

	httpClient := httpclient.NewHttpClientWithProxy(getProxyConfig(c.Settings))
	ch := buildChannel(c, httpClient)

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
	case channel.TypeAnthropic, channel.TypeMinimaxAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:           anthropic.PlatformDirect,
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
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
				OnRefreshed: svc.refreshOAuthTokenFunc(c),
			})

			transformer, err := claudecode.NewOutboundTransformer(claudecode.Params{
				TokenProvider: tokens,
				BaseURL:       c.BaseURL,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create claudecode outbound transformer: %w", err)
			}

			ch.Outbound = transformer
			ch.startTokenProvider = func() {
				tokens.StartAutoRefresh(context.Background(), oauth.AutoRefreshOptions{})
			}
			ch.stopTokenProvider = tokens.StopAutoRefresh

			return ch, nil
		}

		// Non-OAuth: use APIKeyProvider for multi-key rotation support
		apiKeyProvider := getAPIKeyProvider(ch)
		tokens := oauth.NewAPIKeyTokenProvider(apiKeyProvider.Get)

		transformer, err := claudecode.NewOutboundTransformer(claudecode.Params{
			TokenProvider: tokens,
			BaseURL:       c.BaseURL,
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
			Type:           anthropic.PlatformBedrock,
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
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
			Type:      anthropic.PlatformVertex,
			Region:    c.Credentials.GCP.Region,
			ProjectID: c.Credentials.GCP.ProjectID,
			JSONData:  c.Credentials.GCP.JSONData,
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
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
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
				OnRefreshed: svc.refreshOAuthTokenFunc(c),
			})

			transformer, err := codex.NewOutboundTransformer(codex.Params{
				TokenProvider: p,
				BaseURL:       c.BaseURL,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create codex outbound transformer: %w", err)
			}

			ch.Outbound = transformer
			ch.startTokenProvider = func() {
				p.StartAutoRefresh(context.Background(), oauth.AutoRefreshOptions{})
			}
			ch.stopTokenProvider = p.StopAutoRefresh

			return ch, nil
		}

		// Non-OAuth: use APIKeyProvider for multi-key rotation support
		apiKeyProvider := getAPIKeyProvider(ch)
		tokens := oauth.NewAPIKeyTokenProvider(apiKeyProvider.Get)

		transformer, err := codex.NewOutboundTransformer(codex.Params{
			TokenProvider: tokens,
			BaseURL:       c.BaseURL,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create codex outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeOpenai, channel.TypeDeepinfra, channel.TypeMinimax,
		channel.TypePpio, channel.TypeSiliconflow,
		channel.TypeVercel, channel.TypeAihubmix, channel.TypeBurncloud, channel.TypeGithub:
		transformer, err := openai.NewOutboundTransformerWithConfig(&openai.Config{
			PlatformType:   openai.PlatformOpenAI,
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeOpenaiResponses:
		transformer, err := responses.NewOutboundTransformerWithConfig(&responses.Config{
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeGemini:
		transformer, err := gemini.NewOutboundTransformerWithConfig(gemini.Config{
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		ch.Outbound = transformer

		return ch, nil
	case channel.TypeGeminiVertex:
		transformer, err := gemini.NewOutboundTransformerWithConfig(gemini.Config{
			BaseURL:        c.BaseURL,
			APIKeyProvider: getAPIKeyProvider(ch),
			PlatformType:   gemini.PlatformVertex,
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
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create antigravity outbound transformer: %w", err)
		}

		ch.Outbound = transformer
		tokens := transformer.GetTokenProvider()
		if tokens != nil {
			ch.startTokenProvider = func() {
				tokens.StartAutoRefresh(context.Background(), oauth.AutoRefreshOptions{})
			}
			ch.stopTokenProvider = tokens.StopAutoRefresh
		}

		return ch, nil
	default:
		return nil, errors.New("unknown channel type")
	}
}

func isOAuthJSON(s string) bool {
	trimmed := strings.TrimSpace(s)
	return strings.HasPrefix(trimmed, "{") && strings.Contains(s, "access_token")
}

func (svc *ChannelService) refreshOAuthTokenFunc(ch *ent.Channel) func(ctx context.Context, refreshed *oauth.OAuthCredentials) error {
	return func(ctx context.Context, refreshed *oauth.OAuthCredentials) error {
		if refreshed == nil {
			return nil
		}

		credJSON, err := refreshed.ToJSON()
		if err != nil {
			return err
		}

		updated := ch.Credentials

		updated.APIKey = credJSON
		updated.OAuth = refreshed

		dbCtx := privacy.DecisionContext(ctx, privacy.Allow)
		_, err = svc.entFromContext(dbCtx).Channel.UpdateOneID(ch.ID).SetCredentials(updated).Save(dbCtx)

		return err
	}
}

// preloadModelPrices loads active model prices for a channel and caches them.
func (svc *ChannelService) preloadModelPrices(ctx context.Context, ch *Channel) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	prices, err := svc.entFromContext(ctx).ChannelModelPrice.Query().
		Where(
			channelmodelprice.ChannelID(ch.ID),
			channelmodelprice.DeletedAtEQ(0),
		).
		All(ctx)
	if err != nil {
		log.Warn(ctx, "failed to preload model prices", log.Int("channel_id", ch.ID), log.Cause(err))
		return
	}

	cache := make(map[string]*ent.ChannelModelPrice, len(prices))
	for _, p := range prices {
		cache[p.ModelID] = p
	}

	ch.cachedModelPrices = cache
	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "preloaded model prices", log.Int("channel_id", ch.ID), log.Int("count", len(cache)))
	}
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
