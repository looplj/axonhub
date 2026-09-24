package modelscope

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xurl"
	"github.com/looplj/axonhub/llm/transformer"
)

const (
	modelScopeImagePath = "/images/generations"
	modelScopeTasksPath = "/tasks/"

	defaultImageTaskPollInterval = 2 * time.Second
	defaultImageTaskTimeout      = 15 * time.Minute
	defaultImageDownloadTimeout  = 2 * time.Minute

	// defaultImageSize replaces the OpenAI "auto" size, which ModelScope does
	// not accept. It is the resolution the gateway asks for when the caller does
	// not pin one, so a caller that only knows "auto" still gets a concrete size.
	defaultImageSize = "2048x2048"
)

// imageSizeOrDefault resolves the size sent to ModelScope. ModelScope takes a
// WxH string and has no "auto", but OpenAI-shaped callers (the Codex image tool
// among them) always send "auto", so it is mapped onto the gateway default
// rather than being dropped and left to the upstream default.
func imageSizeOrDefault(size string) string {
	if s := strings.TrimSpace(size); s != "" && !strings.EqualFold(s, "auto") {
		return s
	}

	return defaultImageSize
}

// imageTaskAPIKeyMeta carries the API key that submitted the task from the
// outbound request through to the response transformation.
//
// A ModelScope task belongs to the key that created it, so polling and result
// download must reuse that key. Re-resolving the key per call is not safe when a
// channel has several keys: the provider may hand out a different one, and the
// upstream then reports the task as missing. The auth config is cleared before
// the response is transformed, so the key travels as request metadata.
const imageTaskAPIKeyMeta = "api_key"

// imageResponse covers both the async task submission response (task_id) and the
// task query response (task_status plus output_images).
type imageResponse struct {
	Created      int64             `json:"created"`
	Data         []llm.ImageData   `json:"data"`
	TaskID       string            `json:"task_id"`
	TaskStatus   string            `json:"task_status"`
	OutputImages []json.RawMessage `json:"output_images"`
	Message      string            `json:"message"`
	Errors       *struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// buildImageRequest converts a unified image request into a ModelScope image task
// submission. Image generation and image editing share this endpoint; a top-level
// image_url is what turns the call into an edit.
//
// X-ModelScope-Async-Mode is always sent: the documented contract for this
// endpoint is an asynchronous task, and models that cannot serve a synchronous
// call reject a headerless submission outright. The polling half of the round
// trip requires X-ModelScope-Task-Type, which waitImageTask supplies.
func (t *OutboundTransformer) buildImageRequest(ctx context.Context, req *llm.Request) (*httpclient.Request, error) {
	if req == nil || req.Image == nil {
		return nil, fmt.Errorf("%w: image request is required", transformer.ErrInvalidRequest)
	}

	switch req.APIFormat {
	case llm.APIFormatOpenAIImageGeneration, llm.APIFormatOpenAIImageEdit:
	default:
		return nil, fmt.Errorf("%w: ModelScope does not support image api format %q", transformer.ErrInvalidRequest, req.APIFormat)
	}

	prompt := strings.TrimSpace(req.Image.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("%w: prompt is required for image requests", transformer.ErrInvalidRequest)
	}

	if len(req.Image.Mask) > 0 {
		return nil, fmt.Errorf("%w: ModelScope image requests do not support masks", transformer.ErrInvalidRequest)
	}

	body := map[string]any{
		"model":  req.Model,
		"prompt": prompt,
	}

	body["size"] = imageSizeOrDefault(req.Image.Size)

	if len(req.Image.Images) > 0 {
		images := make([]string, 0, len(req.Image.Images))
		for _, data := range req.Image.Images {
			images = append(images, encodeImageBytesToDataURL(data))
		}

		if len(images) == 1 {
			body["image_url"] = images[0]
		} else {
			body["image_url"] = images
		}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ModelScope image request: %w", err)
	}

	headers := make(http.Header)
	headers.Set("Accept", "application/json")
	headers.Set("Content-Type", "application/json")
	headers.Set("X-ModelScope-Async-Mode", "true")

	// Resolve the key once: the task is created with it, so polling and download
	// must reuse exactly this key rather than resolving a new one per call.
	apiKey := t.apiKeyProvider.Get(ctx)
	if apiKey == "" {
		return nil, fmt.Errorf("%w: no API key available for ModelScope image request", transformer.ErrInvalidRequest)
	}

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     strings.TrimRight(t.baseURL, "/") + modelScopeImagePath,
		Headers: headers,
		Body:    payload,
		Auth: &httpclient.AuthConfig{
			Type:   httpclient.AuthTypeBearer,
			APIKey: apiKey,
		},
		// The provider-facing format is the ModelScope one so the async task
		// response is never mistaken for a ready-to-use OpenAI image response.
		RequestType: llm.RequestTypeImage.String(),
		APIFormat:   llm.APIFormatModelScopeImage.String(),
	}
	httpReq.TransformerMetadata = map[string]any{
		"model":             req.Model,
		imageTaskAPIKeyMeta: apiKey,
	}

	return httpReq, nil
}

