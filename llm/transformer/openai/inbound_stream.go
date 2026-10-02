package openai

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/streams"
)

type openAIInboundChoice struct {
	output   bool
	finished bool
	tools    map[int]*llm.ToolCall
}

type openAIInboundStream struct {
	source  streams.Stream[*llm.Response]
	ctx     context.Context
	current *llm.Response
	queue   []*llm.Response
	choices map[int]*openAIInboundChoice
	usage   bool
	done    bool
	ended   bool
	id      string
	model   string
	created int64
}

func newOpenAIInboundStream(source streams.Stream[*llm.Response], ctx context.Context) streams.Stream[*llm.Response] {
	return &openAIInboundStream{source: source, ctx: ctx, choices: make(map[int]*openAIInboundChoice)}
}

func (s *openAIInboundStream) Next() bool {
	if len(s.queue) > 0 {
		s.current = s.queue[0]
		s.queue = s.queue[1:]
		return true
	}
	if s.ended {
		return false
	}
	for s.source.Next() {
		s.current = s.source.Current()
		if s.current != nil {
			s.observe(s.current)
			return true
		}
	}
	s.ended = true
	if s.source.Err() != nil || s.ctx.Err() != nil || s.done || !s.usage {
		return false
	}
	s.finalize()
	return s.Next()
}

func (s *openAIInboundStream) Current() *llm.Response { return s.current }
func (s *openAIInboundStream) Err() error             { return s.source.Err() }
func (s *openAIInboundStream) Close() error           { return s.source.Close() }

func (s *openAIInboundStream) observe(response *llm.Response) {
	if response.Object == "[DONE]" {
		s.done = true
		return
	}
	if response.ID != "" {
		s.id = response.ID
	}
	if response.Model != "" {
		s.model = response.Model
	}
	if response.Created != 0 {
		s.created = response.Created
	}
	if response.Usage != nil {
		s.usage = true
	}
	for _, choice := range response.Choices {
		state := s.choices[choice.Index]
		if state == nil {
			state = &openAIInboundChoice{tools: make(map[int]*llm.ToolCall)}
			s.choices[choice.Index] = state
		}
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			state.finished = true
		}
		for _, message := range []*llm.Message{choice.Delta, choice.Message} {
			if message == nil {
				continue
			}
			if message.Content.Content != nil && *message.Content.Content != "" || len(message.Content.MultipleContent) > 0 ||
				message.ReasoningContent != nil && *message.ReasoningContent != "" || message.Reasoning != nil && *message.Reasoning != "" || message.Refusal != "" || message.Audio != nil {
				state.output = true
			}
			for _, tool := range message.ToolCalls {
				state.output = true
				if state.tools[tool.Index] == nil {
					state.tools[tool.Index] = &llm.ToolCall{}
				}
				acc := state.tools[tool.Index]
				if tool.Function.Name != "" {
					acc.Function.Name = tool.Function.Name
				}
				acc.Function.Arguments += tool.Function.Arguments
			}
		}
	}
}

func (s *openAIInboundStream) finalize() {
	indexes := make([]int, 0, len(s.choices))
	for index, choice := range s.choices {
		if !choice.output {
			return
		}
		for _, tool := range choice.tools {
			if strings.TrimSpace(tool.Function.Name) == "" || !json.Valid([]byte(tool.Function.Arguments)) {
				return
			}
		}
		if !choice.finished {
			indexes = append(indexes, index)
		}
	}
	if len(indexes) == 0 {
		return
	}
	sort.Ints(indexes)
	choices := make([]llm.Choice, 0, len(indexes))
	for _, index := range indexes {
		reason := "stop"
		if len(s.choices[index].tools) > 0 {
			reason = "tool_calls"
		}
		choices = append(choices, llm.Choice{Index: index, Delta: &llm.Message{}, FinishReason: &reason})
	}
	s.queue = []*llm.Response{{ID: s.id, Model: s.model, Created: s.created, Object: "chat.completion.chunk", Choices: choices}, llm.DoneResponse}
	s.queue[0].StreamCompletionEvidence = llm.StreamCompletionEvidenceOpenAIChatEOF
}
