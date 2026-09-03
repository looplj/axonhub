package openai

import (
	"bytes"
	"context"
	"encoding/base64"
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
	transformer "github.com/looplj/axonhub/llm/transformer"
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

func TestImageInboundTransformer_TransformRequest_Generation_WithSingleImage(t *testing.T) {
	inbound := NewImageGenerationInboundTransformer()

	// 1x1 red pixel PNG
	pngBytes := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)

	reqBody, err := json.Marshal(map[string]any{
		"prompt": "make this image brighter",
		"model":  "gpt-image-1",
		"image":  dataURL,
	})
	require.NoError(t, err)

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/images/generations",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    reqBody,
	}

	llmReq, err := inbound.TransformRequest(context.Background(), httpReq)
	require.NoError(t, err)

	require.NotNil(t, llmReq.Image)
	require.Len(t, llmReq.Image.Images, 1)
	assert.Equal(t, pngBytes, llmReq.Image.Images[0])
}

func TestImageInboundTransformer_TransformRequest_Generation_WithMultipleImages(t *testing.T) {
	inbound := NewImageGenerationInboundTransformer()

	pngBytes1 := []byte{0x89, 0x50, 0x4E, 0x47}
	pngBytes2 := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D}
	dataURL1 := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes1)
	dataURL2 := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes2)

	reqBody, err := json.Marshal(map[string]any{
		"prompt": "combine these images",
		"model":  "gpt-image-1",
		"image":  []string{dataURL1, dataURL2},
	})
	require.NoError(t, err)

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/images/generations",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    reqBody,
	}

	llmReq, err := inbound.TransformRequest(context.Background(), httpReq)
	require.NoError(t, err)

	require.NotNil(t, llmReq.Image)
	require.Len(t, llmReq.Image.Images, 2)
	assert.Equal(t, pngBytes1, llmReq.Image.Images[0])
	assert.Equal(t, pngBytes2, llmReq.Image.Images[1])
}

func TestImageInboundTransformer_TransformRequest_Generation_WithInvalidImageField(t *testing.T) {
	inbound := NewImageGenerationInboundTransformer()

	reqBody := []byte(`{
		"prompt":"a cat",
		"model":"gpt-image-1",
		"image":123
	}`)

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/images/generations",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    reqBody,
	}

	_, err := inbound.TransformRequest(context.Background(), httpReq)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image field must be a string or array of strings")
}

func TestImageInboundTransformer_TransformRequest_Generation_WithNonDataURLImage(t *testing.T) {
	inbound := NewImageGenerationInboundTransformer()

	reqBody := []byte(`{
		"prompt":"a cat",
		"model":"gpt-image-1",
		"image":"https://example.com/image.png"
	}`)

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/images/generations",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    reqBody,
	}

	_, err := inbound.TransformRequest(context.Background(), httpReq)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image must be a data URL")
}

func TestImageInboundTransformer_TransformRequest_Generation_WithNonBase64DataURL(t *testing.T) {
	inbound := NewImageGenerationInboundTransformer()

	reqBody := []byte(`{
		"prompt":"a cat",
		"model":"gpt-image-1",
		"image":"data:image/png,raw-not-base64"
	}`)

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/images/generations",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    reqBody,
	}

	_, err := inbound.TransformRequest(context.Background(), httpReq)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be base64-encoded")
}

func TestImageInboundTransformer_Generation_RoundTrip_WithImage(t *testing.T) {
	inbound := NewImageGenerationInboundTransformer()

	pngBytes := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)

	reqBody, err := json.Marshal(map[string]any{
		"prompt": "make this brighter",
		"model":  "gpt-image-1",
		"image":  dataURL,
	})
	require.NoError(t, err)

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/images/generations",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    reqBody,
	}

	llmReq, err := inbound.TransformRequest(context.Background(), httpReq)
	require.NoError(t, err)

	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ot := tr.(*OutboundTransformer)

	outReq, err := ot.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	assert.Equal(t, "https://api.openai.com/v1/images/generations", outReq.URL)

	var body map[string]any
	require.NoError(t, json.Unmarshal(outReq.Body, &body))

	imageField, ok := body["image"].(string)
	require.True(t, ok, "image should be forwarded as a string in the outbound request")
	assert.Contains(t, imageField, "data:image/png;base64,")
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

