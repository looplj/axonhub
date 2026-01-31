package jina

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestEmbeddingInboundTransformer_TransformRequest(t *testing.T) {
	transformer := NewEmbeddingInboundTransformer()

	t.Run("valid string input", func(t *testing.T) {
		reqBody := map[string]any{
			"model": "jina-embeddings-v3",
			"input": "The quick brown fox",
		}
		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		httpReq := &httpclient.Request{
			Body: body,
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
		}

		llmReq, err := transformer.TransformRequest(context.Background(), httpReq)
		require.NoError(t, err)
		require.NotNil(t, llmReq)
		require.Equal(t, "jina-embeddings-v3", llmReq.Model)
		require.Equal(t, llm.APIFormatJinaEmbedding, llmReq.APIFormat)
		require.Nil(t, llmReq.Stream)
		require.NotNil(t, llmReq.Embedding)
		require.Empty(t, llmReq.Embedding.Task)
	})

	t.Run("valid string input with task", func(t *testing.T) {
		reqBody := map[string]any{
			"model": "jina-embeddings-v3",
			"input": "The quick brown fox",
			"task":  "retrieval.query",
		}
		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		httpReq := &httpclient.Request{
			Body: body,
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
		}

		llmReq, err := transformer.TransformRequest(context.Background(), httpReq)
		require.NoError(t, err)
		require.NotNil(t, llmReq)
		require.Equal(t, "retrieval.query", llmReq.Embedding.Task)
	})

	t.Run("valid array input", func(t *testing.T) {
		reqBody := map[string]any{
			"model": "jina-embeddings-v3",
			"input": []string{"Hello", "World"},
		}
		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		httpReq := &httpclient.Request{
			Body: body,
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
		}

		llmReq, err := transformer.TransformRequest(context.Background(), httpReq)
		require.NoError(t, err)
		require.NotNil(t, llmReq)
	})

	t.Run("valid array input with all task types", func(t *testing.T) {
		tasks := []string{
			"text-matching",
			"retrieval.query",
			"retrieval.passage",
			"separation",
			"classification",
			"none",
		}

		for _, task := range tasks {
			reqBody := map[string]any{
				"model": "jina-embeddings-v3",
				"input": []string{"Hello", "World"},
				"task":  task,
			}
			body, err := json.Marshal(reqBody)
			require.NoError(t, err)

			httpReq := &httpclient.Request{
				Body: body,
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
			}

			llmReq, err := transformer.TransformRequest(context.Background(), httpReq)
			require.NoError(t, err)
			require.NotNil(t, llmReq)
			require.Equal(t, task, llmReq.Embedding.Task)
		}
	})

	t.Run("missing model", func(t *testing.T) {
		reqBody := map[string]any{
			"input": "test",
		}
		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		httpReq := &httpclient.Request{
			Body: body,
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
		}

		_, err = transformer.TransformRequest(context.Background(), httpReq)
		require.Error(t, err)
		require.Contains(t, err.Error(), "model is required")
	})

	t.Run("missing input", func(t *testing.T) {
		reqBody := map[string]any{
			"model": "jina-embeddings-v3",
		}
		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		httpReq := &httpclient.Request{
			Body: body,
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
		}

		_, err = transformer.TransformRequest(context.Background(), httpReq)
		require.Error(t, err)
		require.Contains(t, err.Error(), "input cannot be empty string")
	})

	t.Run("empty string input", func(t *testing.T) {
		reqBody := map[string]any{
			"model": "jina-embeddings-v3",
			"input": "",
		}
		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		httpReq := &httpclient.Request{
			Body: body,
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
		}

		_, err = transformer.TransformRequest(context.Background(), httpReq)
		require.Error(t, err)
		require.Contains(t, err.Error(), "input cannot be empty string")
	})

	t.Run("empty array input", func(t *testing.T) {
		reqBody := map[string]any{
			"model": "jina-embeddings-v3",
			"input": []string{},
		}
		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		httpReq := &httpclient.Request{
			Body: body,
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
		}

		_, err = transformer.TransformRequest(context.Background(), httpReq)
		require.Error(t, err)
		require.Contains(t, err.Error(), "input cannot be empty array")
	})

	t.Run("nil http request", func(t *testing.T) {
		_, err := transformer.TransformRequest(context.Background(), nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "http request is nil")
	})

	t.Run("empty body", func(t *testing.T) {
		httpReq := &httpclient.Request{
			Body: []byte{},
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
		}

		_, err := transformer.TransformRequest(context.Background(), httpReq)
		require.Error(t, err)
		require.Contains(t, err.Error(), "request body is empty")
	})

	t.Run("unsupported content type", func(t *testing.T) {
		httpReq := &httpclient.Request{
			Body: []byte("test"),
			Headers: http.Header{
				"Content-Type": []string{"text/plain"},
			},
		}

		_, err := transformer.TransformRequest(context.Background(), httpReq)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported content type")
	})

	t.Run("invalid json body", func(t *testing.T) {
		httpReq := &httpclient.Request{
			Body: []byte("not valid json"),
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
		}

		_, err := transformer.TransformRequest(context.Background(), httpReq)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to decode embedding request")
	})
}

