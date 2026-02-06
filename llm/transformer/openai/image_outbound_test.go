package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestTransformRequest_RoutesToImageGenerationAPI_WhenImageRequestPresent(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}

	ot := tr.(*OutboundTransformer)
	req := &llm.Request{
		Model:       "gpt-4o-mini",
		RequestType: llm.RequestTypeImage,
		APIFormat:   llm.APIFormatOpenAIImageGeneration,
		Image: &llm.ImageRequest{
			Prompt:       "Generate a beautiful sunset over mountains",
			OutputFormat: "png",
			Size:         "1024x1024",
		},
	}

	hreq, err := ot.TransformRequest(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "https://api.openai.com/v1/images/generations", hreq.URL)
}

func TestTransformRequest_RoutesToImageGenerationAPI_WhenModelIsImageCapable(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}

	ot := tr.(*OutboundTransformer)

	req := &llm.Request{
		Model:       "gpt-image-1",
		RequestType: llm.RequestTypeImage,
		APIFormat:   llm.APIFormatOpenAIImageGeneration,
		Image: &llm.ImageRequest{
			Prompt: "Create an image of a futuristic city",
		},
	}

	hreq, err := ot.TransformRequest(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "https://api.openai.com/v1/images/generations", hreq.URL)
}

func TestTransformRequest_RoutesToChatCompletions_WhenTextOnly(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}

	ot := tr.(*OutboundTransformer)

	req := &llm.Request{
		Model: "gpt-4o-mini",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
		}},
	}

	hreq, err := ot.TransformRequest(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "https://api.openai.com/v1/chat/completions", hreq.URL)
}

// Test Image Generation API (images/generations)

func TestBuildImageGenerateRequest_BasicPrompt(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ot := tr.(*OutboundTransformer)
	req := &llm.Request{
		Model: "dall-e-3",
		Image: &llm.ImageRequest{
			Prompt: "A cute baby sea otter",
		},
	}

	httpReq, err := ot.buildImageGenerateRequest(req, "test-key")
	require.NoError(t, err)
	require.NotNil(t, httpReq)

	// Verify URL
	assert.Equal(t, "https://api.openai.com/v1/images/generations", httpReq.URL)
	assert.Equal(t, http.MethodPost, httpReq.Method)

	// Verify headers
	assert.Equal(t, "application/json", httpReq.Headers.Get("Content-Type"))

	// Verify body
	var body map[string]any

	err = json.Unmarshal(httpReq.Body, &body)
	require.NoError(t, err)
	assert.Equal(t, "A cute baby sea otter", body["prompt"])
	assert.Equal(t, "dall-e-3", body["model"])
	assert.Equal(t, "b64_json", body["response_format"])
}

func TestBuildImageGenerateRequest_WithParameters(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ot := tr.(*OutboundTransformer)
	req := &llm.Request{
		Model:      "gpt-image-1",
		Modalities: []string{"image"},
		APIFormat:  llm.APIFormatOpenAIImageGeneration,
		Image: &llm.ImageRequest{
			Prompt:       "A futuristic city",
			OutputFormat: "png",
			Size:         "1024x1024",
			Quality:      "high",
			Background:   "transparent",
		},
	}

	httpReq, err := ot.buildImageGenerateRequest(req, "test-key")
	require.NoError(t, err)

	// Verify body
	var body map[string]any

	err = json.Unmarshal(httpReq.Body, &body)
	require.NoError(t, err)
	assert.Equal(t, "A futuristic city", body["prompt"])
	assert.Equal(t, "gpt-image-1", body["model"])
	assert.Equal(t, "png", body["output_format"])
	assert.Equal(t, "1024x1024", body["size"])
	assert.Equal(t, "high", body["quality"])
	assert.Equal(t, "transparent", body["background"])
}

