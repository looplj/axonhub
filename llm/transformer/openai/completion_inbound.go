package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xjson"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

type CompletionInboundTransformer struct{}

func NewCompletionInboundTransformer() *CompletionInboundTransformer {
	return &CompletionInboundTransformer{}
}

func (t *CompletionInboundTransformer) TransformRequest(
	ctx context.Context,
	httpReq *httpclient.Request,
) (*llm.Request, error) {
	if httpReq == nil {
		return nil, fmt.Errorf("%w: http request is nil", transformer.ErrInvalidRequest)
	}

	if len(httpReq.Body) == 0 {
		return nil, fmt.Errorf("%w: request body is empty", transformer.ErrInvalidRequest)
	}

	contentType := httpReq.Headers.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}

	if !strings.Contains(strings.ToLower(contentType), "application/json") {
		return nil, fmt.Errorf("%w: unsupported content type: %s", transformer.ErrInvalidRequest, contentType)
	}

	var compReq CompletionRequest

	err := json.Unmarshal(httpReq.Body, &compReq)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to decode completion request: %w", transformer.ErrInvalidRequest, err)
	}

	if compReq.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	if compReq.Prompt == "" {
		return nil, fmt.Errorf("%w: prompt is required", transformer.ErrInvalidRequest)
	}

	llmReq := &llm.Request{
		Model:         compReq.Model,
		Messages:      []llm.Message{},
		RawRequest:    httpReq,
		RequestType:   llm.RequestTypeCompletion,
		APIFormat:     llm.APIFormatOpenAICompletion,
		Stream:        compReq.Stream,
		StreamOptions: compReq.StreamOptions,
		Completion: &llm.CompletionRequest{
			Prompt:           compReq.Prompt,
			Suffix:           compReq.Suffix,
			MaxTokens:        compReq.MaxTokens,
			Temperature:      compReq.Temperature,
			TopP:             compReq.TopP,
			N:                compReq.N,
			Logprobs:         compReq.Logprobs,
			Echo:             compReq.Echo,
			PresencePenalty:  compReq.PresencePenalty,
			FrequencyPenalty: compReq.FrequencyPenalty,
			BestOf:           compReq.BestOf,
			LogitBias:        compReq.LogitBias,
			Seed:             compReq.Seed,
			User:             compReq.User,
		},
	}

	if compReq.Stop != nil {
		llmReq.Completion.Stop = &llm.Stop{
			Stop:         compReq.Stop.Stop,
			MultipleStop: compReq.Stop.MultipleStop,
		}
	}

	if compReq.Temperature != nil {
		llmReq.Temperature = compReq.Temperature
	}

	if compReq.TopP != nil {
		llmReq.TopP = compReq.TopP
	}

	if compReq.FrequencyPenalty != nil {
		llmReq.FrequencyPenalty = compReq.FrequencyPenalty
	}

	if compReq.PresencePenalty != nil {
		llmReq.PresencePenalty = compReq.PresencePenalty
	}

	if compReq.MaxTokens != nil {
		llmReq.MaxTokens = compReq.MaxTokens
	}

	if compReq.User != "" {
		llmReq.User = &compReq.User
	}

	if compReq.Seed != nil {
		llmReq.Seed = compReq.Seed
	}

	return llmReq, nil
}

func (t *CompletionInboundTransformer) TransformResponse(
	ctx context.Context,
	llmResp *llm.Response,
) (*httpclient.Response, error) {
	if llmResp == nil {
		return nil, fmt.Errorf("completion response is nil")
	}

	var body []byte
	statusCode := http.StatusOK

	if llmResp.Completion != nil {
		resp := CompletionResponse{
			ID:      llmResp.ID,
			Object:  llmResp.Object,
			Created: llmResp.Created,
			Model:   llmResp.Model,
			Choices: make([]CompletionChoice, len(llmResp.Completion.Choices)),
		}

		for i, c := range llmResp.Completion.Choices {
			resp.Choices[i] = CompletionChoice{
				Text:         c.Text,
				Index:        c.Index,
				Logprobs:     c.Logprobs,
				FinishReason: c.FinishReason,
			}
		}

		if llmResp.Usage != nil {
			resp.Usage = CompletionUsage{
				PromptTokens:     llmResp.Usage.PromptTokens,
				CompletionTokens: llmResp.Usage.CompletionTokens,
				TotalTokens:      llmResp.Usage.TotalTokens,
			}
		}

		var err error
		body, err = json.Marshal(resp)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal completion response: %w", err)
		}
	} else if llmResp.Error != nil {
		body = xjson.MustMarshal(&OpenAIError{Detail: llmResp.Error.Detail})
		statusCode = llmResp.Error.StatusCode
	} else {
		return nil, fmt.Errorf("completion response missing completion data")
	}

	return &httpclient.Response{
		StatusCode: statusCode,
		Body:       body,
		Headers: http.Header{
			"Content-Type":  []string{"application/json"},
			"Cache-Control": []string{"no-cache"},
		},
	}, nil
}

