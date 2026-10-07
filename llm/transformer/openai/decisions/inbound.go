package decisions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xjson"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

type InboundTransformer struct{}

func NewInboundTransformer() *InboundTransformer {
	return &InboundTransformer{}
}

func (t *InboundTransformer) TransformRequest(_ context.Context, request *httpclient.Request) (*llm.Request, error) {
	if request == nil {
		return nil, fmt.Errorf("%w: http request is nil", transformer.ErrInvalidRequest)
	}
	if len(request.Body) == 0 {
		return nil, fmt.Errorf("%w: request body is empty", transformer.ErrInvalidRequest)
	}
	contentType := request.Headers.Get("Content-Type")
	if contentType != "" && !strings.Contains(strings.ToLower(contentType), "application/json") {
		return nil, fmt.Errorf("%w: unsupported content type: %s", transformer.ErrInvalidRequest, contentType)
	}

	var wire struct {
		Model     string            `json:"model"`
		Input     json.RawMessage   `json:"input"`
		Questions []json.RawMessage `json:"questions"`
		Stream    *bool             `json:"stream"`
	}
	decoder := json.NewDecoder(bytes.NewReader(request.Body))
	if err := decoder.Decode(&wire); err != nil {
		return nil, fmt.Errorf("%w: failed to decode decisions request: %w", transformer.ErrInvalidRequest, err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("%w: trailing data after decisions request", transformer.ErrInvalidRequest)
		}
		return nil, fmt.Errorf("%w: invalid trailing data after decisions request: %w", transformer.ErrInvalidRequest, err)
	}
	if wire.Stream != nil && *wire.Stream {
		return nil, fmt.Errorf("%w: streaming is not supported for decisions requests", transformer.ErrInvalidRequest)
	}
	if strings.TrimSpace(wire.Model) == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}
	if err := validateDecisionsInput(wire.Input); err != nil {
		return nil, fmt.Errorf("%w: %w", transformer.ErrInvalidRequest, err)
	}
	if len(wire.Questions) == 0 {
		return nil, fmt.Errorf("%w: questions are required", transformer.ErrInvalidRequest)
	}

	return &llm.Request{
		Model:       wire.Model,
		RawRequest:  request,
		RequestType: llm.RequestTypeDecisions,
		APIFormat:   llm.APIFormatOpenAIDecisions,
		Decisions: &llm.DecisionsRequest{
			Body:      append([]byte(nil), request.Body...),
			Model:     wire.Model,
			Input:     append(json.RawMessage(nil), wire.Input...),
			Questions: append([]json.RawMessage(nil), wire.Questions...),
		},
	}, nil
}

func validateDecisionsInput(input json.RawMessage) error {
	input = bytes.TrimSpace(input)
	if len(input) == 0 || bytes.Equal(input, []byte("null")) {
		return fmt.Errorf("input is required")
	}

	switch input[0] {
	case '"':
		var text string
		if err := json.Unmarshal(input, &text); err != nil {
			return fmt.Errorf("input must be a string or supported message structure")
		}
		if text == "" {
			return fmt.Errorf("input string cannot be empty")
		}
		return nil
	case '[':
		var items []json.RawMessage
		if err := json.Unmarshal(input, &items); err != nil {
			return fmt.Errorf("input must be a string or supported message structure")
		}
		if len(items) == 0 {
			return fmt.Errorf("input array cannot be empty")
		}
		for i, item := range items {
			if err := validateDecisionsInputItem(item); err != nil {
				return fmt.Errorf("input[%d]: %w", i, err)
			}
		}
		return nil
	default:
		return fmt.Errorf("input must be a string or supported message structure")
	}
}

func validateDecisionsInputItem(item json.RawMessage) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(item, &object); err != nil || object == nil {
		return fmt.Errorf("item must be an object")
	}

	var itemType string
	if err := json.Unmarshal(object["type"], &itemType); err != nil {
		return fmt.Errorf("type is required")
	}

	switch itemType {
	case "message":
		return validateDecisionsMessageContent(object["content"])
	case "input_text":
		return validateDecisionsNonEmptyString(object["text"], "text")
	case "input_image":
		return validateDecisionsNonEmptyString(object["image_url"], "image_url")
	default:
		return fmt.Errorf("unsupported item type %q", itemType)
	}
}

