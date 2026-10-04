package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

func imageSSEFixture(t *testing.T, prefix, kind string) *httpclient.StreamEvent {
	t.Helper()
	event := ImageStreamEvent{
		Type: prefix + kind, B64JSON: "ZmluYWw=", CreatedAt: 123,
		Background: "opaque", OutputFormat: "png", Quality: "high", Size: "1024x1024",
	}
	if kind == llm.ImageEventPartial {
		event.B64JSON = "cHJldmlldw=="
		event.PartialImageIndex = lo.ToPtr(0)
	} else {
		event.Usage = &ImagesResponseUsage{InputTokens: 10, OutputTokens: 20, TotalTokens: 30,
			Cost:               lo.ToPtr(0.12),
			InputTokensDetails: &ImagesResponseUsageInputTokensDetails{TextTokens: 4, ImageTokens: 6},
		}
	}
	data, err := json.Marshal(event)
	require.NoError(t, err)
	return &httpclient.StreamEvent{Type: event.Type, Data: data}
}

func TestImagesSSE_RequestForwarding(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://example.test/v1", "test-key")
	require.NoError(t, err)
	for _, edit := range []bool{false, true} {
		name := lo.Ternary(edit, "edit", "generation")
		t.Run(name, func(t *testing.T) {
			inbound := NewImageGenerationInboundTransformer()
			if edit {
				inbound = NewImageEditInboundTransformer()
			}
			body := `{"model":"gpt-image-1","prompt":"a cat","stream":true,"partial_images":3,"n":2}`
			if edit {
				body = `{"model":"gpt-image-1","prompt":"a cat","image":"data:image/png;base64,iVBORw0KGgo=","stream":true,"partial_images":3,"n":2}`
			}
			request, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
				Headers: http.Header{"Content-Type": []string{"application/json"}}, Body: []byte(body),
			})
			require.NoError(t, err)
			require.True(t, lo.FromPtr(request.Stream))
			require.Equal(t, int64(3), *request.Image.PartialImages)
			require.Equal(t, int64(2), *request.Image.N)
			raw, err := outbound.TransformRequest(t.Context(), request)
			require.NoError(t, err)
			require.True(t, lo.FromPtr(request.Stream), "outbound must not disable streaming")
			require.Equal(t, "text/event-stream", raw.Headers.Get("Accept"))
			if edit {
				_, params, err := mime.ParseMediaType(raw.Headers.Get("Content-Type"))
				require.NoError(t, err)
				form, err := multipart.NewReader(bytes.NewReader(raw.Body), params["boundary"]).ReadForm(1 << 20)
				require.NoError(t, err)
				defer form.RemoveAll()
				require.Equal(t, []string{"true"}, form.Value["stream"])
				require.Equal(t, []string{"3"}, form.Value["partial_images"])
				require.Equal(t, []string{"2"}, form.Value["n"])
				// Exercise the multipart inbound as well as JSON edit.
				roundtrip, err := inbound.TransformRequest(t.Context(), raw)
				require.NoError(t, err)
				require.True(t, lo.FromPtr(roundtrip.Stream))
				require.Equal(t, int64(3), *roundtrip.Image.PartialImages)
				require.Equal(t, int64(2), *roundtrip.Image.N)
			} else {
				var payload map[string]any
				require.NoError(t, json.Unmarshal(raw.Body, &payload))
				require.Equal(t, true, payload["stream"])
				require.Equal(t, float64(3), payload["partial_images"])
				require.Equal(t, float64(2), payload["n"])
			}
		})
	}
}

