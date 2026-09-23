package orchestrator

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/gemini"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func newModelAuditOutbound(t *testing.T, format llm.APIFormat) transformer.Outbound {
	t.Helper()
	var outbound transformer.Outbound
	var err error
	switch format {
	case llm.APIFormatAnthropicMessage:
		outbound, err = anthropic.NewOutboundTransformer("https://example.invalid", "test-key")
	case llm.APIFormatGeminiContents:
		outbound, err = gemini.NewOutboundTransformer("https://example.invalid", "test-key")
	case llm.APIFormatOpenAIResponse:
		outbound, err = responses.NewOutboundTransformer("https://example.invalid", "test-key")
	default:
		outbound, err = openai.NewOutboundTransformer("https://example.invalid", "test-key")
	}
	require.NoError(t, err)
	return outbound
}

func prepareModelAuditCandidate(t *testing.T, ctx context.Context, db *ent.Client, state *PersistenceState, format llm.APIFormat, passThrough, forceStream bool) {
	t.Helper()
	entity, err := db.Channel.Get(ctx, state.Request.ChannelID)
	require.NoError(t, err)
	entity.Settings = &objects.ChannelSettings{
		PassThroughBody:        lo.ToPtr(passThrough),
		BodyOverrideOperations: []objects.OverrideOperation{{Op: objects.OverrideOpSet, Path: "model", Value: "wire-b"}},
	}
	if forceStream {
		entity.Policies.Stream = objects.CapabilityPolicyRequire
	}
	channel := &biz.Channel{Channel: entity, Outbound: newModelAuditOutbound(t, format)}
	state.OriginalModel = "client-alias"
	state.ChannelModelsCandidates = []*ChannelModelsCandidate{{
		Channel: channel, APIFormat: string(format),
		Models: []biz.ChannelModelEntry{{RequestModel: "client-alias", ActualModel: "routed-a", Source: "direct"}},
	}}
}

func modelAuditMiddlewares(inbound *PersistentInboundTransformer, outbound *PersistentOutboundTransformer) []pipeline.Middleware {
	return []pipeline.Middleware{
		applyModelMapping(inbound),
		applyPassThroughResponse(outbound, nil),
		applyPassThroughStream(outbound, nil),
		applyPassThroughRequestBody(outbound, nil),
		applyOverrideRequestBody(outbound),
		finalizeTransportRequest(outbound),
		persistRequestExecution(outbound),
		captureRawProviderResponse(outbound, nil),
		captureRawProviderStream(outbound, nil),
	}
}

