package exa

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
)

func newTestTransformer(t *testing.T) *OutboundTransformer {
	t.Helper()
	tr, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://api.exa.ai",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)
	return tr
}

func TestNewOutboundTransformerWithConfig(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		_, err := NewOutboundTransformerWithConfig(nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "config cannot be nil")
	})

	t.Run("nil APIKeyProvider", func(t *testing.T) {
		_, err := NewOutboundTransformerWithConfig(&Config{
			BaseURL: "https://api.exa.ai",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "api key provider is required")
	})

	t.Run("empty BaseURL", func(t *testing.T) {
		_, err := NewOutboundTransformerWithConfig(&Config{
			APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "base url is required")
	})

	t.Run("valid config", func(t *testing.T) {
		tr, err := NewOutboundTransformerWithConfig(&Config{
			BaseURL:        "https://api.exa.ai",
			APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
		})
		require.NoError(t, err)
		require.NotNil(t, tr)
	})
}

func TestOutboundTransformer_TransformRequest(t *testing.T) {
	tr := newTestTransformer(t)
	ctx := context.Background()

	t.Run("nil request", func(t *testing.T) {
		_, err := tr.TransformRequest(ctx, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "search request is nil")
	})

	t.Run("nil Search", func(t *testing.T) {
		_, err := tr.TransformRequest(ctx, &llm.Request{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "search request is nil")
	})

	t.Run("empty query", func(t *testing.T) {
		_, err := tr.TransformRequest(ctx, &llm.Request{
			Search: &llm.SearchRequest{Query: ""},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "query is required")
	})

	t.Run("basic request", func(t *testing.T) {
		req, err := tr.TransformRequest(ctx, &llm.Request{
			Search: &llm.SearchRequest{Query: "hello"},
		})
		require.NoError(t, err)
		require.Equal(t, http.MethodPost, req.Method)
		require.Contains(t, req.URL, "/search")
		require.Equal(t, "application/json", req.Headers.Get("Content-Type"))

		var body SearchRequest
		require.NoError(t, json.Unmarshal(req.Body, &body))
		require.Equal(t, "hello", body.Query)
		require.Equal(t, 5, body.NumResults)

		require.NotNil(t, req.Auth)
		require.Equal(t, httpclient.AuthTypeBearer, req.Auth.Type)
	})

	t.Run("with MaxResults", func(t *testing.T) {
		req, err := tr.TransformRequest(ctx, &llm.Request{
			Search: &llm.SearchRequest{
				Query:      "hello",
				MaxResults: lo.ToPtr(3),
			},
		})
		require.NoError(t, err)

		var body SearchRequest
		require.NoError(t, json.Unmarshal(req.Body, &body))
		require.Equal(t, 3, body.NumResults)
	})

	t.Run("with AllowedDomains and BlockedDomains", func(t *testing.T) {
		req, err := tr.TransformRequest(ctx, &llm.Request{
			Search: &llm.SearchRequest{
				Query:          "hello",
				AllowedDomains: []string{"example.com", "test.com"},
				BlockedDomains: []string{"blocked.com"},
			},
		})
		require.NoError(t, err)

		var body SearchRequest
		require.NoError(t, json.Unmarshal(req.Body, &body))
		require.Equal(t, []string{"example.com", "test.com"}, body.IncludeDomains)
		require.Equal(t, []string{"blocked.com"}, body.ExcludeDomains)
	})

	t.Run("with ExtraBody", func(t *testing.T) {
		req, err := tr.TransformRequest(ctx, &llm.Request{
			Search: &llm.SearchRequest{
				Query:     "hello",
				ExtraBody: json.RawMessage(`{"type":"keyword"}`),
			},
		})
		require.NoError(t, err)

		var body map[string]any
		require.NoError(t, json.Unmarshal(req.Body, &body))
		require.Equal(t, "keyword", body["type"])
		require.Equal(t, "hello", body["query"])
	})

	t.Run("TransformerMetadata contains query", func(t *testing.T) {
		req, err := tr.TransformRequest(ctx, &llm.Request{
			Search: &llm.SearchRequest{Query: "my search query"},
		})
		require.NoError(t, err)
		require.NotNil(t, req.TransformerMetadata)
		require.Equal(t, "my search query", req.TransformerMetadata["query"])
	})
}

func TestOutboundTransformer_TransformResponse(t *testing.T) {
	tr := newTestTransformer(t)
	ctx := context.Background()

	t.Run("nil response", func(t *testing.T) {
		_, err := tr.TransformResponse(ctx, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "http response is nil")
	})

	t.Run("status 401 error", func(t *testing.T) {
		_, err := tr.TransformResponse(ctx, &httpclient.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       []byte(`{"message":"unauthorized"}`),
		})
		require.Error(t, err)
	})

	t.Run("empty body", func(t *testing.T) {
		_, err := tr.TransformResponse(ctx, &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       nil,
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "response body is empty")
	})

	t.Run("valid response with results and CostDollars", func(t *testing.T) {
		score := 0.95
		exaResp := SearchResponse{
			Results: []SearchResult{
				{
					Title:         "Test Title",
					URL:           "https://example.com",
					Text:          "Some content",
					Score:         &score,
					PublishedDate: "2025-01-01",
					Favicon:       "https://example.com/favicon.ico",
				},
			},
		}
		body, err := json.Marshal(exaResp)
		require.NoError(t, err)

		resp, err := tr.TransformResponse(ctx, &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       body,
			Request: &httpclient.Request{
				TransformerMetadata: map[string]any{"query": "test query"},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, resp.Search)
		require.Len(t, resp.Search.Results, 1)

		result := resp.Search.Results[0]
		require.Equal(t, "Test Title", result.Title)
		require.Equal(t, "https://example.com", result.URL)
		require.Equal(t, "Some content", result.Content)
		require.NotNil(t, result.Score)
		require.InDelta(t, 0.95, *result.Score, 0.001)
		require.Equal(t, "2025-01-01", result.PublishedDate)
		require.Equal(t, "https://example.com/favicon.ico", result.Favicon)

		require.NotNil(t, resp.Usage)
		require.Equal(t, int64(1), resp.Usage.Quantity)

		require.Equal(t, llm.RequestTypeSearch, resp.RequestType)
		require.Equal(t, llm.APIFormatExaSearch, resp.APIFormat)
	})

	t.Run("valid response without CostDollars", func(t *testing.T) {
		exaResp := SearchResponse{
			Results: []SearchResult{
				{Title: "No Cost", URL: "https://example.com", Text: "content"},
			},
		}
		body, err := json.Marshal(exaResp)
		require.NoError(t, err)

		resp, err := tr.TransformResponse(ctx, &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       body,
		})
		require.NoError(t, err)
		require.NotNil(t, resp.Usage)
		require.Equal(t, int64(1), resp.Usage.Quantity)
	})

	t.Run("query from TransformerMetadata", func(t *testing.T) {
		exaResp := SearchResponse{Results: []SearchResult{}}
		body, err := json.Marshal(exaResp)
		require.NoError(t, err)

		resp, err := tr.TransformResponse(ctx, &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       body,
			Request: &httpclient.Request{
				TransformerMetadata: map[string]any{"query": "recovered query"},
			},
		})
		require.NoError(t, err)
		require.Equal(t, "recovered query", resp.Search.Query)
	})
}

func TestOutboundTransformer_TransformError(t *testing.T) {
	tr := newTestTransformer(t)
	ctx := context.Background()

	t.Run("nil error", func(t *testing.T) {
		respErr := tr.TransformError(ctx, nil)
		require.Equal(t, http.StatusInternalServerError, respErr.StatusCode)
		require.Equal(t, http.StatusText(http.StatusInternalServerError), respErr.Detail.Message)
	})

	t.Run("top-level message", func(t *testing.T) {
		respErr := tr.TransformError(ctx, &httpclient.Error{
			StatusCode: http.StatusTooManyRequests,
			Body:       []byte(`{"message":"rate limit exceeded"}`),
		})
		require.Equal(t, http.StatusTooManyRequests, respErr.StatusCode)
		require.Equal(t, "rate limit exceeded", respErr.Detail.Message)
	})

	t.Run("nested error message", func(t *testing.T) {
		respErr := tr.TransformError(ctx, &httpclient.Error{
			StatusCode: http.StatusUnauthorized,
			Body:       []byte(`{"error":{"message":"invalid key","type":"auth"}}`),
		})
		require.Equal(t, http.StatusUnauthorized, respErr.StatusCode)
		require.Equal(t, "invalid key", respErr.Detail.Message)
	})

	t.Run("non-JSON body", func(t *testing.T) {
		respErr := tr.TransformError(ctx, &httpclient.Error{
			StatusCode: http.StatusBadGateway,
			Body:       []byte(`not json`),
		})
		require.Equal(t, http.StatusBadGateway, respErr.StatusCode)
		require.Equal(t, http.StatusText(http.StatusBadGateway), respErr.Detail.Message)
	})
}

func TestOutboundTransformer_TransformStream(t *testing.T) {
	tr := newTestTransformer(t)
	_, err := tr.TransformStream(context.Background(), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exa search does not support streaming")
}

func TestOutboundTransformer_AggregateStreamChunks(t *testing.T) {
	tr := newTestTransformer(t)
	_, _, err := tr.AggregateStreamChunks(context.Background(), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exa search does not support streaming")
}
