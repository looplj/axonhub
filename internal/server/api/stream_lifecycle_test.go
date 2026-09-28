package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestRawOpenAIChatStreamLifecyclePersistsAndWritesProtocolOutcomes(t *testing.T) {
	tests := []struct {
		name             string
		rawEvents        []string
		cancelBeforeEOF  bool
		cancelAfter      string
		sourceErr        error
		requestBody      string
		wantErrorFrame   bool
		wantRequest      request.Status
		wantExecution    requestexecution.Status
		wantFinishReason []string
		wantDone         bool
	}{
		{
			name:        "first choice finishes before second choice and transport fails",
			requestBody: `{"stream":true,"n":2}`,
			rawEvents: []string{
				`{"id":"chatcmpl-split","object":"chat.completion.chunk","model":"gpt-test","choices":[{"index":0,"delta":{"content":"first"},"finish_reason":"stop"}]}`,
				`{"id":"chatcmpl-split","object":"chat.completion.chunk","model":"gpt-test","choices":[{"index":1,"delta":{"content":"second"},"finish_reason":null}]}`,
			},
			sourceErr:   io.ErrUnexpectedEOF,
			wantRequest: request.StatusFailed, wantExecution: requestexecution.StatusFailed,
			wantFinishReason: []string{"stop"}, wantErrorFrame: true,
		},
		{
			name:        "two choices finish in separate chunks",
			requestBody: `{"stream":true,"n":2}`,
			rawEvents: []string{
				`{"id":"chatcmpl-split-ok","object":"chat.completion.chunk","model":"gpt-test","choices":[{"index":0,"delta":{"content":"first"},"finish_reason":"stop"}]}`,
				`{"id":"chatcmpl-split-ok","object":"chat.completion.chunk","model":"gpt-test","choices":[{"index":1,"delta":{"content":"second"},"finish_reason":"stop"}]}`,
			},
			wantRequest: request.StatusCompleted, wantExecution: requestexecution.StatusCompleted,
			wantFinishReason: []string{"stop"},
		},
		{
			name: "text clean EOF",
			rawEvents: []string{
				`{"id":"chatcmpl-text","object":"chat.completion.chunk","model":"gpt-test","choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-text","object":"chat.completion.chunk","model":"gpt-test","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`,
			},
			wantRequest:      request.StatusCompleted,
			wantExecution:    requestexecution.StatusCompleted,
			wantFinishReason: []string{"stop"},
			wantDone:         true,
		},
		{
			name: "fragmented valid tool clean EOF",
			rawEvents: []string{
				`{"id":"chatcmpl-tool","object":"chat.completion.chunk","model":"gpt-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"type":"function","function":{"name":"lookup","arguments":"{\"q\":"}}]},"finish_reason":null}]}`,
				`{"id":"chatcmpl-tool","object":"chat.completion.chunk","model":"gpt-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"ping\"}"}}]},"finish_reason":null}]}`,
				`{"id":"chatcmpl-tool","object":"chat.completion.chunk","model":"gpt-test","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`,
			},
			wantRequest:      request.StatusCompleted,
			wantExecution:    requestexecution.StatusCompleted,
			wantFinishReason: []string{"tool_calls"},
			wantDone:         true,
		},
		{
			name: "partially finished multi choice",
			rawEvents: []string{
				`{"id":"chatcmpl-multi","object":"chat.completion.chunk","model":"gpt-test","choices":[{"index":0,"delta":{"content":"first"},"finish_reason":"stop"},{"index":1,"delta":{"content":"second"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-multi","object":"chat.completion.chunk","model":"gpt-test","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`,
			},
			wantRequest:      request.StatusCompleted,
			wantExecution:    requestexecution.StatusCompleted,
			wantFinishReason: []string{"stop", "stop"},
			wantDone:         true,
		},
		{
			name: "canceled before clean EOF",
			rawEvents: []string{
				`{"id":"chatcmpl-canceled","object":"chat.completion.chunk","model":"gpt-test","choices":[{"index":0,"delta":{"content":"partial"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-canceled","object":"chat.completion.chunk","model":"gpt-test","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`,
			},
			cancelBeforeEOF:  true,
			wantRequest:      request.StatusCanceled,
			wantExecution:    requestexecution.StatusCanceled,
			wantFinishReason: nil,
			wantDone:         false,
		},
		{
			name: "two choices trailing provider error",
			rawEvents: []string{
				`{"id":"chatcmpl-error","object":"chat.completion.chunk","model":"gpt-test","choices":[{"index":0,"delta":{"content":"first"},"finish_reason":"stop"},{"index":1,"delta":{"content":"second"},"finish_reason":null}]}`,
				`{"error":{"message":"provider disconnected"}}`,
			},
			wantRequest:      request.StatusFailed,
			wantExecution:    requestexecution.StatusFailed,
			wantFinishReason: []string{"stop"},
			wantDone:         false,
		},
		{
			name: "cancel after synthetic finish",
			rawEvents: []string{
				`{"id":"chatcmpl-after-finish","object":"chat.completion.chunk","model":"gpt-test","choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-after-finish","object":"chat.completion.chunk","model":"gpt-test","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`,
			},
			cancelAfter:      "finish",
			wantRequest:      request.StatusCompleted,
			wantExecution:    requestexecution.StatusCompleted,
			wantFinishReason: []string{"stop"},
			wantDone:         true,
		},
		{
			name: "cancel after done",
			rawEvents: []string{
				`{"id":"chatcmpl-after-done","object":"chat.completion.chunk","model":"gpt-test","choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-after-done","object":"chat.completion.chunk","model":"gpt-test","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`,
			},
			cancelAfter:      "done",
			wantRequest:      request.StatusCompleted,
			wantExecution:    requestexecution.StatusCompleted,
			wantFinishReason: []string{"stop"},
			wantDone:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
			defer client.Close()
			baseCtx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
			project, err := client.Project.Create().SetName("stream-test").SetDescription("stream-test").Save(baseCtx)
			require.NoError(t, err)
			systemService := biz.NewSystemService(biz.SystemServiceParams{CacheConfig: xcache.Config{Mode: xcache.ModeMemory}, Ent: client})
			channelService := biz.NewChannelServiceForTest(client)
			usageLogService := biz.NewUsageLogService(client, systemService, channelService)
			dataStorageService := &biz.DataStorageService{
				AbstractService: &biz.AbstractService{}, SystemService: systemService,
				Cache: xcache.NewFromConfig[ent.DataStorage](xcache.Config{Mode: xcache.ModeMemory}),
			}
			requestService := biz.NewRequestService(client, systemService.CacheConfig, systemService, usageLogService, dataStorageService, biz.NewLiveStreamRegistry())
			require.NoError(t, systemService.SetStoragePolicy(baseCtx, &biz.StoragePolicy{StoreChunks: true, StoreRequestBody: true, StoreResponseBody: true}))

			requestBody := tt.requestBody
			if requestBody == "" {
				requestBody = `{"stream":true}`
			}
			req, err := client.Request.Create().SetProjectID(project.ID).SetModelID("gpt-test").SetFormat(llm.APIFormatOpenAIChatCompletion.String()).
				SetStatus(request.StatusProcessing).SetStream(true).SetRequestBody(objects.JSONRawMessage(requestBody)).Save(baseCtx)
			require.NoError(t, err)
			execution, err := client.RequestExecution.Create().SetRequestID(req.ID).SetProjectID(project.ID).SetModelID("gpt-test").
				SetFormat(llm.APIFormatOpenAIChatCompletion.String()).SetStatus(requestexecution.StatusProcessing).SetStream(true).
				SetRequestBody(objects.JSONRawMessage(`{"stream":true}`)).Save(baseCtx)
			require.NoError(t, err)

			streamCtx := baseCtx
			var cancel context.CancelFunc
			if tt.cancelBeforeEOF || tt.cancelAfter != "" {
				streamCtx, cancel = context.WithCancel(baseCtx)
				if tt.cancelBeforeEOF {
					cancel()
				}
			}
			outbound, err := openai.NewOutboundTransformer("https://provider.test", "provider-key")
			require.NoError(t, err)
			state := &orchestrator.PersistenceState{}
			raw := make([]*httpclient.StreamEvent, 0, len(tt.rawEvents))
			for _, event := range tt.rawEvents {
				raw = append(raw, &httpclient.StreamEvent{Data: []byte(event)})
			}
			var source streams.Stream[*httpclient.StreamEvent] = streams.SliceStream(raw)
			if tt.sourceErr != nil {
				source = &errorAfterStream{items: raw, err: tt.sourceErr}
			}
			provider := orchestrator.NewOutboundPersistentStream(streamCtx, source, req, execution, requestService, usageLogService, outbound, nil, state)
			normalized, err := outbound.TransformStream(streamCtx, nil, provider)
			require.NoError(t, err)
			converted, err := openai.NewInboundTransformer().TransformStream(streamCtx, normalized)
			require.NoError(t, err)
			clientStream := orchestrator.NewInboundPersistentStream(streamCtx, converted, req, execution, requestService, openai.NewInboundTransformer(), nil, state)
			var writerStream streams.Stream[*httpclient.StreamEvent] = clientStream
			if tt.cancelAfter != "" {
				writerStream = &cancelOnEventStream{source: clientStream, cancel: cancel, trigger: tt.cancelAfter}
			}

			writer := httptest.NewRecorder()
			ginContext, _ := gin.CreateTestContext(writer)
			ginContext.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			WriteSSEStream(ginContext, writerStream)
			require.NoError(t, clientStream.Close())

			savedRequest, err := client.Request.Get(baseCtx, req.ID)
			require.NoError(t, err)
			savedExecution, err := client.RequestExecution.Get(baseCtx, execution.ID)
			require.NoError(t, err)
			require.Equal(t, tt.wantRequest, savedRequest.Status)
			require.Equal(t, tt.wantExecution, savedExecution.Status)
			body := writer.Body.String()
			require.Equal(t, tt.wantDone, strings.Contains(body, "data: [DONE]"))
			if tt.wantErrorFrame {
				require.Contains(t, body, "event:error", body)
			}
			for _, reason := range tt.wantFinishReason {
				require.Contains(t, body, `"finish_reason":"`+reason+`"`)
			}
			if tt.wantRequest == request.StatusCompleted {
				require.NotContains(t, body, "event:error")
			}
			if tt.name == "two choices finish in separate chunks" || tt.name == "partially finished multi choice" {
				require.Equal(t, 2, strings.Count(body, `"finish_reason":"stop"`), body)
			}
		})
	}
}

type cancelOnEventStream struct {
	source  streams.Stream[*httpclient.StreamEvent]
	cancel  context.CancelFunc
	trigger string
}

func (s *cancelOnEventStream) Next() bool { return s.source.Next() }

func (s *cancelOnEventStream) ExpectedStreamChoices() int {
	return streamExpectedChoices(s.source)
}

func (s *cancelOnEventStream) Current() *httpclient.StreamEvent {
	event := s.source.Current()
	if event != nil && ((s.trigger == "finish" && strings.Contains(string(event.Data), `"finish_reason":"stop"`)) ||
		(s.trigger == "done" && strings.EqualFold(string(event.Data), "[DONE]"))) {
		s.cancel()
	}
	return event
}

func (s *cancelOnEventStream) Err() error   { return s.source.Err() }
func (s *cancelOnEventStream) Close() error { return s.source.Close() }
