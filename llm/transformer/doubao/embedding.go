package doubao

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// multimodalEmbeddingRequest represents the Doubao multimodal embedding request format.
// Endpoint: POST /embeddings/multimodal
type multimodalEmbeddingRequest struct {
	Model          string                        `json:"model"`
	Input          []multimodalEmbeddingInputItem `json:"input"`
	EncodingFormat string                        `json:"encoding_format,omitempty"`
	Dimensions     *int                          `json:"dimensions,omitempty"`
}

// multimodalEmbeddingInputItem represents a single input item in the multimodal embedding request.
type multimodalEmbeddingInputItem struct {
	Type     string                          `json:"type"`
	Text     string                          `json:"text,omitempty"`
	ImageURL *multimodalEmbeddingImageURL     `json:"image_url,omitempty"`
}

type multimodalEmbeddingImageURL struct {
	URL string `json:"url"`
}

// transformEmbeddingRequest transforms unified llm.Request to Doubao multimodal embedding HTTP request.
func (t *OutboundTransformer) transformEmbeddingRequest(
	ctx context.Context,
	llmReq *llm.Request,
) (*httpclient.Request, error) {
	if llmReq.Embedding == nil {
		return nil, fmt.Errorf("embedding request is nil in llm.Request")
	}

	input := buildMultimodalInput(llmReq.Embedding.Input)

	req := multimodalEmbeddingRequest{
		Model:          llmReq.Model,
		Input:          input,
		EncodingFormat: llmReq.Embedding.EncodingFormat,
		Dimensions:     llmReq.Embedding.Dimensions,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding request: %w", err)
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	apiKey := t.APIKeyProvider.Get(ctx)

	auth := &httpclient.AuthConfig{
		Type:   "bearer",
		APIKey: apiKey,
	}

	url := t.BaseURL + "/embeddings/multimodal"

	return &httpclient.Request{
		Method:      http.MethodPost,
		URL:         url,
		Headers:     headers,
		Body:        body,
		Auth:        auth,
		RequestType: string(llm.RequestTypeEmbedding),
		APIFormat:   string(llm.APIFormatOpenAIEmbedding),
	}, nil
}

// multimodalEmbeddingResponse represents the Doubao multimodal embedding response format.
type multimodalEmbeddingResponse struct {
	ID     string                        `json:"id"`
	Object string                        `json:"object"`
	Data   multimodalEmbeddingData       `json:"data"`
	Model  string                        `json:"model"`
	Usage  multimodalEmbeddingUsage      `json:"usage"`
}

type multimodalEmbeddingData struct {
	Object         string        `json:"object"`
	Embedding      llm.Embedding `json:"embedding"`
	SparseEmbedding json.RawMessage `json:"sparse_embedding,omitempty"`
}

type multimodalEmbeddingUsage struct {
	PromptTokens        int64                                `json:"prompt_tokens"`
	TotalTokens         int64                                `json:"total_tokens"`
	PromptTokensDetails *multimodalEmbeddingPromptTokensDetails `json:"prompt_tokens_details,omitempty"`
}

type multimodalEmbeddingPromptTokensDetails struct {
	TextTokens  int64 `json:"text_tokens"`
	ImageTokens int64 `json:"image_tokens"`
}

// transformEmbeddingResponse transforms Doubao multimodal embedding HTTP response to unified llm.Response.
func (t *OutboundTransformer) transformEmbeddingResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	if httpResp.StatusCode >= 400 {
		_, err := t.Outbound.TransformResponse(ctx, httpResp)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("HTTP error %d", httpResp.StatusCode)
	}

	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	var embResp multimodalEmbeddingResponse
	if err := json.Unmarshal(httpResp.Body, &embResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal embedding response: %w", err)
	}

	llmResp := &llm.Response{
		RequestType: llm.RequestTypeEmbedding,
		APIFormat:   llm.APIFormatOpenAIEmbedding,
		Embedding: &llm.EmbeddingResponse{
			ID:     embResp.ID,
			Object: embResp.Object,
			Data: []llm.EmbeddingData{
				{
					Object:    embResp.Data.Object,
					Embedding: embResp.Data.Embedding,
					Index:     0,
				},
			},
		},
		Model: embResp.Model,
	}

	if embResp.Usage.PromptTokens > 0 || embResp.Usage.TotalTokens > 0 {
		llmResp.Usage = &llm.Usage{
			PromptTokens: embResp.Usage.PromptTokens,
			TotalTokens:  embResp.Usage.TotalTokens,
		}

		if embResp.Usage.PromptTokensDetails != nil {
			llmResp.Usage.PromptTokensDetails = &llm.PromptTokensDetails{
				TextTokens:  embResp.Usage.PromptTokensDetails.TextTokens,
				ImageTokens: embResp.Usage.PromptTokensDetails.ImageTokens,
			}
		}
	}

	return llmResp, nil
}

// buildMultimodalInput converts EmbeddingInput to doubao multimodal input format.
// Each text string becomes an input item with type "text".
func buildMultimodalInput(input llm.EmbeddingInput) []multimodalEmbeddingInputItem {
	var texts []string

	switch {
	case input.String != "":
		texts = []string{input.String}
	case len(input.StringArray) > 0:
		texts = input.StringArray
	}

	items := make([]multimodalEmbeddingInputItem, 0, len(texts))
	for _, text := range texts {
		items = append(items, multimodalEmbeddingInputItem{
			Type: "text",
			Text: text,
		})
	}

	return items
}