func TestOutboundTransformer_TransformRequest_Embedding(t *testing.T) {
	t.Run("valid embedding request with /v1 suffix", func(t *testing.T) {
		config := &Config{
			BaseURL: "https://api.jina.ai/v1",
			APIKey:  "test-key",
		}
		transformer, err := NewOutboundTransformerWithConfig(config)
		require.NoError(t, err)

		llmReq := &llm.Request{
			Model:       "jina-embeddings-v3",
			RequestType: llm.RequestTypeEmbedding,
			Embedding: &llm.EmbeddingRequest{
				Input: llm.EmbeddingInput{String: "Hello world"},
			},
		}

		httpReq, err := transformer.TransformRequest(context.Background(), llmReq)
		require.NoError(t, err)
		require.NotNil(t, httpReq)
		require.Equal(t, http.MethodPost, httpReq.Method)
		require.Equal(t, "https://api.jina.ai/v1/embeddings", httpReq.URL)
		require.Equal(t, "application/json", httpReq.Headers.Get("Content-Type"))
		require.NotNil(t, httpReq.Auth)
		require.Equal(t, "bearer", httpReq.Auth.Type)

		var jinaReq EmbeddingRequest

		err = json.Unmarshal(httpReq.Body, &jinaReq)
		require.NoError(t, err)
		require.Equal(t, "text-matching", jinaReq.Task)
	})

	t.Run("valid embedding request without /v1 suffix", func(t *testing.T) {
		config := &Config{
			BaseURL: "https://api.jina.ai",
			APIKey:  "test-key",
		}
		transformer, err := NewOutboundTransformerWithConfig(config)
		require.NoError(t, err)

		llmReq := &llm.Request{
			Model:       "jina-embeddings-v3",
			RequestType: llm.RequestTypeEmbedding,
			Embedding: &llm.EmbeddingRequest{
				Input: llm.EmbeddingInput{String: "Hello world"},
			},
		}

		httpReq, err := transformer.TransformRequest(context.Background(), llmReq)
		require.NoError(t, err)
		require.Equal(t, "https://api.jina.ai/v1/embeddings", httpReq.URL)
	})

	t.Run("embedding request with explicit task", func(t *testing.T) {
		config := &Config{
			BaseURL: "https://api.jina.ai/v1",
			APIKey:  "test-key",
		}
		transformer, err := NewOutboundTransformerWithConfig(config)
		require.NoError(t, err)

		llmReq := &llm.Request{
			Model:       "jina-embeddings-v3",
			RequestType: llm.RequestTypeEmbedding,
			Embedding: &llm.EmbeddingRequest{
				Input: llm.EmbeddingInput{String: "Hello world"},
				Task:  "retrieval.query",
			},
		}

		httpReq, err := transformer.TransformRequest(context.Background(), llmReq)
		require.NoError(t, err)

		var jinaReq EmbeddingRequest

		err = json.Unmarshal(httpReq.Body, &jinaReq)
		require.NoError(t, err)
		require.Equal(t, "retrieval.query", jinaReq.Task)
	})

	t.Run("embedding request with empty task defaults to text-matching", func(t *testing.T) {
		config := &Config{
			BaseURL: "https://api.jina.ai/v1",
			APIKey:  "test-key",
		}
		transformer, err := NewOutboundTransformerWithConfig(config)
		require.NoError(t, err)

		llmReq := &llm.Request{
			Model:       "jina-embeddings-v3",
			RequestType: llm.RequestTypeEmbedding,
			Embedding: &llm.EmbeddingRequest{
				Input: llm.EmbeddingInput{String: "Hello world"},
				Task:  "",
			},
		}

		httpReq, err := transformer.TransformRequest(context.Background(), llmReq)
		require.NoError(t, err)

		var jinaReq EmbeddingRequest

		err = json.Unmarshal(httpReq.Body, &jinaReq)
		require.NoError(t, err)
		require.Equal(t, "text-matching", jinaReq.Task)
	})

	t.Run("embedding request nil llm request", func(t *testing.T) {
		config := &Config{
			BaseURL: "https://api.jina.ai/v1",
			APIKey:  "test-key",
		}
		transformer, err := NewOutboundTransformerWithConfig(config)
		require.NoError(t, err)

		_, err = transformer.TransformRequest(context.Background(), nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "request is nil")
	})

	t.Run("embedding request missing embedding in request", func(t *testing.T) {
		config := &Config{
			BaseURL: "https://api.jina.ai/v1",
			APIKey:  "test-key",
		}
		transformer, err := NewOutboundTransformerWithConfig(config)
		require.NoError(t, err)

		llmReq := &llm.Request{
			Model:       "jina-embeddings-v3",
			RequestType: llm.RequestTypeEmbedding,
		}

		_, err = transformer.TransformRequest(context.Background(), llmReq)
		require.Error(t, err)
		require.Contains(t, err.Error(), "embedding request is nil")
	})

	t.Run("embedding request wrong request type", func(t *testing.T) {
		config := &Config{
			BaseURL: "https://api.jina.ai/v1",
			APIKey:  "test-key",
		}
		transformer, err := NewOutboundTransformerWithConfig(config)
		require.NoError(t, err)

		llmReq := &llm.Request{
			Model:       "jina-embeddings-v3",
			RequestType: llm.RequestTypeChat,
		}

		_, err = transformer.TransformRequest(context.Background(), llmReq)
		require.Error(t, err)
		require.Contains(t, err.Error(), "is not supported")
	})
}

