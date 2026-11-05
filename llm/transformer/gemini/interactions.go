package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

var (
	ErrInteractionsUnsupported = errors.New("gemini interactions parameter is unsupported")
	ErrInteractionsIncomplete  = errors.New("gemini interactions stream ended before done")
)

type interactionsRequest struct {
	Model             string                  `json:"model"`
	SystemInstruction string                  `json:"system_instruction,omitempty"`
	Input             json.RawMessage         `json:"input"`
	Tools             []interactionsTool      `json:"tools,omitempty"`
	GenerationConfig  *interactionsGeneration `json:"generation_config,omitempty"`
	ResponseFormat    json.RawMessage         `json:"response_format,omitempty"`
	Store             *bool                   `json:"store,omitempty"`
	Stream            *bool                   `json:"stream,omitempty"`
	PreviousID        string                  `json:"previous_interaction_id,omitempty"`
	Continuation      string                  `json:"continuation_token,omitempty"`
	Background        *bool                   `json:"background,omitempty"`
	Environment       json.RawMessage         `json:"environment,omitempty"`
	EnvironmentID     string                  `json:"environment_id,omitempty"`
	Env               json.RawMessage         `json:"env,omitempty"`
	Webhook           json.RawMessage         `json:"webhook,omitempty"`
	WebhookConfig     json.RawMessage         `json:"webhook_config,omitempty"`
	Agent             json.RawMessage         `json:"agent,omitempty"`
	AgentConfig       json.RawMessage         `json:"agent_config,omitempty"`
}

type interactionsTool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type interactionsGeneration struct {
	Seed            *int64          `json:"seed,omitempty"`
	Temperature     *float64        `json:"temperature,omitempty"`
	ToolChoice      json.RawMessage `json:"tool_choice,omitempty"`
	AllowedTools    json.RawMessage `json:"allowed_tools,omitempty"`
	ThinkingLevel   string          `json:"thinking_level,omitempty"`
	ThinkingSummary string          `json:"thinking_summaries,omitempty"`
	MaxOutputTokens *int64          `json:"max_output_tokens,omitempty"`
	StopSequences   []string        `json:"stop_sequences,omitempty"`
}

type interactionsContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type interactionsStep struct {
	Type      string                `json:"type"`
	Content   []interactionsContent `json:"content,omitempty"`
	ID        string                `json:"id,omitempty"`
	CallID    string                `json:"call_id,omitempty"`
	Name      string                `json:"name,omitempty"`
	Arguments map[string]any        `json:"arguments,omitempty"`
	Result    any                   `json:"result,omitempty"`
}

type interactionsUsage struct {
	InputTokens       int64 `json:"input_tokens"`
	OutputTokens      int64 `json:"output_tokens"`
	TotalTokens       int64 `json:"total_tokens"`
	TotalInputTokens  int64 `json:"total_input_tokens"`
	TotalOutputTokens int64 `json:"total_output_tokens"`
}

type interactionsResponse struct {
	Object  string             `json:"object,omitempty"`
	ID      string             `json:"id"`
	Model   string             `json:"model,omitempty"`
	Status  string             `json:"status"`
	Created string             `json:"created,omitempty"`
	Updated string             `json:"updated,omitempty"`
	Steps   []interactionsStep `json:"steps,omitempty"`
	Usage   *interactionsUsage `json:"usage,omitempty"`
	Error   *interactionsError `json:"error,omitempty"`
	EventID string             `json:"event_id,omitempty"`
}

type interactionsError struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
	Type    string `json:"type,omitempty"`
}

type interactionsEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type interactionsEventData struct {
	Interaction   *interactionsResponse `json:"interaction,omitempty"`
	InteractionID string                `json:"interaction_id,omitempty"`
	Status        string                `json:"status,omitempty"`
	Step          *interactionsStep     `json:"step,omitempty"`
	Delta         *interactionsDelta    `json:"delta,omitempty"`
	Usage         *interactionsUsage    `json:"usage,omitempty"`
	Error         *interactionsError    `json:"error,omitempty"`
	EventID       string                `json:"event_id,omitempty"`
}

type interactionsDelta struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// InteractionsInboundTransformer converts the stateless Interactions API to the
// unified client-facing request/response model.
type InteractionsInboundTransformer struct{}

func NewInteractionsInboundTransformer() *InteractionsInboundTransformer {
	return &InteractionsInboundTransformer{}
}

func (t *InteractionsInboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatGeminiInteractions
}

