package search

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestInboundTransformer_TransformRequest(t *testing.T) {
	tr := NewInboundTransformer()

	t.Run("defaults", func(t *testing.T) {
		body, err := json.Marshal(map[string]any{
			"query": "hello",
		})
		require.NoError(t, err)

		httpReq := &httpclient.Request{
			Body: body,
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
		}

		llmReq, err := tr.TransformRequest(context.Background(), httpReq)
		require.NoError(t, err)
		require.Equal(t, "__search", llmReq.Model)
		require.Equal(t, llm.RequestTypeSearch, llmReq.RequestType)
		require.Equal(t, llm.APIFormatTavilySearch, llmReq.APIFormat)
		require.NotNil(t, llmReq.Search)
		require.Equal(t, "hello", llmReq.Search.Query)
		require.NotNil(t, llmReq.Search.MaxResults)
		require.Equal(t, 10, *llmReq.Search.MaxResults)
	})

	t.Run("explicit", func(t *testing.T) {
		body, err := json.Marshal(map[string]any{
			"query":       "q",
			"model":       "__tavily_search",
			"max_results": 10,
			"allowed_domains": []string{
				"example.com",
			},
			"blocked_domains": []string{"pinterest.com"},
			"extra_body": map[string]any{
				"search_depth": "advanced",
			},
		})
		require.NoError(t, err)

		httpReq := &httpclient.Request{Body: body}

		llmReq, err := tr.TransformRequest(context.Background(), httpReq)
		require.NoError(t, err)
		require.Equal(t, "__tavily_search", llmReq.Model)
		require.Equal(t, 10, *llmReq.Search.MaxResults)
		require.Equal(t, []string{"example.com"}, llmReq.Search.AllowedDomains)
		require.Equal(t, []string{"pinterest.com"}, llmReq.Search.BlockedDomains)
		require.Contains(t, string(llmReq.Search.ExtraBody), "search_depth")
	})

	t.Run("missing query", func(t *testing.T) {
		body, err := json.Marshal(map[string]any{
			"model": "__search",
		})
		require.NoError(t, err)

		_, err = tr.TransformRequest(context.Background(), &httpclient.Request{Body: body})
		require.Error(t, err)
		require.Contains(t, err.Error(), "query is required")
	})

	t.Run("invalid max_results", func(t *testing.T) {
		body, err := json.Marshal(map[string]any{
			"query":       "q",
			"max_results": 30,
		})
		require.NoError(t, err)

		_, err = tr.TransformRequest(context.Background(), &httpclient.Request{Body: body})
		require.Error(t, err)
		require.Contains(t, err.Error(), "max_results")
	})
}