func TestOutboundTransformer_TransformResponse_Embedding(t *testing.T) {
	config := &Config{
		BaseURL: "https://api.jina.ai/v1",
		APIKey:  "test-key",
	}
	transformer, err := NewOutboundTransformerWithConfig(config)
	require.NoError(t, err)

	t.Run("valid embedding response", func(t *testing.T) {
		embResp := EmbeddingResponse{
			Object: "list",
			Model:  "jina-embeddings-v3",
			Data: []EmbeddingData{
				{
					Object:    "embedding",
					Index:     0,
					Embedding: llm.Embedding{Embedding: []float64{0.1, 0.2, 0.3}},
				},
			},
			Usage: EmbeddingUsage{
				PromptTokens: 5,
				TotalTokens:  5,
			},
		}

		respBody, err := json.Marshal(embResp)
		require.NoError(t, err)

		httpResp := &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       respBody,
			Request: &httpclient.Request{
				APIFormat: string(llm.APIFormatJinaEmbedding),
			},
		}

		llmResp, err := transformer.TransformResponse(context.Background(), httpResp)
		require.NoError(t, err)
		require.NotNil(t, llmResp)
		require.Equal(t, "list", llmResp.Embedding.Object)
		require.Equal(t, "jina-embeddings-v3", llmResp.Model)
		require.NotNil(t, llmResp.Embedding.Usage)
		require.Equal(t, int64(5), llmResp.Embedding.Usage.PromptTokens)
		require.Equal(t, int64(5), llmResp.Embedding.Usage.TotalTokens)
	})

	t.Run("embedding response nil http response", func(t *testing.T) {
		_, err := transformer.TransformResponse(context.Background(), nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "http response is nil")
	})

	t.Run("embedding response http error 400", func(t *testing.T) {
		httpResp := &httpclient.Response{
			StatusCode: http.StatusBadRequest,
			Body:       []byte(`{"error": {"message": "Invalid request"}}`),
			Request: &httpclient.Request{
				APIFormat: string(llm.APIFormatJinaEmbedding),
			},
		}

		_, err := transformer.TransformResponse(context.Background(), httpResp)
		require.Error(t, err)
	})

	t.Run("embedding response empty response body", func(t *testing.T) {
		httpResp := &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       []byte{},
			Request: &httpclient.Request{
				APIFormat: string(llm.APIFormatJinaEmbedding),
			},
		}

		_, err := transformer.TransformResponse(context.Background(), httpResp)
		require.Error(t, err)
		require.Contains(t, err.Error(), "response body is empty")
	})

	t.Run("embedding response invalid json response", func(t *testing.T) {
		httpResp := &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       []byte("not valid json"),
			Request: &httpclient.Request{
				APIFormat: string(llm.APIFormatJinaEmbedding),
			},
		}

		_, err := transformer.TransformResponse(context.Background(), httpResp)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to unmarshal embedding response")
	})
}