func TestImagesSSE_MultipleFinalImages(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://example.test/v1", "test-key")
	require.NoError(t, err)
	for _, inbound := range []*ImageInboundTransformer{NewImageGenerationInboundTransformer(), NewImageEditInboundTransformer()} {
		t.Run(inbound.APIFormat().String(), func(t *testing.T) {
			prefix := imageStreamPrefix(inbound.APIFormat())
			first := imageSSEFixture(t, prefix, llm.ImageEventCompleted)
			second := imageSSEFixture(t, prefix, llm.ImageEventCompleted)
			second.Data = bytes.ReplaceAll(second.Data, []byte("ZmluYWw="), []byte("c2Vjb25k"))
			chunks := []*httpclient.StreamEvent{first, imageSSEFixture(t, prefix, llm.ImageEventPartial), second}
			req := &httpclient.Request{RequestType: llm.RequestTypeImage.String(), APIFormat: inbound.APIFormat().String(), Body: []byte(`{"n":2}`)}
			normalized, err := outbound.TransformStream(t.Context(), req, streams.SliceStream(chunks))
			require.NoError(t, err)
			stream, err := inbound.TransformStream(t.Context(), normalized)
			require.NoError(t, err)
			defer stream.Close()
			var converted []*httpclient.StreamEvent
			for stream.Next() {
				converted = append(converted, stream.Current())
			}
			require.NoError(t, stream.Err())
			require.Len(t, converted, 3)
			require.False(t, *converted[0].ImageStreamCompleted, "first image must not finish a two-image request")
			require.True(t, *converted[2].ImageStreamCompleted)
			require.Nil(t, first.ImageStreamCompleted, "shared raw events must not be mutated")
			for _, aggregate := range []func() ([]byte, llm.ResponseMeta, error){
				func() ([]byte, llm.ResponseMeta, error) { return inbound.AggregateStreamChunks(t.Context(), converted) },
				func() ([]byte, llm.ResponseMeta, error) {
					return outbound.AggregateStreamChunks(t.Context(), req, chunks)
				},
			} {
				body, meta, err := aggregate()
				require.NoError(t, err)
				require.True(t, meta.Completed)
				require.Equal(t, int64(60), meta.Usage.TotalTokens)
				require.Equal(t, int64(12), meta.Usage.PromptTokensDetails.ImageTokens)
				require.InDelta(t, 0.24, *meta.Usage.Cost, 1e-9)
				var response ImagesResponse
				require.NoError(t, json.Unmarshal(body, &response))
				require.Len(t, response.Data, 2)
				require.Equal(t, "ZmluYWw=", response.Data[0].B64JSON)
				require.Equal(t, "c2Vjb25k", response.Data[1].B64JSON)
				require.NotContains(t, string(body), "cHJldmlldw==")
			}
			// The inbound aggregator has no request argument. The per-event
			// completion metadata must prevent a truncated batch being accepted.
			_, meta, err := inbound.AggregateStreamChunks(t.Context(), converted[:1])
			require.ErrorIs(t, err, llm.ErrStreamIncomplete)
			require.False(t, meta.Completed)
		})
	}
}

