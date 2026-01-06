package bailian

import (
	"strings"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

type toolCallKey struct {
	choiceIndex int
	callIndex   int
}

type bailianStreamFilter struct {
	source  streams.Stream[*llm.Response]
	current *llm.Response

	pending []*llm.Response

	sawToolCalls   bool
	bufferedText   strings.Builder
	lastTextChoice int
	toolArgs       map[toolCallKey]string
}

func newBailianStreamFilter(source streams.Stream[*llm.Response]) streams.Stream[*llm.Response] {
	return &bailianStreamFilter{
		source:   source,
		toolArgs: make(map[toolCallKey]string),
	}
}

func (s *bailianStreamFilter) Next() bool {
	if len(s.pending) > 0 {
		s.current = s.pending[0]
		s.pending = s.pending[1:]
		return true
	}

	for s.source.Next() {
		resp := s.source.Current()
		if resp == nil {
			continue
		}

		if resp == llm.DoneResponse {
			s.current = resp
			return true
		}

		filtered := s.filterResponse(resp)
		if len(s.pending) > 0 {
			s.current = s.pending[0]
			s.pending = s.pending[1:]
			return true
		}
		if filtered == nil {
			continue
		}

		s.current = filtered
		return true
	}

	return false
}

func (s *bailianStreamFilter) Current() *llm.Response {
	return s.current
}

func (s *bailianStreamFilter) Err() error {
	return s.source.Err()
}

func (s *bailianStreamFilter) Close() error {
	return s.source.Close()
}

func (s *bailianStreamFilter) filterResponse(resp *llm.Response) *llm.Response {
	if resp == nil {
		return nil
	}

	hasFinish := false
	for i := range resp.Choices {
		choice := &resp.Choices[i]
		if choice.FinishReason != nil {
			hasFinish = true
		}

		if choice.Delta != nil && len(choice.Delta.ToolCalls) > 0 {
			s.sawToolCalls = true
			s.bufferedText.Reset()
			s.filterToolCalls(choice.Index, choice.Delta.ToolCalls)
		}
	}

	for i := range resp.Choices {
		choice := &resp.Choices[i]
		text := extractTextDelta(choice)
		if text == "" {
			continue
		}

		if s.sawToolCalls {
			continue
		}

		s.lastTextChoice = choice.Index
		s.bufferedText.WriteString(text)
	}

	if hasFinish && !s.sawToolCalls && s.bufferedText.Len() > 0 {
		textChunk := buildTextChunk(resp, s.lastTextChoice, s.bufferedText.String())
		s.bufferedText.Reset()
		s.pending = append(s.pending, textChunk, resp)
		return nil
	}

	return resp
}

func (s *bailianStreamFilter) filterToolCalls(choiceIndex int, toolCalls []llm.ToolCall) {
	if len(toolCalls) == 0 {
		return
	}

	for i := range toolCalls {
		tc := &toolCalls[i]
		arg := tc.Function.Arguments
		if arg == "" {
			continue
		}

		key := toolCallKey{choiceIndex: choiceIndex, callIndex: tc.Index}
		if strings.TrimSpace(arg) == "{}" && strings.TrimSpace(s.toolArgs[key]) != "" {
			tc.Function.Arguments = ""
			continue
		}

		s.toolArgs[key] += arg
	}
}

func extractTextDelta(choice *llm.Choice) string {
	if choice == nil || choice.Delta == nil {
		return ""
	}

	var text strings.Builder
	if choice.Delta.Content.Content != nil {
		value := *choice.Delta.Content.Content
		if value != "" {
			text.WriteString(value)
		}
		choice.Delta.Content.Content = nil
	}

	if len(choice.Delta.Content.MultipleContent) > 0 {
		kept := choice.Delta.Content.MultipleContent[:0]
		for _, part := range choice.Delta.Content.MultipleContent {
			if part.Type == "text" && part.Text != nil && *part.Text != "" {
				text.WriteString(*part.Text)
				continue
			}
			kept = append(kept, part)
		}
		choice.Delta.Content.MultipleContent = kept
	}

	return text.String()
}

func buildTextChunk(base *llm.Response, choiceIndex int, text string) *llm.Response {
	if base == nil {
		return nil
	}

	textCopy := text
	return &llm.Response{
		ID:                base.ID,
		Object:            base.Object,
		Created:           base.Created,
		Model:             base.Model,
		SystemFingerprint: base.SystemFingerprint,
		ServiceTier:       base.ServiceTier,
		Choices: []llm.Choice{{
			Index: choiceIndex,
			Delta: &llm.Message{
				Content: llm.MessageContent{
					Content: &textCopy,
				},
			},
		}},
	}
}
