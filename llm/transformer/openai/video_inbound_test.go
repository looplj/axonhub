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
)

func TestVideoInboundTransformer_TransformRequest_JSON(t *testing.T) {
	inbound := NewVideoInboundTransformer()

	reqBody := []byte(`{
		"model":"sora-2",
		"prompt":"a cat walking",
		"input_reference":"https://example.com/a.png",
		"seconds":8,
		"size":"1280x720"
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
	assert.Equal(t, loPtrInt64(8), llmReq.Video.Duration)
	assert.Equal(t, "1280x720", llmReq.Video.Size)
	assert.Equal(t, "a cat walking", firstVideoText(llmReq.Video.Content))
	assert.Equal(t, "https://example.com/a.png", firstVideoImageURL(llmReq.Video.Content))
}

func TestVideoInboundTransformer_TransformRequest_Multipart_WithInputReferenceFile(t *testing.T) {
	inbound := NewVideoInboundTransformer()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	require.NoError(t, writer.WriteField("model", "sora-2"))
	require.NoError(t, writer.WriteField("prompt", "a cat walking"))
	require.NoError(t, writer.WriteField("seconds", "8"))
	require.NoError(t, writer.WriteField("size", "1280x720"))

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

	ref := firstVideoImageURL(llmReq.Video.Content)
	require.NotEmpty(t, ref)
	assert.Contains(t, ref, "data:image/png;base64,")
}

func TestVideoInboundTransformer_TransformResponse_JSON(t *testing.T) {
	inbound := NewVideoInboundTransformer()

	llmResp := &llm.Response{
		RequestType: llm.RequestTypeVideo,
		APIFormat:   llm.APIFormatOpenAIVideo,
		Video: &llm.VideoResponse{
			ID:        "vid_1",
			Status:    "running",
			Model:     "sora-2",
			Prompt:    "a cat",
			Duration:  loPtrInt64(8),
			Size:      "1280x720",
			Progress:  loPtrFloat64(50),
			CreatedAt: 1700000000,
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
	assert.Equal(t, loPtrInt64(8), oaiResp.Seconds)
	assert.Equal(t, "1280x720", oaiResp.Size)
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
