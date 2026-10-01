package orchestrator

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/pipeline/stream"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestChatCompletionOrchestrator_PreservesFields_whenChannelRequiresStream(t *testing.T) {
	// Given: a real HTTP SSE upstream and a channel with the supported require policy.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !assert.Equal(t, "/v1/chat/completions", r.URL.Path) ||
			!assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization")) {
			http.Error(w, "unexpected upstream request", http.StatusBadRequest)
			return
		}
		var body struct {
			Stream bool `json:"stream"`
		}
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) || !assert.True(t, body.Stream) {
			http.Error(w, "expected streaming JSON request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, chunk := range []string{
			`{"id":"fields","object":"chat.completion.chunk","created":1720000000,"model":"gpt-4","service_tier":"priority","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi","refusal":"Cannot ","reasoning":"think ","reasoning_content":"plan ","audio":{"id":"audio_1","expires_at":1730000000,"data":"QUJD","transcript":"Hello"}},"logprobs":{"content":[{"token":"Hi","logprob":-0.1,"bytes":[72,105],"top_logprobs":[{"token":"Hey","logprob":-1.5,"bytes":[72,101,121]}]}]}}]}`,
			`{"id":"fields","object":"chat.completion.chunk","created":1720000000,"model":"gpt-4","choices":[{"index":0,"delta":{"content":" there","refusal":"help.","reasoning":"more","reasoning_content":"next","audio":{"data":"REVG","transcript":" world"}},"logprobs":{"content":[{"token":" there","logprob":-0.2,"bytes":[32,116,104,101,114,101]}]},"finish_reason":"stop"}]}`,
			`{"id":"fields","object":"chat.completion.chunk","created":1720000000,"model":"gpt-4","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":8,"total_tokens":18}}`,
			`[DONE]`,
		} {
			_, err := fmt.Fprintf(w, "data: %s\n\n", chunk)
			if !assert.NoError(t, err) {
				return
			}
		}
	}))
	defer upstream.Close()

	client := enttest.NewEntClient(t, "sqlite3", "file:"+t.Name()+"?mode=memory&_fk=0")
	defer client.Close()
	ctx := ent.NewContext(authz.WithTestBypass(t.Context()), client)
	project := createTestProject(t, ctx, client)
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Require stream fields").
		SetBaseURL(upstream.URL + "/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-api-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetPolicies(objects.ChannelPolicies{Stream: objects.CapabilityPolicyRequire}).
		Save(ctx)
	require.NoError(t, err)
	outbound, err := openai.NewOutboundTransformer(ch.BaseURL, ch.Credentials.APIKey)
	require.NoError(t, err)
	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)
	o := &ChatCompletionOrchestrator{
		channelSelector: &staticChannelSelector{candidates: channelsToTestCandidates([]*biz.Channel{{Channel: ch, Outbound: outbound}}, "gpt-4")},
		Inbound:         openai.NewInboundTransformer(), RequestService: requestService,
		ChannelService: channelService, PromptProvider: &stubPromptProvider{},
		SystemService: systemService, UsageLogService: usageLogService,
		PipelineFactory: pipeline.NewFactory(httpclient.NewHttpClientWithClient(upstream.Client())),
		ModelMapper:     NewModelMapper(), channelLimiterManager: NewChannelLimiterManager(),
		Middlewares: []pipeline.Middleware{stream.EnsureUsage()},
	}
	ctx = contexts.WithProjectID(ctx, project.ID)
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if !assert.NoError(t, err) {
			http.Error(w, "failed to read request", http.StatusBadRequest)
			return
		}
		result, err := o.Process(ctx, &httpclient.Request{Method: r.Method, URL: r.URL.Path, Headers: r.Header, Body: body})
		if !assert.NoError(t, err) {
			http.Error(w, "orchestration failed", http.StatusInternalServerError)
			return
		}
		if !assert.Nil(t, result.ChatCompletionStream) || !assert.NotNil(t, result.ChatCompletion) {
			http.Error(w, "expected non-streaming completion", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(result.ChatCompletion.StatusCode)
		_, err = w.Write(result.ChatCompletion.Body)
		assert.NoError(t, err)
	}))
	defer gateway.Close()

	// When: a client explicitly requests a non-streaming response over HTTP.
	response, err := gateway.Client().Post(gateway.URL+"/v1/chat/completions", "application/json",
		strings.NewReader(`{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}],"stream":false,"logprobs":true}`))
	require.NoError(t, err)
	defer response.Body.Close()

	// Then: the assembled JSON preserves all supported fields through the real pipeline.
	require.Equal(t, http.StatusOK, response.StatusCode)
	var got openai.Response
	require.NoError(t, json.NewDecoder(response.Body).Decode(&got))
	require.Equal(t, "chat.completion", got.Object)
	require.Equal(t, "priority", got.ServiceTier)
	require.Len(t, got.Choices, 1)
	message := got.Choices[0].Message
	require.NotNil(t, message)
	require.Equal(t, "Cannot help.", message.Refusal)
	require.NotNil(t, message.Content.Content)
	require.Equal(t, "Hi there", *message.Content.Content)
	require.NotNil(t, message.Reasoning)
	require.Equal(t, "think more", *message.Reasoning)
	require.NotNil(t, message.ReasoningContent)
	require.Equal(t, "plan next", *message.ReasoningContent)
	require.Equal(t, &openai.OutputAudio{ID: "audio_1", ExpiresAt: 1730000000, Data: "QUJDREVG", Transcript: "Hello world"}, message.Audio)
	require.NotNil(t, got.Choices[0].Logprobs)
	require.Equal(t, []openai.TokenLogprob{
		{Token: "Hi", Logprob: -0.1, Bytes: []int{72, 105}, TopLogprobs: []openai.TopLogprob{{Token: "Hey", Logprob: -1.5, Bytes: []int{72, 101, 121}}}},
		{Token: " there", Logprob: -0.2, Bytes: []int{32, 116, 104, 101, 114, 101}},
	}, got.Choices[0].Logprobs.Content)
	require.NotNil(t, got.Usage)
	require.Equal(t, int64(18), got.Usage.TotalTokens)
	t.Logf("HTTP require-stream driver: refusal=%q audio=%+v reasoning=%q reasoning_content=%q service_tier=%q logprobs=%+v usage=%+v",
		message.Refusal, message.Audio, *message.Reasoning, *message.ReasoningContent, got.ServiceTier, got.Choices[0].Logprobs, got.Usage)
}
