package orchestrator

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestPersistentStreams_ImagesSSE(t *testing.T) {
	for _, edit := range []bool{false, true} {
		for _, n := range []int{1, 2} {
			for _, outcome := range []string{"completed", "incomplete", "canceled", "error", "malformed", "done"} {
				complete := outcome == "completed"
				name := "images"
				if edit {
					name += "/edit"
				} else {
					name += "/generation"
				}
				name += fmt.Sprintf("/n=%d/%s", n, outcome)
				t.Run(name, func(t *testing.T) {
					client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
					defer client.Close()
					ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
					streamCtx, cancel := context.WithCancel(ctx)
					defer cancel()
					project := createTestProject(t, ctx, client)
					channel := createTestChannel(t, ctx, client)
					_, requestService, systemService, usageLogService := setupTestServices(t, client)
					require.NoError(t, systemService.SetStoragePolicy(ctx, &biz.StoragePolicy{
						StoreChunks: true, StoreRequestBody: true, StoreResponseBody: true,
					}))
					inbound := openai.NewImageGenerationInboundTransformer()
					prefix := "image_generation."
					if edit {
						inbound = openai.NewImageEditInboundTransformer()
						prefix = "image_edit."
					}
					format := inbound.APIFormat()
					outbound, err := openai.NewOutboundTransformer("https://example.test", "test-key")
					require.NoError(t, err)
					body := []byte(fmt.Sprintf(`{"stream":true,"n":%d}`, n))
					req, err := client.Request.Create().SetProjectID(project.ID).SetChannelID(channel.ID).
						SetModelID("gpt-image-1").SetStatus(request.StatusProcessing).
						SetRequestBody(body).SetStream(true).Save(ctx)
					require.NoError(t, err)
					execution, err := client.RequestExecution.Create().SetRequestID(req.ID).SetProjectID(project.ID).
						SetChannelID(channel.ID).SetModelID("gpt-image-1").SetFormat(format.String()).
						SetStatus(requestexecution.StatusProcessing).SetRequestBody(body).SetStream(true).Save(ctx)
					require.NoError(t, err)
					events := []*httpclient.StreamEvent{{Type: prefix + "partial_image", Data: []byte(`{"type":"` + prefix + `partial_image","b64_json":"cHJldmlldw==","partial_image_index":0,"created_at":123}`)}}
					if complete || n > 1 {
						events = append(events, &httpclient.StreamEvent{Type: prefix + "completed", Data: []byte(`{"type":"` + prefix + `completed","b64_json":"ZmluYWw=","created_at":123,"usage":{"input_tokens":10,"output_tokens":20,"total_tokens":30}}`)})
					}
					if complete && n > 1 {
						events = append(events, &httpclient.StreamEvent{Type: prefix + "completed", Data: []byte(`{"type":"` + prefix + `completed","b64_json":"c2Vjb25k","created_at":123,"usage":{"input_tokens":10,"output_tokens":20,"total_tokens":30}}`)})
					}
					if outcome == "done" {
						events = append(events, &llm.DoneStreamEvent)
					}
					if outcome == "error" {
						events = append(events, &httpclient.StreamEvent{Type: "error", Data: []byte(`{"error":{"message":"image failed"}}`)})
					}
					if outcome == "malformed" {
						events = append(events, &httpclient.StreamEvent{Type: prefix + "completed", Data: []byte(`{"type":"` + prefix + `completed","b64_json":"final","created_at":"bad"}`)})
					}
					rawReq := &httpclient.Request{RequestType: llm.RequestTypeImage.String(), APIFormat: format.String(), Body: body}
					state := &PersistenceState{RawProviderRequest: rawReq}
					raw := NewOutboundPersistentStream(streamCtx, streams.SliceStream(events), req, execution,
						requestService, usageLogService, outbound, nil, state)
					normalized, err := outbound.TransformStream(streamCtx, rawReq, raw)
					require.NoError(t, err)
					converted, err := inbound.TransformStream(streamCtx, normalized)
					require.NoError(t, err)
					stream := NewInboundPersistentStream(streamCtx, converted, req, execution, requestService, inbound, nil, state)
					for stream.Next() {
						event := stream.Current()
						if (complete && IsTerminalStreamEvent(event)) || (outcome == "canceled" &&
							((n == 1 && event.Type == prefix+"partial_image") || (n > 1 && event.Type == prefix+"completed"))) {
							cancel()
						}
					}
					if complete {
						require.NoError(t, stream.Err())
					} else if outcome == "canceled" {
						require.ErrorIs(t, stream.Err(), context.Canceled)
					} else if outcome == "error" {
						require.ErrorContains(t, stream.Err(), "image failed")
					} else if outcome == "malformed" {
						require.ErrorContains(t, stream.Err(), "invalid image stream event")
					} else {
						require.ErrorIs(t, stream.Err(), llm.ErrStreamIncomplete)
					}
					require.NoError(t, stream.Close())
					savedRequest, err := client.Request.Get(ctx, req.ID)
					require.NoError(t, err)
					savedExecution, err := client.RequestExecution.Get(ctx, execution.ID)
					require.NoError(t, err)
					want := request.StatusFailed
					if complete {
						want = request.StatusCompleted
					} else if outcome == "canceled" {
						want = request.StatusCanceled
					}
					require.Equal(t, want, savedRequest.Status)
					require.Equal(t, string(want), string(savedExecution.Status))
					require.Equal(t, complete, state.StreamCompleted)
					if complete {
						require.Equal(t, "ZmluYWw=", gjson.GetBytes(savedRequest.ResponseBody, "data.0.b64_json").String())
						require.NotContains(t, string(savedRequest.ResponseBody), "cHJldmlldw==")
						require.Len(t, gjson.GetBytes(savedRequest.ResponseBody, "data").Array(), n)
						if n > 1 {
							require.Equal(t, "c2Vjb25k", gjson.GetBytes(savedRequest.ResponseBody, "data.1.b64_json").String())
						}
						require.Equal(t, int64(30*n), gjson.GetBytes(savedRequest.ResponseBody, "usage.total_tokens").Int())
						require.Equal(t, int64(30*n), gjson.GetBytes(savedExecution.ResponseBody, "usage.total_tokens").Int())
						usage, err := client.UsageLog.Query().Only(ctx)
						require.NoError(t, err)
						require.Equal(t, int64(30*n), usage.TotalTokens)
					} else {
						usageCount, err := client.UsageLog.Query().Count(ctx)
						require.NoError(t, err)
						require.Zero(t, usageCount, "incomplete requests must not create successful usage records")
					}
					require.NotEmpty(t, savedRequest.ResponseChunks)
					require.NotEmpty(t, savedExecution.ResponseChunks)
				})
			}
		}
	}
}

