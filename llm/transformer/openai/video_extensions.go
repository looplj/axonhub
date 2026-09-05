package openai

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/looplj/axonhub/llm/transformer"
)

func parseVideoJSONRequest(body []byte) (*VideoCreateRequest, error) {
	var request VideoCreateRequest
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, fmt.Errorf("%w: failed to decode video request: %w", transformer.ErrInvalidRequest, err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, fmt.Errorf("%w: video request must be a JSON object", transformer.ErrInvalidRequest)
	}
	extras := make(map[string]json.RawMessage)
	for key, value := range fields {
		if !isKnownVideoRequestField(key) {
			extras[key] = value
		}
	}

	extraBody, err := mergeVideoExtraBody(request.ExtraBody, extras)
	if err != nil {
		return nil, err
	}
	request.ExtraBody = extraBody
	return &request, nil
}

func applyVideoMultipartExtensions(request *VideoCreateRequest, fields map[string]string) error {
	request.Ratio = strings.TrimSpace(fields["ratio"])
	request.Resolution = strings.TrimSpace(fields["resolution"])
	request.ServiceTier = strings.TrimSpace(fields["service_tier"])
	request.CallbackURL = strings.TrimSpace(fields["callback_url"])
	if raw := strings.TrimSpace(fields["execution_expires_after"]); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("%w: execution_expires_after must be an integer", transformer.ErrInvalidRequest)
		}
		request.ExecutionExpiresAfter = &value
	}
	if raw := strings.TrimSpace(fields["content"]); raw != "" {
		if err := json.Unmarshal([]byte(raw), &request.Content); err != nil || request.Content == nil {
			return fmt.Errorf("%w: content must be a JSON array", transformer.ErrInvalidRequest)
		}
	}

	if raw := strings.TrimSpace(fields["return_last_frame"]); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("%w: return_last_frame must be a boolean", transformer.ErrInvalidRequest)
		}
		request.ReturnLastFrame = &value
	}
	if raw := strings.TrimSpace(fields["tools"]); raw != "" {
		if err := json.Unmarshal([]byte(raw), &request.Tools); err != nil || request.Tools == nil {
			return fmt.Errorf("%w: tools must be a JSON array", transformer.ErrInvalidRequest)
		}
	}

	extras := make(map[string]json.RawMessage)
	for key, value := range fields {
		if isKnownVideoRequestField(key) {
			continue
		}
		raw := json.RawMessage(value)
		if !json.Valid(raw) {
			return fmt.Errorf("%w: native field %q must be valid JSON", transformer.ErrInvalidRequest, key)
		}
		extras[key] = raw
	}

	extraBody, err := mergeVideoExtraBody(json.RawMessage(fields["extra_body"]), extras)
	if err != nil {
		return err
	}
	request.ExtraBody = extraBody
	return nil
}

func mergeVideoExtraBody(explicit json.RawMessage, extras map[string]json.RawMessage) (json.RawMessage, error) {
	merged := make(map[string]json.RawMessage)
	if len(explicit) != 0 {
		if err := json.Unmarshal(explicit, &merged); err != nil || merged == nil {
			return nil, fmt.Errorf("%w: extra_body must be a JSON object", transformer.ErrInvalidRequest)
		}
	}
	for key, value := range extras {
		merged[key] = value
	}
	if len(merged) == 0 {
		return nil, nil
	}
	encoded, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to encode extra_body: %w", transformer.ErrInvalidRequest, err)
	}
	return encoded, nil
}

func isKnownVideoRequestField(field string) bool {
	switch field {
	case "model", "prompt", "input_reference", "content", "seconds", "size", "ratio", "resolution", "service_tier", "execution_expires_after", "callback_url", "return_last_frame", "tools", "extra_body":
		return true
	default:
		return false
	}
}