func TestImagesSSE_MultiImageFailures(t *testing.T) {
	first := imageSSEFixture(t, "image_generation.", llm.ImageEventCompleted)
	req := &httpclient.Request{APIFormat: llm.APIFormatOpenAIImageGeneration.String(), Body: []byte(`{"n":2}`)}
	for _, tc := range []struct {
		name     string
		last     *httpclient.StreamEvent
		contains string
	}{
		{name: "EOF", contains: "without a terminal"},
		{name: "DONE", last: &llm.DoneStreamEvent, contains: "without a terminal"},
		{name: "late error", last: &httpclient.StreamEvent{Type: "error", Data: []byte(`{"error":{"message":"second image failed"}}`)}, contains: "second image failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chunks := []*httpclient.StreamEvent{first}
			if tc.last != nil {
				chunks = append(chunks, tc.last)
			}
			stream := newImageOutboundStream(t.Context(), req, streams.SliceStream(chunks))
			defer stream.Close()
			require.True(t, stream.Next())
			require.False(t, stream.Current().Image.StreamCompleted)
			require.Empty(t, stream.Current().Choices)
			require.False(t, stream.Next())
			require.Error(t, stream.Err())
			if tc.last == nil || tc.last == &llm.DoneStreamEvent {
				require.ErrorIs(t, stream.Err(), llm.ErrStreamIncomplete)
			} else {
				require.ErrorContains(t, stream.Err(), tc.contains)
			}
			_, meta, err := aggregateImageStream(t.Context(), req, chunks)
			require.Error(t, err)
			require.False(t, meta.Completed)
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	stream := newImageOutboundStream(ctx, req, streams.SliceStream([]*httpclient.StreamEvent{first, first}))
	defer stream.Close()
	require.True(t, stream.Next())
	cancel()
	require.False(t, stream.Next())
	require.ErrorIs(t, stream.Err(), context.Canceled)

	// A stream-side transport failure before the second final is a failure;
	// the same trailing failure after both finals cannot undo success.
	for _, count := range []int{1, 2} {
		source := &imageTestStream{Stream: streams.SliceStream([]*httpclient.StreamEvent{first, first}[:count]), err: io.ErrUnexpectedEOF}
		stream := newImageOutboundStream(t.Context(), req, source)
		for stream.Next() {
			_ = stream.Current()
		}
		if count == 1 {
			require.ErrorIs(t, stream.Err(), io.ErrUnexpectedEOF)
		} else {
			require.NoError(t, stream.Err())
		}
		require.NoError(t, stream.Close())
	}
}

func TestImagesSSE_ProgressUsesMultipartLoggingCount(t *testing.T) {
	progress := NewImageStreamProgress(&httpclient.Request{APIFormat: llm.APIFormatOpenAIImageEdit.String(), Body: []byte("multipart bytes"), JSONBody: []byte(`{"n":"2"}`)})
	first := imageSSEFixture(t, "image_edit.", llm.ImageEventCompleted)
	require.False(t, *progress.Observe(first).ImageStreamCompleted)
	require.True(t, *progress.Observe(first).ImageStreamCompleted)
	require.Nil(t, first.ImageStreamCompleted)
	invalid := &httpclient.StreamEvent{Type: "image_edit.completed", Data: []byte(`{"b64_json":""}`)}
	require.False(t, *progress.Observe(invalid).ImageStreamCompleted)
	malformed := &httpclient.StreamEvent{Type: "image_edit.completed", Data: []byte(`{"b64_json":"final","created_at":"bad"}`)}
	require.False(t, *progress.Observe(malformed).ImageStreamCompleted)
}

func TestImagesSSE_ProgressReusesValidatedMetadata(t *testing.T) {
	req := &httpclient.Request{APIFormat: llm.APIFormatOpenAIImageGeneration.String(), Body: []byte(`{"n":2}`)}
	progress := NewImageStreamProgress(req)
	first := progress.Observe(imageSSEFixture(t, "image_generation.", llm.ImageEventCompleted))
	require.False(t, *first.ImageStreamCompleted)
	require.Same(t, first, progress.Observe(first), "observing annotated events must not count the same image twice")
	second := progress.Observe(imageSSEFixture(t, "image_generation.", llm.ImageEventCompleted))
	require.True(t, *second.ImageStreamCompleted)

	for _, last := range []*httpclient.StreamEvent{
		{Type: "image_generation.completed", Data: []byte(`{"type":"image_generation.completed","b64_json":"final","created_at":"bad"}`)},
		{Type: "image_generation.completed", Data: []byte(`{"type":"image_generation.completed","b64_json":"final","usage":{"total_tokens":"bad"}}`)},
	} {
		progress := NewImageStreamProgress(req)
		invalid := progress.Observe(last)
		require.False(t, *invalid.ImageStreamCompleted)
		valid := progress.Observe(imageSSEFixture(t, "image_generation.", llm.ImageEventCompleted))
		require.False(t, *valid.ImageStreamCompleted, "malformed finals must not advance the image count")
		stream := newImageOutboundStream(t.Context(), req, streams.SliceStream([]*httpclient.StreamEvent{invalid, valid}))
		require.False(t, stream.Next())
		require.ErrorContains(t, stream.Err(), "invalid image stream event")
		require.NoError(t, stream.Close())
		_, meta, err := aggregateImageStream(t.Context(), req, []*httpclient.StreamEvent{invalid, valid})
		require.ErrorContains(t, err, "invalid image stream event")
		require.False(t, meta.Completed)
	}
}

func TestImagesSSE_RoundtripAndAggregation(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://example.test/v1", "test-key")
	require.NoError(t, err)
	for _, inbound := range []*ImageInboundTransformer{NewImageGenerationInboundTransformer(), NewImageEditInboundTransformer()} {
		t.Run(inbound.APIFormat().String(), func(t *testing.T) {
			prefix := imageStreamPrefix(inbound.APIFormat())
			chunks := []*httpclient.StreamEvent{
				imageSSEFixture(t, prefix, llm.ImageEventPartial),
				imageSSEFixture(t, prefix, llm.ImageEventCompleted),
			}
			rawReq := &httpclient.Request{RequestType: llm.RequestTypeImage.String(), APIFormat: inbound.APIFormat().String()}
			unified, err := outbound.TransformStream(t.Context(), rawReq, streams.SliceStream(chunks))
			require.NoError(t, err)
			converted, err := inbound.TransformStream(t.Context(), unified)
			require.NoError(t, err)
			defer converted.Close()
			var received []*httpclient.StreamEvent
			for converted.Next() {
				received = append(received, converted.Current())
			}
			require.NoError(t, converted.Err())
			require.Len(t, received, 2, "Images streams must not contain a Chat [DONE]")
			require.Equal(t, prefix+llm.ImageEventPartial, received[0].Type)
			var partial ImageStreamEvent
			require.NoError(t, json.Unmarshal(received[0].Data, &partial))
			require.NotNil(t, partial.PartialImageIndex)
			require.Zero(t, *partial.PartialImageIndex)
			require.Nil(t, partial.Usage)
			var completed ImageStreamEvent
			require.NoError(t, json.Unmarshal(received[1].Data, &completed))
			require.Equal(t, prefix+llm.ImageEventCompleted, completed.Type)
			require.Nil(t, completed.PartialImageIndex)
			require.Equal(t, int64(30), completed.Usage.TotalTokens)
			require.Equal(t, 0.12, *completed.Usage.Cost)
			require.Equal(t, int64(6), completed.Usage.InputTokensDetails.ImageTokens)
			require.Equal(t, "1024x1024", completed.Size)
			for _, aggregate := range []func() ([]byte, llm.ResponseMeta, error){
				func() ([]byte, llm.ResponseMeta, error) { return inbound.AggregateStreamChunks(t.Context(), received) },
				func() ([]byte, llm.ResponseMeta, error) {
					return outbound.AggregateStreamChunks(t.Context(), rawReq, chunks)
				},
			} {
				body, meta, err := aggregate()
				require.NoError(t, err)
				require.True(t, meta.Completed)
				require.Equal(t, int64(30), meta.Usage.TotalTokens)
				var response ImagesResponse
				require.NoError(t, json.Unmarshal(body, &response))
				require.Len(t, response.Data, 1)
				require.Equal(t, "ZmluYWw=", response.Data[0].B64JSON)
				require.NotContains(t, string(body), "cHJldmlldw==")
			}
		})
	}
}

func TestImagesSSE_Failures(t *testing.T) {
	partial := imageSSEFixture(t, "image_generation.", llm.ImageEventPartial)
	cases := []struct {
		name     string
		chunks   []*httpclient.StreamEvent
		contains string
		is       error
	}{
		{"preview then EOF", []*httpclient.StreamEvent{partial}, "", llm.ErrStreamIncomplete},
		{"preview then DONE", []*httpclient.StreamEvent{partial, &llm.DoneStreamEvent}, "", llm.ErrStreamIncomplete},
		{"invalid JSON", []*httpclient.StreamEvent{{Data: []byte(`{`)}}, "invalid image stream event", nil},
		{"wrong family", []*httpclient.StreamEvent{imageSSEFixture(t, "image_edit.", llm.ImageEventCompleted)}, "unsupported image stream event", nil},
		{"empty final", []*httpclient.StreamEvent{{Data: []byte(`{"type":"image_generation.completed","b64_json":""}`)}}, "missing image data", nil},
		{"missing preview index", []*httpclient.StreamEvent{{Data: []byte(`{"type":"image_generation.partial_image","b64_json":"a"}`)}}, "partial_image_index", nil},
		{"upstream error", []*httpclient.StreamEvent{partial, {Type: "error", Data: []byte(`{"error":{"message":"quota exceeded","code":"quota"}}`)}}, "quota exceeded", nil},
		{"empty error", []*httpclient.StreamEvent{{Type: "error"}}, "stream error", nil},
		{"error type in JSON", []*httpclient.StreamEvent{{Data: []byte(`{"type":"error","message":"quota exceeded","code":"quota"}`)}}, "quota exceeded", nil},
		{"flattened SSE error", []*httpclient.StreamEvent{{Type: "error", Data: []byte(`{"message":"quota exceeded","code":"quota"}`)}}, "quota exceeded", nil},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, meta, err := aggregateImageStream(t.Context(), &httpclient.Request{APIFormat: llm.APIFormatOpenAIImageGeneration.String()}, tt.chunks)
			require.Error(t, err)
			require.False(t, meta.Completed)
			if tt.is != nil {
				require.ErrorIs(t, err, tt.is)
			} else {
				require.ErrorContains(t, err, tt.contains)
			}
		})
	}
}

