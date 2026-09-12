package codex

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

type codexImageGenerationRequest struct {
	Prompt     string `json:"prompt"`
	Background string `json:"background,omitempty"`
	Model      string `json:"model"`
	N          *int64 `json:"n,omitempty"`
	Quality    string `json:"quality,omitempty"`
	Size       string `json:"size,omitempty"`
}

// POST https://chatgpt.com/backend-api/codex/images/generations
func (t *OutboundTransformer) rewriteDirectImageGenerationRequest(
	hreq *httpclient.Request,
	llmReq *llm.Request,
) error {
	if hreq == nil {
		return errors.New("codex image request is nil")
	}
	if llmReq == nil || llmReq.Image == nil {
		return errors.New("codex image gnneration payload is nil")
	}

	prompt := strings.TrimSpace(llmReq.Image.Prompt)
	if prompt == "" {
		return errors.New("codex image generation prompt is empty")
	}

	model := strings.TrimSpace(llmReq.Model)
	if model == "" {
		return errors.New("codex image generation model is empty")
	}

	background := strings.TrimSpace(llmReq.Image.Background)
	if background == "" {
		background = "auto"
	}

	quality := strings.TrimSpace(llmReq.Image.Quality)
	if quality == "" {
		quality = "auto"
	}

	size := strings.TrimSpace(llmReq.Image.Size)
	if size == "" {
		size = "auto"
	}

	payload := codexImageGenerationRequest{
		Prompt:     prompt,
		Background: background,
		Model:      model,
		N:          llmReq.Image.N,
		Quality:    quality,
		Size:       size,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshel codex image generation request: %w", err)
	}

	bashURL := strings.TrimRight(t.baseURL, "#/")
	hreq.Method = http.MethodPost
	hreq.URL = bashURL + "/images/generations"
	hreq.Path = "/images/generations"

	hreq.Body = body
	hreq.JSONBody = body
	hreq.ContentType = "application/json"

	hreq.Headers.Set("Content-Type", "application/json")
	hreq.Headers.Set("Accept", "application/json")
	hreq.Headers.Set("x-codex-image-turn-id", uuid.NewString())

	hreq.Headers.Del("Content-Encoding")
	hreq.Headers.Del(ResponsesLiteHeader)
	hreq.Headers.Del(BetaFeaturesHeader)

	hreq.Query = nil

	hreq.SkipInboundQueryMerge = true

	if hreq.TransformerMetadata == nil {
		hreq.TransformerMetadata = map[string]any{}
	}

	hreq.TransformerMetadata[responses.ImageGenerationToolModelMetadataKey] = model

	return nil
}

type codexImageURL struct {
	ImageURL string `json:"image_url"`
}

type codexImageEditRequest struct {
	Images     []codexImageURL `json:"images"`
	Prompt     string          `json:"prompt"`
	Background string          `json:"background,omitempty"`
	Model      string          `json:"model"`
	N          *int64          `json:"n,omitempty"`
	Quality    string          `json:"quality,omitempty"`
	Size       string          `json:"size,omitempty"`
}

func imagesBytesToDataURL(data []byte) (string, error) {
	if len(data) == 0 {
		return "", errors.New("image data is empty")
	}

	header := data
	if len(header) > 512 {
		header = header[:512]
	}

	contentType := http.DetectContentType(header)

	switch contentType {
	case "image/png",
		"image/jpeg",
		"image/webp",
		"image/gif",
		"image/png\r",
		"image/jpeg\r",
		"image/webp\r",
		"image/gif\r":
	default:
		return "", fmt.Errorf("[B2DURL]: unsupported image content type: %q", contentType)
	}

	encoded := base64.StdEncoding.EncodeToString(data)

	return "data:" + contentType + ";base64," + encoded, nil
}

