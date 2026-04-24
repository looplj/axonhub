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

func (c *GithubCopilotQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	// Extract OAuth credentials
	if ch.Credentials.OAuth == nil && strings.TrimSpace(ch.Credentials.APIKey) == "" {
		return QuotaData{}, fmt.Errorf("channel has no credentials")
	}

	var accessToken string
	if ch.Credentials.OAuth != nil {
		accessToken = ch.Credentials.OAuth.AccessToken
	} else if strings.TrimSpace(ch.Credentials.APIKey) != "" {
		// Usually GitHub Copilot channels have the oauth token stored either directly or as oauth json
		creds, err := oauth.ParseCredentialsJSON(ch.Credentials.APIKey)
		if err == nil && creds.AccessToken != "" {
			accessToken = creds.AccessToken
		} else {
			// fallback to using the api key itself as the token
			accessToken = strings.TrimSpace(ch.Credentials.APIKey)
		}
	}

	if accessToken == "" {
		return QuotaData{}, fmt.Errorf("GitHub access token is missing")
	}

	hc := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = c.httpClient.WithProxy(ch.Settings.Proxy)
	}

	// Fetch user info
	userReq := httpclient.NewRequestBuilder().
		WithMethod("GET").
		WithURL("https://api.github.com/copilot_internal/user").
		WithHeader("Authorization", "token "+accessToken).
		WithHeader("Accept", "application/json").
		Build()

	copilot.SetCopilotHeaders(userReq.Headers)

	userResp, err := hc.Do(ctx, userReq)

	if err != nil {
		return QuotaData{}, fmt.Errorf("failed to fetch copilot user info: %w", err)
	}
	if userResp.StatusCode < 200 || userResp.StatusCode >= 300 {
		return QuotaData{}, fmt.Errorf("failed to fetch copilot user info, status: %d", userResp.StatusCode)
	}

	var userPayload struct {
		CopilotPlan          *string        `json:"copilot_plan"`
		AccessTypeSku        *string        `json:"access_type_sku"`
		LimitedUserQuotas    map[string]any `json:"limited_user_quotas"`
		MonthlyQuotas        map[string]any `json:"monthly_quotas"`
		QuotaSnapshots       map[string]any `json:"quota_snapshots"`
		LimitedUserResetDate *string        `json:"limited_user_reset_date"`
		QuotaResetDateUTC    *string        `json:"quota_reset_date_utc"`
		QuotaResetDate       *string        `json:"quota_reset_date"`
	}
	if err := json.Unmarshal(userResp.Body, &userPayload); err != nil {
		return QuotaData{}, fmt.Errorf("failed to parse copilot user response: %w", err)
	}

	// Combine into RawData for UI
	rawData := map[string]any{
		"copilot_plan": userPayload.CopilotPlan,
	}
	if userPayload.AccessTypeSku != nil {
		rawData["access_type_sku"] = *userPayload.AccessTypeSku
		// Map access_type_sku to plan_type for UI badge
		switch *userPayload.AccessTypeSku {
		case "copilot_free":
			rawData["plan_type"] = "Free"
		case "free_limited_copilot":
			rawData["plan_type"] = "Free"
		case "copilot_pro":
			rawData["plan_type"] = "Pro"
		case "copilot_pro_plus":
			rawData["plan_type"] = "Pro+"
		case "copilot_business":
			rawData["plan_type"] = "Business"
		case "copilot_enterprise":
			rawData["plan_type"] = "Enterprise"
		case "free_educational_quota":
			rawData["plan_type"] = "Edu"
		}
	}
	if userPayload.QuotaResetDateUTC != nil {
		rawData["quota_reset_date_utc"] = *userPayload.QuotaResetDateUTC
	}
	if userPayload.LimitedUserResetDate != nil {
		rawData["limited_user_reset_date"] = *userPayload.LimitedUserResetDate
	}
	if userPayload.QuotaResetDate != nil {
		rawData["quota_reset_date"] = *userPayload.QuotaResetDate
	}
	if userPayload.LimitedUserQuotas != nil {
		rawData["limited_user_quotas"] = userPayload.LimitedUserQuotas
	}
	if userPayload.MonthlyQuotas != nil {
		rawData["total_quotas"] = userPayload.MonthlyQuotas
	}
	if userPayload.QuotaSnapshots != nil {
		rawData["quota_snapshots"] = userPayload.QuotaSnapshots
	}

	// Normalize Status and calculate exhaustion
	overallStatus := "available"
	lowestPercentage := 100.0

	// Helper to extract number
	getNumber := func(val any) (float64, bool) {
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

	// 1. Check limited quotas (Free accounts)
	if userPayload.LimitedUserQuotas != nil {
		for key, remainingVal := range userPayload.LimitedUserQuotas {
			if remaining, ok := getNumber(remainingVal); ok {
				total := remaining
				if userPayload.MonthlyQuotas != nil {
					if t, ok := getNumber(userPayload.MonthlyQuotas[key]); ok && t > 0 {
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
	if userPayload.QuotaSnapshots != nil {
		for _, snapshot := range userPayload.QuotaSnapshots {
			if s, ok := snapshot.(map[string]any); ok {
				unlimited, _ := s["unlimited"].(bool)
				if !unlimited {
					if pct, ok := getNumber(s["percent_remaining"]); ok {
						if pct < lowestPercentage {
							lowestPercentage = pct
						}
					}
				}
			}
		}
	}

	if lowestPercentage <= 0 {
		overallStatus = "exhausted"
	} else if lowestPercentage < 20 {
		overallStatus = "warning"
	}

	// Parse next reset date
	var nextResetAt *time.Time
	if userPayload.QuotaResetDateUTC != nil && *userPayload.QuotaResetDateUTC != "" {
		if t, err := time.Parse(time.RFC3339, *userPayload.QuotaResetDateUTC); err == nil {
			nextResetAt = &t
		}
	} else if userPayload.QuotaResetDate != nil && *userPayload.QuotaResetDate != "" {
		if t, err := time.Parse("2006-01-02", *userPayload.QuotaResetDate); err == nil {
			nextResetAt = &t
		}
	} else if userPayload.LimitedUserResetDate != nil && *userPayload.LimitedUserResetDate != "" {
		if t, err := time.Parse("2006-01-02", *userPayload.LimitedUserResetDate); err == nil {
			nextResetAt = &t
		}
	}

	return QuotaData{
		Status:       overallStatus,
		ProviderType: "github_copilot",
		RawData:      rawData,
		NextResetAt:  nextResetAt,
		Ready:        overallStatus == "available" || overallStatus == "warning",
	}, nil
}

func (c *GithubCopilotQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	return ch.Type == channel.TypeGithubCopilot
}
