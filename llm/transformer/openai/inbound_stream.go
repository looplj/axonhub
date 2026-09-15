package openai

import (
	"sort"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/streams"
)

// openAIInboundStream finalizes complete OpenAI Chat Completions responses when
// an upstream provider closes a clean stream without finish_reason or [DONE].
// Usage is used as the positive completion signal so truncated streams are not
// silently converted into successful responses.
type openAIInboundStream struct {
	source     streams.Stream[*llm.Response]
	queue      []*llm.Response
	queueIndex int
	current    *llm.Response
	finalized  bool

	responseID    string
	responseModel string
	responseTime  int64
	usage         *llm.Usage
	doneSeen      bool
	choices       map[int]*openAIInboundChoice
}

type openAIInboundChoice struct {
	index        int
	hasOutput    bool
	hasToolCalls bool
	finished     bool
}

func newOpenAIInboundStream(source streams.Stream[*llm.Response]) streams.Stream[*llm.Response] {
	return &openAIInboundStream{
		source:  source,
		choices: make(map[int]*openAIInboundChoice),
	}
}

func (s *openAIInboundStream) Next() bool {
	if s.nextQueued() {
		return true
	}

	s.queue = nil
	s.queueIndex = 0

	if s.source.Next() {
		response := s.source.Current()
		if response == nil {
			return s.Next()
		}

		s.observe(response)
		s.current = response

		return true
	}

	if s.source.Err() != nil {
		return false
	}

	s.finalize()

	return s.nextQueued()
}

func (s *openAIInboundStream) nextQueued() bool {
	if s.queueIndex >= len(s.queue) {
		return false
	}

	s.current = s.queue[s.queueIndex]
	s.queueIndex++

	return true
}

func (s *openAIInboundStream) Current() *llm.Response {
	return s.current
}

func (s *openAIInboundStream) Err() error {
	return s.source.Err()
}

func (s *openAIInboundStream) Close() error {
	return s.source.Close()
}

func (s *openAIInboundStream) observe(response *llm.Response) {
	if response == nil {
		return
	}

	if response.Object == "[DONE]" {
		s.doneSeen = true
		return
	}

	if response.ID != "" {
		s.responseID = response.ID
	}
	if response.Model != "" {
		s.responseModel = response.Model
	}
	if response.Created != 0 {
		s.responseTime = response.Created
	}
	if response.Usage != nil {
		s.usage = response.Usage
	}

	for _, choice := range response.Choices {
		state, ok := s.choices[choice.Index]
		if !ok {
			state = &openAIInboundChoice{index: choice.Index}
			s.choices[choice.Index] = state
		}

		if choice.FinishReason != nil && strings.TrimSpace(*choice.FinishReason) != "" {
			state.finished = true
		}

		for _, message := range []*llm.Message{choice.Delta, choice.Message} {
			if message == nil {
				continue
			}

			if len(message.ToolCalls) > 0 {
				state.hasToolCalls = true
			}
			if hasOpenAIInboundOutput(message) {
				state.hasOutput = true
			}
		}
	}
}

func hasOpenAIInboundOutput(message *llm.Message) bool {
	if message == nil {
		return false
	}

	if message.Content.Content != nil && *message.Content.Content != "" {
		return true
	}
	if len(message.Content.MultipleContent) > 0 {
		return true
	}
	if len(message.ToolCalls) > 0 {
		return true
	}
	if message.ReasoningContent != nil && *message.ReasoningContent != "" {
		return true
	}
	if message.Reasoning != nil && *message.Reasoning != "" {
		return true
	}
	if message.Refusal != "" || message.Audio != nil || len(message.InlineToolResults) > 0 {
		return true
	}

	return false
}

func (s *openAIInboundStream) finalize() {
	if s.finalized {
		return
	}
	s.finalized = true

	if s.doneSeen {
		return
	}

	missingChoices := s.missingChoices()
	if len(missingChoices) == 0 {
		return
	}

	// A clean EOF without usage is still ambiguous: it may be a truncated
	// response. Leave it to the existing incomplete-stream handling instead of
	// fabricating a successful finish reason.
	if s.usage == nil {
		return
	}

	choices := make([]llm.Choice, 0, len(missingChoices))
	for _, state := range missingChoices {
		finishReason := "stop"
		if state.hasToolCalls {
			finishReason = "tool_calls"
		}

		choices = append(choices, llm.Choice{
			Index:        state.index,
			Delta:        &llm.Message{},
			FinishReason: &finishReason,
		})
	}

	s.queue = append(s.queue, &llm.Response{
		ID:      s.responseID,
		Object:  "chat.completion.chunk",
		Created: s.responseTime,
		Model:   s.responseModel,
		Choices: choices,
	})
	s.queue = append(s.queue, llm.DoneResponse)
}

func (s *openAIInboundStream) missingChoices() []*openAIInboundChoice {
	indexes := make([]int, 0, len(s.choices))
	for index, state := range s.choices {
		if state.hasOutput && !state.finished {
			indexes = append(indexes, index)
		}
	}
	sort.Ints(indexes)

	missing := make([]*openAIInboundChoice, 0, len(indexes))
	for _, index := range indexes {
		missing = append(missing, s.choices[index])
	}

	return missing
}
