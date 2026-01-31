package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestImageInboundTransformer_TransformRequest_Generation_JSON(t *testing.T) {
	inbound := NewImageGenerationInboundTransformer()

	reqBody := []byte(`{
		"prompt":"a cat",
		"model":"dall-e-3",
		"n":2,
		"response_format":"url",
		"size":"1024x1024",
		"user":"u1"
	}`)

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/images/generations",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    reqBody,
	}

	llmReq, err := inbound.TransformRequest(context.Background(), httpReq)
	require.NoError(t, err)

	assert.Equal(t, llm.RequestTypeImage, llmReq.RequestType)
	assert.Equal(t, llm.APIFormatOpenAIImageGeneration, llmReq.APIFormat)
	assert.Equal(t, "dall-e-3", llmReq.Model)
	assert.Contains(t, llmReq.Modalities, "image")
	assert.NotNil(t, llmReq.Stream)
	assert.False(t, *llmReq.Stream)
	require.NotNil(t, llmReq.Image)
	assert.Equal(t, "a cat", llmReq.Image.Prompt)
	assert.Equal(t, lo.ToPtr(int64(2)), llmReq.Image.N)
	assert.Equal(t, "url", llmReq.Image.ResponseFormat)
	assert.Equal(t, "1024x1024", llmReq.Image.Size)

	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ot := tr.(*OutboundTransformer)

	outReq, err := ot.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	assert.Equal(t, "https://api.openai.com/v1/images/generations", outReq.URL)
	assert.Contains(t, string(outReq.Body), `"response_format":"url"`)
	assert.Contains(t, string(outReq.Body), `"n":2`)
}

func TestImageInboundTransformer_TransformRequest_Edit_Multipart_WithMask(t *testing.T) {
	inbound := NewImageEditInboundTransformer()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	require.NoError(t, writer.WriteField("prompt", "make it blue"))
	require.NoError(t, writer.WriteField("model", "dall-e-2"))
	require.NoError(t, writer.WriteField("response_format", "b64_json"))

	addFilePart(t, writer, "image", "image.png", "image/png", []byte("img"))
	addFilePart(t, writer, "mask", "mask.png", "image/png", []byte("msk"))

	require.NoError(t, writer.Close())

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/images/edits",
		Headers: http.Header{"Content-Type": []string{writer.FormDataContentType()}},
		Body:    body.Bytes(),
	}

	llmReq, err := inbound.TransformRequest(context.Background(), httpReq)
	require.NoError(t, err)

	assert.Equal(t, llm.RequestTypeImage, llmReq.RequestType)
	assert.Equal(t, llm.APIFormatOpenAIImageEdit, llmReq.APIFormat)
	assert.Contains(t, llmReq.Modalities, "image")
	require.NotNil(t, llmReq.Image)
	assert.Equal(t, "make it blue", llmReq.Image.Prompt)
	assert.Equal(t, "b64_json", llmReq.Image.ResponseFormat)
	assert.Len(t, llmReq.Image.Images, 1)
	assert.NotNil(t, llmReq.Image.Mask)

	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ot := tr.(*OutboundTransformer)

	outReq, err := ot.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	assert.Equal(t, "https://api.openai.com/v1/images/edits", outReq.URL)
	assert.Contains(t, string(outReq.Body), `name="mask"`)
	assert.Contains(t, string(outReq.Body), `name="image"`)
	assert.Contains(t, string(outReq.Body), `name="prompt"`)
}

func TestImageInboundTransformer_TransformRequest_Variation_Multipart(t *testing.T) {
	inbound := NewImageVariationInboundTransformer()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	require.NoError(t, writer.WriteField("model", "dall-e-2"))
	require.NoError(t, writer.WriteField("n", "2"))
	addFilePart(t, writer, "image", "image.png", "image/png", []byte("img"))
	require.NoError(t, writer.Close())

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/images/variations",
		Headers: http.Header{"Content-Type": []string{writer.FormDataContentType()}},
		Body:    body.Bytes(),
	}

	llmReq, err := inbound.TransformRequest(context.Background(), httpReq)
	require.NoError(t, err)

	assert.Equal(t, llm.RequestTypeImage, llmReq.RequestType)
	assert.Equal(t, llm.APIFormatOpenAIImageVariation, llmReq.APIFormat)
	require.NotNil(t, llmReq.Image)
	assert.Equal(t, lo.ToPtr(int64(2)), llmReq.Image.N)
	assert.Len(t, llmReq.Image.Images, 1)

	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ot := tr.(*OutboundTransformer)

	outReq, err := ot.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	assert.Equal(t, "https://api.openai.com/v1/images/variations", outReq.URL)
	assert.NotContains(t, string(outReq.Body), `name="prompt"`)
}

