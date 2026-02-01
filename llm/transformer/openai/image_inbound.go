package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	transformer "github.com/looplj/axonhub/llm/transformer"
)

const (
	maxImageBodySize = 20 * 1024 * 1024
	maxImageFileSize = 4 * 1024 * 1024
	maxImageCount    = 10
)

var allowedImageTypes = []string{
	"image/png",
	"image/jpeg",
	"image/gif",
	"image/webp",
}

// ImageGenerationRequest represents the request structure for image generation API.
type ImageGenerationRequest struct {
	Prompt            string `json:"prompt"`
	Model             string `json:"model"`
	N                 *int64 `json:"n,omitempty"`
	Quality           string `json:"quality,omitempty"`
	ResponseFormat    string `json:"response_format,omitempty"`
	Size              string `json:"size,omitempty"`
	Style             string `json:"style,omitempty"`
	User              string `json:"user,omitempty"`
	Background        string `json:"background,omitempty"`
	OutputFormat      string `json:"output_format,omitempty"`
	OutputCompression *int64 `json:"output_compression,omitempty"`
	Moderation        string `json:"moderation,omitempty"`
	PartialImages     *int64 `json:"partial_images,omitempty"`
	Stream            bool   `json:"stream,omitempty"`
}

type ImageInboundTransformer struct {
	apiFormat llm.APIFormat
}

func NewImageGenerationInboundTransformer() *ImageInboundTransformer {
	return &ImageInboundTransformer{
		apiFormat: llm.APIFormatOpenAIImageGeneration,
	}
}

func NewImageEditInboundTransformer() *ImageInboundTransformer {
	return &ImageInboundTransformer{
		apiFormat: llm.APIFormatOpenAIImageEdit,
	}
}

func NewImageVariationInboundTransformer() *ImageInboundTransformer {
	return &ImageInboundTransformer{
		apiFormat: llm.APIFormatOpenAIImageVariation,
	}
}

func (t *ImageInboundTransformer) APIFormat() llm.APIFormat {
	return t.apiFormat
}

func (t *ImageInboundTransformer) TransformRequest(ctx context.Context, httpReq *httpclient.Request) (*llm.Request, error) {
	if httpReq == nil {
		return nil, fmt.Errorf("%w: http request is nil", transformer.ErrInvalidRequest)
	}

	if len(httpReq.Body) == 0 {
		return nil, fmt.Errorf("%w: request body is empty", transformer.ErrInvalidRequest)
	}

	//nolint:exhaustive // Only image-related API formats are handled here
	switch t.apiFormat {
	case llm.APIFormatOpenAIImageGeneration:
		return t.transformGenerationRequest(httpReq)
	case llm.APIFormatOpenAIImageEdit:
		return t.transformEditRequest(httpReq)
	case llm.APIFormatOpenAIImageVariation:
		return t.transformVariationRequest(ctx, httpReq)
	default:
		return nil, fmt.Errorf("%w: unknown image api format: %s", transformer.ErrInvalidRequest, t.apiFormat)
	}
}

func (t *ImageInboundTransformer) TransformResponse(ctx context.Context, llmResp *llm.Response) (*httpclient.Response, error) {
	if llmResp == nil || llmResp.Image == nil {
		return nil, fmt.Errorf("%w: image response is nil", transformer.ErrInvalidResponse)
	}

	created := llmResp.Created
	if created == 0 {
		created = time.Now().Unix()
	}

	oaiResp := ImagesResponse{
		Created: created,
		Data:    make([]ImageData, 0),
	}

	img := llmResp.Image
	oaiResp.Created = img.Created
	oaiResp.Background = img.Background
	oaiResp.OutputFormat = img.OutputFormat
	oaiResp.Quality = img.Quality
	oaiResp.Size = img.Size

	if img.Usage != nil {
		oaiResp.Usage = &ImagesResponseUsage{
			InputTokens:  img.Usage.InputTokens,
			OutputTokens: img.Usage.OutputTokens,
			TotalTokens:  img.Usage.TotalTokens,
		}
		if img.Usage.InputTokensDetails != nil {
			oaiResp.Usage.InputTokensDetails = &ImagesResponseUsageInputTokensDetails{
				ImageTokens: img.Usage.InputTokensDetails.ImageTokens,
				TextTokens:  img.Usage.InputTokensDetails.TextTokens,
			}
		}
	}

	for _, data := range img.Data {
		oaiResp.Data = append(oaiResp.Data, ImageData{
			B64JSON:       data.B64JSON,
			URL:           data.URL,
			RevisedPrompt: data.RevisedPrompt,
		})
	}

	body, err := json.Marshal(oaiResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal image response: %w", err)
	}

	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Headers: http.Header{
			"Content-Type":  []string{"application/json"},
			"Cache-Control": []string{"no-cache"},
		},
	}, nil
}

