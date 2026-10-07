package decisions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
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
	if wire.Stream != nil && *wire.Stream {
		return nil, fmt.Errorf("%w: streaming is not supported for decisions requests", transformer.ErrInvalidRequest)
	}
	if strings.TrimSpace(wire.Model) == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}
	if len(wire.Input) == 0 || bytes.Equal(wire.Input, []byte("null")) {
		return nil, fmt.Errorf("%w: input is required", transformer.ErrInvalidRequest)
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
	if err == nil {
		return &httpclient.Error{StatusCode: http.StatusInternalServerError, Body: []byte(`{"error":{"message":"Internal server error","type":"api_error"}}`)}
	}
	return &httpclient.Error{StatusCode: http.StatusBadRequest, Body: []byte(fmt.Sprintf(`{"error":{"message":%q,"type":"invalid_request_error"}}`, err.Error()))}
}

var _ transformer.Inbound = (*InboundTransformer)(nil)
