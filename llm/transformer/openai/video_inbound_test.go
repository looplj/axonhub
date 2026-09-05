package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

func TestVideoInboundTransformer_TransformRequest_JSON(t *testing.T) {
	inbound := NewVideoInboundTransformer()

	reqBody := []byte(`{
		"model":"sora-2",
		"prompt":"a cat walking",
		"input_reference":"https://example.com/a.png",
		"seconds":"8",
		"size":"1280x720",
		"ratio":"16:9",
		"resolution":"720p",
		"callback_url":"http:\/\/example.com/callback",
		"return_last_frame":true,
		"tools":[{"type":"web_search"}],
		"extra_body":{"provider_hint":{"mode":"fast"}},
		"camera_fixed":true
	}`)

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/videos",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    reqBody,
	}

	llmReq, err := inbound.TransformRequest(context.Background(), httpReq)
	require.NoError(t, err)

	assert.Equal(t, llm.RequestTypeVideo, llmReq.RequestType)
	assert.Equal(t, llm.APIFormatOpenAIVideo, llmReq.APIFormat)
	assert.Equal(t, "sora-2", llmReq.Model)
	require.NotNil(t, llmReq.Video)
	assert.Equal(t, "sora-2", llmReq.Video.Model)
	assert.Equal(t, loPtrString("8"), llmReq.Video.Duration)
	assert.Equal(t, "1280x720", llmReq.Video.Size)
	assert.Equal(t, "16:9", llmReq.Video.Ratio)
	assert.Equal(t, "720p", llmReq.Video.Resolution)
	assert.Equal(t, "http://example.com/callback", llmReq.Video.CallbackURL)
	assert.Equal(t, loPtrBool(true), llmReq.Video.ReturnLastFrame)
	require.Len(t, llmReq.Video.Tools, 1)
	assert.JSONEq(t, `{"type":"web_search"}`, string(llmReq.Video.Tools[0]))
	assert.JSONEq(t, `{"provider_hint":{"mode":"fast"},"camera_fixed":true}`, string(llmReq.ExtraBody))
	assert.Equal(t, "a cat walking", firstVideoText(llmReq.Video.Content))
	assert.Equal(t, "https://example.com/a.png", firstVideoImageURL(llmReq.Video.Content))
}

func TestVideoInboundTransformer_TransformRequest_JSONPreservesContent(t *testing.T) {
	// Given: a public OpenAI video request with typed content items.
	inbound := NewVideoInboundTransformer()
	httpReq := &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body: []byte(`{
			"model":"zenmux-video",
			"content":[
				{"type":"text","text":"keep this prompt"},
				{"type":"image_url","image_url":{"url":"https://example.com/last.png"},"role":"last_frame"},
				{"type":"video_url","video_url":{"url":"https://example.com/reference.mp4"},"role":"reference_video"}
			]
		}`),
	}

	// When: the public request is converted to the unified video request.
	llmReq, err := inbound.TransformRequest(context.Background(), httpReq)

	// Then: the supplied content items are preserved in order and without synthesis.
	require.NoError(t, err)
	require.NotNil(t, llmReq.Video)
	require.Equal(t, []llm.VideoContent{
		{Type: "text", Text: "keep this prompt"},
		{Type: "image_url", ImageURL: &llm.VideoImageURL{URL: "https://example.com/last.png"}, Role: "last_frame"},
		{Type: "video_url", VideoURL: &llm.VideoURL{URL: "https://example.com/reference.mp4"}, Role: "reference_video"},
	}, llmReq.Video.Content)
}

func TestVideoInboundTransformer_TransformRequest_Multipart_WithInputReferenceFile(t *testing.T) {
	inbound := NewVideoInboundTransformer()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	require.NoError(t, writer.WriteField("model", "sora-2"))
	require.NoError(t, writer.WriteField("prompt", "a cat walking"))
	require.NoError(t, writer.WriteField("seconds", "8"))
	require.NoError(t, writer.WriteField("size", "1280x720"))
	require.NoError(t, writer.WriteField("callback_url", "http://example.com/callback"))
	require.NoError(t, writer.WriteField("return_last_frame", "true"))
	require.NoError(t, writer.WriteField("tools", `[{"type":"web_search"}]`))
	require.NoError(t, writer.WriteField("extra_body", `{"provider_hint":{"mode":"fast"}}`))
	require.NoError(t, writer.WriteField("camera_fixed", "true"))

	addFilePartVideo(t, writer, "input_reference", "ref.png", "image/png", []byte("pngdata"))
	require.NoError(t, writer.Close())

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/videos",
		Headers: http.Header{"Content-Type": []string{writer.FormDataContentType()}},
		Body:    body.Bytes(),
	}

	llmReq, err := inbound.TransformRequest(context.Background(), httpReq)
	require.NoError(t, err)

	assert.Equal(t, llm.RequestTypeVideo, llmReq.RequestType)
	assert.Equal(t, llm.APIFormatOpenAIVideo, llmReq.APIFormat)
	require.NotNil(t, llmReq.Video)
	assert.Equal(t, "http://example.com/callback", llmReq.Video.CallbackURL)
	assert.Equal(t, loPtrBool(true), llmReq.Video.ReturnLastFrame)
	require.Len(t, llmReq.Video.Tools, 1)
	assert.JSONEq(t, `{"type":"web_search"}`, string(llmReq.Video.Tools[0]))
	assert.JSONEq(t, `{"provider_hint":{"mode":"fast"},"camera_fixed":true}`, string(llmReq.ExtraBody))

	ref := firstVideoImageURL(llmReq.Video.Content)
	require.NotEmpty(t, ref)
	assert.Contains(t, ref, "data:image/png;base64,")
}

