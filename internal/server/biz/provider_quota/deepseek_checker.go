package provider_quota

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
)

const deepseekDefaultBaseURL = "https://api.deepseek.com"

// DeepSeekBalanceInfo is one currency balance reported by /user/balance.
// The DeepSeek API returns the amounts as strings, but some compatible
// deployments send numbers instead, so the values stay untyped here and are
// parsed with toFloat64.
type DeepSeekBalanceInfo struct {
	Currency        string `json:"currency"`
	TotalBalance    any    `json:"total_balance"`
	GrantedBalance  any    `json:"granted_balance"`
	ToppedUpBalance any    `json:"topped_up_balance"`
}

// DeepSeekBalanceResponse is the response from GET /user/balance.
type DeepSeekBalanceResponse struct {
	IsAvailable  *bool                 `json:"is_available"`
	BalanceInfos []DeepSeekBalanceInfo `json:"balance_infos"`
}

// DeepSeekQuotaChecker reads the account balance for DeepSeek channels.
// DeepSeek bills pay-as-you-go, so there is no quota window to measure: the
// balance itself is the quota information operators need.
type DeepSeekQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

// NewDeepSeekQuotaChecker creates a DeepSeek balance checker.
func NewDeepSeekQuotaChecker(httpClient *httpclient.HttpClient) *DeepSeekQuotaChecker {
	return &DeepSeekQuotaChecker{httpClient: httpClient}
}

// CheckQuota reads the balance of every configured API-key account through the
// shared per-key collector. A funded account keeps the channel available, while
// one empty or failing account cannot exhaust it.
func (c *DeepSeekQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	hc := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = c.httpClient.WithProxy(ch.Settings.Proxy)
	}

	balanceURL := buildDeepSeekBalanceURL(ch.BaseURL)

	return CollectAPIKeyQuotas(ctx, ch, "deepseek", time.Now(), func(ctx context.Context, _ *ent.Channel, apiKey string) (QuotaData, error) {
		return fetchDeepSeekAccountQuota(ctx, hc, balanceURL, apiKey)
	})
}

// fetchDeepSeekAccountQuota reads and normalizes one account's balance. The
// provider's is_available flag stays authoritative, and the raw balance fields
// are kept for the account's own detail view.
func fetchDeepSeekAccountQuota(
	ctx context.Context,
	hc *httpclient.HttpClient,
	balanceURL string,
	apiKey string,
) (QuotaData, error) {
	response, err := fetchDeepSeekBalance(ctx, hc, balanceURL, apiKey)
	if err != nil {
		return QuotaData{}, err
	}

	balances := parseDeepSeekBalances(response.BalanceInfos)
	status := determineDeepSeekStatus(response.IsAvailable, balances)

	rawData := map[string]any{}
	if response.IsAvailable != nil {
		rawData["is_available"] = *response.IsAvailable
	}
	if len(response.BalanceInfos) > 0 {
		rawData["balance_infos"] = convertDeepSeekBalanceInfos(response.BalanceInfos)
	}

	return NormalizeQuotaData(QuotaData{
		Status:       status,
		ProviderType: "deepseek",
		RawData:      rawData,
		Ready:        IsReadyStatus(status),
		Balances:     balances,
	}), nil
}