func TestUpstreamModelPipeline_NonStreaming(t *testing.T) {
	tests := []struct {
		name        string
		format      llm.APIFormat
		response    string
		reported    string
		passThrough bool
		wantError   bool
	}{
		{
			name: "OpenAI final override", format: llm.APIFormatOpenAIChatCompletion,
			response: `{"id":"resp","model":"wire-b","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`,
			reported: "wire-b",
		},
		{
			name: "OpenAI response reports the channel model", format: llm.APIFormatOpenAIChatCompletion,
			response: `{"id":"resp","model":"routed-a","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`,
			reported: "routed-a",
		},
		{
			name: "model survives response conversion failure", format: llm.APIFormatOpenAIChatCompletion,
			response: `{"id":"resp","model":"wire-b","choices":"invalid"}`,
			reported: "wire-b", wantError: true,
		},
		{
			name: "OpenAI pass through and override", format: llm.APIFormatOpenAIChatCompletion, passThrough: true,
			response: `{"id":"resp","model":"wire-b","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`,
			reported: "wire-b",
		},
		{
			name: "Anthropic protocol conversion", format: llm.APIFormatAnthropicMessage,
			response: `{"id":"resp","type":"message","role":"assistant","model":"wire-b","content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`,
			reported: "wire-b",
		},
		{
			name: "Responses protocol conversion", format: llm.APIFormatOpenAIResponse,
			response: `{"id":"resp","object":"response","status":"completed","model":"wire-b","output":[{"type":"message","id":"msg","status":"completed","role":"assistant","content":[{"type":"output_text","text":"hello","annotations":[]}]}]}`,
			reported: "wire-b",
		},
		{
			name: "Gemini URL model survives unrelated JSON override", format: llm.APIFormatGeminiContents,
			response: `{"responseId":"resp","modelVersion":"routed-a","candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"hello"}]},"finishReason":"STOP"}]}`,
			reported: "routed-a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, db, state := newUpstreamModelPersistenceTest(t)
			prepareModelAuditCandidate(t, ctx, db, state, tt.format, tt.passThrough, false)
			inbound, outbound := NewPersistentTransformers(state, openai.NewInboundTransformer())
			executor := &mockExecutor{response: &httpclient.Response{
				StatusCode: http.StatusOK, Headers: http.Header{"Content-Type": {"application/json"}}, Body: []byte(tt.response),
			}}
			pipe := pipeline.NewFactory(executor).Pipeline(inbound, outbound, pipeline.WithMiddlewares(modelAuditMiddlewares(inbound, outbound)...))
			result, err := pipe.Process(ctx, buildTestRequest("client-alias", "hello", false))
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result.Response)
				if tt.passThrough {
					require.Equal(t, tt.response, string(result.Response.Body))
				} else {
					require.Equal(t, "client-alias", gjson.GetBytes(result.Response.Body, "model").String(), "client rewriting is separate from upstream evidence")
				}
			}
			require.Equal(t, "wire-b", gjson.GetBytes(executor.lastRequest.Body, "model").String())
			if tt.format == llm.APIFormatGeminiContents {
				require.Contains(t, executor.lastRequest.URL, "/models/routed-a:")
			}
			saved, err := db.RequestExecution.Get(ctx, state.RequestExec.ID)
			require.NoError(t, err)
			require.Equal(t, "routed-a", saved.ModelID, "persisted model remains the channel model used for pricing")
			require.Equal(t, tt.reported, saved.UpstreamModelID)
			if tt.wantError {
				require.Equal(t, requestexecution.StatusFailed, saved.Status)
			}
			require.Equal(t, tt.passThrough, saved.PassThroughApplied)
			require.JSONEq(t, "{}", string(saved.RequestBody))
			require.Empty(t, saved.ResponseBody)
		})
	}
}

func TestUpstreamModelPipeline_ChannelModelPricingAfterOverride(t *testing.T) {
	ctx, db, state := newUpstreamModelPersistenceTest(t)
	prepareModelAuditCandidate(t, ctx, db, state, llm.APIFormatOpenAIChatCompletion, false, false)
	_, err := db.ChannelModelPrice.Create().
		SetChannelID(state.Request.ChannelID).
		SetModelID("routed-a").
		SetReferenceID("channel-model-price").
		SetPrice(objects.ModelPrice{Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeUsage,
				Pricing: objects.Pricing{
					Mode: objects.PricingModeFlatFee, FlatFee: lo.ToPtr(decimal.NewFromFloat(0.5)),
				},
			},
		}}).Save(ctx)
	require.NoError(t, err)
	channelService := state.UsageLogService.ChannelService
	channel, err := channelService.GetChannel(ctx, state.Request.ChannelID)
	require.NoError(t, err)
	channelService.PreloadModelPricesForTest(ctx, channel)
	channelService.SetEnabledChannelsForTest([]*biz.Channel{channel})

	inbound, outbound := NewPersistentTransformers(state, openai.NewInboundTransformer())
	executor := &mockExecutor{response: &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"id":"resp","model":"wire-b","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`),
	}}
	pipe := pipeline.NewFactory(executor).Pipeline(inbound, outbound, pipeline.WithMiddlewares(modelAuditMiddlewares(inbound, outbound)...))
	_, err = pipe.Process(ctx, buildTestRequest("client-alias", "hello", false))
	require.NoError(t, err)
	require.Equal(t, "wire-b", gjson.GetBytes(executor.lastRequest.Body, "model").String())

	usage := &llm.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}
	usageLog, err := state.UsageLogService.CreateUsageLogFromRequest(ctx, state.Request, state.RequestExec, usage)
	require.NoError(t, err)
	require.Equal(t, "routed-a", usageLog.ModelID)
	require.NotNil(t, usageLog.TotalCost)
	require.InDelta(t, 0.5, *usageLog.TotalCost, 1e-12)
	state.UsageLogService.InjectUsageCost(ctx, state.RequestExec.ChannelID, state.RequestExec.ModelID, usage)
	require.Equal(t, usageLog.TotalCost, usage.Cost)
}