// Tracks upstream consumption to ensure previews are not buffered to completion
// and a transport error after a completed event cannot undo semantic success.
type imageTestStream struct {
	streams.Stream[*httpclient.StreamEvent]
	reads  int
	closed bool
	err    error
}

func (s *imageTestStream) Next() bool   { s.reads++; return s.Stream.Next() }
func (s *imageTestStream) Err() error   { return s.err }
func (s *imageTestStream) Close() error { s.closed = true; return s.Stream.Close() }

func TestImagesSSE_Lifecycle(t *testing.T) {
	req := &httpclient.Request{APIFormat: llm.APIFormatOpenAIImageGeneration.String()}
	source := &imageTestStream{Stream: streams.SliceStream([]*httpclient.StreamEvent{
		imageSSEFixture(t, "image_generation.", llm.ImageEventPartial),
		imageSSEFixture(t, "image_generation.", llm.ImageEventCompleted),
	}), err: io.ErrUnexpectedEOF}
	stream := newImageOutboundStream(t.Context(), req, source)
	require.True(t, stream.Next())
	require.Equal(t, 1, source.reads)
	require.Same(t, stream.Current(), stream.Current(), "Current must not advance a stateful source")
	require.True(t, stream.Next())
	require.Equal(t, llm.ImageEventCompleted, stream.Current().Image.EventType)
	require.True(t, stream.Next())
	require.Same(t, llm.DoneResponse, stream.Current())
	require.False(t, stream.Next())
	require.NoError(t, stream.Err())
	require.Equal(t, 2, source.reads)
	require.NoError(t, stream.Close())
	require.True(t, source.closed)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	stream = newImageOutboundStream(ctx, req, source)
	require.False(t, stream.Next())
	require.ErrorIs(t, stream.Err(), context.Canceled)

	source = &imageTestStream{Stream: streams.SliceStream([]*httpclient.StreamEvent{}), err: io.ErrUnexpectedEOF}
	stream = newImageOutboundStream(t.Context(), req, source)
	require.False(t, stream.Next())
	require.ErrorIs(t, stream.Err(), io.ErrUnexpectedEOF)
}

