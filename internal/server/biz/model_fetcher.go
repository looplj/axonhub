package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/anthropic/claudecode"
	"github.com/looplj/axonhub/llm/transformer/antigravity"
	"github.com/looplj/axonhub/llm/transformer/gemini/vertex"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
	"github.com/looplj/axonhub/llm/transformer/openai/copilot"
)

const providerConfCacheDuration = 1 * time.Hour

// ModelFetcher handles fetching models from provider APIs.
type ModelFetcher struct {
	httpClient          *httpclient.HttpClient
	channelService      *ChannelService
	copilotFetcher      *providerConfFetcher
	geminiVertexFetcher *providerConfFetcher
}

// providerConfFetcher handles fetching models from PublicProviderConf with caching.
type providerConfFetcher struct {
	modelsCache    []ModelIdentify
	cacheMu        sync.RWMutex
	cacheTimestamp time.Time
	cacheDuration  time.Duration
	providerURL    string
}

// fetch fetches models with caching using double-check locking.
func (f *providerConfFetcher) fetch(ctx context.Context, httpClient *httpclient.HttpClient) []ModelIdentify {
	f.cacheMu.RLock()
	if len(f.modelsCache) > 0 && time.Since(f.cacheTimestamp) < f.cacheDuration {
		models := make([]ModelIdentify, len(f.modelsCache))
		copy(models, f.modelsCache)
		f.cacheMu.RUnlock()
		return models
	}
	f.cacheMu.RUnlock()

	f.cacheMu.Lock()
	defer f.cacheMu.Unlock()

	// Double-check after acquiring write lock
	if len(f.modelsCache) > 0 && time.Since(f.cacheTimestamp) < f.cacheDuration {
		models := make([]ModelIdentify, len(f.modelsCache))
		copy(models, f.modelsCache)
		return models
	}

	models, err := f.fetchFromSource(ctx, httpClient)
	if err != nil {
		slog.Error("failed to fetch models from source", "providerURL", f.providerURL, "error", err)
		// If fetch failed but cache exists, return defensive copy
		if len(f.modelsCache) > 0 {
			cached := make([]ModelIdentify, len(f.modelsCache))
			copy(cached, f.modelsCache)
			return cached
		}
		return nil
	}
	if len(models) > 0 {
		// Store a copy in cache to avoid shared backing array
		f.modelsCache = make([]ModelIdentify, len(models))
		copy(f.modelsCache, models)
		f.cacheTimestamp = time.Now()

		// Return a copy to callers
		copied := make([]ModelIdentify, len(models))
		copy(copied, models)
		return copied
	}

	return nil
}

// fetchFromSource fetches models from PublicProviderConf.
func (f *providerConfFetcher) fetchFromSource(ctx context.Context, httpClient *httpclient.HttpClient) ([]ModelIdentify, error) {
	req := &httpclient.Request{
		Method: http.MethodGet,
		URL:    f.providerURL,
		Headers: http.Header{
			"Accept": []string{"application/json"},
		},
	}

	resp, err := httpClient.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch models: non-OK status %d: %s", resp.StatusCode, string(resp.Body))
	}

	type providerConfResponse struct {
		ID     string `json:"id"`
		Models []struct {
			ID string `json:"id"`
		} `json:"models"`
	}

	var conf providerConfResponse
	if err := json.Unmarshal(resp.Body, &conf); err != nil {
		return nil, fmt.Errorf("failed to parse provider conf: %w", err)
	}

	if conf.ID == "" {
		return nil, fmt.Errorf("provider ID not found in response")
	}

	// Build models slice, filtering out empty IDs
	models := make([]ModelIdentify, 0, len(conf.Models))
	for _, m := range conf.Models {
		if m.ID != "" {
			models = append(models, ModelIdentify{ID: m.ID})
		}
	}

	return models, nil
}

// NewModelFetcher creates a new ModelFetcher instance.
func NewModelFetcher(httpClient *httpclient.HttpClient, channelService *ChannelService) *ModelFetcher {
	return &ModelFetcher{
		httpClient:     httpClient,
		channelService: channelService,
		copilotFetcher: &providerConfFetcher{
			cacheDuration: providerConfCacheDuration,
			providerURL:   copilot.ProviderConfURL,
		},
		geminiVertexFetcher: &providerConfFetcher{
			cacheDuration: providerConfCacheDuration,
			providerURL:   vertex.ProviderConfURL,
		},
	}
}

