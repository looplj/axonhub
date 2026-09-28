package orchestrator

import (
	"bytes"

	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

type StreamTerminalTracker struct {
	expectedChoices int
	choices         map[int]struct{}
	finished        map[int]struct{}
	terminal        bool
}

func NewStreamTerminalTracker(expectedChoices int) *StreamTerminalTracker {
	return &StreamTerminalTracker{
		expectedChoices: expectedChoices,
		choices:         make(map[int]struct{}),
		finished:        make(map[int]struct{}),
	}
}

func NewStreamTerminalTrackerForRequest(request *ent.Request) *StreamTerminalTracker {
	if request == nil {
		return NewStreamTerminalTracker(0)
	}
	expected := ExpectedStreamChoices(request)
	return NewStreamTerminalTracker(expected)
}

func ExpectedStreamChoices(request *ent.Request) int {
	if request == nil {
		return 0
	}
	if n := gjson.GetBytes(request.RequestBody, "n"); n.Exists() {
		if n.Int() > 0 {
			return int(n.Int())
		}
		return 0
	}
	if gjson.GetBytes(request.RequestBody, "stream").Bool() {
		return 1
	}
	return 0
}

func (t *StreamTerminalTracker) Observe(event *httpclient.StreamEvent) bool {
	if t.terminal || event == nil {
		return t.terminal
	}
	if bytes.Equal(event.Data, llm.DoneStreamEvent.Data) || isNonChoiceTerminalEvent(event) {
		t.terminal = true
		return true
	}

	choices := gjson.GetBytes(event.Data, "choices")
	if choices.IsArray() {
		choices.ForEach(func(_, choice gjson.Result) bool {
			index := int(choice.Get("index").Int())
			t.choices[index] = struct{}{}
			if choice.Get("finish_reason").Type == gjson.String && choice.Get("finish_reason").String() != "" {
				t.finished[index] = struct{}{}
			}
			return true
		})
		if t.expectedChoices > 0 && len(t.finished) >= t.expectedChoices && len(t.choices) == t.expectedChoices {
			t.terminal = true
		}
	}

	if !t.terminal {
		candidates := gjson.GetBytes(event.Data, "candidates")
		if candidates.IsArray() && hasNonEmptyJSONStringField(event.Data, "candidates", "finishReason") {
			t.terminal = true
		}
	}

	return t.terminal
}

func (t *StreamTerminalTracker) AllChoicesFinished() bool {
	return t.expectedChoices > 0 && len(t.choices) == t.expectedChoices && len(t.finished) == t.expectedChoices
}

func isNonChoiceTerminalEvent(event *httpclient.StreamEvent) bool {
	eventType := gjson.GetBytes(event.Data, "type").String()
	return isResponsesTerminalEvent(event.Type) ||
		isResponsesTerminalEvent(eventType) ||
		event.Type == "message_stop" ||
		eventType == "message_stop" ||
		event.Type == "speech.audio.done" ||
		eventType == "speech.audio.done" ||
		event.Type == "transcript.text.done" ||
		eventType == "transcript.text.done" ||
		event.Type == httpclient.BinaryStreamDoneEventType
}
