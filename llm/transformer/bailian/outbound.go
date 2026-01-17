package bailian

import (
	"context"
	"fmt"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// Config holds all configuration for the Bailian outbound transformer.
type Config struct {
	BaseURL string `json:"base_url,omitempty"`
	APIKey  string `json:"api_key,omitempty"`

	// ReplaceDeveloperRoleWithSystem replaces developer role with system in messages.
	ReplaceDeveloperRoleWithSystem bool `json:"replaceDeveloperRoleWithSystem,omitempty"`
}

// OutboundTransformer implements transformer.Outbound for Bailian (OpenAI-compatible) format.
type OutboundTransformer struct {
	transformer.Outbound

	config *Config
}

// NewOutboundTransformer creates a new Bailian OutboundTransformer with legacy parameters.
// Deprecated: Use NewOutboundTransformerWithConfig instead.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		BaseURL: baseURL,
		APIKey:  apiKey,
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new Bailian OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	if config == nil {
		return nil, fmt.Errorf("invalid Bailian transformer configuration: config is nil")
	}

	base, err := openai.NewOutboundTransformer(config.BaseURL, config.APIKey)
	if err != nil {
		return nil, fmt.Errorf("invalid Bailian transformer configuration: %w", err)
	}

	return &OutboundTransformer{Outbound: base, config: config}, nil
}

// TransformRequest applies Bailian-specific request normalization before delegating to OpenAI-compatible transformer.
func (t *OutboundTransformer) TransformRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if t.config != nil && t.config.ReplaceDeveloperRoleWithSystem {
		llmReq = replaceDeveloperRole(llmReq)
	}

	llmReq = mergeConsecutiveToolCallMessages(llmReq)

	return t.Outbound.TransformRequest(ctx, llmReq)
}

func replaceDeveloperRole(req *llm.Request) *llm.Request {
	if req == nil || len(req.Messages) == 0 {
		return req
	}

	replaced := false

	messages := make([]llm.Message, len(req.Messages))
	for i, msg := range req.Messages {
		if strings.EqualFold(msg.Role, "developer") {
			msg.Role = "system"
			replaced = true
		}

		messages[i] = msg
	}

	if !replaced {
		return req
	}

	updated := *req
	updated.Messages = messages

	return &updated
}

func mergeConsecutiveToolCallMessages(req *llm.Request) *llm.Request {
	if req == nil || len(req.Messages) < 2 {
		return req
	}

	changed := false
	messages := make([]llm.Message, 0, len(req.Messages))

	var pending *llm.Message

	for i := range req.Messages {
		msg := req.Messages[i]

		if isMergeableToolCallMessage(msg) {
			if pending == nil {
				pendingMsg := msg
				pending = &pendingMsg
			} else {
				pending.ToolCalls = append(pending.ToolCalls, msg.ToolCalls...)
				changed = true
			}

			continue
		}

		if pending != nil {
			messages = append(messages, *pending)
			pending = nil
		}

		messages = append(messages, msg)
	}

	if pending != nil {
		messages = append(messages, *pending)
	}

	if !changed {
		return req
	}

	updated := *req
	updated.Messages = messages

	return &updated
}

func isMergeableToolCallMessage(msg llm.Message) bool {
	if !strings.EqualFold(msg.Role, "assistant") {
		return false
	}

	if len(msg.ToolCalls) == 0 {
		return false
	}

	if msg.ToolCallID != nil || msg.Name != nil || msg.Refusal != "" || msg.MessageIndex != nil || msg.ToolCallName != nil || msg.ToolCallIsError != nil {
		return false
	}

	if msg.ReasoningContent != nil || msg.ReasoningSignature != nil || msg.RedactedReasoningContent != nil || msg.CacheControl != nil {
		return false
	}

	return isEmptyMessageContent(msg.Content)
}

func isEmptyMessageContent(content llm.MessageContent) bool {
	if content.Content != nil && *content.Content != "" {
		return false
	}

	return len(content.MultipleContent) == 0
}

// TransformStream applies Bailian-specific streaming normalization on top of OpenAI-compatible stream.
func (t *OutboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	baseStream, err := t.Outbound.TransformStream(ctx, stream)
	if err != nil {
		return nil, err
	}

	return newBailianStreamFilter(baseStream), nil
}
