package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

type EmbeddingRequest struct {
	Input          llm.EmbeddingInput `json:"input"`
	Model          string             `json:"model"`
	EncodingFormat string             `json:"encoding_format,omitempty"`
	Dimensions     *int               `json:"dimensions,omitempty"`
	User           string             `json:"user,omitempty"`
}

type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  EmbeddingUsage  `json:"usage"`
}

type EmbeddingData struct {
	Object    string        `json:"object"`
	Embedding llm.Embedding `json:"embedding"`
	Index     int           `json:"index"`
}

type EmbeddingUsage struct {
	PromptTokens int64 `json:"prompt_tokens"`
	TotalTokens  int64 `json:"total_tokens"`
}

// transformEmbeddingRequest transforms unified llm.Request to HTTP embedding request.
func (t *OutboundTransformer) transformEmbeddingRequest(
	_ context.Context,
	llmReq *llm.Request,
) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("llm request is nil")
	}

	if llmReq.Embedding == nil {
		return nil, fmt.Errorf("embedding request is nil in llm.Request")
	}

	embReq := EmbeddingRequest{
		Input:          llmReq.Embedding.Input,
		Model:          llmReq.Model,
		EncodingFormat: llmReq.Embedding.EncodingFormat,
		Dimensions:     llmReq.Embedding.Dimensions,
		User:           llmReq.Embedding.User,
	}

	// Re-marshal to JSON (ensure clean output)
	body, err := json.Marshal(embReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding request: %w", err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	// Build URL, reuse same logic as chat
	url := t.buildEmbeddingURL()

	// Build auth config
	var auth *httpclient.AuthConfig

	//nolint:exhaustive // Checked.
	switch t.config.Type {
	case PlatformAzure:
		auth = &httpclient.AuthConfig{
			Type:      "api_key",
			APIKey:    t.config.APIKey,
			HeaderKey: "Api-Key",
		}
	default:
		auth = &httpclient.AuthConfig{
			Type:   "bearer",
			APIKey: t.config.APIKey,
		}
	}

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth:    auth,
	}

	// Set metadata for response routing
	if httpReq.TransformerMetadata == nil {
		httpReq.TransformerMetadata = make(map[string]any)
	}

	httpReq.TransformerMetadata["outbound_format_type"] = llm.APIFormatOpenAIEmbedding.String()

	return httpReq, nil
}

// buildEmbeddingURL constructs the embedding API URL.
func (t *OutboundTransformer) buildEmbeddingURL() string {
	//nolint:exhaustive // Checked.
	switch t.config.Type {
	case PlatformAzure:
		if strings.HasSuffix(t.config.BaseURL, "/openai/v1") {
			// Azure URL already includes /openai/v1
			return fmt.Sprintf("%s/embeddings?api-version=%s",
				t.config.BaseURL, t.config.APIVersion)
		}

		if strings.HasSuffix(t.config.BaseURL, "/openai") {
			// Azure URL includes /openai but not /v1
			return fmt.Sprintf("%s/v1/embeddings?api-version=%s",
				t.config.BaseURL, t.config.APIVersion)
		}
		// Default case for other Azure URLs
		return fmt.Sprintf("%s/openai/v1/embeddings?api-version=%s",
			t.config.BaseURL, t.config.APIVersion)
	default:
		// RawURL is true, use the base URL as is
		if t.config.RawURL {
			return t.config.BaseURL + "/embeddings"
		}
		// Standard OpenAI API
		// Check if URL already contains /v1/ in the path (e.g., https://api.deepinfra.com/v1/openai)
		if strings.Contains(t.config.BaseURL, "/v1/") {
			return t.config.BaseURL + "/embeddings"
		}

		if strings.HasSuffix(t.config.BaseURL, "/v1") {
			return t.config.BaseURL + "/embeddings"
		}

		return t.config.BaseURL + "/v1/embeddings"
	}
}

// transformEmbeddingResponse transforms HTTP embedding response to unified llm.Response.
func (t *OutboundTransformer) transformEmbeddingResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	// Check HTTP status codes, 4xx/5xx should return standard format error
	// Note: httpclient usually already returns *httpclient.Error for 4xx/5xx,
	// this is defensive code to ensure error format conforms to OpenAI spec
	if httpResp.StatusCode >= 400 {
		return nil, t.TransformError(ctx, &httpclient.Error{
			StatusCode: httpResp.StatusCode,
			Body:       httpResp.Body,
		})
	}

	// Check for empty response body
	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	// Parse OpenAI embedding response
	var embResp EmbeddingResponse
	if err := json.Unmarshal(httpResp.Body, &embResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal embedding response: %w", err)
	}

	// Convert OpenAI EmbeddingData to llm.EmbeddingData
	llmEmbeddingData := make([]llm.EmbeddingData, len(embResp.Data))
	for i, data := range embResp.Data {
		llmEmbeddingData[i] = llm.EmbeddingData{
			Object:    data.Object,
			Embedding: data.Embedding,
			Index:     data.Index,
		}
	}

	// Build unified embedding response
	var usage *llm.EmbeddingUsage
	if embResp.Usage.PromptTokens > 0 || embResp.Usage.TotalTokens > 0 {
		usage = &llm.EmbeddingUsage{
			PromptTokens: embResp.Usage.PromptTokens,
			TotalTokens:  embResp.Usage.TotalTokens,
		}
	}

	llmEmbeddingResp := &llm.EmbeddingResponse{
		Object: embResp.Object,
		Data:   llmEmbeddingData,
		Usage:  usage,
	}

	llmResp := &llm.Response{
		RequestType: llm.RequestTypeEmbedding,
		APIFormat:   llm.APIFormatOpenAIEmbedding,
		Embedding:   llmEmbeddingResp,
		Model:       embResp.Model,
	}

	return llmResp, nil
}