func TestUpstreamModelPipeline_StreamingAndAutoAggregate(t *testing.T) {
	for _, tt := range []struct {
		name                     string
		passThrough, forceStream bool
	}{
		{name: "stream conversion"},
		{name: "pass through stream", passThrough: true},
		{name: "upstream stream aggregated for nonstream client", forceStream: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, db, state := newUpstreamModelPersistenceTest(t)
			prepareModelAuditCandidate(t, ctx, db, state, llm.APIFormatOpenAIChatCompletion, tt.passThrough, tt.forceStream)
			state.Request.Stream = !tt.forceStream
			inbound, outbound := NewPersistentTransformers(state, openai.NewInboundTransformer())
			events := []*httpclient.StreamEvent{
				{Data: []byte(`{"id":"resp","model":"wire-b","choices":[{"index":0,"delta":{"role":"assistant","content":"hello"}}]}`)},
				{Data: []byte(`{"id":"resp","model":"changed-c","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":"stop"}]}`)},
				{Data: []byte("[DONE]")},
			}
			executor := &mockExecutor{streamEvents: events}
			pipe := pipeline.NewFactory(executor).Pipeline(inbound, outbound, pipeline.WithMiddlewares(modelAuditMiddlewares(inbound, outbound)...))
			result, err := pipe.Process(ctx, buildTestRequest("client-alias", "hello", !tt.forceStream))
			require.NoError(t, err)
			if tt.forceStream {
				require.NotNil(t, result.Response)
				require.Nil(t, result.EventStream)
				require.Contains(t, string(result.Response.Body), "hello world")
			} else {
				require.NotNil(t, result.EventStream)
				var received []*httpclient.StreamEvent
				for result.EventStream.Next() {
					received = append(received, result.EventStream.Current())
				}
				require.NoError(t, result.EventStream.Err())
				require.NoError(t, result.EventStream.Close())
				if tt.passThrough {
					require.Equal(t, events, received)
				}
			}
			// Pass-through drains the transformation branch asynchronously.
			require.Eventually(t, func() bool {
				saved, err := db.RequestExecution.Get(ctx, state.RequestExec.ID)
				return err == nil && saved.Status == requestexecution.StatusCompleted && saved.ModelID == "routed-a" && saved.UpstreamModelID == "wire-b"
			}, time.Second, 5*time.Millisecond)
			saved, err := db.RequestExecution.Get(ctx, state.RequestExec.ID)
			require.NoError(t, err)
			require.Equal(t, "routed-a", saved.ModelID, "persisted model remains the channel model used for pricing")
			require.Equal(t, "wire-b", saved.UpstreamModelID, "only the first reported name is kept")
			require.Empty(t, saved.ResponseChunks)
			require.Empty(t, saved.ResponseBody)
		})
	}
}

