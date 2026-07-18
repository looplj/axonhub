package openai

import (
	"bytes"
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
