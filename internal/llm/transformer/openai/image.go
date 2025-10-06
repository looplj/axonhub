package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// buildResponsesAPIRequest builds the HTTP request to call the OpenAI Responses API
// for image generation.
func (t *OutboundTransformer) buildResponsesAPIRequest(ctx context.Context, chatReq *llm.Request) (*httpclient.Request, error) {
	chatReq.Stream = lo.ToPtr(false)

	rawReq, err := t.rt.TransformRequest(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request: %w", err)
	}

	if rawReq.Metadata == nil {
		rawReq.Metadata = map[string]string{}
	}

	rawReq.Metadata["outbound_format_type"] = llm.APIFormatOpenAIResponse.String()

	return rawReq, nil
}

// buildImageGenerationAPIRequest builds the HTTP request to call the OpenAI Image Generation API.
// based on whether images are present in the request.
func (t *OutboundTransformer) buildImageGenerationAPIRequest(ctx context.Context, chatReq *llm.Request) (*httpclient.Request, error) {
	chatReq.Stream = lo.ToPtr(false)

	// Check if there are images in the messages
	hasImages := hasImagesInMessages(chatReq.Messages)

	var (
		rawReq *httpclient.Request
		err    error
	)

	if hasImages {
		// Use Image Edit API (images/edits)
		rawReq, err = t.buildImageEditRequest(chatReq)
	} else {
		// Use Image Generation API (images/generations)
		rawReq, err = t.buildImageGenerateRequest(chatReq)
	}

	if err != nil {
		return nil, err
	}

	if rawReq.Metadata == nil {
		rawReq.Metadata = map[string]string{}
	}

	rawReq.Metadata["outbound_format_type"] = llm.APIFormatOpenAIImageGeneration.String()
	// Save model to metadata for response transformation
	rawReq.Metadata["model"] = chatReq.Model

	return rawReq, nil
}

// buildImageGenerateRequest builds request for Image Generation API (images/generations).
func (t *OutboundTransformer) buildImageGenerateRequest(chatReq *llm.Request) (*httpclient.Request, error) {
	// Extract prompt from messages
	prompt, err := extractPromptFromMessages(chatReq.Messages)
	if err != nil {
		return nil, err
	}

	// Build request body
	reqBody := map[string]any{
		"prompt": prompt,
		"model":  chatReq.Model,
	}

	// Extract image generation parameters from tools
	for _, tool := range chatReq.Tools {
		if tool.Type == "image_generation" && tool.ImageGeneration != nil {
			if tool.ImageGeneration.OutputFormat != "" {
				reqBody["output_format"] = tool.ImageGeneration.OutputFormat
			}

			if tool.ImageGeneration.Size != "" {
				reqBody["size"] = tool.ImageGeneration.Size
			}

			if tool.ImageGeneration.Quality != "" {
				reqBody["quality"] = tool.ImageGeneration.Quality
			}

			if tool.ImageGeneration.Background != "" {
				reqBody["background"] = tool.ImageGeneration.Background
			}

			if tool.ImageGeneration.Moderation != "" {
				reqBody["moderation"] = tool.ImageGeneration.Moderation
			}

			if tool.ImageGeneration.OutputCompression != nil {
				reqBody["output_compression"] = *tool.ImageGeneration.OutputCompression
			}

			if tool.ImageGeneration.PartialImages != nil {
				reqBody["partial_images"] = *tool.ImageGeneration.PartialImages
			}

			break
		}
	}

	// Always use b64_json response format for consistency
	// gpt-image-1 does not allow customize response format
	if chatReq.Model != "gpt-image-1" {
		reqBody["response_format"] = "b64_json"
	}

	// Add user if specified
	if chatReq.User != nil {
		reqBody["user"] = *chatReq.User
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	// Build URL
	url := t.config.BaseURL + "/images/generations"

	// Build auth config
	var auth *httpclient.AuthConfig

	switch t.config.Type {
	case PlatformAzure:
		auth = &httpclient.AuthConfig{
			Type:      "api_key",
			APIKey:    t.config.APIKey,
			HeaderKey: "Api-Key",
		}
	case PlatformOpenAI:
		auth = &httpclient.AuthConfig{
			Type:   "bearer",
			APIKey: t.config.APIKey,
		}
	}

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth:    auth,
	}, nil
}