func TestUpstreamModelPersistence_CodexImageResponsesStream(t *testing.T) {
	for _, format := range []llm.APIFormat{llm.APIFormatOpenAIImageGeneration, llm.APIFormatOpenAIImageEdit} {
		t.Run(string(format), func(t *testing.T) {
			ctx, db, state := newUpstreamModelPersistenceTest(t)
			outbound, err := codex.NewOutboundTransformer(codex.Params{
				BaseURL: "https://chatgpt.com/backend-api/codex#",
				TokenProvider: oauth.NewStaticTokenProvider(&oauth.OAuthCredentials{
					AccessToken: "test-token",
				}),
			})
			require.NoError(t, err)
			providerRequest, err := outbound.TransformRequest(ctx, &llm.Request{
				Model: "gpt-image-2", RequestType: llm.RequestTypeImage, APIFormat: format,
				RawRequest: &httpclient.Request{Headers: http.Header{}},
				Image: &llm.ImageRequest{
					Prompt: "draw a tree", Images: [][]byte{[]byte("image-data")},
				},
			})
			require.NoError(t, err)
			require.Equal(t, string(format), providerRequest.APIFormat)
			require.Equal(t, llm.APIFormatOpenAIResponse, outbound.APIFormat())
			state.RawProviderRequest = providerRequest
			execution := createUpstreamModelTestExecution(t, ctx, db, state.Request, "gpt-image-2", format, true)
			events := []*httpclient.StreamEvent{
				{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp","model":"gpt-5.4-mini","status":"in_progress","output":[]}}`)},
				{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp","model":"gpt-5.4-mini","status":"completed","output":[{"type":"image_generation_call","id":"img","status":"completed","result":"aW1hZ2U="}]}}`)},
			}
			stream := NewOutboundPersistentStream(ctx, &sliceEventStream{events: events}, state.Request, execution,
				state.RequestService, state.UsageLogService, outbound, nil, state)
			for stream.Next() {
				_ = stream.Current()
			}
			require.NoError(t, stream.Close())
			saved, err := db.RequestExecution.Get(ctx, execution.ID)
			require.NoError(t, err)
			require.Equal(t, requestexecution.StatusCompleted, saved.Status)
			require.Equal(t, "gpt-image-2", saved.ModelID)
			require.Equal(t, "gpt-5.4-mini", saved.UpstreamModelID)
			require.Empty(t, saved.ResponseBody)
			require.Empty(t, saved.ResponseChunks)
		})
	}
}

func TestUpstreamModelPipeline_AlphaSearchKeepsChannelModel(t *testing.T) {
	ctx, db, state := newUpstreamModelPersistenceTest(t)
	prepareModelAuditCandidate(t, ctx, db, state, llm.APIFormatOpenAIAlphaSearch, false, false)
	inbound, outbound := NewPersistentTransformers(state, openai.NewAlphaSearchInboundTransformer())
	executor := &mockExecutor{response: &httpclient.Response{
		StatusCode: http.StatusOK,
		Request:    &httpclient.Request{APIFormat: string(llm.APIFormatOpenAIAlphaSearch)},
		Body:       []byte(`{"results":[{"title":"Go","model":"search-result"}]}`),
	}}
	pipe := pipeline.NewFactory(executor).Pipeline(inbound, outbound, pipeline.WithMiddlewares(modelAuditMiddlewares(inbound, outbound)...))
	result, err := pipe.Process(ctx, &httpclient.Request{
		Method: http.MethodPost, Headers: http.Header{"Content-Type": {"application/json"}},
		Body: []byte(`{"model":"client-alias","commands":{"search_query":[{"q":"golang"}]}}`),
	})
	require.NoError(t, err)
	require.JSONEq(t, string(executor.response.Body), string(result.Response.Body))
	saved, err := db.RequestExecution.Get(ctx, state.RequestExec.ID)
	require.NoError(t, err)
	require.Equal(t, "routed-a", saved.ModelID)
	require.Empty(t, saved.UpstreamModelID, "search results do not report the upstream model")
}

func TestUpstreamModelPipeline_TextTranscription(t *testing.T) {
	ctx, db, state := newUpstreamModelPersistenceTest(t)
	state.RequestExec = createUpstreamModelTestExecution(t, ctx, db, state.Request, "whisper-1", llm.APIFormatOpenAITranscription, false)
	outbound := newModelAuditOutbound(t, llm.APIFormatOpenAIChatCompletion)
	middleware := &persistRequestExecutionMiddleware{outbound: &PersistentOutboundTransformer{state: state, wrapped: outbound}}
	_, err := middleware.OnOutboundRawRequest(ctx, &httpclient.Request{})
	require.NoError(t, err)
	transcript := `{"model":"whisper-1"}`
	raw, err := middleware.OnOutboundRawResponse(ctx, &httpclient.Response{
		StatusCode: http.StatusOK, Headers: http.Header{"Content-Type": {"text/plain"}},
		Request: &httpclient.Request{APIFormat: string(llm.APIFormatOpenAITranscription)}, Body: []byte(transcript),
	})
	require.NoError(t, err)
	response, err := outbound.TransformResponse(ctx, raw)
	require.NoError(t, err)
	require.Equal(t, transcript, response.Transcription.Text)
	_, err = middleware.OnOutboundLlmResponse(ctx, response)
	require.NoError(t, err)
	saved, err := db.RequestExecution.Get(ctx, state.RequestExec.ID)
	require.NoError(t, err)
	require.Empty(t, saved.UpstreamModelID)
}

func TestUpstreamModelPipeline_ChannelRetry(t *testing.T) {
	for _, reported := range []string{"first-a", ""} {
		t.Run("second response model="+reported, func(t *testing.T) {
			ctx, db, state := newUpstreamModelPersistenceTest(t)
			prepareModelAuditCandidate(t, ctx, db, state, llm.APIFormatOpenAIChatCompletion, false, false)
			first := state.ChannelModelsCandidates[0]
			first.Channel.Settings.BodyOverrideOperations[0].Value = "first-a"
			secondRow := db.Channel.Create().SetName("retry-channel").SetType(first.Channel.Type).
				SetBaseURL("https://example.invalid").SetCredentials(first.Channel.Credentials).
				SetSupportedModels([]string{"routed-b"}).SetDefaultTestModel("routed-b").SaveX(ctx)
			secondRow.Settings = &objects.ChannelSettings{
				PassThroughBody:        lo.ToPtr(false),
				BodyOverrideOperations: []objects.OverrideOperation{{Op: objects.OverrideOpSet, Path: "model", Value: "second-b"}},
			}
			state.ChannelModelsCandidates = append(state.ChannelModelsCandidates, &ChannelModelsCandidate{
				Channel:   &biz.Channel{Channel: secondRow, Outbound: newModelAuditOutbound(t, llm.APIFormatOpenAIChatCompletion)},
				APIFormat: string(llm.APIFormatOpenAIChatCompletion),
				Models:    []biz.ChannelModelEntry{{RequestModel: "client-alias", ActualModel: "routed-b", Source: "direct"}},
			})
			inbound, outbound := NewPersistentTransformers(state, openai.NewInboundTransformer())
			executor := pipeline.NewMockExecutor(t)
			executor.EXPECT().Do(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, req *httpclient.Request) (*httpclient.Response, error) {
				require.Equal(t, "first-a", gjson.GetBytes(req.Body, "model").String())
				return &httpclient.Response{StatusCode: http.StatusOK, Request: req, Body: []byte(`{"id":"first","model":"first-a","choices":[]}`)}, nil
			}).Once()
			executor.EXPECT().Do(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, req *httpclient.Request) (*httpclient.Response, error) {
				require.Equal(t, "second-b", gjson.GetBytes(req.Body, "model").String())
				return &httpclient.Response{
					StatusCode: http.StatusOK, Request: req,
					Body: []byte(fmt.Sprintf(`{"id":"second","model":%q,"choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`, reported)),
				}, nil
			}).Once()
			pipe := pipeline.NewFactory(executor).Pipeline(inbound, outbound,
				pipeline.WithRetry(1, 0, 0), pipeline.WithEmptyResponseDetection(),
				pipeline.WithMiddlewares(modelAuditMiddlewares(inbound, outbound)...))
			result, err := pipe.Process(ctx, buildTestRequest("client-alias", "hello", false))
			require.NoError(t, err)
			require.NotNil(t, result.Response)
			executions, err := db.RequestExecution.Query().Order(ent.Asc(requestexecution.FieldID)).All(ctx)
			require.NoError(t, err)
			require.Len(t, executions, 2)
			require.NotEqual(t, executions[0].ChannelID, executions[1].ChannelID)
			require.Equal(t, "routed-a", executions[0].ModelID)
			require.Equal(t, "routed-b", executions[1].ModelID)
			require.Equal(t, requestexecution.StatusFailed, executions[0].Status)
			require.Equal(t, "first-a", executions[0].UpstreamModelID)
			require.Equal(t, requestexecution.StatusCompleted, executions[1].Status)
			require.Equal(t, reported, executions[1].UpstreamModelID)
		})
	}
}

