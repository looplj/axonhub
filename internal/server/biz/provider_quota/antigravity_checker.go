package provider_quota

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/transformer/antigravity"
)

const antigravityQuotaURL = antigravity.EndpointProd + "/v1internal:fetchAvailableModels"
const antigravityQuotaSummaryURL = antigravity.EndpointProd + "/v1internal:retrieveUserQuotaSummary"

func antigravityQuotaEndpoints(ch *ent.Channel) []string {
	endpoints := make([]string, 0, 3)
	if ch != nil && ch.BaseURL != "" {
		trimmed := strings.TrimRight(strings.TrimSpace(ch.BaseURL), "/")
		if trimmed != "" {
			endpoints = append(endpoints, trimmed)
		}
	}
	for _, ep := range []string{antigravity.EndpointProd, antigravity.EndpointDaily} {
		if !slices.Contains(endpoints, ep) {
			endpoints = append(endpoints, ep)
		}
	}
	return endpoints
}

type antigravityQuotaResponse struct {
	Models map[string]struct {
		DisplayName string `json:"displayName"`
		IsInternal  bool   `json:"isInternal"`
		QuotaInfo   *struct {
			RemainingFraction *float64 `json:"remainingFraction"`
			ResetTime         any      `json:"resetTime"`
		} `json:"quotaInfo"`
	} `json:"models"`
}

type AntigravityQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

func NewAntigravityQuotaChecker(httpClient *httpclient.HttpClient) *AntigravityQuotaChecker {
	return &AntigravityQuotaChecker{httpClient: httpClient}
}

func (c *AntigravityQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	httpClient := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		httpClient = c.httpClient.WithProxy(ch.Settings.Proxy)
	}

	accessToken, projectID, err := c.credentials(ctx, ch, httpClient)
	if err != nil {
		return QuotaData{}, err
	}

	body := map[string]any{}
	if projectID != "" {
		body["project"] = projectID
	}

	endpoints := antigravityQuotaEndpoints(ch)
	var lastErr error

	for _, endpoint := range endpoints {
		summaryURL := endpoint + "/v1internal:retrieveUserQuotaSummary"
		modelsURL := endpoint + "/v1internal:fetchAvailableModels"

		request := httpclient.NewRequestBuilder().
			WithMethod(http.MethodPost).
			WithURL(summaryURL).
			WithBearerToken(accessToken).
			WithHeader("Content-Type", "application/json").
			WithBody(body).
			Build()
		antigravity.SetClientHeaders(request.Headers)

		response, err := httpClient.Do(ctx, request)
		if err != nil {
			var upstreamError *httpclient.Error
			if errors.As(err, &upstreamError) {
				if upstreamError.StatusCode == http.StatusUnauthorized || upstreamError.StatusCode == http.StatusForbidden {
					return QuotaData{}, fmt.Errorf("fetch Antigravity quota summary: %w", err)
				}
				if upstreamError.StatusCode != http.StatusNotFound && upstreamError.StatusCode != http.StatusNotImplemented {
					lastErr = fmt.Errorf("fetch Antigravity quota summary from %s: %w", endpoint, err)
					continue
				}
			} else {
				lastErr = fmt.Errorf("fetch Antigravity quota summary from %s: %w", endpoint, err)
				continue
			}
		} else if response.StatusCode == http.StatusOK {
			if quota, parseErr := parseAntigravityQuotaSummary(response.Body); parseErr == nil {
				return quota, nil
			}
		} else if response.StatusCode != http.StatusNotFound && response.StatusCode != http.StatusNotImplemented {
			lastErr = fmt.Errorf("fetch Antigravity quota summary from %s: status %d", endpoint, response.StatusCode)
			if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
				return QuotaData{}, lastErr
			}
			continue
		}

		// Older servers expose only per-model session quotas. Never infer weekly
		// allowances from those responses or label model IDs as time windows.
		legacyReq := httpclient.NewRequestBuilder().
			WithMethod(http.MethodPost).
			WithURL(modelsURL).
			WithBearerToken(accessToken).
			WithHeader("Content-Type", "application/json").
			WithBody(body).
			Build()
		antigravity.SetClientHeaders(legacyReq.Headers)

		response, err = httpClient.Do(ctx, legacyReq)
		if err != nil {
			lastErr = fmt.Errorf("fetch legacy Antigravity quota from %s: %w", endpoint, err)
			continue
		}
		if response.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("fetch legacy Antigravity quota from %s: status %d", endpoint, response.StatusCode)
			continue
		}

		return parseAntigravityQuota(response.Body)
	}

	if lastErr != nil {
		return QuotaData{}, lastErr
	}

	return QuotaData{}, fmt.Errorf("no Antigravity quota endpoint succeeded")
}

func (c *AntigravityQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	return ch.Type == channel.TypeAntigravity
}

