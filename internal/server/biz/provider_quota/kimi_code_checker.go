package provider_quota

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
)

const kimiCodeDefaultBaseURL = "https://api.kimi.com/coding/v1"

type kimiCodeUsageResponse struct {
	Usage         map[string]any   `json:"usage"`
	Limits        []map[string]any `json:"limits"`
	BoosterWallet map[string]any   `json:"boosterWallet"`
}

type kimiCodeUsageRow struct {
	Label             string  `json:"label"`
	Used              int64   `json:"used"`
	Limit             int64   `json:"limit"`
	ResetAt           *string `json:"resetAt,omitempty"`
	ResetAfterSeconds *int64  `json:"resetAfterSeconds,omitempty"`
}

type kimiCodeBoosterWallet struct {
	BalanceCents              int64  `json:"balanceCents"`
	TotalCents                int64  `json:"totalCents"`
	MonthlyChargeLimitEnabled bool   `json:"monthlyChargeLimitEnabled"`
	MonthlyChargeLimitCents   int64  `json:"monthlyChargeLimitCents"`
	MonthlyUsedCents          int64  `json:"monthlyUsedCents"`
	Currency                  string `json:"currency"`
}

type KimiCodeQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

func NewKimiCodeQuotaChecker(httpClient *httpclient.HttpClient) *KimiCodeQuotaChecker {
	return &KimiCodeQuotaChecker{httpClient: httpClient}
}

func (c *KimiCodeQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	apiKey := strings.TrimSpace(ch.Credentials.APIKey)
	if apiKey == "" && len(ch.Credentials.APIKeys) > 0 {
		apiKey = strings.TrimSpace(ch.Credentials.APIKeys[0])
	}
	if apiKey == "" {
		return QuotaData{}, fmt.Errorf("channel has no API key")
	}

	request := httpclient.NewRequestBuilder().
		WithMethod("GET").
		WithURL(buildKimiCodeUsageURL(ch.BaseURL)).
		WithBearerToken(apiKey).
		WithHeader("Accept", "application/json").
		Build()

	hc := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = c.httpClient.WithProxy(ch.Settings.Proxy)
	}

	resp, err := hc.Do(ctx, request)
	if err != nil {
		return QuotaData{}, fmt.Errorf("kimi code usage request failed: %w", err)
	}

	return parseKimiCodeUsageResponse(resp.Body)
}

func (c *KimiCodeQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	return ch.Type == channel.TypeMoonshotCoding
}

func buildKimiCodeUsageURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return kimiCodeDefaultBaseURL + "/usages"
	}

	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return kimiCodeDefaultBaseURL + "/usages"
	}

	parsed.RawQuery = ""
	parsed.Fragment = ""
	path := strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(path, "/v1") {
		path += "/v1"
	}
	parsed.Path = path + "/usages"
	return parsed.String()
}

func parseKimiCodeUsageResponse(body []byte) (QuotaData, error) {
	var response kimiCodeUsageResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return QuotaData{}, fmt.Errorf("failed to parse kimi code usage response: %w", err)
	}

	rows := make([]kimiCodeUsageRow, 0, len(response.Limits)+1)
	if row, ok := parseKimiCodeUsageRow(response.Usage, "Weekly limit"); ok {
		rows = append(rows, row)
	}
	for i, item := range response.Limits {
		detail := item
		if nested, ok := item["detail"].(map[string]any); ok {
			detail = nested
		}
		label := kimiCodeLimitLabel(item, detail, i)
		if row, ok := parseKimiCodeUsageRow(detail, label); ok {
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		return QuotaData{}, fmt.Errorf("kimi code usage response contains no quota limits")
	}

	status := "available"
	limits := make([]QuotaLimitStatus, 0, len(rows))
	var nextResetAt *time.Time
	for _, row := range rows {
		ratio := 0.0
		if row.Limit > 0 {
			ratio = float64(row.Used) / float64(row.Limit)
		}
		rowStatus := kimiCodeStatusForUsageRatio(ratio)
		status = worseKimiCodeStatus(status, rowStatus)

		var resetAt *time.Time
		if row.ResetAt != nil {
			if parsed, err := time.Parse(time.RFC3339Nano, *row.ResetAt); err == nil {
				resetAt = &parsed
				if nextResetAt == nil || parsed.Before(*nextResetAt) {
					nextResetAt = &parsed
				}
			}
		} else if row.ResetAfterSeconds != nil && *row.ResetAfterSeconds > 0 {
			parsed := time.Now().Add(time.Duration(*row.ResetAfterSeconds) * time.Second)
			resetAt = &parsed
			if nextResetAt == nil || parsed.Before(*nextResetAt) {
				nextResetAt = &parsed
			}
		}
		limits = append(limits, NewTokenLimitStatus(rowStatus, ratio, resetAt))
	}

	rawData := map[string]any{"rows": rows}
	if wallet, ok := parseKimiCodeBoosterWallet(response.BoosterWallet); ok {
		rawData["boosterWallet"] = wallet
	}

	return QuotaData{
		Status:       status,
		ProviderType: "kimi_code",
		RawData:      rawData,
		NextResetAt:  nextResetAt,
		Ready:        IsReadyStatus(status),
		Limits:       limits,
	}, nil
}