func (t *CompletionInboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*llm.Response],
) (streams.Stream[*httpclient.StreamEvent], error) {
	return streams.NoNil(streams.MapErr(stream, func(chunk *llm.Response) (*httpclient.StreamEvent, error) {
		return t.transformStreamChunk(chunk)
	})), nil
}

func (t *CompletionInboundTransformer) transformStreamChunk(llmResp *llm.Response) (*httpclient.StreamEvent, error) {
	if llmResp == nil {
		return nil, fmt.Errorf("completion response is nil")
	}

	if llmResp.Object == "[DONE]" {
		return &httpclient.StreamEvent{
			Data: []byte("[DONE]"),
		}, nil
	}

	if llmResp.Completion == nil {
		return nil, nil
	}

	resp := CompletionResponse{
		ID:      llmResp.ID,
		Object:  llmResp.Object,
		Created: llmResp.Created,
		Model:   llmResp.Model,
		Choices: make([]CompletionChoice, len(llmResp.Completion.Choices)),
	}

	for i, c := range llmResp.Completion.Choices {
		resp.Choices[i] = CompletionChoice{
			Text:         c.Text,
			Index:        c.Index,
			Logprobs:     c.Logprobs,
			FinishReason: c.FinishReason,
		}
	}

	if llmResp.Usage != nil {
		resp.Usage = CompletionUsage{
			PromptTokens:     llmResp.Usage.PromptTokens,
			CompletionTokens: llmResp.Usage.CompletionTokens,
			TotalTokens:      llmResp.Usage.TotalTokens,
		}
	}

	eventData, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal completion stream chunk: %w", err)
	}

	return &httpclient.StreamEvent{
		Type: "",
		Data: eventData,
	}, nil
}

func (t *CompletionInboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return AggregateCompletionStreamChunks(ctx, chunks)
}

func (t *CompletionInboundTransformer) TransformError(ctx context.Context, rawErr error) *httpclient.Error {
	if rawErr == nil {
		return &httpclient.Error{
			StatusCode: http.StatusInternalServerError,
			Status:     http.StatusText(http.StatusInternalServerError),
			Body:       xjson.MustMarshal(&OpenAIError{Detail: llm.ErrorDetail{Message: "An unexpected error occurred", Type: "unexpected_error"}}),
		}
	}

	if errors.Is(rawErr, transformer.ErrInvalidModel) {
		return &httpclient.Error{
			StatusCode: http.StatusUnprocessableEntity,
			Status:     http.StatusText(http.StatusUnprocessableEntity),
			Body:       xjson.MustMarshal(&OpenAIError{Detail: llm.ErrorDetail{Message: rawErr.Error(), Type: "invalid_model_error"}}),
		}
	}

	if httpErr, ok := errors.AsType[*httpclient.Error](rawErr); ok {
		return httpErr
	}

	if errors.Is(rawErr, transformer.ErrInvalidRequest) {
		return &httpclient.Error{
			StatusCode: http.StatusBadRequest,
			Status:     http.StatusText(http.StatusBadRequest),
			Body:       xjson.MustMarshal(&OpenAIError{Detail: llm.ErrorDetail{Message: rawErr.Error(), Type: "invalid_request_error"}}),
		}
	}

	if llmErr, ok := errors.AsType[*llm.ResponseError](rawErr); ok {
		return &httpclient.Error{
			StatusCode: llmErr.StatusCode,
			Status:     http.StatusText(llmErr.StatusCode),
			Body:       xjson.MustMarshal(&OpenAIError{Detail: llmErr.Detail}),
		}
	}

	return &httpclient.Error{
		StatusCode: http.StatusInternalServerError,
		Status:     http.StatusText(http.StatusInternalServerError),
		Body:       xjson.MustMarshal(&OpenAIError{Detail: llm.ErrorDetail{Message: rawErr.Error(), Type: "internal_server_error"}}),
	}
}