func (t *InteractionsInboundTransformer) TransformRequest(_ context.Context, request *httpclient.Request) (*llm.Request, error) {
	if request == nil || len(request.Body) == 0 {
		return nil, fmt.Errorf("%w: request body is empty", transformer.ErrInvalidRequest)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(request.Body, &raw); err != nil {
		return nil, fmt.Errorf("%w: invalid interactions request: %w", transformer.ErrInvalidRequest, err)
	}
	var req interactionsRequest
	if err := json.Unmarshal(request.Body, &req); err != nil {
		return nil, fmt.Errorf("%w: invalid interactions request: %w", transformer.ErrInvalidRequest, err)
	}
	if req.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}
	if len(req.Input) == 0 || string(req.Input) == "null" {
		return nil, fmt.Errorf("%w: input is required", transformer.ErrInvalidRequest)
	}
	if req.Stream == nil {
		req.Stream = interactionStreamFlag(request)
	}
	if req.Store != nil && *req.Store {
		return nil, unsupportedInteractionField("store=true")
	}
	if req.PreviousID != "" {
		return nil, unsupportedInteractionField("previous_interaction_id")
	}
	if req.Continuation != "" {
		return nil, unsupportedInteractionField("continuation_token")
	}
	if req.Background != nil && *req.Background {
		return nil, unsupportedInteractionField("background=true")
	}
	for _, field := range []struct {
		name string
		data json.RawMessage
	}{
		{"environment", req.Environment},
		{"environment_id", rawString(req.EnvironmentID)},
		{"env", req.Env},
		{"webhook", req.Webhook},
		{"webhook_config", req.WebhookConfig},
		{"agent", req.Agent},
		{"agent_config", req.AgentConfig},
	} {
		if len(field.data) > 0 && string(field.data) != "null" {
			return nil, unsupportedInteractionField(field.name)
		}
	}

	messages, err := interactionsInputMessages(req.Input)
	if err != nil {
		return nil, err
	}
	tools := make([]llm.Tool, 0, len(req.Tools))
	for _, tool := range req.Tools {
		if tool.Type != "function" && tool.Type != "" {
			return nil, fmt.Errorf("%w: tool type %q", transformer.ErrInvalidRequest, tool.Type)
		}
		tools = append(tools, llm.Tool{Type: llm.ToolTypeFunction, Function: llm.Function{
			Name: tool.Name, Description: tool.Description, Parameters: tool.Parameters,
		}})
	}
	if req.SystemInstruction != "" {
		messages = append([]llm.Message{{Role: "system", Content: llm.MessageContent{Content: &req.SystemInstruction}}}, messages...)
	}
	var responseFormat *llm.ResponseFormat
	if len(req.ResponseFormat) > 0 && string(req.ResponseFormat) != "null" {
		var parsed llm.ResponseFormat
		if err := json.Unmarshal(req.ResponseFormat, &parsed); err != nil {
			return nil, fmt.Errorf("%w: response_format is invalid", transformer.ErrInvalidRequest)
		}
		responseFormat = &parsed
	}

	return &llm.Request{
		Model: req.Model, Messages: messages, Tools: tools,
		Stream: req.Stream, RequestType: llm.RequestTypeChat,
		APIFormat: llm.APIFormatGeminiInteractions, RawRequest: request,
		Seed:                generationInt(req.GenerationConfig, func(c *interactionsGeneration) *int64 { return c.Seed }),
		MaxCompletionTokens: generationInt(req.GenerationConfig, func(c *interactionsGeneration) *int64 { return c.MaxOutputTokens }),
		Stop:                interactionStop(req.GenerationConfig),
		ResponseFormat:      responseFormat,
	}, nil
}

func unsupportedInteractionField(field string) error {
	return fmt.Errorf("%w: %s", ErrInteractionsUnsupported, field)
}

func interactionStreamFlag(request *httpclient.Request) *bool {
	stream := strings.Contains(request.Path, "/stream") || strings.Contains(request.URL, "/stream")
	return &stream
}

func generationInt(config *interactionsGeneration, get func(*interactionsGeneration) *int64) *int64 {
	if config == nil {
		return nil
	}
	return get(config)
}

func rawString(value string) json.RawMessage {
	if value == "" {
		return nil
	}
	data, _ := json.Marshal(value)
	return data
}

func interactionStop(config *interactionsGeneration) *llm.Stop {
	if config == nil || len(config.StopSequences) == 0 {
		return nil
	}
	return &llm.Stop{MultipleStop: append([]string(nil), config.StopSequences...)}
}

