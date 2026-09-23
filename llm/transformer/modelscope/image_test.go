package modelscope

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xurl"
)

// rotatingKeyProvider hands out a different key on every call, mimicking a
// channel that balances across several keys. The task round trip must survive it
// by reusing the submission key instead of re-resolving per call.
type rotatingKeyProvider struct {
	keys []string
	next int
}

func (p *rotatingKeyProvider) Get(context.Context) string {
	key := p.keys[p.next%len(p.keys)]
	p.next++
	return key
}

func newImageTransformer(t *testing.T, baseURL string) *OutboundTransformer {
	t.Helper()

	built, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	outbound, ok := built.(*OutboundTransformer)
	require.True(t, ok)
	outbound.pollInterval = time.Millisecond

	return outbound
}

func TestBuildImageRequest_GenerationOmitsImageURL(t *testing.T) {
	t.Parallel()

	transformer := newImageTransformer(t, "https://example.com/v1")

	req, err := transformer.TransformRequest(context.Background(), &llm.Request{
		Model:       "Qwen/Qwen-Image-2.1",
		RequestType: llm.RequestTypeImage,
		APIFormat:   llm.APIFormatOpenAIImageGeneration,
		Image: &llm.ImageRequest{
			Prompt: "a fox",
			Size:   "auto",
		},
	})
	require.NoError(t, err)

	require.Equal(t, http.MethodPost, req.Method)
	require.Equal(t, "https://example.com/v1/images/generations", req.URL)
	require.Empty(t, req.Headers.Get("X-ModelScope-Async-Mode"),
		"the submission must not depend on the async-mode header; the endpoint returns a task_id either way")
	require.Equal(t, llm.APIFormatModelScopeImage.String(), req.APIFormat)
	require.Equal(t, llm.RequestTypeImage.String(), req.RequestType)

	var body map[string]any
	require.NoError(t, json.Unmarshal(req.Body, &body))
	require.Equal(t, "Qwen/Qwen-Image-2.1", body["model"])
	require.Equal(t, "a fox", body["prompt"])
	require.Equal(t, defaultImageSize, body["size"],
		"ModelScope has no size=auto, so the OpenAI auto size maps onto the gateway default")
	require.NotContains(t, body, "image_url")
	require.Equal(t, "Qwen/Qwen-Image-2.1", req.TransformerMetadata["model"])
}

func TestBuildImageRequest_SizeMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		size string
		want string
	}{
		{name: "auto maps to the gateway default", size: "auto", want: defaultImageSize},
		{name: "empty maps to the gateway default", size: "", want: defaultImageSize},
		{name: "auto is case insensitive", size: "AUTO", want: defaultImageSize},
		{name: "explicit size is passed through", size: "1024x1024", want: "1024x1024"},
		{name: "explicit size is trimmed", size: " 1664x1664 ", want: "1664x1664"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			transformer := newImageTransformer(t, "https://example.com/v1")

			req, err := transformer.TransformRequest(context.Background(), &llm.Request{
				Model:       "Qwen/Qwen-Image-2.1",
				RequestType: llm.RequestTypeImage,
				APIFormat:   llm.APIFormatOpenAIImageGeneration,
				Image: &llm.ImageRequest{
					Prompt: "a fox",
					Size:   tt.size,
				},
			})
			require.NoError(t, err)

			var body map[string]any
			require.NoError(t, json.Unmarshal(req.Body, &body))
			require.Equal(t, tt.want, body["size"])
		})
	}
}

func TestBuildImageRequest_EditSendsImageURL(t *testing.T) {
	t.Parallel()

	transformer := newImageTransformer(t, "https://example.com/v1")

	pixel := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}

	req, err := transformer.TransformRequest(context.Background(), &llm.Request{
		Model:       "Qwen/Qwen-Image-2.1",
		RequestType: llm.RequestTypeImage,
		APIFormat:   llm.APIFormatOpenAIImageEdit,
		Image: &llm.ImageRequest{
			Prompt: "repaint the sky",
			Images: [][]byte{pixel},
		},
	})
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(req.Body, &body))

	imageURL, ok := body["image_url"].(string)
	require.True(t, ok, "a single reference image is sent as a bare string")
	require.True(t, xurl.IsDataURL(imageURL))
	require.Equal(t, base64.StdEncoding.EncodeToString(pixel), xurl.ExtractBase64FromDataURL(imageURL))
}