// fetchDeepSeekBalance performs one balance request for a single account.
func fetchDeepSeekBalance(
	ctx context.Context,
	hc *httpclient.HttpClient,
	balanceURL string,
	apiKey string,
) (DeepSeekBalanceResponse, error) {
	httpRequest := httpclient.NewRequestBuilder().
		WithMethod("GET").
		WithURL(balanceURL).
		WithBearerToken(apiKey).
		WithHeader("Content-Type", "application/json").
		Build()

	resp, err := hc.Do(ctx, httpRequest)
	if err != nil {
		return DeepSeekBalanceResponse{}, fmt.Errorf("DeepSeek balance request failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return DeepSeekBalanceResponse{}, fmt.Errorf("DeepSeek balance API returned status %d: %s", resp.StatusCode, string(resp.Body))
	}

	var response DeepSeekBalanceResponse
	if err := json.Unmarshal(resp.Body, &response); err != nil {
		return DeepSeekBalanceResponse{}, fmt.Errorf("failed to parse DeepSeek balance response: %w", err)
	}
	if response.IsAvailable == nil && len(response.BalanceInfos) == 0 {
		return DeepSeekBalanceResponse{}, fmt.Errorf("DeepSeek balance response missing balance information")
	}

	return response, nil
}

// determineDeepSeekStatus maps the provider's authoritative is_available flag
// onto the shared status vocabulary. When the flag is missing, a positive
// balance makes the account available. DeepSeek bills pay-as-you-go, so it has
// no usage window; the balance rows are informational and the status alone
// drives routing.
func determineDeepSeekStatus(isAvailable *bool, balances []QuotaBalance) string {
	if isAvailable != nil && !*isAvailable {
		return "exhausted"
	}

	hasPositiveBalance := false
	for _, balance := range balances {
		if balance.Total > 0 {
			hasPositiveBalance = true
			break
		}
	}

	if hasPositiveBalance {
		return "available"
	}
	if isAvailable != nil && *isAvailable {
		return "available"
	}
	if len(balances) > 0 {
		return "exhausted"
	}

	return "unknown"
}

// SupportsChannel reports whether the channel uses DeepSeek's balance API,
// either through a dedicated channel type or an OpenAI-compatible DeepSeek URL.
func (c *DeepSeekQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	switch ch.Type {
	case channel.TypeDeepseek, channel.TypeDeepseekAnthropic:
		return true
	case channel.TypeOpenai, channel.TypeOpenaiResponses:
		return DetectProviderFromURL(ch.BaseURL) == "deepseek"
	default:
		return false
	}
}

// buildDeepSeekBalanceURL derives the balance endpoint from the channel base
// URL while preserving the relay's scheme and path prefix.
func buildDeepSeekBalanceURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return deepseekDefaultBaseURL + "/user/balance"
	}

	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" {
		return deepseekDefaultBaseURL + "/user/balance"
	}

	// DeepSeek inference accepts both http and https relays, and the balance
	// endpoint lives beside the configured API prefix. Keep the relay's scheme
	// and prefix, stripping only the known API suffix.
	if parsed.Scheme == "" {
		parsed.Scheme = "https"
	}

	path := strings.TrimRight(parsed.Path, "/")
	for _, suffix := range []string{"/v1", "/anthropic"} {
		if before, found := strings.CutSuffix(path, suffix); found {
			path = before
			break
		}
	}
	parsed.Path = strings.TrimRight(path, "/") + "/user/balance"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.User = nil

	return parsed.String()
}

// parseDeepSeekBalances converts provider balance entries into normalized
// balances, skipping entries without a parseable total.
func parseDeepSeekBalances(infos []DeepSeekBalanceInfo) []QuotaBalance {
	balances := make([]QuotaBalance, 0, len(infos))
	for _, info := range infos {
		total, ok := toFloat64(info.TotalBalance)
		if !ok {
			continue
		}

		balance := QuotaBalance{
			Currency: strings.ToUpper(strings.TrimSpace(info.Currency)),
			Total:    total,
		}
		if granted, ok := toFloat64(info.GrantedBalance); ok {
			balance.Granted = &granted
		}
		if toppedUp, ok := toFloat64(info.ToppedUpBalance); ok {
			balance.ToppedUp = &toppedUp
		}

		balances = append(balances, balance)
	}

	return balances
}

// convertDeepSeekBalanceInfos keeps the provider's raw amount fields for
// debugging without exposing credentials.
func convertDeepSeekBalanceInfos(infos []DeepSeekBalanceInfo) []map[string]any {
	result := make([]map[string]any, 0, len(infos))
	for _, info := range infos {
		entry := map[string]any{
			"currency": info.Currency,
		}
		if info.TotalBalance != nil {
			entry["total_balance"] = info.TotalBalance
		}
		if info.GrantedBalance != nil {
			entry["granted_balance"] = info.GrantedBalance
		}
		if info.ToppedUpBalance != nil {
			entry["topped_up_balance"] = info.ToppedUpBalance
		}
		result = append(result, entry)
	}

	return result
}