func interactionsInputMessages(input json.RawMessage) ([]llm.Message, error) {
	var text string
	if json.Unmarshal(input, &text) == nil {
		return []llm.Message{{Role: "user", Content: llm.MessageContent{Content: &text}}}, nil
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(input, &items); err != nil {
		return nil, fmt.Errorf("%w: input must be a string or content array", transformer.ErrInvalidRequest)
	}
	messages := make([]llm.Message, 0, len(items))
	for _, item := range items {
		var typ string
		_ = json.Unmarshal(item["type"], &typ)
		switch typ {
		case "text", "":
			var value string
			if err := json.Unmarshal(item["text"], &value); err != nil {
				return nil, fmt.Errorf("%w: text input is invalid", transformer.ErrInvalidRequest)
			}
			messages = append(messages, llm.Message{Role: "user", Content: llm.MessageContent{Content: &value}})
		case "function_result":
			var callID, name string
			_ = json.Unmarshal(item["call_id"], &callID)
			_ = json.Unmarshal(item["name"], &name)
			result := item["result"]
			if len(result) == 0 || !json.Valid(result) {
				return nil, fmt.Errorf("%w: function_result is invalid", transformer.ErrInvalidRequest)
			}
			value := string(result)
			messages = append(messages, llm.Message{Role: "tool", ToolCallID: &callID, ToolCallName: &name, Content: llm.MessageContent{Content: &value}})
		case "function_call":
			var call interactionsStep
			data, _ := json.Marshal(item)
			if err := json.Unmarshal(data, &call); err != nil {
				return nil, fmt.Errorf("%w: function_call is invalid", transformer.ErrInvalidRequest)
			}
			args, err := json.Marshal(call.Arguments)
			if err != nil {
				return nil, err
			}
			messages = append(messages, llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: call.ID, Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: call.Name, Arguments: string(args)}}}})
		case "user_input", "model_output", "thought":
			role := "user"
			if typ != "user_input" {
				role = "assistant"
			}
			contentMessages, err := interactionsStepMessages(item, role, typ == "thought")
			if err != nil {
				return nil, err
			}
			messages = append(messages, contentMessages...)
		default:
			return nil, fmt.Errorf("%w: input type %q", transformer.ErrInvalidRequest, typ)
		}
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("%w: input is empty", transformer.ErrInvalidRequest)
	}
	return messages, nil
}

func interactionsStepMessages(item map[string]json.RawMessage, role string, reasoning bool) ([]llm.Message, error) {
	var contents []interactionsContent
	if len(item["content"]) > 0 {
		var text string
		if json.Unmarshal(item["content"], &text) == nil {
			contents = []interactionsContent{{Type: "text", Text: text}}
		} else if err := json.Unmarshal(item["content"], &contents); err != nil {
			return nil, fmt.Errorf("%w: step content is invalid", transformer.ErrInvalidRequest)
		}
	} else {
		var text string
		if err := json.Unmarshal(item["text"], &text); err != nil {
			return nil, fmt.Errorf("%w: step content is missing", transformer.ErrInvalidRequest)
		}
		contents = []interactionsContent{{Type: "text", Text: text}}
	}
	messages := make([]llm.Message, 0, len(contents))
	for _, content := range contents {
		if content.Type != "text" {
			continue
		}
		value := content.Text
		message := llm.Message{Role: role, Content: llm.MessageContent{Content: &value}}
		if reasoning {
			message.Content.Content = nil
			message.ReasoningContent = &value
		}
		messages = append(messages, message)
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("%w: step content is empty", transformer.ErrInvalidRequest)
	}
	return messages, nil
}

func (t *InteractionsInboundTransformer) TransformResponse(_ context.Context, response *llm.Response) (*httpclient.Response, error) {
	if response == nil {
		return nil, fmt.Errorf("response is nil")
	}
	body, err := json.Marshal(interactionsResponseFromLLM(response))
	if err != nil {
		return nil, err
	}
	return &httpclient.Response{StatusCode: http.StatusOK, Body: body, Headers: http.Header{"Content-Type": []string{"application/json"}}}, nil
}

func (t *InteractionsInboundTransformer) TransformStream(_ context.Context, source streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
	return &interactionsInboundStream{source: source}, nil
}

func (t *InteractionsInboundTransformer) TransformError(_ context.Context, err error) *httpclient.Error {
	message := "internal server error"
	status := http.StatusInternalServerError
	if err != nil {
		message = err.Error()
		if errors.Is(err, transformer.ErrInvalidRequest) || errors.Is(err, ErrInteractionsUnsupported) {
			status = http.StatusBadRequest
		}
		var responseErr *llm.ResponseError
		if errors.As(err, &responseErr) && responseErr.StatusCode != 0 {
			status = responseErr.StatusCode
			if responseErr.Detail.Message != "" {
				message = responseErr.Detail.Message
			}
		}
	}
	body, _ := json.Marshal(interactionsResponse{Object: "interaction", Status: "failed", Error: &interactionsError{Message: message, Code: "api_error", Type: "api_error"}})
	return &httpclient.Error{StatusCode: status, Status: http.StatusText(status), Body: body}
}

