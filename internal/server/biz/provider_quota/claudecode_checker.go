package provider_quota

import (
	"context"
	"fmt"
	"net/http"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
)

type ClaudeCodeQuotaChecker struct{}

func NewClaudeCodeQuotaChecker() *ClaudeCodeQuotaChecker {
	return &ClaudeCodeQuotaChecker{}
}

func (c *ClaudeCodeQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (http.Header, []byte, error) {
	// Verify credentials
	if ch.Credentials == nil || ch.Credentials.APIKey == "" {
		return nil, nil, fmt.Errorf("claudecode channel missing API key")
	}

	// Build HTTP request directly
	httpRequest := httpclient.NewRequestBuilder().
		WithMethod("POST").
		WithURL(getEndpointURL(ch.BaseURL)).
		WithHeader("x-api-key", ch.Credentials.APIKey).
		WithHeader("anthropic-version", "2023-06-01").
		WithHeader("content-type", "application/json").
		WithBody(map[string]interface{}{
			"model": "claude-haiku-4-5",
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "limit",
				},
			},
			"max_tokens": 1,
		}).
		Build()

	// Execute HTTP request
	httpClient := httpclient.NewHttpClient()

	httpResponse, err := httpClient.Do(ctx, httpRequest)
	if err != nil {
		return nil, nil, fmt.Errorf("quota check request failed: %w", err)
	}

	// Return raw headers and body
	return httpResponse.Headers, httpResponse.Body, nil
}

func (c *ClaudeCodeQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	return ch.Type == channel.TypeClaudecode
}

func getEndpointURL(baseURL string) string {
	if baseURL == "" {
		return "https://api.anthropic.com/v1/messages"
	}
	// Ensure it ends with the messages endpoint
	if len(baseURL) > 0 && baseURL[len(baseURL)-1] != '/' {
		return baseURL + "/v1/messages"
	}
	return baseURL + "v1/messages"
}
