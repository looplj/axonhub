package tavily

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
		BaseURL:        "https://api.tavily.com",
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
			BaseURL: "https://api.tavily.com",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "api key provider is required")
	})

	t.Run("empty BaseURL", func(t *testing.T) {
		_, err := NewOutboundTransformerWithConfig(&Config{
			APIKeyProvider: auth.NewStaticKeyProvider("key"),
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "base url is required")
	})

	t.Run("valid config", func(t *testing.T) {
		tr, err := NewOutboundTransformerWithConfig(&Config{
			BaseURL:        "https://api.tavily.com",
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
		require.Equal(t, httpclient.AuthTypeBearer, req.Auth.Type)

		var body map[string]any
		require.NoError(t, json.Unmarshal(req.Body, &body))
		require.Equal(t, "hello", body["query"])
		require.EqualValues(t, 5, body["max_results"])
	})

	t.Run("with max_results", func(t *testing.T) {
		req, err := tr.TransformRequest(ctx, &llm.Request{
			Search: &llm.SearchRequest{
				Query:      "hello",
				MaxResults: lo.ToPtr(3),
			},
		})
		require.NoError(t, err)

		var body map[string]any
		require.NoError(t, json.Unmarshal(req.Body, &body))
		require.EqualValues(t, 3, body["max_results"])
	})

	t.Run("with domains", func(t *testing.T) {
		req, err := tr.TransformRequest(ctx, &llm.Request{
			Search: &llm.SearchRequest{
				Query:          "hello",
				AllowedDomains: []string{"example.com"},
				BlockedDomains: []string{"pinterest.com"},
			},
		})
		require.NoError(t, err)

		var body map[string]any
		require.NoError(t, json.Unmarshal(req.Body, &body))
		require.Equal(t, []any{"example.com"}, body["include_domains"])
		require.Equal(t, []any{"pinterest.com"}, body["exclude_domains"])
	})

	t.Run("with extra_body", func(t *testing.T) {
		req, err := tr.TransformRequest(ctx, &llm.Request{
			Search: &llm.SearchRequest{
				Query:     "hello",
				ExtraBody: json.RawMessage(`{"search_depth":"advanced"}`),
			},
		})
		require.NoError(t, err)

		var body map[string]any
		require.NoError(t, json.Unmarshal(req.Body, &body))
		require.Equal(t, "advanced", body["search_depth"])
		require.Equal(t, "hello", body["query"])
	})

	t.Run("transformer metadata contains query", func(t *testing.T) {
		req, err := tr.TransformRequest(ctx, &llm.Request{
			Search: &llm.SearchRequest{Query: "my query"},
		})
		require.NoError(t, err)
		require.Equal(t, "my query", req.TransformerMetadata["query"])
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

	t.Run("status 400 with error body", func(t *testing.T) {
		errBody, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"message": "bad request",
				"type":    "invalid_request",
			},
		})
		_, err := tr.TransformResponse(ctx, &httpclient.Response{
			StatusCode: 400,
			Body:       errBody,
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "bad request")
	})

	t.Run("empty body", func(t *testing.T) {
		_, err := tr.TransformResponse(ctx, &httpclient.Response{
			StatusCode: 200,
			Body:       nil,
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "response body is empty")
	})

	t.Run("valid response with results and usage", func(t *testing.T) {
		respBody, _ := json.Marshal(SearchResponse{
			Query: "hello",
			Results: []SearchResult{
				{Title: "Result 1", URL: "https://example.com", Content: "content1", Score: 0.95},
				{Title: "Result 2", URL: "https://example.org", Content: "content2", Score: 0.80},
			},
			ResponseTime: 1.23,
			Usage:        &SearchUsage{Credits: 2},
		})

		resp, err := tr.TransformResponse(ctx, &httpclient.Response{
			StatusCode: 200,
			Body:       respBody,
			Request: &httpclient.Request{
				TransformerMetadata: map[string]any{"query": "metadata-query"},
			},
		})
		require.NoError(t, err)
		require.Equal(t, llm.RequestTypeSearch, resp.RequestType)
		require.Equal(t, llm.APIFormatTavilySearch, resp.APIFormat)
		require.NotNil(t, resp.Search)
		require.Equal(t, "hello", resp.Search.Query)
		require.Len(t, resp.Search.Results, 2)
		require.Equal(t, "Result 1", resp.Search.Results[0].Title)
		require.Equal(t, "https://example.com", resp.Search.Results[0].URL)
		require.Equal(t, "content1", resp.Search.Results[0].Content)
		require.InDelta(t, 0.95, *resp.Search.Results[0].Score, 0.001)
		require.InDelta(t, 1.23, resp.Search.ResponseTime, 0.001)
		require.NotNil(t, resp.Usage)
		require.Equal(t, int64(1), resp.Usage.Quantity)
	})

	t.Run("valid response without usage", func(t *testing.T) {
		respBody, _ := json.Marshal(SearchResponse{
			Query: "hello",
			Results: []SearchResult{
				{Title: "Result 1", URL: "https://example.com", Content: "content1", Score: 0.9},
			},
			ResponseTime: 0.5,
		})

		resp, err := tr.TransformResponse(ctx, &httpclient.Response{
			StatusCode: 200,
			Body:       respBody,
		})
		require.NoError(t, err)
		require.NotNil(t, resp.Usage)
		require.Equal(t, int64(1), resp.Usage.Quantity)
	})

	t.Run("body query takes precedence over metadata", func(t *testing.T) {
		respBody, _ := json.Marshal(SearchResponse{
			Query:        "body-query",
			Results:      []SearchResult{},
			ResponseTime: 0.1,
		})

		resp, err := tr.TransformResponse(ctx, &httpclient.Response{
			StatusCode: 200,
			Body:       respBody,
			Request: &httpclient.Request{
				TransformerMetadata: map[string]any{"query": "metadata-query"},
			},
		})
		require.NoError(t, err)
		require.Equal(t, "body-query", resp.Search.Query)
	})

	t.Run("falls back to metadata query when body query is empty", func(t *testing.T) {
		respBody, _ := json.Marshal(SearchResponse{
			Query:        "",
			Results:      []SearchResult{},
			ResponseTime: 0.1,
		})

		resp, err := tr.TransformResponse(ctx, &httpclient.Response{
			StatusCode: 200,
			Body:       respBody,
			Request: &httpclient.Request{
				TransformerMetadata: map[string]any{"query": "metadata-query"},
			},
		})
		require.NoError(t, err)
		require.Equal(t, "metadata-query", resp.Search.Query)
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

	t.Run("structured error body", func(t *testing.T) {
		errBody, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"message": "rate limit",
				"type":    "rate_limit",
			},
		})
		respErr := tr.TransformError(ctx, &httpclient.Error{
			StatusCode: 429,
			Body:       errBody,
		})
		require.Equal(t, 429, respErr.StatusCode)
		require.Equal(t, "rate limit", respErr.Detail.Message)
		require.Equal(t, "rate_limit", respErr.Detail.Type)
	})

	t.Run("non-JSON body", func(t *testing.T) {
		respErr := tr.TransformError(ctx, &httpclient.Error{
			StatusCode: 502,
			Body:       []byte("bad gateway html"),
		})
		require.Equal(t, 502, respErr.StatusCode)
		require.Equal(t, http.StatusText(502), respErr.Detail.Message)
		require.Equal(t, "api_error", respErr.Detail.Type)
	})
}

func TestOutboundTransformer_TransformStream(t *testing.T) {
	tr := newTestTransformer(t)
	_, err := tr.TransformStream(context.Background(), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "tavily search does not support streaming")
}

func TestOutboundTransformer_AggregateStreamChunks(t *testing.T) {
	tr := newTestTransformer(t)
	_, _, err := tr.AggregateStreamChunks(context.Background(), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "tavily search does not support streaming")
}