func TestImageInboundTransformer_TransformRequest_Edit_Multipart_AcceptsImageAboveLegacy4MBLimit(t *testing.T) {
	inbound := NewImageEditInboundTransformer()

	imageData := bytes.Repeat([]byte{0x89}, 5*1024*1024)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	require.NoError(t, writer.WriteField("prompt", "edit image"))
	addFilePart(t, writer, "image", "large.png", "image/png", imageData)
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
	require.Len(t, llmReq.Image.Images, 1)
	assert.Len(t, llmReq.Image.Images[0], len(imageData))
}

func TestImageInboundTransformer_TransformRequest_Edit_JSON(t *testing.T) {
	inbound := NewImageEditInboundTransformer()

	pngBytes := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)

	reqBody, err := json.Marshal(map[string]any{
		"prompt":          "make it blue",
		"model":           "sensenova-u1.5-lite",
		"image":           dataURL,
		"size":            "1024x1024",
		"n":               2,
		"response_format": "b64_json",
	})
	require.NoError(t, err)

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/images/edits",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    reqBody,
	}

	llmReq, err := inbound.TransformRequest(context.Background(), httpReq)
	require.NoError(t, err)

	assert.Equal(t, llm.RequestTypeImage, llmReq.RequestType)
	assert.Equal(t, llm.APIFormatOpenAIImageEdit, llmReq.APIFormat)
	assert.Equal(t, "sensenova-u1.5-lite", llmReq.Model)
	assert.Contains(t, llmReq.Modalities, "image")
	require.NotNil(t, llmReq.Stream)
	assert.False(t, *llmReq.Stream)
	require.NotNil(t, llmReq.Image)
	assert.Equal(t, "make it blue", llmReq.Image.Prompt)
	assert.Equal(t, "1024x1024", llmReq.Image.Size)
	assert.Equal(t, lo.ToPtr(int64(2)), llmReq.Image.N)
	assert.Equal(t, "b64_json", llmReq.Image.ResponseFormat)
	require.Len(t, llmReq.Image.Images, 1)
	assert.Equal(t, pngBytes, llmReq.Image.Images[0])
}

func TestImageInboundTransformer_TransformRequest_Edit_JSON_MultipleImagesAndMask(t *testing.T) {
	inbound := NewImageEditInboundTransformer()

	pngBytes1 := []byte{0x89, 0x50, 0x4E, 0x47}
	pngBytes2 := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D}
	maskBytes := []byte{0x89, 0x4D, 0x53, 0x4B}
	dataURL1 := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes1)
	dataURL2 := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes2)
	maskURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(maskBytes)

	reqBody, err := json.Marshal(map[string]any{
		"prompt": "combine these images",
		"model":  "gpt-image-1",
		"image":  []string{dataURL1, dataURL2},
		"mask":   maskURL,
	})
	require.NoError(t, err)

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/images/edits",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    reqBody,
	}

	llmReq, err := inbound.TransformRequest(context.Background(), httpReq)
	require.NoError(t, err)

	require.NotNil(t, llmReq.Image)
	require.Len(t, llmReq.Image.Images, 2)
	assert.Equal(t, pngBytes1, llmReq.Image.Images[0])
	assert.Equal(t, pngBytes2, llmReq.Image.Images[1])
	assert.Equal(t, maskBytes, llmReq.Image.Mask)
}

func TestImageInboundTransformer_TransformRequest_Edit_JSON_RequiresPrompt(t *testing.T) {
	inbound := NewImageEditInboundTransformer()

	pngBytes := []byte{0x89, 0x50, 0x4E, 0x47}
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)

	reqBody, err := json.Marshal(map[string]any{
		"model": "sensenova-u1.5-lite",
		"image": dataURL,
	})
	require.NoError(t, err)

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/images/edits",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    reqBody,
	}

	_, err = inbound.TransformRequest(context.Background(), httpReq)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prompt is required")
}

func TestImageInboundTransformer_TransformRequest_Edit_JSON_RequiresImage(t *testing.T) {
	inbound := NewImageEditInboundTransformer()

	reqBody := []byte(`{"prompt":"make it blue","model":"sensenova-u1.5-lite"}`)

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/images/edits",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    reqBody,
	}

	_, err := inbound.TransformRequest(context.Background(), httpReq)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one image is required")
}

func TestImageInboundTransformer_TransformRequest_Edit_JSON_RejectsStream(t *testing.T) {
	inbound := NewImageEditInboundTransformer()

	pngBytes := []byte{0x89, 0x50, 0x4E, 0x47}
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)

	reqBody, err := json.Marshal(map[string]any{
		"prompt": "make it blue",
		"model":  "sensenova-u1.5-lite",
		"image":  dataURL,
		"stream": true,
	})
	require.NoError(t, err)

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/images/edits",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    reqBody,
	}

	_, err = inbound.TransformRequest(context.Background(), httpReq)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support streaming")
}

