package zenmux

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

func (t *OutboundTransformer) buildVideoRequest(ctx context.Context, request *llm.Request) (*httpclient.Request, error) {
	if request.Video == nil {
		return nil, fmt.Errorf("%w: video request is required", transformer.ErrInvalidRequest)
	}
	if strings.TrimSpace(request.Model) == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}
	if len(request.Video.Content) == 0 {
		return nil, fmt.Errorf("%w: content is required", transformer.ErrInvalidRequest)
	}
	if strings.TrimSpace(request.Video.ServiceTier) != "" {
		return nil, fmt.Errorf("%w: service_tier is unsupported for ZenMux video", transformer.ErrInvalidRequest)
	}
	if request.Video.ExecutionExpiresAfter != nil {
		return nil, fmt.Errorf("%w: execution_expires_after is unsupported for ZenMux video", transformer.ErrInvalidRequest)
	}
	ratio := strings.TrimSpace(request.Video.Ratio)
	resolution := strings.TrimSpace(request.Video.Resolution)
	if ratio == "" && resolution == "" && strings.TrimSpace(request.Video.Size) != "" {
		mappedRatio, mappedResolution, ok := inferNativeRatioResolution(request.Video.Size)
		if !ok {
			return nil, fmt.Errorf("%w: size %q cannot be mapped to ratio/resolution, please set ratio and resolution", transformer.ErrInvalidRequest, request.Video.Size)
		}
		ratio, resolution = mappedRatio, mappedResolution
	}

	content, err := buildNativeContent(request.Video.Content)
	if err != nil {
		return nil, err
	}
	duration, err := parseDuration(request.Video.Duration)
	if err != nil {
		return nil, err
	}
	nativeRequest := nativeCreateRequest{
		Model:           request.Model,
		Content:         content,
		Resolution:      resolution,
		Ratio:           ratio,
		Duration:        duration,
		Seed:            request.Video.Seed,
		GenerateAudio:   request.Video.GenerateAudio,
		Frames:          request.Video.Frames,
		CameraFixed:     request.Video.CameraFixed,
		Watermark:       request.Video.Watermark,
		Draft:           request.Video.Draft,
		CallbackURL:     request.Video.CallbackURL,
		ReturnLastFrame: request.Video.ReturnLastFrame,
		Tools:           request.Video.Tools,
	}
	body, err := json.Marshal(nativeRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ZenMux video request: %w", err)
	}
	if len(strings.TrimSpace(string(request.ExtraBody))) != 0 {
		var extraFields map[string]json.RawMessage
		if err := json.Unmarshal(request.ExtraBody, &extraFields); err != nil || extraFields == nil {
			return nil, fmt.Errorf("%w: extra_body must be a JSON object", transformer.ErrInvalidRequest)
		}
		for _, field := range []string{"service_tier", "execution_expires_after"} {
			if _, ok := extraFields[field]; ok {
				return nil, fmt.Errorf("%w: %s is unsupported for ZenMux video", transformer.ErrInvalidRequest, field)
			}
		}
		var nativeFields map[string]json.RawMessage
		if err := json.Unmarshal(body, &nativeFields); err != nil {
			return nil, fmt.Errorf("failed to decode ZenMux video request: %w", err)
		}
		for key, value := range nativeFields {
			extraFields[key] = value
		}
		body, err = json.Marshal(extraFields)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal ZenMux video request: %w", err)
		}
	}

	return &httpclient.Request{
		Method:      http.MethodPost,
		URL:         t.baseURL + t.videoPath,
		Headers:     http.Header{"Content-Type": []string{"application/json"}, "Accept": []string{"application/json"}},
		Body:        body,
		Auth:        &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: t.apiKeyProvider.Get(ctx)},
		RequestType: llm.RequestTypeVideo.String(),
		APIFormat:   llm.APIFormatZenmuxVideo.String(),
		TransformerMetadata: map[string]any{
			"model": request.Model,
		},
	}, nil
}