func TestBuildImageGenerateRequest_NoPrompt(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ot := tr.(*OutboundTransformer)
	req := &llm.Request{
		Model: "dall-e-3",
		Image: &llm.ImageRequest{
			Prompt: "",
		},
	}

	_, err = ot.buildImageGenerateRequest(req, "test-key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "prompt is required")
}

// Test Image Edit API (images/edits)

func TestBuildImageEditRequest_WithImage(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ot := tr.(*OutboundTransformer)
	// Simple 1x1 red pixel PNG in base64 (decoded to bytes)
	imageData, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFBQIAX8jx0gAAAABJRU5ErkJggg==")

	req := &llm.Request{
		Model:     "gpt-image-1",
		APIFormat: llm.APIFormatOpenAIImageEdit,
		Image: &llm.ImageRequest{
			Prompt: "Make this image brighter",
			Images: [][]byte{imageData},
		},
	}

	httpReq, err := ot.buildImageEditRequest(req, "test-key")
	require.NoError(t, err)
	require.NotNil(t, httpReq)

	// Verify URL
	assert.Equal(t, "https://api.openai.com/v1/images/edits", httpReq.URL)
	assert.Equal(t, http.MethodPost, httpReq.Method)

	// Verify headers - should be multipart/form-data
	assert.Contains(t, httpReq.Headers.Get("Content-Type"), "multipart/form-data")
}

func TestBuildImageEditRequest_NoImage(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ot := tr.(*OutboundTransformer)
	req := &llm.Request{
		Model:     "gpt-image-1",
		APIFormat: llm.APIFormatOpenAIImageEdit,
		Image: &llm.ImageRequest{
			Prompt: "Make this image brighter",
			Images: [][]byte{},
		},
	}

	_, err = ot.buildImageEditRequest(req, "test-key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one image is required")
}

func TestBuildImageEditRequest_NoPrompt(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ot := tr.(*OutboundTransformer)
	imageData, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFBQIAX8jx0gAAAABJRU5ErkJggg==")

	req := &llm.Request{
		Model:     "gpt-image-1",
		APIFormat: llm.APIFormatOpenAIImageEdit,
		Image: &llm.ImageRequest{
			Prompt: "",
			Images: [][]byte{imageData},
		},
	}

	_, err = ot.buildImageEditRequest(req, "test-key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "prompt is required")
}

// Test buildImageGenerationAPIRequest routing

func TestBuildImageGenerationAPIRequest_RoutesToGenerate(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ot := tr.(*OutboundTransformer)
	req := &llm.Request{
		Model: "dall-e-3",
		Image: &llm.ImageRequest{
			Prompt: "A sunset",
		},
	}

	httpReq, err := ot.buildImageGenerationAPIRequest(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "https://api.openai.com/v1/images/generations", httpReq.URL)
	assert.Equal(t, string(llm.APIFormatOpenAIImageGeneration), httpReq.APIFormat)
	assert.Equal(t, "dall-e-3", httpReq.TransformerMetadata["model"])
}

func TestBuildImageGenerationAPIRequest_RoutesToGeneration(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ot := tr.(*OutboundTransformer)

	req := &llm.Request{
		Model: "gpt-image-1",
		Image: &llm.ImageRequest{
			Prompt: "Generate an image of a cat",
		},
	}

	httpReq, err := ot.buildImageGenerationAPIRequest(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "https://api.openai.com/v1/images/generations", httpReq.URL)
	assert.Equal(t, string(llm.APIFormatOpenAIImageGeneration), httpReq.APIFormat)
	assert.Equal(t, "gpt-image-1", httpReq.TransformerMetadata["model"])
}

// Test response transformation

func TestTransformImageGenerationResponse_BasicResponse(t *testing.T) {
	body := []byte(`{
		"created": 1730000000,
		"data": [
			{
				"b64_json": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
			}
		]
	}`)

	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	}

	resp, err := transformImageGenerationResponse(httpResp)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, "img-1730000000", resp.ID)
	assert.Equal(t, "chat.completion", resp.Object)
	assert.Equal(t, int64(1730000000), resp.Created)
	// Image response should not have Choices
	assert.Empty(t, resp.Choices)
	// Image response should have Image field
	require.NotNil(t, resp.Image)
	assert.Len(t, resp.Image.Data, 1)
	assert.NotEmpty(t, resp.Image.Data[0].B64JSON)
}

func TestTransformImageGenerationResponse_WithUsage(t *testing.T) {
	body := []byte(`{
		"created": 1730000000,
		"data": [
			{
				"b64_json": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
			}
		],
		"usage": {
			"input_tokens": 10,
			"output_tokens": 256,
			"total_tokens": 266,
			"input_tokens_details": {
				"image_tokens": 0,
				"text_tokens": 10
			}
		}
	}`)

	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	}

	resp, err := transformImageGenerationResponse(httpResp)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Usage)

	assert.Equal(t, int64(10), resp.Usage.PromptTokens)
	assert.Equal(t, int64(256), resp.Usage.CompletionTokens)
	assert.Equal(t, int64(266), resp.Usage.TotalTokens)
}

