package orchestrator

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
)

func newUpstreamModelPersistenceTest(t *testing.T) (context.Context, *ent.Client, *PersistenceState) {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx := ent.NewContext(authz.WithTestBypass(context.Background()), client)
	project := createTestProject(t, ctx, client)
	channel := createTestChannel(t, ctx, client)
	_, requestService, systemService, usageLogService := setupTestServices(t, client)
	require.NoError(t, systemService.SetStoragePolicy(ctx, &biz.StoragePolicy{
		StoreRequestBody:  false,
		StoreResponseBody: false,
		StoreChunks:       false,
	}))

	req, err := client.Request.Create().
		SetProjectID(project.ID).
		SetChannelID(channel.ID).
		SetModelID("client-alias").
		SetStatus(request.StatusPending).
		SetRequestBody([]byte(`{}`)).
		Save(ctx)
	require.NoError(t, err)

	return ctx, client, &PersistenceState{
		Request:         req,
		RequestService:  requestService,
		SystemService:   systemService,
		UsageLogService: usageLogService,
		ModelMapper:     NewModelMapper(),
	}
}

func createUpstreamModelTestExecution(t *testing.T, ctx context.Context, client *ent.Client, req *ent.Request, model string, format llm.APIFormat, stream bool) *ent.RequestExecution {
	t.Helper()
	execution, err := client.RequestExecution.Create().
		SetRequestID(req.ID).
		SetProjectID(req.ProjectID).
		SetChannelID(req.ChannelID).
		SetModelID(model).
		SetRequestBody([]byte(`{}`)).
		SetFormat(string(format)).
		SetStatus(requestexecution.StatusPending).
		SetStream(stream).
		Save(ctx)
	require.NoError(t, err)
	return execution
}

