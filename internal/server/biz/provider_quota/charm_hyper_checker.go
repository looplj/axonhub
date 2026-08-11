package provider_quota

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
)

// CharmHyper default baseline for status thresholds.
const charmHyperBaseline = 100.0

// CharmHyperCreditsResponse represents the response from GET /v1/credits.
type CharmHyperCreditsResponse struct {
	Balance float64 `json:"balance"`
}

// CharmHyperQuotaChecker checks quota for Charm Hyper channels.
type CharmHyperQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

// NewCharmHyperQuotaChecker creates a new CharmHyperQuotaChecker.
func NewCharmHyperQuotaChecker(httpClient *httpclient.HttpClient) *CharmHyperQuotaChecker {
	return &CharmHyperQuotaChecker{httpClient: httpClient}
}

// CheckQuota checks the Charm Hyper credit balance for the given channel.
func (c *CharmHyperQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	apiKey := c.extractAPIKey(ch)
	if apiKey == "" {
		return QuotaData{}, fmt.Errorf("missing API key for Charm Hyper channel")
	}

	baseURL := ch.BaseURL
	if baseURL == "" {
		baseURL = "https://hyper.charm.land"
	}
	quotaURL := buildCharmHyperQuotaURL(baseURL)

	httpRequest := httpclient.NewRequestBuilder().
		WithMethod("GET").
		WithURL(quotaURL).
		WithBearerToken(apiKey).
		WithHeader("Accept", "application/json").
		Build()

	hc := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = c.httpClient.WithProxy(ch.Settings.Proxy)
	}

	resp, err := hc.Do(ctx, httpRequest)
	if err != nil {
		return QuotaData{}, fmt.Errorf("executing Charm Hyper quota request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return QuotaData{}, fmt.Errorf("Charm Hyper quota API returned status %d: %s", resp.StatusCode, string(resp.Body))
	}

	return c.parseResponse(resp.Body)
}

// parseResponse parses the API response body into QuotaData.
func (c *CharmHyperQuotaChecker) parseResponse(body []byte) (QuotaData, error) {
	var resp CharmHyperCreditsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return QuotaData{}, fmt.Errorf("parsing Charm Hyper quota response: %w", err)
	}

	status, ready, usageRatio := c.computeStatus(resp.Balance)

	return QuotaData{
		Status:       status,
		ProviderType: "charm_hyper",
		RawData:      map[string]any{"balance": resp.Balance},
		NextResetAt:  nil,
		Ready:        ready,
		Limits: []QuotaLimitStatus{
			NewTokenLimitStatus(status, usageRatio, nil),
		},
	}, nil
}

// computeStatus determines status, ready, and usage ratio from the balance.
func (c *CharmHyperQuotaChecker) computeStatus(balance float64) (status string, ready bool, usageRatio float64) {
	usageRatio = 1.0 - balance/charmHyperBaseline
	if usageRatio < 0 {
		usageRatio = 0
	}

	if balance == 0 {
		return "exhausted", false, 1.0
	}
	if balance <= 20 {
		return "warning", true, usageRatio
	}
	return "available", true, usageRatio
}

// SupportsChannel checks if the checker supports the given channel.
func (c *CharmHyperQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	if ch.Type != channel.TypeOpenai && ch.Type != channel.TypeOpenaiResponses {
		return false
	}
	return DetectProviderFromURL(ch.BaseURL) == "charm_hyper"
}

// extractAPIKey extracts the API key from channel credentials.
func (c *CharmHyperQuotaChecker) extractAPIKey(ch *ent.Channel) string {
	if ch.Credentials.APIKey != "" {
		return ch.Credentials.APIKey
	}
	if len(ch.Credentials.APIKeys) > 0 {
		return ch.Credentials.APIKeys[0]
	}
	return ""
}

// buildCharmHyperQuotaURL builds the full quota URL from the base URL.
func buildCharmHyperQuotaURL(baseURL string) string {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "https://hyper.charm.land/v1/credits"
	}
	parsed.Path = "/v1/credits"
	parsed.RawQuery = ""
	parsed.User = nil
	parsed.Fragment = ""
	return parsed.String()
}
