package provider_quota

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
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
	if ch.Credentials == nil || ch.Credentials.OAuth == nil {
		return nil, nil, fmt.Errorf("codex channel missing OAuth credentials")
	}

	oauth := ch.Credentials.OAuth

	// Extract chatgpt_account_id from id_token JWT
	accountID := codex.ExtractChatGPTAccountIDFromJWT(oauth.IDToken)
	if accountID == "" {
		return nil, nil, fmt.Errorf("failed to extract account ID from id_token")
	}

	// Build request
	req, err := http.NewRequestWithContext(ctx, "GET", "https://chatgpt.com/backend-api/wham/usage", nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", oauth.AccessToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "codex_cli_rs/0.76.0 (Debian 13.0.0; x86_64) WindowsTerminal")
	req.Header.Set("Chatgpt-Account-Id", accountID)

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("quota request failed: %w", err)
	}
	defer resp.Body.Close()

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
