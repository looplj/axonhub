package openai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

// buildImageGenerationAPIRequest builds the HTTP request to call the OpenAI Image Generation API.
// based on whether images are present in the request.
func (t *OutboundTransformer) buildImageGenerationAPIRequest(chatReq *llm.Request) (*httpclient.Request, error) {
	chatReq.Stream = lo.ToPtr(false)

	if chatReq.Image == nil {
		return nil, fmt.Errorf("image request is required")
	}

	var (
		rawReq  *httpclient.Request
		fmtType llm.APIFormat
		err     error
	)

	//nolint:exhaustive // Only image-related API formats are handled here
	switch chatReq.APIFormat {
	case llm.APIFormatOpenAIImageVariation:
		rawReq, err = t.buildImageVariationRequest(chatReq)
		fmtType = llm.APIFormatOpenAIImageVariation
	case llm.APIFormatOpenAIImageEdit:
		rawReq, err = t.buildImageEditRequest(chatReq)
		fmtType = llm.APIFormatOpenAIImageEdit
	default:
		rawReq, err = t.buildImageGenerateRequest(chatReq)
		fmtType = llm.APIFormatOpenAIImageGeneration
	}

	if err != nil {
		return nil, err
	}

	rawReq.RequestType = llm.RequestTypeImage.String()
	rawReq.APIFormat = fmtType.String()
	// Save model to TransformerMetadata for response transformation
	if rawReq.TransformerMetadata == nil {
		rawReq.TransformerMetadata = map[string]any{}
	}

	rawReq.TransformerMetadata["model"] = chatReq.Model

	return rawReq, nil
}

func isModelSupportResponseFormat(model string) bool {
	return !strings.HasPrefix(model, "gpt-image-")
}

