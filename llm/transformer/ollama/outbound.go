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
	Model    string                 `json:"model"`
	Messages []ChatMessage          `json:"messages"`
	Stream   *bool                  `json:"stream,omitempty"`
	Options  *Options               `json:"options,omitempty"`
	Tools    []ollamaToolDefinition `json:"tools,omitempty"`
}

type ChatMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	Images    []string         `json:"images,omitempty"`
	Thinking  string           `json:"thinking,omitempty"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaToolDefinition struct {
	Type     string             `json:"type"`
	Function ollamaToolFunction `json:"function"`
}

type ollamaToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

type ollamaToolCall struct {
	Function ollamaToolCallFunction `json:"function"`
}

type ollamaToolCallFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Arguments   map[string]any `json:"arguments"`
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
	PromptEvalCount    int64 `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64 `json:"prompt_eval_duration,omitempty"`
	EvalCount          int64 `json:"eval_count,omitempty"`
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
		chatMsg := ChatMessage{
			Role:    msg.Role,
			Content: getContentString(msg.Content),
			Images:  getImages(msg.Content),
		}

		if len(msg.ToolCalls) > 0 {
			chatMsg.ToolCalls = make([]ollamaToolCall, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				rawArgs := tc.Function.Arguments
				if rawArgs == "" {
					rawArgs = "{}"
				}

				var args map[string]any
				if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
					return nil, fmt.Errorf("failed to parse tool call arguments for %s: %w", tc.Function.Name, err)
				}

				chatMsg.ToolCalls = append(chatMsg.ToolCalls, ollamaToolCall{
					Function: ollamaToolCallFunction{
						Name:      tc.Function.Name,
						Arguments: args,
					},
				})
			}
		}

		ollamaReq.Messages = append(ollamaReq.Messages, chatMsg)
	}

	for _, tool := range llmReq.Tools {
		if tool.Type != llm.ToolTypeFunction {
			continue
		}

		parameters := tool.Function.Parameters
		if parameters == nil {
			parameters = json.RawMessage("{}")
		}

		ollamaReq.Tools = append(ollamaReq.Tools, ollamaToolDefinition{
			Type: "function",
			Function: ollamaToolFunction{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  parameters,
			},
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

func getImages(content llm.MessageContent) []string {
	if len(content.MultipleContent) == 0 {
		return nil
	}

	images := make([]string, 0)

	for _, part := range content.MultipleContent {
		if part.Type != "image_url" || part.ImageURL == nil {
			continue
		}

		image := strings.TrimSpace(part.ImageURL.URL)
		if image == "" {
			continue
		}

		if comma := strings.Index(image, ","); strings.HasPrefix(image, "data:") && comma >= 0 {
			image = image[comma+1:]
		}

		if image != "" {
			images = append(images, image)
		}
	}

	return images
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
	var toolCalls []llm.ToolCall

	if ollamaResp.Message != nil {
		content = ollamaResp.Message.Content
		if ollamaResp.Message.Thinking != "" {
			reasoningContent = &ollamaResp.Message.Thinking
		}

		if len(ollamaResp.Message.ToolCalls) > 0 {
			toolCalls = make([]llm.ToolCall, 0, len(ollamaResp.Message.ToolCalls))
			for i, tc := range ollamaResp.Message.ToolCalls {
				args, _ := json.Marshal(tc.Function.Arguments)
				if string(args) == "null" {
					args = []byte("{}")
				}

				toolCalls = append(toolCalls, llm.ToolCall{
					ID:   fmt.Sprintf("ollama-%s-%d", ollamaResp.Model, i),
					Type: "function",
					Function: llm.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: string(args),
					},
					Index: i,
				})
			}
		}
	}

	finishReason := "stop"
	if ollamaResp.DoneReason != "" {
		finishReason = ollamaResp.DoneReason
	}

	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
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
					ToolCalls:        toolCalls,
				},
				FinishReason: &finishReason,
			},
		},
		Usage: &llm.Usage{
			PromptTokens:     ollamaResp.PromptEvalCount,
			CompletionTokens: ollamaResp.EvalCount,
			TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
		},
	}, nil
}

