package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestTransformRequest_RoutesToImageGenerationAPI_WhenImageToolPresent(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}

	ot := tr.(*OutboundTransformer)
	req := &llm.Request{
		Model: "gpt-4o-mini",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("Generate a beautiful sunset over mountains")},
		}},
		Tools: []llm.Tool{{
			Type: "image_generation",
			ImageGeneration: &llm.ImageGeneration{
				OutputFormat: "png",
				Size:         "1024x1024",
			},
		}},
		Modalities: []string{"image"},
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
		Model: "gpt-image-1",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("Create an image of a futuristic city")},
		}},
		Modalities: []string{"image"},
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

func TestTransformResponse_FromResponsesAPI_ImageResult(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}

	ot := tr.(*OutboundTransformer)

	body := []byte(`{
		"id":"resp_123",
		"object":"response",
		"created_at": 1730000000,
		"model":"gpt-image-1",
		"status":"completed",
		"output":[
			{
				"id":"item_123",
				"type":"image_generation_call",
				"result":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
			}
		]
	}`)

	// Add TransformerMetadata to indicate this is a responses API call
	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Request: &httpclient.Request{
			TransformerMetadata: map[string]any{
				"outbound_format_type": "openai/responses",
			},
		},
	}

	resp, err := ot.TransformResponse(context.Background(), httpResp)
	assert.NoError(t, err)

	if assert.Len(t, resp.Choices, 1) {
		choice := resp.Choices[0]
		assert.NotNil(t, choice.Message)

		parts := choice.Message.Content.MultipleContent
		if assert.NotEmpty(t, parts) {
			assert.Equal(t, "image_url", parts[0].Type)

			if assert.NotNil(t, parts[0].ImageURL) {
				assert.Contains(t, parts[0].ImageURL.URL, "data:image/png;base64,")
			}
		}
	}
}

func TestTransformResponse_FromResponsesAPI_ImageResult_WithMeta(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}

	ot := tr.(*OutboundTransformer)

	body := []byte(`{
		"id":"resp_123",
		"object":"response",
		"created_at": 1730000000,
		"model":"gpt-image-1",
		"status":"completed",
		"output":[
			{
				"id":"item_123",
				"type":"image_generation_call",
				"result":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
			}
		],
		"usage":{
			"input_tokens":10,
			"output_tokens":0,
			"total_tokens":10
		}
	}`)

	// Add TransformerMetadata to indicate this is a responses API call
	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Request: &httpclient.Request{
			TransformerMetadata: map[string]any{
				"outbound_format_type": "openai/responses",
			},
		},
	}

	resp, err := ot.TransformResponse(context.Background(), httpResp)
	assert.NoError(t, err)

	// Test that meta information is preserved
	assert.Equal(t, "resp_123", resp.ID)
	assert.Equal(t, "gpt-image-1", resp.Model)
	assert.Equal(t, int64(1730000000), resp.Created)
	assert.NotNil(t, resp.Usage)
	assert.Equal(t, int64(10), resp.Usage.PromptTokens)
	assert.Equal(t, int64(0), resp.Usage.CompletionTokens)
	assert.Equal(t, int64(10), resp.Usage.TotalTokens)
}

func TestTransformResponse_DebugResponsesAPI(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}

	ot := tr.(*OutboundTransformer)

	// Simple debugging test to understand the response structure
	body := []byte(`{
		"id":"resp_123",
		"object":"response",
		"created_at": 1730000000,
		"model":"gpt-image-1",
		"status":"completed",
		"output":[
			{
				"id":"item_123",
				"type":"image_generation_call",
				"result":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
			}
		]
	}`)

	// Add TransformerMetadata to indicate this is a responses API call
	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Request: &httpclient.Request{
			TransformerMetadata: map[string]any{
				"outbound_format_type": "openai/responses",
			},
		},
	}

	resp, err := ot.TransformResponse(context.Background(), httpResp)
	assert.NoError(t, err)

	t.Logf("Response ID: %s", resp.ID)
	t.Logf("Response Model: %s", resp.Model)
	t.Logf("Response Created: %d", resp.Created)
	t.Logf("Number of Choices: %d", len(resp.Choices))
	t.Logf("Response Object: %s", resp.Object)

	if len(resp.Choices) > 0 {
		t.Logf("First Choice Index: %d", resp.Choices[0].Index)
		t.Logf("First Choice Message: %+v", resp.Choices[0].Message)

		if resp.Choices[0].Message != nil {
			t.Logf("Message Content: %+v", resp.Choices[0].Message.Content)
		}
	}
}

func TestTransformRequest_BuildsCorrectMetadata(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}

	ot := tr.(*OutboundTransformer)
	req := &llm.Request{
		Model: "gpt-4o-mini",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("Generate a beautiful sunset over mountains")},
		}},
		Tools: []llm.Tool{{
			Type: "image_generation",
			ImageGeneration: &llm.ImageGeneration{
				OutputFormat: "jpeg",
				Size:         "1024x1024",
			},
		}},
		Modalities: []string{"image"},
	}

	hreq, err := ot.buildResponsesAPIRequest(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, hreq)
	assert.Equal(t, "openai/responses", hreq.TransformerMetadata["outbound_format_type"])
}

