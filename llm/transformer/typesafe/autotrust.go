// 职责: 在现有决策流水线中接入 AutoTrust 原生请求、响应与错误。
// 关联文件:
//
//	llm/systemone.go
//	llm/transformer/typesafe/inbound.go
//	llm/transformer/typesafe/outbound.go
//	llm/transformer/interfaces.go
//
// 注意事项:
//
//	上游模型由部署决定；网关 model 和 X-AxonHub-Model 必须在覆盖操作后清理。
//	原生正文保留扩展字段，概率与选项顺序不作转换。
package typesafe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strings"

	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

const AutoTrustModelHeader = "X-AxonHub-Model"

type AutoTrustInboundTransformer struct{}

func NewAutoTrustInboundTransformer() *AutoTrustInboundTransformer {
	return &AutoTrustInboundTransformer{}
}

type autoTrustWireRequest struct {
	Model    string          `json:"model"`
	Kind     string          `json:"kind"`
	State    json.RawMessage `json:"state"`
	Question *string         `json:"question"`
	Options  []*string       `json:"options"`
	Stream   bool            `json:"stream"`
}

func parseAutoTrustRequest(body []byte) (*autoTrustWireRequest, error) {
	var wire autoTrustWireRequest
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, fmt.Errorf("%w: invalid autotrust request: %w", transformer.ErrInvalidRequest, err)
	}
	if wire.Stream {
		return nil, fmt.Errorf("%w: autotrust does not support streaming", transformer.ErrInvalidRequest)
	}
	if wire.Kind != "choice" && wire.Kind != "noul" && wire.Kind != "score" {
		return nil, fmt.Errorf("%w: kind must be choice, noul, or score", transformer.ErrInvalidRequest)
	}
	if wire.Question == nil {
		return nil, fmt.Errorf("%w: question must be a string", transformer.ErrInvalidRequest)
	}
	state := bytes.TrimSpace(wire.State)
	if len(state) > 0 && state[0] != '"' && state[0] != '{' && state[0] != '[' {
		return nil, fmt.Errorf("%w: state must be a string, object, or array", transformer.ErrInvalidRequest)
	}
	if wire.Kind == "choice" && len(wire.Options) < 2 {
		return nil, fmt.Errorf("%w: choice requires at least two options", transformer.ErrInvalidRequest)
	}
	for _, option := range wire.Options {
		if option == nil {
			return nil, fmt.Errorf("%w: options must contain strings", transformer.ErrInvalidRequest)
		}
	}
	return &wire, nil
}

func validateAutoTrustJSONRequest(request *httpclient.Request) error {
	if request == nil {
		return fmt.Errorf("%w: http request is nil", transformer.ErrInvalidRequest)
	}
	contentType := request.ContentType
	if contentType == "" {
		contentType = request.Headers.Get("Content-Type")
	}
	if contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || (mediaType != "application/json" && !strings.HasSuffix(mediaType, "+json")) {
			return fmt.Errorf("%w: autotrust requests require application/json", transformer.ErrInvalidRequest)
		}
	}
	if len(request.Body) == 0 {
		return fmt.Errorf("%w: request body is empty", transformer.ErrInvalidRequest)
	}
	if !json.Valid(request.Body) {
		return fmt.Errorf("%w: request body must be valid JSON", transformer.ErrInvalidRequest)
	}
	return nil
}

