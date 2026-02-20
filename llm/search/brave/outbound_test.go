package brave

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
		BaseURL:        "https://api.search.brave.com",
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

	t.Run("nil api key provider", func(t *testing.T) {
		_, err := NewOutboundTransformerWithConfig(&Config{
			BaseURL: "https://api.search.brave.com",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "api key provider is required")
	})

	t.Run("empty base url", func(t *testing.T) {
		_, err := NewOutboundTransformerWithConfig(&Config{
			APIKeyProvider: auth.NewStaticKeyProvider("key"),
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "base url is required")
	})

	t.Run("valid config", func(t *testing.T) {
		tr, err := NewOutboundTransformerWithConfig(&Config{
			BaseURL:        "https://api.search.brave.com",
			APIKeyProvider: auth.NewStaticKeyProvider("key"),
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

	t.Run("nil search", func(t *testing.T) {
		_, err := tr.TransformRequest(ctx, &llm.Request{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "search request is nil")
	})

	t.Run("empty query", func(t *testing.T) {
		_, err := tr.TransformRequest(ctx, &llm.Request{
			Search: &llm.SearchRequest{Query: "   "},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "query is required")
	})

	t.Run("basic request defaults", func(t *testing.T) {
		req, err := tr.TransformRequest(ctx, &llm.Request{
			Search: &llm.SearchRequest{Query: "hello"},
		})
		require.NoError(t, err)
		require.Equal(t, http.MethodGet, req.Method)
		require.Contains(t, req.URL, "/res/v1/web/search")
		require.Nil(t, req.Body)
		require.Equal(t, "hello", req.Query.Get("q"))
		require.Equal(t, "10", req.Query.Get("count"))
		require.NotNil(t, req.Auth)
		require.Equal(t, httpclient.AuthTypeAPIKey, req.Auth.Type)
		require.Equal(t, "X-Subscription-Token", req.Auth.HeaderKey)
		require.Equal(t, "test-key", req.Auth.APIKey)
		require.True(t, req.SkipInboundQueryMerge)
		require.Equal(t, llm.RequestTypeSearch.String(), req.RequestType)
		require.Equal(t, llm.APIFormatBraveSearch.String(), req.APIFormat)
	})

	t.Run("with max results", func(t *testing.T) {
		req, err := tr.TransformRequest(ctx, &llm.Request{
			Search: &llm.SearchRequest{
				Query:      "hello",
				MaxResults: lo.ToPtr(3),
			},
		})
		require.NoError(t, err)
		require.Equal(t, "3", req.Query.Get("count"))
	})

	t.Run("with allowed domains", func(t *testing.T) {
		req, err := tr.TransformRequest(ctx, &llm.Request{
			Search: &llm.SearchRequest{
				Query:          "hello",
				AllowedDomains: []string{"example.com", "test.com"},
			},
		})
		require.NoError(t, err)
		q := req.Query.Get("q")
		require.Contains(t, q, "(site:example.com OR site:test.com)")
	})

	t.Run("with blocked domains", func(t *testing.T) {
		req, err := tr.TransformRequest(ctx, &llm.Request{
			Search: &llm.SearchRequest{
				Query:          "hello",
				BlockedDomains: []string{"bad.com"},
			},
		})
		require.NoError(t, err)
		q := req.Query.Get("q")
		require.Contains(t, q, "-site:bad.com")
	})

	t.Run("with both allowed and blocked domains", func(t *testing.T) {
		req, err := tr.TransformRequest(ctx, &llm.Request{
			Search: &llm.SearchRequest{
				Query:          "hello",
				AllowedDomains: []string{"example.com"},
				BlockedDomains: []string{"bad.com"},
			},
		})
		require.NoError(t, err)
		q := req.Query.Get("q")
		require.Contains(t, q, "site:example.com")
		require.Contains(t, q, "-site:bad.com")
	})

	t.Run("with extra body query params", func(t *testing.T) {
		extra, err := json.Marshal(map[string]any{
			"country":   "US",
			"freshness": "pw",
		})
		require.NoError(t, err)

		req, err := tr.TransformRequest(ctx, &llm.Request{
			Search: &llm.SearchRequest{
				Query:     "hello",
				ExtraBody: extra,
			},
		})
		require.NoError(t, err)
		require.Equal(t, "US", req.Query.Get("country"))
		require.Equal(t, "pw", req.Query.Get("freshness"))
	})

	t.Run("transformer metadata contains query", func(t *testing.T) {
		req, err := tr.TransformRequest(ctx, &llm.Request{
			Search: &llm.SearchRequest{Query: "original query"},
		})
		require.NoError(t, err)
		require.NotNil(t, req.TransformerMetadata)
		require.Equal(t, "original query", req.TransformerMetadata["query"])
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

	t.Run("error status 429", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"message": "rate limit exceeded",
				"type":    "rate_limit",
			},
		})
		_, err := tr.TransformResponse(ctx, &httpclient.Response{
			StatusCode: 429,
			Body:       body,
		})
		require.Error(t, err)
	})

	t.Run("empty body", func(t *testing.T) {
		_, err := tr.TransformResponse(ctx, &httpclient.Response{
			StatusCode: 200,
			Body:       nil,
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "response body is empty")
	})

	t.Run("valid response", func(t *testing.T) {
		body, err := json.Marshal(WebSearchResponse{
			Web: struct {
				Results []WebResult `json:"results"`
			}{
				Results: []WebResult{
					{
						Title:       "Test Title",
						URL:         "https://example.com",
						Description: "Test description",
						Age:         "2 days ago",
						Favicon:     "https://example.com/favicon.ico",
					},
				},
			},
		})
		require.NoError(t, err)

		resp, err := tr.TransformResponse(ctx, &httpclient.Response{
			StatusCode: 200,
			Body:       body,
			Request: &httpclient.Request{
				TransformerMetadata: map[string]any{"query": "my query"},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, resp.Search)
		require.Equal(t, llm.RequestTypeSearch, resp.RequestType)
		require.Equal(t, llm.APIFormatBraveSearch, resp.APIFormat)
		require.Equal(t, "my query", resp.Search.Query)
		require.Len(t, resp.Search.Results, 1)

		r := resp.Search.Results[0]
		require.Equal(t, "Test Title", r.Title)
		require.Equal(t, "https://example.com", r.URL)
		require.Equal(t, "Test description", r.Content)
		require.Equal(t, "2 days ago", r.PublishedDate)
		require.Equal(t, "https://example.com/favicon.ico", r.Favicon)
	})

	t.Run("query from transformer metadata", func(t *testing.T) {
		body, _ := json.Marshal(WebSearchResponse{})
		resp, err := tr.TransformResponse(ctx, &httpclient.Response{
			StatusCode: 200,
			Body:       body,
			Request: &httpclient.Request{
				TransformerMetadata: map[string]any{"query": "recovered"},
			},
		})
		require.NoError(t, err)
		require.Equal(t, "recovered", resp.Search.Query)
	})
}

func TestOutboundTransformer_TransformError(t *testing.T) {
	tr := newTestTransformer(t)
	ctx := context.Background()

	t.Run("nil error", func(t *testing.T) {
		resp := tr.TransformError(ctx, nil)
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		require.Equal(t, "Internal Server Error", resp.Detail.Message)
	})

	t.Run("structured error body", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"message": "rate limit",
				"type":    "rate_limit",
			},
		})
		resp := tr.TransformError(ctx, &httpclient.Error{
			StatusCode: 429,
			Body:       body,
		})
		require.Equal(t, 429, resp.StatusCode)
		require.Equal(t, "rate limit", resp.Detail.Message)
		require.Equal(t, "rate_limit", resp.Detail.Type)
	})

	t.Run("non-json body fallback", func(t *testing.T) {
		resp := tr.TransformError(ctx, &httpclient.Error{
			StatusCode: 503,
			Body:       []byte("service unavailable"),
		})
		require.Equal(t, 503, resp.StatusCode)
		require.Equal(t, http.StatusText(503), resp.Detail.Message)
	})
}

func TestOutboundTransformer_TransformStream(t *testing.T) {
	tr := newTestTransformer(t)
	_, err := tr.TransformStream(context.Background(), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "brave search does not support streaming")
}

func TestOutboundTransformer_AggregateStreamChunks(t *testing.T) {
	tr := newTestTransformer(t)
	_, _, err := tr.AggregateStreamChunks(context.Background(), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "brave search does not support streaming")
}