func buildNativeContent(content []llm.VideoContent) ([]nativeContent, error) {
	native := make([]nativeContent, 0, len(content))
	for index, item := range content {
		entry := nativeContent{Type: item.Type}
		switch item.Type {
		case "text":
			if strings.TrimSpace(item.Text) == "" || item.Role != "" || item.ImageURL != nil || item.VideoURL != nil || item.AudioURL != nil {
				return nil, fmt.Errorf("%w: invalid text content at index %d", transformer.ErrInvalidRequest, index)
			}
			entry.Text = item.Text
		case "image_url":
			if item.ImageURL == nil || strings.TrimSpace(item.ImageURL.URL) == "" || item.Text != "" || item.VideoURL != nil || item.AudioURL != nil || !validImageRole(item.Role) {
				return nil, fmt.Errorf("%w: invalid image_url content at index %d", transformer.ErrInvalidRequest, index)
			}
			entry.Role, entry.ImageURL = item.Role, &nativeMediaURL{URL: item.ImageURL.URL}
		case "video_url":
			if item.VideoURL == nil || strings.TrimSpace(item.VideoURL.URL) == "" || strings.TrimSpace(item.VideoURL.MIMEType) != "" || item.Text != "" || item.ImageURL != nil || item.AudioURL != nil || item.Role != "reference_video" {
				return nil, fmt.Errorf("%w: invalid video_url content at index %d", transformer.ErrInvalidRequest, index)
			}
			entry.Role, entry.VideoURL = item.Role, &nativeMediaURL{URL: item.VideoURL.URL}
		case "audio_url":
			if item.AudioURL == nil || strings.TrimSpace(item.AudioURL.URL) == "" || item.Text != "" || item.ImageURL != nil || item.VideoURL != nil || item.Role != "reference_audio" {
				return nil, fmt.Errorf("%w: invalid audio_url content at index %d", transformer.ErrInvalidRequest, index)
			}
			entry.Role, entry.AudioURL = item.Role, &nativeMediaURL{URL: item.AudioURL.URL}
		default:
			return nil, fmt.Errorf("%w: unsupported content type %q", transformer.ErrInvalidRequest, item.Type)
		}
		native = append(native, entry)
	}
	return native, nil
}

func validImageRole(role string) bool {
	switch role {
	case "", "first_frame", "last_frame", "reference_image":
		return true
	default:
		return false
	}
}

func (t *OutboundTransformer) parseCreateResponse(response *httpclient.Response) (*llm.Response, error) {
	parsed, err := parseNativeVideoResponse(response, "create")
	if err != nil {
		return nil, err
	}
	return toLLMVideoResponse(parsed, response), nil
}

func (t *OutboundTransformer) BuildGetVideoTaskRequest(ctx context.Context, providerTaskID string) (*httpclient.Request, error) {
	if strings.TrimSpace(providerTaskID) == "" {
		return nil, fmt.Errorf("%w: providerTaskID is required", transformer.ErrInvalidRequest)
	}
	return &httpclient.Request{
		Method:      http.MethodGet,
		URL:         t.baseURL + t.videoPath + "/" + providerTaskID,
		Headers:     http.Header{"Accept": []string{"application/json"}},
		Auth:        &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: t.apiKeyProvider.Get(ctx)},
		RequestType: llm.RequestTypeVideo.String(),
		APIFormat:   llm.APIFormatZenmuxVideo.String(),
	}, nil
}

func (t *OutboundTransformer) ParseGetVideoTaskResponse(ctx context.Context, response *httpclient.Response) (*llm.Response, error) {
	parsed, err := parseNativeVideoResponse(response, "poll")
	if err != nil {
		return nil, err
	}
	return toLLMVideoResponse(parsed, response), nil
}

func (t *OutboundTransformer) BuildDeleteVideoTaskRequest(ctx context.Context, providerTaskID string) (*httpclient.Request, error) {
	return nil, ErrVideoTaskDeleteUnsupported
}
