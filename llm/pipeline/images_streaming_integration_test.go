package pipeline_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// Withhold the final image until the consumer receives the preview. This uses
// real HTTP and the production SSE decoder, so buffering until completion would
// deadlock and fail at the bounded context deadline.
func TestPipeline_ImagesSSE_RealHTTP(t *testing.T) {
	for _, tc := range []struct {
		name      string
		edit      bool
		multipart bool
	}{
		{name: "generation"},
		{name: "edit/json-images", edit: true},
		{name: "edit/multipart", edit: true, multipart: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			release := make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			var finalSent atomic.Bool
			type capturedRequest struct{ path, contentType, accept, body string }
			captured := make(chan capturedRequest, 1)
			prefix := "image_generation."
			inbound := openai.NewImageGenerationInboundTransformer()
			if tc.edit {
				prefix = "image_edit."
				inbound = openai.NewImageEditInboundTransformer()
			}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				captured <- capturedRequest{r.URL.Path, r.Header.Get("Content-Type"), r.Header.Get("Accept"), string(body)}
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(w, "event: %spartial_image\ndata: {\"type\":\"%spartial_image\",\"b64_json\":\"cHJldmlldw==\",\"partial_image_index\":0,\"created_at\":123}\n\n", prefix, prefix)
				w.(http.Flusher).Flush()
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
				finalSent.Store(true)
				fmt.Fprintf(w, "event: %scompleted\ndata: {\"type\":\"%scompleted\",\"b64_json\":\"ZmluYWw=\",\"created_at\":123,\"size\":\"1024x1024\",\"output_format\":\"png\",\"usage\":{\"input_tokens\":10,\"output_tokens\":20,\"total_tokens\":30}}\n\n", prefix, prefix)
				w.(http.Flusher).Flush()
			}))
			defer upstream.Close()
			defer unblock()
			outbound, err := openai.NewOutboundTransformer(upstream.URL, "test-key")
			require.NoError(t, err)
			pipe := pipeline.NewFactory(httpclient.NewHttpClientWithClient(upstream.Client())).Pipeline(
				inbound, outbound, pipeline.WithEmptyResponseDetection(), pipeline.WithRetry(0, 1, 0))
			body := []byte(`{"model":"gpt-image-1","prompt":"a cat","stream":true,"partial_images":1}`)
			contentType := "application/json"
			imageBytes := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
			if tc.edit {
				body = []byte(`{"model":"gpt-image-1","prompt":"a cat","images":[{"image_url":"data:image/png;base64,iVBORw0KGgo="}],"stream":true,"partial_images":1}`)
			}
			if tc.multipart {
				var buffer bytes.Buffer
				writer := multipart.NewWriter(&buffer)
				for key, value := range map[string]string{"model": "gpt-image-1", "prompt": "a cat", "stream": "true", "partial_images": "1"} {
					require.NoError(t, writer.WriteField(key, value))
				}
				file, err := writer.CreatePart(textproto.MIMEHeader{
					"Content-Disposition": []string{`form-data; name="image"; filename="input.png"`},
					"Content-Type":        []string{"image/png"},
				})
				require.NoError(t, err)
				_, err = file.Write(imageBytes)
				require.NoError(t, err)
				require.NoError(t, writer.Close())
				body = buffer.Bytes()
				contentType = writer.FormDataContentType()
			}
			result, err := pipe.Process(ctx, &httpclient.Request{Method: http.MethodPost,
				Headers: http.Header{"Content-Type": []string{contentType}}, Body: body})
			require.NoError(t, err)
			require.True(t, result.Stream)
			defer result.EventStream.Close()
			require.True(t, result.EventStream.Next())
			preview := result.EventStream.Current()
			require.Equal(t, prefix+"partial_image", preview.Type)
			require.False(t, finalSent.Load(), "preview must arrive before final generation")
			request := <-captured
			require.Equal(t, "text/event-stream", request.accept)
			if tc.edit {
				require.Equal(t, "/v1/images/edits", request.path)
				_, params, err := mime.ParseMediaType(request.contentType)
				require.NoError(t, err)
				form, err := multipart.NewReader(strings.NewReader(request.body), params["boundary"]).ReadForm(1 << 20)
				require.NoError(t, err)
				defer form.RemoveAll()
				require.Equal(t, []string{"true"}, form.Value["stream"])
				require.Equal(t, []string{"1"}, form.Value["partial_images"])
				require.Len(t, form.File["image"], 1)
				file, err := form.File["image"][0].Open()
				require.NoError(t, err)
				defer file.Close()
				forwarded, err := io.ReadAll(file)
				require.NoError(t, err)
				require.Equal(t, imageBytes, forwarded)
			} else {
				var payload map[string]any
				require.NoError(t, json.Unmarshal([]byte(request.body), &payload))
				require.Equal(t, true, payload["stream"])
				require.Equal(t, float64(1), payload["partial_images"])
				require.Equal(t, "/v1/images/generations", request.path)
			}
			unblock()
			require.True(t, result.EventStream.Next())
			final := result.EventStream.Current()
			require.Equal(t, prefix+"completed", final.Type)
			var event openai.ImageStreamEvent
			require.NoError(t, json.Unmarshal(final.Data, &event))
			require.Equal(t, "ZmluYWw=", event.B64JSON)
			require.Equal(t, int64(30), event.Usage.TotalTokens)
			require.False(t, result.EventStream.Next())
			require.NoError(t, result.EventStream.Err())
		})
	}
}

