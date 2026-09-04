package provider_quota

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

	request := httpclient.NewRequestBuilder().
		WithMethod(http.MethodPost).
		WithURL(antigravityQuotaURL).
		WithBearerToken(accessToken).
		WithHeader("Content-Type", "application/json").
		WithHeader("User-Agent", antigravity.GetUserAgent()).
		WithHeader("X-Client-Name", "antigravity").
		WithHeader("X-Client-Version", antigravity.GetVersion()).
		WithBody(body).
		Build()

	response, err := httpClient.Do(ctx, request)
	if err != nil {
		return QuotaData{}, fmt.Errorf("fetch Antigravity quota: %w", err)
	}

	return parseAntigravityQuota(response.Body)
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
	maxRemaining := 0.0
	var nextResetAt *time.Time

	for modelID, model := range response.Models {
		if model.IsInternal || model.QuotaInfo == nil || model.QuotaInfo.RemainingFraction == nil {
			continue
		}

		remaining := max(0, min(1, *model.QuotaInfo.RemainingFraction))
		usageRatio := 1 - remaining
		status := antigravityQuotaStatus(usageRatio)
		resetAt := parseAntigravityResetTime(model.QuotaInfo.ResetTime)
		if resetAt != nil && (nextResetAt == nil || resetAt.Before(*nextResetAt)) {
			nextResetAt = resetAt
		}
		maxRemaining = max(maxRemaining, remaining)

		modelData := map[string]any{
			"displayName":         model.DisplayName,
			"remainingPercentage": remaining * 100,
			"status":              status,
		}
		if resetAt != nil {
			modelData["resetAt"] = resetAt.Format(time.RFC3339)
		}
		models[modelID] = modelData
	}

	if len(models) == 0 {
		return QuotaData{}, fmt.Errorf("Antigravity quota response has no quota data")
	}
	overallStatus := antigravityQuotaStatus(1 - maxRemaining)

	return QuotaData{
		Status:       overallStatus,
		ProviderType: "antigravity",
		RawData:      map[string]any{"models": models},
		NextResetAt:  nextResetAt,
		Ready:        IsReadyStatus(overallStatus),
	}, nil
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
