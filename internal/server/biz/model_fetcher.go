package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/anthropic/claudecode"
	"github.com/looplj/axonhub/llm/transformer/antigravity"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
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
	Models []ModelIdentify
	Error  *string
}

// FetchModels fetches available models from the provider API.
func (f *ModelFetcher) FetchModels(ctx context.Context, input FetchModelsInput) (*FetchModelsResult, error) {
	// do not support volcengine for now.
	if input.ChannelType == channel.TypeVolcengine.String() {
		return &FetchModelsResult{
			Models: []ModelIdentify{},
		}, nil
	}

	if input.ChannelType == channel.TypeAntigravity.String() {
		models := lo.Map(antigravity.DefaultModels(), func(id string, _ int) ModelIdentify {
			return ModelIdentify{ID: id}
		})

		return &FetchModelsResult{
			Models: models,
			Error:  nil,
		}, nil
	}

	var (
		apiKey      string
		proxyConfig *httpclient.ProxyConfig
	)

	if input.APIKey != nil && *input.APIKey != "" {
		apiKey = *input.APIKey
	}

	if input.ChannelID != nil {
		ctx = privacy.DecisionContext(ctx, privacy.Allow)

		ch, err := f.channelService.entFromContext(ctx).Channel.Get(ctx, *input.ChannelID)
		if err != nil {
			return &FetchModelsResult{
				Models: []ModelIdentify{},
				Error:  lo.ToPtr(fmt.Sprintf("failed to get channel: %v", err)),
			}, nil
		}

		if ch.Credentials.IsOAuth() {
			//nolint:exhaustive // only support codex and claudecode for now.
			switch ch.Type {
			case channel.TypeCodex:
				models := lo.Map(codex.DefaultModels(), func(id string, _ int) ModelIdentify { return ModelIdentify{ID: id} })

				return &FetchModelsResult{
					Models: models,
					Error:  nil,
				}, nil
			case channel.TypeClaudecode:
				models := lo.Map(claudecode.DefaultModels(), func(id string, _ int) ModelIdentify { return ModelIdentify{ID: id} })

				return &FetchModelsResult{
					Models: models,
					Error:  nil,
				}, nil
			}
		}

		if apiKey == "" {
			apiKey = ch.Credentials.APIKey
			if apiKey == "" && len(ch.Credentials.APIKeys) > 0 {
				apiKey = ch.Credentials.APIKeys[0]
			}
		}

		if ch.Settings != nil {
			proxyConfig = ch.Settings.Proxy
		}
	}

	if apiKey == "" {
		return &FetchModelsResult{
			Models: []ModelIdentify{},
			Error:  lo.ToPtr("API key is required"),
		}, nil
	}

	if isOAuthJSON(apiKey) {
		//nolint:exhaustive // only support codex and claudecode for now.
		switch input.ChannelType {
		case channel.TypeCodex.String():
			models := lo.Map(codex.DefaultModels(), func(id string, _ int) ModelIdentify { return ModelIdentify{ID: id} })
			return &FetchModelsResult{
				Models: models,
				Error:  nil,
			}, nil
		case channel.TypeClaudecode.String():
			models := lo.Map(claudecode.DefaultModels(), func(id string, _ int) ModelIdentify { return ModelIdentify{ID: id} })
			return &FetchModelsResult{
				Models: models,
				Error:  nil,
			}, nil
		}
	}

	// Validate channel type
	channelType := channel.Type(input.ChannelType)
	if err := channel.TypeValidator(channelType); err != nil {
		return &FetchModelsResult{
			Models: []ModelIdentify{},
			Error:  lo.ToPtr(fmt.Sprintf("invalid channel type: %v", err)),
		}, nil
	}

	modelsURL, authHeaders := f.prepareModelsEndpoint(channelType, input.BaseURL)

	req := &httpclient.Request{
		Method:  http.MethodGet,
		URL:     modelsURL,
		Headers: authHeaders,
	}

	if channelType.IsAnthropic() || channelType.IsAnthropicLike() {
		req.Headers.Set("X-Api-Key", apiKey)
	} else if channelType.IsGemini() {
		req.Headers.Set("X-Goog-Api-Key", apiKey)
	} else {
		req.Headers.Set("Authorization", "Bearer "+apiKey)
	}

	var httpClient *httpclient.HttpClient
	if proxyConfig != nil {
		httpClient = httpclient.NewHttpClientWithProxy(proxyConfig)
	} else {
		httpClient = f.httpClient
	}

	if channelType.IsGemini() {
		models, err := f.fetchGeminiModels(ctx, httpClient, req)
		if err != nil {
			return &FetchModelsResult{
				Models: []ModelIdentify{},
				Error:  lo.ToPtr(fmt.Sprintf("failed to fetch models: %v", err)),
			}, nil
		}

		return &FetchModelsResult{
			Models: lo.Uniq(models),
			Error:  nil,
		}, nil
	}

	var (
		resp *httpclient.Response
		err  error
	)

	if channelType.IsAnthropic() || channelType.IsAnthropicLike() {
		resp, err = httpClient.Do(ctx, req)
		if err != nil || resp.StatusCode != http.StatusOK {
			req.Headers.Del("X-Api-Key")
			req.Headers.Set("Authorization", "Bearer "+apiKey)
			resp, err = httpClient.Do(ctx, req)
		}
	} else {
		resp, err = httpClient.Do(ctx, req)
	}

	if err != nil {
		return &FetchModelsResult{
			Models: []ModelIdentify{},
			Error:  lo.ToPtr(fmt.Sprintf("failed to fetch models: %v", err)),
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return &FetchModelsResult{
			Models: []ModelIdentify{},
			Error:  lo.ToPtr(fmt.Sprintf("failed to fetch models: %v", resp.StatusCode)),
		}, nil
	}

	models, err := f.parseModelsResponse(resp.Body)
	if err != nil {
		return &FetchModelsResult{
			Models: []ModelIdentify{},
			Error:  lo.ToPtr(fmt.Sprintf("failed to parse models response: %v", err)),
		}, nil
	}

	return &FetchModelsResult{
		Models: lo.Uniq(models),
		Error:  nil,
	}, nil
}

type geminiListModelsResponse struct {
	Models        []GeminiModelResponse `json:"models"`
	NextPageToken string                `json:"nextPageToken"`
}

func (f *ModelFetcher) fetchGeminiModels(ctx context.Context, httpClient *httpclient.HttpClient, req *httpclient.Request) ([]ModelIdentify, error) {
	const maxPages = 50
	const pageSize = 1000

	allModels := make([]ModelIdentify, 0, 128)
	pageToken := ""
	seenTokens := make(map[string]struct{}, 8)

	for i := 0; i < maxPages; i++ {
		pageURL, err := withGeminiModelsPagination(req.URL, pageSize, pageToken)
		if err != nil {
			return nil, err
		}

		req.URL = pageURL

		resp, err := httpClient.Do(ctx, req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected status: %s", resp.RawResponse.Status)
		}

		var page geminiListModelsResponse
		if err := json.Unmarshal(resp.Body, &page); err != nil {
			models, parseErr := f.parseModelsResponse(resp.Body)
			if parseErr != nil {
				return nil, fmt.Errorf("failed to parse models response: paginated unmarshal: %w; fallback parse: %w", err, parseErr)
			}
			allModels = append(allModels, models...)
			return allModels, nil
		}

		for _, model := range page.Models {
			allModels = append(allModels, ModelIdentify{
				ID: strings.TrimPrefix(model.Name, "models/"),
			})
		}

		if page.NextPageToken == "" {
			return allModels, nil
		}

		if _, ok := seenTokens[page.NextPageToken]; ok {
			return allModels, nil
		}

		seenTokens[page.NextPageToken] = struct{}{}
		pageToken = page.NextPageToken
	}

	return allModels, nil
}

func withGeminiModelsPagination(modelsURL string, pageSize int, pageToken string) (string, error) {
	parsed, err := url.Parse(modelsURL)
	if err != nil {
		return "", err
	}

	query := parsed.Query()
	if pageSize > 0 {
		query.Set("pageSize", strconv.Itoa(pageSize))
	}
	if pageToken != "" {
		query.Set("pageToken", pageToken)
	} else {
		query.Del("pageToken")
	}

	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// prepareModelsEndpoint returns the models endpoint URL and auth headers for the given channel type.
func (f *ModelFetcher) prepareModelsEndpoint(channelType channel.Type, baseURL string) (string, http.Header) {
	headers := make(http.Header)

	baseURL = strings.TrimSuffix(baseURL, "/")

	useRawURL := false

	if before, ok := strings.CutSuffix(baseURL, "#"); ok {
		baseURL = before
		useRawURL = true
	}

	switch {
	case channelType.IsAnthropic():
		headers.Set("Anthropic-Version", "2023-06-01")

		baseURL = strings.TrimSuffix(baseURL, "/anthropic")
		baseURL = strings.TrimSuffix(baseURL, "/claude")

		if useRawURL {
			return baseURL + "/models", headers
		}

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
		if strings.Contains(baseURL, "/v1") {
			return baseURL + "/models", headers
		}

		return baseURL + "/v1beta/models", headers
	case channelType == channel.TypeGithub:
		// GitHub Models uses a separate catalog endpoint
		return "https://models.github.ai/catalog/models", headers
	default:
		if useRawURL {
			return baseURL + "/models", headers
		}

		if strings.Contains(baseURL, "/v1") {
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

type commonModelsResponse struct {
	Data   []ModelIdentify       `json:"data"`
	Models []GeminiModelResponse `json:"models"`
}

var jsonArrayRegex = regexp.MustCompile(`\[[^\]]*\]`)

// ExtractJSONArray uses regex to extract JSON array from body and unmarshal to target.
func ExtractJSONArray(body []byte, target any) error {
	matches := jsonArrayRegex.FindAll(body, -1)
	if len(matches) == 0 {
		return fmt.Errorf("no JSON array found in response")
	}

	for _, match := range matches {
		if err := json.Unmarshal(match, target); err == nil {
			return nil
		}
	}

	return fmt.Errorf("failed to unmarshal any JSON array")
}

// parseModelsResponse parses the models response from the provider API.
func (f *ModelFetcher) parseModelsResponse(body []byte) ([]ModelIdentify, error) {
	// First, try to parse as direct array (e.g., GitHub Models response)
	var directArray []ModelIdentify
	if err := json.Unmarshal(body, &directArray); err == nil && len(directArray) > 0 {
		return directArray, nil
	}

	var response commonModelsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		if err := ExtractJSONArray(body, &response.Data); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
	}

	if len(response.Models) > 0 {
		for _, model := range response.Models {
			// remove "models/" prefix for gemini.
			response.Data = append(response.Data, ModelIdentify{
				ID: strings.TrimPrefix(model.Name, "models/"),
			})
		}
	}

	return response.Data, nil
}