func TestVideoInboundTransformer_TransformRequest_MultipartPreservesContent(t *testing.T) {
	// Given: a multipart request carrying a typed content[] JSON field.
	inbound := NewVideoInboundTransformer()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.WriteField("model", "zenmux-video"))
	require.NoError(t, writer.WriteField("content", `[{"type":"text","text":"multipart prompt"},{"type":"image_url","image_url":{"url":"https://example.com/first.png"},"role":"first_frame"}]`))
	require.NoError(t, writer.Close())
	httpReq := &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{writer.FormDataContentType()}},
		Body:    body.Bytes(),
	}

	// When: the multipart request is converted to the unified video request.
	llmReq, err := inbound.TransformRequest(context.Background(), httpReq)

	// Then: the typed multipart content is forwarded instead of requiring prompt.
	require.NoError(t, err)
	require.NotNil(t, llmReq.Video)
	require.Equal(t, []llm.VideoContent{
		{Type: "text", Text: "multipart prompt"},
		{Type: "image_url", ImageURL: &llm.VideoImageURL{URL: "https://example.com/first.png"}, Role: "first_frame"},
	}, llmReq.Video.Content)
}

func TestVideoInboundTransformer_TransformResponse_JSON(t *testing.T) {
	inbound := NewVideoInboundTransformer()

	llmResp := &llm.Response{
		RequestType: llm.RequestTypeVideo,
		APIFormat:   llm.APIFormatOpenAIVideo,
		Video: &llm.VideoResponse{
			ID:           "vid_1",
			Status:       "running",
			Model:        "sora-2",
			Prompt:       "a cat",
			Duration:     loPtrString("8"),
			Size:         "1280x720",
			Progress:     loPtrFloat64(50),
			LastFrameURL: "https://example.com/last.jpg",
			Content:      json.RawMessage(`{"video_url":"https://example.com/video.mp4","last_frame_url":"https://example.com/last.jpg","provider_metadata":{"request_id":"native-1"}}`),
			CreatedAt:    1700000000,
		},
	}

	httpResp, err := inbound.TransformResponse(context.Background(), llmResp)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResp.StatusCode)

	var oaiResp OpenAIVideoObject
	require.NoError(t, json.Unmarshal(httpResp.Body, &oaiResp))
	assert.Equal(t, "vid_1", oaiResp.ID)
	assert.Equal(t, "in_progress", oaiResp.Status)
	assert.Equal(t, "sora-2", oaiResp.Model)
	assert.Equal(t, "a cat", oaiResp.Prompt)
	assert.Equal(t, loPtrString("8"), oaiResp.Seconds)
	assert.Equal(t, "1280x720", oaiResp.Size)
	assert.Equal(t, "https://example.com/last.jpg", oaiResp.LastFrameURL)
	assert.JSONEq(t, `{"video_url":"https://example.com/video.mp4","last_frame_url":"https://example.com/last.jpg","provider_metadata":{"request_id":"native-1"}}`, string(oaiResp.Content))
}

func TestVideoInboundTransformer_TransformRequest_rejects_non_object_extra_body(t *testing.T) {
	// Given
	inbound := NewVideoInboundTransformer()
	httpReq := &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    []byte(`{"model":"sora-2","prompt":"a cat walking","extra_body":[]}`),
	}

	// When
	_, err := inbound.TransformRequest(context.Background(), httpReq)

	// Then
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
}

func addFilePartVideo(t *testing.T, writer *multipart.Writer, fieldName, filename, contentType string, data []byte) {
	t.Helper()

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="`+fieldName+`"; filename="`+filename+`"`)
	h.Set("Content-Type", contentType)

	part, err := writer.CreatePart(h)
	require.NoError(t, err)

	_, err = part.Write(data)
	require.NoError(t, err)
}

func loPtrInt64(v int64) *int64       { return &v }
func loPtrFloat64(v float64) *float64 { return &v }
func loPtrString(v string) *string    { return &v }
func loPtrBool(v bool) *bool          { return &v }