func TestTransformImageGenerationResponse_MultipleImages(t *testing.T) {
	body := []byte(`{
		"created": 1730000000,
		"data": [
			{
				"b64_json": "image1data"
			},
			{
				"b64_json": "image2data"
			}
		],
		"output_format": "webp"
	}`)

	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	}

	resp, err := transformImageGenerationResponse(httpResp)
	require.NoError(t, err)
	// Image response should not have Choices
	assert.Empty(t, resp.Choices)
	// Verify Image field
	require.NotNil(t, resp.Image)
	assert.Len(t, resp.Image.Data, 2)
	assert.Equal(t, "image1data", resp.Image.Data[0].B64JSON)
	assert.Equal(t, "image2data", resp.Image.Data[1].B64JSON)
}

func TestTransformImageGenerationResponse_WithModelInTransformerMetadata(t *testing.T) {
	body := []byte(`{
		"created": 1730000000,
		"data": [
			{
				"b64_json": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
			}
		]
	}`)

	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Request: &httpclient.Request{
			TransformerMetadata: map[string]any{
				"model": "dall-e-3",
			},
		},
	}

	resp, err := transformImageGenerationResponse(httpResp)
	require.NoError(t, err)
	assert.Equal(t, "dall-e-3", resp.Model)
}

func TestTransformImageGenerationResponse_WithoutModelInTransformerMetadata(t *testing.T) {
	body := []byte(`{
		"created": 1730000000,
		"data": [
			{
				"b64_json": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
			}
		]
	}`)

	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	}

	resp, err := transformImageGenerationResponse(httpResp)
	require.NoError(t, err)
	assert.Equal(t, "image-generation", resp.Model)
}

// Test extractImageData

func TestExtractImageData_ValidDataURL(t *testing.T) {
	dataURL := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFBQIAX8jx0gAAAABJRU5ErkJggg=="
	formFile, err := extractFile(dataURL)
	require.NoError(t, err)
	assert.NotEmpty(t, formFile.Data)
	assert.Equal(t, "png", formFile.Format)
	assert.Equal(t, "image/png", formFile.ContentType)
}

func TestExtractImageData_InvalidDataURL(t *testing.T) {
	dataURL := "data:image/png;base64"
	_, err := extractFile(dataURL)
	assert.Error(t, err)
	// This will fail because there's no comma separator
	assert.Contains(t, err.Error(), "invalid data URL format")
}

func TestExtractImageData_NonDataURL(t *testing.T) {
	url := "https://example.com/image.png"
	_, err := extractFile(url)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only data URLs are supported")
}

func TestExtractImageData_JPEGFormat(t *testing.T) {
	dataURL := "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQEAYABgAAD/2wBDAAEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQH/2wBDAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQH/wAARCAABAAEDASIAAhEBAxEB/8QAFQABAQAAAAAAAAAAAAAAAAAAAAv/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/8QAFQEBAQAAAAAAAAAAAAAAAAAAAAX/xAAUEQEAAAAAAAAAAAAAAAAAAAAA/9oADAMBAAIRAxEAPwA/8A8A"
	formFile, err := extractFile(dataURL)
	require.NoError(t, err)
	assert.NotEmpty(t, formFile.Data)
	assert.Equal(t, "jpeg", formFile.Format)
	assert.Equal(t, "image/jpeg", formFile.ContentType)
}

// Integration test with TransformRequest

func TestTransformRequest_ImageGeneration_Integration(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ot := tr.(*OutboundTransformer)
	req := &llm.Request{
		Model:       "dall-e-3",
		RequestType: llm.RequestTypeImage,
		Image: &llm.ImageRequest{
			Prompt: "A beautiful landscape",
		},
	}

	httpReq, err := ot.TransformRequest(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "https://api.openai.com/v1/images/generations", httpReq.URL)
	assert.Equal(t, string(llm.APIFormatOpenAIImageGeneration), httpReq.APIFormat)
}

func TestTransformResponse_ImageGeneration_Integration(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ot := tr.(*OutboundTransformer)
	body := []byte(`{
		"created": 1730000000,
		"data": [
			{
				"b64_json": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
			}
		]
	}`)

	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Request: &httpclient.Request{
			APIFormat: string(llm.APIFormatOpenAIImageGeneration),
		},
	}

	resp, err := ot.TransformResponse(context.Background(), httpResp)
	require.NoError(t, err)
	require.NotNil(t, resp)
	// Image generation responses now use resp.Image instead of resp.Choices
	assert.Empty(t, resp.Choices)
	require.NotNil(t, resp.Image)
	require.Len(t, resp.Image.Data, 1)
	assert.NotEmpty(t, resp.Image.Data[0].B64JSON)
}
