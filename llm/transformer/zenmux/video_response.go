package zenmux

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

func parseDuration(value *string) (*int64, error) {
	if value == nil {
		return nil, nil
	}
	duration, err := strconv.ParseInt(strings.TrimSpace(*value), 10, 64)
	if err != nil || duration <= 0 {
		return nil, fmt.Errorf("%w: duration must be a positive integer", transformer.ErrInvalidRequest)
	}
	return &duration, nil
}

func parseNativeVideoResponse(response *httpclient.Response, operation string) (*nativeVideoResponse, error) {
	if response == nil {
		return nil, fmt.Errorf("%w: http response is nil", transformer.ErrInvalidResponse)
	}
	var parsed nativeVideoResponse
	if err := json.Unmarshal(response.Body, &parsed); err != nil {
		return nil, fmt.Errorf("%w: failed to unmarshal ZenMux video %s response: %w", transformer.ErrInvalidResponse, operation, err)
	}
	if strings.TrimSpace(parsed.ID) == "" {
		return nil, fmt.Errorf("%w: missing id in ZenMux video response", transformer.ErrInvalidResponse)
	}
	if len(parsed.Content) != 0 && string(parsed.Content) != "null" {
		var content nativeVideoContent
		if err := json.Unmarshal(parsed.Content, &content); err != nil {
			return nil, fmt.Errorf("%w: invalid ZenMux video content: %w", transformer.ErrInvalidResponse, err)
		}
		parsed.ParsedContent = &content
	}
	status := strings.ToLower(strings.TrimSpace(parsed.Status))
	switch status {
	case "queued", "running", "succeeded", "failed":
		parsed.Status = status
	default:
		return nil, fmt.Errorf("%w: unsupported ZenMux video status %q", transformer.ErrInvalidResponse, parsed.Status)
	}
	return &parsed, nil
}

func toLLMVideoResponse(response *nativeVideoResponse, httpResponse *httpclient.Response) *llm.Response {
	model := response.Model
	if model == "" && httpResponse.Request != nil && httpResponse.Request.TransformerMetadata != nil {
		if requestModel, ok := httpResponse.Request.TransformerMetadata["model"].(string); ok {
			model = requestModel
		}
	}
	video := &llm.VideoResponse{ID: response.ID, Status: response.Status, Model: model}
	if response.ParsedContent != nil {
		video.VideoURL = response.ParsedContent.VideoURL
		video.LastFrameURL = response.ParsedContent.LastFrameURL
		video.Content = response.Content
	}
	if response.Error != nil {
		video.Error = &llm.VideoError{Code: response.Error.Code, Message: response.Error.Message}
	}
	return &llm.Response{
		ID:          response.ID,
		Object:      "video",
		Model:       model,
		RequestType: llm.RequestTypeVideo,
		APIFormat:   llm.APIFormatZenmuxVideo,
		Video:       video,
		Choices:     []llm.Choice{},
	}
}