func (t *InteractionsInboundTransformer) AggregateStreamChunks(_ context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	var steps []interactionsStep
	var usage *interactionsUsage
	var id, model, status string
	for _, chunk := range chunks {
		if chunk == nil || len(chunk.Data) == 0 || string(chunk.Data) == "[DONE]" {
			continue
		}
		var event interactionsEventData
		if json.Unmarshal(chunk.Data, &event) != nil {
			continue
		}
		if event.Interaction != nil {
			id, model, status, usage = event.Interaction.ID, event.Interaction.Model, event.Interaction.Status, event.Interaction.Usage
			if len(event.Interaction.Steps) > 0 {
				steps = append(steps, event.Interaction.Steps...)
			}
		}
		if event.Status != "" {
			status = event.Status
		}
		if event.Usage != nil {
			usage = event.Usage
		}
		if event.Delta != nil {
			switch event.Delta.Type {
			case "text":
				if len(steps) == 0 || steps[len(steps)-1].Type != "model_output" {
					steps = append(steps, interactionsStep{Type: "model_output"})
				}
				steps[len(steps)-1].Content = append(steps[len(steps)-1].Content, interactionsContent{Type: "text", Text: event.Delta.Text})
			case "arguments_delta":
				if len(steps) == 0 || steps[len(steps)-1].Type != "function_call" {
					steps = append(steps, interactionsStep{Type: "function_call"})
				}
				var args map[string]any
				if json.Unmarshal([]byte(event.Delta.Arguments), &args) == nil {
					steps[len(steps)-1].Arguments = args
				}
			}
		}
	}
	body, err := json.Marshal(interactionsResponse{Object: "interaction", ID: id, Model: model, Status: status, Steps: steps, Usage: usage})
	return body, llm.ResponseMeta{ID: id, Usage: interactionsUsageToLLM(usage), Completed: status == "completed"}, err
}

// InteractionsOutboundTransformer converts unified requests to Google's
// stateless Interactions endpoint.
type InteractionsOutboundTransformer struct{ config Config }

func NewInteractionsOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	return NewInteractionsOutboundTransformerWithConfig(Config{BaseURL: baseURL, APIKeyProvider: auth.NewStaticKeyProvider(apiKey)})
}

func NewInteractionsOutboundTransformerWithConfig(config Config) (transformer.Outbound, error) {
	config = clenupConfig(config)
	if config.EndpointPath == "" {
		config.EndpointPath = "/" + config.APIVersion + "/interactions"
	}
	return &InteractionsOutboundTransformer{config: config}, nil
}

func (t *InteractionsOutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatGeminiInteractions
}

func (t *InteractionsOutboundTransformer) TransformRequest(ctx context.Context, request *llm.Request) (*httpclient.Request, error) {
	if request == nil {
		return nil, fmt.Errorf("request is nil")
	}
	if request.Model == "" || len(request.Messages) == 0 {
		return nil, fmt.Errorf("%w: model and messages are required", transformer.ErrInvalidRequest)
	}
	body, err := json.Marshal(interactionsRequestFromLLM(request))
	if err != nil {
		return nil, err
	}
	accept := "application/json"
	if request.Stream != nil && *request.Stream {
		accept = "text/event-stream"
	}
	headers := http.Header{"Content-Type": []string{"application/json"}, "Accept": []string{accept}}
	var authConfig *httpclient.AuthConfig
	if t.config.APIKeyProvider != nil {
		if key := t.config.APIKeyProvider.Get(ctx); key != "" {
			authConfig = &httpclient.AuthConfig{Type: httpclient.AuthTypeAPIKey, APIKey: key, HeaderKey: "x-goog-api-key"}
		}
	}
	return &httpclient.Request{Method: http.MethodPost, URL: strings.TrimSuffix(t.config.BaseURL, "/") + t.config.EndpointPath, Headers: headers, Body: body, Auth: authConfig, APIFormat: string(llm.APIFormatGeminiInteractions), RequestType: llm.RequestTypeChat.String(), SkipInboundQueryMerge: true}, nil
}

func (t *InteractionsOutboundTransformer) TransformResponse(_ context.Context, response *httpclient.Response) (*llm.Response, error) {
	if response == nil || len(response.Body) == 0 {
		return nil, fmt.Errorf("interactions response body is empty")
	}
	var parsed interactionsResponse
	if err := json.Unmarshal(response.Body, &parsed); err != nil {
		return nil, fmt.Errorf("invalid interactions response: %w", err)
	}
	return interactionsResponseToLLM(&parsed), nil
}

func (t *InteractionsOutboundTransformer) TransformError(_ context.Context, response *httpclient.Error) *llm.ResponseError {
	message := "interactions request failed"
	if response != nil && len(response.Body) > 0 {
		var parsed interactionsResponse
		if json.Unmarshal(response.Body, &parsed) == nil && parsed.Error != nil {
			message = parsed.Error.Message
		} else {
			message = string(response.Body)
		}
	}
	status := http.StatusInternalServerError
	if response != nil && response.StatusCode != 0 {
		status = response.StatusCode
	}
	return &llm.ResponseError{StatusCode: status, Detail: llm.ErrorDetail{Type: "api_error", Message: message}}
}

