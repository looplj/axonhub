package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

type Config struct {
	BaseURL        string              `json:"base_url,omitempty"`
	APIKeyProvider auth.APIKeyProvider `json:"-"`

	// EndpointPath is an optional custom path override for this endpoint.
	// When set, it replaces the default API path (e.g., "/api/chat").
	// Must start with "/". Skips default version normalization when set.
	EndpointPath string `json:"endpoint_path,omitempty"`
}

type OutboundTransformer struct {
	config *Config
}

func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	if config.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required for Ollama transformer")
	}

	return &OutboundTransformer{
		config: config,
	}, nil
}

func (t *OutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOllamaChat
}

type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   *bool         `json:"stream,omitempty"`
	Options  *Options      `json:"options,omitempty"`
}

type ChatMessage struct {
	Role     string `json:"role"`
	Content  string `json:"content"`
	Thinking string `json:"thinking,omitempty"`
}

type Options struct {
	Temperature *float64 `json:"temperature,omitempty"`
	TopP        *float64 `json:"top_p,omitempty"`
	TopK        *int     `json:"top_k,omitempty"`
	NumCtx      *int     `json:"num_ctx,omitempty"`
	NumPredict  *int     `json:"num_predict,omitempty"`
	Stop        []string `json:"stop,omitempty"`
}

