package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
)

func newTestEmbeddingTransformer() *OutboundTransformer {
	t, _ := NewOutboundTransformerWithConfig(Config{
		BaseURL:        "https://generativelanguage.googleapis.com",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	return t.(*OutboundTransformer)
}

func TestTransformEmbeddingRequest_SingleText(t *testing.T) {
	tr := newTestEmbeddingTransformer()

	llmReq := &llm.Request{
		Model:       "gemini-embedding-001",
		RequestType: llm.RequestTypeEmbedding,
		Embedding: &llm.EmbeddingRequest{
			Input: llm.EmbeddingInput{String: "Hello world"},
		},
	}

	httpReq, err := tr.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)
	require.NotNil(t, httpReq)

	require.Equal(t, http.MethodPost, httpReq.Method)
	require.Contains(t, httpReq.URL, "models/gemini-embedding-001:embedContent")
	require.Equal(t, string(llm.RequestTypeEmbedding), httpReq.RequestType)
	require.Equal(t, string(llm.APIFormatGeminiEmbedding), httpReq.APIFormat)

	var geminiReq EmbedContentRequest
	err = json.Unmarshal(httpReq.Body, &geminiReq)
	require.NoError(t, err)
	require.Equal(t, "models/gemini-embedding-001", geminiReq.Model)
	require.NotNil(t, geminiReq.Content)
	require.Len(t, geminiReq.Content.Parts, 1)
	require.Equal(t, "Hello world", geminiReq.Content.Parts[0].Text)
}

func TestTransformEmbeddingRequest_BatchTexts(t *testing.T) {
	tr := newTestEmbeddingTransformer()

	llmReq := &llm.Request{
		Model:       "gemini-embedding-001",
		RequestType: llm.RequestTypeEmbedding,
		Embedding: &llm.EmbeddingRequest{
			Input: llm.EmbeddingInput{StringArray: []string{"Hello", "World", "Test"}},
		},
	}

	httpReq, err := tr.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)
	require.NotNil(t, httpReq)

	require.Contains(t, httpReq.URL, "models/gemini-embedding-001:batchEmbedContents")

	var batchReq BatchEmbedContentsRequest
	err = json.Unmarshal(httpReq.Body, &batchReq)
	require.NoError(t, err)
	require.Len(t, batchReq.Requests, 3)
	require.Equal(t, "Hello", batchReq.Requests[0].Content.Parts[0].Text)
	require.Equal(t, "World", batchReq.Requests[1].Content.Parts[0].Text)
	require.Equal(t, "Test", batchReq.Requests[2].Content.Parts[0].Text)
}

func TestTransformEmbeddingRequest_WithDimensions(t *testing.T) {
	tr := newTestEmbeddingTransformer()

	dims := 256
	llmReq := &llm.Request{
		Model:       "gemini-embedding-001",
		RequestType: llm.RequestTypeEmbedding,
		Embedding: &llm.EmbeddingRequest{
			Input:      llm.EmbeddingInput{String: "Hello"},
			Dimensions: &dims,
		},
	}

	httpReq, err := tr.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	var geminiReq EmbedContentRequest
	err = json.Unmarshal(httpReq.Body, &geminiReq)
	require.NoError(t, err)
	require.NotNil(t, geminiReq.OutputDimensionality)
	require.Equal(t, int32(256), *geminiReq.OutputDimensionality)
}

func TestTransformEmbeddingRequest_WithTaskType(t *testing.T) {
	tr := newTestEmbeddingTransformer()

	llmReq := &llm.Request{
		Model:       "gemini-embedding-001",
		RequestType: llm.RequestTypeEmbedding,
		Embedding: &llm.EmbeddingRequest{
			Input: llm.EmbeddingInput{String: "Hello"},
			Task:  "retrieval.query",
		},
	}

	httpReq, err := tr.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	var geminiReq EmbedContentRequest
	err = json.Unmarshal(httpReq.Body, &geminiReq)
	require.NoError(t, err)
	require.Equal(t, "RETRIEVAL_QUERY", geminiReq.TaskType)
}

func TestTransformEmbeddingRequest_NilEmbedding(t *testing.T) {
	tr := newTestEmbeddingTransformer()

	llmReq := &llm.Request{
		Model:       "gemini-embedding-001",
		RequestType: llm.RequestTypeEmbedding,
	}

	_, err := tr.TransformRequest(context.Background(), llmReq)
	require.Error(t, err)
	require.Contains(t, err.Error(), "embedding request is nil")
}