func TestBuildImageRequest_MultipleImagesSentAsArray(t *testing.T) {
	t.Parallel()

	transformer := newImageTransformer(t, "https://example.com/v1")

	pixel := []byte{0x89, 0x50, 0x4e, 0x47}

	req, err := transformer.TransformRequest(context.Background(), &llm.Request{
		Model:       "Qwen/Qwen-Image-2.1",
		RequestType: llm.RequestTypeImage,
		APIFormat:   llm.APIFormatOpenAIImageEdit,
		Image: &llm.ImageRequest{
			Prompt: "merge",
			Images: [][]byte{pixel, pixel},
		},
	})
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(req.Body, &body))

	images, ok := body["image_url"].([]any)
	require.True(t, ok, "multiple reference images are sent as an array")
	require.Len(t, images, 2)
}

func TestBuildImageRequest_RejectsMaskAndUnsupportedFormat(t *testing.T) {
	t.Parallel()

	transformer := newImageTransformer(t, "https://example.com/v1")

	_, err := transformer.TransformRequest(context.Background(), &llm.Request{
		Model:       "Qwen/Qwen-Image-2.1",
		RequestType: llm.RequestTypeImage,
		APIFormat:   llm.APIFormatOpenAIImageEdit,
		Image: &llm.ImageRequest{
			Prompt: "repaint",
			Mask:   []byte{0x01},
		},
	})
	require.ErrorContains(t, err, "do not support masks")

	_, err = transformer.TransformRequest(context.Background(), &llm.Request{
		Model:       "Qwen/Qwen-Image-2.1",
		RequestType: llm.RequestTypeImage,
		APIFormat:   llm.APIFormatOpenAIImageVariation,
		Image:       &llm.ImageRequest{Prompt: "x"},
	})
	require.ErrorContains(t, err, "does not support image api format")
}

func TestBuildImageRequest_RejectsEmptyPrompt(t *testing.T) {
	t.Parallel()

	transformer := newImageTransformer(t, "https://example.com/v1")

	_, err := transformer.TransformRequest(context.Background(), &llm.Request{
		Model:       "Qwen/Qwen-Image-2.1",
		RequestType: llm.RequestTypeImage,
		APIFormat:   llm.APIFormatOpenAIImageGeneration,
		Image:       &llm.ImageRequest{Prompt: "   "},
	})
	require.ErrorContains(t, err, "prompt is required")
}

// TestTransformImageResponse_PollsTaskAndReturnsBase64 covers the async round
// trip: submitting returns a task id, polling succeeds, and the output URL is
// downloaded and exposed as b64_json for clients that only read b64_json.
func TestTransformImageResponse_PollsTaskAndReturnsBase64(t *testing.T) {
	t.Parallel()

	pngBytes := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x01}

	var polls int

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/img.png", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(pngBytes)
	})

	mux.HandleFunc("/v1/tasks/task-1", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "image_generation", r.Header.Get("X-ModelScope-Task-Type"))
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		polls++

		status := "RUNNING"
		if polls > 1 {
			status = "SUCCEED"
		}

		payload := map[string]any{
			"task_id":       "task-1",
			"task_status":   status,
			"output_images": []string{server.URL + "/img.png"},
		}
		require.NoError(t, json.NewEncoder(w).Encode(payload))
	})

	transformer := newImageTransformer(t, server.URL+"/v1")

	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"task_id":"task-1","task_status":"PENDING"}`),
		Request: &httpclient.Request{
			APIFormat:           llm.APIFormatModelScopeImage.String(),
			TransformerMetadata: map[string]any{"model": "Qwen/Qwen-Image-2.1"},
		},
	}

	resp, err := transformer.TransformResponse(context.Background(), httpResp)
	require.NoError(t, err)
	require.NotNil(t, resp.Image)
	require.Len(t, resp.Image.Data, 1)
	require.Equal(t, base64.StdEncoding.EncodeToString(pngBytes), resp.Image.Data[0].B64JSON)
	require.Equal(t, server.URL+"/img.png", resp.Image.Data[0].URL)
	require.Equal(t, llm.RequestTypeImage, resp.RequestType)
	require.GreaterOrEqual(t, polls, 2, "the task must be polled until it succeeds")
}

// TestTransformImageResponse_HonoursCallerDeadline pins the inner polling budget
// to the caller's deadline. Without it the transformer would keep polling after
// the gateway has already given up on the request.
func TestTransformImageResponse_HonoursCallerDeadline(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/v1/tasks/task-3", func(w http.ResponseWriter, _ *http.Request) {
		// Never reaches a terminal status, so only the deadline can stop the loop.
		_, _ = w.Write([]byte(`{"task_id":"task-3","task_status":"RUNNING"}`))
	})

	transformer := newImageTransformer(t, server.URL+"/v1")
	transformer.taskTimeout = time.Hour

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"task_id":"task-3","task_status":"PENDING"}`),
		Request:    &httpclient.Request{APIFormat: llm.APIFormatModelScopeImage.String()},
	}

	start := time.Now()
	_, err := transformer.TransformResponse(ctx, httpResp)
	elapsed := time.Since(start)

	require.Error(t, err)
	require.Less(t, elapsed, 5*time.Second,
		"polling must stop at the caller deadline, not at the one-hour fallback")
}

