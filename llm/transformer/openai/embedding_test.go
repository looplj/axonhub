package openai

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
		require.NotNil(t, llmReq.Embedding)
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
		require.Contains(t, err.Error(), "input cannot be empty string")
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
			PlatformType: PlatformOpenAI,
			BaseURL:      "https://api.openai.com/v1",
			APIKey:       "test-key",
		}
		transformer, err := NewOutboundTransformerWithConfig(config)
		require.NoError(t, err)

		llmReq := &llm.Request{
			Model:       "text-embedding-ada-002",
			RequestType: llm.RequestTypeEmbedding,
			Embedding: &llm.EmbeddingRequest{
				Input: llm.EmbeddingInput{String: "Hello world"},
			},
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
			PlatformType: PlatformOpenAI,
			BaseURL:      "https://api.openai.com/v1",
			APIKey:       "test-key",
		}
		transformer, err := NewOutboundTransformerWithConfig(config)
		require.NoError(t, err)

		_, err = transformer.TransformRequest(context.Background(), nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "request is nil")
	})

	t.Run("missing embedding request", func(t *testing.T) {
		config := &Config{
			PlatformType: PlatformOpenAI,
			BaseURL:      "https://api.openai.com/v1",
			APIKey:       "test-key",
		}
		transformer, err := NewOutboundTransformerWithConfig(config)
		require.NoError(t, err)

		llmReq := &llm.Request{
			Model:       "text-embedding-ada-002",
			RequestType: llm.RequestTypeEmbedding,
		}

		_, err = transformer.TransformRequest(context.Background(), llmReq)
		require.Error(t, err)
		require.Contains(t, err.Error(), "embedding request is nil in llm.Request")
	})
}

func TestEmbeddingOutboundTransformer_TransformResponse(t *testing.T) {
	config := &Config{
		PlatformType: PlatformOpenAI,
		BaseURL:      "https://api.openai.com/v1",
		APIKey:       "test-key",
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
				APIFormat: string(llm.APIFormatOpenAIEmbedding),
			},
		}

		llmResp, err := transformer.TransformResponse(context.Background(), httpResp)
		require.NoError(t, err)
		require.NotNil(t, llmResp)
		require.Equal(t, "list", llmResp.Embedding.Object)
		require.Equal(t, "text-embedding-ada-002", llmResp.Model)
		require.NotNil(t, llmResp.Embedding.Usage)
		require.Equal(t, int64(5), llmResp.Embedding.Usage.PromptTokens)
		require.Equal(t, int64(5), llmResp.Embedding.Usage.TotalTokens)
		require.NotNil(t, llmResp.Embedding)
	})

	t.Run("response with upstream ID", func(t *testing.T) {
		embResp := EmbeddingResponse{
			Object: "list",
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
				APIFormat: string(llm.APIFormatOpenAIEmbedding),
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
				APIFormat: string(llm.APIFormatOpenAIEmbedding),
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
				APIFormat: string(llm.APIFormatOpenAIEmbedding),
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
				APIFormat: string(llm.APIFormatOpenAIEmbedding),
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
				APIFormat: string(llm.APIFormatOpenAIEmbedding),
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
		PlatformType: PlatformOpenAI,
		BaseURL:      "https://api.openai.com/v1",
		APIKey:       "test-key",
	}
	outbound, err := NewOutboundTransformerWithConfig(config)
	require.NoError(t, err)
	require.Equal(t, llm.APIFormatOpenAIChatCompletion, outbound.APIFormat())
}

func TestEmbeddingOutboundTransformer_TransformError(t *testing.T) {
	config := &Config{
		PlatformType: PlatformOpenAI,
		BaseURL:      "https://api.openai.com/v1",
		APIKey:       "test-key",
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
				PlatformType: PlatformOpenAI,
				BaseURL:      tc.baseURL,
				APIKey:       "test-key",
			}
			transformer, err := NewOutboundTransformerWithConfig(config)
			require.NoError(t, err)

			llmReq := &llm.Request{
				Model:       "text-embedding-ada-002",
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

func TestOutboundTransformer_RawURL_Embedding(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		request     *llm.Request
		expectedURL string
	}{
		{
			name: "raw URL enabled for embedding",
			config: &Config{
				PlatformType: PlatformOpenAI,
				BaseURL:      "https://custom.api.com/v100#",
				APIKey:       "test-key",
			},
			request: &llm.Request{
				RequestType: llm.RequestTypeEmbedding,
				Model:       "text-embedding-ada-002",
				Embedding: &llm.EmbeddingRequest{
					Input: llm.EmbeddingInput{
						StringArray: []string{"hello", "world"},
					},
				},
			},
			expectedURL: "https://custom.api.com/v100/embeddings",
		},
		{
			name: "raw URL auto-enabled with # suffix for embedding",
			config: &Config{
				PlatformType: PlatformOpenAI,
				BaseURL:      "https://custom.api.com/v20#",
				APIKey:       "test-key",
			},
			request: &llm.Request{
				RequestType: llm.RequestTypeEmbedding,
				Model:       "text-embedding-ada-002",
				Embedding: &llm.EmbeddingRequest{
					Input: llm.EmbeddingInput{
						StringArray: []string{"hello", "world"},
					},
				},
			},
			expectedURL: "https://custom.api.com/v20/embeddings",
		},
		{
			name: "raw URL false for embedding",
			config: &Config{
				PlatformType: PlatformOpenAI,
				BaseURL:      "https://api.openai.com",
				APIKey:       "test-key",
				RawURL:       false,
			},
			request: &llm.Request{
				RequestType: llm.RequestTypeEmbedding,
				Model:       "text-embedding-ada-002",
				Embedding: &llm.EmbeddingRequest{
					Input: llm.EmbeddingInput{
						StringArray: []string{"hello", "world"},
					},
				},
			},
			expectedURL: "https://api.openai.com/v1/embeddings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformerInterface, err := NewOutboundTransformerWithConfig(tt.config)
			if err != nil {
				t.Fatalf("Failed to create transformer: %v", err)
			}

			transformer := transformerInterface.(*OutboundTransformer)

			result, err := transformer.TransformRequest(t.Context(), tt.request)
			if err != nil {
				t.Fatalf("TransformRequest() unexpected error = %v", err)
			}

			if result.URL != tt.expectedURL {
				t.Errorf("Expected URL %s, got %s", tt.expectedURL, result.URL)
			}
		})
	}
}