func (t *InteractionsOutboundTransformer) TransformStream(_ context.Context, request *httpclient.Request, source streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return &interactionsOutboundStream{source: source, model: interactionsModelFromRequest(request)}, nil
}

func interactionsModelFromRequest(request *httpclient.Request) string {
	if request == nil {
		return ""
	}
	var body interactionsRequest
	_ = json.Unmarshal(request.Body, &body)
	return body.Model
}

func (t *InteractionsOutboundTransformer) AggregateStreamChunks(_ context.Context, _ *httpclient.Request, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return (&InteractionsInboundTransformer{}).AggregateStreamChunks(context.Background(), chunks)
}

func interactionsRequestFromLLM(request *llm.Request) interactionsRequest {
	input := make([]any, 0, len(request.Messages))
	var systemInstruction strings.Builder
	for _, message := range request.Messages {
		if message.Role == "system" {
			if message.Content.Content != nil {
				systemInstruction.WriteString(*message.Content.Content)
			}
			continue
		}
		if message.Role == "tool" {
			result := message.Content.Content
			var value any = ""
			if result != nil {
				value = *result
			}
			input = append(input, interactionsStep{Type: "function_result", CallID: stringValue(message.ToolCallID), Name: stringValue(message.ToolCallName), Result: []interactionsContent{{Type: "text", Text: fmt.Sprint(value)}}})
			continue
		}
		if len(message.ToolCalls) > 0 {
			for _, call := range message.ToolCalls {
				var args map[string]any
				_ = json.Unmarshal([]byte(call.Function.Arguments), &args)
				input = append(input, interactionsStep{Type: "function_call", ID: call.ID, Name: call.Function.Name, Arguments: args})
			}
			continue
		}
		text := message.Content.Content
		if text != nil {
			input = append(input, interactionsContent{Type: "text", Text: *text})
		}
	}
	tools := make([]interactionsTool, 0, len(request.Tools))
	for _, tool := range request.Tools {
		if tool.Type == llm.ToolTypeFunction {
			tools = append(tools, interactionsTool{Type: "function", Name: tool.Function.Name, Description: tool.Function.Description, Parameters: tool.Function.Parameters})
		}
	}
	config := &interactionsGeneration{
		Seed:            request.Seed,
		Temperature:     request.Temperature,
		MaxOutputTokens: request.MaxCompletionTokens,
		ThinkingLevel:   request.ReasoningEffort,
	}
	if request.ReasoningSummary != nil {
		config.ThinkingSummary = *request.ReasoningSummary
	}
	if request.ToolChoice != nil {
		config.ToolChoice, _ = json.Marshal(request.ToolChoice)
	}
	if request.Stop != nil {
		if request.Stop.Stop != nil {
			config.StopSequences = []string{*request.Stop.Stop}
		} else {
			config.StopSequences = append([]string(nil), request.Stop.MultipleStop...)
		}
	}
	store := false
	var responseFormat json.RawMessage
	if request.ResponseFormat != nil {
		responseFormat, _ = json.Marshal(request.ResponseFormat)
	}
	return interactionsRequest{Model: request.Model, SystemInstruction: systemInstruction.String(), Input: mustMarshalInput(input), Tools: tools, GenerationConfig: config, ResponseFormat: responseFormat, Store: &store, Stream: request.Stream}
}

