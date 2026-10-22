package provider_quota

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
)

const (
	deepSeekProviderType = "deepseek"

	// deepSeekAccountsAvailabilityGroup lets a multi-key channel stay routable
	// while any enabled account still has balance.
	deepSeekAccountsAvailabilityGroup = "deepseek_accounts"

	// DeepSeekDefaultBaseURL is the official DeepSeek API origin. The balance
	// endpoint lives outside the versioned inference prefix.
	DeepSeekDefaultBaseURL = "https://api.deepseek.com"

	deepSeekBalancePath = "/user/balance"
)

// DeepSeekBalanceInfo is one currency entry returned by GET /user/balance.
// The API encodes amounts as decimal strings.
type DeepSeekBalanceInfo struct {
	Currency        string `json:"currency"`
	TotalBalance    string `json:"total_balance"`
	GrantedBalance  string `json:"granted_balance"`
	ToppedUpBalance string `json:"topped_up_balance"`
}

// DeepSeekBalanceResponse is the payload of GET /user/balance.
type DeepSeekBalanceResponse struct {
	IsAvailable  *bool                 `json:"is_available"`
	BalanceInfos []DeepSeekBalanceInfo `json:"balance_infos"`
}

// DeepSeekQuotaChecker reads the prepaid account balance from DeepSeek's
// official balance API (https://api-docs.deepseek.com/api/get-user-balance).
// DeepSeek does not expose usage windows, so the normalized quota carries the
// available balance in RawData and a single credits limit for routing.
type DeepSeekQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

func NewDeepSeekQuotaChecker(httpClient *httpclient.HttpClient) *DeepSeekQuotaChecker {
	return &DeepSeekQuotaChecker{httpClient: httpClient}
}

// SupportsChannel reports whether the channel is a DeepSeek channel.
func (c *DeepSeekQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	switch ch.Type {
	case channel.TypeDeepseek, channel.TypeDeepseekAnthropic:
		return true
	default:
		return false
	}
}

// CheckQuota calls GET /user/balance for every enabled API key. The endpoint
// rejects invalid keys with 401/403; those surface as ErrInvalidCredentials so
// the service drops stale cached quota data instead of backing off silently.
// A multi-key channel is available while at least one enabled account has
// balance.
func (c *DeepSeekQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	keys := deepSeekEnabledAPIKeys(ch)
	if len(keys) == 0 {
		return QuotaData{}, fmt.Errorf("%w: channel has no enabled API key", ErrInvalidCredentials)
	}

	hc := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = c.httpClient.WithProxy(ch.Settings.Proxy)
	}
	hc = hc.WithRejectHTTPSDowngrade()

	if len(keys) == 1 {
		return c.checkQuotaWithAPIKey(ctx, hc, ch, keys[0])
	}

	accounts := make([]deepSeekAccountQuota, 0, len(keys))
	var firstTransientErr error
	invalidCount := 0

	for _, apiKey := range keys {
		account := deepSeekAccountQuota{
			Account: deepSeekKeySuffix(apiKey),
			Ref:     deepSeekKeyRef(apiKey),
		}
		quotaData, err := c.checkQuotaWithAPIKey(ctx, hc, ch, apiKey)
		if err != nil {
			account.Error = err.Error()
			accounts = append(accounts, account)
			if errors.Is(err, ErrInvalidCredentials) {
				invalidCount++
			} else if firstTransientErr == nil {
				firstTransientErr = err
			}
			continue
		}
		account.Quota = quotaData
		accounts = append(accounts, account)
	}

	successCount := 0
	for _, account := range accounts {
		if account.Error == "" {
			successCount++
		}
	}
	if successCount == 0 {
		if firstTransientErr != nil {
			return QuotaData{}, fmt.Errorf("DeepSeek balance check failed for all %d enabled keys: %w", len(keys), firstTransientErr)
		}
		return QuotaData{}, fmt.Errorf("%w: all %d DeepSeek API keys were rejected", ErrInvalidCredentials, len(keys))
	}

	quotaData := buildDeepSeekMultiAccountQuota(accounts)
	if quotaData.Status == "exhausted" && firstTransientErr != nil {
		// An unreadable account may still have balance. Do not replace cached
		// quota with an exhausted verdict based on incomplete data.
		return QuotaData{}, fmt.Errorf("DeepSeek balance check incomplete: %w", firstTransientErr)
	}

	return quotaData, nil
}

