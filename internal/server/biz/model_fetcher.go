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
	Models []objects.ModelIdentify
	Error  *string
}

// FetchModels fetches available models from the provider API.
func (f *ModelFetcher) FetchModels(ctx context.Context, input FetchModelsInput) (*FetchModelsResult, error) {
	// do not support volcengine for now.
	if input.ChannelType == channel.TypeVolcengine.String() {
		return &FetchModelsResult{
			Models: []objects.ModelIdentify{},
		}, nil
	}
	// Get API key from channel if not provided
	apiKey := ""
	if input.APIKey != nil && *input.APIKey != "" {
		apiKey = *input.APIKey
	} else if input.ChannelID != nil {
		// Query channel to get API key
		ctx = privacy.DecisionContext(ctx, privacy.Allow)

		ch, err := f.channelService.entFromContext(ctx).Channel.Get(ctx, *input.ChannelID)
		if err != nil {
			return &FetchModelsResult{
				Models: []objects.ModelIdentify{},
				Error:  lo.ToPtr(fmt.Sprintf("failed to get channel: %v", err)),
			}, nil
		}

		apiKey = ch.Credentials.APIKey
	}

	if apiKey == "" {
		return &FetchModelsResult{
			Models: []objects.ModelIdentify{},
			Error:  lo.ToPtr("API key is required"),
		}, nil
	}

	// Validate channel type
	channelType := channel.Type(input.ChannelType)
	if err := channel.TypeValidator(channelType); err != nil {
		return &FetchModelsResult{
			Models: []objects.ModelIdentify{},
			Error:  lo.ToPtr(fmt.Sprintf("invalid channel type: %v", err)),
		}, nil
	}

	modelsURL, authHeaders := f.prepareModelsEndpoint(channelType, input.BaseURL)

	req := &httpclient.Request{
		Method:  http.MethodGet,
		URL:     modelsURL,
		Headers: authHeaders,
	}

	if channelType.IsAnthropic() {
		req.Headers.Set("X-Api-Key", apiKey)
	} else if channelType.IsGemini() {
		req.Headers.Set("X-Goog-Api-Key", apiKey)
	} else {
		req.Headers.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := f.httpClient.Do(ctx, req)
	if err != nil {
		return &FetchModelsResult{
			Models: []objects.ModelIdentify{},
			Error:  lo.ToPtr(fmt.Sprintf("failed to fetch models: %v", err)),
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return &FetchModelsResult{
			Models: []objects.ModelIdentify{},
			Error:  lo.ToPtr(fmt.Sprintf("failed to fetch models: %v", resp.StatusCode)),
		}, nil
	}

	models, err := f.parseModelsResponse(resp.Body)
	if err != nil {
		return &FetchModelsResult{
			Models: []objects.ModelIdentify{},
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

	baseURL = strings.TrimSuffix(baseURL, "/")

	switch {
	case channelType.IsAnthropic():
		headers.Set("Anthropic-Version", "2023-06-01")

		baseURL = strings.TrimSuffix(baseURL, "/anthropic")
		baseURL = strings.TrimSuffix(baseURL, "/claude")

		if strings.HasSuffix(baseURL, "/v1") {
			return baseURL + "/models", headers
		}

		return baseURL + "/v1/models", headers
	case channelType == channel.TypeZhipuAnthropic || channelType == channel.TypeZaiAnthropic:
		baseURL = strings.TrimSuffix(baseURL, "/anthropic")
		return baseURL + "/paas/v4/models", headers
	case channelType == channel.TypeZai || channelType == channel.TypeZhipu:
		baseURL = strings.TrimSuffix(baseURL, "/v4")
		return baseURL + "/v4/models", headers
	case channelType.IsAnthropicLike():
		baseURL = strings.TrimSuffix(baseURL, "/anthropic")
		baseURL = strings.TrimSuffix(baseURL, "/claude")

		return baseURL + "/v1/models", headers
	case channelType.IsGemini():
		if strings.HasSuffix(baseURL, "/v1beta") {
			return baseURL + "/models", headers
		}

		if strings.HasSuffix(baseURL, "/v1") {
			return baseURL + "/models", headers
		}

		return baseURL + "/v1beta/models", headers
	default:
		if strings.HasSuffix(baseURL, "/v1") {
			return baseURL + "/models", headers
		}

		return baseURL + "/v1/models", headers
	}
}

type GeminiModelResponse struct {
	Name        string `json:"name"`
	BaseModelID string `json:"baseModelId"`
	Version     string `json:"version"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
}

// parseModelsResponse parses the models response from the provider API.
func (f *ModelFetcher) parseModelsResponse(body []byte) ([]objects.ModelIdentify, error) {
	// Most providers use OpenAI-compatible format
	var response struct {
		Data   []objects.ModelIdentify `json:"data"`
		Models []GeminiModelResponse   `json:"models"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(response.Models) > 0 {
		for _, model := range response.Models {
			// remove "models/" prefix for gemini.
			response.Data = append(response.Data, objects.ModelIdentify{
				ID: strings.TrimPrefix(model.Name, "models/"),
			})
		}
	}

	return response.Data, nil
}