func mustMarshalInput(input []any) json.RawMessage {
	data, _ := json.Marshal(input)
	return data
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func interactionsResponseFromLLM(response *llm.Response) interactionsResponse {
	result := interactionsResponse{Object: "interaction", ID: response.ID, Model: response.Model, Status: "completed", Usage: interactionsUsageFromLLM(response.Usage)}
	if response.TransformerMetadata != nil {
		if status, ok := response.TransformerMetadata["interaction_status"].(string); ok && status != "" {
			result.Status = status
		}
		if eventID, ok := response.TransformerMetadata["interaction_event_id"].(string); ok {
			result.EventID = eventID
		}
	}
	if response.Error != nil {
		result.Status = "failed"
		result.Error = &interactionsError{Message: response.Error.Detail.Message, Code: response.Error.Detail.Type, Type: response.Error.Detail.Type}
	}
	for _, choice := range response.Choices {
		message := choice.Message
		if message == nil {
			message = choice.Delta
		}
		if message == nil {
			continue
		}
		if message.Content.Content != nil {
			result.Steps = append(result.Steps, interactionsStep{Type: "model_output", Content: []interactionsContent{{Type: "text", Text: *message.Content.Content}}})
		}
		for _, call := range message.ToolCalls {
			var args map[string]any
			_ = json.Unmarshal([]byte(call.Function.Arguments), &args)
			result.Steps = append(result.Steps, interactionsStep{Type: "function_call", ID: call.ID, Name: call.Function.Name, Arguments: args})
		}
	}
	if len(result.Steps) > 0 {
		for _, step := range result.Steps {
			if step.Type == "function_call" {
				result.Status = "requires_action"
				break
			}
		}
	}
	return result
}

func interactionsResponseToLLM(response *interactionsResponse) *llm.Response {
	result := &llm.Response{ID: response.ID, Model: response.Model, Object: "interaction", Usage: interactionsUsageToLLM(response.Usage), RequestType: llm.RequestTypeChat, APIFormat: llm.APIFormatGeminiInteractions}
	message := &llm.Message{Role: "assistant"}
	for _, step := range response.Steps {
		switch step.Type {
		case "model_output":
			for _, content := range step.Content {
				if content.Type != "text" {
					continue
				}
				value := content.Text
				if message.Content.Content == nil {
					message.Content.Content = &value
				} else {
					*message.Content.Content += value
				}
			}
		case "function_call":
			args, _ := json.Marshal(step.Arguments)
			message.ToolCalls = append(message.ToolCalls, llm.ToolCall{ID: step.ID, Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: step.Name, Arguments: string(args)}, Index: len(message.ToolCalls)})
		}
	}
	if len(message.ToolCalls) > 0 || message.Content.Content != nil {
		result.Choices = []llm.Choice{{Index: 0, Message: message}}
	}
	if response.Status == "failed" || response.Status == "cancelled" || response.Status == "incomplete" {
		result.Error = &llm.ResponseError{StatusCode: http.StatusBadRequest, Detail: llm.ErrorDetail{Type: "api_error", Message: response.ErrorMessage()}}
	}
	return result
}

func (r *interactionsResponse) ErrorMessage() string {
	if r.Error == nil {
		return "interactions failed"
	}
	return r.Error.Message
}

func interactionsUsageFromLLM(usage *llm.Usage) *interactionsUsage {
	if usage == nil {
		return nil
	}
	return &interactionsUsage{InputTokens: usage.PromptTokens, OutputTokens: usage.CompletionTokens, TotalTokens: usage.TotalTokens}
}

func interactionsUsageToLLM(usage *interactionsUsage) *llm.Usage {
	if usage == nil {
		return nil
	}
	input, output := usage.InputTokens, usage.OutputTokens
	if input == 0 {
		input = usage.TotalInputTokens
	}
	if output == 0 {
		output = usage.TotalOutputTokens
	}
	total := usage.TotalTokens
	if total == 0 {
		total = input + output
	}
	return &llm.Usage{PromptTokens: input, CompletionTokens: output, TotalTokens: total}
}

type interactionsInboundStream struct {
	source      streams.Stream[*llm.Response]
	current     *httpclient.StreamEvent
	queue       []*httpclient.StreamEvent
	err         error
	created     bool
	textStarted bool
	toolStarted bool
	terminal    bool
	nextStep    int
	currentStep int
	stepOpen    bool
	doneQueued  bool
}