func (c *DeepSeekQuotaChecker) checkQuotaWithAPIKey(
	ctx context.Context,
	hc *httpclient.HttpClient,
	ch *ent.Channel,
	apiKey string,
) (QuotaData, error) {
	request := httpclient.NewRequestBuilder().
		WithMethod(http.MethodGet).
		WithURL(buildDeepSeekBalanceURL(ch.BaseURL)).
		WithBearerToken(apiKey).
		WithHeader("Accept", "application/json").
		Build()

	resp, err := hc.Do(ctx, request)
	if err != nil {
		if httpErr, ok := errors.AsType[*httpclient.Error](err); ok {
			if httpErr.StatusCode == http.StatusUnauthorized || httpErr.StatusCode == http.StatusForbidden {
				return QuotaData{}, fmt.Errorf("%w: DeepSeek balance API returned %d", ErrInvalidCredentials, httpErr.StatusCode)
			}
			return QuotaData{}, fmt.Errorf("DeepSeek balance API returned %d", httpErr.StatusCode)
		}
		return QuotaData{}, fmt.Errorf("DeepSeek balance request failed: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return QuotaData{}, fmt.Errorf("DeepSeek balance API returned %d", resp.StatusCode)
	}

	return c.parseResponse(resp.Body)
}

type deepSeekAccountQuota struct {
	Account string
	Ref     string
	Quota   QuotaData
	Error   string
}

func buildDeepSeekMultiAccountQuota(accounts []deepSeekAccountQuota) QuotaData {
	limits := make([]QuotaLimitStatus, 0, len(accounts))
	balances := make([]map[string]any, 0, len(accounts))
	rawAccounts := make([]map[string]any, 0, len(accounts))
	anyAvailable := false
	anyWarning := false

	for _, account := range accounts {
		rawAccount := map[string]any{"account": account.Account, "ref": account.Ref}
		if account.Error != "" {
			rawAccount["error"] = account.Error
			rawAccounts = append(rawAccounts, rawAccount)
			continue
		}

		rawAccount["status"] = account.Quota.Status
		rawAccount["ready"] = account.Quota.Ready
		if accountBalances, ok := account.Quota.RawData["balances"].([]map[string]any); ok {
			accountBalancesWithLabel := make([]map[string]any, 0, len(accountBalances))
			for _, balance := range accountBalances {
				labeled := make(map[string]any, len(balance)+1)
				maps.Copy(labeled, balance)
				labeled["account"] = account.Account
				accountBalancesWithLabel = append(accountBalancesWithLabel, labeled)
				balances = append(balances, labeled)
			}
			rawAccount["balances"] = accountBalancesWithLabel
		}
		rawAccounts = append(rawAccounts, rawAccount)

		for _, limit := range account.Quota.Limits {
			limit.AvailabilityGroup = deepSeekAccountsAvailabilityGroup
			limit.Account = account.Ref
			limits = append(limits, limit)
		}

		switch account.Quota.Status {
		case "available":
			anyAvailable = true
		case "warning":
			anyWarning = true
		}
	}

	status := "exhausted"
	if anyAvailable {
		status = "available"
	} else if anyWarning {
		status = "warning"
	}

	return NormalizeQuotaData(QuotaData{
		Status:       status,
		ProviderType: deepSeekProviderType,
		RawData: map[string]any{
			"is_available": IsReadyStatus(status),
			"balances":     balances,
			"accounts":     rawAccounts,
		},
		Limits: limits,
	})
}

func (c *DeepSeekQuotaChecker) parseResponse(body []byte) (QuotaData, error) {
	var resp DeepSeekBalanceResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return QuotaData{}, fmt.Errorf("failed to parse DeepSeek balance response: %w", err)
	}
	if resp.IsAvailable == nil {
		return QuotaData{}, errors.New("DeepSeek balance response is missing is_available")
	}

	balances := make([]map[string]any, 0, len(resp.BalanceInfos))
	hasPositiveBalance := false
	for _, info := range resp.BalanceInfos {
		currency := strings.ToUpper(strings.TrimSpace(info.Currency))
		if currency == "" {
			return QuotaData{}, errors.New("DeepSeek balance response contains an entry without a currency")
		}

		total, err := parseDeepSeekAmount(currency, "total_balance", info.TotalBalance, true)
		if err != nil {
			return QuotaData{}, err
		}
		granted, err := parseDeepSeekAmount(currency, "granted_balance", info.GrantedBalance, false)
		if err != nil {
			return QuotaData{}, err
		}
		toppedUp, err := parseDeepSeekAmount(currency, "topped_up_balance", info.ToppedUpBalance, false)
		if err != nil {
			return QuotaData{}, err
		}

		if total > 0 {
			hasPositiveBalance = true
		}
		balances = append(balances, map[string]any{
			"currency":          currency,
			"total_balance":     total,
			"granted_balance":   granted,
			"topped_up_balance": toppedUp,
		})
	}

	// An empty balance list is only meaningful when DeepSeek explicitly says
	// the account cannot make calls. A successful response without any balance
	// entry is malformed and should be retried rather than cached as exhausted.
	if len(balances) == 0 && *resp.IsAvailable {
		return QuotaData{}, errors.New("DeepSeek balance response contains no balance info")
	}

	status := "available"
	usageRatio := 0.0
	if !*resp.IsAvailable || !hasPositiveBalance {
		status = "exhausted"
		usageRatio = 1.0
	}

	return NormalizeQuotaData(QuotaData{
		Status:       status,
		ProviderType: deepSeekProviderType,
		RawData: map[string]any{
			"is_available": *resp.IsAvailable,
			"balances":     balances,
		},
		Ready: IsReadyStatus(status),
		Limits: []QuotaLimitStatus{
			NewTokenLimitStatus(status, usageRatio, nil).WithWindow(QuotaWindowCredits, 0),
		},
	}), nil
}