func (t *ImageInboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
	return nil, fmt.Errorf("%w: image request does not support streaming", transformer.ErrInvalidRequest)
}

func (t *ImageInboundTransformer) TransformError(ctx context.Context, rawErr error) *httpclient.Error {
	chatInbound := NewInboundTransformer()
	return chatInbound.TransformError(ctx, rawErr)
}

func (t *ImageInboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("%w: image request does not support streaming", transformer.ErrInvalidRequest)
}

func (t *ImageInboundTransformer) transformGenerationRequest(httpReq *httpclient.Request) (*llm.Request, error) {
	contentType := strings.ToLower(httpReq.Headers.Get("Content-Type"))
	if !strings.Contains(contentType, "application/json") {
		return nil, fmt.Errorf("%w: generations requires application/json", transformer.ErrInvalidRequest)
	}

	var genReq ImageGenerationRequest

	if err := json.Unmarshal(httpReq.Body, &genReq); err != nil {
		return nil, fmt.Errorf("%w: failed to decode generation request: %w", transformer.ErrInvalidRequest, err)
	}

	if genReq.Stream {
		return nil, fmt.Errorf("%w: image generation does not support streaming", transformer.ErrInvalidRequest)
	}

	model := genReq.Model
	if model == "" {
		model = "dall-e-2"
	}

	if genReq.Prompt == "" {
		return nil, fmt.Errorf("%w: prompt is required", transformer.ErrInvalidRequest)
	}

	imageReq := &llm.ImageRequest{
		Prompt:            genReq.Prompt,
		N:                 genReq.N,
		Size:              genReq.Size,
		Quality:           genReq.Quality,
		ResponseFormat:    genReq.ResponseFormat,
		User:              genReq.User,
		Background:        genReq.Background,
		OutputFormat:      genReq.OutputFormat,
		OutputCompression: genReq.OutputCompression,
		Moderation:        genReq.Moderation,
		PartialImages:     genReq.PartialImages,
		Style:             genReq.Style,
	}

	llmReq := &llm.Request{
		Model:       model,
		Modalities:  []string{"image"},
		Stream:      lo.ToPtr(false),
		RawRequest:  httpReq,
		RequestType: llm.RequestTypeImage,
		APIFormat:   t.apiFormat,
		Image:       imageReq,
	}

	return llmReq, nil
}

func (t *ImageInboundTransformer) transformEditRequest(httpReq *httpclient.Request) (*llm.Request, error) {
	formData, err := parseMultipartRequest(httpReq)
	if err != nil {
		return nil, err
	}

	if strings.EqualFold(strings.TrimSpace(formData.Fields["stream"]), "true") {
		return nil, fmt.Errorf("%w: image edit does not support streaming", transformer.ErrInvalidRequest)
	}

	prompt := strings.TrimSpace(formData.Fields["prompt"])
	if prompt == "" {
		return nil, fmt.Errorf("%w: prompt is required for image edits", transformer.ErrInvalidRequest)
	}

	model := strings.TrimSpace(formData.Fields["model"])
	if model == "" {
		model = "dall-e-2"
	}

	if len(formData.Images) == 0 {
		return nil, fmt.Errorf("%w: at least one image is required for edits", transformer.ErrInvalidRequest)
	}

	// Extract image data for ImageRequest
	images := make([][]byte, 0, len(formData.Images))
	for _, img := range formData.Images {
		images = append(images, img.Data)
	}

	var mask []byte
	if formData.Mask != nil {
		mask = formData.Mask.Data
	}

	user := strings.TrimSpace(formData.Fields["user"])

	imageReq := &llm.ImageRequest{
		Prompt:            prompt,
		Images:            images,
		Mask:              mask,
		N:                 parseOptionalInt64(formData.Fields["n"]),
		Size:              strings.TrimSpace(formData.Fields["size"]),
		Quality:           strings.TrimSpace(formData.Fields["quality"]),
		ResponseFormat:    strings.TrimSpace(formData.Fields["response_format"]),
		User:              user,
		Background:        strings.TrimSpace(formData.Fields["background"]),
		OutputFormat:      strings.TrimSpace(formData.Fields["output_format"]),
		OutputCompression: parseOptionalInt64(formData.Fields["output_compression"]),
		InputFidelity:     strings.TrimSpace(formData.Fields["input_fidelity"]),
		PartialImages:     parseOptionalInt64(formData.Fields["partial_images"]),
	}

	llmReq := &llm.Request{
		Model:       model,
		Modalities:  []string{"image"},
		Stream:      lo.ToPtr(false),
		RawRequest:  httpReq,
		RequestType: llm.RequestTypeImage,
		APIFormat:   t.apiFormat,
		Image:       imageReq,
	}

	return llmReq, nil
}