func (s *interactionsInboundStream) Next() bool {
	if len(s.queue) > 0 {
		s.current, s.queue = s.queue[0], s.queue[1:]
		return true
	}
	for s.source.Next() {
		response := s.source.Current()
		if response == nil {
			continue
		}
		if response.Object == "[DONE]" {
			s.enqueueDone()
			return s.Next()
		}
		if response.Error != nil {
			data := interactionEventJSON("error", interactionsResponseFromLLM(response))
			s.terminal = true
			s.queue = append(s.queue, &httpclient.StreamEvent{Type: "error", Data: data})
			s.enqueueDone()
			return s.Next()
		}
		if response.Object == "interaction.created" {
			s.created = true
			s.queue = append(s.queue, &httpclient.StreamEvent{Type: "interaction.created", Data: interactionEventJSON("interaction.created", interactionsResponseFromLLM(withInteractionStatus(response, "in_progress")))})
			return s.Next()
		}
		if response.Object == "interaction.status_update" {
			s.queue = append(s.queue, &httpclient.StreamEvent{Type: "interaction.status_update", Data: interactionEventJSON("interaction.status_update", interactionsResponseFromLLM(withInteractionStatus(response, "in_progress")))})
			return s.Next()
		}
		if response.Object == "interaction.completed" {
			s.terminal = true
			if s.stepOpen {
				s.queue = append(s.queue, interactionStepStopEvent(s.currentStep))
				s.stepOpen = false
			}
			s.queue = append(s.queue,
				&httpclient.StreamEvent{Type: "interaction.completed", Data: interactionEventJSON("interaction.completed", interactionsResponseFromLLM(response))},
			)
			s.enqueueDone()
			return s.Next()
		}
		if len(response.Choices) > 0 {
			message := response.Choices[0].Delta
			if message == nil {
				message = response.Choices[0].Message
			}
			if message != nil && message.Content.Content != nil {
				if !s.created {
					s.created = true
					created := &llm.Response{ID: response.ID, Model: response.Model, Object: "interaction"}
					s.queue = append(s.queue, &httpclient.StreamEvent{Type: "interaction.created", Data: interactionEventJSON("interaction.created", interactionsResponseFromLLM(withInteractionStatus(created, "in_progress")))})
				}
				if !s.textStarted {
					s.textStarted = true
					s.currentStep = s.nextStep
					s.queue = append(s.queue, &httpclient.StreamEvent{Type: "step.start", Data: interactionStepEventJSON("step.start", s.currentStep, interactionsStep{Type: "model_output"})})
					s.nextStep++
					s.stepOpen = true
				}
				s.queue = append(s.queue, &httpclient.StreamEvent{Type: "step.delta", Data: interactionDeltaEventJSON("step.delta", s.currentStep, interactionsDelta{Type: "text", Text: *message.Content.Content})})
				return s.Next()
			}
			if message != nil && len(message.ToolCalls) > 0 {
				call := message.ToolCalls[0]
				if !s.toolStarted {
					s.toolStarted = true
					s.currentStep = s.nextStep
					s.queue = append(s.queue, &httpclient.StreamEvent{Type: "step.start", Data: interactionStepEventJSON("step.start", s.currentStep, interactionsStep{Type: "function_call", ID: call.ID, Name: call.Function.Name, Arguments: map[string]any{}})})
					s.nextStep++
					s.stepOpen = true
				}
				s.queue = append(s.queue, &httpclient.StreamEvent{Type: "step.delta", Data: interactionDeltaEventJSON("step.delta", s.currentStep, interactionsDelta{Type: "arguments_delta", Arguments: call.Function.Arguments})})
				return s.Next()
			}
		}
	}
	if err := s.source.Err(); err != nil {
		s.err = err
	} else if !s.terminal {
		s.err = ErrInteractionsIncomplete
	}
	return false
}
func (s *interactionsInboundStream) Current() *httpclient.StreamEvent { return s.current }
func (s *interactionsInboundStream) Err() error                       { return s.err }
func (s *interactionsInboundStream) Close() error                     { return s.source.Close() }

func (s *interactionsInboundStream) enqueueDone() {
	if s.doneQueued {
		return
	}
	s.doneQueued = true
	s.queue = append(s.queue, &httpclient.StreamEvent{Type: "done", Data: []byte("[DONE]")})
}

func withInteractionStatus(response *llm.Response, status string) *llm.Response {
	if response == nil {
		return nil
	}
	clone := *response
	clone.TransformerMetadata = make(map[string]any, len(response.TransformerMetadata)+1)
	maps.Copy(clone.TransformerMetadata, response.TransformerMetadata)
	clone.TransformerMetadata["interaction_status"] = status
	return &clone
}

func interactionEventJSON(eventType string, response interactionsResponse) []byte {
	if eventType == "interaction.status_update" {
		data, _ := json.Marshal(struct {
			InteractionID string `json:"interaction_id"`
			Status        string `json:"status"`
			EventID       string `json:"event_id,omitempty"`
			EventType     string `json:"event_type"`
		}{InteractionID: response.ID, Status: response.Status, EventID: response.EventID, EventType: eventType})
		return data
	}
	if eventType == "error" {
		data, _ := json.Marshal(struct {
			Error     *interactionsError `json:"error"`
			EventID   string             `json:"event_id,omitempty"`
			EventType string             `json:"event_type"`
		}{Error: response.Error, EventID: response.EventID, EventType: eventType})
		return data
	}
	data, _ := json.Marshal(struct {
		Interaction interactionsResponse `json:"interaction"`
		EventType   string               `json:"event_type"`
	}{Interaction: response, EventType: eventType})
	return data
}

func interactionStepEventJSON(eventType string, index int, step interactionsStep) []byte {
	data, _ := json.Marshal(struct {
		Index     int              `json:"index"`
		Step      interactionsStep `json:"step"`
		EventType string           `json:"event_type"`
	}{Index: index, Step: step, EventType: eventType})
	return data
}

func interactionDeltaEventJSON(eventType string, index int, delta interactionsDelta) []byte {
	data, _ := json.Marshal(struct {
		Index     int               `json:"index"`
		Delta     interactionsDelta `json:"delta"`
		EventType string            `json:"event_type"`
	}{Index: index, Delta: delta, EventType: eventType})
	return data
}