func TestPassThroughStream_ImagesSSECountsBeforeFanOut(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	for _, prefix := range []string{"image_generation.", "image_edit."} {
		format := llm.APIFormatOpenAIImageGeneration
		if prefix == "image_edit." {
			format = llm.APIFormatOpenAIImageEdit
		}
		state := &PersistenceState{
			CurrentCandidate:      &ChannelModelsCandidate{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "images", Settings: &objects.ChannelSettings{PassThroughBody: lo.ToPtr(true)}}}},
			OriginalRequestStream: lo.ToPtr(true),
			LlmRequest:            &llm.Request{APIFormat: format, Stream: lo.ToPtr(true), RawRequest: &httpclient.Request{APIFormat: format.String()}},
			RawProviderRequest:    &httpclient.Request{APIFormat: format.String(), Body: []byte(`{"n":2}`)},
		}
		wrapped, err := openai.NewOutboundTransformer("https://example.test", "test-key")
		require.NoError(t, err)
		outbound := &PersistentOutboundTransformer{state: state, wrapped: wrapped}
		original := []*httpclient.StreamEvent{
			{Type: prefix + "completed", Data: []byte(`{"type":"` + prefix + `completed","b64_json":"first"}`)},
			{Type: prefix + "completed", Data: []byte(`{"type":"` + prefix + `completed","b64_json":"second"}`)},
		}
		pipelineStream, err := captureRawProviderStream(outbound, nil).OnOutboundRawStream(ctx, streams.SliceStream(original))
		require.NoError(t, err)
		defer pipelineStream.Close()
		var pipelineEvents []*httpclient.StreamEvent
		for pipelineStream.Next() {
			pipelineEvents = append(pipelineEvents, pipelineStream.Current())
		}
		require.Len(t, pipelineEvents, 2)
		rawStream, err := applyPassThroughStream(outbound, nil).OnInboundRawStream(ctx, streams.SliceStream([]*httpclient.StreamEvent{}))
		require.NoError(t, err)
		defer rawStream.Close()
		var rawEvents []*httpclient.StreamEvent
		for rawStream.Next() {
			rawEvents = append(rawEvents, rawStream.Current())
		}
		require.Len(t, rawEvents, 2)
		for _, events := range [][]*httpclient.StreamEvent{pipelineEvents, rawEvents} {
			require.False(t, IsTerminalStreamEvent(events[0]))
			require.True(t, IsTerminalStreamEvent(events[1]))
		}
		require.Nil(t, original[0].ImageStreamCompleted, "the shared upstream event must remain immutable")
	}
}

