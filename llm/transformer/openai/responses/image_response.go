package responses

import (
	"fmt"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/internal/pkg/xurl"
)

func BuildImageResponse(upstream *Response, metadata map[string]any) (*llm.Response, error) {
	if upstream == nil {
		return nil, fmt.Errorf("response is nil")
	}

	if upstream.Error != nil {
		return nil, fmt.Errorf("codex image response error: %s", upstream.Error.Message)
	}

	format, _ := metadata["codex_image_output_format"].(string)
	if format == "" {
		format = "png"
	}

	imageResp := &llm.ImageResponse{
		Created:      upstream.CreatedAt,
		Data:         make([]llm.ImageData, 0),
		OutputFormat: format,
	}

	if quality, ok := metadata["codex_image_quality"].(string); ok {
		imageResp.Quality = quality
	}
	if size, ok := metadata["codex_image_size"].(string); ok {
		imageResp.Size = size
	}
	if background, ok := metadata["codex_image_background"].(string); ok {
		imageResp.Background = background
	}

	for _, item := range upstream.Output {
		if item.Type != "image_generation_call" || item.Result == nil || *item.Result == "" {
			continue
		}

		b64JSON := xurl.ExtractBase64FromDataURL(*item.Result)
		imageResp.Data = append(imageResp.Data, llm.ImageData{
			B64JSON: b64JSON,
		})
	}

	if len(imageResp.Data) == 0 {
		return nil, fmt.Errorf("codex image response did not contain image_generation_call result")
	}

	result := &llm.Response{
		ID:          upstream.ID,
		Object:      "image.generation",
		Created:     upstream.CreatedAt,
		Model:       upstream.Model,
		RequestType: llm.RequestTypeImage,
		Image:       imageResp,
	}

	if upstream.Usage != nil {
		result.Usage = upstream.Usage.ToUsage()
	}

	if result.Model == "" {
		if model, ok := metadata["codex_image_model"].(string); ok {
			result.Model = model
		}
	}

	return result, nil
}