// FetchModelsInput represents the input for fetching models.
type FetchModelsInput struct {
	ChannelType string
	BaseURL     string
	//nolint:gosec // G117: Field name contains "APIKey" but this is input data, not a hardcoded secret
	APIKey    *string
	ChannelID *int
}

// FetchModelsResult represents the result of fetching models.
type FetchModelsResult struct {
	Models []ModelIdentify
	Error  *string
}

// FetchModels fetches available models from the provider API.
func (f *ModelFetcher) getDefaultModels(ctx context.Context, channelType string) []ModelIdentify {
	return f.getDefaultModelsByType(ctx, channel.Type(channelType))
}

func (f *ModelFetcher) getDefaultModelsByType(ctx context.Context, typ channel.Type) []ModelIdentify {
	//nolint:exhaustive // only supports default model fetching for specific channel types (antigravity, codex, claudecode, github_copilot, gemini_vertex).
	switch typ {
	case channel.TypeAntigravity:
		return lo.Map(antigravity.DefaultModels(), func(id string, _ int) ModelIdentify { return ModelIdentify{ID: id} })
	case channel.TypeCodex:
		return lo.Map(codex.DefaultModels(), func(id string, _ int) ModelIdentify { return ModelIdentify{ID: id} })
	case channel.TypeClaudecode:
		return lo.Map(claudecode.DefaultModels(), func(id string, _ int) ModelIdentify { return ModelIdentify{ID: id} })
	case channel.TypeGithubCopilot:
		return f.fetchCopilotModels(ctx)
	case channel.TypeGeminiVertex:
		return f.fetchGeminiVertexModels(ctx)
	default:
		return nil
	}
}

// fetchCopilotModels fetches GitHub Copilot models from PublicProviderConf with caching.
func (f *ModelFetcher) fetchCopilotModels(ctx context.Context) []ModelIdentify {
	return f.copilotFetcher.fetch(ctx, f.httpClient)
}

// fetchGeminiVertexModels fetches Gemini Vertex models from PublicProviderConf with caching.
func (f *ModelFetcher) fetchGeminiVertexModels(ctx context.Context) []ModelIdentify {
	return f.geminiVertexFetcher.fetch(ctx, f.httpClient)
}

func (f *ModelFetcher) tryReturnDefaultModels(ctx context.Context, channelType string) (*FetchModelsResult, bool) {
	models := f.getDefaultModels(ctx, channelType)
	if models != nil {
		return &FetchModelsResult{Models: models}, true
	}

	return nil, false
}

func (f *ModelFetcher) FetchModels(ctx context.Context, input FetchModelsInput) (*FetchModelsResult, error) {
	if input.ChannelType == channel.TypeVolcengine.String() {
		return &FetchModelsResult{
			Models: []ModelIdentify{},
		}, nil
	}

	if result, ok := f.tryReturnDefaultModels(ctx, input.ChannelType); ok {
		return result, nil
	}

	var (
		apiKey      string
		proxyConfig *httpclient.ProxyConfig
	)

	if input.APIKey != nil && *input.APIKey != "" {
		apiKey = *input.APIKey
	}

	if input.ChannelID != nil {
		ch, err := f.channelService.entFromContext(ctx).Channel.Get(ctx, *input.ChannelID)
		if err != nil {
			return &FetchModelsResult{
				Models: []ModelIdentify{},
				Error:  lo.ToPtr(fmt.Sprintf("failed to get channel: %v", err)),
			}, nil
		}

		if ch.Credentials.IsOAuth() {
			if models := f.getDefaultModelsByType(ctx, ch.Type); models != nil {
				return &FetchModelsResult{Models: models}, nil
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
		if result, ok := f.tryReturnDefaultModels(ctx, input.ChannelType); ok {
			return result, nil
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

	// GitHub Copilot uses cached provider conf instead of API endpoint
	if channelType == channel.TypeGithubCopilot {
		models := f.fetchCopilotModels(ctx)
		if models == nil {
			return &FetchModelsResult{
				Models: []ModelIdentify{},
				Error:  lo.ToPtr("failed to fetch copilot models"),
			}, nil
		}
		return &FetchModelsResult{
			Models: models,
			Error:  nil,
		}, nil
	}

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
	case channelType == channel.TypeDoubao || channelType == channel.TypeVolcengine:
		baseURL = strings.TrimSuffix(baseURL, "/v3")
		return baseURL + "/v3/models", headers
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
	case channelType == channel.TypeGithubCopilot:
		// GitHub Copilot models are fetched from cached provider conf, not via API endpoint
		// Return empty URL to indicate no direct model API - use fetchCopilotModels instead
		return "", headers
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