func TestUpstreamModelPersistence_NonStreamingAttempts(t *testing.T) {
	ctx, client, state := newUpstreamModelPersistenceTest(t)
	middleware := &persistRequestExecutionMiddleware{
		outbound: &PersistentOutboundTransformer{state: state},
	}
	mapper := &apiKeyModelMappingMiddleware{
		RequestModel: "client-alias",
		inbound:      &PersistentInboundTransformer{state: state},
	}

	attempts := []struct {
		name          string
		body          string
		contentType   string
		responseModel string
		format        llm.APIFormat
		wantModel     string
		err           error
	}{
		{name: "empty response triggers retry", format: llm.APIFormatOpenAIChatCompletion, body: `{"model":"model-a","choices":[]}`, responseModel: "model-a", wantModel: "model-a", err: pipeline.ErrEmptyResponse},
		{name: "retry reports its own model", format: llm.APIFormatOpenAIChatCompletion, body: `{"model":"model-b","choices":[]}`, responseModel: "model-b", wantModel: "model-b"},
		{name: "synthetic image model remains unknown", format: llm.APIFormatOpenAIImageGeneration, body: `{"created":123,"data":[]}`, responseModel: "requested-image-model"},
		{name: "image modelVersion takes precedence over synthesized model", format: llm.APIFormatGeminiContents, body: `{"modelVersion":"reported-image-version","candidates":[]}`, responseModel: "requested-image-model", wantModel: "reported-image-version"},
		// Some relays answer a non-streaming request with a JSON body labelled as an
		// event stream. The name is conclusively present, so it must still be recorded.
		{name: "relay labels a JSON body as an event stream", format: llm.APIFormatOpenAIChatCompletion, contentType: "text/event-stream", body: `{"model":"model-a","choices":[]}`, responseModel: "model-a", wantModel: "model-a"},
		{name: "relay sends a malformed media type", format: llm.APIFormatOpenAIChatCompletion, contentType: "application/json; x=", body: `{"model":"model-a","choices":[]}`, responseModel: "model-a", wantModel: "model-a"},
	}

	for _, attempt := range attempts {
		t.Run(attempt.name, func(t *testing.T) {
			state.RequestExec = createUpstreamModelTestExecution(t, ctx, client, state.Request, attempt.responseModel, attempt.format, false)
			_, err := middleware.OnOutboundRawRequest(ctx, &httpclient.Request{})
			require.NoError(t, err)
			_, err = middleware.OnOutboundRawResponse(ctx, &httpclient.Response{
				StatusCode: http.StatusOK,
				Headers:    http.Header{"Content-Type": {attempt.contentType}},
				Body:       []byte(attempt.body),
			})
			require.NoError(t, err)
			response := &llm.Response{ID: "response-id", Model: attempt.responseModel}
			// Match the production middleware order: persistence, then model rewrite,
			// followed by the optional empty-response error and retry.
			_, err = middleware.OnOutboundLlmResponse(ctx, response)
			require.NoError(t, err)
			_, err = mapper.OnOutboundLlmResponse(ctx, response)
			require.NoError(t, err)
			require.Equal(t, "client-alias", response.Model)
			wantStatus := requestexecution.StatusCompleted
			if attempt.err != nil {
				middleware.OnOutboundRawError(ctx, attempt.err)
				wantStatus = requestexecution.StatusFailed
			}

			saved, err := client.RequestExecution.Get(ctx, state.RequestExec.ID)
			require.NoError(t, err)
			require.Equal(t, attempt.wantModel, saved.UpstreamModelID)
			require.Equal(t, wantStatus, saved.Status)
			require.Empty(t, saved.ResponseBody)
		})
	}

	// A transport failure on the next attempt must not inherit the last raw
	// response, even when no OnOutboundRawResponse callback occurs.
	state.RequestExec = createUpstreamModelTestExecution(t, ctx, client, state.Request, "next-model", llm.APIFormatOpenAIChatCompletion, false)
	_, err := middleware.OnOutboundRawRequest(ctx, &httpclient.Request{})
	require.NoError(t, err)
	middleware.OnOutboundRawError(ctx, io.ErrUnexpectedEOF)
	saved, err := client.RequestExecution.Get(ctx, state.RequestExec.ID)
	require.NoError(t, err)
	require.Empty(t, saved.UpstreamModelID)
	require.Equal(t, requestexecution.StatusFailed, saved.Status)
}