func TestImageInboundTransformer_TransformRequest_Edit_Multipart_ImageArrayFieldName(t *testing.T) {
	inbound := NewImageEditInboundTransformer()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	require.NoError(t, writer.WriteField("prompt", "change dog color to black"))
	require.NoError(t, writer.WriteField("model", "gpt-image-1.5"))

	addFilePart(t, writer, "image[]", "dog.png", "image/png", []byte("dogimg"))
	require.NoError(t, writer.Close())

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/images/edits",
		Headers: http.Header{"Content-Type": []string{writer.FormDataContentType()}},
		Body:    body.Bytes(),
	}

	llmReq, err := inbound.TransformRequest(context.Background(), httpReq)
	require.NoError(t, err)

	assert.Equal(t, llm.RequestTypeImage, llmReq.RequestType)
	assert.Equal(t, llm.APIFormatOpenAIImageEdit, llmReq.APIFormat)
	assert.Equal(t, "gpt-image-1.5", llmReq.Model)
	require.NotNil(t, llmReq.Image)
	assert.Equal(t, "change dog color to black", llmReq.Image.Prompt)
	assert.Len(t, llmReq.Image.Images, 1)
	assert.Equal(t, []byte("dogimg"), llmReq.Image.Images[0])
}

func TestImageInboundTransformer_TransformRequest_Edit_Multipart_MultipleImagesWithArraySyntax(t *testing.T) {
	inbound := NewImageEditInboundTransformer()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	require.NoError(t, writer.WriteField("prompt", "combine these images"))
	require.NoError(t, writer.WriteField("model", "gpt-image-1.5"))

	addFilePart(t, writer, "image[]", "img1.png", "image/png", []byte("img1"))
	addFilePart(t, writer, "image[]", "img2.png", "image/png", []byte("img2"))
	require.NoError(t, writer.Close())

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/images/edits",
		Headers: http.Header{"Content-Type": []string{writer.FormDataContentType()}},
		Body:    body.Bytes(),
	}

	llmReq, err := inbound.TransformRequest(context.Background(), httpReq)
	require.NoError(t, err)

	require.NotNil(t, llmReq.Image)
	assert.Len(t, llmReq.Image.Images, 2)
	assert.Equal(t, []byte("img1"), llmReq.Image.Images[0])
	assert.Equal(t, []byte("img2"), llmReq.Image.Images[1])
}

func TestImageInboundTransformer_TransformRequest_Edit_Multipart_MixedImageFieldNames(t *testing.T) {
	inbound := NewImageEditInboundTransformer()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	require.NoError(t, writer.WriteField("prompt", "edit images"))
	require.NoError(t, writer.WriteField("model", "dall-e-2"))

	addFilePart(t, writer, "image", "img1.png", "image/png", []byte("img1"))
	addFilePart(t, writer, "image[]", "img2.png", "image/png", []byte("img2"))
	require.NoError(t, writer.Close())

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/images/edits",
		Headers: http.Header{"Content-Type": []string{writer.FormDataContentType()}},
		Body:    body.Bytes(),
	}

	llmReq, err := inbound.TransformRequest(context.Background(), httpReq)
	require.NoError(t, err)

	require.NotNil(t, llmReq.Image)
	assert.Len(t, llmReq.Image.Images, 2)
}

func TestImageInboundTransformer_TransformResponse_ToImagesResponse(t *testing.T) {
	inbound := NewImageGenerationInboundTransformer()

	resp := &llm.Response{
		Created: 123,
		Image: &llm.ImageResponse{
			Created: 123,
			Data: []llm.ImageData{
				{
					B64JSON: "AAA",
					URL:     "data:image/png;base64,AAA",
				},
				{
					URL: "https://example.com/a.png",
				},
			},
		},
	}

	httpResp, err := inbound.TransformResponse(context.Background(), resp)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResp.StatusCode)

	var oaiResp ImagesResponse
	require.NoError(t, json.Unmarshal(httpResp.Body, &oaiResp))

	assert.Equal(t, int64(123), oaiResp.Created)
	require.Len(t, oaiResp.Data, 2)
	assert.Equal(t, "AAA", oaiResp.Data[0].B64JSON)
	assert.Equal(t, "https://example.com/a.png", oaiResp.Data[1].URL)
}

func addFilePart(t *testing.T, writer *multipart.Writer, fieldName, filename, contentType string, data []byte) {
	t.Helper()

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="`+fieldName+`"; filename="`+filename+`"`)
	h.Set("Content-Type", contentType)

	part, err := writer.CreatePart(h)
	require.NoError(t, err)

	_, err = part.Write(data)
	require.NoError(t, err)
}
