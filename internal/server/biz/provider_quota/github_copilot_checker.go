package provider_quota

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/transformer/openai/copilot"
)

type GithubCopilotQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

func NewGithubCopilotQuotaChecker(httpClient *httpclient.HttpClient) *GithubCopilotQuotaChecker {
	return &GithubCopilotQuotaChecker{
		httpClient: httpClient,
	}
}

type copilotUserPayload struct {
	CopilotPlan          *string        `json:"copilot_plan"`
	AccessTypeSku        *string        `json:"access_type_sku"`
	LimitedUserQuotas    map[string]any `json:"limited_user_quotas"`
	MonthlyQuotas        map[string]any `json:"monthly_quotas"`
	QuotaSnapshots       map[string]any `json:"quota_snapshots"`
	LimitedUserResetDate *string        `json:"limited_user_reset_date"`
	QuotaResetDateUTC    *string        `json:"quota_reset_date_utc"`
	QuotaResetDate       *string        `json:"quota_reset_date"`
}

func (c *GithubCopilotQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	accessToken, err := c.getAccessToken(ch)
	if err != nil {
		return QuotaData{}, err
	}

	hc := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = c.httpClient.WithProxy(ch.Settings.Proxy)
	}

	payload, err := c.fetchUserPayload(ctx, hc, accessToken)
	if err != nil {
		return QuotaData{}, err
	}

	status, _ := c.calculateStatus(payload)

	return QuotaData{
		Status:       status,
		ProviderType: "github_copilot",
		RawData:      c.prepareRawData(payload),
		NextResetAt:  c.parseResetDate(payload),
		Ready:        status == "available" || status == "warning",
	}, nil
}

func (c *GithubCopilotQuotaChecker) getAccessToken(ch *ent.Channel) (string, error) {
	if ch.Credentials.OAuth == nil && strings.TrimSpace(ch.Credentials.APIKey) == "" {
		return "", fmt.Errorf("channel has no credentials")
	}

	if ch.Credentials.OAuth != nil {
		return ch.Credentials.OAuth.AccessToken, nil
	}

	// Usually GitHub Copilot channels have the oauth token stored either directly or as oauth json
	creds, err := oauth.ParseCredentialsJSON(ch.Credentials.APIKey)
	if err == nil && creds.AccessToken != "" {
		return creds.AccessToken, nil
	}

	// fallback to using the api key itself as the token
	token := strings.TrimSpace(ch.Credentials.APIKey)
	if token == "" {
		return "", fmt.Errorf("GitHub access token is missing")
	}
	return token, nil
}

func (c *GithubCopilotQuotaChecker) fetchUserPayload(ctx context.Context, hc *httpclient.HttpClient, accessToken string) (*copilotUserPayload, error) {
	userReq := httpclient.NewRequestBuilder().
		WithMethod("GET").
		WithURL("https://api.github.com/copilot_internal/user").
		WithHeader("Authorization", "token "+accessToken).
		WithHeader("Accept", "application/json").
		Build()

	copilot.SetCopilotHeaders(userReq.Headers)

	userResp, err := hc.Do(ctx, userReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch copilot user info: %w", err)
	}
	if userResp.StatusCode < 200 || userResp.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to fetch copilot user info, status: %d", userResp.StatusCode)
	}

	var payload copilotUserPayload
	if err := json.Unmarshal(userResp.Body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse copilot user response: %w", err)
	}
	return &payload, nil
}

func (c *GithubCopilotQuotaChecker) mapPlanType(sku string) string {
	switch sku {
	case "copilot_free", "free_limited_copilot":
		return "Free"
	case "copilot_pro":
		return "Pro"
	case "copilot_pro_plus":
		return "Pro+"
	case "copilot_business":
		return "Business"
	case "copilot_enterprise":
		return "Enterprise"
	case "free_educational_quota":
		return "Edu"
	default:
		return ""
	}
}

func (c *GithubCopilotQuotaChecker) prepareRawData(payload *copilotUserPayload) map[string]any {
	rawData := map[string]any{
		"copilot_plan": payload.CopilotPlan,
	}
	if payload.AccessTypeSku != nil {
		rawData["access_type_sku"] = *payload.AccessTypeSku
		if plan := c.mapPlanType(*payload.AccessTypeSku); plan != "" {
			rawData["plan_type"] = plan
		}
	}
	if payload.QuotaResetDateUTC != nil {
		rawData["quota_reset_date_utc"] = *payload.QuotaResetDateUTC
	}
	if payload.LimitedUserResetDate != nil {
		rawData["limited_user_reset_date"] = *payload.LimitedUserResetDate
	}
	if payload.QuotaResetDate != nil {
		rawData["quota_reset_date"] = *payload.QuotaResetDate
	}
	if payload.LimitedUserQuotas != nil {
		rawData["limited_user_quotas"] = payload.LimitedUserQuotas
	}
	if payload.MonthlyQuotas != nil {
		rawData["total_quotas"] = payload.MonthlyQuotas
	}
	if payload.QuotaSnapshots != nil {
		rawData["quota_snapshots"] = payload.QuotaSnapshots
	}
	return rawData
}

func (c *GithubCopilotQuotaChecker) getNumber(val any) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

func (c *GithubCopilotQuotaChecker) calculateStatus(payload *copilotUserPayload) (string, float64) {
	lowestPercentage := 100.0

	// 1. Check limited quotas (Free accounts)
	if payload.LimitedUserQuotas != nil {
		for key, remainingVal := range payload.LimitedUserQuotas {
			if remaining, ok := c.getNumber(remainingVal); ok {
				total := remaining
				if payload.MonthlyQuotas != nil {
					if t, ok := c.getNumber(payload.MonthlyQuotas[key]); ok && t > 0 {
						total = t
					}
				}
				if total > 0 {
					pct := (remaining / total) * 100
					if pct < lowestPercentage {
						lowestPercentage = pct
					}
				}
			}
		}
	}

	// 2. Check quota snapshots (EDU/Premium accounts)
	if payload.QuotaSnapshots != nil {
		for _, snapshot := range payload.QuotaSnapshots {
			if s, ok := snapshot.(map[string]any); ok {
				unlimited, _ := s["unlimited"].(bool)
				if !unlimited {
					if pct, ok := c.getNumber(s["percent_remaining"]); ok {
						if pct < lowestPercentage {
							lowestPercentage = pct
						}
					}
				}
			}
		}
	}

	status := "available"
	if lowestPercentage <= 0 {
		status = "exhausted"
	} else if lowestPercentage < 20 {
		status = "warning"
	}
	return status, lowestPercentage
}

func (c *GithubCopilotQuotaChecker) parseResetDate(payload *copilotUserPayload) *time.Time {
	if payload.QuotaResetDateUTC != nil && *payload.QuotaResetDateUTC != "" {
		if t, err := time.Parse(time.RFC3339, *payload.QuotaResetDateUTC); err == nil {
			return &t
		}
	}
	if payload.QuotaResetDate != nil && *payload.QuotaResetDate != "" {
		if t, err := time.Parse("2006-01-02", *payload.QuotaResetDate); err == nil {
			return &t
		}
	}
	if payload.LimitedUserResetDate != nil && *payload.LimitedUserResetDate != "" {
		if t, err := time.Parse("2006-01-02", *payload.LimitedUserResetDate); err == nil {
			return &t
		}
	}
	return nil
}


func (c *GithubCopilotQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	return ch.Type == channel.TypeGithubCopilot
}
