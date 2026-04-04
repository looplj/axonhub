package anthropic

import (
	"context"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"

	"github.com/looplj/axonhub/axon/agent"
	axoncontext "github.com/looplj/axonhub/axon/context"
)

func (p *Provider) ChatStream(ctx context.Context, model string, tools []agent.ToolDefinition, messages []agent.Message) (<-chan agent.StreamEvent, error) {
	system, msgParams := convertMessages(messages)
	toolParams := convertTools(tools)

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: defaultMaxTokens,
		Messages:  msgParams,
	}
	if len(system) > 0 {
		params.System = system
	}
	if len(toolParams) > 0 {
		params.Tools = toolParams
	}

	if budget := reasoningEffortToBudget(p.reasoningEffort); budget > 0 {
		params.Thinking = anthropic.ThinkingConfigParamUnion{
			OfEnabled: &anthropic.ThinkingConfigEnabledParam{
				BudgetTokens: budget,
			},
		}
	}

	var reqOpts []option.RequestOption
	if threadID := axoncontext.ThreadID(ctx); threadID != "" {
		reqOpts = append(reqOpts, option.WithHeader(p.threadHeader, threadID))
	}
	if traceID := axoncontext.TraceID(ctx); traceID != "" {
		reqOpts = append(reqOpts, option.WithHeader(p.traceHeader, traceID))
	}

	events := make(chan agent.StreamEvent, 256)

	go func() {
		defer close(events)

		stream := p.client.Messages.NewStreaming(ctx, params, reqOpts...)

		streamProcessor := newStreamProcessor(stream, events)
		if err := streamProcessor.process(); err != nil {
			emitError(events, err)
		}
	}()

	return events, nil
}

func emitError(events chan<- agent.StreamEvent, err error) {
	events <- agent.StreamEvent{
		Type:  agent.StreamEventError,
		Error: err,
	}
}

type streamProcessor struct {
	stream              *ssestream.Stream[anthropic.MessageStreamEventUnion]
	events              chan<- agent.StreamEvent
	accumulatedText     strings.Builder
	accumulatedThinking strings.Builder
	thinkingSignature   string
	toolCallBuilders    map[int64]*toolCallAccumulator
	usage               *agent.Usage
}

func newStreamProcessor(stream *ssestream.Stream[anthropic.MessageStreamEventUnion], events chan<- agent.StreamEvent) *streamProcessor {
	return &streamProcessor{
		stream:           stream,
		events:           events,
		toolCallBuilders: make(map[int64]*toolCallAccumulator),
	}
}

func (p *streamProcessor) process() error {
	for p.stream.Next() {
		event := p.stream.Current()

		if err := p.handleEvent(event); err != nil {
			return err
		}
	}

	if err := p.stream.Err(); err != nil {
		return wrapAPIError(err)
	}

	p.emitDone()

	return nil
}

func (p *streamProcessor) handleEvent(event anthropic.MessageStreamEventUnion) error {
	switch event.Type {
	case "message_start":
		return p.handleMessageStart(event)
	case "content_block_start":
		return p.handleContentBlockStart(event)
	case "content_block_delta":
		return p.handleContentBlockDelta(event)
	case "content_block_stop":
		return p.handleContentBlockStop()
	case "message_delta":
		return p.handleMessageDelta(event)
	case "message_stop":
		return nil
	}

	return nil
}

func (p *streamProcessor) handleMessageStart(e anthropic.MessageStreamEventUnion) error {
	p.usage = &agent.Usage{
		InputTokens:  int(e.Message.Usage.InputTokens),
		OutputTokens: int(e.Message.Usage.OutputTokens),
	}
	p.emitUsage(p.usage)
	return nil
}

func (p *streamProcessor) handleContentBlockStart(e anthropic.MessageStreamEventUnion) error {
	if e.ContentBlock.Type == "tool_use" && e.ContentBlock.ID != "" {
		p.toolCallBuilders[e.Index] = &toolCallAccumulator{
			id:   e.ContentBlock.ID,
			name: e.ContentBlock.Name,
		}
	}
	return nil
}

func (p *streamProcessor) handleContentBlockDelta(e anthropic.MessageStreamEventUnion) error {
	delta := e.Delta

	switch delta.Type {
	case "text_delta":
		p.accumulatedText.WriteString(delta.Text)
		p.emit(agent.StreamEvent{
			Type: agent.StreamEventTextDelta,
			Text: delta.Text,
		})

	case "thinking_delta":
		p.accumulatedThinking.WriteString(delta.Thinking)
		p.emit(agent.StreamEvent{
			Type: agent.StreamEventThinkingDelta,
			Thinking: &agent.Thinking{
				Content: delta.Thinking,
			},
		})

	case "input_json_delta":
		if builder, ok := p.toolCallBuilders[e.Index]; ok {
			builder.jsonParts = append(builder.jsonParts, delta.PartialJSON)
			p.emit(agent.StreamEvent{
				Type: agent.StreamEventToolCallDelta,
				Text: delta.PartialJSON,
				ToolCall: &agent.ToolCall{
					ID:   builder.id,
					Name: builder.name,
				},
			})
		}

	case "signature_delta":
		p.thinkingSignature = delta.Signature
	}
	return nil
}

func (p *streamProcessor) handleContentBlockStop() error {
	if p.accumulatedText.Len() > 0 {
		text := p.accumulatedText.String()
		p.emit(agent.StreamEvent{
			Type: agent.StreamEventTextComplete,
			Text: text,
		})
		p.accumulatedText.Reset()
	}

	if p.accumulatedThinking.Len() > 0 {
		thinking := p.accumulatedThinking.String()
		p.emit(agent.StreamEvent{
			Type: agent.StreamEventThinkingComplete,
			Thinking: &agent.Thinking{
				Content:   thinking,
				Signature: p.thinkingSignature,
			},
		})
		p.accumulatedThinking.Reset()
		p.thinkingSignature = ""
	}

	return nil
}

func (p *streamProcessor) handleMessageDelta(e anthropic.MessageStreamEventUnion) error {
	if p.usage == nil {
		p.usage = &agent.Usage{}
	}
	p.usage.OutputTokens = int(e.Usage.OutputTokens)
	p.emitUsage(p.usage)

	if builder, ok := p.toolCallBuilders[e.Index]; ok {
		input := builder.buildJSON()
		p.emit(agent.StreamEvent{
			Type: agent.StreamEventToolCallComplete,
			ToolCall: &agent.ToolCall{
				ID:    builder.id,
				Name:  builder.name,
				Input: input,
			},
		})
	}

	return nil
}

func (p *streamProcessor) emit(ev agent.StreamEvent) {
	select {
	case p.events <- ev:
	default:
	}
}

func (p *streamProcessor) emitUsage(usage *agent.Usage) {
	p.emit(agent.StreamEvent{
		Type:  agent.StreamEventUsage,
		Usage: usage,
	})
}

func (p *streamProcessor) emitError(err error) {
	p.emit(agent.StreamEvent{
		Type:  agent.StreamEventError,
		Error: err,
	})
}

func (p *streamProcessor) emitDone() {
	p.emit(agent.StreamEvent{
		Type: agent.StreamEventDone,
	})
}

type toolCallAccumulator struct {
	id        string
	name      string
	jsonParts []string
}

func (a *toolCallAccumulator) buildJSON() string {
	var sb strings.Builder
	for _, part := range a.jsonParts {
		sb.WriteString(part)
	}
	return sb.String()
}
