package responses

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

// TestBuildImageResponse_FromRequestPipeline drives the full request→response
// path: buildImageToolRequest must persist the requested image options into the
// metadata under the keys BuildImageResponse reads, so the client-facing
// response echoes them back. This guards against the request/response metadata
// key mismatch that direct-injection tests cannot catch.
func TestBuildImageResponse_FromRequestPipeline(t *testing.T) {
	src := &llm.Request{
		Model:       "gpt-image-1",
		RequestType: llm.RequestTypeImage,
		APIFormat:   llm.APIFormatOpenAIImageGeneration,
		Image: &llm.ImageRequest{
			Prompt:       "a cat",
			OutputFormat: "webp",
			Quality:      "high",
			Size:         "1024x1024",
			Background:   "opaque",
		},
	}

	toolReq, err := buildImageToolRequest(src)
	require.NoError(t, err)

	upstream := &Response{
		ID:        "resp_img_pipeline",
		CreatedAt: 1760000000,
		Model:     "gpt-image-1",
		Output: []Item{
			{Type: "image_generation_call", Result: lo.ToPtr("iVBORw0KGgo=")},
		},
	}

	resp, err := BuildImageResponse(upstream, toolReq.TransformerMetadata)
	require.NoError(t, err)
	require.NotNil(t, resp.Image)
	require.Equal(t, "webp", resp.Image.OutputFormat)
	require.Equal(t, "high", resp.Image.Quality)
	require.Equal(t, "1024x1024", resp.Image.Size)
	require.Equal(t, "opaque", resp.Image.Background)
}

func TestBuildImageResponse_Success(t *testing.T) {
	upstream := &Response{
		ID:        "resp_img_001",
		CreatedAt: 1760000000,
		Model:     "gpt-image-2",
		Output: []Item{
			{
				Type:   "image_generation_call",
				Result: lo.ToPtr("iVBORw0KGgo="),
			},
		},
	}

	resp, err := BuildImageResponse(upstream, map[string]any{
		"codex_image_output_format": "png",
		"codex_image_quality":       "high",
		"codex_image_size":          "1024x1024",
		"codex_image_background":    "opaque",
	})
	require.NoError(t, err)
	require.Equal(t, llm.RequestTypeImage, resp.RequestType)
	require.Equal(t, "resp_img_001", resp.ID)
	require.Equal(t, "gpt-image-2", resp.Model)
	require.NotNil(t, resp.Image)
	require.Equal(t, "png", resp.Image.OutputFormat)
	require.Equal(t, "high", resp.Image.Quality)
	require.Equal(t, "1024x1024", resp.Image.Size)
	require.Equal(t, "opaque", resp.Image.Background)
	require.Len(t, resp.Image.Data, 1)
	require.Equal(t, "iVBORw0KGgo=", resp.Image.Data[0].B64JSON)
}

func TestBuildImageResponse_MultipleImages(t *testing.T) {
	upstream := &Response{
		ID:        "resp_img_multi",
		CreatedAt: 1760000000,
		Model:     "gpt-image-2",
		Output: []Item{
			{Type: "image_generation_call", Result: lo.ToPtr("image1base64")},
			{Type: "image_generation_call", Result: lo.ToPtr("image2base64")},
		},
	}

	resp, err := BuildImageResponse(upstream, map[string]any{
		"codex_image_output_format": "webp",
	})
	require.NoError(t, err)
	require.Len(t, resp.Image.Data, 2)
	require.Equal(t, "image1base64", resp.Image.Data[0].B64JSON)
	require.Equal(t, "image2base64", resp.Image.Data[1].B64JSON)
	require.Equal(t, "webp", resp.Image.OutputFormat)
}

func TestBuildImageResponse_SkipsNonImageItems(t *testing.T) {
	upstream := &Response{
		ID:        "resp_mixed",
		CreatedAt: 1760000000,
		Model:     "gpt-image-2",
		Output: []Item{
			{Type: "message", ID: "msg_1"},
			{Type: "image_generation_call", Result: lo.ToPtr("base64only")},
			{Type: "function_call", ID: "fn_1"},
		},
	}

	resp, err := BuildImageResponse(upstream, map[string]any{
		"codex_image_output_format": "png",
	})
	require.NoError(t, err)
	require.Len(t, resp.Image.Data, 1)
	require.Equal(t, "base64only", resp.Image.Data[0].B64JSON)
}

