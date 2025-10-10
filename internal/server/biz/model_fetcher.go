package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// ModelFetcher handles fetching models from provider APIs.
type ModelFetcher struct {
	httpClient     *httpclient.HttpClient
	channelService *ChannelService
}

// NewModelFetcher creates a new ModelFetcher instance.
func NewModelFetcher(httpClient *httpclient.HttpClient, channelService *ChannelService) *ModelFetcher {
	return &ModelFetcher{
		httpClient:     httpClient,
		channelService: channelService,
	}
}

// FetchModelsInput represents the input for fetching models.
type FetchModelsInput struct {
	ChannelType string
	BaseURL     string
	APIKey      *string
	ChannelID   *int
}

// FetchModelsResult represents the result of fetching models.
type FetchModelsResult struct {
	Models []objects.LLMModel
	Error  *string
}

// FetchModels fetches available models from the provider API.
func (f *ModelFetcher) FetchModels(ctx context.Context, input FetchModelsInput) (*FetchModelsResult, error) {
	// Get API key from channel if not provided
	apiKey := ""
	if input.APIKey != nil && *input.APIKey != "" {
		apiKey = *input.APIKey
	} else if input.ChannelID != nil {
		// Query channel to get API key
		ctx = privacy.DecisionContext(ctx, privacy.Allow)

		ch, err := f.channelService.Ent.Channel.Get(ctx, *input.ChannelID)
		if err != nil {
			return &FetchModelsResult{
				Models: []objects.LLMModel{},
				Error:  lo.ToPtr(fmt.Sprintf("failed to get channel: %v", err)),
			}, nil
		}

		apiKey = ch.Credentials.APIKey
	}

	if apiKey == "" {
		return &FetchModelsResult{
			Models: []objects.LLMModel{},
			Error:  lo.ToPtr("API key is required"),
		}, nil
	}

	// Validate channel type
	channelType := channel.Type(input.ChannelType)
	if err := channel.TypeValidator(channelType); err != nil {
		return &FetchModelsResult{
			Models: []objects.LLMModel{},
			Error:  lo.ToPtr(fmt.Sprintf("invalid channel type: %v", err)),
		}, nil
	}

	// Determine the models endpoint and auth header based on channel type
	modelsURL, authHeaders := f.prepareModelsEndpoint(channelType, input.BaseURL)

	// Build HTTP request
	req := &httpclient.Request{
		Method:  http.MethodGet,
		URL:     modelsURL,
		Headers: authHeaders,
	}

	// For Anthropic, also add x-api-key header
	if channelType.IsAnthropic() {
		req.Headers.Set("X-Api-Key", apiKey)
	} else {
		req.Headers.Set("Authorization", "Bearer "+apiKey)
	}

	// Execute request
	resp, err := f.httpClient.Do(ctx, req)
	if err != nil {
		return &FetchModelsResult{
			Models: []objects.LLMModel{},
			Error:  lo.ToPtr(fmt.Sprintf("failed to fetch models: %v", err)),
		}, nil
	}

	// Parse response based on provider
	models, err := f.parseModelsResponse(resp.Body)
	if err != nil {
		return &FetchModelsResult{
			Models: []objects.LLMModel{},
			Error:  lo.ToPtr(fmt.Sprintf("failed to parse models response: %v", err)),
		}, nil
	}

	return &FetchModelsResult{
		Models: models,
		Error:  nil,
	}, nil
}

// prepareModelsEndpoint returns the models endpoint URL and auth headers for the given channel type.
func (f *ModelFetcher) prepareModelsEndpoint(channelType channel.Type, baseURL string) (string, http.Header) {
	headers := make(http.Header)

	// Ensure baseURL ends with /
	baseURL = strings.TrimSuffix(baseURL, "/")

	switch {
	case channelType.IsAnthropic():
		// Anthropic API
		headers.Set("Anthropic-Version", "2023-06-01")
		return baseURL + "/models", headers
	case channelType == channel.TypeZhipuAnthropic || channelType == channel.TypeZaiAnthropic:
		baseURL = strings.TrimSuffix(baseURL, "/anthropic")
		return baseURL + "/paas/v4/models", headers
	case channelType.IsAnthropicLike():
		baseURL = strings.TrimSuffix(baseURL, "/anthropic")
		return baseURL + "/v1/models", headers
	default:
		return baseURL + "/models", headers
	}
}

// parseModelsResponse parses the models response from the provider API.
func (f *ModelFetcher) parseModelsResponse(body []byte) ([]objects.LLMModel, error) {
	// Most providers use OpenAI-compatible format
	var response struct {
		Data []objects.LLMModel `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response.Data, nil
}