func (c *AntigravityQuotaChecker) credentials(
	ctx context.Context,
	ch *ent.Channel,
	httpClient *httpclient.HttpClient,
) (string, string, error) {
	legacyParts := strings.SplitN(strings.TrimSpace(ch.Credentials.APIKey), "|", 2)
	refreshToken := ""
	projectID := ""
	if len(legacyParts) > 0 {
		refreshToken = legacyParts[0]
	}
	if len(legacyParts) == 2 {
		projectID = legacyParts[1]
	}

	credentials := ch.Credentials.OAuth
	if credentials == nil {
		credentials = &oauth.OAuthCredentials{}
	} else if credentials.AccessToken != "" &&
		(!credentials.IsExpired(time.Now()) || (credentials.RefreshToken == "" && refreshToken == "")) {
		return credentials.AccessToken, projectID, nil
	}

	credentialsCopy := *credentials
	if credentialsCopy.RefreshToken == "" {
		credentialsCopy.RefreshToken = refreshToken
	}
	if credentialsCopy.ClientID == "" {
		credentialsCopy.ClientID = antigravity.ClientID
	}
	if len(credentialsCopy.Scopes) == 0 {
		credentialsCopy.Scopes = antigravity.Scopes
	}
	if credentialsCopy.RefreshToken == "" {
		return "", "", fmt.Errorf("channel has no Antigravity OAuth credentials")
	}

	tokenProvider := antigravity.NewTokenProvider(oauth.TokenProviderParams{
		Credentials: &credentialsCopy,
		HTTPClient:  httpClient,
	})
	refreshed, err := tokenProvider.Get(ctx)
	if err != nil {
		return "", "", fmt.Errorf("refresh Antigravity OAuth token: %w", err)
	}

	return refreshed.AccessToken, projectID, nil
}

func parseAntigravityQuota(body []byte) (QuotaData, error) {
	var response antigravityQuotaResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return QuotaData{}, fmt.Errorf("decode Antigravity quota response: %w", err)
	}
	if len(response.Models) == 0 {
		return QuotaData{}, fmt.Errorf("Antigravity quota response has no models")
	}

	models := make(map[string]any, len(response.Models))
	limits := make([]QuotaLimitStatus, 0, len(response.Models))
	maxRemaining := 0.0
	var nextResetAt *time.Time
	poolLimits := make(map[string]QuotaLimitStatus)

	modelIDs := make([]string, 0, len(response.Models))
	for modelID := range response.Models {
		modelIDs = append(modelIDs, modelID)
	}
	sort.Strings(modelIDs)
	for _, modelID := range modelIDs {
		model := response.Models[modelID]
		if model.IsInternal || model.QuotaInfo == nil || model.QuotaInfo.RemainingFraction == nil {
			continue
		}
		if math.IsNaN(*model.QuotaInfo.RemainingFraction) || math.IsInf(*model.QuotaInfo.RemainingFraction, 0) || *model.QuotaInfo.RemainingFraction < 0 {
			continue
		}

		remaining := max(0, min(1, *model.QuotaInfo.RemainingFraction))
		usageRatio := 1 - remaining
		status := antigravityQuotaStatus(usageRatio)
		resetAt := parseAntigravityResetTime(model.QuotaInfo.ResetTime)
		if resetAt != nil && (nextResetAt == nil || resetAt.Before(*nextResetAt)) {
			nextResetAt = resetAt
		}

		modelData := map[string]any{
			"displayName":         model.DisplayName,
			"remainingPercentage": remaining * 100,
			"status":              status,
		}
		if resetAt != nil {
			modelData["resetAt"] = resetAt.Format(time.RFC3339)
		}
		models[modelID] = modelData
		pool := antigravityModelPool(modelID)
		if pool == "" {
			continue
		}
		window := pool + "_5h"
		if current, exists := poolLimits[window]; !exists || usageRatio > current.UsageRatio {
			poolLimits[window] = NewTokenLimitStatus(status, usageRatio, resetAt).WithWindow(window, 5*time.Hour)
		}
	}

	if len(models) == 0 {
		return QuotaData{}, fmt.Errorf("Antigravity quota response has no quota data")
	}
	for _, pool := range []string{"gemini", "claude_gpt"} {
		if limit, ok := poolLimits[pool+"_5h"]; ok {
			limits = append(limits, limit)
			maxRemaining = max(maxRemaining, 1-limit.UsageRatio)
		}
	}
	if len(limits) == 0 {
		return QuotaData{}, fmt.Errorf("Antigravity quota response has no recognized model pools")
	}
	overallStatus := antigravityQuotaStatus(1 - maxRemaining)

	return NormalizeQuotaData(QuotaData{
		Status:       overallStatus,
		ProviderType: "antigravity",
		RawData:      map[string]any{"models": models},
		NextResetAt:  nextResetAt,
		Ready:        IsReadyStatus(overallStatus),
		Limits:       limits,
	}), nil
}

func antigravityQuotaStatus(usageRatio float64) string {
	if usageRatio >= 1 {
		return "exhausted"
	}
	if usageRatio >= WarningThresholdRatio {
		return "warning"
	}
	return "available"
}

func parseAntigravityResetTime(value any) *time.Time {
	var parsed time.Time
	switch value := value.(type) {
	case string:
		if timestamp, err := strconv.ParseInt(value, 10, 64); err == nil {
			parsed = time.Unix(timestamp, 0)
			if timestamp >= 1_000_000_000_000 {
				parsed = time.UnixMilli(timestamp)
			}
		} else {
			parsed, err = time.Parse(time.RFC3339, value)
			if err != nil {
				return nil
			}
		}
	case float64:
		parsed = time.Unix(int64(value), 0)
		if value >= 1_000_000_000_000 {
			parsed = time.UnixMilli(int64(value))
		}
	default:
		return nil
	}
	return &parsed
}
