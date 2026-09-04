package provider_quota

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/xai/subscription"
)

type XAISubscriptionQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

func NewXAISubscriptionQuotaChecker(httpClient *httpclient.HttpClient) *XAISubscriptionQuotaChecker {
	return &XAISubscriptionQuotaChecker{httpClient: httpClient}
}

func (checker *XAISubscriptionQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	credentials, err := ch.Credentials.ResolveOAuthCredentials()
	if err != nil {
		return QuotaData{}, fmt.Errorf("parse xAI subscription credentials: %w", err)
	}
	accessToken := strings.TrimSpace(credentials.AccessToken)
	if accessToken == "" {
		return QuotaData{}, fmt.Errorf("xAI subscription channel has no OAuth access token")
	}

	client := checker.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		client = checker.httpClient.WithProxy(ch.Settings.Proxy)
	}
	weeklyBody, err := checker.fetchBilling(ctx, client, subscription.BillingWeeklyURL, accessToken)
	if err != nil {
		return QuotaData{}, fmt.Errorf("fetch xAI weekly billing: %w", err)
	}
	monthlyBody, err := checker.fetchBilling(ctx, client, subscription.BillingMonthlyURL, accessToken)
	if err != nil {
		return QuotaData{}, fmt.Errorf("fetch xAI monthly billing: %w", err)
	}

	summary, err := subscription.ParseBillingResponses(weeklyBody, monthlyBody)
	if err != nil {
		return QuotaData{}, fmt.Errorf("parse xAI billing responses: %w", err)
	}
	if plan := subscription.PlanFromAccessToken(accessToken); plan != "" {
		summary.Plan = plan
	}
	return xaiBillingQuotaData(summary), nil
}

func (checker *XAISubscriptionQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	return ch.Type == channel.TypeXaiSubscription
}

func (checker *XAISubscriptionQuotaChecker) fetchBilling(
	ctx context.Context,
	client *httpclient.HttpClient,
	endpoint string,
	accessToken string,
) ([]byte, error) {
	request := httpclient.NewRequestBuilder().
		WithMethod(http.MethodGet).
		WithURL(endpoint).
		WithBearerToken(accessToken).
		WithHeader("Accept", "application/json").
		WithHeader("Content-Type", "application/json").
		WithHeader(subscription.CLITokenAuthHeader, subscription.CLITokenAuth).
		WithHeader(subscription.CLIClientVersionHeader, subscription.CLIClientVersion).
		WithHeader("User-Agent", "grok-pager/"+subscription.CLIClientVersion+" grok-shell/"+subscription.CLIClientVersion+" (macos; aarch64)").
		Build()
	response, err := client.Do(ctx, request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("HTTP %d", response.StatusCode)
	}
	return response.Body, nil
}

func xaiBillingQuotaData(summary subscription.BillingSummary) QuotaData {
	limits := make([]QuotaLimitStatus, 0, 2)
	usagePercent := -1.0
	var weeklyResetAt, monthlyResetAt *time.Time
	if summary.Weekly != nil {
		weeklyResetAt = parseXAIReset(summary.Weekly.ResetAt)
		usagePercent = summary.Weekly.UsagePercent
		limits = append(limits, NewTokenLimitStatus(
			xaiBillingWindowStatus(summary.Weekly.UsagePercent),
			summary.Weekly.UsagePercent/100,
			weeklyResetAt,
		).WithWindow(QuotaWindowWeekly, 7*24*time.Hour))
	}
	if summary.Monthly != nil {
		monthlyResetAt = parseXAIReset(summary.Monthly.ResetAt)
		monthlyStatus := xaiBillingWindowStatus(summary.Monthly.UsagePercent)
		usagePercent = max(usagePercent, summary.Monthly.UsagePercent)
		monthlyLimit := NewTokenLimitStatus(monthlyStatus, summary.Monthly.UsagePercent/100, monthlyResetAt)
		monthlyLimit.Window = QuotaWindowMonthly
		monthlyLimit.PeriodStart = PeriodStartFromMonthlyReset(monthlyResetAt)
		limits = append(limits, monthlyLimit)
	}
	status := "unknown"
	if usagePercent >= 0 {
		status = xaiBillingWindowStatus(usagePercent)
	}
	nextResetAt := earliestReset(weeklyResetAt, monthlyResetAt)
	rawBilling := make(map[string]any, 2)
	if summary.Weekly != nil {
		rawBilling["weekly"] = summary.Weekly
	}
	if summary.Monthly != nil {
		rawBilling["monthly"] = summary.Monthly
	}

	return QuotaData{
		Status:       status,
		ProviderType: "xai_subscription",
		RawData: map[string]any{
			"plan_type": summary.Plan,
			"billing":   rawBilling,
		},
		NextResetAt: nextResetAt,
		Ready:       IsReadyStatus(status),
		Limits:      limits,
	}
}

func xaiBillingWindowStatus(usagePercent float64) string {
	if usagePercent >= 100 {
		return "exhausted"
	}
	if usagePercent >= WarningThresholdRatio*100 {
		return "warning"
	}
	return "available"
}

func parseXAIReset(raw string) *time.Time {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil {
		return nil
	}
	return &parsed
}

func earliestReset(resets ...*time.Time) *time.Time {
	var earliest *time.Time
	for _, reset := range resets {
		if reset != nil && (earliest == nil || reset.Before(*earliest)) {
			earliest = reset
		}
	}
	return earliest
}
