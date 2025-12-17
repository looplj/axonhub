package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
)

func TestRerank_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/rerank")
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Verify request body
		var req llm.RerankRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "test-model", req.Model)
		assert.Equal(t, "test query", req.Query)
		assert.Equal(t, []string{"doc1", "doc2"}, req.Documents)

		// Return mock response
		resp := llm.RerankResponse{
			Results: []llm.RerankResult{
				{Index: 0, RelevanceScore: 0.9, Document: "doc1"},
				{Index: 1, RelevanceScore: 0.5, Document: "doc2"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create transformer
	outbound, err := NewOutboundTransformerWithConfig(&Config{
		Type:    PlatformOpenAI,
		BaseURL: server.URL,
		APIKey:  "test-key",
	})
	require.NoError(t, err)

	rerankTransformer, ok := outbound.(transformer.Transformer)
	require.True(t, ok, "transformer should implement Transformer interface")

	// Execute rerank
	req := &llm.RerankRequest{
		Model:     "test-model",
		Query:     "test query",
		Documents: []string{"doc1", "doc2"},
	}

	resp, err := rerankTransformer.Rerank(context.Background(), req, nil)
	require.NoError(t, err)
	assert.Len(t, resp.Results, 2)
	assert.Equal(t, 0, resp.Results[0].Index)
	assert.Equal(t, 0.9, resp.Results[0].RelevanceScore)
}

func TestRerank_ValidationErrors(t *testing.T) {
	outbound, err := NewOutboundTransformerWithConfig(&Config{
		Type:    PlatformOpenAI,
		BaseURL: "http://localhost",
		APIKey:  "test-key",
	})
	require.NoError(t, err)

	rerankTransformer, ok := outbound.(transformer.Transformer)
	require.True(t, ok)

	tests := []struct {
		name    string
		req     *llm.RerankRequest
		wantErr string
	}{
		{
			name:    "nil request",
			req:     nil,
			wantErr: "rerank request is nil",
		},
		{
			name:    "empty model",
			req:     &llm.RerankRequest{Query: "q", Documents: []string{"d"}},
			wantErr: "model is required",
		},
		{
			name:    "empty query",
			req:     &llm.RerankRequest{Model: "m", Documents: []string{"d"}},
			wantErr: "query is required",
		},
		{
			name:    "empty documents",
			req:     &llm.RerankRequest{Model: "m", Query: "q", Documents: []string{}},
			wantErr: "documents are required",
		},
		{
			name:    "top_n zero",
			req:     &llm.RerankRequest{Model: "m", Query: "q", Documents: []string{"d"}, TopN: lo.ToPtr(0)},
			wantErr: "top_n must be a positive integer",
		},
		{
			name:    "top_n negative",
			req:     &llm.RerankRequest{Model: "m", Query: "q", Documents: []string{"d"}, TopN: lo.ToPtr(-1)},
			wantErr: "top_n must be a positive integer",
		},
		{
			name:    "top_n exceeds documents",
			req:     &llm.RerankRequest{Model: "m", Query: "q", Documents: []string{"d1", "d2"}, TopN: lo.ToPtr(5)},
			wantErr: "top_n (5) cannot exceed the number of documents (2)",
		},
		{
			name:    "empty document string",
			req:     &llm.RerankRequest{Model: "m", Query: "q", Documents: []string{"d1", ""}},
			wantErr: "document at index 1 is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := rerankTransformer.Rerank(context.Background(), tt.req, nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestRerank_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "invalid model"}`))
	}))
	defer server.Close()

	outbound, err := NewOutboundTransformerWithConfig(&Config{
		Type:    PlatformOpenAI,
		BaseURL: server.URL,
		APIKey:  "test-key",
	})
	require.NoError(t, err)

	rerankTransformer, ok := outbound.(transformer.Transformer)
	require.True(t, ok)

	req := &llm.RerankRequest{
		Model:     "invalid-model",
		Query:     "test query",
		Documents: []string{"doc1"},
	}

	_, err = rerankTransformer.Rerank(context.Background(), req, nil)
	require.Error(t, err)

	// Check that the error contains status code
	var rerankErr *RerankError
	require.ErrorAs(t, err, &rerankErr)
	assert.Equal(t, http.StatusBadRequest, rerankErr.StatusCode)
}

func TestRerank_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	outbound, err := NewOutboundTransformerWithConfig(&Config{
		Type:    PlatformOpenAI,
		BaseURL: server.URL,
		APIKey:  "test-key",
	})
	require.NoError(t, err)

	rerankTransformer, ok := outbound.(transformer.Transformer)
	require.True(t, ok)

	req := &llm.RerankRequest{
		Model:     "test-model",
		Query:     "test query",
		Documents: []string{"doc1"},
	}

	_, err = rerankTransformer.Rerank(context.Background(), req, nil)
	require.Error(t, err)

	var rerankErr *RerankError
	require.ErrorAs(t, err, &rerankErr)
	assert.Equal(t, http.StatusInternalServerError, rerankErr.StatusCode)
}

func TestRerank_WithTopN(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req llm.RerankRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Verify top_n is passed
		assert.NotNil(t, req.TopN)
		assert.Equal(t, 2, *req.TopN)

		resp := llm.RerankResponse{
			Results: []llm.RerankResult{
				{Index: 0, RelevanceScore: 0.9},
				{Index: 1, RelevanceScore: 0.8},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	outbound, err := NewOutboundTransformerWithConfig(&Config{
		Type:    PlatformOpenAI,
		BaseURL: server.URL,
		APIKey:  "test-key",
	})
	require.NoError(t, err)

	rerankTransformer, ok := outbound.(transformer.Transformer)
	require.True(t, ok)

	req := &llm.RerankRequest{
		Model:     "test-model",
		Query:     "test query",
		Documents: []string{"doc1", "doc2", "doc3"},
		TopN:      lo.ToPtr(2),
	}

	resp, err := rerankTransformer.Rerank(context.Background(), req, nil)
	require.NoError(t, err)
	assert.Len(t, resp.Results, 2)
}

func TestBuildRerankURL(t *testing.T) {
	tests := []struct {
		name       string
		baseURL    string
		platform   PlatformType
		apiVersion string
		want       string
		wantErr    bool
	}{
		{
			name:     "standard URL without trailing slash",
			baseURL:  "https://api.example.com",
			platform: PlatformOpenAI,
			want:     "https://api.example.com/v1/rerank",
		},
		{
			name:     "URL ending with /v1",
			baseURL:  "https://api.example.com/v1",
			platform: PlatformOpenAI,
			want:     "https://api.example.com/v1/rerank",
		},
		{
			name:       "Azure platform",
			baseURL:    "https://myresource.azure.com",
			platform:   PlatformAzure,
			apiVersion: "2024-01-01",
			want:       "https://myresource.azure.com/rerank?api-version=2024-01-01",
		},
		{
			name:     "Azure without API version",
			baseURL:  "https://myresource.azure.com",
			platform: PlatformAzure,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trans := &OutboundTransformer{
				config: &Config{
					Type:       tt.platform,
					BaseURL:    tt.baseURL,
					APIVersion: tt.apiVersion,
				},
			}

			url, err := trans.buildRerankURL()
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, url)
		})
	}
}