func TestImagesSSE_UnsupportedOptions(t *testing.T) {
	for _, body := range []string{
		`{"prompt":"a cat","stream":true}`,
		`{"model":"dall-e-2","prompt":"a cat","stream":true}`,
		`{"model":"dall-e-3","prompt":"a cat","stream":true}`,
		`{"model":"gpt-image-1","prompt":"a cat","stream":true,"n":0}`,
		`{"model":"gpt-image-1","prompt":"a cat","stream":true,"n":11}`,
		`{"model":"gpt-image-1","prompt":"a cat","stream":true,"partial_images":4}`,
		`{"model":"gpt-image-1","prompt":"a cat","stream":true,"partial_images":-1}`,
	} {
		_, err := NewImageGenerationInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{
			Headers: http.Header{"Content-Type": []string{"application/json"}}, Body: []byte(body),
		})
		require.True(t, errors.Is(err, transformer.ErrInvalidRequest), body)
	}
	// Existing non-streaming multi-image requests continue to work.
	req, err := NewImageGenerationInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    []byte(`{"model":"gpt-image-1","prompt":"a cat","n":2}`),
	})
	require.NoError(t, err)
	require.False(t, lo.FromPtr(req.Stream))
	_, err = NewImageVariationInboundTransformer().TransformStream(t.Context(), streams.SliceStream([]*llm.Response{}))
	require.ErrorContains(t, err, "variations do not support streaming")
	// An unrelated provider's Chat stream is not silently advertised as Images SSE.
	stream, err := NewImageGenerationInboundTransformer().TransformStream(t.Context(), streams.SliceStream([]*llm.Response{{Object: "chat.completion.chunk"}}))
	require.NoError(t, err)
	require.False(t, stream.Next())
	require.ErrorContains(t, stream.Err(), "Images SSE event")

	// Allow an SSE event name when compatible providers omit JSON type.
	raw := imageSSEFixture(t, "image_generation.", llm.ImageEventCompleted)
	raw.Data = []byte(strings.ReplaceAll(string(raw.Data), `"type":"image_generation.completed",`, ""))
	_, err = decodeImageStreamEvent(&httpclient.Request{APIFormat: llm.APIFormatOpenAIImageGeneration.String()}, raw)
	require.NoError(t, err)
}