func TestUpstreamModelPersistence_ConflictsSurviveTermination(t *testing.T) {
	for _, tt := range []struct {
		name                    string
		terminal, cancel        bool
		streamErr, aggregateErr error
		status                  requestexecution.Status
	}{
		{name: "normal terminal", terminal: true, status: requestexecution.StatusCompleted},
		{name: "unexpected EOF", streamErr: io.ErrUnexpectedEOF, status: requestexecution.StatusFailed},
		{name: "client cancellation", cancel: true, status: requestexecution.StatusCanceled},
		{name: "aggregation failure", terminal: true, aggregateErr: io.ErrUnexpectedEOF, status: requestexecution.StatusCompleted},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, db, state := newUpstreamModelPersistenceTest(t)
			execution := createUpstreamModelTestExecution(t, ctx, db, state.Request, "a", llm.APIFormatOpenAIChatCompletion, true)
			events := []*httpclient.StreamEvent{
				{Data: []byte(`{"choices":[]}`)},
				{Data: []byte(`{"model":"a","choices":[]}`)},
				{Data: []byte(`{"model":"a","choices":[]}`)},
				{Data: []byte(`{"model":"b","choices":[]}`)},
				{Data: []byte(`{"model":"c","choices":[]}`)},
			}
			if tt.terminal {
				events = append(events, &httpclient.StreamEvent{Data: []byte("[DONE]")})
			}
			streamCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			outbound := &mockTransformer{apiFormat: llm.APIFormatOpenAIChatCompletion, aggregatedResponse: []byte(`{"id":"resp"}`), aggregatedErr: tt.aggregateErr}
			stream := NewOutboundPersistentStream(streamCtx, &sliceEventStream{events: events, err: tt.streamErr}, state.Request, execution, state.RequestService, state.UsageLogService, outbound, nil, state)
			for stream.Next() {
				_ = stream.Current()
			}
			if tt.cancel {
				cancel()
			}
			require.NoError(t, stream.Close())
			require.NoError(t, stream.Close())
			saved, err := db.RequestExecution.Get(ctx, execution.ID)
			require.NoError(t, err)
			require.Equal(t, tt.status, saved.Status)
			require.Equal(t, "a", saved.UpstreamModelID)
			require.Equal(t, "a", saved.UpstreamModelID, "only the first reported name is kept")
			require.Empty(t, saved.ResponseChunks)
			require.NoError(t, state.RequestService.UpdateRequestExecutionStatus(ctx, saved.ID, saved.Status, "", nil))
			saved, err = db.RequestExecution.Get(ctx, saved.ID)
			require.NoError(t, err)
			require.Equal(t, "a", saved.UpstreamModelID, "status-only updates preserve the recorded name")
		})
	}
}
