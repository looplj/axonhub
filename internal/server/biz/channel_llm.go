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
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/llm/transformer/anthropic"
	"github.com/looplj/axonhub/internal/llm/transformer/doubao"
	"github.com/looplj/axonhub/internal/llm/transformer/gemini"
	"github.com/looplj/axonhub/internal/llm/transformer/modelscope"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/llm/transformer/openrouter"
	"github.com/looplj/axonhub/internal/llm/transformer/xai"
	"github.com/looplj/axonhub/internal/llm/transformer/zai"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

func (c Channel) resolvePrefixedModel(model string) (string, bool) {
	if c.Settings == nil || c.Settings.ExtraModelPrefix == "" {
		return "", false
	}

	prefix := c.Settings.ExtraModelPrefix + "/"
	if !strings.HasPrefix(model, prefix) {
		return "", false
	}

	modelWithoutPrefix := model[len(prefix):]
	if !slices.Contains(c.SupportedModels, modelWithoutPrefix) {
		return "", false
	}

	return modelWithoutPrefix, true
}

func (c Channel) IsModelSupported(model string) bool {
	if slices.Contains(c.SupportedModels, model) {
		return true
	}

	if c.Settings == nil {
		return false
	}

	if _, ok := c.resolvePrefixedModel(model); ok {
		return true
	}

	for _, mapping := range c.Settings.ModelMappings {
		if mapping.From == model && slices.Contains(c.SupportedModels, mapping.To) {
			return true
		}
	}

	return false
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

func (c Channel) ChooseModel(model string) (string, error) {
	if slices.Contains(c.SupportedModels, model) {
		return model, nil
	}

	if c.Settings == nil {
		return "", fmt.Errorf("model %s not supported in channel %s", model, c.Name)
	}

	if resolved, ok := c.resolvePrefixedModel(model); ok {
		return resolved, nil
	}

	for _, mapping := range c.Settings.ModelMappings {
		if mapping.From == model && slices.Contains(c.SupportedModels, mapping.To) {
			return mapping.To, nil
		}
	}

	return "", fmt.Errorf("model %s not supported in channel %s", model, c.Name)
}

// GetOverrideParameters returns the cached override parameters for the channel.
// If the parameters haven't been parsed yet, it parses and caches them.
func (c *Channel) GetOverrideParameters() map[string]any {
	if c.CachedOverrideParams != nil {
		return c.CachedOverrideParams
	}

	if c.Settings == nil || c.Settings.OverrideParameters == "" {
		c.CachedOverrideParams = make(map[string]any)
		return c.CachedOverrideParams
	}

	var overrideParams map[string]any
	if err := json.Unmarshal([]byte(c.Settings.OverrideParameters), &overrideParams); err != nil {
		// If parsing fails, return empty map and log the error
		log.Warn(context.Background(), "failed to parse override parameters",
			log.String("channel", c.Name),
			log.Cause(err),
		)
		c.CachedOverrideParams = make(map[string]any)

		return c.CachedOverrideParams
	}

	c.CachedOverrideParams = overrideParams

	return c.CachedOverrideParams
}

// GetOverrideHeaders returns the cached override headers for the channel.
// If the headers haven't been loaded yet, it loads and caches them.
func (c *Channel) GetOverrideHeaders() []objects.HeaderEntry {
	if c.CachedOverrideHeaders != nil {
		return c.CachedOverrideHeaders
	}

	if c.Settings == nil || len(c.Settings.OverrideHeaders) == 0 {
		c.CachedOverrideHeaders = make([]objects.HeaderEntry, 0)
		return c.CachedOverrideHeaders
	}

	c.CachedOverrideHeaders = c.Settings.OverrideHeaders

	return c.CachedOverrideHeaders
}

// getProxyConfig extracts proxy configuration from channel settings
// Returns nil if no proxy configuration is set (backward compatibility).
func getProxyConfig(channelSettings *objects.ChannelSettings) *objects.ProxyConfig {
	if channelSettings == nil || channelSettings.Proxy == nil {
		// Backward compatibility: default to environment proxy type
		return &objects.ProxyConfig{
			Type: objects.ProxyTypeEnvironment,
		}
	}

	return channelSettings.Proxy
}

//nolint:maintidx // Simple switch statement.
func (svc *ChannelService) buildChannel(c *ent.Channel) (*Channel, error) {
	httpClient := httpclient.NewHttpClientWithProxy(getProxyConfig(c.Settings))

	//nolint:exhaustive // TODO SUPPORT more providers.
	switch c.Type {
	case channel.TypeDoubao:
		transformer, err := doubao.NewOutboundTransformerWithConfig(&doubao.Config{
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeOpenrouter:
		transformer, err := openrouter.NewOutboundTransformer(c.BaseURL, c.Credentials.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeZai, channel.TypeZhipu:
		transformer, err := zai.NewOutboundTransformer(c.BaseURL, c.Credentials.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeXai:
		transformer, err := xai.NewOutboundTransformer(c.BaseURL, c.Credentials.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeAnthropic, channel.TypeLongcatAnthropic, channel.TypeMinimaxAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:    anthropic.PlatformDirect,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeDeepseekAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:    anthropic.PlatformDeepSeek,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeDoubaoAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:    anthropic.PlatformDoubao,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeMoonshotAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:    anthropic.PlatformMoonshot,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeZhipuAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:    anthropic.PlatformZhipu,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeZaiAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:    anthropic.PlatformZai,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil

	case channel.TypeAnthropicAWS:
		// For anthropic_aws, we need to create a transformer with AWS credentials
		// The transformer will handle AWS Bedrock integration
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:            anthropic.PlatformBedrock,
			Region:          c.Credentials.AWS.Region,
			AccessKeyID:     c.Credentials.AWS.AccessKeyID,
			SecretAccessKey: c.Credentials.AWS.SecretAccessKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
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

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
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

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeOpenai,
		channel.TypeDeepseek, channel.TypeMoonshot, channel.TypeLongcat, channel.TypeMinimax,
		channel.TypeGeminiOpenai,
		channel.TypePpio, channel.TypeSiliconflow, channel.TypeVolcengine,
		channel.TypeVercel, channel.TypeAihubmix, channel.TypeBurncloud, channel.TypeBailian:
		transformer, err := openai.NewOutboundTransformerWithConfig(&openai.Config{
			Type:    openai.PlatformOpenAI,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeGemini:
		transformer, err := gemini.NewOutboundTransformerWithConfig(gemini.Config{
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	default:
		return nil, errors.New("unknown channel type")
	}
}

func (svc *ChannelService) ChooseChannels(
	ctx context.Context,
	chatReq *llm.Request,
) ([]*Channel, error) {
	var channels []*Channel

	for _, channel := range svc.EnabledChannels {
		if channel.IsModelSupported(chatReq.Model) {
			channels = append(channels, channel)
		}
	}

	return channels, nil
}
