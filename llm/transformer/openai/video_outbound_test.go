package openai

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
)

func newVideoOutbound(t *testing.T) *OutboundTransformer {
	t.Helper()

	out, err := NewOutboundTransformerWithConfig(&Config{
		PlatformType:   PlatformOpenAI,
		BaseURL:        "https://apihub.agnes-ai.com",
		APIKeyProvider: auth.NewStaticKeyProvider("sk-test"),
	})
	require.NoError(t, err)

	return out.(*OutboundTransformer)
}

func TestOutbound_ParseGetVideoTaskResponse_AgnesSecondsString(t *testing.T) {
	out := newVideoOutbound(t)

	body := []byte(`{
		"id": "video_xxx",
		"object": "video",
		"model": "agnes-video-v2.0",
		"status": "completed",
		"progress": 100,
		"seconds": "3.4",
		"size": "1088x832",
		"created_at": 1781604979,
		"completed_at": 1781605076,
		"video_url": "https://platform-outputs.agnes-ai.space/videos/agnes-video-v2.0/2026/06/16/video_xxx.mp4"
	}`)

	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	}

	resp, err := out.ParseGetVideoTaskResponse(context.Background(), httpResp)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Video)

	assert.Equal(t, "video_xxx", resp.Video.ID)
	assert.Equal(t, "succeeded", resp.Video.Status)
	require.NotNil(t, resp.Video.Duration)
	assert.Equal(t, "3.4", *resp.Video.Duration)
	assert.Equal(t, "https://platform-outputs.agnes-ai.space/videos/agnes-video-v2.0/2026/06/16/video_xxx.mp4", resp.Video.VideoURL)
}

func TestOutbound_ParseGetVideoTaskResponse_SecondsStringInteger(t *testing.T) {
	out := newVideoOutbound(t)

	body := []byte(`{
		"id": "video_yyy",
		"object": "video",
		"status": "queued",
		"progress": 0,
		"seconds": "8",
		"size": "1280x720",
		"created_at": 1781607020,
		"video_url": "https://example.com/video.mp4"
	}`)

	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	}

	resp, err := out.ParseGetVideoTaskResponse(context.Background(), httpResp)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Video)

	assert.Equal(t, "video_yyy", resp.Video.ID)
	assert.Equal(t, "queued", resp.Video.Status)
	require.NotNil(t, resp.Video.Duration)
	assert.Equal(t, "8", *resp.Video.Duration)
	assert.Equal(t, "https://example.com/video.mp4", resp.Video.VideoURL)
}

func TestOutbound_BuildGetVideoTaskRequest(t *testing.T) {
	out := newVideoOutbound(t)

	req, err := out.BuildGetVideoTaskRequest(context.Background(), "task_123")
	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, req.Method)
	assert.Equal(t, "https://apihub.agnes-ai.com/v1/videos/task_123", req.URL)
	assert.Equal(t, string(llm.RequestTypeVideo), req.RequestType)
	assert.Equal(t, string(llm.APIFormatOpenAIVideo), req.APIFormat)
}
