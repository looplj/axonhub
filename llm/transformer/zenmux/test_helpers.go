package zenmux

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func nativeVideoRequest(duration string) *llm.Request {
	return &llm.Request{
		Model:       "provider/video",
		RequestType: llm.RequestTypeVideo,
		Video: &llm.VideoRequest{
			Content: []llm.VideoContent{
				{Type: "text", Text: "a cat reading"},
				{Type: "image_url", Role: "first_frame", ImageURL: &llm.VideoImageURL{URL: "https://example.invalid/first.png"}},
				{Type: "video_url", Role: "reference_video", VideoURL: &llm.VideoURL{URL: "https://example.invalid/reference.mp4"}},
				{Type: "audio_url", Role: "reference_audio", AudioURL: &llm.AudioURL{URL: "https://example.invalid/reference.mp3"}},
			},
			Resolution:      "720p",
			Ratio:           "16:9",
			Duration:        &duration,
			Seed:            newInt64(-1),
			GenerateAudio:   newBool(true),
			Frames:          newInt64(120),
			CameraFixed:     newBool(false),
			Watermark:       newBool(true),
			Draft:           newBool(false),
			CallbackURL:     "https://example.invalid/callback",
			ReturnLastFrame: newBool(true),
			Tools:           []json.RawMessage{json.RawMessage(`{"type":"web_search"}`)},
		},
	}
}

func invalidContentRequest(content llm.VideoContent) *llm.Request {
	req := nativeVideoRequest("5")
	req.Video.Content = []llm.VideoContent{content}
	return req
}

type capturedRequest struct {
	method string
	path   string
	auth   string
}

func sendProviderRequest(t *testing.T, client *http.Client, providerRequest *httpclient.Request) {
	t.Helper()
	request, err := http.NewRequest(providerRequest.Method, providerRequest.URL, bytes.NewReader(providerRequest.Body))
	require.NoError(t, err)
	for key, values := range providerRequest.Headers {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}
	if providerRequest.Auth != nil && providerRequest.Auth.Type == httpclient.AuthTypeBearer {
		request.Header.Set("Authorization", "Bearer "+providerRequest.Auth.APIKey)
	}
	response, err := client.Do(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
}

func newString(value string) *string { return &value }

func newInt64(value int64) *int64 { return &value }

func newBool(value bool) *bool { return &value }

func sortedKeys(values map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}
