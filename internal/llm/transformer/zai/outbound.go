package zai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/tracing"
)

// Config holds all configuration for the Zai outbound transformer.
type Config struct {
	// API configuration
	BaseURL string `json:"base_url,omitempty"` // Custom base URL (optional)
	APIKey  string `json:"api_key,omitempty"`  // API key
}

// OutboundTransformer implements transformer.Outbound for Zai format.
type OutboundTransformer struct {
	transformer.Outbound

	BaseURL string
	APIKey  string
}

// NewOutboundTransformer creates a new Zai OutboundTransformer with legacy parameters.
// Deprecated: Use NewOutboundTransformerWithConfig instead.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		BaseURL: baseURL,
		APIKey:  apiKey,
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new Zai OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	t, err := openai.NewOutboundTransformer(config.BaseURL, config.APIKey)
	if err != nil {
		return nil, fmt.Errorf("invalid Zai transformer configuration: %w", err)
	}

	return &OutboundTransformer{
		BaseURL:  config.BaseURL,
		APIKey:   config.APIKey,
		Outbound: t,
	}, nil
}

type Request struct {
	llm.Request

	UserID    string `json:"user_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// TransformRequest transforms ChatCompletionRequest to Request.
func (t *OutboundTransformer) TransformRequest(
	ctx context.Context,
	chatReq *llm.Request,
) (*httpclient.Request, error) {
	// Create Zai-specific request by removing Metadata and adding request_id/user_id
	zaiReq := Request{
		Request:   *chatReq,
		UserID:    "",
		RequestID: "",
	}

	if chatReq.Metadata != nil {
		zaiReq.UserID = chatReq.Metadata["user_id"]
		zaiReq.RequestID = chatReq.Metadata["request_id"]
	}

	if zaiReq.RequestID == "" {
		traceID, _ := tracing.GetTraceID(ctx)
		zaiReq.RequestID = string(traceID)
	}

	// zai only support auto tool choice.
	if zaiReq.ToolChoice != nil {
		zaiReq.ToolChoice = &llm.ToolChoice{
			ToolChoice: lo.ToPtr("auto"),
		}
	}

	// zai request does not support metadata.
	zaiReq.Metadata = nil

	body, err := json.Marshal(zaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request: %w", err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	auth := &httpclient.AuthConfig{
		Type:   "bearer",
		APIKey: t.APIKey,
	}

	baseURL := strings.TrimSuffix(t.BaseURL, "/")
	url := baseURL + "/chat/completions"

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth:    auth,
	}, nil
}