// TransformResponse resolves the ModelScope async image task into a standard
// image response, delegating every other request type to the OpenAI transformer.
func (t *OutboundTransformer) TransformResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	if httpResp == nil || httpResp.Request == nil {
		return t.Outbound.TransformResponse(ctx, httpResp)
	}

	if httpResp.Request.APIFormat != llm.APIFormatModelScopeImage.String() {
		return t.Outbound.TransformResponse(ctx, httpResp)
	}

	return t.transformImageResponse(ctx, httpResp)
}

func (t *OutboundTransformer) transformImageResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("ModelScope image response body is empty")
	}

	// Reuse the key that created the task for the whole round trip.
	apiKey := imageTaskAPIKey(httpResp, t.apiKeyProvider.Get(ctx))

	var payload imageResponse
	if err := json.Unmarshal(httpResp.Body, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode ModelScope image response: %w", err)
	}

	if len(payload.Data) > 0 {
		return t.imageResponseFromData(ctx, httpResp, payload.Created, payload.Data)
	}

	if len(payload.OutputImages) > 0 {
		data, err := parseOutputImages(payload.OutputImages)
		if err != nil {
			return nil, err
		}

		return t.imageResponseFromData(ctx, httpResp, payload.Created, data)
	}

	if payload.TaskID == "" {
		return nil, fmt.Errorf("ModelScope image response has neither image data nor task_id: %s", strings.TrimSpace(string(httpResp.Body)))
	}

	task, err := t.waitImageTask(ctx, apiKey, payload.TaskID)
	if err != nil {
		return nil, err
	}

	data, err := parseOutputImages(task.OutputImages)
	if err != nil {
		return nil, err
	}

	return t.imageResponseFromData(ctx, httpResp, task.Created, data)
}

