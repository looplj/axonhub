package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xurl"
	"github.com/looplj/axonhub/llm/transformer"
)

func parseEditImageInputs(image, images json.RawMessage) ([][]byte, []llm.ImageReference, error) {
	legacy, err := parseGenerationImageField(image)
	if err != nil || len(legacy) > 0 {
		return legacy, nil, err
	}
	if len(images) == 0 {
		return nil, nil, nil
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(images, &entries); err != nil {
		return nil, nil, fmt.Errorf("%w: images must be an array", transformer.ErrInvalidRequest)
	}
	if len(entries) > maxImageCount {
		return nil, nil, fmt.Errorf("%w: too many images", transformer.ErrInvalidRequest)
	}
	var data [][]byte
	refs := make([]llm.ImageReference, 0, len(entries))
	for i, entry := range entries {
		ref, decoded, err := parseEditImageReference(entry)
		if err != nil {
			return nil, nil, fmt.Errorf("images[%d]: %w", i, err)
		}
		refs = append(refs, ref)
		if decoded != nil {
			data = append(data, decoded)
		}
	}
	return data, refs, nil
}

func parseEditImageReference(raw json.RawMessage) (llm.ImageReference, []byte, error) {
	var ref llm.ImageReference
	var legacy string
	if err := json.Unmarshal(raw, &legacy); err == nil {
		data, err := decodeDataURLToBytes(strings.TrimSpace(legacy))
		return llm.ImageReference{ImageURL: strings.TrimSpace(legacy)}, data, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&ref); err != nil {
		return ref, nil, fmt.Errorf("%w: invalid image reference: %w", transformer.ErrInvalidRequest, err)
	}
	if (strings.TrimSpace(ref.ImageURL) == "") == (strings.TrimSpace(ref.FileID) == "") {
		return ref, nil, fmt.Errorf("%w: image reference requires exactly one image_url or file_id", transformer.ErrInvalidRequest)
	}
	if ref.FileID != "" {
		return ref, nil, nil
	}
	if xurl.IsDataURL(ref.ImageURL) {
		data, err := decodeDataURLToBytes(ref.ImageURL)
		return ref, data, err
	}
	u, err := url.Parse(ref.ImageURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return ref, nil, fmt.Errorf("%w: image_url must be an HTTP URL or image data URL", transformer.ErrInvalidRequest)
	}
	return ref, nil, nil
}

func (t *OutboundTransformer) buildImageEditJSONRequest(req *llm.Request, apiKey string) (*httpclient.Request, error) {
	img := *req.Image
	if len(img.ImageReferences) == 0 {
		for _, data := range img.Images {
			img.ImageReferences = append(img.ImageReferences, llm.ImageReference{ImageURL: encodeImageBytesToDataURL(data)})
		}
	}
	if len(img.ImageReferences) == 0 {
		return nil, fmt.Errorf("%w: at least one image is required for editing", transformer.ErrInvalidRequest)
	}
	if img.MaskReference == nil && len(img.Mask) > 0 {
		img.MaskReference = &llm.ImageReference{ImageURL: encodeImageBytesToDataURL(img.Mask)}
	}
	if img.PartialImages != nil || img.ResponseFormat != "" || img.Style != "" {
		return nil, fmt.Errorf("%w: partial_images, response_format and style are unsupported for JSON image edits", transformer.ErrInvalidRequest)
	}
	body, err := json.Marshal(struct {
		Model string `json:"model"`
		*llm.ImageRequest
	}{Model: req.Model, ImageRequest: &img})
	if err != nil {
		return nil, fmt.Errorf("marshal JSON image edit: %w", err)
	}
	return &httpclient.Request{
		Method: http.MethodPost, URL: t.imageRequestURL("/images/edits"), Body: body,
		Headers: http.Header{"Content-Type": {"application/json"}, "Accept": {"application/json"}},
		Auth:    &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: apiKey},
	}, nil
}

func (t *OutboundTransformer) imageRequestURL(path string) string {
	if t.config.RawURL {
		return strings.TrimRight(t.config.BaseURL, "/")
	}
	if t.config.EndpointPath != "" {
		path = t.config.EndpointPath
	}
	return t.config.BaseURL + path
}
