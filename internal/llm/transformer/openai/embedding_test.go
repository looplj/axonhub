package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

func TestEmbeddingInboundTransformer_TransformRequest(t *testing.T) {
	transformer := NewEmbeddingInboundTransformer()

	t.Run("valid string input", func(t *testing.T) {
		reqBody := map[string]any{
			"model": "text-embedding-ada-002",
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
		require.Equal(t, "text-embedding-ada-002", llmReq.Model)
		require.Equal(t, llm.APIFormatOpenAIEmbedding, llmReq.APIFormat)
		require.Nil(t, llmReq.Stream)
		require.NotEmpty(t, llmReq.ExtraBody)
	})

	t.Run("valid array input", func(t *testing.T) {
		reqBody := map[string]any{
			"model": "text-embedding-ada-002",
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
			"model": "text-embedding-ada-002",
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
		require.Contains(t, err.Error(), "input is required")
	})

	t.Run("empty string input", func(t *testing.T) {
		reqBody := map[string]any{
			"model": "text-embedding-ada-002",
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
			"model": "text-embedding-ada-002",
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

	t.Run("whitespace only input", func(t *testing.T) {
		reqBody := map[string]any{
			"model": "text-embedding-ada-002",
			"input": "   ",
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

	t.Run("valid token ids input", func(t *testing.T) {
		reqBody := map[string]any{
			"model": "text-embedding-ada-002",
			"input": []int{1234, 5678, 9012},
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

	t.Run("valid nested token ids input", func(t *testing.T) {
		reqBody := map[string]any{
			"model": "text-embedding-ada-002",
			"input": [][]int{{1234, 5678}, {9012, 3456}},
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

	t.Run("empty nested array input", func(t *testing.T) {
		reqBody := map[string]any{
			"model": "text-embedding-ada-002",
			"input": [][]int{{}, {1234}},
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
		require.Contains(t, err.Error(), "input[0] cannot be empty array")
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

func TestEmbeddingOutboundTransformer_TransformRequest(t *testing.T) {
	t.Run("valid request with /v1 suffix", func(t *testing.T) {
		config := &Config{
			Type:    PlatformOpenAI,
			BaseURL: "https://api.openai.com/v1",
			APIKey:  "test-key",
		}
		transformer, err := NewOutboundTransformerWithConfig(config)
		require.NoError(t, err)

		embReq := EmbeddingRequest{
			Model: "text-embedding-ada-002",
			Input: &llm.EmbeddingInput{String: "Hello world"},
		}
		extraBody, err := json.Marshal(embReq)
		require.NoError(t, err)

		llmReq := &llm.Request{
			Model:       "text-embedding-ada-002",
			ExtraBody:   extraBody,
			RequestType: llm.RequestTypeEmbedding,
		}

		httpReq, err := transformer.TransformRequest(context.Background(), llmReq)
		require.NoError(t, err)
		require.NotNil(t, httpReq)
		require.Equal(t, http.MethodPost, httpReq.Method)
		require.Equal(t, "https://api.openai.com/v1/embeddings", httpReq.URL)
		require.Equal(t, "application/json", httpReq.Headers.Get("Content-Type"))
		require.NotNil(t, httpReq.Auth)
		require.Equal(t, "bearer", httpReq.Auth.Type)
		require.NotNil(t, httpReq)
		require.Equal(t, "https://api.openai.com/v1/embeddings", httpReq.URL)
	})

	t.Run("nil llm request", func(t *testing.T) {
		config := &Config{
			Type:    PlatformOpenAI,
			BaseURL: "https://api.openai.com/v1",
			APIKey:  "test-key",
		}
		transformer, err := NewOutboundTransformerWithConfig(config)
		require.NoError(t, err)

		_, err = transformer.TransformRequest(context.Background(), nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "request is nil")
	})

	t.Run("missing extra body", func(t *testing.T) {
		config := &Config{
			Type:    PlatformOpenAI,
			BaseURL: "https://api.openai.com/v1",
			APIKey:  "test-key",
		}
		transformer, err := NewOutboundTransformerWithConfig(config)
		require.NoError(t, err)

		llmReq := &llm.Request{
			Model:       "text-embedding-ada-002",
			RequestType: llm.RequestTypeEmbedding,
		}

		_, err = transformer.TransformRequest(context.Background(), llmReq)
		require.Error(t, err)
		require.Contains(t, err.Error(), "embedding request missing in ExtraBody")
	})
}

func TestEmbeddingOutboundTransformer_TransformResponse(t *testing.T) {
	config := &Config{
		Type:    PlatformOpenAI,
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
	}
	transformer, err := NewOutboundTransformerWithConfig(config)
	require.NoError(t, err)

	t.Run("valid response", func(t *testing.T) {
		embResp := EmbeddingResponse{
			Object: "list",
			Model:  "text-embedding-ada-002",
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
				TransformerMetadata: map[string]any{
					"outbound_format_type": llm.APIFormatOpenAIEmbedding.String(),
				},
			},
		}

		llmResp, err := transformer.TransformResponse(context.Background(), httpResp)
		require.NoError(t, err)
		require.NotNil(t, llmResp)
		require.Equal(t, "list", llmResp.Embedding.Object)
		require.Equal(t, "text-embedding-ada-002", llmResp.Embedding.Model)
		require.NotNil(t, llmResp.Embedding.Usage)
		require.Equal(t, int64(5), llmResp.Embedding.Usage.PromptTokens)
		require.Equal(t, int64(5), llmResp.Embedding.Usage.TotalTokens)
		require.NotNil(t, llmResp.Embedding)
	})

	t.Run("response with upstream ID", func(t *testing.T) {
		embResp := EmbeddingResponse{
			Object: "list",
			Model:  "text-embedding-ada-002",
			Data: []EmbeddingData{
				{
					Object:    "embedding",
					Index:     0,
					Embedding: llm.Embedding{Embedding: []float64{0.1, 0.2, 0.3}},
				},
			},
		}

		respBody, err := json.Marshal(embResp)
		require.NoError(t, err)

		httpResp := &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       respBody,
			Request: &httpclient.Request{
				TransformerMetadata: map[string]any{
					"outbound_format_type": llm.APIFormatOpenAIEmbedding.String(),
				},
			},
		}

		llmResp, err := transformer.TransformResponse(context.Background(), httpResp)
		require.NoError(t, err)
		require.Equal(t, "", llmResp.ID)
	})

	t.Run("nil http response", func(t *testing.T) {
		_, err := transformer.TransformResponse(context.Background(), nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "http response is nil")
	})

	t.Run("http error 400", func(t *testing.T) {
		httpResp := &httpclient.Response{
			StatusCode: http.StatusBadRequest,
			Body:       []byte(`{"error": {"message": "Invalid request"}}`),
			Request: &httpclient.Request{
				TransformerMetadata: map[string]any{
					"outbound_format_type": llm.APIFormatOpenAIEmbedding.String(),
				},
			},
		}

		_, err := transformer.TransformResponse(context.Background(), httpResp)
		require.Error(t, err)
		// Error is returned from transformEmbeddingResponse
		require.Contains(t, err.Error(), "400")
	})

	t.Run("http error 500", func(t *testing.T) {
		httpResp := &httpclient.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       []byte(`{"error": {"message": "Internal server error"}}`),
			Request: &httpclient.Request{
				TransformerMetadata: map[string]any{
					"outbound_format_type": llm.APIFormatOpenAIEmbedding.String(),
				},
			},
		}

		_, err := transformer.TransformResponse(context.Background(), httpResp)
		require.Error(t, err)
		// Error is returned from transformEmbeddingResponse
		require.Contains(t, err.Error(), "500")
	})

	t.Run("empty response body", func(t *testing.T) {
		httpResp := &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       []byte{},
			Request: &httpclient.Request{
				TransformerMetadata: map[string]any{
					"outbound_format_type": llm.APIFormatOpenAIEmbedding.String(),
				},
			},
		}

		_, err := transformer.TransformResponse(context.Background(), httpResp)
		require.Error(t, err)
		require.Contains(t, err.Error(), "response body is empty")
	})

	t.Run("invalid json response", func(t *testing.T) {
		httpResp := &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       []byte("not valid json"),
			Request: &httpclient.Request{
				TransformerMetadata: map[string]any{
					"outbound_format_type": llm.APIFormatOpenAIEmbedding.String(),
				},
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
		embResp := EmbeddingResponse{
			Object: "list",
			Model:  "text-embedding-ada-002",
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

		llmResp := &llm.Response{
			Object: "list",
			Model:  "text-embedding-ada-002",
			Embedding: &llm.EmbeddingResponse{
				Object: embResp.Object,
				Data: []llm.EmbeddingData{
					{
						Object:    embResp.Data[0].Object,
						Embedding: embResp.Data[0].Embedding,
						Index:     embResp.Data[0].Index,
					},
				},
				Model: embResp.Model,
				Usage: &llm.EmbeddingUsage{
					PromptTokens: embResp.Usage.PromptTokens,
					TotalTokens:  embResp.Usage.TotalTokens,
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
		require.Equal(t, "text-embedding-ada-002", returnedEmbResp.Model)
		require.Len(t, returnedEmbResp.Data, 1)
	})

	t.Run("valid response without usage", func(t *testing.T) {
		embResp := &EmbeddingResponse{
			Object: "list",
			Model:  "text-embedding-ada-002",
			Data: []EmbeddingData{
				{
					Object:    "embedding",
					Index:     0,
					Embedding: llm.Embedding{Embedding: []float64{0.1, 0.2, 0.3}},
				},
			},
		}

		llmResp := &llm.Response{
			Object: "list",
			Model:  "text-embedding-ada-002",
			Embedding: &llm.EmbeddingResponse{
				Object: embResp.Object,
				Data: []llm.EmbeddingData{
					{
						Object:    embResp.Data[0].Object,
						Embedding: embResp.Data[0].Embedding,
						Index:     embResp.Data[0].Index,
					},
				},
				Model: embResp.Model,
			},
		}

		httpResp, err := transformer.TransformResponse(context.Background(), llmResp)
		require.NoError(t, err)
		require.NotNil(t, httpResp)
	})

	t.Run("nil llm response", func(t *testing.T) {
		_, err := transformer.TransformResponse(context.Background(), nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "embedding response is nil")
	})
}

func TestEmbeddingTransformers_APIFormat(t *testing.T) {
	inbound := NewEmbeddingInboundTransformer()
	require.Equal(t, llm.APIFormatOpenAIEmbedding, inbound.APIFormat())

	config := &Config{
		Type:    PlatformOpenAI,
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
	}
	outbound, err := NewOutboundTransformerWithConfig(config)
	require.NoError(t, err)
	require.Equal(t, llm.APIFormatOpenAIChatCompletion, outbound.APIFormat())
}

func TestEmbeddingOutboundTransformer_TransformError(t *testing.T) {
	config := &Config{
		Type:    PlatformOpenAI,
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
	}
	transformer, err := NewOutboundTransformerWithConfig(config)
	require.NoError(t, err)

	t.Run("nil error", func(t *testing.T) {
		respErr := transformer.TransformError(context.Background(), nil)
		require.NotNil(t, respErr)
		require.Equal(t, http.StatusInternalServerError, respErr.StatusCode)
	})

	t.Run("openai format error", func(t *testing.T) {
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

func TestEmbeddingOutboundTransformer_StreamNotSupported(t *testing.T) {
	// Note: Embedding streaming is rejected at the inbound transformer level,
	// not at the outbound level. The outbound transformer's stream methods
	// are generic and work for all request types.
	t.Skip("Embedding streaming is rejected at inbound level, not outbound level")
}

func TestEmbeddingInboundTransformer_StreamNotSupported(t *testing.T) {
	transformer := NewEmbeddingInboundTransformer()

	t.Run("transform stream returns error", func(t *testing.T) {
		_, err := transformer.TransformStream(context.Background(), nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "do not support streaming")
	})

	t.Run("aggregate stream chunks returns error", func(t *testing.T) {
		_, _, err := transformer.AggregateStreamChunks(context.Background(), nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "do not support streaming")
	})
}

func TestEmbeddingOutboundTransformer_URLBuilding(t *testing.T) {
	testCases := []struct {
		name        string
		baseURL     string
		expectedURL string
	}{
		{
			name:        "with /v1 suffix",
			baseURL:     "https://api.openai.com/v1",
			expectedURL: "https://api.openai.com/v1/embeddings",
		},
		{
			name:        "without /v1 suffix",
			baseURL:     "https://api.openai.com",
			expectedURL: "https://api.openai.com/v1/embeddings",
		},
		{
			name:        "with trailing slash",
			baseURL:     "https://api.openai.com/",
			expectedURL: "https://api.openai.com/v1/embeddings",
		},
		{
			name:        "siliconflow api",
			baseURL:     "https://api.siliconflow.cn/v1",
			expectedURL: "https://api.siliconflow.cn/v1/embeddings",
		},
		{
			name:        "siliconflow api without v1",
			baseURL:     "https://api.siliconflow.cn",
			expectedURL: "https://api.siliconflow.cn/v1/embeddings",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &Config{
				Type:    PlatformOpenAI,
				BaseURL: tc.baseURL,
				APIKey:  "test-key",
			}
			transformer, err := NewOutboundTransformerWithConfig(config)
			require.NoError(t, err)

			embReq := EmbeddingRequest{
				Model: "text-embedding-ada-002",
				Input: &llm.EmbeddingInput{String: "Hello world"},
			}
			extraBody, err := json.Marshal(embReq)
			require.NoError(t, err)

			llmReq := &llm.Request{
				Model:       "text-embedding-ada-002",
				ExtraBody:   extraBody,
				RequestType: llm.RequestTypeEmbedding,
			}

			httpReq, err := transformer.TransformRequest(context.Background(), llmReq)
			require.NoError(t, err)
			require.Equal(t, tc.expectedURL, httpReq.URL)
		})
	}
}