func TestUpstreamModelPersistence_StreamingTerminations(t *testing.T) {
	tests := []struct {
		name         string
		format       llm.APIFormat
		events       []*httpclient.StreamEvent
		streamErr    error
		aggregateErr error
		cancel       bool
		wantModel    string
		wantStatus   requestexecution.Status
	}{
		{
			name: "completed with model only in the first event",
			events: []*httpclient.StreamEvent{
				{Data: []byte(`{"model":"provider-model","choices":[]}`)},
				{Data: []byte(`[DONE]`)},
			},
			wantModel: "provider-model", wantStatus: requestexecution.StatusCompleted,
		},
		{
			name: "transport interruption after Anthropic message start", format: llm.APIFormatAnthropicMessage,
			events:    []*httpclient.StreamEvent{{Type: "message_start", Data: []byte(`{"type":"message_start","message":{"model":"claude-version"}}`)}},
			streamErr: io.ErrUnexpectedEOF, wantModel: "claude-version", wantStatus: requestexecution.StatusFailed,
		},
		{
			name: "clean EOF after Responses metadata", format: llm.APIFormatOpenAIResponse,
			events:    []*httpclient.StreamEvent{{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"model":"gpt-version"}}`)}},
			wantModel: "gpt-version", wantStatus: requestexecution.StatusFailed,
		},
		{
			name: "client canceled after Gemini metadata", format: llm.APIFormatGeminiContents,
			events: []*httpclient.StreamEvent{{Data: []byte(`{"modelVersion":"gemini-version","candidates":[]}`)}},
			cancel: true, wantModel: "gemini-version", wantStatus: requestexecution.StatusCanceled,
		},
		{
			name: "aggregation error after a known terminal",
			events: []*httpclient.StreamEvent{
				{Data: []byte(`{"model":"provider-model","choices":[]}`)},
				{Data: []byte(`[DONE]`)},
			},
			aggregateErr: errors.New("cannot aggregate response"), wantModel: "provider-model", wantStatus: requestexecution.StatusCompleted,
		},
		{
			name: "failed Responses terminal includes model", format: llm.APIFormatOpenAIResponse,
			events:    []*httpclient.StreamEvent{{Type: "response.failed", Data: []byte(`{"type":"response.failed","response":{"model":"gpt-version","status":"failed"}}`)}},
			wantModel: "gpt-version", wantStatus: requestexecution.StatusFailed,
		},
		{
			name:       "no reported model stays unknown",
			events:     []*httpclient.StreamEvent{{Data: []byte(`[DONE]`)}},
			wantStatus: requestexecution.StatusCompleted,
		},
		{
			name:      "binary payload is never model metadata",
			events:    []*httpclient.StreamEvent{{Type: "audio/pcm", Data: []byte(`{"model":"binary-content"}`)}},
			streamErr: io.ErrUnexpectedEOF, wantStatus: requestexecution.StatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, client, state := newUpstreamModelPersistenceTest(t)
			format := tt.format
			if format == "" {
				format = llm.APIFormatOpenAIChatCompletion
			}
			execution := createUpstreamModelTestExecution(t, ctx, client, state.Request, "sent-model", format, true)
			streamCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			source := &sliceEventStream{events: tt.events, err: tt.streamErr}
			outbound := &mockTransformer{
				apiFormat:          llm.APIFormatOpenAIChatCompletion,
				aggregatedResponse: []byte(`{"id":"response-id"}`),
				aggregatedErr:      tt.aggregateErr,
			}
			stream := NewOutboundPersistentStream(streamCtx, source, state.Request, execution, state.RequestService, state.UsageLogService, outbound, nil, state)
			for stream.Next() {
				_ = stream.Current()
			}
			if tt.cancel {
				cancel()
			}
			require.NoError(t, stream.Close())
			require.NoError(t, stream.Close())

			saved, err := client.RequestExecution.Get(ctx, execution.ID)
			require.NoError(t, err)
			require.Equal(t, tt.wantModel, saved.UpstreamModelID)
			require.Equal(t, tt.wantStatus, saved.Status)
			require.Empty(t, saved.ResponseBody, "metadata must survive disabled body storage")
			require.Empty(t, saved.ResponseChunks, "metadata must survive disabled chunk storage")
		})
	}
}

// Streams keep the first reported name even if later chunks report a different one.
func TestUpstreamModelPersistence_KeepsOnlyFirstReportedName(t *testing.T) {
	ctx, client, state := newUpstreamModelPersistenceTest(t)
	execution := createUpstreamModelTestExecution(t, ctx, client, state.Request, "sent-model", llm.APIFormatOpenAIChatCompletion, true)
	events := []*httpclient.StreamEvent{
		{Data: []byte("{\"model\":\"first-model\",\"choices\":[]}")},
		{Data: []byte("{\"model\":\"second-model\",\"choices\":[]}")},
		{Data: []byte("[DONE]")},
	}
	outbound := &mockTransformer{apiFormat: llm.APIFormatOpenAIChatCompletion, aggregatedResponse: []byte("{\"id\":\"resp\"}")}
	stream := NewOutboundPersistentStream(ctx, &sliceEventStream{events: events}, state.Request, execution, state.RequestService, state.UsageLogService, outbound, nil, state)
	for stream.Next() {
		_ = stream.Current()
	}
	require.NoError(t, stream.Close())
	saved, err := client.RequestExecution.Get(ctx, execution.ID)
	require.NoError(t, err)
	require.Equal(t, "first-model", saved.UpstreamModelID, "a later reported name must not overwrite the first")
}