func interactionStepStopEvent(index int) *httpclient.StreamEvent {
	data, _ := json.Marshal(struct {
		Index     int    `json:"index"`
		EventType string `json:"event_type"`
	}{Index: index, EventType: "step.stop"})
	return &httpclient.StreamEvent{Type: "step.stop", Data: data}
}

type interactionsOutboundStream struct {
	source   streams.Stream[*httpclient.StreamEvent]
	current  *llm.Response
	err      error
	model    string
	terminal bool
	done     bool
	stepID   string
	stepName string
}

func (s *interactionsOutboundStream) Next() bool {
	for s.source.Next() {
		event := s.source.Current()
		if event == nil {
			continue
		}
		if event.Type == "done" || string(event.Data) == "[DONE]" {
			if !s.terminal {
				s.err = ErrInteractionsIncomplete
				return false
			}
			s.done = true
			s.current = llm.DoneResponse
			return true
		}
		if event.Type == "" {
			continue
		}
		response, terminal, err := interactionsEventToLLM(event, s.model, s)
		if err != nil {
			s.err = err
			return false
		}
		if response == nil {
			continue
		}
		s.terminal = s.terminal || terminal
		s.current = response
		return true
	}
	if s.done {
		return false
	}
	if s.err == nil {
		s.err = ErrInteractionsIncomplete
	}
	return false
}
func (s *interactionsOutboundStream) Current() *llm.Response { return s.current }
func (s *interactionsOutboundStream) Err() error {
	if s.err != nil {
		return s.err
	}
	return s.source.Err()
}
func (s *interactionsOutboundStream) Close() error { return s.source.Close() }

func interactionsEventToLLM(event *httpclient.StreamEvent, model string, stream *interactionsOutboundStream) (*llm.Response, bool, error) {
	var data interactionsEventData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return nil, false, err
	}
	response := &llm.Response{Model: model, Object: event.Type, RequestType: llm.RequestTypeChat, APIFormat: llm.APIFormatGeminiInteractions}
	if data.Interaction != nil {
		response.ID = data.Interaction.ID
		if data.Interaction.Model != "" {
			response.Model = data.Interaction.Model
		}
		response.Usage = interactionsUsageToLLM(data.Interaction.Usage)
		if data.Interaction.Status != "" {
			response.TransformerMetadata = map[string]any{"interaction_status": data.Interaction.Status}
		}
	}
	if data.InteractionID != "" {
		response.ID = data.InteractionID
	}
	if data.Status != "" {
		if response.TransformerMetadata == nil {
			response.TransformerMetadata = make(map[string]any)
		}
		response.TransformerMetadata["interaction_status"] = data.Status
	}
	if data.EventID != "" {
		if response.TransformerMetadata == nil {
			response.TransformerMetadata = make(map[string]any)
		}
		response.TransformerMetadata["interaction_event_id"] = data.EventID
	}
	switch event.Type {
	case "interaction.created":
		return response, false, nil
	case "interaction.status_update":
		return response, false, nil
	case "step.start":
		if data.Step == nil {
			return response, false, nil
		}
		if data.Step.Type == "function_call" {
			stream.stepID, stream.stepName = data.Step.ID, data.Step.Name
			response.Choices = []llm.Choice{{Index: 0, Delta: &llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: stream.stepID, Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: stream.stepName}, Index: 0}}}}}
		}
		return response, false, nil
	case "step.delta":
		if data.Delta == nil {
			return response, false, nil
		}
		switch data.Delta.Type {
		case "text":
			value := data.Delta.Text
			response.Choices = []llm.Choice{{Index: 0, Delta: &llm.Message{Role: "assistant", Content: llm.MessageContent{Content: &value}}}}
		case "arguments_delta":
			response.Choices = []llm.Choice{{Index: 0, Delta: &llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: stream.stepID, Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: stream.stepName, Arguments: data.Delta.Arguments}, Index: 0}}}}}
		}
		return response, false, nil
	case "step.stop":
		response.Usage = interactionsUsageToLLM(data.Usage)
		return response, false, nil
	case "interaction.completed":
		if data.Interaction == nil {
			return nil, false, fmt.Errorf("interaction.completed event is missing interaction")
		}
		response := interactionsResponseToLLM(data.Interaction)
		response.Object = event.Type
		return response, true, nil
	case "error":
		if data.Error == nil {
			return nil, false, fmt.Errorf("error event is missing error")
		}
		code := data.Error.Code
		if code == "" {
			code = data.Error.Type
		}
		response.Error = &llm.ResponseError{StatusCode: http.StatusBadRequest, Detail: llm.ErrorDetail{Type: code, Message: data.Error.Message}}
		return response, true, nil
	default:
		return nil, false, nil
	}
}

var (
	_ transformer.Inbound  = (*InteractionsInboundTransformer)(nil)
	_ transformer.Outbound = (*InteractionsOutboundTransformer)(nil)
)
