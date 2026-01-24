package provider_quota

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
)

type ClaudeCodeQuotaChecker struct{}

func NewClaudeCodeQuotaChecker() *ClaudeCodeQuotaChecker {
	return &ClaudeCodeQuotaChecker{}
}

func (c *ClaudeCodeQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (http.Header, []byte, error) {
	// Verify credentials
	if ch.Credentials == nil {
		return nil, nil, fmt.Errorf("channel has no credentials")
	}

	// Parse OAuth credentials from apiKey JSON
	var accessToken string
	if ch.Credentials.OAuth != nil {
		accessToken = ch.Credentials.OAuth.AccessToken
	} else if strings.TrimSpace(ch.Credentials.APIKey) != "" {
		creds, err := oauth.ParseCredentialsJSON(ch.Credentials.APIKey)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse OAuth credentials: %w", err)
		}
		accessToken = creds.AccessToken
	}

	if accessToken == "" {
		return nil, nil, fmt.Errorf("channel credentials missing access token")
	}

	// Build HTTP request using Bearer auth like ClaudeCode transformers
	httpRequest := httpclient.NewRequestBuilder().
		WithMethod("POST").
		WithURL(getEndpointURL(ch.BaseURL)).
		WithAuth(&httpclient.AuthConfig{
			Type:   httpclient.AuthTypeBearer,
			APIKey: accessToken,
		}).
		WithHeader("anthropic-beta", "claude-code-20250219,oauth-2025-04-20,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14").
		WithHeader("anthropic-version", "2023-06-01").
		WithHeader("anthropic-dangerous-direct-browser-access", "true").
		WithHeader("x-app", "cli").
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
		return nil, nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	// Check HTTP status
	if httpResponse.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("HTTP %d: %s", httpResponse.StatusCode, string(httpResponse.Body))
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
	// If baseURL already ends with /v1 or /v1/, append /messages
	isV1 := false
	if len(baseURL) > 3 && baseURL[len(baseURL)-3:] == "/v1" {
		isV1 = true
	} else if len(baseURL) > 4 && baseURL[len(baseURL)-4:] == "/v1/" {
		isV1 = true
	}

	if isV1 {
		if baseURL[len(baseURL)-1] != '/' {
			return baseURL + "/messages"
		}
		return baseURL + "messages"
	}
	// Otherwise append /v1/messages
	if len(baseURL) > 0 && baseURL[len(baseURL)-1] != '/' {
		return baseURL + "/v1/messages"
	}
	return baseURL + "v1/messages"
}