func (t *ImageInboundTransformer) transformVariationRequest(ctx context.Context, httpReq *httpclient.Request) (*llm.Request, error) {
	formData, err := parseMultipartRequest(httpReq)
	if err != nil {
		return nil, err
	}

	if strings.EqualFold(strings.TrimSpace(formData.Fields["stream"]), "true") {
		return nil, fmt.Errorf("%w: image variation does not support streaming", transformer.ErrInvalidRequest)
	}

	if strings.TrimSpace(formData.Fields["prompt"]) != "" {
		return nil, fmt.Errorf("%w: prompt is not allowed for variations", transformer.ErrInvalidRequest)
	}

	model := strings.TrimSpace(formData.Fields["model"])
	if model == "" {
		model = "dall-e-2"
	}

	if len(formData.Images) == 0 {
		return nil, fmt.Errorf("%w: image is required for variations", transformer.ErrInvalidRequest)
	}

	if len(formData.Images) > 1 {
		return nil, fmt.Errorf("%w: variations supports a single image", transformer.ErrInvalidRequest)
	}

	user := strings.TrimSpace(formData.Fields["user"])

	imageReq := &llm.ImageRequest{
		Images:         [][]byte{formData.Images[0].Data},
		N:              parseOptionalInt64(formData.Fields["n"]),
		Size:           strings.TrimSpace(formData.Fields["size"]),
		ResponseFormat: strings.TrimSpace(formData.Fields["response_format"]),
		User:           user,
	}

	llmReq := &llm.Request{
		Model:       model,
		Modalities:  []string{"image"},
		Stream:      lo.ToPtr(false),
		RawRequest:  httpReq,
		RequestType: llm.RequestTypeImage,
		APIFormat:   t.apiFormat,
		Image:       imageReq,
	}

	return llmReq, nil
}

type multipartFile struct {
	ContentType string
	Data        []byte
}

type imageFormData struct {
	Images []multipartFile
	Mask   *multipartFile
	Fields map[string]string
}

func parseMultipartRequest(httpReq *httpclient.Request) (*imageFormData, error) {
	if len(httpReq.Body) > maxImageBodySize {
		return nil, fmt.Errorf("%w: request body too large", transformer.ErrInvalidRequest)
	}

	contentType := httpReq.Headers.Get("Content-Type")

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid content-type", transformer.ErrInvalidRequest)
	}

	if !strings.HasPrefix(strings.ToLower(mediaType), "multipart/") {
		return nil, fmt.Errorf("%w: expected multipart/form-data", transformer.ErrInvalidRequest)
	}

	boundary := params["boundary"]
	if boundary == "" {
		return nil, fmt.Errorf("%w: missing boundary in content-type", transformer.ErrInvalidRequest)
	}

	reader := multipart.NewReader(bytes.NewReader(httpReq.Body), boundary)

	formData := &imageFormData{
		Fields: map[string]string{},
	}

	imageCount := 0

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("%w: failed to read multipart", transformer.ErrInvalidRequest)
		}

		fieldName := part.FormName()
		filename := part.FileName()

		if filename == "" {
			value, err := io.ReadAll(io.LimitReader(part, maxImageFileSize+1))
			if err != nil {
				return nil, fmt.Errorf("%w: failed to read multipart field", transformer.ErrInvalidRequest)
			}

			if len(value) > maxImageFileSize {
				return nil, fmt.Errorf("%w: multipart field too large", transformer.ErrInvalidRequest)
			}

			formData.Fields[fieldName] = string(value)

			continue
		}

		imageCount++
		if imageCount > maxImageCount {
			return nil, fmt.Errorf("%w: too many images", transformer.ErrInvalidRequest)
		}

		contentType := strings.TrimSpace(part.Header.Get("Content-Type"))

		data, err := io.ReadAll(io.LimitReader(part, maxImageFileSize+1))
		if err != nil {
			return nil, fmt.Errorf("%w: failed to read multipart file", transformer.ErrInvalidRequest)
		}

		if len(data) > maxImageFileSize {
			return nil, fmt.Errorf("%w: file too large", transformer.ErrInvalidRequest)
		}

		if contentType == "" {
			contentType = http.DetectContentType(lo.Ternary(len(data) > 512, data[:512], data))
		}

		if !isAllowedImageType(contentType) {
			return nil, fmt.Errorf("%w: unsupported image type", transformer.ErrInvalidRequest)
		}

		file := multipartFile{
			ContentType: contentType,
			Data:        data,
		}

		switch fieldName {
		case "image", "image[]":
			formData.Images = append(formData.Images, file)
		case "mask":
			formData.Mask = &file
		default:
		}
	}

	return formData, nil
}

func isAllowedImageType(contentType string) bool {
	for _, allowed := range allowedImageTypes {
		if strings.EqualFold(contentType, allowed) {
			return true
		}
	}

	return false
}

func parseOptionalInt64(s string) *int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}

	return &v
}