func validateDecisionsMessageContent(content json.RawMessage) error {
	content = bytes.TrimSpace(content)
	if len(content) == 0 || bytes.Equal(content, []byte("null")) {
		return fmt.Errorf("message content is required")
	}
	if content[0] == '"' {
		return validateDecisionsNonEmptyString(content, "content")
	}
	if content[0] != '[' {
		return fmt.Errorf("message content must be a string or content array")
	}

	var parts []json.RawMessage
	if err := json.Unmarshal(content, &parts); err != nil || len(parts) == 0 {
		return fmt.Errorf("message content array cannot be empty")
	}
	for i, part := range parts {
		if err := validateDecisionsInputItem(part); err != nil {
			return fmt.Errorf("content[%d]: %w", i, err)
		}
	}
	return nil
}

func validateDecisionsNonEmptyString(value json.RawMessage, field string) error {
	var text string
	if err := json.Unmarshal(value, &text); err != nil || text == "" {
		return fmt.Errorf("%s must be a non-empty string", field)
	}
	return nil
}

func (t *InboundTransformer) TransformResponse(_ context.Context, response *llm.Response) (*httpclient.Response, error) {
	if response == nil || response.Decisions == nil || len(response.Decisions.Body) == 0 {
		return nil, fmt.Errorf("decisions response is empty")
	}
	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
		Body:       append([]byte(nil), response.Decisions.Body...),
	}, nil
}

func (t *InboundTransformer) TransformStream(context.Context, streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
	return nil, fmt.Errorf("%w: decisions does not support streaming", transformer.ErrInvalidRequest)
}

func (t *InboundTransformer) AggregateStreamChunks(context.Context, []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("decisions does not support streaming")
}

func (t *InboundTransformer) TransformError(_ context.Context, err error) *httpclient.Error {
	errValue := reflect.ValueOf(err)
	if err == nil || (errValue.Kind() == reflect.Pointer && errValue.IsNil()) {
		return &httpclient.Error{StatusCode: http.StatusInternalServerError, Body: []byte(`{"error":{"message":"Internal server error","type":"api_error"}}`)}
	}

	if httpErr, ok := errors.AsType[*httpclient.Error](err); ok {
		if httpErr == nil {
			return &httpclient.Error{
				StatusCode: http.StatusInternalServerError,
				Status:     http.StatusText(http.StatusInternalServerError),
				Body:       xjson.MustMarshal(&openAIError{Detail: llm.ErrorDetail{Message: "An unexpected error occurred", Type: "unexpected_error"}}),
			}
		}
		return httpErr
	}

	if errors.Is(err, transformer.ErrInvalidRequest) {
		return &httpclient.Error{
			StatusCode: http.StatusBadRequest,
			Status:     http.StatusText(http.StatusBadRequest),
			Body:       xjson.MustMarshal(&openAIError{Detail: llm.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"}}),
		}
	}

	if responseErr, ok := errors.AsType[*llm.ResponseError](err); ok {
		if responseErr == nil {
			return &httpclient.Error{
				StatusCode: http.StatusInternalServerError,
				Status:     http.StatusText(http.StatusInternalServerError),
				Body:       xjson.MustMarshal(&openAIError{Detail: llm.ErrorDetail{Message: "An unexpected error occurred", Type: "unexpected_error"}}),
			}
		}
		return &httpclient.Error{
			StatusCode: responseErr.StatusCode,
			Status:     http.StatusText(responseErr.StatusCode),
			Body:       xjson.MustMarshal(&openAIError{Detail: responseErr.Detail}),
		}
	}

	return &httpclient.Error{
		StatusCode: http.StatusInternalServerError,
		Status:     http.StatusText(http.StatusInternalServerError),
		Body:       xjson.MustMarshal(&openAIError{Detail: llm.ErrorDetail{Message: err.Error(), Type: "internal_server_error"}}),
	}
}

type openAIError struct {
	Detail llm.ErrorDetail `json:"error"`
}

var _ transformer.Inbound = (*InboundTransformer)(nil)