func TestPassThroughStream_ImagesSSERejectsPrematureTerminal(t *testing.T) {
	for _, format := range []llm.APIFormat{llm.APIFormatOpenAIImageGeneration, llm.APIFormatOpenAIImageEdit} {
		for _, n := range []int{1, 2} {
			for _, terminal := range []*httpclient.StreamEvent{
				&llm.DoneStreamEvent,
				{Type: "response.completed", Data: []byte(`{"type":"response.completed"}`)},
				{Type: "message_stop", Data: []byte(`{"type":"message_stop"}`)},
				{Data: []byte(`{"choices":[{"finish_reason":"stop"}]}`)},
			} {
				t.Run(fmt.Sprintf("%s/n=%d/%s", format, n, terminal.Data), func(t *testing.T) {
					ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
					defer cancel()
					prefix := "image_generation."
					if format == llm.APIFormatOpenAIImageEdit {
						prefix = "image_edit."
					}
					state := &PersistenceState{
						CurrentCandidate: &ChannelModelsCandidate{Channel: &biz.Channel{Channel: &ent.Channel{
							ID: 1, Name: "images", Settings: &objects.ChannelSettings{PassThroughBody: lo.ToPtr(true)},
						}}},
						OriginalRequestStream: lo.ToPtr(true),
						LlmRequest: &llm.Request{APIFormat: format, Stream: lo.ToPtr(true),
							RawRequest: &httpclient.Request{APIFormat: format.String()}},
						RawProviderRequest: &httpclient.Request{APIFormat: format.String(), Body: []byte(fmt.Sprintf(`{"n":%d}`, n))},
					}
					wrapped, err := openai.NewOutboundTransformer("https://example.test", "test-key")
					require.NoError(t, err)
					outbound := &PersistentOutboundTransformer{state: state, wrapped: wrapped}
					original := []*httpclient.StreamEvent{{Type: prefix + "partial_image", Data: []byte(`{"type":"` + prefix + `partial_image","b64_json":"preview","partial_image_index":0}`)}}
					if n == 2 {
						original = append(original, &httpclient.StreamEvent{Type: prefix + "completed", Data: []byte(`{"type":"` + prefix + `completed","b64_json":"first"}`)})
					}
					original = append(original, terminal)
					pipelineStream, err := captureRawProviderStream(outbound, nil).OnOutboundRawStream(ctx, streams.SliceStream(original))
					require.NoError(t, err)
					defer pipelineStream.Close()
					var pipelineEvents []*httpclient.StreamEvent
					for pipelineStream.Next() {
						pipelineEvents = append(pipelineEvents, pipelineStream.Current())
					}
					rawStream, err := applyPassThroughStream(outbound, nil).OnInboundRawStream(ctx, streams.SliceStream([]*httpclient.StreamEvent{}))
					require.NoError(t, err)
					defer rawStream.Close()
					var rawEvents []*httpclient.StreamEvent
					for rawStream.Next() {
						rawEvents = append(rawEvents, rawStream.Current())
					}
					for _, events := range [][]*httpclient.StreamEvent{pipelineEvents, rawEvents} {
						require.Len(t, events, len(original))
						for _, event := range events {
							require.False(t, IsTerminalStreamEvent(event), "foreign terminal markers cannot complete an Images request")
							require.Equal(t, streamTerminalNone, classifyStreamTerminalEvent(event))
						}
					}
					require.Nil(t, terminal.ImageStreamCompleted, "shared raw markers must remain immutable")
				})
			}
		}
	}
}