func TestImagesSSE_MultipartIntegerValidation(t *testing.T) {
	for _, field := range []string{"n", "partial_images"} {
		for _, value := range []string{"abc", "1.5", "1e0", "9223372036854775808", "-9223372036854775809"} {
			t.Run(field+"="+value, func(t *testing.T) {
				_, err := NewImageEditInboundTransformer().TransformRequest(t.Context(), imageSSEMultipartRequest(t, map[string]string{
					"stream": "true", field: value,
				}))
				require.ErrorIs(t, err, transformer.ErrInvalidRequest)
				require.ErrorContains(t, err, field)
			})
		}
	}
	for _, stream := range []string{"", "false", "0", "1", "yes"} {
		t.Run("legacy-nonstream/"+stream, func(t *testing.T) {
			req, err := NewImageEditInboundTransformer().TransformRequest(t.Context(), imageSSEMultipartRequest(t, map[string]string{
				"stream": stream, "n": "abc", "partial_images": "1.5",
			}))
			require.NoError(t, err)
			require.False(t, lo.FromPtr(req.Stream))
			require.Nil(t, req.Image.N)
			require.Nil(t, req.Image.PartialImages)
		})
	}
	for _, value := range []string{"", "  "} {
		req, err := NewImageEditInboundTransformer().TransformRequest(t.Context(), imageSSEMultipartRequest(t, map[string]string{
			"stream": "true", "n": value, "partial_images": value,
		}))
		require.NoError(t, err)
		require.Nil(t, req.Image.N)
		require.Nil(t, req.Image.PartialImages)
	}
	req, err := NewImageEditInboundTransformer().TransformRequest(t.Context(), imageSSEMultipartRequest(t, map[string]string{
		"stream": "true", "n": " 2 ", "partial_images": "0",
	}))
	require.NoError(t, err)
	require.Equal(t, int64(2), *req.Image.N)
	require.Equal(t, int64(0), *req.Image.PartialImages)
}

func imageSSEMultipartRequest(t *testing.T, fields map[string]string) *httpclient.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields = lo.Assign(map[string]string{"model": "gpt-image-1", "prompt": "a cat"}, fields)
	for key, value := range fields {
		require.NoError(t, writer.WriteField(key, value))
	}
	file, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": []string{`form-data; name="image"; filename="input.png"`},
		"Content-Type":        []string{"image/png"},
	})
	require.NoError(t, err)
	_, err = file.Write([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a})
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return &httpclient.Request{Method: http.MethodPost,
		Headers: http.Header{"Content-Type": []string{writer.FormDataContentType()}}, Body: body.Bytes()}
}
