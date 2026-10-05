package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestWriteSSEStream_ImagesCompletion(t *testing.T) {
	for _, prefix := range []string{"image_generation.", "image_edit."} {
		for _, complete := range []bool{false, true} {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
			events := []*httpclient.StreamEvent{{Type: prefix + "partial_image", Data: []byte(`{"type":"` + prefix + `partial_image","b64_json":"a","partial_image_index":0}`)}}
			if complete {
				events = append(events, &httpclient.StreamEvent{Type: prefix + "completed", Data: []byte(`{"type":"` + prefix + `completed","b64_json":"b"}`), ImageStreamCompleted: lo.ToPtr(true)})
			}
			WriteSSEStream(c, streams.SliceStream(events))
			require.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
			require.Contains(t, w.Body.String(), prefix+"partial_image")
			require.NotContains(t, w.Body.String(), "[DONE]")
			if complete {
				require.NotContains(t, w.Body.String(), "event:error")
			} else {
				require.Contains(t, w.Body.String(), "event:error")
			}
		}
	}
}

func TestWriteSSEStream_ImagesPrematureDONE(t *testing.T) {
	for _, format := range []llm.APIFormat{llm.APIFormatOpenAIImageGeneration, llm.APIFormatOpenAIImageEdit} {
		for _, n := range []int{1, 2} {
			t.Run(fmt.Sprintf("%s/n=%d", format, n), func(t *testing.T) {
				prefix := "image_generation."
				if format == llm.APIFormatOpenAIImageEdit {
					prefix = "image_edit."
				}
				progress := openai.NewImageStreamProgress(&httpclient.Request{
					APIFormat: format.String(), Body: []byte(fmt.Sprintf(`{"n":%d}`, n)),
				})
				events := []*httpclient.StreamEvent{{Type: prefix + "partial_image", Data: []byte(`{"type":"` + prefix + `partial_image","b64_json":"preview","partial_image_index":0}`)}}
				if n > 1 {
					events = append(events, &httpclient.StreamEvent{Type: prefix + "completed", Data: []byte(`{"type":"` + prefix + `completed","b64_json":"first"}`)})
				}
				events = append(events, &llm.DoneStreamEvent)
				for i, event := range events {
					events[i] = progress.Observe(event)
				}
				for _, streamErr := range []error{nil, llm.ErrStreamIncomplete} {
					w := httptest.NewRecorder()
					c, _ := gin.CreateTestContext(w)
					c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
					WriteSSEStream(c, &errorAfterStream{items: events, err: streamErr})
					require.Contains(t, w.Body.String(), "event:error", "premature raw DONE must not suppress the incomplete stream error")
				}
			})
		}
	}
}

func TestWriteSSEStream_ImagesMultiImageOutcome(t *testing.T) {
	for _, complete := range []bool{false, true} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
		first := &httpclient.StreamEvent{Type: "image_generation.completed", Data: []byte(`{"type":"image_generation.completed","b64_json":"first"}`), ImageStreamCompleted: lo.ToPtr(false)}
		events := []*httpclient.StreamEvent{first}
		var streamErr error = llm.ErrStreamIncomplete
		if complete {
			events = append(events, &httpclient.StreamEvent{Type: "image_generation.completed", Data: []byte(`{"type":"image_generation.completed","b64_json":"second"}`), ImageStreamCompleted: lo.ToPtr(true)})
			streamErr = nil
		}
		WriteSSEStream(c, &errorAfterStream{items: events, err: streamErr})
		require.Contains(t, w.Body.String(), "first")
		if complete {
			require.Contains(t, w.Body.String(), "second")
			require.NotContains(t, w.Body.String(), "event:error")
		} else {
			require.Contains(t, w.Body.String(), "event:error", "one completed image must not suppress the missing second image error")
		}
	}
}
