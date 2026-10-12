package responses

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

// IsResponsesFormat excludes compact and native Images from transport adaptation.
func IsResponsesFormat(format llm.APIFormat) bool {
	return format == llm.APIFormatOpenAIResponse || format == llm.APIFormatOpenAIResponseWebSocket
}

func checkResponsesFormats(source, target llm.APIFormat) error {
	if !IsResponsesFormat(source) || !IsResponsesFormat(target) {
		return fmt.Errorf("%w: Responses transport formats required", transformer.ErrUnsupportedConversion)
	}
	return nil
}

type protocolResponseBody struct {
	ID     string            `json:"id"`
	Status string            `json:"status"`
	Output []json.RawMessage `json:"output"`
}

// AdaptRequestBody preserves Responses fields without a generic-model roundtrip.
// It accepts normalized WS bodies or response.create frames. Lane routing belongs
// to the caller; stream_id is never forwarded upstream. This does not certify or
// expand continuation history and must precede provider sanitization/finalization.
func AdaptRequestBody(body []byte, source, target llm.APIFormat) ([]byte, error) {
	if err := checkResponsesFormats(source, target); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, fmt.Errorf("%w: invalid Responses object: %w", transformer.ErrInvalidRequest, err)
	}
	var envelope struct {
		Model    string `json:"model"`
		Type     string `json:"type"`
		Generate *bool  `json:"generate"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("%w: invalid Responses fields: %w", transformer.ErrInvalidRequest, err)
	}
	if envelope.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}
	if envelope.Type != "" && (source != llm.APIFormatOpenAIResponseWebSocket || envelope.Type != "response.create") {
		return nil, fmt.Errorf("%w: response.create is required", transformer.ErrUnsupportedConversion)
	}
	if target == llm.APIFormatOpenAIResponse && envelope.Generate != nil && !*envelope.Generate {
		return nil, fmt.Errorf("%w: HTTP cannot perform WebSocket warmup", transformer.ErrUnsupportedConversion)
	}
	delete(fields, "type")
	delete(fields, "stream_id")
	delete(fields, "generate")
	if target == llm.APIFormatOpenAIResponseWebSocket {
		fields["type"] = json.RawMessage(`"response.create"`)
		if envelope.Generate != nil && !*envelope.Generate {
			fields["generate"] = json.RawMessage("false")
		}
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("%w: encode Responses object: %w", transformer.ErrInvalidRequest, err)
	}
	return encoded, nil
}

// AdaptResponseBody keeps provider response/item IDs and unknown fields verbatim.
// HTTP response bodies and aggregated WS responses share the same JSON object.
// A valid object is not evidence of complete or recoverable continuation history.
func AdaptResponseBody(body []byte, source, target llm.APIFormat) ([]byte, error) {
	if err := checkResponsesFormats(source, target); err != nil {
		return nil, err
	}
	var response protocolResponseBody
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("%w: invalid Responses response: %w", transformer.ErrInvalidResponse, err)
	}
	if response.ID == "" {
		return nil, fmt.Errorf("%w: provider response ID is required", transformer.ErrInvalidResponse)
	}
	return bytes.Clone(body), nil
}

// AdaptStreamEvent adapts decoded SSE/WS events without rebuilding provider items.
// The executor/handler remains responsible for SSE framing and WS lane envelopes.
func AdaptStreamEvent(event *httpclient.StreamEvent, source, target llm.APIFormat) (*httpclient.StreamEvent, error) {
	if err := checkResponsesFormats(source, target); err != nil {
		return nil, err
	}
	if event == nil {
		return nil, fmt.Errorf("%w: event is nil", transformer.ErrInvalidResponse)
	}
	var envelope struct {
		Type     string                `json:"type"`
		Response *protocolResponseBody `json:"response"`
	}
	if err := json.Unmarshal(event.Data, &envelope); err != nil {
		return nil, fmt.Errorf("%w: invalid Responses event: %w", transformer.ErrInvalidResponse, err)
	}
	if envelope.Type == "" || (event.Type != "" && event.Type != envelope.Type) {
		return nil, fmt.Errorf("%w: event type is missing or inconsistent", transformer.ErrInvalidResponse)
	}
	if envelope.Type == "response.completed" {
		if envelope.Response == nil || envelope.Response.ID == "" || envelope.Response.Status != "completed" {
			return nil, fmt.Errorf("%w: completed event requires a completed provider response", transformer.ErrInvalidResponse)
		}
	}
	cloned := *event
	cloned.Type = envelope.Type
	cloned.Data = bytes.Clone(event.Data)
	cloned.Headers = event.Headers.Clone()
	return &cloned, nil
}
