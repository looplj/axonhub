package opencode

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/deepseek"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

// route identifies which upstream protocol an OpenCode Go model family speaks.
// OpenCode Go exposes different model families through different endpoints
// (see https://opencode.ai/docs/go), so the outbound transformer dispatches
// by model name.
type route string

const (
	// routeChat is the default OpenAI chat completions endpoint (/v1/chat/completions).
	routeChat route = "chat"
	// routeDeepseek routes DeepSeek models through the DeepSeek transformer,
	// which handles its thinking/reasoning_content conventions.
	routeDeepseek route = "deepseek"
	// routeResponses routes Grok/GPT models through the OpenAI Responses API (/v1/responses).
	routeResponses route = "responses"
	// routeAnthropic routes MiniMax/Qwen models through the Anthropic messages endpoint (/v1/messages).
	routeAnthropic route = "anthropic"
)

// routeMetadataKey carries the selected route on the outbound request so the
// response/stream transformers can dispatch back to the same sub-transformer.
const routeMetadataKey = "opencode_route"

// Config holds all configuration for the OpenCode Go outbound transformer.
type Config struct {
	// BaseURL is the base URL for the OpenCode Go API, required.
	BaseURL string `json:"base_url,omitempty"`

	// APIKeyProvider provides API keys for authentication, required.
	APIKeyProvider auth.APIKeyProvider `json:"-"`
}

// OutboundTransformer implements transformer.Outbound for OpenCode Go.
// It embeds the default OpenAI chat outbound and dispatches to protocol-specific
// sub-transformers based on the requested model.
type OutboundTransformer struct {
	transformer.Outbound // default: OpenAI chat completions

	deepseek  transformer.Outbound
	responses transformer.Outbound
	anthropic transformer.Outbound
}

// sessionHeaderOutbound adds the OpenCode Go session header to a protocol-specific
// outbound transformer. It is used by the legacy Anthropic-only channel.
type sessionHeaderOutbound struct {
	transformer.Outbound
}

// WithSessionHeader decorates an outbound transformer with OpenCode Go session affinity.
func WithSessionHeader(outbound transformer.Outbound) transformer.Outbound {
	if outbound == nil {
		return nil
	}

	return &sessionHeaderOutbound{Outbound: outbound}
}

// NewOutboundTransformer creates a new OpenCode Go OutboundTransformer with legacy parameters.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	return NewOutboundTransformerWithConfig(&Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(apiKey),
	})
}

// NewOutboundTransformerWithConfig creates a new OpenCode Go OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}

	if config.APIKeyProvider == nil {
		return nil, fmt.Errorf("API key provider is required")
	}

	if config.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	chatT, err := openai.NewOutboundTransformerWithConfig(&openai.Config{
		PlatformType:   openai.PlatformOpenAI,
		BaseURL:        config.BaseURL,
		APIKeyProvider: config.APIKeyProvider,
	})
	if err != nil {
		return nil, fmt.Errorf("invalid OpenCode chat transformer configuration: %w", err)
	}

	deepseekT, err := deepseek.NewOutboundTransformerWithConfig(&deepseek.Config{
		BaseURL:        config.BaseURL,
		APIKeyProvider: config.APIKeyProvider,
	})
	if err != nil {
		return nil, fmt.Errorf("invalid OpenCode deepseek transformer configuration: %w", err)
	}

	responsesT, err := responses.NewOutboundTransformerWithConfig(&responses.Config{
		BaseURL:        config.BaseURL,
		APIKeyProvider: config.APIKeyProvider,
	})
	if err != nil {
		return nil, fmt.Errorf("invalid OpenCode responses transformer configuration: %w", err)
	}

	anthropicT, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
		Type:           anthropic.PlatformDirect,
		BaseURL:        config.BaseURL,
		APIKeyProvider: config.APIKeyProvider,
	})
	if err != nil {
		return nil, fmt.Errorf("invalid OpenCode anthropic transformer configuration: %w", err)
	}

	return &OutboundTransformer{
		Outbound:  chatT,
		deepseek:  deepseekT,
		responses: responsesT,
		anthropic: anthropicT,
	}, nil
}