type ChatResponse struct {
	Model      string       `json:"model"`
	CreatedAt  string       `json:"created_at"`
	Message    *ChatMessage `json:"message,omitempty"`
	Done       bool         `json:"done"`
	DoneReason string       `json:"done_reason,omitempty"`

	TotalDuration      int64 `json:"total_duration,omitempty"`
	LoadDuration       int64 `json:"load_duration,omitempty"`
	PromptEvalCount    int   `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64 `json:"prompt_eval_duration,omitempty"`
	EvalCount          int   `json:"eval_count,omitempty"`
	EvalDuration       int64 `json:"eval_duration,omitempty"`
}

func (t *OutboundTransformer) TransformRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("request is nil")
	}

	if llmReq.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	if len(llmReq.Messages) == 0 {
		return nil, fmt.Errorf("%w: messages are required", transformer.ErrInvalidRequest)
	}

	// Ollama defaults to streaming (stream: true); explicitly set to false when not requested
	stream := llmReq.Stream
	if stream == nil {
		stream = new(bool)
		*stream = false
	}

	ollamaReq := &ChatRequest{
		Model:    llmReq.Model,
		Messages: make([]ChatMessage, 0, len(llmReq.Messages)),
		Stream:   stream,
	}

	for _, msg := range llmReq.Messages {
		content := getContentString(msg.Content)
		ollamaReq.Messages = append(ollamaReq.Messages, ChatMessage{
			Role:    msg.Role,
			Content: content,
		})
	}

	ollamaReq.Options = t.buildOptions(llmReq)

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ollama request: %w", err)
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	var authConfig *httpclient.AuthConfig
	if t.config.APIKeyProvider != nil {
		apiKey := t.config.APIKeyProvider.Get(ctx)
		if apiKey != "" {
			authConfig = &httpclient.AuthConfig{
				Type:   "bearer",
				APIKey: apiKey,
			}
		}
	}

	base := strings.TrimSuffix(t.config.BaseURL, "/")
	defaultPath := "/api/chat"

	path := defaultPath
	if t.config.EndpointPath != "" {
		path = t.config.EndpointPath
	}

	url := base + path

	return &httpclient.Request{
		Method:    http.MethodPost,
		URL:       url,
		Headers:   headers,
		Body:      body,
		Auth:      authConfig,
		APIFormat: string(llm.APIFormatOllamaChat),
	}, nil
}

func getContentString(content llm.MessageContent) string {
	if content.Content != nil {
		return *content.Content
	}
	if len(content.MultipleContent) > 0 {
		var parts []string
		for _, part := range content.MultipleContent {
			if part.Type == "text" && part.Text != nil {
				parts = append(parts, *part.Text)
			}
		}
		return strings.Join(parts, "")
	}
	return ""
}

func (t *OutboundTransformer) buildOptions(llmReq *llm.Request) *Options {
	var opts *Options

	if llmReq.Temperature != nil {
		if opts == nil {
			opts = &Options{}
		}
		opts.Temperature = llmReq.Temperature
	}

	if llmReq.TopP != nil {
		if opts == nil {
			opts = &Options{}
		}
		opts.TopP = llmReq.TopP
	}

	if llmReq.MaxTokens != nil {
		if opts == nil {
			opts = &Options{}
		}
		numPredict := int(*llmReq.MaxTokens)
		opts.NumPredict = &numPredict
	}

	if llmReq.Stop != nil {
		var stops []string
		if llmReq.Stop.Stop != nil {
			stops = []string{*llmReq.Stop.Stop}
		} else if len(llmReq.Stop.MultipleStop) > 0 {
			stops = llmReq.Stop.MultipleStop
		}
		if len(stops) > 0 {
			if opts == nil {
				opts = &Options{}
			}
			opts.Stop = stops
		}
	}

	return opts
}

func (t *OutboundTransformer) TransformResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("response is nil")
	}

	var ollamaResp ChatResponse
	if err := json.Unmarshal(httpResp.Body, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ollama response: %w", err)
	}

	content := ""
	var reasoningContent *string
	if ollamaResp.Message != nil {
		content = ollamaResp.Message.Content
		if ollamaResp.Message.Thinking != "" {
			reasoningContent = &ollamaResp.Message.Thinking
		}
	}

	finishReason := "stop"
	if ollamaResp.DoneReason != "" {
		finishReason = ollamaResp.DoneReason
	}

	return &llm.Response{
		ID:      fmt.Sprintf("ollama-%s", ollamaResp.Model),
		Object:  "chat.completion",
		Created: 0,
		Model:   ollamaResp.Model,
		Choices: []llm.Choice{
			{
				Index: 0,
				Message: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: &content,
					},
					ReasoningContent: reasoningContent,
				},
				FinishReason: &finishReason,
			},
		},
		Usage: &llm.Usage{
			PromptTokens:     int64(ollamaResp.PromptEvalCount),
			CompletionTokens: int64(ollamaResp.EvalCount),
			TotalTokens:      int64(ollamaResp.PromptEvalCount + ollamaResp.EvalCount),
		},
	}, nil
}

func (t *OutboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return streams.MapErr(stream, func(event *httpclient.StreamEvent) (*llm.Response, error) {
		return t.TransformStreamChunk(ctx, event)
	}), nil
}

func (t *OutboundTransformer) TransformStreamChunk(ctx context.Context, event *httpclient.StreamEvent) (*llm.Response, error) {
	if event == nil || len(event.Data) == 0 {
		return nil, nil
	}

	var ollamaResp ChatResponse
	if err := json.Unmarshal(event.Data, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ollama stream chunk: %w", err)
	}

	// Extract content and thinking from the message
	content := ""
	var reasoningContent *string
	if ollamaResp.Message != nil {
		content = ollamaResp.Message.Content
		if ollamaResp.Message.Thinking != "" {
			reasoningContent = &ollamaResp.Message.Thinking
		}
	}

	// For streaming, we use Delta instead of Message
	return &llm.Response{
		ID:     fmt.Sprintf("ollama-%s", ollamaResp.Model),
		Object: "chat.completion.chunk",
		Model:  ollamaResp.Model,
		Choices: []llm.Choice{
			{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: &content,
					},
					ReasoningContent: reasoningContent,
				},
				FinishReason: func() *string {
					if ollamaResp.Done && ollamaResp.DoneReason != "" {
						return &ollamaResp.DoneReason
					}
					if ollamaResp.Done {
						reason := "stop"
						return &reason
					}
					return nil
				}(),
			},
		},
	}, nil
}

func (t *OutboundTransformer) TransformError(ctx context.Context, err *httpclient.Error) *llm.ResponseError {
	if err == nil {
		return nil
	}

	return &llm.ResponseError{
		StatusCode: err.StatusCode,
		Detail: llm.ErrorDetail{
			Message: string(err.Body),
			Type:    "ollama_error",
		},
	}
}

func (t *OutboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	if len(chunks) == 0 {
		return nil, llm.ResponseMeta{}, fmt.Errorf("no chunks to aggregate")
	}

	var fullContent strings.Builder
	var fullThinking strings.Builder
	var model string
	var promptEvalCount, evalCount int
	var finishReason string

	for _, chunk := range chunks {
		var ollamaResp ChatResponse
		if err := json.Unmarshal(chunk.Data, &ollamaResp); err != nil {
			return nil, llm.ResponseMeta{}, fmt.Errorf("failed to unmarshal ollama chunk: %w", err)
		}

		if ollamaResp.Model != "" {
			model = ollamaResp.Model
		}
		if ollamaResp.Message != nil {
			fullContent.WriteString(ollamaResp.Message.Content)
			fullThinking.WriteString(ollamaResp.Message.Thinking)
		}
		if ollamaResp.PromptEvalCount > 0 {
			promptEvalCount = ollamaResp.PromptEvalCount
		}
		if ollamaResp.EvalCount > 0 {
			evalCount = ollamaResp.EvalCount
		}
		if ollamaResp.Done && ollamaResp.DoneReason != "" {
			finishReason = ollamaResp.DoneReason
		}
	}

	if finishReason == "" {
		finishReason = "stop"
	}

	contentStr := fullContent.String()
	var reasoningContent *string
	if thinkingStr := fullThinking.String(); thinkingStr != "" {
		reasoningContent = &thinkingStr
	}

	aggregatedResp := &llm.Response{
		ID:      fmt.Sprintf("ollama-%s", model),
		Object:  "chat.completion",
		Created: 0,
		Model:   model,
		Choices: []llm.Choice{
			{
				Index: 0,
				Message: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: &contentStr,
					},
					ReasoningContent: reasoningContent,
				},
				FinishReason: &finishReason,
			},
		},
		Usage: &llm.Usage{
			PromptTokens:     int64(promptEvalCount),
			CompletionTokens: int64(evalCount),
			TotalTokens:      int64(promptEvalCount + evalCount),
		},
	}

	body, err := json.Marshal(aggregatedResp)
	if err != nil {
		return nil, llm.ResponseMeta{}, fmt.Errorf("failed to marshal aggregated response: %w", err)
	}

	return body, llm.ResponseMeta{
		ID: fmt.Sprintf("ollama-%s", model),
		Usage: &llm.Usage{
			PromptTokens:     int64(promptEvalCount),
			CompletionTokens: int64(evalCount),
			TotalTokens:      int64(promptEvalCount + evalCount),
		},
	}, nil
}