func TestEmbeddingInboundTransformer_TransformResponse(t *testing.T) {
	transformer := NewEmbeddingInboundTransformer()

	t.Run("valid response", func(t *testing.T) {
		llmResp := &llm.Response{
			Object: "list",
			Model:  "jina-embeddings-v3",
			Embedding: &llm.EmbeddingResponse{
				Object: "list",
				Data: []llm.EmbeddingData{
					{
						Object:    "embedding",
						Embedding: llm.Embedding{Embedding: []float64{0.1, 0.2, 0.3}},
						Index:     0,
					},
				},
				Usage: &llm.EmbeddingUsage{
					PromptTokens: 5,
					TotalTokens:  5,
				},
			},
		}

		httpResp, err := transformer.TransformResponse(context.Background(), llmResp)
		require.NoError(t, err)
		require.NotNil(t, httpResp)
		require.Equal(t, http.StatusOK, httpResp.StatusCode)
		require.Equal(t, "application/json", httpResp.Headers.Get("Content-Type"))

		var returnedEmbResp EmbeddingResponse

		err = json.Unmarshal(httpResp.Body, &returnedEmbResp)
		require.NoError(t, err)
		require.Equal(t, "list", returnedEmbResp.Object)
		require.Equal(t, "jina-embeddings-v3", returnedEmbResp.Model)
		require.Len(t, returnedEmbResp.Data, 1)
	})

	t.Run("nil llm response", func(t *testing.T) {
		_, err := transformer.TransformResponse(context.Background(), nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "embedding response is nil")
	})
}

func TestEmbeddingTransformers_APIFormat(t *testing.T) {
	inbound := NewEmbeddingInboundTransformer()
	require.Equal(t, llm.APIFormatJinaEmbedding, inbound.APIFormat())

	config := &Config{
		BaseURL: "https://api.jina.ai/v1",
		APIKey:  "test-key",
	}
	outbound, err := NewOutboundTransformerWithConfig(config)
	require.NoError(t, err)
	require.Equal(t, llm.APIFormatJinaRerank, outbound.APIFormat())
}

func TestOutboundTransformer_TransformError_Embedding(t *testing.T) {
	config := &Config{
		BaseURL: "https://api.jina.ai/v1",
		APIKey:  "test-key",
	}
	transformer, err := NewOutboundTransformerWithConfig(config)
	require.NoError(t, err)

	t.Run("nil error", func(t *testing.T) {
		respErr := transformer.TransformError(context.Background(), nil)
		require.NotNil(t, respErr)
		require.Equal(t, http.StatusInternalServerError, respErr.StatusCode)
	})

	t.Run("jina format error", func(t *testing.T) {
		httpErr := &httpclient.Error{
			StatusCode: http.StatusBadRequest,
			Body:       []byte(`{"error": {"message": "Invalid model", "type": "invalid_request_error"}}`),
		}

		respErr := transformer.TransformError(context.Background(), httpErr)
		require.NotNil(t, respErr)
		require.Equal(t, http.StatusBadRequest, respErr.StatusCode)
		require.Equal(t, "Invalid model", respErr.Detail.Message)
	})

	t.Run("non-json error body", func(t *testing.T) {
		httpErr := &httpclient.Error{
			StatusCode: http.StatusServiceUnavailable,
			Body:       []byte("Service unavailable"),
		}

		respErr := transformer.TransformError(context.Background(), httpErr)
		require.NotNil(t, respErr)
		require.Equal(t, http.StatusServiceUnavailable, respErr.StatusCode)
	})
}

func TestOutboundTransformer_StreamNotSupported_Embedding(t *testing.T) {
	config := &Config{
		BaseURL: "https://api.jina.ai/v1",
		APIKey:  "test-key",
	}
	transformer, err := NewOutboundTransformerWithConfig(config)
	require.NoError(t, err)

	t.Run("transform stream returns error", func(t *testing.T) {
		_, err := transformer.TransformStream(context.Background(), nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "does not support streaming")
	})

	t.Run("aggregate stream chunks returns error", func(t *testing.T) {
		_, _, err := transformer.AggregateStreamChunks(context.Background(), nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "does not support streaming")
	})
}

func TestOutboundTransformer_URLBuilding_Embedding(t *testing.T) {
	testCases := []struct {
		name        string
		baseURL     string
		expectedURL string
	}{
		{
			name:        "with /v1 suffix",
			baseURL:     "https://api.jina.ai/v1",
			expectedURL: "https://api.jina.ai/v1/embeddings",
		},
		{
			name:        "without /v1 suffix",
			baseURL:     "https://api.jina.ai",
			expectedURL: "https://api.jina.ai/v1/embeddings",
		},
		{
			name:        "with trailing slash",
			baseURL:     "https://api.jina.ai/",
			expectedURL: "https://api.jina.ai/v1/embeddings",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &Config{
				BaseURL: tc.baseURL,
				APIKey:  "test-key",
			}
			transformer, err := NewOutboundTransformerWithConfig(config)
			require.NoError(t, err)

			llmReq := &llm.Request{
				Model:       "jina-embeddings-v3",
				RequestType: llm.RequestTypeEmbedding,
				Embedding: &llm.EmbeddingRequest{
					Input: llm.EmbeddingInput{String: "Hello world"},
				},
			}

			httpReq, err := transformer.TransformRequest(context.Background(), llmReq)
			require.NoError(t, err)
			require.Equal(t, tc.expectedURL, httpReq.URL)
		})
	}
}
