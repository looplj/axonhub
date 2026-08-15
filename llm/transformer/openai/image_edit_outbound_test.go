package openai

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestImageEditRequestWritesModelBeforeImageFiles(t *testing.T) {
	// Given
	outbound, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	request := &llm.Request{
		Model:       "gpt-image-2",
		RequestType: llm.RequestTypeImage,
		APIFormat:   llm.APIFormatOpenAIImageEdit,
		Image: &llm.ImageRequest{
			Prompt: "Make this image brighter",
			Images: [][]byte{[]byte("image-data")},
		},
	}

	// When
	httpRequest, err := outbound.TransformRequest(t.Context(), request)
	require.NoError(t, err)

	_, params, err := mime.ParseMediaType(httpRequest.Headers.Get("Content-Type"))
	require.NoError(t, err)

	part, err := multipart.NewReader(bytes.NewReader(httpRequest.Body), params["boundary"]).NextPart()
	require.NoError(t, err)

	value, err := io.ReadAll(part)
	require.NoError(t, err)

	// Then
	require.Equal(t, "model", part.FormName())
	require.Equal(t, "gpt-image-2", string(value))
}

func TestImageEditRequestKeepsJPEGMetadataConsistent(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	request := &llm.Request{
		Model:       "gpt-image-2",
		RequestType: llm.RequestTypeImage,
		APIFormat:   llm.APIFormatOpenAIImageEdit,
		Image: &llm.ImageRequest{
			Prompt: "Make this image brighter",
			Images: [][]byte{{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F'}},
		},
	}

	httpRequest, err := outbound.TransformRequest(t.Context(), request)
	require.NoError(t, err)

	var logged struct {
		FormFiles []struct {
			Filename    string `json:"filename"`
			ContentType string `json:"content_type"`
			Format      string `json:"format"`
		} `json:"formFiles"`
	}
	require.NoError(t, json.Unmarshal(httpRequest.JSONBody, &logged))
	require.Len(t, logged.FormFiles, 1)
	require.Equal(t, "image_1.jpg", logged.FormFiles[0].Filename)
	require.Equal(t, "image/jpeg", logged.FormFiles[0].ContentType)
	require.Equal(t, "jpg", logged.FormFiles[0].Format)

	_, params, err := mime.ParseMediaType(httpRequest.Headers.Get("Content-Type"))
	require.NoError(t, err)
	reader := multipart.NewReader(bytes.NewReader(httpRequest.Body), params["boundary"])
	for {
		part, err := reader.NextPart()
		require.NoError(t, err)
		if part.FileName() == "" {
			continue
		}
		require.Equal(t, "image_1.jpg", part.FileName())
		require.Equal(t, "image/jpeg", part.Header.Get("Content-Type"))
		break
	}
}
