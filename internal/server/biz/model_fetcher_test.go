package biz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestExtractJSONArray(t *testing.T) {
	tests := []struct {
		name        string
		body        []byte
		target      any
		expectError bool
	}{
		{
			name:        "valid JSON array with data field",
			body:        []byte(`{"data":[{"id":"model1"},{"id":"model2"}]}`),
			target:      &[]ModelIdentify{},
			expectError: false,
		},
		{
			name:        "valid JSON array without object wrapper",
			body:        []byte(`[{"id":"model1"},{"id":"model2"},{"id":"model3"}]`),
			target:      &[]ModelIdentify{},
			expectError: false,
		},
		{
			name:        "JSON array with nested arrays",
			body:        []byte(`{"data":[],"models":[{"name":"models/gemini-pro"}]}`),
			target:      &[]ModelIdentify{},
			expectError: false,
		},
		{
			name:        "no JSON array in body",
			body:        []byte(`{"object":"value"}`),
			target:      &[]ModelIdentify{},
			expectError: true,
		},
		{
			name:        "empty JSON array",
			body:        []byte(`[]`),
			target:      &[]ModelIdentify{},
			expectError: false,
		},
		{
			name:        "JSON array with single element",
			body:        []byte(`[{"id":"single-model"}]`),
			target:      &[]ModelIdentify{},
			expectError: false,
		},
		{
			name:        "complex response with multiple arrays",
			body:        []byte(`{"data":[{"id":"model1"}],"extra":[{"key":"value"}],"models":[]}`),
			target:      &[]ModelIdentify{},
			expectError: false,
		},
		{
			name:        "invalid JSON array structure",
			body:        []byte(`[invalid json]`),
			target:      &[]ModelIdentify{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ExtractJSONArray(tt.body, tt.target)

			if tt.expectError {
				if err == nil {
					t.Errorf("extractJSONArray() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("extractJSONArray() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestExtractJSONArrayWithModelIdentify(t *testing.T) {
	t.Run("extract and verify models", func(t *testing.T) {
		body := []byte(`{"data":[{"id":"gpt-4"},{"id":"gpt-3.5-turbo"},{"id":"claude-3"}]}`)

		var models []ModelIdentify

		err := ExtractJSONArray(body, &models)
		if err != nil {
			t.Fatalf("extractJSONArray() error: %v", err)
		}

		if len(models) != 3 {
			t.Errorf("expected 3 models, got %d", len(models))
		}

		expectedIDs := []string{"gpt-4", "gpt-3.5-turbo", "claude-3"}
		for i, model := range models {
			if model.ID != expectedIDs[i] {
				t.Errorf("model[%d].ID = %q, want %q", i, model.ID, expectedIDs[i])
			}
		}
	})

	t.Run("extract from plain array", func(t *testing.T) {
		body := []byte(`[{"id":"model-a"},{"id":"model-b"}]`)

		var models []ModelIdentify

		err := ExtractJSONArray(body, &models)
		if err != nil {
			t.Fatalf("extractJSONArray() error: %v", err)
		}

		if len(models) != 2 {
			t.Errorf("expected 2 models, got %d", len(models))
		}
	})

	t.Run("extract empty array", func(t *testing.T) {
		body := []byte(`[]`)

		var models []ModelIdentify

		err := ExtractJSONArray(body, &models)
		if err != nil {
			t.Fatalf("extractJSONArray() error: %v", err)
		}

		if len(models) != 0 {
			t.Errorf("expected 0 models, got %d", len(models))
		}
	})
}

func TestExtractJSONArrayWithGeminiModels(t *testing.T) {
	t.Run("extract gemini models array", func(t *testing.T) {
		body := []byte(`{"models":[{"name":"models/gemini-pro","displayName":"Gemini Pro"}]}`)

		var models []GeminiModelResponse

		err := ExtractJSONArray(body, &models)
		if err != nil {
			t.Fatalf("extractJSONArray() error: %v", err)
		}

		if len(models) != 1 {
			t.Errorf("expected 1 model, got %d", len(models))
		}

		if models[0].Name != "models/gemini-pro" {
			t.Errorf("model.Name = %q, want %q", models[0].Name, "models/gemini-pro")
		}
	})
}

func TestPrepareModelsEndpoint(t *testing.T) {
	fetcher := NewModelFetcher(nil, nil)

	tests := []struct {
		name        string
		channelType channel.Type
		baseURL     string
		expectedURL string
		checkHeader bool
		headerKey   string
		headerValue string
	}{
		{
			name:        "Anthropic with /v1 suffix",
			channelType: channel.TypeAnthropic,
			baseURL:     "https://api.anthropic.com/v1",
			expectedURL: "https://api.anthropic.com/v1/models",
			checkHeader: true,
			headerKey:   "Anthropic-Version",
			headerValue: "2023-06-01",
		},
		{
			name:        "Anthropic without /v1 suffix",
			channelType: channel.TypeAnthropic,
			baseURL:     "https://api.anthropic.com",
			expectedURL: "https://api.anthropic.com/v1/models",
			checkHeader: true,
			headerKey:   "Anthropic-Version",
			headerValue: "2023-06-01",
		},
		{
			name:        "Anthropic with /anthropic suffix",
			channelType: channel.TypeAnthropic,
			baseURL:     "https://api.example.com/anthropic",
			expectedURL: "https://api.example.com/v1/models",
			checkHeader: true,
			headerKey:   "Anthropic-Version",
			headerValue: "2023-06-01",
		},
		{
			name:        "Anthropic with /claude suffix",
			channelType: channel.TypeAnthropic,
			baseURL:     "https://api.example.com/claude",
			expectedURL: "https://api.example.com/v1/models",
			checkHeader: true,
			headerKey:   "Anthropic-Version",
			headerValue: "2023-06-01",
		},
		{
			name:        "Anthropic with raw URL marker (#)",
			channelType: channel.TypeAnthropic,
			baseURL:     "https://api.example.com/custom/path#",
			expectedURL: "https://api.example.com/custom/path/models",
			checkHeader: true,
			headerKey:   "Anthropic-Version",
			headerValue: "2023-06-01",
		},
		{
			name:        "ZhipuAnthropic",
			channelType: channel.TypeZhipuAnthropic,
			baseURL:     "https://open.bigmodel.cn/api/anthropic",
			expectedURL: "https://open.bigmodel.cn/api/paas/v4/models",
		},
		{
			name:        "ZaiAnthropic",
			channelType: channel.TypeZaiAnthropic,
			baseURL:     "https://api.zai.ai/anthropic",
			expectedURL: "https://api.zai.ai/paas/v4/models",
		},
		{
			name:        "Zai",
			channelType: channel.TypeZai,
			baseURL:     "https://api.zai.ai/v4",
			expectedURL: "https://api.zai.ai/v4/models",
		},
		{
			name:        "Zhipu",
			channelType: channel.TypeZhipu,
			baseURL:     "https://open.bigmodel.cn/api",
			expectedURL: "https://open.bigmodel.cn/api/v4/models",
		},
		{
			name:        "DeepseekAnthropic",
			channelType: channel.TypeDeepseekAnthropic,
			baseURL:     "https://api.deepseek.com/anthropic",
			expectedURL: "https://api.deepseek.com/v1/models",
		},
		{
			name:        "MoonshotAnthropic",
			channelType: channel.TypeMoonshotAnthropic,
			baseURL:     "https://api.moonshot.cn/claude",
			expectedURL: "https://api.moonshot.cn/v1/models",
		},
		{
			name:        "Gemini with /v1 suffix",
			channelType: channel.TypeGemini,
			baseURL:     "https://generativelanguage.googleapis.com/v1",
			expectedURL: "https://generativelanguage.googleapis.com/v1/models",
		},
		{
			name:        "Gemini without /v1 suffix",
			channelType: channel.TypeGemini,
			baseURL:     "https://generativelanguage.googleapis.com",
			expectedURL: "https://generativelanguage.googleapis.com/v1beta/models",
		},
		{
			name:        "OpenAI with /v1 suffix",
			channelType: channel.TypeOpenai,
			baseURL:     "https://api.openai.com/v1",
			expectedURL: "https://api.openai.com/v1/models",
		},
		{
			name:        "OpenAI without /v1 suffix",
			channelType: channel.TypeOpenai,
			baseURL:     "https://api.openai.com",
			expectedURL: "https://api.openai.com/v1/models",
		},
		{
			name:        "OpenAI with raw URL marker (#)",
			channelType: channel.TypeOpenai,
			baseURL:     "https://custom.api.com/custom/path#",
			expectedURL: "https://custom.api.com/custom/path/models",
		},
		{
			name:        "Deepseek",
			channelType: channel.TypeDeepseek,
			baseURL:     "https://api.deepseek.com",
			expectedURL: "https://api.deepseek.com/v1/models",
		},
		{
			name:        "Vercel",
			channelType: channel.TypeVercel,
			baseURL:     "https://api.vercel.com",
			expectedURL: "https://api.vercel.com/v1/models",
		},
		{
			name:        "Openrouter",
			channelType: channel.TypeOpenrouter,
			baseURL:     "https://openrouter.ai/api",
			expectedURL: "https://openrouter.ai/api/v1/models",
		},
		{
			name:        "Xai",
			channelType: channel.TypeXai,
			baseURL:     "https://api.x.ai",
			expectedURL: "https://api.x.ai/v1/models",
		},
		{
			name:        "Siliconflow",
			channelType: channel.TypeSiliconflow,
			baseURL:     "https://api.siliconflow.cn",
			expectedURL: "https://api.siliconflow.cn/v1/models",
		},
		{
			name:        "URL with trailing slash",
			channelType: channel.TypeOpenai,
			baseURL:     "https://api.openai.com/",
			expectedURL: "https://api.openai.com/v1/models",
		},
		{
			name:        "URL with double trailing slash",
			channelType: channel.TypeOpenai,
			baseURL:     "https://api.openai.com//",
			expectedURL: "https://api.openai.com//v1/models",
		},
		{
			name:        "Gemini Openai",
			channelType: channel.TypeGeminiOpenai,
			baseURL:     "https://generativelanguage.googleapis.com",
			expectedURL: "https://generativelanguage.googleapis.com/v1/models",
		},
		{
			name:        "Gemini Vertex",
			channelType: channel.TypeGeminiVertex,
			baseURL:     "https://us-central1-aiplatform.googleapis.com",
			expectedURL: "https://us-central1-aiplatform.googleapis.com/v1/models",
		},
		{
			name:        "Longcat Anthropic",
			channelType: channel.TypeLongcatAnthropic,
			baseURL:     "https://api.longcat.ai/anthropic",
			expectedURL: "https://api.longcat.ai/v1/models",
		},
		{
			name:        "Minimax Anthropic",
			channelType: channel.TypeMinimaxAnthropic,
			baseURL:     "https://api.minimax.ai/claude",
			expectedURL: "https://api.minimax.ai/v1/models",
		},
		{
			name:        "Doubao Anthropic",
			channelType: channel.TypeDoubaoAnthropic,
			baseURL:     "https://ark.cn-beijing.volces.com/anthropic",
			expectedURL: "https://ark.cn-beijing.volces.com/v1/models",
		},
		{
			name:        "Doubao",
			channelType: channel.TypeDoubao,
			baseURL:     "https://ark.cn-beijing.volces.com",
			expectedURL: "https://ark.cn-beijing.volces.com/v1/models",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, headers := fetcher.prepareModelsEndpoint(tt.channelType, tt.baseURL)

			if url != tt.expectedURL {
				t.Errorf("prepareModelsEndpoint() URL = %q, want %q", url, tt.expectedURL)
			}

			if tt.checkHeader {
				if headers == nil {
					t.Errorf("prepareModelsEndpoint() headers is nil, expected header %q", tt.headerKey)
				} else if headers.Get(tt.headerKey) != tt.headerValue {
					t.Errorf("prepareModelsEndpoint() header %q = %q, want %q", tt.headerKey, headers.Get(tt.headerKey), tt.headerValue)
				}
			}
		})
	}
}

func TestPrepareModelsEndpointHeaders(t *testing.T) {
	fetcher := NewModelFetcher(nil, nil)

	t.Run("Anthropic sets correct version header", func(t *testing.T) {
		_, headers := fetcher.prepareModelsEndpoint(channel.TypeAnthropic, "https://api.anthropic.com")
		if headers.Get("Anthropic-Version") != "2023-06-01" {
			t.Errorf("Expected Anthropic-Version header to be 2023-06-01, got %q", headers.Get("Anthropic-Version"))
		}
	})

	t.Run("OpenAI does not set special headers", func(t *testing.T) {
		_, headers := fetcher.prepareModelsEndpoint(channel.TypeOpenai, "https://api.openai.com")
		if headers.Get("Anthropic-Version") != "" {
			t.Errorf("Expected no Anthropic-Version header for OpenAI, got %q", headers.Get("Anthropic-Version"))
		}
	})

	t.Run("Gemini does not set special headers", func(t *testing.T) {
		_, headers := fetcher.prepareModelsEndpoint(channel.TypeGemini, "https://generativelanguage.googleapis.com")
		if headers.Get("Anthropic-Version") != "" {
			t.Errorf("Expected no Anthropic-Version header for Gemini, got %q", headers.Get("Anthropic-Version"))
		}
	})

	t.Run("Anthropic-like channels do not set version header", func(t *testing.T) {
		_, headers := fetcher.prepareModelsEndpoint(channel.TypeDeepseekAnthropic, "https://api.deepseek.com")
		if headers.Get("Anthropic-Version") != "" {
			t.Errorf("Expected no Anthropic-Version header for Anthropic-like channels, got %q", headers.Get("Anthropic-Version"))
		}
	})
}

func TestFetchModelsGeminiPagination(t *testing.T) {
	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)

		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if r.URL.Path != "/v1beta/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if got := r.URL.Query().Get("pageSize"); got != "1000" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"missing or invalid pageSize"}`))
			return
		}

		switch r.URL.Query().Get("pageToken") {
		case "":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"models":[{"name":"models/m1"},{"name":"models/m2"}],"nextPageToken":"t1"}`))
		case "t1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"models":[{"name":"models/m3"}]}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"unexpected pageToken"}`))
		}
	}))
	defer server.Close()

	fetcher := NewModelFetcher(httpclient.NewHttpClientWithClient(server.Client()), nil)
	apiKey := "test-key"

	result, err := fetcher.FetchModels(context.Background(), FetchModelsInput{
		ChannelType: channel.TypeGemini.String(),
		BaseURL:     server.URL,
		APIKey:      &apiKey,
	})

	if err != nil {
		t.Fatalf("FetchModels() unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("FetchModels() expected nil result.Error, got: %v", *result.Error)
	}

	if got := int(callCount.Load()); got != 2 {
		t.Fatalf("expected 2 requests, got %d", got)
	}

	ids := make(map[string]struct{}, len(result.Models))
	for _, m := range result.Models {
		ids[m.ID] = struct{}{}
	}

	for _, want := range []string{"m1", "m2", "m3"} {
		if _, ok := ids[want]; !ok {
			t.Fatalf("missing model id %q in result: %#v", want, result.Models)
		}
	}
}
