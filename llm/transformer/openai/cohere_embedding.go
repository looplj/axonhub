// 职责: 在现有嵌入流水线中处理 Cohere Embed v2 原生文本与图像协议。
// 关联文件:
//
//	llm/embedding.go
//	llm/transformer/openai/outbound.go
//	llm/httpclient/model.go
//
// 遵循的 Rules:
//
//	.agents/rules/go-general.md
//
// 注意事项:
//
//	原生正文保留扩展字段；响应数量依据本次实际发送的请求校验，适配器不保存请求状态。
//	input_type 是否必填由目标部署决定，网关不补默认任务。
package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"mime"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

type CohereEmbeddingInboundTransformer struct{}

func NewCohereEmbeddingInboundTransformer() *CohereEmbeddingInboundTransformer {
	return &CohereEmbeddingInboundTransformer{}
}

func (t *CohereEmbeddingInboundTransformer) TransformRequest(_ context.Context, request *httpclient.Request) (*llm.Request, error) {
	if request == nil {
		return nil, fmt.Errorf("%w: http request is nil", transformer.ErrInvalidRequest)
	}
	contentType := request.Headers.Get("Content-Type")
	if contentType == "" {
		contentType = request.ContentType
	}
	if contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || mediaType != "application/json" {
			return nil, fmt.Errorf("%w: Cohere embed requires application/json", transformer.ErrInvalidRequest)
		}
	}
	model, native, err := parseCohereEmbeddingRequest(request.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", transformer.ErrInvalidRequest, err)
	}
	native.Body = append(json.RawMessage(nil), request.Body...)
	return &llm.Request{
		Model:       model,
		RawRequest:  request,
		RequestType: llm.RequestTypeEmbedding,
		APIFormat:   llm.APIFormatCohereEmbed,
		Embedding:   &llm.EmbeddingRequest{Cohere: native},
	}, nil
}

