package provider_quota

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
)

const zaiDefaultQuotaBaseURL = "https://api.z.ai"

type ZaiQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

func NewZaiQuotaChecker(httpClient *httpclient.HttpClient) *ZaiQuotaChecker {
	return &ZaiQuotaChecker{httpClient: httpClient}
}

func (c *ZaiQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	apiKey := zhipuAPIKey(ch)
	if apiKey == "" {
		return QuotaData{}, fmt.Errorf("channel has no API key")
	}

	request := httpclient.NewRequestBuilder().
		WithMethod("GET").
		WithURL(zaiDefaultQuotaBaseURL+"/api/monitor/usage/quota/limit").
		WithHeader("Authorization", apiKey).
		WithHeader("Content-Type", "application/json").
		WithHeader("Accept-Language", "en-US,en").
		Build()

	hc := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = c.httpClient.WithProxy(ch.Settings.Proxy)
	}

	resp, err := hc.Do(ctx, request)
	if err != nil {
		return QuotaData{}, fmt.Errorf("zai quota request failed: %w", err)
	}

	return parseZaiQuotaResponse(resp.Body)
}

func (c *ZaiQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	return ch.Type == channel.TypeZai ||
		ch.Type == channel.TypeZaiAnthropic
}

func parseZaiQuotaResponse(body []byte) (QuotaData, error) {
	return parseZhipuFamilyQuotaResponse(body, "zai")
}
