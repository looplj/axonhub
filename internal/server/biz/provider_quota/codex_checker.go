package provider_quota

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
)

type CodexQuotaChecker struct {
	httpClient *http.Client
}

func NewCodexQuotaChecker() *CodexQuotaChecker {
	return &CodexQuotaChecker{
		httpClient: &http.Client{},
	}
}

func (c *CodexQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (http.Header, []byte, error) {
	// Extract OAuth credentials
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
		return nil, nil, fmt.Errorf("OAuth missing access_token")
	}

	// Extract chatgpt_account_id from access_token JWT
	// The access_token contains the account ID in the https://api.openai.com/auth claim
	accountID := codex.ExtractChatGPTAccountIDFromJWT(accessToken)
	if accountID == "" {
		return nil, nil, fmt.Errorf("failed to extract account ID from access_token (invalid JWT format or missing claim)")
	}

	// Build request
	req, err := http.NewRequestWithContext(ctx, "GET", "https://chatgpt.com/backend-api/wham/usage", nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "codex_cli_rs/0.76.0 (Debian 13.0.0; x86_64) WindowsTerminal")
	req.Header.Set("Chatgpt-Account-Id", accountID)

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("quota request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("quota request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return resp.Header, body, nil
}

func (c *CodexQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	return ch.Type == channel.TypeCodex
}