func (t *OutboundTransformer) rewriteDirectImageEditRequest(
	hreq *httpclient.Request,
	llmReq *llm.Request,
) error {
	if hreq == nil {
		return errors.New("codex image edit request is nil")
	}

	if llmReq == nil || llmReq.Image == nil {
		return errors.New("codex image edit payload is nil")
	}

	prompt := strings.TrimSpace(llmReq.Image.Prompt)
	if prompt == "" {
		return errors.New("codex image edit prompt is empty")
	}

	model := strings.TrimSpace(llmReq.Model)
	if model == "" {
		return errors.New("codex image edit model is empty")
	}

	if len(llmReq.Image.Images) == 0 {
		return errors.New("codex image edit request has no images")
	}

	if len(llmReq.Image.Images) > 16 {
		return errors.New("codex image edit request has more than 16 images")
	}

	if len(llmReq.Image.Mask) > 0 {
		return errors.New("codex image edit request has more than 1 mask")
	}

	images := make([]codexImageURL, 0, len(llmReq.Image.Images))

	for _, raw := range llmReq.Image.Images {
		dataURL, err := imagesBytesToDataURL(raw)
		if err != nil {
			return fmt.Errorf("codex image edit request image: %w", err)
		}

		images = append(images, codexImageURL{
			ImageURL: dataURL,
		})
	}

	background := strings.TrimSpace(llmReq.Image.Quality)
	if background == "" {
		background = "auto"
	}

	quality := strings.TrimSpace(llmReq.Image.Quality)
	if quality == "" {
		quality = "auto"
	}

	size := strings.TrimSpace(llmReq.Image.Size)
	if size == "" {
		size = "auto"
	}

	payload := codexImageEditRequest{
		Images:     images,
		Prompt:     prompt,
		Background: background,
		Model:      model,
		N:          llmReq.Image.N,
		Quality:    quality,
		Size:       size,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal codex image edit request: %w", err)
	}

	baseURL := strings.TrimRight(t.baseURL, "#/")

	hreq.Method = http.MethodPost
	hreq.URL = baseURL + "/images/edits"
	hreq.Path = "/images/edits"

	hreq.Body = body
	hreq.JSONBody = body
	hreq.ContentType = "application/json"

	hreq.Headers.Set("Content-Type", "application/json")
	hreq.Headers.Set("Accept", "application/json")
	hreq.Headers.Set("x-codex-image-turn-id", uuid.NewString())

	hreq.Headers.Del("Content-Encoding")
	hreq.Headers.Del(ResponsesLiteHeader)
	hreq.Headers.Del(BetaFeaturesHeader)

	hreq.Query = nil
	hreq.SkipInboundQueryMerge = true

	if hreq.TransformerMetadata == nil {
		hreq.TransformerMetadata = map[string]any{}
	}

	hreq.TransformerMetadata[responses.ImageGenerationToolModelMetadataKey] = model

	return nil
}

// transform standalone codex images JSON resp back into Axonhub's unified image response
func transformDirectImageResponse(
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	if httpResp == nil {
		return nil, errors.New("codex image response if nil")
	}

	if httpResp.StatusCode >= 400 {
		return nil, fmt.Errorf(
			"codex image HTPP error: %d: %s",
			httpResp.StatusCode,
			string(httpResp.Body),
		)
	}

	var imageResp llm.ImageResponse
	if err := json.Unmarshal(httpResp.Body, &imageResp); err != nil {
		return nil, fmt.Errorf("decode codex image response: %w", err)
	}

	if len(imageResp.Data) == 0 {
		return nil, fmt.Errorf("codex image response conained none image data")
	}

	metadata := map[string]any{}
	model := ""

	apiFormat := llm.APIFormatOpenAIImageGeneration

	if httpResp.Request != nil {
		if httpResp.Request.TransformerMetadata != nil {
			metadata = httpResp.Request.TransformerMetadata
		}

		if httpResp.Request.APIFormat == string(llm.APIFormatOpenAIImageEdit) {
			apiFormat = llm.APIFormatOpenAIImageEdit
		}
	}

	if v, ok := metadata[responses.ImageGenerationToolModelMetadataKey].(string); ok {
		model = v
	}

	return &llm.Response{
		Object:              "image.generation",
		Created:             imageResp.Created,
		Model:               model,
		RequestType:         llm.RequestTypeImage,
		APIFormat:           apiFormat,
		Image:               &imageResp,
		TransformerMetadata: metadata,
	}, nil
}