func TestTransformImageResponse_TaskFailureIsReported(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/v1/tasks/task-2", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"task_id":"task-2","task_status":"FAILED","message":"prompt rejected"}`))
	})

	transformer := newImageTransformer(t, server.URL+"/v1")

	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"task_id":"task-2","task_status":"PENDING"}`),
		Request:    &httpclient.Request{APIFormat: llm.APIFormatModelScopeImage.String()},
	}

	_, err := transformer.TransformResponse(context.Background(), httpResp)
	require.ErrorContains(t, err, "prompt rejected")
}

func TestTransformImageResponse_DelegatesNonImageFormats(t *testing.T) {
	t.Parallel()

	transformer := newImageTransformer(t, "https://example.com/v1")

	// A chat response must keep flowing through the embedded OpenAI transformer
	// instead of the ModelScope image task handling.
	_, err := transformer.TransformResponse(context.Background(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"id":"chatcmpl-1","object":"chat.completion","created":1,"model":"m","choices":[]}`),
		Request:    &httpclient.Request{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
	})
	require.NoError(t, err)
}

// TestImageTask_ReusesSubmissionKey pins the multi-key fix: the key that
// creates the task must authenticate every later call for that task, even when
// the provider would hand out a different key each time.
func TestImageTask_ReusesSubmissionKey(t *testing.T) {
	t.Parallel()

	var pollKeys []string

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/v1/tasks/task-key", func(w http.ResponseWriter, r *http.Request) {
		pollKeys = append(pollKeys, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		_, _ = w.Write([]byte(`{"task_id":"task-key","task_status":"SUCCEED","output_images":[]}`))
	})

	transformer := newImageTransformer(t, server.URL+"/v1")
	transformer.apiKeyProvider = &rotatingKeyProvider{keys: []string{"key-a", "key-b", "key-c"}}

	built, err := transformer.TransformRequest(context.Background(), &llm.Request{
		Model:       "Qwen/Qwen-Image-2.1",
		RequestType: llm.RequestTypeImage,
		APIFormat:   llm.APIFormatOpenAIImageGeneration,
		Image:       &llm.ImageRequest{Prompt: "a fox"},
	})
	require.NoError(t, err)
	require.Equal(t, "key-a", built.Auth.APIKey, "the first resolved key submits the task")

	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"task_id":"task-key","task_status":"PENDING"}`),
		Request:    built,
	}

	_, err = transformer.TransformResponse(context.Background(), httpResp)
	// The stub returns no output images, so a descriptive error is expected; the
	// assertion that matters is which key was used for the poll.
	require.Error(t, err)
	require.Equal(t, []string{"key-a"}, pollKeys,
		"polling must reuse the submitting key, not resolve a new one")
}

// TestTransformImageResponse_BoundsEachPollRequest pins the second review point:
// an individual task query must be cut off by the same deadline the loop uses,
// including when that deadline comes from the transformer's own budget rather
// than from the caller.
func TestTransformImageResponse_BoundsEachPollRequest(t *testing.T) {
	t.Parallel()

	released := make(chan struct{})

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/v1/tasks/task-slow", func(w http.ResponseWriter, r *http.Request) {
		// Block past the transformer's own task budget.
		select {
		case <-r.Context().Done():
		case <-released:
		}
	})
	defer close(released)

	transformer := newImageTransformer(t, server.URL+"/v1")
	transformer.taskTimeout = 80 * time.Millisecond
	transformer.pollInterval = time.Millisecond

	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"task_id":"task-slow","task_status":"PENDING"}`),
		Request:    &httpclient.Request{APIFormat: llm.APIFormatModelScopeImage.String()},
	}

	start := time.Now()
	_, err := transformer.TransformResponse(context.Background(), httpResp)
	elapsed := time.Since(start)

	require.Error(t, err)
	require.Less(t, elapsed, 5*time.Second,
		"a hung poll must be bounded by the poll deadline, not run until the server responds")
}