// routeForModel returns the sub-transformer route for an OpenCode Go model ID.
func routeForModel(model string) route {
	switch {
	case strings.HasPrefix(model, "deepseek"):
		return routeDeepseek
	case strings.HasPrefix(model, "grok"), strings.HasPrefix(model, "gpt"):
		return routeResponses
	case strings.HasPrefix(model, "minimax"), strings.HasPrefix(model, "qwen3"):
		return routeAnthropic
	default:
		return routeChat
	}
}

func (t *OutboundTransformer) sub(r route) transformer.Outbound {
	switch r {
	case routeDeepseek:
		return t.deepseek
	case routeResponses:
		return t.responses
	case routeAnthropic:
		return t.anthropic
	default:
		return t.Outbound
	}
}

func routeFromResponse(httpResp *httpclient.Response) route {
	if httpResp != nil && httpResp.Request != nil && httpResp.Request.TransformerMetadata != nil {
		if r, ok := httpResp.Request.TransformerMetadata[routeMetadataKey].(string); ok && r != "" {
			return route(r)
		}
	}

	return routeChat
}

func routeFromHTTPRequest(req *httpclient.Request) route {
	if req != nil && req.TransformerMetadata != nil {
		if r, ok := req.TransformerMetadata[routeMetadataKey].(string); ok && r != "" {
			return route(r)
		}
	}

	return routeChat
}

// TransformRequest dispatches to the sub-transformer for the requested model and
// records the chosen route on the outbound request for response routing.
func (t *OutboundTransformer) TransformRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("request is nil")
	}

	r := routeForModel(llmReq.Model)
	httpReq, err := t.sub(r).TransformRequest(ctx, llmReq)
	if err != nil {
		return nil, err
	}

	if httpReq.TransformerMetadata == nil {
		httpReq.TransformerMetadata = map[string]any{}
	}
	httpReq.TransformerMetadata[routeMetadataKey] = string(r)
	setSessionHeader(ctx, llmReq, httpReq)

	return httpReq, nil
}

func (t *sessionHeaderOutbound) TransformRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	httpReq, err := t.Outbound.TransformRequest(ctx, llmReq)
	if err != nil {
		return nil, err
	}

	setSessionHeader(ctx, llmReq, httpReq)

	return httpReq, nil
}

func setSessionHeader(ctx context.Context, llmReq *llm.Request, httpReq *httpclient.Request) {
	if httpReq == nil {
		return
	}

	if httpReq.Headers == nil {
		httpReq.Headers = make(http.Header)
	}

	httpReq.Headers.Set(SessionHeader, resolveSessionID(ctx, llmReq))
}

func resolveSessionID(ctx context.Context, llmReq *llm.Request) string {
	if llmReq != nil && llmReq.RawRequest != nil {
		if sessionID := GetSessionIDFromHeaders(llmReq.RawRequest.Headers); sessionID != "" {
			return sessionID
		}
	}

	if sessionID, ok := shared.GetSessionID(ctx); ok {
		if sessionID = strings.TrimSpace(sessionID); sessionID != "" {
			return sessionID
		}
	}

	return uuid.NewString()
}

// TransformResponse dispatches to the sub-transformer recorded on the request.
func (t *OutboundTransformer) TransformResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	return t.sub(routeFromResponse(httpResp)).TransformResponse(ctx, httpResp)
}

// TransformStream dispatches to the sub-transformer recorded on the request.
func (t *OutboundTransformer) TransformStream(ctx context.Context, req *httpclient.Request, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return t.sub(routeFromHTTPRequest(req)).TransformStream(ctx, req, stream)
}

// AggregateStreamChunks dispatches to the sub-transformer recorded on the request.
func (t *OutboundTransformer) AggregateStreamChunks(ctx context.Context, req *httpclient.Request, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return t.sub(routeFromHTTPRequest(req)).AggregateStreamChunks(ctx, req, chunks)
}

// TransformError delegates to the default OpenAI chat error handling.
func (t *OutboundTransformer) TransformError(ctx context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	return t.Outbound.TransformError(ctx, rawErr)
}