func (t *AutoTrustInboundTransformer) TransformRequest(_ context.Context, request *httpclient.Request) (*llm.Request, error) {
	if err := validateAutoTrustJSONRequest(request); err != nil {
		return nil, err
	}
	wire, err := parseAutoTrustRequest(request.Body)
	if err != nil {
		return nil, err
	}
	model := strings.TrimSpace(wire.Model)
	headerModel := strings.TrimSpace(request.Headers.Get(AutoTrustModelHeader))
	if model != "" && headerModel != "" && model != headerModel {
		return nil, fmt.Errorf("%w: model and %s must match", transformer.ErrInvalidRequest, AutoTrustModelHeader)
	}
	if model == "" {
		model = headerModel
	}
	if model == "" {
		return nil, fmt.Errorf("%w: model or %s is required", transformer.ErrInvalidRequest, AutoTrustModelHeader)
	}
	return &llm.Request{
		Model:       model,
		RequestType: llm.RequestTypeSystemOne,
		APIFormat:   llm.APIFormatAutoTrustDecide,
		RawRequest:  request,
		SystemOne: &llm.SystemOneRequest{AutoTrust: &llm.AutoTrustRequest{
			Body: bytes.Clone(request.Body),
		}},
	}, nil
}

func (t *AutoTrustInboundTransformer) TransformResponse(_ context.Context, response *llm.Response) (*httpclient.Response, error) {
	if response == nil || response.SystemOne == nil || response.SystemOne.AutoTrust == nil || len(response.SystemOne.AutoTrust.Body) == 0 {
		return nil, fmt.Errorf("autotrust response is empty")
	}
	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
		Body:       bytes.Clone(response.SystemOne.AutoTrust.Body),
	}, nil
}