func parseKimiCodeUsageRow(raw map[string]any, fallbackLabel string) (kimiCodeUsageRow, bool) {
	if len(raw) == 0 {
		return kimiCodeUsageRow{}, false
	}
	limit, limitOK := kimiCodeInt(raw["limit"])
	used, usedOK := kimiCodeInt(raw["used"])
	if !usedOK && limitOK {
		if remaining, ok := kimiCodeInt(raw["remaining"]); ok {
			used, usedOK = limit-remaining, true
		}
	}
	if !limitOK && !usedOK {
		return kimiCodeUsageRow{}, false
	}

	label := fallbackLabel
	for _, key := range []string{"name", "title"} {
		if value, ok := raw[key].(string); ok && value != "" {
			label = value
			break
		}
	}

	return kimiCodeUsageRow{
		Label:             label,
		Used:              used,
		Limit:             limit,
		ResetAt:           kimiCodeResetAt(raw),
		ResetAfterSeconds: kimiCodeResetAfterSeconds(raw),
	}, true
}

func kimiCodeLimitLabel(item, detail map[string]any, index int) string {
	for _, key := range []string{"name", "title", "scope"} {
		for _, source := range []map[string]any{item, detail} {
			if value, ok := source[key].(string); ok && value != "" {
				return value
			}
		}
	}

	window, _ := item["window"].(map[string]any)
	duration, ok := firstKimiCodeInt(window["duration"], item["duration"], detail["duration"])
	if ok {
		unit, _ := firstKimiCodeString(window["timeUnit"], item["timeUnit"], detail["timeUnit"])
		switch {
		case strings.Contains(unit, "MINUTE") && duration >= 60 && duration%60 == 0:
			return fmt.Sprintf("%dh limit", duration/60)
		case strings.Contains(unit, "MINUTE"):
			return fmt.Sprintf("%dm limit", duration)
		case strings.Contains(unit, "HOUR"):
			return fmt.Sprintf("%dh limit", duration)
		case strings.Contains(unit, "DAY"):
			return fmt.Sprintf("%dd limit", duration)
		default:
			return fmt.Sprintf("%ds limit", duration)
		}
	}
	return fmt.Sprintf("Limit #%d", index+1)
}

func kimiCodeResetAt(raw map[string]any) *string {
	for _, key := range []string{"reset_at", "resetAt", "reset_time", "resetTime"} {
		if value, ok := raw[key].(string); ok && value != "" {
			return &value
		}
	}
	return nil
}

func kimiCodeResetAfterSeconds(raw map[string]any) *int64 {
	for _, key := range []string{"reset_in", "resetIn", "ttl", "window"} {
		if value, ok := kimiCodeInt(raw[key]); ok && value > 0 {
			return &value
		}
	}
	return nil
}

func parseKimiCodeBoosterWallet(raw map[string]any) (kimiCodeBoosterWallet, bool) {
	balance, ok := raw["balance"].(map[string]any)
	if !ok || balance["type"] != "BOOSTER" {
		return kimiCodeBoosterWallet{}, false
	}
	amount, ok := kimiCodeInt(balance["amount"])
	if !ok || amount <= 0 {
		return kimiCodeBoosterWallet{}, false
	}
	amountLeft, _ := kimiCodeInt(balance["amountLeft"])
	monthlyLimit, limitCurrency := kimiCodeMoney(raw["monthlyChargeLimit"])
	monthlyUsed, usedCurrency := kimiCodeMoney(raw["monthlyUsed"])
	currency := limitCurrency
	if currency == "" {
		currency = usedCurrency
	}
	if currency == "" {
		currency = "USD"
	}

	return kimiCodeBoosterWallet{
		BalanceCents:              kimiCodeFixedPointToCents(amountLeft),
		TotalCents:                kimiCodeFixedPointToCents(amount),
		MonthlyChargeLimitEnabled: raw["monthlyChargeLimitEnabled"] == true,
		MonthlyChargeLimitCents:   monthlyLimit,
		MonthlyUsedCents:          monthlyUsed,
		Currency:                  currency,
	}, true
}

func kimiCodeMoney(raw any) (int64, string) {
	record, ok := raw.(map[string]any)
	if !ok {
		return 0, ""
	}
	cents, _ := kimiCodeInt(record["priceInCents"])
	currency, _ := record["currency"].(string)
	return cents, currency
}

func kimiCodeFixedPointToCents(value int64) int64 {
	if value > 0 && value < 1_000_000 {
		return 1
	}
	return (value + 500_000) / 1_000_000
}

func kimiCodeInt(value any) (int64, bool) {
	switch value := value.(type) {
	case float64:
		return int64(value), true
	case string:
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return int64(parsed), true
		}
	}
	return 0, false
}

func firstKimiCodeInt(values ...any) (int64, bool) {
	for _, value := range values {
		if parsed, ok := kimiCodeInt(value); ok {
			return parsed, true
		}
	}
	return 0, false
}

func firstKimiCodeString(values ...any) (string, bool) {
	for _, value := range values {
		if parsed, ok := value.(string); ok {
			return parsed, true
		}
	}
	return "", false
}

func kimiCodeStatusForUsageRatio(ratio float64) string {
	if ratio >= 1 {
		return "exhausted"
	}
	if ratio >= WarningThresholdRatio {
		return "warning"
	}
	return "available"
}

func worseKimiCodeStatus(a, b string) string {
	rank := map[string]int{"available": 0, "warning": 1, "exhausted": 2}
	if rank[b] > rank[a] {
		return b
	}
	return a
}
