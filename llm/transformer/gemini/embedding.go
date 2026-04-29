package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// EmbedContentConfig holds optional parameters for the EmbedContent method.
type EmbedContentConfig struct {
	// Type of task for which the embedding will be used.
	TaskType string `json:"taskType,omitempty"`
	// Title for the text. Only applicable when TaskType is RETRIEVAL_DOCUMENT.
	Title string `json:"title,omitempty"`
	// Reduced dimension for the output embedding.
	OutputDimensionality *int32 `json:"outputDimensionality,omitempty"`
}

// ContentEmbedding is the embedding generated from an input content.
type ContentEmbedding struct {
	// A list of floats representing an embedding.
	Values []float32 `json:"values,omitempty"`
}

// EmbedContentRequest is the Gemini embedContent request body.
type EmbedContentRequest struct {
	Model                string   `json:"model,omitempty"`
	Content              *Content `json:"content"`
	TaskType             string   `json:"taskType,omitempty"`
	Title                string   `json:"title,omitempty"`
	OutputDimensionality *int32   `json:"outputDimensionality,omitempty"`
}

// EmbedContentResponse is the response for single embedContent.
type EmbedContentResponse struct {
	Embedding *ContentEmbedding `json:"embedding,omitempty"`
}

// BatchEmbedContentsRequest is the batchEmbedContents request body.
type BatchEmbedContentsRequest struct {
	Requests []*EmbedContentRequest `json:"requests"`
}

// BatchEmbedContentsResponse is the response for batchEmbedContents.
type BatchEmbedContentsResponse struct {
	Embeddings []*ContentEmbedding `json:"embeddings,omitempty"`
}

// transformEmbeddingRequest transforms unified llm.Request to Gemini embedding HTTP request.
func (t *OutboundTransformer) transformEmbeddingRequest(
	ctx context.Context,
	llmReq *llm.Request,
) (*httpclient.Request, error) {
	if llmReq.Embedding == nil {
		return nil, fmt.Errorf("embedding request is nil in llm.Request")
	}

	if llmReq.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	// Collect all input texts
	texts := embeddingInputToTexts(llmReq.Embedding.Input)
	if len(texts) == 0 {
		return nil, fmt.Errorf("embedding input is empty")
	}

	// Map task from llm.EmbeddingRequest to Gemini TaskType
	taskType := mapEmbeddingTaskType(llmReq.Embedding.Task)

	// Map dimensions
	var outputDim *int32

	if llmReq.Embedding.Dimensions != nil {
		d := int32(*llmReq.Embedding.Dimensions)
		outputDim = &d
	}

	modelRef := "models/" + llmReq.Model

	var (
		body []byte
		url  string
		err  error
	)

	if len(texts) == 1 {
		// Single text: use embedContent
		req := &EmbedContentRequest{
			Model: modelRef,
			Content: &Content{
				Parts: []*Part{{Text: texts[0]}},
			},
			TaskType:             taskType,
			OutputDimensionality: outputDim,
		}

		body, err = json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal embed content request: %w", err)
		}

		url = t.buildEmbeddingURL(llmReq.Model, false)
	} else {
		// Multiple texts: use batchEmbedContents
		requests := make([]*EmbedContentRequest, len(texts))
		for i, text := range texts {
			requests[i] = &EmbedContentRequest{
				Model: modelRef,
				Content: &Content{
					Parts: []*Part{{Text: text}},
				},
				TaskType:             taskType,
				OutputDimensionality: outputDim,
			}
		}

		batchReq := &BatchEmbedContentsRequest{
			Requests: requests,
		}

		body, err = json.Marshal(batchReq)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal batch embed content request: %w", err)
		}

		url = t.buildEmbeddingURL(llmReq.Model, true)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	// Prepare authentication
	var authConfig *httpclient.AuthConfig

	apiKey := t.config.APIKeyProvider.Get(ctx)
	authConfig = &httpclient.AuthConfig{
		Type:      "api_key",
		APIKey:    apiKey,
		HeaderKey: "x-goog-api-key",
	}

	httpReq := &httpclient.Request{
		Method:                http.MethodPost,
		URL:                   url,
		Headers:               headers,
		Body:                  body,
		Auth:                  authConfig,
		RequestType:           string(llm.RequestTypeEmbedding),
		APIFormat:             string(llm.APIFormatGeminiEmbedding),
		SkipInboundQueryMerge: true,
	}

	return httpReq, nil
}