func TestPipeline_ImagesSSE_MultipleImagesRealHTTP(t *testing.T) {
	for _, edit := range []bool{false, true} {
		for _, outcome := range []string{"completed", "incomplete", "done", "error"} {
			t.Run(fmt.Sprintf("edit=%t/%s", edit, outcome), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
				defer cancel()
				release := make(chan struct{})
				var once sync.Once
				unblock := func() { once.Do(func() { close(release) }) }
				var secondSent atomic.Bool
				prefix := "image_generation."
				inbound := openai.NewImageGenerationInboundTransformer()
				if edit {
					prefix = "image_edit."
					inbound = openai.NewImageEditInboundTransformer()
				}
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprintf(w, "event: %scompleted\ndata: {\"type\":\"%scompleted\",\"b64_json\":\"Zmlyc3Q=\",\"usage\":{\"input_tokens\":10,\"output_tokens\":20,\"total_tokens\":30}}\n\n", prefix, prefix)
					w.(http.Flusher).Flush()
					select {
					case <-release:
					case <-r.Context().Done():
						return
					}
					if outcome == "completed" {
						secondSent.Store(true)
						fmt.Fprintf(w, "event: %scompleted\ndata: {\"type\":\"%scompleted\",\"b64_json\":\"c2Vjb25k\",\"usage\":{\"input_tokens\":11,\"output_tokens\":21,\"total_tokens\":32}}\n\n", prefix, prefix)
					} else if outcome == "error" {
						fmt.Fprint(w, "event: error\ndata: {\"error\":{\"message\":\"second image failed\"}}\n\n")
					} else if outcome == "done" {
						fmt.Fprint(w, "data: [DONE]\n\n")
					}
					w.(http.Flusher).Flush()
				}))
				defer upstream.Close()
				defer unblock()
				outbound, err := openai.NewOutboundTransformer(upstream.URL, "test-key")
				require.NoError(t, err)
				pipe := pipeline.NewFactory(httpclient.NewHttpClientWithClient(upstream.Client())).Pipeline(inbound, outbound, pipeline.WithEmptyResponseDetection(), pipeline.WithRetry(0, 0, 0))
				payload := map[string]any{"model": "gpt-image-1", "prompt": "a cat", "stream": true, "n": 2}
				if edit {
					payload["images"] = []map[string]string{{"image_url": "data:image/png;base64,iVBORw0KGgo="}}
				}
				body, err := json.Marshal(payload)
				require.NoError(t, err)
				result, err := pipe.Process(ctx, &httpclient.Request{Method: http.MethodPost, Headers: http.Header{"Content-Type": []string{"application/json"}}, Body: body})
				require.NoError(t, err)
				defer result.EventStream.Close()
				require.True(t, result.EventStream.Next())
				first := result.EventStream.Current()
				require.Equal(t, prefix+"completed", first.Type)
				require.False(t, *first.ImageStreamCompleted)
				require.False(t, secondSent.Load(), "deliver the first final image before the second is generated")
				unblock()
				if outcome == "completed" {
					require.True(t, result.EventStream.Next())
					second := result.EventStream.Current()
					require.True(t, *second.ImageStreamCompleted)
					body, meta, err := inbound.AggregateStreamChunks(t.Context(), []*httpclient.StreamEvent{first, second})
					require.NoError(t, err)
					require.True(t, meta.Completed)
					require.Equal(t, int64(62), meta.Usage.TotalTokens)
					var response openai.ImagesResponse
					require.NoError(t, json.Unmarshal(body, &response))
					require.Len(t, response.Data, 2)
				}
				require.False(t, result.EventStream.Next())
				if outcome == "completed" {
					require.NoError(t, result.EventStream.Err())
				} else if outcome == "error" {
					require.ErrorContains(t, result.EventStream.Err(), "second image failed")
				} else {
					require.ErrorIs(t, result.EventStream.Err(), llm.ErrStreamIncomplete)
				}
			})
		}
	}
}