func TestBuildImageResponse_NilResponse(t *testing.T) {
	_, err := BuildImageResponse(nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "response is nil")
}

func TestBuildImageResponse_ResponseError(t *testing.T) {
	upstream := &Response{
		ID:    "resp_err",
		Error: &Error{Message: "generation failed"},
	}

	_, err := BuildImageResponse(upstream, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "generation failed")
}

func TestBuildImageResponse_NoImageResult(t *testing.T) {
	upstream := &Response{
		ID:     "resp_empty",
		Output: []Item{},
	}

	_, err := BuildImageResponse(upstream, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "did not contain image_generation_call result")
}

func TestBuildImageResponse_ModelFromMetadata(t *testing.T) {
	upstream := &Response{
		ID:        "resp_no_model",
		CreatedAt: 1760000000,
		Output: []Item{
			{Type: "image_generation_call", Result: lo.ToPtr("base64data")},
		},
	}

	resp, err := BuildImageResponse(upstream, map[string]any{
		"codex_image_model": "gpt-image-1",
	})
	require.NoError(t, err)
	require.Equal(t, "gpt-image-1", resp.Model)
}

func TestBuildImageResponse_DefaultOutputFormat(t *testing.T) {
	upstream := &Response{
		ID:        "resp_default",
		CreatedAt: 1760000000,
		Output: []Item{
			{Type: "image_generation_call", Result: lo.ToPtr("data")},
		},
	}

	resp, err := BuildImageResponse(upstream, nil)
	require.NoError(t, err)
	require.Equal(t, "png", resp.Image.OutputFormat)
}

func TestBuildImageResponse_ExtractsBase64FromDataURL(t *testing.T) {
	upstream := &Response{
		ID:        "resp_dataurl",
		CreatedAt: 1760000000,
		Output: []Item{
			{Type: "image_generation_call", Result: lo.ToPtr("data:image/png;base64,realb64data")},
		},
	}

	resp, err := BuildImageResponse(upstream, nil)
	require.NoError(t, err)
	require.Equal(t, "realb64data", resp.Image.Data[0].B64JSON)
}

func TestBuildImageResponse_SkipsEmptyResult(t *testing.T) {
	upstream := &Response{
		ID:        "resp_empty_result",
		CreatedAt: 1760000000,
		Output: []Item{
			{Type: "image_generation_call", Result: lo.ToPtr("")},
			{Type: "image_generation_call", Result: lo.ToPtr("valid")},
		},
	}

	resp, err := BuildImageResponse(upstream, nil)
	require.NoError(t, err)
	require.Len(t, resp.Image.Data, 1)
	require.Equal(t, "valid", resp.Image.Data[0].B64JSON)
}

func TestBuildImageToolRequest_PreservesSystemMessages(t *testing.T) {
	src := &llm.Request{
		Model:       "gpt-image-1",
		RequestType: llm.RequestTypeImage,
		APIFormat:   llm.APIFormatOpenAIImageGeneration,
		Messages: []llm.Message{
			{
				Role: "system",
				Content: llm.MessageContent{
					Content: lo.ToPtr("system instruction"),
				},
			},
		},
		Image: &llm.ImageRequest{
			Prompt: "a cat",
		},
	}

	toolReq, err := buildImageToolRequest(src)
	require.NoError(t, err)
	require.Len(t, toolReq.Messages, 3)
	require.Equal(t, "system", toolReq.Messages[0].Role)
	require.Equal(t, "You are a helpful assistant that can generate images based on user requests. Must use the image generation tool.", *toolReq.Messages[0].Content.Content)
	require.Equal(t, "system", toolReq.Messages[1].Role)
	require.Equal(t, "system instruction", *toolReq.Messages[1].Content.Content)
	require.Equal(t, "user", toolReq.Messages[2].Role)
}
