package orchestrator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestChannelTestRequestFormats(t *testing.T) {
	tests := []struct {
		name         string
		channelType  channel.Type
		path         string
		response     string
		status       int
		stream       bool
		wantSuccess  bool
		forcedFormat string
	}{
		{name: "systemone", channelType: channel.TypeTypesafe, path: "/v1/systemone", response: `{"model":"test-model","answers":{"connection":{"type":"noul","noul":0.95}},"usage":{"input_tokens":10,"output_tokens":0}}`, wantSuccess: true},
		{name: "openai", channelType: channel.TypeOpenai, path: "/v1/chat/completions", response: `{"id":"test","model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`, wantSuccess: true},
		{name: "mixed endpoints forced chat", channelType: channel.TypeOpenai, forcedFormat: llm.APIFormatOpenAIChatCompletion.String(), path: "/v1/chat/completions", response: `{"id":"test","model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`, wantSuccess: true},
		{name: "openai required stream", channelType: channel.TypeOpenai, path: "/v1/chat/completions", stream: true, response: "data: {\"id\":\"test\",\"model\":\"test-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"}}]}\n\ndata: {\"id\":\"test\",\"model\":\"test-model\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}\n\ndata: [DONE]\n\n", wantSuccess: true},
		{name: "anthropic", channelType: channel.TypeAnthropic, path: "/v1/messages", response: `{"id":"test","type":"message","role":"assistant","model":"test-model","content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`, wantSuccess: true},
		{name: "gemini", channelType: channel.TypeGemini, path: "/v1beta/models/test-model:generateContent", response: `{"candidates":[{"content":{"role":"model","parts":[{"text":"hello"}]},"finishReason":"STOP","index":0}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1,"totalTokenCount":2},"modelVersion":"test-model"}`, wantSuccess: true},
		{name: "systemone empty answers", channelType: channel.TypeTypesafe, path: "/v1/systemone", response: `{"model":"test-model","answers":{}}`},
		{name: "systemone unrelated answer", channelType: channel.TypeTypesafe, path: "/v1/systemone", response: `{"model":"test-model","answers":{"other":{"type":"noul","noul":0.95}}}`},
		{name: "systemone invalid JSON", channelType: channel.TypeTypesafe, path: "/v1/systemone", response: `not-json`},
		{name: "systemone upstream error", channelType: channel.TypeTypesafe, path: "/v1/systemone", status: http.StatusUnauthorized, response: `{"error":{"message":"upstream rejected test request","type":"invalid_api_key"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, client := setupTest(t)
			ctx = contexts.WithProjectID(ctx, createTestProject(t, ctx, client).ID)
			var mu sync.Mutex
			var keys []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
				if key == "" {
					key = r.Header.Get("X-Api-Key")
				}
				if key == "" {
					key = r.Header.Get("X-Goog-Api-Key")
				}
				mu.Lock()
				keys = append(keys, key)
				mu.Unlock()
				if r.Method != http.MethodPost || r.URL.Path != tt.path || (key != "test-key-1" && key != "test-key-2") {
					http.Error(w, "incorrect test endpoint or key", http.StatusBadRequest)
					return
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				var valid bool
				switch tt.channelType {
				case channel.TypeTypesafe:
					questions, _ := body["questions"].(map[string]any)
					question, _ := questions["connection"].(map[string]any)
					valid = body["model"] == "test-model" && body["state"] == "system prompt\n\nuser prompt" && question["type"] == "noul" && question["instructions"] != nil && body["messages"] == nil && body["stream"] == nil && body["max_tokens"] == nil
				case channel.TypeGemini:
					valid = body["contents"] != nil && body["systemInstruction"] != nil && body["messages"] == nil
				default:
					valid = body["model"] == "test-model" && body["messages"] != nil && body["stream"] == tt.stream
					if tt.channelType == channel.TypeAnthropic {
						valid = valid && body["system"] != nil && body["max_tokens"] == float64(256)
					}
				}
				if !valid {
					http.Error(w, fmt.Sprintf("incorrect test payload: %v", body), http.StatusBadRequest)
					return
				}
				contentType := "application/json"
				if tt.stream {
					contentType = "text/event-stream"
				}
				w.Header().Set("Content-Type", contentType)
				if tt.status != 0 {
					w.WriteHeader(tt.status)
				}
				_, _ = w.Write([]byte(tt.response))
			}))
			t.Cleanup(server.Close)
			baseURL := server.URL + "/v1"
			if tt.channelType == channel.TypeGemini {
				baseURL = server.URL
			}
			policies := objects.ChannelPolicies{}
			if tt.stream {
				policies.Stream = objects.CapabilityPolicyRequire
			}
			ch, err := client.Channel.Create().SetType(tt.channelType).SetName(tt.name).
				SetBaseURL(baseURL).SetCredentials(objects.ChannelCredentials{APIKeys: []string{"test-key-1", "test-key-2"}}).
				SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").SetPolicies(policies).Save(ctx)
			require.NoError(t, err)
			if tt.forcedFormat != "" {
				ch, err = client.Channel.UpdateOne(ch).SetEndpoints([]objects.ChannelEndpoint{
					{APIFormat: llm.APIFormatTypeSafeSystemOne.String(), Path: "/systemone"},
				}).SetSettings(&objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{
					{Model: "test-model", APIFormats: []string{tt.forcedFormat}},
				}}).Save(ctx)
				require.NoError(t, err)
			}
			channelService, requestService, systemService, usageLogService := setupTestServices(t, client)
			require.NoError(t, systemService.SetChannelSetting(ctx, biz.SystemChannelSettings{TestSystemPrompt: "system prompt", TestUserPrompt: "user prompt"}))
			require.NoError(t, systemService.SetRetryPolicy(ctx, &biz.RetryPolicy{}))
			protection := biz.NewPromptProtectionRuleService(biz.PromptProtectionRuleServiceParams{Ent: client})
			t.Cleanup(protection.Stop)
			processor := NewTestChannelOrchestrator(channelService, requestService, systemService, usageLogService, protection, httpclient.NewHttpClient())
			id := objects.GUID{Type: "Channel", ID: ch.ID}

			result, err := processor.TestChannel(ctx, id, nil, nil)
			require.NoError(t, err)
			require.Equal(t, tt.wantSuccess, result.Success, "%s", lo.FromPtr(result.Error))
			if tt.wantSuccess {
				if tt.channelType == channel.TypeTypesafe {
					require.Contains(t, lo.FromPtr(result.Message), `"noul":0.95`)
				} else {
					require.Equal(t, "hello", lo.FromPtr(result.Message))
				}
			} else {
				require.NotEmpty(t, lo.FromPtr(result.Error))
			}

			keyResult, err := processor.TestSingleAPIKey(ctx, id, "test-key-2", nil, nil)
			require.NoError(t, err)
			require.Equal(t, tt.wantSuccess, keyResult.Success, "%s", lo.FromPtr(keyResult.Error))
			mu.Lock()
			lastKey := ""
			if len(keys) > 0 {
				lastKey = keys[len(keys)-1]
			}
			mu.Unlock()
			require.Equal(t, "test-key-2", lastKey)

			batch, err := processor.TestChannelAPIKeys(ctx, id, nil, nil)
			require.NoError(t, err)
			require.Equal(t, 2, batch.Total)
			wantCount := 0
			if tt.wantSuccess {
				wantCount = 2
			}
			require.Equal(t, wantCount, batch.SuccessCount)
			require.Equal(t, 2-wantCount, batch.FailedCount)
			mu.Lock()
			batchKeys := append([]string(nil), keys...)
			mu.Unlock()
			require.Len(t, batchKeys, 4)
			require.ElementsMatch(t, []string{"test-key-1", "test-key-2"}, batchKeys[2:])
		})
	}
}

func TestChannelTestRejectsSystemOneStreamPolicy(t *testing.T) {
	ctx, client := setupTest(t)
	ctx = contexts.WithProjectID(ctx, createTestProject(t, ctx, client).ID)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "unexpected upstream request", http.StatusBadRequest)
	}))
	t.Cleanup(server.Close)
	ch, err := client.Channel.Create().SetType(channel.TypeTypesafe).SetName("System One stream policy").
		SetBaseURL(server.URL + "/v1").SetCredentials(objects.ChannelCredentials{APIKeys: []string{"test-key-1", "test-key-2"}}).
		SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").
		SetPolicies(objects.ChannelPolicies{Stream: objects.CapabilityPolicyRequire}).Save(ctx)
	require.NoError(t, err)
	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)
	protection := biz.NewPromptProtectionRuleService(biz.PromptProtectionRuleServiceParams{Ent: client})
	t.Cleanup(protection.Stop)
	processor := NewTestChannelOrchestrator(channelService, requestService, systemService, usageLogService, protection, httpclient.NewHttpClient())
	id := objects.GUID{Type: "Channel", ID: ch.ID}

	_, err = processor.TestChannel(ctx, id, nil, nil)
	require.ErrorContains(t, err, "systemone does not support streaming")
	keyResult, err := processor.TestSingleAPIKey(ctx, id, "test-key-1", nil, nil)
	require.NoError(t, err)
	require.False(t, keyResult.Success)
	require.Contains(t, lo.FromPtr(keyResult.Error), "systemone does not support streaming")
	batch, err := processor.TestChannelAPIKeys(ctx, id, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 0, batch.SuccessCount)
	require.Equal(t, 2, batch.FailedCount)
	for _, result := range batch.Results {
		require.Contains(t, lo.FromPtr(result.Error), "systemone does not support streaming")
	}
	require.Zero(t, calls.Load())
}