func TestTransformEmbeddingResponse_Single(t *testing.T) {
	tr := newTestEmbeddingTransformer()

	respBody := EmbedContentResponse{
		Embedding: &ContentEmbedding{
			Values: []float32{0.1, 0.2, 0.3},
		},
	}

	body, err := json.Marshal(respBody)
	require.NoError(t, err)

	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Request: &httpclient.Request{
			RequestType: string(llm.RequestTypeEmbedding),
			APIFormat:   string(llm.APIFormatGeminiEmbedding),
		},
	}

	llmResp, err := tr.TransformResponse(context.Background(), httpResp)
	require.NoError(t, err)
	require.NotNil(t, llmResp)
	require.Equal(t, llm.RequestTypeEmbedding, llmResp.RequestType)
	require.Equal(t, llm.APIFormatGeminiEmbedding, llmResp.APIFormat)
	require.NotNil(t, llmResp.Embedding)
	require.Len(t, llmResp.Embedding.Data, 1)
	require.Equal(t, "embedding", llmResp.Embedding.Data[0].Object)
	require.Equal(t, 0, llmResp.Embedding.Data[0].Index)
	require.InDelta(t, 0.1, llmResp.Embedding.Data[0].Embedding.Embedding[0], 0.001)
	require.InDelta(t, 0.2, llmResp.Embedding.Data[0].Embedding.Embedding[1], 0.001)
	require.InDelta(t, 0.3, llmResp.Embedding.Data[0].Embedding.Embedding[2], 0.001)
}

func TestTransformEmbeddingResponse_Batch(t *testing.T) {
	tr := newTestEmbeddingTransformer()

	respBody := BatchEmbedContentsResponse{
		Embeddings: []*ContentEmbedding{
			{Values: []float32{0.1, 0.2}},
			{Values: []float32{0.3, 0.4}},
		},
	}

	body, err := json.Marshal(respBody)
	require.NoError(t, err)

	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Request: &httpclient.Request{
			RequestType: string(llm.RequestTypeEmbedding),
			APIFormat:   string(llm.APIFormatGeminiEmbedding),
		},
	}

	llmResp, err := tr.TransformResponse(context.Background(), httpResp)
	require.NoError(t, err)
	require.NotNil(t, llmResp)
	require.NotNil(t, llmResp.Embedding)
	require.Len(t, llmResp.Embedding.Data, 2)
	require.Equal(t, 0, llmResp.Embedding.Data[0].Index)
	require.Equal(t, 1, llmResp.Embedding.Data[1].Index)
	require.InDelta(t, 0.1, llmResp.Embedding.Data[0].Embedding.Embedding[0], 0.001)
	require.InDelta(t, 0.3, llmResp.Embedding.Data[1].Embedding.Embedding[0], 0.001)
}

func TestTransformEmbeddingResponse_Error(t *testing.T) {
	tr := newTestEmbeddingTransformer()

	httpResp := &httpclient.Response{
		StatusCode: http.StatusBadRequest,
		Body:       []byte(`{"error":{"code":400,"message":"Invalid request","status":"INVALID_ARGUMENT"}}`),
		Request: &httpclient.Request{
			RequestType: string(llm.RequestTypeEmbedding),
			APIFormat:   string(llm.APIFormatGeminiEmbedding),
		},
	}

	_, err := tr.TransformResponse(context.Background(), httpResp)
	require.Error(t, err)
}

func TestBuildEmbeddingURL(t *testing.T) {
	t.Run("standard API", func(t *testing.T) {
		tr := newTestEmbeddingTransformer()
		url := tr.buildEmbeddingURL("gemini-embedding-001", false)
		require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models/gemini-embedding-001:embedContent", url)
	})

	t.Run("standard API batch", func(t *testing.T) {
		tr := newTestEmbeddingTransformer()
		url := tr.buildEmbeddingURL("gemini-embedding-001", true)
		require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models/gemini-embedding-001:batchEmbedContents", url)
	})

	t.Run("vertex AI", func(t *testing.T) {
		vt, _ := NewOutboundTransformerWithConfig(Config{
			BaseURL:        "https://us-central1-aiplatform.googleapis.com",
			APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
			PlatformType:   PlatformVertex,
		})
		tr := vt.(*OutboundTransformer)
		url := tr.buildEmbeddingURL("gemini-embedding-001", false)
		require.Equal(t, "https://us-central1-aiplatform.googleapis.com/v1/publishers/google/models/gemini-embedding-001:embedContent", url)
	})
}

func TestMapEmbeddingTaskType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"retrieval.query", "RETRIEVAL_QUERY"},
		{"retrieval.passage", "RETRIEVAL_DOCUMENT"},
		{"text-matching", "SEMANTIC_SIMILARITY"},
		{"classification", "CLASSIFICATION"},
		{"clustering", "CLUSTERING"},
		{"unknown", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapEmbeddingTaskType(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}
