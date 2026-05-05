package codex

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

const (
	defaultImageMainModel = "gpt-5.4-mini"
	defaultImageToolModel = "gpt-image-2"
)

func codexImageMainModel() string {
	if model := strings.TrimSpace(os.Getenv("AXONHUB_CODEX_IMAGE_MAIN_MODEL")); model != "" {
		return model
	}

	return defaultImageMainModel
}

func (t *OutboundTransformer) transformImageRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq.Image == nil {
		return nil, errors.New("image request is required")
	}
	if llmReq.Image.N != nil && *llmReq.Image.N > 1 {
		return nil, fmt.Errorf("codex image generation supports n=1 only, got %d", *llmReq.Image.N)
	}

	toolModel := strings.TrimSpace(llmReq.Model)
	if toolModel == "" {
		toolModel = defaultImageToolModel
	}

	imageTool := &llm.ImageGeneration{
		Model:             toolModel,
		Background:        llmReq.Image.Background,
		InputFidelity:     llmReq.Image.InputFidelity,
		Moderation:        llmReq.Image.Moderation,
		OutputCompression: llmReq.Image.OutputCompression,
		OutputFormat:      llmReq.Image.OutputFormat,
		PartialImages:     llmReq.Image.PartialImages,
		N:                 llmReq.Image.N,
		ResponseFormat:    llmReq.Image.ResponseFormat,
		Quality:           llmReq.Image.Quality,
		Size:              llmReq.Image.Size,
		Style:             llmReq.Image.Style,
	}

	if len(llmReq.Image.Mask) > 0 {
		imageTool.InputImageMask = map[string]any{
			"image_url": imageDataURL(llmReq.Image.Mask),
		}
	}

	contentParts := []llm.MessageContentPart{}
	if llmReq.Image.Prompt != "" {
		contentParts = append(contentParts, llm.MessageContentPart{
			Type: "text",
			Text: lo.ToPtr(llmReq.Image.Prompt),
		})
	}

	for _, image := range llmReq.Image.Images {
		if len(image) == 0 {
			continue
		}

		url := imageDataURL(image)
		contentParts = append(contentParts, llm.MessageContentPart{
			Type: "image_url",
			ImageURL: &llm.ImageURL{
				URL: url,
			},
		})
	}

	if len(contentParts) == 0 {
		return nil, errors.New("prompt or image is required for codex image request")
	}

	imageReq := *llmReq
	imageReq.TransformerMetadata = cloneTransformerMetadata(llmReq.TransformerMetadata)
	imageReq.Model = codexImageMainModel()
	imageReq.RequestType = llm.RequestTypeChat
	imageReq.APIFormat = llm.APIFormatOpenAIResponse
	imageReq.Messages = []llm.Message{{
		Role: "user",
		Content: llm.MessageContent{
			MultipleContent: contentParts,
		},
	}}
	imageReq.Tools = []llm.Tool{{
		Type:            llm.ToolTypeImageGeneration,
		ImageGeneration: imageTool,
	}}
	imageReq.ToolChoice = &llm.ToolChoice{
		NamedToolChoice: &llm.NamedToolChoice{
			Type: llm.ToolTypeImageGeneration,
		},
	}
	imageReq.Stream = lo.ToPtr(true)
	imageReq.Store = lo.ToPtr(false)
	imageReq.ParallelToolCalls = lo.ToPtr(true)
	imageReq.TransformOptions.ArrayInputs = lo.ToPtr(true)

	if imageTool.OutputFormat != "" {
		imageReq.TransformerMetadata["image_output_format"] = imageTool.OutputFormat
	}

	hreq, err := t.TransformRequest(ctx, &imageReq)
	if err != nil {
		return nil, err
	}

	hreq.RequestType = llm.RequestTypeImage.String()
	hreq.APIFormat = string(llm.APIFormatOpenAIResponse)
	hreq.TransformerMetadata["codex_image_request_model"] = toolModel

	return hreq, nil
}

func cloneTransformerMetadata(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}

	return dst
}

func imageDataURL(data []byte) string {
	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") {
		contentType = "image/png"
	}

	return fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(data))
}

func (t *OutboundTransformer) transformImageResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	resp, err := t.responsesOutbound.TransformResponse(ctx, httpResp)
	if err != nil {
		return nil, err
	}

	if resp == nil {
		return nil, errors.New("codex image response is nil")
	}

	imageResp := &llm.ImageResponse{
		Created: time.Now().Unix(),
		Data:    []llm.ImageData{},
	}

	if resp.Created != 0 {
		imageResp.Created = resp.Created
	}

	for _, choice := range resp.Choices {
		if choice.Message == nil {
			continue
		}

		for _, part := range choice.Message.Content.MultipleContent {
			if part.Type != "image_url" || part.ImageURL == nil {
				continue
			}

			format, encoded := splitImageDataURL(part.ImageURL.URL)
			if encoded == "" {
				continue
			}

			imageResp.Data = append(imageResp.Data, llm.ImageData{
				B64JSON: encoded,
			})

			if format != "" && imageResp.OutputFormat == "" {
				imageResp.OutputFormat = format
			}

			if part.TransformerMetadata != nil {
				if v, ok := part.TransformerMetadata["background"].(*string); ok && v != nil {
					imageResp.Background = *v
				}
				if v, ok := part.TransformerMetadata["output_format"].(*string); ok && v != nil {
					imageResp.OutputFormat = *v
				}
				if v, ok := part.TransformerMetadata["quality"].(*string); ok && v != nil {
					imageResp.Quality = *v
				}
				if v, ok := part.TransformerMetadata["size"].(*string); ok && v != nil {
					imageResp.Size = *v
				}
			}
		}

		if choice.Delta != nil {
			for _, part := range choice.Delta.Content.MultipleContent {
				if part.Type != "image_url" || part.ImageURL == nil {
					continue
				}

				format, encoded := splitImageDataURL(part.ImageURL.URL)
				if encoded == "" {
					continue
				}

				imageResp.Data = append(imageResp.Data, llm.ImageData{
					B64JSON: encoded,
				})

				if format != "" && imageResp.OutputFormat == "" {
					imageResp.OutputFormat = format
				}
			}
		}
	}

	if len(imageResp.Data) == 0 {
		return nil, errors.New("codex image response did not include image_generation_call result")
	}

	resp.RequestType = llm.RequestTypeImage
	resp.Image = imageResp
	resp.Choices = nil

	return resp, nil
}

func splitImageDataURL(url string) (string, string) {
	const prefix = "data:image/"
	if !strings.HasPrefix(url, prefix) {
		return "", ""
	}

	header, encoded, ok := strings.Cut(url, ",")
	if !ok {
		return "", ""
	}

	format := strings.TrimPrefix(header, prefix)
	if before, _, ok := strings.Cut(format, ";"); ok {
		format = before
	}

	return format, encoded
}
