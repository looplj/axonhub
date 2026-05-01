package openai

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
	"github.com/looplj/axonhub/llm/transformer/shared"
)

type CompletionOutboundTransformer struct {
	config *Config
}

func NewCompletionOutboundTransformer(config *Config) (transformer.Outbound, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if config.APIKeyProvider == nil {
		return nil, fmt.Errorf("API key provider is required")
	}

	if config.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	if strings.HasSuffix(config.BaseURL, "##") {
		config.RawURL = true
		config.BaseURL = strings.TrimSuffix(config.BaseURL, "##")
	} else if !config.RawURL {
		if config.EndpointPath != "" {
			config.BaseURL = transformer.NormalizeBaseURL(config.BaseURL, "")
		} else {
			config.BaseURL = transformer.NormalizeBaseURL(config.BaseURL, "v1")
		}
	}

	return &CompletionOutboundTransformer{
		config: config,
	}, nil
}

func (t *CompletionOutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAICompletion
}

func (t *CompletionOutboundTransformer) TransformRequest(
	ctx context.Context,
	llmReq *llm.Request,
) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("llm request is nil")
	}

	if llmReq.Completion == nil {
		return nil, fmt.Errorf("completion request is nil in llm.Request")
	}

	if llmReq.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	compReq := CompletionRequest{
		Model:            llmReq.Model,
		Prompt:           llmReq.Completion.Prompt,
		Suffix:           llmReq.Completion.Suffix,
		MaxTokens:        llmReq.Completion.MaxTokens,
		Temperature:      llmReq.Completion.Temperature,
		TopP:             llmReq.Completion.TopP,
		N:                llmReq.Completion.N,
		Stream:           llmReq.Stream,
		StreamOptions:    llmReq.StreamOptions,
		Logprobs:         llmReq.Completion.Logprobs,
		Echo:             llmReq.Completion.Echo,
		PresencePenalty:  llmReq.Completion.PresencePenalty,
		FrequencyPenalty: llmReq.Completion.FrequencyPenalty,
		BestOf:           llmReq.Completion.BestOf,
		LogitBias:        llmReq.Completion.LogitBias,
		Seed:             llmReq.Completion.Seed,
		User:             llmReq.Completion.User,
	}

	if llmReq.Completion.Stop != nil {
		compReq.Stop = &Stop{
			Stop:         llmReq.Completion.Stop.Stop,
			MultipleStop: llmReq.Completion.Stop.MultipleStop,
		}
	}

	body, err := json.Marshal(compReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal completion request: %w", err)
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	apiKey := t.config.APIKeyProvider.Get(ctx)

	url := t.buildURL()

	scope := shared.TransportScope{
		BaseURL:         t.config.BaseURL,
		AccountIdentity: t.config.AccountIdentity,
	}

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth: &httpclient.AuthConfig{
			Type:   "bearer",
			APIKey: apiKey,
		},
		RequestType: string(llm.RequestTypeCompletion),
		APIFormat:   string(llm.APIFormatOpenAICompletion),
		Metadata:    scope.Metadata(),
	}, nil
}

func (t *CompletionOutboundTransformer) buildURL() string {
	if t.config.RawURL {
		return t.config.BaseURL
	}

	if t.config.EndpointPath != "" {
		return t.config.BaseURL + t.config.EndpointPath
	}

	return t.config.BaseURL + "/completions"
}

func (t *CompletionOutboundTransformer) TransformResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	if httpResp.StatusCode >= 400 {
		return nil, t.TransformError(ctx, &httpclient.Error{
			StatusCode: httpResp.StatusCode,
			Body:       httpResp.Body,
		})
	}

	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	var compResp CompletionResponse
	if err := json.Unmarshal(httpResp.Body, &compResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal completion response: %w", err)
	}

	return completionResponseToLLM(&compResp), nil
}

func completionResponseToLLM(compResp *CompletionResponse) *llm.Response {
	llmChoices := make([]llm.CompletionChoice, len(compResp.Choices))
	for i, c := range compResp.Choices {
		llmChoices[i] = llm.CompletionChoice{
			Text:         c.Text,
			Index:        c.Index,
			Logprobs:     c.Logprobs,
			FinishReason: c.FinishReason,
		}
	}

	llmResp := &llm.Response{
		ID:          compResp.ID,
		Object:      compResp.Object,
		Created:     compResp.Created,
		Model:       compResp.Model,
		RequestType: llm.RequestTypeCompletion,
		APIFormat:   llm.APIFormatOpenAICompletion,
		Completion: &llm.CompletionResponse{
			Choices: llmChoices,
		},
	}

	if compResp.Usage.PromptTokens > 0 || compResp.Usage.TotalTokens > 0 {
		llmResp.Usage = &llm.Usage{
			PromptTokens:     compResp.Usage.PromptTokens,
			CompletionTokens: compResp.Usage.CompletionTokens,
			TotalTokens:      compResp.Usage.TotalTokens,
		}
	}

	return llmResp
}

func (t *CompletionOutboundTransformer) TransformStream(
	ctx context.Context,
	req *httpclient.Request,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	return streams.MapErr(stream, func(event *httpclient.StreamEvent) (*llm.Response, error) {
		return t.TransformStreamChunk(ctx, event)
	}), nil
}

func (t *CompletionOutboundTransformer) TransformStreamChunk(
	ctx context.Context,
	event *httpclient.StreamEvent,
) (*llm.Response, error) {
	if bytes.HasPrefix(event.Data, []byte("[DONE]")) {
		return llm.DoneResponse, nil
	}

	if len(event.Data) == 0 {
		return nil, nil
	}

	httpResp := &httpclient.Response{
		Body: event.Data,
	}

	return t.TransformResponse(ctx, httpResp)
}

func (t *CompletionOutboundTransformer) AggregateStreamChunks(
	ctx context.Context, _ *httpclient.Request,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return AggregateCompletionStreamChunks(ctx, chunks)
}

func (t *CompletionOutboundTransformer) TransformError(
	ctx context.Context,
	rawErr *httpclient.Error,
) *llm.ResponseError {
	return TransformOpenAIError(ctx, rawErr)
}

var _ transformer.Outbound = (*CompletionOutboundTransformer)(nil)