func parseCohereEmbeddingRequest(body []byte) (string, *llm.CohereEmbeddingRequest, error) {
	var wire struct {
		Model     string    `json:"model"`
		InputType string    `json:"input_type"`
		Stream    bool      `json:"stream"`
		Texts     []*string `json:"texts"`
		Images    []string  `json:"images"`
		Inputs    []struct {
			Content []struct {
				Type     string  `json:"type"`
				Text     *string `json:"text"`
				ImageURL *struct {
					URL string `json:"url"`
				} `json:"image_url"`
			} `json:"content"`
		} `json:"inputs"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		return "", nil, fmt.Errorf("invalid Cohere embed request: %w", err)
	}
	if strings.TrimSpace(wire.Model) == "" {
		return "", nil, fmt.Errorf("model is required")
	}
	if wire.Stream {
		return "", nil, fmt.Errorf("Cohere embed does not support streaming")
	}
	switch wire.InputType {
	case "", "search_document", "search_query", "classification", "clustering", "image":
	default:
		return "", nil, fmt.Errorf("non-empty input_type must be search_document, search_query, classification, clustering, or image")
	}
	forms := 0
	for _, present := range []bool{wire.Texts != nil, wire.Images != nil, wire.Inputs != nil} {
		if present {
			forms++
		}
	}
	if forms != 1 {
		return "", nil, fmt.Errorf("provide one of texts, images, or inputs")
	}
	count := len(wire.Texts) + len(wire.Images) + len(wire.Inputs)
	if count == 0 {
		return "", nil, fmt.Errorf("embedding input cannot be empty")
	}
	for i, text := range wire.Texts {
		if text == nil {
			return "", nil, fmt.Errorf("texts[%d] must be a string", i)
		}
	}
	for i, image := range wire.Images {
		if !strings.HasPrefix(image, "data:image/") {
			return "", nil, fmt.Errorf("images[%d] must be an image data URI", i)
		}
	}
	for i, input := range wire.Inputs {
		if len(input.Content) == 0 {
			return "", nil, fmt.Errorf("inputs[%d].content cannot be empty", i)
		}
		for j, part := range input.Content {
			switch part.Type {
			case "text":
				if part.Text == nil {
					return "", nil, fmt.Errorf("inputs[%d].content[%d].text is required", i, j)
				}
			case "image_url":
				if part.ImageURL == nil || strings.TrimSpace(part.ImageURL.URL) == "" {
					return "", nil, fmt.Errorf("inputs[%d].content[%d].image_url.url is required", i, j)
				}
			default:
				return "", nil, fmt.Errorf("inputs[%d].content[%d] has unsupported type %q", i, j, part.Type)
			}
		}
	}
	return wire.Model, &llm.CohereEmbeddingRequest{
		Body:       body,
		InputCount: count,
	}, nil
}

func (t *CohereEmbeddingInboundTransformer) TransformResponse(_ context.Context, response *llm.Response) (*httpclient.Response, error) {
	if response == nil || response.Embedding == nil || response.Embedding.Cohere == nil || response.Embedding.Cohere.VectorCount == 0 {
		return nil, fmt.Errorf("Cohere embedding response is empty")
	}
	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": {"application/json"}},
		Body:       append([]byte(nil), response.Embedding.Cohere.Body...),
	}, nil
}

func (t *CohereEmbeddingInboundTransformer) TransformError(ctx context.Context, err error) *httpclient.Error {
	if responseErr, ok := errors.AsType[*llm.ResponseError](err); ok && responseErr != nil {
		if httpErr, native := responseErr.Cause.(*httpclient.Error); !native || httpErr == nil {
			// 上层错误详情已确定对客户端的语义，不能继续取出更深层的原始正文。
			responseCopy := *responseErr
			responseCopy.Cause = nil
			return NewInboundTransformer().TransformError(ctx, &responseCopy)
		}
	}
	return NewInboundTransformer().TransformError(ctx, err)
}

func (t *CohereEmbeddingInboundTransformer) TransformStream(context.Context, streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
	return nil, fmt.Errorf("%w: Cohere embed does not support streaming", transformer.ErrInvalidRequest)
}

func (t *CohereEmbeddingInboundTransformer) AggregateStreamChunks(context.Context, []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("Cohere embed does not support streaming")
}

type CohereEmbeddingOutboundTransformer struct {
	config Config
}

func NewCohereEmbeddingOutboundTransformerWithConfig(config *Config) (*CohereEmbeddingOutboundTransformer, error) {
	if config == nil || config.APIKeyProvider == nil || strings.TrimSpace(config.BaseURL) == "" {
		return nil, fmt.Errorf("Cohere embed requires base URL and API key provider")
	}
	configCopy := *config
	if strings.HasSuffix(configCopy.BaseURL, "##") {
		configCopy.RawURL = true
		configCopy.BaseURL = strings.TrimSuffix(configCopy.BaseURL, "##")
	}
	configCopy.BaseURL = strings.TrimRight(configCopy.BaseURL, "/")
	if !configCopy.RawURL {
		version := "v2"
		if configCopy.EndpointPath != "" {
			version = ""
		}
		configCopy.BaseURL = transformer.NormalizeBaseURL(configCopy.BaseURL, version)
	}
	return &CohereEmbeddingOutboundTransformer{config: configCopy}, nil
}

func (t *CohereEmbeddingOutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatCohereEmbed
}

func (t *CohereEmbeddingOutboundTransformer) AllowPassThroughBody(context.Context, *llm.Request, *httpclient.Request) bool {
	return false
}

func (t *CohereEmbeddingOutboundTransformer) AllowPassThroughResponse(context.Context, *llm.Request, *httpclient.Request) bool {
	return false
}

func (t *CohereEmbeddingOutboundTransformer) TransformRequest(ctx context.Context, request *llm.Request) (*httpclient.Request, error) {
	if request == nil || request.Embedding == nil || request.Embedding.Cohere == nil {
		return nil, fmt.Errorf("%w: native Cohere embedding request is required", transformer.ErrInvalidRequest)
	}
	if request.Stream != nil && *request.Stream {
		return nil, fmt.Errorf("%w: Cohere embed does not support streaming", transformer.ErrInvalidRequest)
	}
	if strings.TrimSpace(request.Model) == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}
	if _, _, err := parseCohereEmbeddingRequest(request.Embedding.Cohere.Body); err != nil {
		return nil, fmt.Errorf("%w: %w", transformer.ErrInvalidRequest, err)
	}
	body, err := sjson.SetBytes(request.Embedding.Cohere.Body, "model", request.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to patch Cohere embedding model: %w", err)
	}
	url := t.config.BaseURL
	if !t.config.RawURL {
		path := t.config.EndpointPath
		if path == "" {
			path = "/embed"
		}
		url += path
	}
	return &httpclient.Request{
		Method: http.MethodPost,
		URL:    url,
		Headers: http.Header{
			"Content-Type": {"application/json"},
			"Accept":       {"application/json"},
		},
		Body: body,
		Auth: &httpclient.AuthConfig{
			Type:   httpclient.AuthTypeBearer,
			APIKey: t.config.APIKeyProvider.Get(ctx),
		},
		RequestType: llm.RequestTypeEmbedding.String(),
		APIFormat:   llm.APIFormatCohereEmbed.String(),
	}, nil
}

func (t *CohereEmbeddingOutboundTransformer) TransformResponse(ctx context.Context, response *httpclient.Response) (*llm.Response, error) {
	if response == nil {
		return nil, fmt.Errorf("%w: http response is nil", transformer.ErrInvalidResponse)
	}
	if response.StatusCode >= http.StatusMultipleChoices {
		return nil, t.TransformError(ctx, &httpclient.Error{
			StatusCode: response.StatusCode,
			Body:       response.Body,
			Headers:    response.Headers,
		})
	}
	if !json.Valid(response.Body) {
		return nil, fmt.Errorf("%w: invalid Cohere embedding JSON response", transformer.ErrInvalidResponse)
	}
	root := gjson.ParseBytes(response.Body)
	count, err := cohereVectorCount(root.Get("embeddings"))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", transformer.ErrInvalidResponse, err)
	}
	if response.Request != nil {
		_, request, err := parseCohereEmbeddingRequest(response.Request.Body)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid sent Cohere request: %w", transformer.ErrInvalidResponse, err)
		}
		if count != request.InputCount {
			return nil, fmt.Errorf("%w: received %d embeddings for %d inputs", transformer.ErrInvalidResponse, count, request.InputCount)
		}
	}
	result := &llm.Response{
		ID:          root.Get("id").String(),
		Model:       root.Get("model").String(),
		RequestType: llm.RequestTypeEmbedding,
		APIFormat:   llm.APIFormatCohereEmbed,
		Embedding: &llm.EmbeddingResponse{
			Cohere: &llm.CohereEmbeddingResponse{
				Body:        append(json.RawMessage(nil), response.Body...),
				VectorCount: count,
			},
		},
	}
	billedUnits := root.Get("usage.billed_units")
	if !billedUnits.Get("input_tokens").Exists() && !billedUnits.Get("image_tokens").Exists() {
		billedUnits = root.Get("meta.billed_units")
	}
	var total int64
	hasUsage := false
	for _, field := range []string{"input_tokens", "image_tokens"} {
		tokens := billedUnits.Get(field)
		if !tokens.Exists() {
			continue
		}
		var amount int64
		if err := json.Unmarshal([]byte(tokens.Raw), &amount); err != nil || tokens.Type != gjson.Number || amount < 0 {
			return nil, fmt.Errorf("%w: invalid billed %s", transformer.ErrInvalidResponse, field)
		}
		if amount > math.MaxInt64-total {
			return nil, fmt.Errorf("%w: billed token total overflows int64", transformer.ErrInvalidResponse)
		}
		total += amount
		hasUsage = true
	}
	if hasUsage {
		result.Usage = &llm.Usage{
			PromptTokens: total,
			TotalTokens:  total,
		}
	}
	return result, nil
}

func cohereVectorCount(embeddings gjson.Result) (int, error) {
	count := 0
	var invalid error
	embeddings.ForEach(func(key, batch gjson.Result) bool {
		switch key.String() {
		case "float", "int8", "uint8", "binary", "ubinary", "base64":
		default:
			return true
		}
		vectors := batch.Array()
		if !batch.IsArray() || len(vectors) == 0 || (count > 0 && count != len(vectors)) {
			invalid = fmt.Errorf("invalid or inconsistent embedding batch for %s", key.String())
			return false
		}
		for _, vector := range vectors {
			if key.String() == "base64" {
				if vector.Type != gjson.String || vector.String() == "" {
					invalid = fmt.Errorf("invalid base64 embedding")
					return false
				}
				decoded, err := base64.StdEncoding.DecodeString(vector.String())
				if err != nil {
					invalid = fmt.Errorf("invalid base64 embedding: %w", err)
					return false
				}
				if len(decoded) == 0 {
					invalid = fmt.Errorf("base64 embedding must contain non-empty data")
					return false
				}
				continue
			}
			values := vector.Array()
			if !vector.IsArray() || len(values) == 0 {
				invalid = fmt.Errorf("embedding vector must be a non-empty numeric array")
				return false
			}
			for _, value := range values {
				if value.Type != gjson.Number || math.IsNaN(value.Float()) || math.IsInf(value.Float(), 0) {
					invalid = fmt.Errorf("embedding vector contains an invalid number")
					return false
				}
			}
		}
		count = len(vectors)
		return true
	})
	if invalid != nil {
		return 0, invalid
	}
	if !embeddings.IsObject() || count == 0 {
		return 0, fmt.Errorf("Cohere response has no embedding vectors")
	}
	return count, nil
}

func (t *CohereEmbeddingOutboundTransformer) TransformError(_ context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	if rawErr == nil {
		return &llm.ResponseError{
			StatusCode: http.StatusInternalServerError,
			Detail: llm.ErrorDetail{
				Message: "Internal server error",
				Type:    "api_error",
			},
		}
	}
	message := gjson.GetBytes(rawErr.Body, "message").String()
	if message == "" {
		message = gjson.GetBytes(rawErr.Body, "error.message").String()
	}
	if message == "" {
		message = string(rawErr.Body)
	}
	return &llm.ResponseError{
		StatusCode: rawErr.StatusCode,
		Detail: llm.ErrorDetail{
			Message: message,
			Type:    "api_error",
		},
		Cause: rawErr,
	}
}

func (t *CohereEmbeddingOutboundTransformer) TransformStream(context.Context, *httpclient.Request, streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return nil, fmt.Errorf("%w: Cohere embed does not support streaming", transformer.ErrInvalidRequest)
}

func (t *CohereEmbeddingOutboundTransformer) AggregateStreamChunks(context.Context, *httpclient.Request, []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("Cohere embed does not support streaming")
}

var (
	_ transformer.Inbound                   = (*CohereEmbeddingInboundTransformer)(nil)
	_ transformer.Outbound                  = (*CohereEmbeddingOutboundTransformer)(nil)
	_ transformer.PassThroughBodyPolicy     = (*CohereEmbeddingOutboundTransformer)(nil)
	_ transformer.PassThroughResponsePolicy = (*CohereEmbeddingOutboundTransformer)(nil)
)