// Test Image Generation API (images/generations)

func TestBuildImageGenerateRequest_BasicPrompt(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ot := tr.(*OutboundTransformer)
	req := &llm.Request{
		Model: "dall-e-3",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("A cute baby sea otter")},
		}},
		Modalities: []string{"image"},
	}

	httpReq, err := ot.buildImageGenerateRequest(req)
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
		Model: "gpt-image-1",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("A futuristic city")},
		}},
		Tools: []llm.Tool{{
			Type: "image_generation",
			ImageGeneration: &llm.ImageGeneration{
				OutputFormat: "png",
				Size:         "1024x1024",
				Quality:      "high",
				Background:   "transparent",
			},
		}},
		Modalities: []string{"image"},
		User:       lo.ToPtr("user-123"),
	}

	httpReq, err := ot.buildImageGenerateRequest(req)
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
	assert.Equal(t, "user-123", body["user"])
}

func TestBuildImageGenerateRequest_NoPrompt(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ot := tr.(*OutboundTransformer)
	req := &llm.Request{
		Model:      "dall-e-3",
		Messages:   []llm.Message{},
		Modalities: []string{"image"},
	}

	_, err = ot.buildImageGenerateRequest(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "prompt is required")
}

// Test Image Edit API (images/edits)

func TestBuildImageEditRequest_WithImage(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ot := tr.(*OutboundTransformer)
	// Simple 1x1 red pixel PNG in base64
	imageData := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFBQIAX8jx0gAAAABJRU5ErkJggg=="

	req := &llm.Request{
		Model: "gpt-image-1",
		Messages: []llm.Message{{
			Role: "user",
			Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{
						Type: "text",
						Text: lo.ToPtr("Make this image brighter"),
					},
					{
						Type: "image_url",
						ImageURL: &llm.ImageURL{
							URL: imageData,
						},
					},
				},
			},
		}},
		Modalities: []string{"image"},
	}

	httpReq, err := ot.buildImageEditRequest(req)
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
		Model: "gpt-image-1",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("Make this image brighter")},
		}},
		Modalities: []string{"image"},
	}

	_, err = ot.buildImageEditRequest(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one image is required")
}

func TestBuildImageEditRequest_NoPrompt(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ot := tr.(*OutboundTransformer)
	imageData := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFBQIAX8jx0gAAAABJRU5ErkJggg=="

	req := &llm.Request{
		Model: "gpt-image-1",
		Messages: []llm.Message{{
			Role: "user",
			Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{
						Type: "image_url",
						ImageURL: &llm.ImageURL{
							URL: imageData,
						},
					},
				},
			},
		}},
		Modalities: []string{"image"},
	}

	_, err = ot.buildImageEditRequest(req)
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
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("A sunset")},
		}},
		Modalities: []string{"image"},
	}

	httpReq, err := ot.buildImageGenerationAPIRequest(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "https://api.openai.com/v1/images/generations", httpReq.URL)
	assert.Equal(t, llm.APIFormatOpenAIImageGeneration.String(), httpReq.TransformerMetadata["outbound_format_type"])
	assert.Equal(t, "dall-e-3", httpReq.TransformerMetadata["model"])
}

func TestBuildImageGenerationAPIRequest_RoutesToEdit(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ot := tr.(*OutboundTransformer)
	imageData := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFBQIAX8jx0gAAAABJRU5ErkJggg=="

	req := &llm.Request{
		Model: "gpt-image-1",
		Messages: []llm.Message{{
			Role: "user",
			Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{
						Type: "text",
						Text: lo.ToPtr("Edit this"),
					},
					{
						Type: "image_url",
						ImageURL: &llm.ImageURL{
							URL: imageData,
						},
					},
				},
			},
		}},
		Modalities: []string{"image"},
	}

	httpReq, err := ot.buildImageGenerationAPIRequest(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "https://api.openai.com/v1/images/edits", httpReq.URL)
	assert.Equal(t, llm.APIFormatOpenAIImageGeneration.String(), httpReq.TransformerMetadata["outbound_format_type"])
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
	assert.Len(t, resp.Choices, 1)

	choice := resp.Choices[0]
	assert.Equal(t, 0, choice.Index)
	assert.NotNil(t, choice.Message)
	assert.Equal(t, "assistant", choice.Message.Role)
	assert.Len(t, choice.Message.Content.MultipleContent, 1)
	assert.Equal(t, "image_url", choice.Message.Content.MultipleContent[0].Type)
	assert.Contains(t, choice.Message.Content.MultipleContent[0].ImageURL.URL, "data:image/png;base64,")
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
	assert.Len(t, resp.Choices, 2)

	// Verify first image
	assert.Equal(t, 0, resp.Choices[0].Index)
	assert.Contains(t, resp.Choices[0].Message.Content.MultipleContent[0].ImageURL.URL, "data:image/webp;base64,image1data")

	// Verify second image
	assert.Equal(t, 1, resp.Choices[1].Index)
	assert.Contains(t, resp.Choices[1].Message.Content.MultipleContent[0].ImageURL.URL, "data:image/webp;base64,image2data")
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

