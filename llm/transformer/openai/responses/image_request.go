package responses

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xurl"
	"github.com/looplj/axonhub/llm/transformer"
)

const ImageGenerationToolModelMetadataKey = "image_generation_tool_model"

func buildImageToolRequest(src *llm.Request) (*llm.Request, error) {
	if src == nil || src.Image == nil {
		return nil, fmt.Errorf("%w: image request is required", transformer.ErrInvalidRequest)
	}

	if src.APIFormat != llm.APIFormatOpenAIImageGeneration && src.APIFormat != llm.APIFormatOpenAIImageEdit {
		return nil, fmt.Errorf("%w: responses image tool only supports generation and edit", transformer.ErrInvalidRequest)
	}

	prompt := src.Image.Prompt
	if prompt == "" {
		return nil, fmt.Errorf("%w: prompt is required", transformer.ErrInvalidRequest)
	}

	if src.Image.N != nil && *src.Image.N > 1 {
		return nil, fmt.Errorf("%w: responses image tool does not support n greater than 1", transformer.ErrInvalidRequest)
	}

	action := "generate"
	imageURLs := imageDataURLs(src)
	if src.APIFormat == llm.APIFormatOpenAIImageEdit {
		action = "edit"
		if len(imageURLs) == 0 {
			return nil, fmt.Errorf("%w: at least one image is required for edits", transformer.ErrInvalidRequest)
		}
	}

	content := []llm.MessageContentPart{
		{
			Type: "text",
			Text: lo.ToPtr(prompt),
		},
	}

	for _, imageURL := range imageURLs {
		content = append(content, llm.MessageContentPart{
			Type: "image_url",
			ImageURL: &llm.ImageURL{
				URL: imageURL,
			},
		})
	}

	// Always include a default system instruction and preserve any system
	// messages from the source request so they are emitted as top-level
	// instructions in the Responses API payload.
	messages := []llm.Message{
		{
			Role: "system",
			Content: llm.MessageContent{
				Content: lo.ToPtr("You are a helpful assistant that can generate images based on user requests. Must use the image generation tool."),
			},
		},
	}
	for _, msg := range src.Messages {
		if msg.Role == "system" {
			messages = append(messages, msg)
		}
	}
	messages = append(messages, llm.Message{
		Role: "user",
		Content: llm.MessageContent{
			MultipleContent: content,
		},
	})

	metadata := map[string]any{}
	maps.Copy(metadata, src.TransformerMetadata)

	toolModel := src.Model
	if model, ok := metadata[ImageGenerationToolModelMetadataKey].(string); ok && strings.TrimSpace(model) != "" {
		toolModel = strings.TrimSpace(model)
	}

	imageTool := &llm.ImageGeneration{
		Model:             toolModel,
		Background:        src.Image.Background,
		InputFidelity:     src.Image.InputFidelity,
		Moderation:        src.Image.Moderation,
		OutputCompression: src.Image.OutputCompression,
		OutputFormat:      src.Image.OutputFormat,
		PartialImages:     src.Image.PartialImages,
		Quality:           src.Image.Quality,
		Size:              src.Image.Size,
	}

	if maskURL := maskDataURL(src); maskURL != "" {
		imageTool.InputImageMask = map[string]any{
			"image_url": maskURL,
		}
	}

	metadata["image_generation_action"] = action

	// Persist the requested image options so BuildImageResponse can echo them
	// back on the client-facing response.
	if src.Image.OutputFormat != "" {
		metadata["codex_image_output_format"] = src.Image.OutputFormat
	}
	if src.Image.Quality != "" {
		metadata["codex_image_quality"] = src.Image.Quality
	}
	if src.Image.Size != "" {
		metadata["codex_image_size"] = src.Image.Size
	}
	if src.Image.Background != "" {
		metadata["codex_image_background"] = src.Image.Background
	}
	if toolModel != "" {
		metadata["codex_image_model"] = toolModel
	}

	return &llm.Request{
		Model:    src.Model,
		Messages: messages,
		User:     lo.Ternary(src.Image.User != "", lo.ToPtr(src.Image.User), src.User),
		Tools: []llm.Tool{
			{
				Type:            llm.ToolTypeImageGeneration,
				ImageGeneration: imageTool,
			},
		},
		ToolChoice: &llm.ToolChoice{
			ToolChoice: lo.ToPtr("required"),
		},
		// The Responses API has no top-level modalities field; image output is
		// driven by the image_generation tool above, so it is not set here.
		Stream:              src.Stream,
		StreamOptions:       src.StreamOptions,
		Store:               src.Store,
		RequestType:         llm.RequestTypeChat,
		APIFormat:           llm.APIFormatOpenAIResponse,
		RawRequest:          src.RawRequest,
		TransformerMetadata: metadata,
		TransformOptions:    src.TransformOptions,
		ServiceTier:         src.ServiceTier,
		SafetyIdentifier:    src.SafetyIdentifier,
		PromptCacheKey:      src.PromptCacheKey,
		Metadata:            src.Metadata,
	}, nil
}

func imageDataURLs(req *llm.Request) []string {
	if req == nil || req.Image == nil {
		return nil
	}

	if urls := imageURLsFromRawJSON(req.RawRequest, "image"); len(urls) > 0 {
		return urls
	}

	urls := make([]string, 0, len(req.Image.Images))
	for _, data := range req.Image.Images {
		urls = append(urls, bytesToImageDataURL(data))
	}

	return urls
}

func maskDataURL(req *llm.Request) string {
	if req == nil || req.Image == nil {
		return ""
	}

	if urls := imageURLsFromRawJSON(req.RawRequest, "mask"); len(urls) > 0 {
		return urls[0]
	}

	if len(req.Image.Mask) == 0 {
		return ""
	}

	return bytesToImageDataURL(req.Image.Mask)
}

func imageURLsFromRawJSON(raw *httpclient.Request, field string) []string {
	if raw == nil || len(raw.JSONBody) == 0 {
		return nil
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw.JSONBody, &body); err != nil {
		return nil
	}

	rawValue, ok := body[field]
	if !ok || len(rawValue) == 0 {
		return nil
	}

	var single string
	if err := json.Unmarshal(rawValue, &single); err == nil {
		if strings.HasPrefix(single, "data:image/") {
			return []string{single}
		}

		return nil
	}

	var many []string
	if err := json.Unmarshal(rawValue, &many); err != nil {
		return nil
	}

	urls := make([]string, 0, len(many))
	for _, item := range many {
		if strings.HasPrefix(item, "data:image/") {
			urls = append(urls, item)
		}
	}

	return urls
}

func bytesToImageDataURL(data []byte) string {
	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") {
		contentType = "image/png"
	}

	return xurl.BuildDataURL(contentType, base64.StdEncoding.EncodeToString(data), true)
}