func (t *OutboundTransformer) TransformStream(ctx context.Context, req *httpclient.Request, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
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
	var toolCalls []llm.ToolCall

	if ollamaResp.Message != nil {
		content = ollamaResp.Message.Content
		if ollamaResp.Message.Thinking != "" {
			reasoningContent = &ollamaResp.Message.Thinking
		}

		if len(ollamaResp.Message.ToolCalls) > 0 {
			toolCalls = make([]llm.ToolCall, 0, len(ollamaResp.Message.ToolCalls))
			for i, tc := range ollamaResp.Message.ToolCalls {
				args, _ := json.Marshal(tc.Function.Arguments)
				if string(args) == "null" {
					args = []byte("{}")
				}

				toolCalls = append(toolCalls, llm.ToolCall{
					ID:   fmt.Sprintf("ollama-%s-%d", ollamaResp.Model, i),
					Type: "function",
					Function: llm.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: string(args),
					},
					Index: i,
				})
			}
		}
	}

	// For streaming, we use Delta instead of Message
	resp := &llm.Response{
		ID:     fmt.Sprintf("ollama-%s", ollamaResp.Model),
		Object: "chat.completion.chunk",
		Model:  ollamaResp.Model,
		Choices: []llm.Choice{
			{
				Index: 0,
				Delta: &llm.Message{
					Role:             "assistant",
					Content:          llm.MessageContent{Content: &content},
					ReasoningContent: reasoningContent,
					ToolCalls:        toolCalls,
				},
				FinishReason: func() *string {
					if len(toolCalls) > 0 {
						reason := "tool_calls"
						return &reason
					}

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
	}

	if ollamaResp.Done && (ollamaResp.PromptEvalCount > 0 || ollamaResp.EvalCount > 0) {
		resp.Usage = &llm.Usage{
			PromptTokens:     ollamaResp.PromptEvalCount,
			CompletionTokens: ollamaResp.EvalCount,
			TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
		}
	}

	return resp, nil
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

func (t *OutboundTransformer) AggregateStreamChunks(ctx context.Context, _ *httpclient.Request, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	if len(chunks) == 0 {
		return nil, llm.ResponseMeta{}, fmt.Errorf("no chunks to aggregate")
	}

	var fullContent strings.Builder
	var fullThinking strings.Builder
	var model string
	var promptEvalCount, evalCount int64
	var finishReason string
	var lastToolCalls []ollamaToolCall

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

			if len(ollamaResp.Message.ToolCalls) > 0 {
				lastToolCalls = ollamaResp.Message.ToolCalls
			}
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

	var toolCalls []llm.ToolCall
	if len(lastToolCalls) > 0 {
		toolCalls = make([]llm.ToolCall, 0, len(lastToolCalls))
		for i, tc := range lastToolCalls {
			args, _ := json.Marshal(tc.Function.Arguments)
			if string(args) == "null" {
				args = []byte("{}")
			}

			toolCalls = append(toolCalls, llm.ToolCall{
				ID:   fmt.Sprintf("ollama-%s-%d", model, i),
				Type: "function",
				Function: llm.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: string(args),
				},
				Index: i,
			})
		}
	}

	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
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
					ToolCalls:        toolCalls,
				},
				FinishReason: &finishReason,
			},
		},
		Usage: &llm.Usage{
			PromptTokens:     promptEvalCount,
			CompletionTokens: evalCount,
			TotalTokens:      promptEvalCount + evalCount,
		},
	}

	body, err := json.Marshal(aggregatedResp)
	if err != nil {
		return nil, llm.ResponseMeta{}, fmt.Errorf("failed to marshal aggregated response: %w", err)
	}

	return body, llm.ResponseMeta{
		ID: fmt.Sprintf("ollama-%s", model),
		Usage: &llm.Usage{
			PromptTokens:     promptEvalCount,
			CompletionTokens: evalCount,
			TotalTokens:      promptEvalCount + evalCount,
		},
	}, nil
}