// Test hasImagesInMessages

func TestHasImagesInMessages_WithImages(t *testing.T) {
	messages := []llm.Message{
		{
			Role: "user",
			Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{
						Type: "text",
						Text: lo.ToPtr("Edit this image"),
					},
					{
						Type: "image_url",
						ImageURL: &llm.ImageURL{
							URL: "data:image/png;base64,abc123",
						},
					},
				},
			},
		},
	}

	assert.True(t, hasImagesInMessages(messages))
}

func TestHasImagesInMessages_WithoutImages(t *testing.T) {
	messages := []llm.Message{
		{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("Generate an image")},
		},
	}

	assert.False(t, hasImagesInMessages(messages))
}

func TestHasImagesInMessages_WithTextOnly(t *testing.T) {
	messages := []llm.Message{
		{
			Role: "user",
			Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{
						Type: "text",
						Text: lo.ToPtr("Just text"),
					},
				},
			},
		},
	}

	assert.False(t, hasImagesInMessages(messages))
}

func TestHasImagesInMessages_EmptyMessages(t *testing.T) {
	messages := []llm.Message{}
	assert.False(t, hasImagesInMessages(messages))
}

func TestHasImagesInMessages_MultipleMessagesWithImage(t *testing.T) {
	messages := []llm.Message{
		{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("First message")},
		},
		{
			Role: "user",
			Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{
						Type: "image_url",
						ImageURL: &llm.ImageURL{
							URL: "data:image/png;base64,xyz789",
						},
					},
				},
			},
		},
	}

	assert.True(t, hasImagesInMessages(messages))
}

// Test extractPromptFromMessages

func TestExtractPromptFromMessages_SimpleContent(t *testing.T) {
	messages := []llm.Message{
		{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("Generate a sunset")},
		},
	}

	prompt, err := extractPromptFromMessages(messages)
	require.NoError(t, err)
	assert.Equal(t, "Generate a sunset", prompt)
}

func TestExtractPromptFromMessages_MultipleContent(t *testing.T) {
	messages := []llm.Message{
		{
			Role: "user",
			Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{
						Type: "text",
						Text: lo.ToPtr("Edit this image"),
					},
					{
						Type: "image_url",
						ImageURL: &llm.ImageURL{
							URL: "data:image/png;base64,abc123",
						},
					},
				},
			},
		},
	}

	prompt, err := extractPromptFromMessages(messages)
	require.NoError(t, err)
	assert.Equal(t, "Edit this image", prompt)
}

func TestExtractPromptFromMessages_NoPrompt(t *testing.T) {
	messages := []llm.Message{
		{
			Role: "user",
			Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{
						Type: "image_url",
						ImageURL: &llm.ImageURL{
							URL: "data:image/png;base64,abc123",
						},
					},
				},
			},
		},
	}

	_, err := extractPromptFromMessages(messages)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "prompt is required")
}

func TestExtractPromptFromMessages_EmptyMessages(t *testing.T) {
	messages := []llm.Message{}

	_, err := extractPromptFromMessages(messages)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "prompt is required")
}

func TestExtractPromptFromMessages_MultipleMessages(t *testing.T) {
	messages := []llm.Message{
		{
			Role: "system",
			Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{
						Type: "image_url",
						ImageURL: &llm.ImageURL{
							URL: "data:image/png;base64,abc123",
						},
					},
				},
			},
		},
		{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("Second message prompt")},
		},
	}

	prompt, err := extractPromptFromMessages(messages)
	require.NoError(t, err)
	assert.Equal(t, "Second message prompt", prompt)
}

// Integration test with TransformRequest

func TestTransformRequest_ImageGeneration_Integration(t *testing.T) {
	tr, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)

	ot := tr.(*OutboundTransformer)
	req := &llm.Request{
		Model: "dall-e-3",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("A beautiful landscape")},
		}},
		Modalities: []string{"image"},
	}

	httpReq, err := ot.TransformRequest(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "https://api.openai.com/v1/images/generations", httpReq.URL)
	assert.Equal(t, llm.APIFormatOpenAIImageGeneration.String(), httpReq.TransformerMetadata["outbound_format_type"])
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
			TransformerMetadata: map[string]any{
				"outbound_format_type": llm.APIFormatOpenAIImageGeneration.String(),
			},
		},
	}

	resp, err := ot.TransformResponse(context.Background(), httpResp)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Choices, 1)
	assert.Equal(t, "image_url", resp.Choices[0].Message.Content.MultipleContent[0].Type)
}
