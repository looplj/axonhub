package zenmux

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

func TestZenMuxNativeVideoCreatePoll(t *testing.T) {
	// Given: a ZenMux transformer and a request using every supported native field.
	var captured []capturedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = append(captured, capturedRequest{
			method: r.Method,
			path:   r.URL.Path,
			auth:   r.Header.Get("Authorization"),
		})
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(server.Close)

	videoRequest := nativeVideoRequest("5")
	outbound, err := NewOutboundTransformer(server.URL, "zenmux-test-key")
	require.NoError(t, err)

	// When: the unified video request is transformed for ZenMux.
	createRequest, err := outbound.TransformRequest(context.Background(), videoRequest)

	// Then: the native create request has the documented endpoint, auth, type, and fields.
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, createRequest.Method)
	require.Equal(t, server.URL+"/videos", createRequest.URL)
	require.Equal(t, llm.RequestTypeVideo.String(), createRequest.RequestType)
	require.Equal(t, llm.APIFormatZenmuxVideo.String(), createRequest.APIFormat)
	require.Equal(t, &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "zenmux-test-key"}, createRequest.Auth)
	sendProviderRequest(t, server.Client(), createRequest)
	require.Equal(t, capturedRequest{method: http.MethodPost, path: "/videos", auth: "Bearer zenmux-test-key"}, captured[0])

	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(createRequest.Body, &body))
	require.Equal(t, []string{
		"callback_url", "camera_fixed", "content", "draft", "duration", "frames", "generate_audio", "model", "ratio", "resolution", "return_last_frame", "seed", "tools", "watermark",
	}, sortedKeys(body))
	var duration int64
	require.NoError(t, json.Unmarshal(body["duration"], &duration))
	require.Equal(t, int64(5), duration)
	var content []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body["content"], &content))
	require.Len(t, content, 4)
	require.Contains(t, content[0], "text")
	require.Contains(t, content[1], "image_url")
	require.Contains(t, content[2], "video_url")
	require.Contains(t, content[3], "audio_url")

	createResponse := &httpclient.Response{
		Body:    []byte(`{"id":"task-123","status":"queued","model":"provider/video"}`),
		Request: createRequest,
	}

	// When: the provider's create response is transformed.
	created, err := outbound.TransformResponse(context.Background(), createResponse)

	// Then: the queued task is normalized for the existing video lifecycle.
	require.NoError(t, err)
	require.Equal(t, "task-123", created.Video.ID)
	require.Equal(t, "queued", created.Video.Status)
	require.Equal(t, llm.RequestTypeVideo, created.RequestType)

	videoTasks := outbound.(transformer.VideoTaskOutbound)
	pollRequest, err := videoTasks.BuildGetVideoTaskRequest(context.Background(), "task-123")
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, pollRequest.Method)
	require.Equal(t, server.URL+"/videos/task-123", pollRequest.URL)
	require.Equal(t, httpclient.AuthTypeBearer, pollRequest.Auth.Type)
	require.Equal(t, llm.APIFormatZenmuxVideo.String(), pollRequest.APIFormat)
	sendProviderRequest(t, server.Client(), pollRequest)
	require.Equal(t, capturedRequest{method: http.MethodGet, path: "/videos/task-123", auth: "Bearer zenmux-test-key"}, captured[1])

	// When: queued, running, and succeeded poll responses are parsed.
	queued, err := videoTasks.ParseGetVideoTaskResponse(context.Background(), &httpclient.Response{
		Body: []byte(`{"id":"task-123","status":"queued","model":"provider/video"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "queued", queued.Video.Status)
	running, err := videoTasks.ParseGetVideoTaskResponse(context.Background(), &httpclient.Response{
		Body: []byte(`{"id":"task-123","status":"running","model":"provider/video"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "running", running.Video.Status)
	succeeded, err := videoTasks.ParseGetVideoTaskResponse(context.Background(), &httpclient.Response{
		Body: []byte(`{"id":"task-123","status":"succeeded","model":"provider/video","content":{"video_url":"https://example.invalid/video.mp4","last_frame_url":"https://example.invalid/last.jpg","provider_metadata":{"request_id":"native-1"}}}`),
	})

	// Then: the nested native video URL is exposed in the unified response.
	require.NoError(t, err)
	require.Equal(t, "succeeded", succeeded.Video.Status)
	require.Equal(t, "https://example.invalid/video.mp4", succeeded.Video.VideoURL)
	require.Equal(t, "https://example.invalid/last.jpg", succeeded.Video.LastFrameURL)
	require.JSONEq(t, `{"video_url":"https://example.invalid/video.mp4","last_frame_url":"https://example.invalid/last.jpg","provider_metadata":{"request_id":"native-1"}}`, string(succeeded.Video.Content))
}

func TestZenMuxNativeVideoValidation(t *testing.T) {
	// Given: a ZenMux transformer configured with a test endpoint.
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	outbound, err := NewOutboundTransformer(server.URL, "zenmux-test-key")
	require.NoError(t, err)

	tests := map[string]*llm.Request{
		"empty model": func() *llm.Request {
			req := nativeVideoRequest("5")
			req.Model = ""
			return req
		}(),
		"empty content": func() *llm.Request {
			req := nativeVideoRequest("5")
			req.Video.Content = nil
			return req
		}(),
		"empty duration":       nativeVideoRequest(""),
		"zero duration":        nativeVideoRequest("0"),
		"fractional duration":  nativeVideoRequest("1.5"),
		"negative duration":    nativeVideoRequest("-1"),
		"non numeric duration": nativeVideoRequest("five"),
		"unsupported service tier": func() *llm.Request {
			req := nativeVideoRequest("5")
			req.Video.ServiceTier = "default"
			return req
		}(),
		"unsupported execution expiry": func() *llm.Request {
			req := nativeVideoRequest("5")
			req.Video.ExecutionExpiresAfter = newInt64(60)
			return req
		}(),
		"text with image field": invalidContentRequest(llm.VideoContent{
			Type: "text", Text: "prompt", ImageURL: &llm.VideoImageURL{URL: "https://example.invalid/image.png"},
		}),
		"text with empty text": invalidContentRequest(llm.VideoContent{
			Type: "text",
		}),
		"image with text field": invalidContentRequest(llm.VideoContent{
			Type: "image_url", Text: "conflicting", ImageURL: &llm.VideoImageURL{URL: "https://example.invalid/image.png"},
		}),
		"image with empty URL": invalidContentRequest(llm.VideoContent{
			Type: "image_url", ImageURL: &llm.VideoImageURL{},
		}),
		"image with invalid role": invalidContentRequest(llm.VideoContent{
			Type: "image_url", Role: "reference_video", ImageURL: &llm.VideoImageURL{URL: "https://example.invalid/image.png"},
		}),
		"video with missing role": invalidContentRequest(llm.VideoContent{
			Type: "video_url", VideoURL: &llm.VideoURL{URL: "https://example.invalid/video.mp4"},
		}),
		"video with empty URL": invalidContentRequest(llm.VideoContent{
			Type: "video_url", Role: "reference_video", VideoURL: &llm.VideoURL{},
		}),
		"video with invalid role": invalidContentRequest(llm.VideoContent{
			Type: "video_url", Role: "reference_audio", VideoURL: &llm.VideoURL{URL: "https://example.invalid/video.mp4"},
		}),
		"video with mime type": invalidContentRequest(llm.VideoContent{
			Type: "video_url", Role: "reference_video", VideoURL: &llm.VideoURL{URL: "https://example.invalid/video.mp4", MIMEType: "video/mp4"},
		}),
		"audio with missing role": invalidContentRequest(llm.VideoContent{
			Type: "audio_url", AudioURL: &llm.AudioURL{URL: "https://example.invalid/audio.mp3"},
		}),
		"audio with empty URL": invalidContentRequest(llm.VideoContent{
			Type: "audio_url", Role: "reference_audio", AudioURL: &llm.AudioURL{},
		}),
		"audio with invalid role": invalidContentRequest(llm.VideoContent{
			Type: "audio_url", Role: "reference_video", AudioURL: &llm.AudioURL{URL: "https://example.invalid/audio.mp3"},
		}),
		"audio with image field": invalidContentRequest(llm.VideoContent{
			Type: "audio_url", Role: "reference_audio", AudioURL: &llm.AudioURL{URL: "https://example.invalid/audio.mp3"}, ImageURL: &llm.VideoImageURL{URL: "https://example.invalid/image.png"},
		}),
	}

	for name, request := range tests {
		t.Run(name, func(t *testing.T) {
			// When: an invalid native video request is transformed.
			_, err := outbound.TransformRequest(context.Background(), request)

			// Then: the transformer reports a typed invalid-request error.
			require.Error(t, err)
			require.ErrorIs(t, err, transformer.ErrInvalidRequest)
		})
	}

	videoTasks := outbound.(transformer.VideoTaskOutbound)
	for name, response := range map[string][]byte{
		"nil response":       nil,
		"malformed response": []byte(`{"id":`),
		"missing task id":    []byte(`{"status":"queued"}`),
		"unknown status":     []byte(`{"id":"task-123","status":"cancelled"}`),
	} {
		t.Run(name, func(t *testing.T) {
			// When: a malformed provider poll response is parsed.
			var httpResponse *httpclient.Response
			if response != nil {
				httpResponse = &httpclient.Response{Body: response}
			}
			_, err := videoTasks.ParseGetVideoTaskResponse(context.Background(), httpResponse)

			// Then: the untrusted response is rejected as invalid.
			require.Error(t, err)
			require.ErrorIs(t, err, transformer.ErrInvalidResponse)
		})
	}

	// Given: the provider reports a failed task.
	failed, err := videoTasks.ParseGetVideoTaskResponse(context.Background(), &httpclient.Response{
		Body: []byte(`{"id":"task-123","status":"failed","model":"provider/video","error":{"code":"generation_failed","message":"provider rejected the model"}}`),
	})

	// Then: the failure remains a structured video response.
	require.NoError(t, err)
	require.Equal(t, "failed", failed.Video.Status)
	require.Equal(t, "generation_failed", failed.Video.Error.Code)
	require.Equal(t, "provider rejected the model", failed.Video.Error.Message)

	// When: a native delete request is requested.
	deleteRequest, err := videoTasks.BuildDeleteVideoTaskRequest(context.Background(), "task-123")

	// Then: deletion is explicitly unsupported and cannot issue an upstream request.
	require.Error(t, err)
	require.ErrorIs(t, err, ErrVideoTaskDeleteUnsupported)
	require.Nil(t, deleteRequest)
}

func TestZenMuxNativeVideoPreservesOpaqueExtraBody(t *testing.T) {
	// Given: a native request with an additional provider field.
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	outbound, err := NewOutboundTransformer(server.URL, "zenmux-test-key")
	require.NoError(t, err)
	request := nativeVideoRequest("5")
	request.ExtraBody = json.RawMessage(`{"camera_motion":"fixed"}`)

	// When: the request is transformed.
	providerRequest, err := outbound.TransformRequest(context.Background(), request)

	// Then: the opaque native field is preserved while typed fields remain authoritative.
	require.NoError(t, err)
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(providerRequest.Body, &body))
	require.Equal(t, json.RawMessage(`"fixed"`), body["camera_motion"])
	require.Equal(t, json.RawMessage(`"https://example.invalid/callback"`), body["callback_url"])
}

func TestZenMuxNativeVideoRejectsUnsupportedExtraBodyFields(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	outbound, err := NewOutboundTransformer(server.URL, "zenmux-test-key")
	require.NoError(t, err)

	for name, extraBody := range map[string]string{
		"service_tier":            `{"service_tier":"default"}`,
		"execution_expires_after": `{"execution_expires_after":60}`,
	} {
		t.Run(name, func(t *testing.T) {
			// Given: a native request with a ZenMux-unsupported field in extra_body.
			request := nativeVideoRequest("5")
			request.ExtraBody = json.RawMessage(extraBody)

			// When: the request is transformed.
			_, err := outbound.TransformRequest(context.Background(), request)

			// Then: the unsupported field is rejected instead of forwarded upstream.
			require.ErrorIs(t, err, transformer.ErrInvalidRequest)
		})
	}
}

func TestZenMuxNativeVideoUsesCustomEndpointPath(t *testing.T) {
	// Given: a native transformer configured with a custom endpoint path.
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	outbound, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        server.URL,
		EndpointPath:   "/custom/videos",
		APIKeyProvider: auth.NewStaticKeyProvider("zenmux-test-key"),
	})
	require.NoError(t, err)

	// When: create and poll requests are built.
	createRequest, err := outbound.TransformRequest(context.Background(), nativeVideoRequest("5"))
	require.NoError(t, err)
	videoTasks := outbound.(transformer.VideoTaskOutbound)
	pollRequest, err := videoTasks.BuildGetVideoTaskRequest(context.Background(), "task-123")

	// Then: both operations use the configured path.
	require.NoError(t, err)
	require.Equal(t, server.URL+"/custom/videos", createRequest.URL)
	require.Equal(t, server.URL+"/custom/videos/task-123", pollRequest.URL)
}

func TestZenMuxDelegatesNonVideoRequestsToOpenAI(t *testing.T) {
	// Given: a ZenMux transformer and a normal chat request.
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	outbound, err := NewOutboundTransformer(server.URL, "zenmux-test-key")
	require.NoError(t, err)
	request := &llm.Request{
		Model: "provider/chat",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: newString("hello")},
		}},
		RequestType: llm.RequestTypeChat,
	}

	// When: the non-video request is transformed.
	providerRequest, err := outbound.TransformRequest(context.Background(), request)

	// Then: existing OpenAI chat behavior supplies the request shape and endpoint.
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, providerRequest.Method)
	require.Equal(t, server.URL+"/chat/completions", providerRequest.URL)
	require.Equal(t, llm.APIFormatOpenAIChatCompletion.String(), providerRequest.APIFormat)
}