func TestImageInboundTransformer_TransformRequest_Edit_JSON_RejectsNonDataURLImage(t *testing.T) {
	inbound := NewImageEditInboundTransformer()

	reqBody := []byte(`{
		"prompt":"make it blue",
		"model":"sensenova-u1.5-lite",
		"image":"https://example.com/image.png"
	}`)

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/images/edits",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    reqBody,
	}

	_, err := inbound.TransformRequest(context.Background(), httpReq)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image must be a data URL")
}

func TestImageInboundTransformer_TransformRequest_Edit_JSON_BodyTooLarge(t *testing.T) {
	inbound := NewImageEditInboundTransformer()
	originalMaxBodySize := maxImageBodySize
	maxImageBodySize = 32
	t.Cleanup(func() { maxImageBodySize = originalMaxBodySize })

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/images/edits",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    make([]byte, maxImageBodySize+1),
	}

	_, err := inbound.TransformRequest(context.Background(), httpReq)
	require.Error(t, err)
	assert.ErrorIs(t, err, transformer.ErrInvalidRequest)
	assert.Contains(t, err.Error(), "request body too large")
}

func TestImageInboundTransformer_TransformRequest_Edit_JSON_RejectsTooManyImagesBeforeDecoding(t *testing.T) {
	inbound := NewImageEditInboundTransformer()
	images := make([]string, maxImageCount+1)
	for i := range images {
		images[i] = "not-a-data-url"
	}
	reqBody, err := json.Marshal(map[string]any{"prompt": "edit", "image": images})
	require.NoError(t, err)

	_, err = inbound.TransformRequest(context.Background(), &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/images/edits",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    reqBody,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many images")
}

func TestDecodeDataURLToBytes_RejectsUnsupportedImageType(t *testing.T) {
	_, err := decodeDataURLToBytes("data:text/plain;base64,aGVsbG8=")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported image content type")
}

func TestDecodeDataURLToBytes_RejectsOversizedImage(t *testing.T) {
	originalMaxFileSize := maxImageFileSize
	maxImageFileSize = 4
	t.Cleanup(func() { maxImageFileSize = originalMaxFileSize })

	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	_, err := decodeDataURLToBytes("data:image/png;base64," + base64.StdEncoding.EncodeToString(png))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image file too large")
}

func TestDecodeDataURLToBytes_RejectsOversizedImageBeforeDecode(t *testing.T) {
	originalMaxFileSize := maxImageFileSize
	maxImageFileSize = 3
	t.Cleanup(func() { maxImageFileSize = originalMaxFileSize })

	_, err := decodeDataURLToBytes("data:image/png;base64,AAAAAA==")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image file too large")
}

func TestImageInboundTransformer_TransformRequest_Edit_JSON_TooManyImages(t *testing.T) {
	inbound := NewImageEditInboundTransformer()

	pngBytes := []byte{0x89, 0x50, 0x4E, 0x47}
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)

	images := make([]string, maxImageCount+1)
	for i := range images {
		images[i] = dataURL
	}

	reqBody, err := json.Marshal(map[string]any{
		"prompt": "make it blue",
		"model":  "sensenova-u1.5-lite",
		"image":  images,
	})
	require.NoError(t, err)

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/images/edits",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    reqBody,
	}

	_, err = inbound.TransformRequest(context.Background(), httpReq)
	require.Error(t, err)
	assert.ErrorIs(t, err, transformer.ErrInvalidRequest)
	assert.Contains(t, err.Error(), "too many images")
}

func TestImageInboundTransformer_Edit_JSON_RoundTrip_ToMultipartOutbound(t *testing.T) {
	inbound := NewImageEditInboundTransformer()

	pngBytes := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)

	reqBody, err := json.Marshal(map[string]any{
		"prompt": "make this brighter",
		"model":  "dall-e-2",
		"image":  dataURL,
	})
	require.NoError(t, err)

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "http://localhost/v1/images/edits",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    reqBody,
	}

	llmReq, err := inbound.TransformRequest(context.Background(), httpReq)
	require.NoError(t, err)

	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ot := tr.(*OutboundTransformer)

	outReq, err := ot.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	assert.Equal(t, "https://api.openai.com/v1/images/edits", outReq.URL)
	assert.Contains(t, outReq.Headers.Get("Content-Type"), "multipart/form-data")
	assert.Contains(t, string(outReq.Body), `name="image"`)
	assert.Contains(t, string(outReq.Body), `name="prompt"`)
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