func (t *AutoTrustInboundTransformer) TransformError(ctx context.Context, err error) *httpclient.Error {
	if responseErr, ok := errors.AsType[*llm.ResponseError](err); ok {
		if responseErr == nil {
			return (&SystemOneInboundTransformer{}).TransformError(ctx, nil)
		}
		// 仅直接上游错误保留原文；上层转换后的错误详情不能被深层 Cause 覆盖。
		if httpErr, ok := responseErr.Cause.(*httpclient.Error); ok && httpErr != nil {
			return httpErr
		}
		return (&SystemOneInboundTransformer{}).TransformError(ctx, responseErr)
	}
	if httpErr, ok := errors.AsType[*httpclient.Error](err); ok {
		if httpErr == nil {
			return (&SystemOneInboundTransformer{}).TransformError(ctx, nil)
		}
		return httpErr
	}
	if errors.Is(err, transformer.ErrInvalidRequest) {
		body, marshalErr := json.Marshal(map[string]any{
			"error": llm.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		if marshalErr != nil {
			return &httpclient.Error{StatusCode: http.StatusBadRequest, Body: []byte(`{"error":{"message":"Invalid request","type":"invalid_request_error"}}`)}
		}
		return &httpclient.Error{StatusCode: http.StatusBadRequest, Body: body}
	}
	return (&SystemOneInboundTransformer{}).TransformError(ctx, err)
}

func (t *AutoTrustInboundTransformer) TransformStream(context.Context, streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
	return nil, fmt.Errorf("%w: autotrust does not support streaming", transformer.ErrInvalidRequest)
}

func (t *AutoTrustInboundTransformer) AggregateStreamChunks(context.Context, []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("autotrust does not support streaming")
}

type AutoTrustOutboundTransformer struct {
	config Config
	rawURL bool
}

func NewAutoTrustOutboundTransformerWithConfig(config *Config) (*AutoTrustOutboundTransformer, error) {
	if config == nil || config.APIKeyProvider == nil {
		return nil, fmt.Errorf("config and API key provider are required")
	}
	if strings.TrimSpace(config.BaseURL) == "" {
		return nil, fmt.Errorf("base URL is required")
	}
	copied := *config
	rawURL := strings.HasSuffix(copied.BaseURL, "##")
	if rawURL {
		copied.BaseURL = strings.TrimRight(strings.TrimSuffix(copied.BaseURL, "##"), "/")
	} else if copied.EndpointPath != "" {
		copied.BaseURL = transformer.NormalizeBaseURL(copied.BaseURL, "")
	} else {
		copied.BaseURL = transformer.NormalizeBaseURL(copied.BaseURL, "v1")
	}
	return &AutoTrustOutboundTransformer{config: copied, rawURL: rawURL}, nil
}

func (t *AutoTrustOutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatAutoTrustDecide
}

func (t *AutoTrustOutboundTransformer) AllowPassThroughBody(context.Context, *llm.Request, *httpclient.Request) bool {
	return false
}

func (t *AutoTrustOutboundTransformer) AllowPassThroughResponse(context.Context, *llm.Request, *httpclient.Request) bool {
	return false
}

func (t *AutoTrustOutboundTransformer) TransformRequest(ctx context.Context, request *llm.Request) (*httpclient.Request, error) {
	if request == nil || request.SystemOne == nil || request.SystemOne.AutoTrust == nil {
		return nil, fmt.Errorf("autotrust request is required")
	}
	if strings.TrimSpace(request.Model) == "" {
		return nil, fmt.Errorf("model is required")
	}
	if request.Stream != nil && *request.Stream {
		return nil, fmt.Errorf("%w: autotrust does not support streaming", transformer.ErrInvalidRequest)
	}
	if _, err := parseAutoTrustRequest(request.SystemOne.AutoTrust.Body); err != nil {
		return nil, err
	}
	body, err := sjson.DeleteBytes(request.SystemOne.AutoTrust.Body, "model")
	if err != nil {
		return nil, fmt.Errorf("failed to remove autotrust gateway model: %w", err)
	}
	url := t.config.BaseURL
	if !t.rawURL {
		if t.config.EndpointPath != "" {
			url += t.config.EndpointPath
		} else {
			url += "/decide"
		}
	}
	return &httpclient.Request{
		Method:      http.MethodPost,
		URL:         url,
		Headers:     http.Header{"Content-Type": []string{"application/json"}, "Accept": []string{"application/json"}},
		Body:        body,
		Auth:        &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: t.config.APIKeyProvider.Get(ctx)},
		RequestType: llm.RequestTypeSystemOne.String(),
		APIFormat:   llm.APIFormatAutoTrustDecide.String(),
	}, nil
}

func (t *AutoTrustOutboundTransformer) FinalizeTransportRequest(request *httpclient.Request) *httpclient.Request {
	if request == nil {
		return nil
	}
	cloned := *request
	cloned.Headers = request.Headers.Clone()
	for name := range cloned.Headers {
		if strings.EqualFold(name, AutoTrustModelHeader) {
			delete(cloned.Headers, name)
		}
	}
	// 正文经过原生解析和 JSON 参数覆盖；固定字段路径不产生路径解析错误。
	cloned.Body, _ = sjson.DeleteBytes(request.Body, "model")
	if len(request.JSONBody) > 0 {
		cloned.JSONBody = bytes.Clone(cloned.Body)
	}
	return &cloned
}

func (t *AutoTrustOutboundTransformer) TransformResponse(ctx context.Context, response *httpclient.Response) (*llm.Response, error) {
	if response == nil {
		return nil, fmt.Errorf("http response is nil")
	}
	if response.StatusCode >= http.StatusMultipleChoices {
		return nil, t.TransformError(ctx, &httpclient.Error{
			StatusCode: response.StatusCode,
			Headers:    response.Headers,
			Body:       response.Body,
		})
	}
	var wire struct {
		Model         string     `json:"model"`
		Kind          string     `json:"kind"`
		Options       []*string  `json:"options"`
		Probabilities []*float64 `json:"probabilities"`
		ChoiceIndex   *int       `json:"choice_index"`
		Choice        *string    `json:"choice"`
		Usage         *struct {
			PromptTokens     *int64 `json:"prompt_tokens"`
			CompletionTokens *int64 `json:"completion_tokens"`
			TotalTokens      *int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(response.Body, &wire); err != nil {
		return nil, fmt.Errorf("%w: invalid autotrust response: %w", transformer.ErrInvalidResponse, err)
	}
	if wire.Kind != "choice" && wire.Kind != "noul" && wire.Kind != "score" {
		return nil, fmt.Errorf("%w: invalid autotrust kind", transformer.ErrInvalidResponse)
	}
	if len(wire.Options) < 2 || len(wire.Options) != len(wire.Probabilities) {
		return nil, fmt.Errorf("%w: autotrust options and probabilities must have matching non-empty lengths", transformer.ErrInvalidResponse)
	}
	if wire.ChoiceIndex == nil || *wire.ChoiceIndex < 0 || *wire.ChoiceIndex >= len(wire.Options) {
		return nil, fmt.Errorf("%w: invalid autotrust choice_index", transformer.ErrInvalidResponse)
	}
	parsed := &llm.AutoTrustResponse{
		Body:          bytes.Clone(response.Body),
		Kind:          wire.Kind,
		Options:       make([]string, len(wire.Options)),
		Probabilities: make([]float64, len(wire.Probabilities)),
		ChoiceIndex:   wire.ChoiceIndex,
	}
	for i, probability := range wire.Probabilities {
		if probability == nil || *probability < 0 || *probability > 1 || wire.Options[i] == nil {
			return nil, fmt.Errorf("%w: autotrust options must be strings and probabilities must be numbers from 0 to 1", transformer.ErrInvalidResponse)
		}
		parsed.Options[i] = *wire.Options[i]
		parsed.Probabilities[i] = *probability
	}
	if wire.Choice == nil || *wire.Choice != parsed.Options[*wire.ChoiceIndex] {
		return nil, fmt.Errorf("%w: autotrust choice must match choice_index", transformer.ErrInvalidResponse)
	}
	parsed.Choice = *wire.Choice
	result := &llm.Response{
		Model:       wire.Model,
		RequestType: llm.RequestTypeSystemOne,
		APIFormat:   llm.APIFormatAutoTrustDecide,
		SystemOne:   &llm.SystemOneResponse{AutoTrust: parsed},
	}
	if wire.Usage != nil && (wire.Usage.PromptTokens != nil || wire.Usage.CompletionTokens != nil || wire.Usage.TotalTokens != nil) {
		usage := &llm.Usage{}
		if wire.Usage.PromptTokens != nil {
			usage.PromptTokens = *wire.Usage.PromptTokens
		}
		if wire.Usage.CompletionTokens != nil {
			usage.CompletionTokens = *wire.Usage.CompletionTokens
		}
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
		if wire.Usage.TotalTokens != nil {
			usage.TotalTokens = *wire.Usage.TotalTokens
		}
		if usage.PromptTokens < 0 || usage.CompletionTokens < 0 || usage.TotalTokens < 0 {
			return nil, fmt.Errorf("%w: autotrust usage cannot be negative", transformer.ErrInvalidResponse)
		}
		result.Usage = usage
	}
	return result, nil
}

func (t *AutoTrustOutboundTransformer) TransformError(ctx context.Context, err *httpclient.Error) *llm.ResponseError {
	return (&OutboundTransformer{}).TransformError(ctx, err)
}

func (t *AutoTrustOutboundTransformer) TransformStream(context.Context, *httpclient.Request, streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return nil, fmt.Errorf("%w: autotrust does not support streaming", transformer.ErrInvalidRequest)
}

func (t *AutoTrustOutboundTransformer) AggregateStreamChunks(context.Context, *httpclient.Request, []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("autotrust does not support streaming")
}

var (
	_ transformer.Inbound                   = (*AutoTrustInboundTransformer)(nil)
	_ transformer.Outbound                  = (*AutoTrustOutboundTransformer)(nil)
	_ transformer.PassThroughBodyPolicy     = (*AutoTrustOutboundTransformer)(nil)
	_ transformer.PassThroughResponsePolicy = (*AutoTrustOutboundTransformer)(nil)
	_ transformer.TransportRequestFinalizer = (*AutoTrustOutboundTransformer)(nil)
)
