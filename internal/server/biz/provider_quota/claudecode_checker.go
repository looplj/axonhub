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
	"github.com/looplj/axonhub/llm/transformer/anthropic/claudecode"
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
		WithHeader("anthropic-beta", claudecode.ClaudeCodeBetaHeader).
		WithHeader("anthropic-version", claudecode.ClaudeCodeVersionHeader).
		WithHeader("anthropic-dangerous-direct-browser-access", claudecode.ClaudeCodeBrowserAccessHeader).
		WithHeader("x-app", claudecode.ClaudeCodeAppHeader).
		WithHeader("content-type", "application/json").
		WithBody(map[string]interface{}{
			"model": claudecode.ClaudeCodeQuotaCheckModel,
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

	baseURL = strings.TrimSuffix(baseURL, "/")

	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL + "/messages"
	}

	return baseURL + "/v1/messages"
}