// buildImageGenerateRequest builds request for Image Generation API (images/generations).
func (t *OutboundTransformer) buildImageGenerateRequest(chatReq *llm.Request) (*httpclient.Request, error) {
	model := chatReq.Model

	prompt := chatReq.Image.Prompt
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required for image generation")
	}

	// Build request body
	reqBody := map[string]any{
		"prompt": prompt,
		"model":  model,
	}

	// Extract image generation parameters from Image field
	img := chatReq.Image
	if img.N != nil {
		reqBody["n"] = *img.N
	}

	if img.OutputFormat != "" {
		reqBody["output_format"] = img.OutputFormat
	}

	if img.Size != "" {
		reqBody["size"] = img.Size
	}

	if img.Quality != "" {
		reqBody["quality"] = img.Quality
	}

	if img.Background != "" {
		reqBody["background"] = img.Background
	}

	if img.Moderation != "" {
		reqBody["moderation"] = img.Moderation
	}

	if img.Style != "" {
		reqBody["style"] = img.Style
	}

	if img.OutputCompression != nil {
		reqBody["output_compression"] = *img.OutputCompression
	}

	if img.PartialImages != nil {
		reqBody["partial_images"] = *img.PartialImages
	}

	if img.ResponseFormat != "" && isModelSupportResponseFormat(model) {
		reqBody["response_format"] = img.ResponseFormat
	}

	if isModelSupportResponseFormat(chatReq.Model) {
		if _, ok := reqBody["response_format"]; !ok {
			reqBody["response_format"] = "b64_json"
		}
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

	switch t.config.PlatformType {
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
//
//nolint:maintidx // Complex function for building multipart form data
func (t *OutboundTransformer) buildImageEditRequest(chatReq *llm.Request) (*httpclient.Request, error) {
	model := chatReq.Model

	prompt := chatReq.Image.Prompt
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required for image generation")
	}

	var (
		formFiles []FormFile
		maskFile  *FormFile
	)

	// Convert raw image bytes to FormFiles

	for i, data := range chatReq.Image.Images {
		formFiles = append(formFiles, FormFile{
			Filename:    fmt.Sprintf("image_%d.png", i+1),
			ContentType: "image/png",
			Data:        data,
			Format:      "png",
		})
	}

	if len(chatReq.Image.Mask) > 0 {
		maskFile = &FormFile{
			Filename:    "mask.png",
			ContentType: "image/png",
			Data:        chatReq.Image.Mask,
			Format:      "png",
		}
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

	if maskFile != nil {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="mask"; filename="%s"`, maskFile.Filename))
		h.Set("Content-Type", maskFile.ContentType)

		part, err := writer.CreatePart(h)
		if err != nil {
			return nil, fmt.Errorf("failed to create mask file: %w", err)
		}

		if _, err := io.Copy(part, bytes.NewReader(maskFile.Data)); err != nil {
			return nil, fmt.Errorf("failed to write mask data: %w", err)
		}

		jsonBody["mask"] = maskFile
	}

	// Add prompt
	if err := writer.WriteField("prompt", prompt); err != nil {
		return nil, fmt.Errorf("failed to write prompt field: %w", err)
	}

	// Add model if specified
	if model != "" {
		if err := writer.WriteField("model", model); err != nil {
			return nil, fmt.Errorf("failed to write model field: %w", err)
		}

		jsonBody["model"] = model
	}

	// Extract image edit parameters from Image field
	if img := chatReq.Image; img != nil {
		if img.N != nil {
			if err := writer.WriteField("n", fmt.Sprintf("%d", *img.N)); err != nil {
				return nil, fmt.Errorf("failed to write n field: %w", err)
			}

			jsonBody["n"] = *img.N
		}

		if img.OutputFormat != "" {
			if err := writer.WriteField("output_format", img.OutputFormat); err != nil {
				return nil, fmt.Errorf("failed to write output_format field: %w", err)
			}

			jsonBody["output_format"] = img.OutputFormat
		}

		if img.Size != "" {
			if err := writer.WriteField("size", img.Size); err != nil {
				return nil, fmt.Errorf("failed to write size field: %w", err)
			}

			jsonBody["size"] = img.Size
		}

		if img.Quality != "" {
			if err := writer.WriteField("quality", img.Quality); err != nil {
				return nil, fmt.Errorf("failed to write quality field: %w", err)
			}

			jsonBody["quality"] = img.Quality
		}

		if img.Background != "" {
			if err := writer.WriteField("background", img.Background); err != nil {
				return nil, fmt.Errorf("failed to write background field: %w", err)
			}

			jsonBody["background"] = img.Background
		}

		if img.InputFidelity != "" {
			if err := writer.WriteField("input_fidelity", img.InputFidelity); err != nil {
				return nil, fmt.Errorf("failed to write input_fidelity field: %w", err)
			}

			jsonBody["input_fidelity"] = img.InputFidelity
		}

		if img.OutputCompression != nil {
			if err := writer.WriteField("output_compression", fmt.Sprintf("%d", *img.OutputCompression)); err != nil {
				return nil, fmt.Errorf("failed to write output_compression field: %w", err)
			}

			jsonBody["output_compression"] = *img.OutputCompression
		}

		if img.PartialImages != nil {
			if err := writer.WriteField("partial_images", fmt.Sprintf("%d", *img.PartialImages)); err != nil {
				return nil, fmt.Errorf("failed to write partial_images field: %w", err)
			}

			jsonBody["partial_images"] = *img.PartialImages
		}

		if img.ResponseFormat != "" && model != "gpt-image-1" {
			if err := writer.WriteField("response_format", img.ResponseFormat); err != nil {
				return nil, fmt.Errorf("failed to write response_format field: %w", err)
			}

			jsonBody["response_format"] = img.ResponseFormat
		}
	}

	if model != "gpt-image-1" {
		if _, ok := jsonBody["response_format"]; !ok {
			if err := writer.WriteField("response_format", "b64_json"); err != nil {
				return nil, fmt.Errorf("failed to write response_format field: %w", err)
			}

			jsonBody["response_format"] = "b64_json"
		}
	}

	// Add user if specified
	if chatReq.Image.User != "" {
		if err := writer.WriteField("user", chatReq.Image.User); err != nil {
			return nil, fmt.Errorf("failed to write user field: %w", err)
		}

		jsonBody["user"] = chatReq.Image.User
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

	switch t.config.PlatformType {
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

func (t *OutboundTransformer) buildImageVariationRequest(chatReq *llm.Request) (*httpclient.Request, error) {
	model := chatReq.Model

	var formFiles []FormFile

	// Convert raw image bytes to FormFiles
	for i, data := range chatReq.Image.Images {
		formFiles = append(formFiles, FormFile{
			Filename:    fmt.Sprintf("image_%d.png", i+1),
			ContentType: "image/png",
			Data:        data,
			Format:      "png",
		})
	}

	if len(formFiles) == 0 {
		return nil, fmt.Errorf("%w: image is required for image variations", transformer.ErrInvalidRequest)
	}

	if len(formFiles) > 1 {
		return nil, fmt.Errorf("%w: only one image is supported for image variations", transformer.ErrInvalidRequest)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename="%s"`, formFiles[0].Filename))
	h.Set("Content-Type", formFiles[0].ContentType)

	part, err := writer.CreatePart(h)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, bytes.NewReader(formFiles[0].Data)); err != nil {
		return nil, fmt.Errorf("failed to write image data: %w", err)
	}

	jsonBody := map[string]any{
		"formFiles": formFiles,
	}

	if model != "" {
		if err := writer.WriteField("model", model); err != nil {
			return nil, fmt.Errorf("failed to write model field: %w", err)
		}

		jsonBody["model"] = model
	}

	// Extract variation parameters from Image field
	if img := chatReq.Image; img != nil {
		if img.N != nil {
			if err := writer.WriteField("n", fmt.Sprintf("%d", *img.N)); err != nil {
				return nil, fmt.Errorf("failed to write n field: %w", err)
			}

			jsonBody["n"] = *img.N
		}

		if img.Size != "" {
			if err := writer.WriteField("size", img.Size); err != nil {
				return nil, fmt.Errorf("failed to write size field: %w", err)
			}

			jsonBody["size"] = img.Size
		}

		if img.ResponseFormat != "" && model != "gpt-image-1" {
			if err := writer.WriteField("response_format", img.ResponseFormat); err != nil {
				return nil, fmt.Errorf("failed to write response_format field: %w", err)
			}

			jsonBody["response_format"] = img.ResponseFormat
		}
	}

	if model != "gpt-image-1" {
		if _, ok := jsonBody["response_format"]; !ok {
			if err := writer.WriteField("response_format", "b64_json"); err != nil {
				return nil, fmt.Errorf("failed to write response_format field: %w", err)
			}

			jsonBody["response_format"] = "b64_json"
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	headers := make(http.Header)
	headers.Set("Content-Type", writer.FormDataContentType())
	headers.Set("Accept", "application/json")

	url := t.config.BaseURL + "/images/variations"

	var auth *httpclient.AuthConfig

	switch t.config.PlatformType {
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

	// Read model from request TransformerMetadata
	model := "image-generation"

	if httpResp.Request != nil && httpResp.Request.TransformerMetadata != nil {
		if m, ok := httpResp.Request.TransformerMetadata["model"].(string); ok && m != "" {
			model = m
		}
	}

	// Convert to llm.Response format
	resp := &llm.Response{
		ID:          fmt.Sprintf("img-%d", imgResp.Created),
		Object:      "chat.completion",
		Created:     imgResp.Created,
		Model:       model,
		RequestType: llm.RequestTypeImage,
	}

	// Build ImageResponse
	imageResponse := &llm.ImageResponse{
		Created:      imgResp.Created,
		Data:         make([]llm.ImageData, 0, len(imgResp.Data)),
		Background:   imgResp.Background,
		OutputFormat: imgResp.OutputFormat,
		Quality:      imgResp.Quality,
		Size:         imgResp.Size,
	}

	// Convert usage information if present
	if imgResp.Usage != nil {
		resp.Usage = &llm.Usage{
			PromptTokens:     imgResp.Usage.InputTokens,
			CompletionTokens: imgResp.Usage.OutputTokens,
			TotalTokens:      imgResp.Usage.TotalTokens,
		}

		imageResponse.Usage = &llm.ImageUsage{
			InputTokens:  imgResp.Usage.InputTokens,
			OutputTokens: imgResp.Usage.OutputTokens,
			TotalTokens:  imgResp.Usage.TotalTokens,
		}
		if imgResp.Usage.InputTokensDetails != nil {
			imageResponse.Usage.InputTokensDetails = &llm.ImageInputTokenDetails{
				ImageTokens: imgResp.Usage.InputTokensDetails.ImageTokens,
				TextTokens:  imgResp.Usage.InputTokensDetails.TextTokens,
			}
		}
	}

	// Convert each image to ImageData
	for _, img := range imgResp.Data {
		imageResponse.Data = append(imageResponse.Data, llm.ImageData{
			B64JSON:       img.B64JSON,
			URL:           img.URL,
			RevisedPrompt: img.RevisedPrompt,
		})
	}

	resp.Image = imageResponse

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
