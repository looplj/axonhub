package biz

import (
	"context"
	"encoding/json"
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
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/bailian"
	"github.com/looplj/axonhub/llm/transformer/deepseek"
	"github.com/looplj/axonhub/llm/transformer/doubao"
	"github.com/looplj/axonhub/llm/transformer/gemini"
	geminioai "github.com/looplj/axonhub/llm/transformer/gemini/openai"
	"github.com/looplj/axonhub/llm/transformer/jina"
	"github.com/looplj/axonhub/llm/transformer/longcat"
	"github.com/looplj/axonhub/llm/transformer/modelscope"
	"github.com/looplj/axonhub/llm/transformer/moonshot"
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

// GetOverrideParameters returns the cached override parameters for the channel.
// If the parameters haven't been parsed yet, it parses and caches them.
//
// WARNING: The returned map is internal cached state.
// DO NOT modify the returned map or its contents.
// Modifications will not persist and may cause data inconsistency.
func (c *Channel) GetOverrideParameters() map[string]any {
	if c.cachedOverrideParams != nil {
		return c.cachedOverrideParams
	}

	if c.Settings == nil || c.Settings.OverrideParameters == "" {
		c.cachedOverrideParams = make(map[string]any)
		return c.cachedOverrideParams
	}

	var overrideParams map[string]any
	if err := json.Unmarshal([]byte(c.Settings.OverrideParameters), &overrideParams); err != nil {
		// If parsing fails, return empty map and log the error
		log.Warn(context.Background(), "failed to parse override parameters",
			log.String("channel", c.Name),
			log.Cause(err),
		)
		c.cachedOverrideParams = make(map[string]any)

		return c.cachedOverrideParams
	}

	c.cachedOverrideParams = overrideParams

	return c.cachedOverrideParams
}

// GetOverrideHeaders returns the cached override headers for the channel.
// If the headers haven't been loaded yet, it loads and caches them.
//
// WARNING: The returned slice is internal cached state.
// DO NOT modify the returned slice or its elements.
// Modifications will not persist and may cause data inconsistency.
func (c *Channel) GetOverrideHeaders() []objects.HeaderEntry {
	if c.cachedOverrideHeaders != nil {
		return c.cachedOverrideHeaders
	}

	if c.Settings == nil || len(c.Settings.OverrideHeaders) == 0 {
		c.cachedOverrideHeaders = make([]objects.HeaderEntry, 0)
		return c.cachedOverrideHeaders
	}

	c.cachedOverrideHeaders = c.Settings.OverrideHeaders

	return c.cachedOverrideHeaders
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

// buildChannelWithTransformer is a helper function to build a Channel with the given transformer.
func buildChannelWithTransformer(
	c *ent.Channel,
	transformer transformer.Outbound,
	httpClient *httpclient.HttpClient,
) *Channel {
	ch := &Channel{
		Channel:    c,
		Outbound:   transformer,
		HTTPClient: httpClient,
	}
	entries := ch.GetModelEntries()

	headers := ch.GetOverrideHeaders()

	params := ch.GetOverrideParameters()
	if log.DebugEnabled(context.Background()) {
		log.Debug(context.Background(), "pre cached settings",
			log.String("channel", ch.Name), log.Int("entries", len(entries)),
			log.Int("headers", len(headers)),
			log.Int("params", len(params)),
		)
	}

	return ch
}

//nolint:maintidx // Simple switch statement.
func (svc *ChannelService) buildChannel(c *ent.Channel) (*Channel, error) {
	httpClient := httpclient.NewHttpClientWithProxy(getProxyConfig(c.Settings))

	//nolint:exhaustive // TODO SUPPORT more providers.
	buildCodexTransformer := func() (*Channel, error) {
		transformer, err := codex.NewOutboundTransformer(codex.Params{
			CredentialsJSON: c.Credentials.APIKey,
			CacheConfig:     svc.SystemService.CacheConfig,
			ChannelID:       c.ID,
			HTTPClient:      httpClient,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create codex outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	}

	switch c.Type {
	case channel.TypeDoubao, channel.TypeVolcengine:
		transformer, err := doubao.NewOutboundTransformerWithConfig(&doubao.Config{
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	case channel.TypeOpenrouter:
		transformer, err := openrouter.NewOutboundTransformer(c.BaseURL, c.Credentials.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	case channel.TypeZai, channel.TypeZhipu:
		transformer, err := zai.NewOutboundTransformer(c.BaseURL, c.Credentials.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	case channel.TypeDeepseek:
		transformer, err := deepseek.NewOutboundTransformer(c.BaseURL, c.Credentials.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	case channel.TypeMoonshot:
		transformer, err := moonshot.NewOutboundTransformer(c.BaseURL, c.Credentials.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil

	case channel.TypeXai:
		transformer, err := xai.NewOutboundTransformer(c.BaseURL, c.Credentials.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	case channel.TypeLongcatAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:    anthropic.PlatformLongCat,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	case channel.TypeAnthropic, channel.TypeMinimaxAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:    anthropic.PlatformDirect,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	case channel.TypeClaudecode:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:    anthropic.PlatformClaudeCode,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	case channel.TypeDeepseekAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:    anthropic.PlatformDeepSeek,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	case channel.TypeDoubaoAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:    anthropic.PlatformDoubao,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	case channel.TypeMoonshotAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:    anthropic.PlatformMoonshot,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	case channel.TypeZhipuAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:    anthropic.PlatformZhipu,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	case channel.TypeZaiAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:    anthropic.PlatformZai,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil

	case channel.TypeAnthropicAWS:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:    anthropic.PlatformBedrock,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
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

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	case channel.TypeAnthropicFake:
		// For anthropic_fake, we use the fake transformer for testing
		fakeTransformer := anthropic.NewFakeTransformer()

		return &Channel{
			Channel:  c,
			Outbound: fakeTransformer,
		}, nil
	case channel.TypeOpenaiFake:
		fakeTransformer := openai.NewFakeTransformer()

		return &Channel{
			Channel:  c,
			Outbound: fakeTransformer,
		}, nil
	case channel.TypeModelscope:
		transformer, err := modelscope.NewOutboundTransformerWithConfig(&modelscope.Config{
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	case channel.TypeGeminiOpenai:
		transformer, err := geminioai.NewOutboundTransformerWithConfig(&geminioai.Config{
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	case channel.TypeLongcat:
		transformer, err := longcat.NewOutboundTransformer(c.BaseURL, c.Credentials.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	case channel.TypeBailian:
		transformer, err := bailian.NewOutboundTransformerWithConfig(&bailian.Config{
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	case channel.TypeCodex:
		return buildCodexTransformer()
	case channel.TypeOpenai, channel.TypeDeepinfra, channel.TypeMinimax,
		channel.TypePpio, channel.TypeSiliconflow,
		channel.TypeVercel, channel.TypeAihubmix, channel.TypeBurncloud, channel.TypeGithub:
		transformer, err := openai.NewOutboundTransformerWithConfig(&openai.Config{
			PlatformType: openai.PlatformOpenAI,
			BaseURL:      c.BaseURL,
			APIKey:       c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	case channel.TypeOpenaiResponses:
		transformer, err := responses.NewOutboundTransformerWithConfig(&responses.Config{
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	case channel.TypeGemini:
		transformer, err := gemini.NewOutboundTransformerWithConfig(gemini.Config{
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	case channel.TypeGeminiVertex:
		transformer, err := gemini.NewOutboundTransformerWithConfig(gemini.Config{
			BaseURL:      c.BaseURL,
			APIKey:       c.Credentials.APIKey,
			PlatformType: gemini.PlatformVertex,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	case channel.TypeJina:
		transformer, err := jina.NewOutboundTransformer(c.BaseURL, c.Credentials.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return buildChannelWithTransformer(c, transformer, httpClient), nil
	default:
		return nil, errors.New("unknown channel type")
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

	cache := make(map[string]objects.ModelPrice, len(prices))
	for _, p := range prices {
		cache[p.ModelID] = p.Price
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
