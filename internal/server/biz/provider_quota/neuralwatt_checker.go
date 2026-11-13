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
	Plan         *string  `json:"plan,omitempty"`
	Status       *string  `json:"status,omitempty"`
	KwhIncluded  *float64 `json:"kwh_included,omitempty"`
	KwhUsed      *float64 `json:"kwh_used,omitempty"`
	KwhRemaining *float64 `json:"kwh_remaining,omitempty"`
	InOverage    *bool    `json:"in_overage,omitempty"`
	KwhResetDate *string  `json:"kwh_reset_date,omitempty"`
}

type NeuralWattUsagePeriod struct {
	CostUSD   *float64 `json:"cost_usd,omitempty"`
	Requests  *int64   `json:"requests,omitempty"`
	Tokens    *int64   `json:"tokens,omitempty"`
	EnergyKwh *float64 `json:"energy_kwh,omitempty"`
}

type NeuralWattDailyFreeEnergy struct {
	KwhIncluded  *float64 `json:"kwh_included,omitempty"`
	KwhUsed      *float64 `json:"kwh_used,omitempty"`
	KwhRemaining *float64 `json:"kwh_remaining,omitempty"`
	ResetAt      *string  `json:"reset_at,omitempty"`
	ModelLabels  []string `json:"model_labels,omitempty"`
}

type NeuralWattUsageResponse struct {
	Balance         *NeuralWattBalance         `json:"balance,omitempty"`
	Subscription    *NeuralWattSubscription    `json:"subscription,omitempty"`
	DailyFreeEnergy *NeuralWattDailyFreeEnergy `json:"daily_free_energy,omitempty"`
	Usage           *struct {
		Lifetime     *NeuralWattUsagePeriod `json:"lifetime,omitempty"`
		CurrentMonth *NeuralWattUsagePeriod `json:"current_month,omitempty"`
	} `json:"usage,omitempty"`
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
	if response.Subscription != nil && response.Subscription.KwhResetDate != nil {
		if t, err := time.Parse(time.RFC3339, *response.Subscription.KwhResetDate); err == nil {
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

	if dfe := response.DailyFreeEnergy; dfe != nil {
		dfeMap := map[string]any{}
		if dfe.KwhIncluded != nil {
			dfeMap["kwh_included"] = *dfe.KwhIncluded
		}
		if dfe.KwhUsed != nil {
			dfeMap["kwh_used"] = *dfe.KwhUsed
		}
		if dfe.KwhRemaining != nil {
			dfeMap["kwh_remaining"] = *dfe.KwhRemaining
		}
		if dfe.ResetAt != nil {
			dfeMap["reset_at"] = *dfe.ResetAt
		}
		dfeMap["model_labels"] = dfe.ModelLabels
		rawData["daily_free_energy"] = dfeMap
	}

	if cm := response.Usage; cm != nil && cm.CurrentMonth != nil {
		cmMap := map[string]any{}
		if cm.CurrentMonth.CostUSD != nil {
			cmMap["cost_usd"] = *cm.CurrentMonth.CostUSD
		}
		if cm.CurrentMonth.Requests != nil {
			cmMap["requests"] = *cm.CurrentMonth.Requests
		}
		if cm.CurrentMonth.Tokens != nil {
			cmMap["tokens"] = *cm.CurrentMonth.Tokens
		}
		if cm.CurrentMonth.EnergyKwh != nil {
			cmMap["energy_kwh"] = *cm.CurrentMonth.EnergyKwh
		}
		rawData["usage_current_month"] = cmMap
	}

	var usageRatio float64
	if response.Subscription != nil && response.Subscription.KwhIncluded != nil && *response.Subscription.KwhIncluded > 0 {
		if response.Subscription.KwhUsed != nil {
			usageRatio = *response.Subscription.KwhUsed / *response.Subscription.KwhIncluded
		} else if response.Subscription.KwhRemaining != nil {
			usageRatio = 1.0 - (*response.Subscription.KwhRemaining / *response.Subscription.KwhIncluded)
		}
	}

	limit := NewTokenLimitStatus(normalizedStatus, usageRatio, nextResetAt)
	limit.Window = "kwh"
	// The kWh window resets monthly at kwh_reset_date; stamping the period
	// start is what enables the on-track marker and the period cost/quota
	// estimate shared with the other windowed providers.
	limit.PeriodStart = PeriodStartFromMonthlyReset(nextResetAt)

	limits := []QuotaLimitStatus{limit}

	// The daily free energy promo is display-only: no request modality maps
	// to QuotaLimitTypeFreeEnergy, so routing, the channel status and the
	// overall NextResetAt ignore it (displayOnlyLimitTypes). It deliberately
	// has no PeriodStart — building the limit explicitly (not via WithWindow)
	// keeps it out of period cost aggregation, PeriodQuotaEstimate, and the
	// frontend backpressure mirror.
	if dfe := response.DailyFreeEnergy; dfe != nil && dfe.KwhIncluded != nil && *dfe.KwhIncluded > 0 {
		dfeRatio := 0.0
		if dfe.KwhUsed != nil {
			dfeRatio = *dfe.KwhUsed / *dfe.KwhIncluded
		} else if dfe.KwhRemaining != nil {
			dfeRatio = 1.0 - (*dfe.KwhRemaining / *dfe.KwhIncluded)
		}
		if dfeRatio < 0 {
			dfeRatio = 0
		} else if dfeRatio > 1 {
			dfeRatio = 1
		}
		dfeStatus := "available"
		switch {
		case dfeRatio >= 1:
			dfeStatus = "exhausted"
		case dfeRatio >= 1-warningRemainingFraction:
			dfeStatus = "warning"
		}
		var dfeResetAt *time.Time
		if dfe.ResetAt != nil {
			if t, err := time.Parse(time.RFC3339, *dfe.ResetAt); err == nil {
				dfeResetAt = &t
			}
		}

		limits = append(limits, QuotaLimitStatus{
			Type:        QuotaLimitTypeFreeEnergy,
			Status:      dfeStatus,
			UsageRatio:  dfeRatio,
			Ready:       IsReadyStatus(dfeStatus),
			NextResetAt: dfeResetAt,
			Window:      "daily_free_energy",
		})
	}

	return NormalizeQuotaData(QuotaData{
		Status:       normalizedStatus,
		ProviderType: "neuralwatt",
		RawData:      rawData,
		NextResetAt:  nextResetAt,
		Ready:        IsReadyStatus(normalizedStatus),
		Limits:       limits,
	}), nil
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

	if sub.KwhResetDate != nil {
		result["kwh_reset_date"] = *sub.KwhResetDate
	}

	return result
}