func (t *OutboundTransformer) waitImageTask(ctx context.Context, apiKey, taskID string) (*imageResponse, error) {
	// Follow the caller's deadline when it has one: the gateway already bounds
	// non-streaming requests, so inventing a longer inner budget would only make
	// the transformer keep polling for a request the caller has already given up
	// on. The fixed timeout below is the fallback for callers without a deadline.
	deadline := time.Time{}
	if d, ok := ctx.Deadline(); ok {
		deadline = d
	} else if t.taskTimeout > 0 {
		deadline = time.Now().Add(t.taskTimeout)
	} else {
		deadline = time.Now().Add(defaultImageTaskTimeout)
	}

	taskURL := strings.TrimRight(t.baseURL, "/") + modelScopeTasksPath + url.PathEscape(taskID)

	for {
		pollCtx, cancel := context.WithDeadline(ctx, deadline)
		task, err := t.getImageTask(pollCtx, apiKey, taskURL)
		cancel()

		if err != nil {
			return nil, err
		}

		switch strings.ToUpper(strings.TrimSpace(task.TaskStatus)) {
		case "SUCCEED", "SUCCESS":
			return task, nil
		case "FAILED":
			return nil, fmt.Errorf("ModelScope image task %s failed: %s", taskID, imageTaskErrorMessage(task))
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("ModelScope image task %s timed out", taskID)
		}

		interval := t.pollInterval
		if interval <= 0 {
			interval = defaultImageTaskPollInterval
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}

func (t *OutboundTransformer) getImageTask(ctx context.Context, apiKey, taskURL string) (*imageResponse, error) {
	headers := make(http.Header)
	headers.Set("Accept", "application/json")
	headers.Set("X-ModelScope-Task-Type", "image_generation")

	request := &httpclient.Request{
		Method:  http.MethodGet,
		URL:     taskURL,
		Headers: headers,
		Auth: &httpclient.AuthConfig{
			Type:   httpclient.AuthTypeBearer,
			APIKey: apiKey,
		},
	}

	if t.httpClient != nil {
		resp, err := t.httpClient.Do(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("failed to query ModelScope image task: %w", err)
		}

		return decodeImageTask(resp.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, taskURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create ModelScope image task request: %w", err)
	}

	httpReq.Header = headers
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to query ModelScope image task: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read ModelScope image task response: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("ModelScope image task query failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return decodeImageTask(body)
}

// imageTaskAPIKey returns the key that created the task, falling back to the
// provided key when the request metadata is unavailable. Every task-scoped call
// must use the returned key so a multi-key channel cannot mix keys mid-task.
func imageTaskAPIKey(httpResp *httpclient.Response, fallback string) string {
	if httpResp != nil && httpResp.Request != nil && httpResp.Request.TransformerMetadata != nil {
		if key, ok := httpResp.Request.TransformerMetadata[imageTaskAPIKeyMeta].(string); ok && key != "" {
			return key
		}
	}

	return fallback
}

func decodeImageTask(body []byte) (*imageResponse, error) {
	var task imageResponse
	if err := json.Unmarshal(body, &task); err != nil {
		return nil, fmt.Errorf("failed to decode ModelScope image task response: %w", err)
	}

	return &task, nil
}

func (t *OutboundTransformer) imageResponseFromData(
	ctx context.Context,
	httpResp *httpclient.Response,
	created int64,
	data []llm.ImageData,
) (*llm.Response, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("ModelScope image response contains no images")
	}

	if created == 0 {
		created = time.Now().Unix()
	}

	model := "image-generation"
	if httpResp.Request != nil && httpResp.Request.TransformerMetadata != nil {
		if m, ok := httpResp.Request.TransformerMetadata["model"].(string); ok && m != "" {
			model = m
		}
	}

	imageResp := &llm.ImageResponse{
		Created: created,
		Data:    make([]llm.ImageData, 0, len(data)),
	}

	for _, item := range data {
		b64 := item.B64JSON
		if b64 == "" && item.URL != "" {
			var err error

			b64, err = t.downloadImageBase64(ctx, item.URL)
			if err != nil {
				return nil, err
			}
		}

		imageResp.Data = append(imageResp.Data, llm.ImageData{
			B64JSON:       b64,
			URL:           item.URL,
			RevisedPrompt: item.RevisedPrompt,
		})
	}

	return &llm.Response{
		ID:          fmt.Sprintf("modelscope-image-%d", created),
		Object:      "chat.completion",
		Created:     created,
		Model:       model,
		RequestType: llm.RequestTypeImage,
		APIFormat:   llm.APIFormatModelScopeImage,
		Image:       imageResp,
	}, nil
}

func parseOutputImages(raw []json.RawMessage) ([]llm.ImageData, error) {
	images := make([]llm.ImageData, 0, len(raw))

	for _, item := range raw {
		var imageURL string
		if err := json.Unmarshal(item, &imageURL); err == nil && imageURL != "" {
			images = append(images, llm.ImageData{URL: imageURL})

			continue
		}

		var image struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		}
		if err := json.Unmarshal(item, &image); err != nil {
			return nil, fmt.Errorf("failed to decode ModelScope output image: %w", err)
		}

		if image.URL == "" && image.B64JSON == "" {
			continue
		}

		images = append(images, llm.ImageData{URL: image.URL, B64JSON: image.B64JSON})
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("ModelScope image response contains no output images")
	}

	return images, nil
}

// downloadImageBase64 materializes an output image as base64. Codex built-in
// image tooling only reads data[].b64_json, and ModelScope returns URLs, so the
// conversion has to happen here rather than being delegated to the client.
func (t *OutboundTransformer) downloadImageBase64(ctx context.Context, imageURL string) (string, error) {
	if xurl.IsDataURL(imageURL) {
		parsed := xurl.ParseDataURL(imageURL)
		if parsed == nil || !parsed.IsBase64 {
			return "", fmt.Errorf("invalid image data URL")
		}

		return parsed.Data, nil
	}

	downloadCtx, cancel := context.WithTimeout(ctx, defaultImageDownloadTimeout)
	defer cancel()

	if t.httpClient != nil {
		resp, err := t.httpClient.Do(downloadCtx, &httpclient.Request{
			Method: http.MethodGet,
			URL:    imageURL,
		})
		if err != nil {
			return "", fmt.Errorf("failed to download ModelScope output image: %w", err)
		}

		if len(resp.Body) == 0 {
			return "", fmt.Errorf("downloaded ModelScope output image is empty")
		}

		return base64.StdEncoding.EncodeToString(resp.Body), nil
	}

	req, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create image download request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download ModelScope output image: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("failed to download ModelScope output image: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read ModelScope output image: %w", err)
	}

	if len(body) == 0 {
		return "", fmt.Errorf("downloaded ModelScope output image is empty")
	}

	return base64.StdEncoding.EncodeToString(body), nil
}

func encodeImageBytesToDataURL(data []byte) string {
	mediaType := http.DetectContentType(data)
	if !strings.HasPrefix(mediaType, "image/") {
		mediaType = "image/png"
	}

	return xurl.BuildDataURL(mediaType, base64.StdEncoding.EncodeToString(data), true)
}

func imageTaskErrorMessage(task *imageResponse) string {
	if task == nil {
		return "unknown error"
	}

	if task.Message != "" {
		return task.Message
	}

	if task.Errors != nil && task.Errors.Message != "" {
		return task.Errors.Message
	}

	return "unknown error"
}
