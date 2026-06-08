package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	entrequest "github.com/looplj/axonhub/internal/ent/request"
	entrequestexecution "github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

type speechAPISelector struct {
	candidates []*orchestrator.ChannelModelsCandidate
}

func (s *speechAPISelector) Select(context.Context, *llm.Request) ([]*orchestrator.ChannelModelsCandidate, error) {
	return s.candidates, nil
}

type speechAPIExecutor struct {
	streamEvents   []*httpclient.StreamEvent
	doStreamCalled bool
	lastRequest    *httpclient.Request
}

func (e *speechAPIExecutor) Do(context.Context, *httpclient.Request) (*httpclient.Response, error) {
	return nil, errors.New("unexpected non-stream speech request")
}

func (e *speechAPIExecutor) DoStream(_ context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	e.doStreamCalled = true
	e.lastRequest = request

	return streams.SliceStream(e.streamEvents), nil
}

func setupSpeechAPITestServices(t *testing.T, client *ent.Client) (*biz.ChannelService, *biz.RequestService, *biz.SystemService, *biz.UsageLogService) {
	t.Helper()

	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	systemService := biz.NewSystemService(biz.SystemServiceParams{
		CacheConfig: cacheConfig,
		Ent:         client,
	})
	dataStorageService := &biz.DataStorageService{
		AbstractService: &biz.AbstractService{},
		SystemService:   systemService,
		Cache:           xcache.NewFromConfig[ent.DataStorage](cacheConfig),
	}
	channelService := biz.NewChannelServiceForTest(client)
	usageLogService := biz.NewUsageLogService(client, systemService, channelService)
	requestService := biz.NewRequestService(client, systemService, usageLogService, dataStorageService, biz.NewLiveStreamRegistry())

	return channelService, requestService, systemService, usageLogService
}

func TestShouldUseBinarySpeechStream(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		want        bool
		wantErr     bool
	}{
		{
			name:        "audio stream format",
			contentType: "application/json",
			body:        `{"stream_format":"audio"}`,
			want:        true,
		},
		{
			name:        "sse stream format",
			contentType: "application/json",
			body:        `{"stream_format":"sse"}`,
		},
		{
			name:        "default non-stream",
			contentType: "application/json",
			body:        `{}`,
		},
		{
			name:        "non-json content type lets transformer report validation",
			contentType: "multipart/form-data",
			body:        `{"stream_format":"audio"}`,
		},
		{
			name:        "invalid json",
			contentType: "application/json",
			body:        `{`,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &httpclient.Request{
				Headers: http.Header{"Content-Type": []string{tt.contentType}},
				Body:    []byte(tt.body),
			}

			got, err := shouldUseBinarySpeechStream(req)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestOpenAIHandlers_CreateSpeech_AudioStreamEndToEnd(t *testing.T) {
	ctx := ent.NewContext(authz.WithTestBypass(context.Background()), enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0"))
	client := ent.FromContext(ctx)
	t.Cleanup(func() {
		_ = client.Close()
	})

	modelID := "gpt-4o-mini-tts"
	channelRow, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Speech Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-api-key"}).
		SetSupportedModels([]string{modelID}).
		SetDefaultTestModel(modelID).
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channelService, requestService, systemService, usageLogService := setupSpeechAPITestServices(t, client)
	require.NoError(t, systemService.SetStoragePolicy(ctx, &biz.StoragePolicy{
		StoreChunks:       true,
		LivePreview:       false,
		StoreRequestBody:  true,
		StoreResponseBody: true,
	}))

	outbound, err := openai.NewOutboundTransformer(channelRow.BaseURL, channelRow.Credentials.APIKey)
	require.NoError(t, err)

	bizChannel := &biz.Channel{Channel: channelRow, Outbound: outbound}
	selector := &speechAPISelector{
		candidates: []*orchestrator.ChannelModelsCandidate{{
			Channel: bizChannel,
			Models: []biz.ChannelModelEntry{{
				RequestModel: modelID,
				ActualModel:  modelID,
				Source:       "direct",
			}},
		}},
	}
	executor := &speechAPIExecutor{
		streamEvents: []*httpclient.StreamEvent{
			{Type: "audio/mpeg", Data: []byte{0x01, 0x02}},
			{Type: "audio/mpeg", Data: []byte{0x03}},
			{Type: httpclient.BinaryStreamDoneEventType},
		},
	}

	speechInbound := openai.NewSpeechInboundTransformer()
	promptProtectionService := biz.NewPromptProtectionRuleService(biz.PromptProtectionRuleServiceParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		Ent:         client,
	})
	t.Cleanup(promptProtectionService.Stop)

	speechOrchestrator := orchestrator.NewChatCompletionOrchestrator(
		channelService,
		nil,
		requestService,
		httpclient.NewHttpClient(),
		speechInbound,
		systemService,
		usageLogService,
		nil,
		nil,
		promptProtectionService,
		biz.NewLiveStreamRegistry(),
		orchestrator.NewChannelLimiterManager(),
		nil,
	).WithChannelSelector(selector)
	speechOrchestrator.PipelineFactory = pipeline.NewFactory(executor)

	handlers := &OpenAIHandlers{
		SpeechHandlers:           NewChatCompletionHandlers(speechOrchestrator),
		SpeechInboundTransformer: speechInbound,
	}

	body := `{"model":"gpt-4o-mini-tts","input":"Hi","voice":"alloy","stream_format":"audio"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", strings.NewReader(body))
	c.Request = c.Request.WithContext(ctx)
	c.Request.Header.Set("Content-Type", "application/json")

	handlers.CreateSpeech(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "audio/mpeg", w.Header().Get("Content-Type"))
	require.Equal(t, []byte{0x01, 0x02, 0x03}, w.Body.Bytes())
	require.True(t, executor.doStreamCalled)
	require.NotNil(t, executor.lastRequest)
	require.Equal(t, string(llm.APIFormatOpenAISpeech), executor.lastRequest.APIFormat)

	var providerBody map[string]any
	require.NoError(t, json.Unmarshal(executor.lastRequest.Body, &providerBody))
	require.Equal(t, "audio", providerBody["stream_format"])
	require.NotContains(t, providerBody, "stream")

	requests, err := client.Request.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, requests, 1)
	require.Equal(t, entrequest.StatusCompleted, requests[0].Status)
	require.True(t, requests[0].Stream)
	require.Contains(t, string(requests[0].ResponseBody), `"object":"audio.speech.stream"`)
	require.Contains(t, string(requests[0].ResponseBody), `"audio_bytes":3`)

	executions, err := client.RequestExecution.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, executions, 1)
	require.Equal(t, entrequestexecution.StatusCompleted, executions[0].Status)
	require.True(t, executions[0].Stream)
	require.Contains(t, string(executions[0].ResponseBody), `"object":"audio.speech.stream"`)
	require.Contains(t, string(executions[0].ResponseBody), `"audio_bytes":3`)
}
