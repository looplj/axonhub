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

const warningRemainingFraction = 0.2

type NeuralWattBalance struct {
	CreditsRemainingUSD *float64 `json:"credits_remaining_usd,omitempty"`
	TotalCreditsUSD     *float64 `json:"total_credits_usd,omitempty"`
	AccountingMethod    *string  `json:"accounting_method,omitempty"`
}

type NeuralWattSubscription struct {
	Plan               *string  `json:"plan,omitempty"`
	Status             *string  `json:"status,omitempty"`
	CurrentPeriodStart *string  `json:"current_period_start,omitempty"`
	CurrentPeriodEnd   *string  `json:"current_period_end,omitempty"`
	KwhIncluded        *float64 `json:"kwh_included,omitempty"`
	KwhUsed            *float64 `json:"kwh_used,omitempty"`
	KwhRemaining       *float64 `json:"kwh_remaining,omitempty"`
	InOverage          *bool    `json:"in_overage,omitempty"`
}

type NeuralWattUsageResponse struct {
	Balance      *NeuralWattBalance      `json:"balance,omitempty"`
	Subscription *NeuralWattSubscription `json:"subscription,omitempty"`
}

type NeuralWattQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

func NewNeuralWattQuotaChecker(httpClient *httpclient.HttpClient) *NeuralWattQuotaChecker {
	return &NeuralWattQuotaChecker{
		httpClient: httpClient,
	}
}

func (c *NeuralWattQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	apiKey := strings.TrimSpace(ch.Credentials.APIKey)
	if apiKey == "" && len(ch.Credentials.APIKeys) > 0 {
		apiKey = ch.Credentials.APIKeys[0]
	}

	if apiKey == "" {
		return QuotaData{}, fmt.Errorf("channel has no API key")
	}

	quotaURL := buildNeuralWattQuotaURL(ch.BaseURL)

	httpRequest := httpclient.NewRequestBuilder().
		WithMethod("GET").
		WithURL(quotaURL).
		WithBearerToken(apiKey).
		WithHeader("Content-Type", "application/json").
		Build()

	hc := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = c.httpClient.WithProxy(ch.Settings.Proxy)
	}

	resp, err := hc.Do(ctx, httpRequest)
	if err != nil {
		return QuotaData{}, fmt.Errorf("quota request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return QuotaData{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(resp.Body))
	}

	return c.parseResponse(resp.Body)
}

func (c *NeuralWattQuotaChecker) parseResponse(body []byte) (QuotaData, error) {
	var response NeuralWattUsageResponse

	if err := json.Unmarshal(body, &response); err != nil {
		return QuotaData{}, fmt.Errorf("failed to parse neuralwatt usage response: %w", err)
	}

	normalizedStatus := "unknown"

	if response.Subscription != nil {
		if response.Subscription.InOverage != nil && *response.Subscription.InOverage {
			normalizedStatus = "exhausted"
		} else if response.Subscription.KwhRemaining != nil && *response.Subscription.KwhRemaining <= 0 {
			normalizedStatus = "exhausted"
		} else if isNeuralWattLowRemaining(response.Subscription) {
			normalizedStatus = "warning"
		} else if response.Subscription.KwhRemaining != nil {
			normalizedStatus = "available"
		}
	}

	var nextResetAt *time.Time
	if response.Subscription != nil && response.Subscription.CurrentPeriodEnd != nil {
		if t, err := time.Parse(time.RFC3339, *response.Subscription.CurrentPeriodEnd); err == nil {
			nextResetAt = &t
		}
	}

	rawData := map[string]any{}

	if response.Balance != nil {
		rawData["balance"] = convertNeuralWattBalanceToMap(response.Balance)
	}

	if response.Subscription != nil {
		rawData["subscription"] = convertNeuralWattSubscriptionToMap(response.Subscription)
	}

	return QuotaData{
		Status:       normalizedStatus,
		ProviderType: "neuralwatt",
		RawData:      rawData,
		NextResetAt:  nextResetAt,
		Ready:        normalizedStatus == "available" || normalizedStatus == "warning",
	}, nil
}

func (c *NeuralWattQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	if ch.Type != channel.TypeOpenai && ch.Type != channel.TypeOpenaiResponses {
		return false
	}

	return DetectProviderFromURL(ch.BaseURL) == "neuralwatt"
}

func isNeuralWattLowRemaining(sub *NeuralWattSubscription) bool {
	if sub.KwhRemaining == nil || sub.KwhIncluded == nil || *sub.KwhIncluded <= 0 {
		return false
	}

	return *sub.KwhRemaining < *sub.KwhIncluded*warningRemainingFraction
}

func buildNeuralWattQuotaURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "https://api.neuralwatt.com/v1/quota"
	}

	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme == "" && parsed.Host == "") {
		return "https://api.neuralwatt.com/v1/quota"
	}

	scheme := parsed.Scheme
	if scheme == "http" {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s/v1/quota", scheme, parsed.Host)
}

func convertNeuralWattBalanceToMap(balance *NeuralWattBalance) map[string]any {
	result := make(map[string]any)

	if balance.CreditsRemainingUSD != nil {
		result["credits_remaining_usd"] = *balance.CreditsRemainingUSD
	}

	if balance.TotalCreditsUSD != nil {
		result["total_credits_usd"] = *balance.TotalCreditsUSD
	}

	if balance.AccountingMethod != nil {
		result["accounting_method"] = *balance.AccountingMethod
	}

	return result
}

func convertNeuralWattSubscriptionToMap(sub *NeuralWattSubscription) map[string]any {
	result := make(map[string]any)

	if sub.Plan != nil {
		result["plan"] = *sub.Plan
	}

	if sub.Status != nil {
		result["status"] = *sub.Status
	}

	if sub.CurrentPeriodStart != nil {
		result["current_period_start"] = *sub.CurrentPeriodStart
	}

	if sub.CurrentPeriodEnd != nil {
		result["current_period_end"] = *sub.CurrentPeriodEnd
	}

	if sub.KwhIncluded != nil {
		result["kwh_included"] = *sub.KwhIncluded
	}

	if sub.KwhUsed != nil {
		result["kwh_used"] = *sub.KwhUsed
	}

	if sub.KwhRemaining != nil {
		result["kwh_remaining"] = *sub.KwhRemaining
	}

	if sub.InOverage != nil {
		result["in_overage"] = *sub.InOverage
	}

	return result
}