func parseDeepSeekAmount(currency, field, value string, required bool) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		if required {
			return 0, fmt.Errorf("DeepSeek balance for %s is missing %s", currency, field)
		}
		return 0, nil
	}

	amount, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 {
		return 0, fmt.Errorf("DeepSeek balance for %s has an invalid %s: %q", currency, field, value)
	}

	return amount, nil
}

func deepSeekEnabledAPIKeys(ch *ent.Channel) []string {
	disabled := make(map[string]struct{}, len(ch.DisabledAPIKeys))
	for _, entry := range ch.DisabledAPIKeys {
		if entry.IsExpired() {
			continue
		}
		key := strings.TrimSpace(entry.Key)
		if key != "" {
			disabled[key] = struct{}{}
		}
	}

	keys := make([]string, 0, len(ch.Credentials.APIKeys)+1)
	seen := make(map[string]struct{}, len(ch.Credentials.APIKeys)+1)
	for _, candidate := range ch.Credentials.GetAllAPIKeys() {
		key := strings.TrimSpace(candidate)
		if key == "" {
			continue
		}
		if _, ok := disabled[key]; ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}

	return keys
}

func deepSeekKeySuffix(key string) string {
	runes := []rune(strings.TrimSpace(key))
	if len(runes) <= 4 {
		return "key"
	}
	return string(runes[len(runes)-4:])
}

func deepSeekKeyRef(key string) string {
	digest := sha256.Sum256([]byte("deepseek:" + key))
	return hex.EncodeToString(digest[:])[:8]
}

// buildDeepSeekBalanceURL resolves the balance endpoint from the channel base
// URL. DeepSeek accepts inference base URLs with a /v1, /beta, or /anthropic
// suffix, but the balance endpoint is unversioned:
// https://api.deepseek.com/user/balance.
func buildDeepSeekBalanceURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return DeepSeekDefaultBaseURL + deepSeekBalancePath
	}

	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return DeepSeekDefaultBaseURL + deepSeekBalancePath
	}

	path := strings.TrimRight(parsed.Path, "/")
	for {
		lowerPath := strings.ToLower(path)
		stripped := false
		for _, suffix := range []string{"/anthropic", "/v1", "/beta"} {
			if strings.HasSuffix(lowerPath, suffix) {
				path = strings.TrimRight(path[:len(path)-len(suffix)], "/")
				stripped = true
				break
			}
		}
		if !stripped {
			break
		}
	}

	if strings.HasSuffix(strings.ToLower(path), deepSeekBalancePath) {
		parsed.Path = path
	} else {
		parsed.Path = path + deepSeekBalancePath
	}
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	parsed.User = nil

	return parsed.String()
}