// buildImageEditRequest builds request for Image Edit API (images/edits).
func (t *OutboundTransformer) buildImageEditRequest(chatReq *llm.Request) (*httpclient.Request, error) {
	// Extract prompt from messages
	prompt, err := extractPromptFromMessages(chatReq.Messages)
	if err != nil {
		return nil, err
	}

	// Extract images and prepare form files
	formFiles, err := extractImageFiles(chatReq)
	if err != nil {
		return nil, err
	}

	if len(formFiles) == 0 {
		return nil, fmt.Errorf("at least one image is required for image editing,%w", transformer.ErrInvalidRequest)
	}

	// Build multipart form data and JSONBody together
	jsonBody := map[string]any{
		"prompt":    prompt,
		"formFiles": formFiles,
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add images with proper MIME headers
	for _, formFile := range formFiles {
		// Create proper MIME header with Content-Type
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename="%s"`, formFile.Filename))
		h.Set("Content-Type", formFile.ContentType)

		part, err := writer.CreatePart(h)
		if err != nil {
			return nil, fmt.Errorf("failed to create form file: %w", err)
		}

		if _, err := io.Copy(part, bytes.NewReader(formFile.Data)); err != nil {
			return nil, fmt.Errorf("failed to write image data: %w", err)
		}
	}

	// Add prompt
	if err := writer.WriteField("prompt", prompt); err != nil {
		return nil, fmt.Errorf("failed to write prompt field: %w", err)
	}

	// Add model if specified
	if chatReq.Model != "" {
		if err := writer.WriteField("model", chatReq.Model); err != nil {
			return nil, fmt.Errorf("failed to write model field: %w", err)
		}

		jsonBody["model"] = chatReq.Model
	}

	// Extract image generation parameters from tools
	for _, tool := range chatReq.Tools {
		if tool.Type == "image_generation" && tool.ImageGeneration != nil {
			if tool.ImageGeneration.OutputFormat != "" {
				if err := writer.WriteField("output_format", tool.ImageGeneration.OutputFormat); err != nil {
					return nil, fmt.Errorf("failed to write output_format field: %w", err)
				}

				jsonBody["output_format"] = tool.ImageGeneration.OutputFormat
			}

			if tool.ImageGeneration.Size != "" {
				if err := writer.WriteField("size", tool.ImageGeneration.Size); err != nil {
					return nil, fmt.Errorf("failed to write size field: %w", err)
				}

				jsonBody["size"] = tool.ImageGeneration.Size
			}

			if tool.ImageGeneration.Quality != "" {
				if err := writer.WriteField("quality", tool.ImageGeneration.Quality); err != nil {
					return nil, fmt.Errorf("failed to write quality field: %w", err)
				}

				jsonBody["quality"] = tool.ImageGeneration.Quality
			}

			if tool.ImageGeneration.Background != "" {
				if err := writer.WriteField("background", tool.ImageGeneration.Background); err != nil {
					return nil, fmt.Errorf("failed to write background field: %w", err)
				}

				jsonBody["background"] = tool.ImageGeneration.Background
			}

			if tool.ImageGeneration.InputFidelity != "" {
				if err := writer.WriteField("input_fidelity", tool.ImageGeneration.InputFidelity); err != nil {
					return nil, fmt.Errorf("failed to write input_fidelity field: %w", err)
				}

				jsonBody["input_fidelity"] = tool.ImageGeneration.InputFidelity
			}

			if tool.ImageGeneration.OutputCompression != nil {
				if err := writer.WriteField("output_compression", fmt.Sprintf("%d", *tool.ImageGeneration.OutputCompression)); err != nil {
					return nil, fmt.Errorf("failed to write output_compression field: %w", err)
				}

				jsonBody["output_compression"] = *tool.ImageGeneration.OutputCompression
			}

			if tool.ImageGeneration.PartialImages != nil {
				if err := writer.WriteField("partial_images", fmt.Sprintf("%d", *tool.ImageGeneration.PartialImages)); err != nil {
					return nil, fmt.Errorf("failed to write partial_images field: %w", err)
				}

				jsonBody["partial_images"] = *tool.ImageGeneration.PartialImages
			}

			break
		}
	}

	// Always use b64_json response format for consistency
	if chatReq.Model != "gpt-image-1" {
		if err := writer.WriteField("response_format", "b64_json"); err != nil {
			return nil, fmt.Errorf("failed to write response_format field: %w", err)
		}

		jsonBody["response_format"] = "b64_json"
	}

	// Add user if specified
	if chatReq.User != nil {
		if err := writer.WriteField("user", *chatReq.User); err != nil {
			return nil, fmt.Errorf("failed to write user field: %w", err)
		}

		jsonBody["user"] = *chatReq.User
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", writer.FormDataContentType())
	headers.Set("Accept", "application/json")

	// Build URL
	url := t.config.BaseURL + "/images/edits"

	// Build auth config
	var auth *httpclient.AuthConfig

	switch t.config.Type {
	case PlatformAzure:
		auth = &httpclient.AuthConfig{
			Type:      "api_key",
			APIKey:    t.config.APIKey,
			HeaderKey: "Api-Key",
		}
	case PlatformOpenAI:
		auth = &httpclient.AuthConfig{
			Type:   "bearer",
			APIKey: t.config.APIKey,
		}
	}

	// Marshal JSONBody
	jsonBodyBytes, err := json.Marshal(jsonBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON body: %w", err)
	}

	return &httpclient.Request{
		Method:      http.MethodPost,
		URL:         url,
		Headers:     headers,
		ContentType: writer.FormDataContentType(),
		Body:        body.Bytes(),
		JSONBody:    jsonBodyBytes,
		Auth:        auth,
	}, nil
}

type FormFile struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Data        []byte `json:"data"`
	Format      string `json:"format"` // image format like "png", "jpeg", etc.
}

// transformImageGenerationResponse transforms the OpenAI Image Generation/Edit API response
// to the unified llm.Response format.
func transformImageGenerationResponse(httpResp *httpclient.Response) (*llm.Response, error) {
	// Parse the OpenAI ImagesResponse
	var imgResp ImagesResponse
	if err := json.Unmarshal(httpResp.Body, &imgResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal images response: %w", err)
	}

	// Read model from request metadata
	model := "image-generation"

	if httpResp.Request != nil && httpResp.Request.Metadata != nil {
		if m, ok := httpResp.Request.Metadata["model"]; ok && m != "" {
			model = m
		}
	}

	// Convert to llm.Response format
	resp := &llm.Response{
		ID:      fmt.Sprintf("img-%d", imgResp.Created),
		Object:  "chat.completion",
		Created: imgResp.Created,
		Model:   model,
		Choices: make([]llm.Choice, 0, len(imgResp.Data)),
	}

	// Convert usage information if present
	if imgResp.Usage != nil {
		resp.Usage = &llm.Usage{
			PromptTokens:     imgResp.Usage.InputTokens,
			CompletionTokens: imgResp.Usage.OutputTokens,
			TotalTokens:      imgResp.Usage.TotalTokens,
		}
	}

	// Convert each image to a choice with image_url content
	for i, img := range imgResp.Data {
		// Build data URL from base64 data
		var imageURL string

		if img.B64JSON != "" {
			// Determine the image format based on output_format
			format := "png"
			if imgResp.OutputFormat != "" {
				format = imgResp.OutputFormat
			}

			imageURL = fmt.Sprintf("data:image/%s;base64,%s", format, img.B64JSON)
		} else if img.URL != "" {
			imageURL = img.URL
		}

		choice := llm.Choice{
			Index: i,
			Message: &llm.Message{
				Role: "assistant",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type: "image_url",
							ImageURL: &llm.ImageURL{
								URL: imageURL,
							},
						},
					},
				},
			},
			FinishReason: lo.ToPtr("stop"),
		}

		resp.Choices = append(resp.Choices, choice)
	}

	return resp, nil
}

// ImagesResponse represents the response from OpenAI Image Generation/Edit API.
type ImagesResponse struct {
	Created      int64                `json:"created"`
	Data         []ImageData          `json:"data"`
	Background   string               `json:"background,omitempty"`
	OutputFormat string               `json:"output_format,omitempty"`
	Quality      string               `json:"quality,omitempty"`
	Size         string               `json:"size,omitempty"`
	Usage        *ImagesResponseUsage `json:"usage,omitempty"`
}

// ImageData represents a single image in the response.
type ImageData struct {
	B64JSON       string `json:"b64_json,omitempty"`
	URL           string `json:"url,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

// ImagesResponseUsage represents usage information for image generation.
type ImagesResponseUsage struct {
	InputTokens        int64                                  `json:"input_tokens"`
	OutputTokens       int64                                  `json:"output_tokens"`
	TotalTokens        int64                                  `json:"total_tokens"`
	InputTokensDetails *ImagesResponseUsageInputTokensDetails `json:"input_tokens_details,omitempty"`
}

// ImagesResponseUsageInputTokensDetails represents detailed input token information.
type ImagesResponseUsageInputTokensDetails struct {
	ImageTokens int64 `json:"image_tokens"`
	TextTokens  int64 `json:"text_tokens"`
}

// hasImagesInMessages checks if any message contains image content.
func hasImagesInMessages(messages []llm.Message) bool {
	for _, msg := range messages {
		if len(msg.Content.MultipleContent) > 0 {
			for _, part := range msg.Content.MultipleContent {
				if part.Type == "image_url" {
					return true
				}
			}
		}
	}

	return false
}

// extractPromptFromMessages extracts the text prompt from messages.
func extractPromptFromMessages(messages []llm.Message) (string, error) {
	for _, msg := range messages {
		if msg.Content.Content != nil {
			return *msg.Content.Content, nil
		}

		if len(msg.Content.MultipleContent) > 0 {
			for _, part := range msg.Content.MultipleContent {
				if part.Type == "text" && part.Text != nil {
					return *part.Text, nil
				}
			}
		}
	}

	return "", fmt.Errorf("prompt is required for image generation")
}

func extractImageFiles(chatReq *llm.Request) ([]FormFile, error) {
	// Extract images and prepare form files
	var (
		formFiles []FormFile
		count     int
	)

	for _, msg := range chatReq.Messages {
		if len(msg.Content.MultipleContent) > 0 {
			for _, part := range msg.Content.MultipleContent {
				if part.Type == "image_url" && part.ImageURL != nil {
					count++
					// Extract image data as FormFile
					formFile, err := extractFile(part.ImageURL.URL)
					if err != nil {
						return nil, fmt.Errorf("failed to extract image data: %w", err)
					}

					formFile.Filename = fmt.Sprintf("image_%d.%s", count, formFile.Format)
					formFiles = append(formFiles, formFile)
				}
			}
		}
	}

	return formFiles, nil
}

// extractFile extracts base64 image data and returns FormFile.
func extractFile(url string) (FormFile, error) {
	// Check if it's a data URL (data:image/png;base64,...)
	if len(url) > 5 && url[:5] == "data:" {
		// Find the base64 data part
		parts := bytes.SplitN([]byte(url), []byte(","), 2)
		if len(parts) != 2 {
			return FormFile{}, fmt.Errorf("%w: invalid data URL format: missing comma separator", transformer.ErrInvalidRequest)
		}

		// Extract format from data URL prefix (e.g., "data:image/png;base64,")
		header := string(parts[0])
		format := "png" // default format

		// Parse format from header like "data:image/png;base64" or "data:image/jpeg;base64"
		if len(header) > 11 && header[:5] == "data:" && header[5:11] == "image/" {
			// Extract format between "image/" and ";base64" or just "image/" if no base64 specified
			formatPart := header[11:]
			if semicolonIndex := strings.Index(formatPart, ";"); semicolonIndex > 0 {
				format = formatPart[:semicolonIndex]
			} else {
				format = formatPart
			}
		}

		// Decode base64
		data, err := base64.StdEncoding.DecodeString(string(parts[1]))
		if err != nil {
			return FormFile{}, fmt.Errorf("failed to decode base64 data: %w", err)
		}

		contentType := fmt.Sprintf("image/%s", format)

		return FormFile{
			Filename:    fmt.Sprintf("image.%s", format),
			ContentType: contentType,
			Data:        data,
			Format:      format,
		}, nil
	}

	return FormFile{}, fmt.Errorf("%w: only data URLs are supported for image editing", transformer.ErrInvalidRequest)
}