// buildEmbeddingURL constructs the embedding API URL.
func (t *OutboundTransformer) buildEmbeddingURL(model string, isBatch bool) string {
	version := t.config.APIVersion
	if version == "" {
		version = DefaultAPIVersion
	}

	action := "embedContent"
	if isBatch {
		action = "batchEmbedContents"
	}

	// For Vertex AI platform
	if t.config.PlatformType == PlatformVertex {
		baseURL := strings.TrimSuffix(t.config.BaseURL, "/")
		if strings.Contains(baseURL, "/v1/") {
			return fmt.Sprintf("%s/publishers/google/models/%s:%s", baseURL, model, action)
		}

		return fmt.Sprintf("%s/v1/publishers/google/models/%s:%s", baseURL, model, action)
	}

	return fmt.Sprintf("%s/%s/models/%s:%s", t.config.BaseURL, version, model, action)
}

// transformEmbeddingResponse transforms Gemini embedding HTTP response to unified llm.Response.
func (t *OutboundTransformer) transformEmbeddingResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	if httpResp.StatusCode >= 400 {
		return nil, t.TransformError(ctx, &httpclient.Error{
			StatusCode: httpResp.StatusCode,
			Body:       httpResp.Body,
		})
	}

	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	// Try parsing as batch response first (has "embeddings" array)
	var batchResp BatchEmbedContentsResponse
	if err := json.Unmarshal(httpResp.Body, &batchResp); err == nil && len(batchResp.Embeddings) > 0 {
		return convertBatchEmbeddingResponse(&batchResp), nil
	}

	// Parse as single embedContent response (has "embedding" object)
	var singleResp EmbedContentResponse
	if err := json.Unmarshal(httpResp.Body, &singleResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal embed content response: %w", err)
	}

	return convertSingleEmbeddingResponse(&singleResp), nil
}

// convertSingleEmbeddingResponse converts a single EmbedContentResponse to unified llm.Response.
func convertSingleEmbeddingResponse(resp *EmbedContentResponse) *llm.Response {
	var data []llm.EmbeddingData
	if resp.Embedding != nil {
		data = []llm.EmbeddingData{
			{
				Object:    "embedding",
				Embedding: llm.Embedding{Embedding: float32sToFloat64s(resp.Embedding.Values)},
				Index:     0,
			},
		}
	}

	return &llm.Response{
		RequestType: llm.RequestTypeEmbedding,
		APIFormat:   llm.APIFormatGeminiEmbedding,
		Embedding: &llm.EmbeddingResponse{
			Object: "list",
			Data:   data,
		},
	}
}

// convertBatchEmbeddingResponse converts BatchEmbedContentsResponse to unified llm.Response.
func convertBatchEmbeddingResponse(resp *BatchEmbedContentsResponse) *llm.Response {
	data := make([]llm.EmbeddingData, len(resp.Embeddings))
	for i, emb := range resp.Embeddings {
		data[i] = llm.EmbeddingData{
			Object:    "embedding",
			Embedding: llm.Embedding{Embedding: float32sToFloat64s(emb.Values)},
			Index:     i,
		}
	}

	return &llm.Response{
		RequestType: llm.RequestTypeEmbedding,
		APIFormat:   llm.APIFormatGeminiEmbedding,
		Embedding: &llm.EmbeddingResponse{
			Object: "list",
			Data:   data,
		},
	}
}

// embeddingInputToTexts extracts text strings from EmbeddingInput.
func embeddingInputToTexts(input llm.EmbeddingInput) []string {
	if input.String != "" {
		return []string{input.String}
	}

	if len(input.StringArray) > 0 {
		return input.StringArray
	}

	// Token arrays (IntArray, IntArrayArray) are not supported by Gemini embedding API.
	return nil
}

// mapEmbeddingTaskType maps unified task type to Gemini TaskType enum.
func mapEmbeddingTaskType(task string) string {
	switch strings.ToLower(task) {
	case "retrieval.query", "retrieval_query":
		return "RETRIEVAL_QUERY"
	case "retrieval.passage", "retrieval_document":
		return "RETRIEVAL_DOCUMENT"
	case "semantic_similarity", "text-matching":
		return "SEMANTIC_SIMILARITY"
	case "classification":
		return "CLASSIFICATION"
	case "clustering":
		return "CLUSTERING"
	case "question_answering":
		return "QUESTION_ANSWERING"
	case "fact_verification":
		return "FACT_VERIFICATION"
	case "code_retrieval_query":
		return "CODE_RETRIEVAL_QUERY"
	default:
		return ""
	}
}

// float32sToFloat64s converts a []float32 to []float64.
func float32sToFloat64s(f32s []float32) []float64 {
	f64s := make([]float64, len(f32s))
	for i, v := range f32s {
		f64s[i] = float64(v)
	}

	return f64s
}
