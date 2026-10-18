package openai

import (
	"fmt"

	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
)

// toolCallIndexer fills in the tool call indexes that some upstreams leave out of stream chunks.
// Gemini's OpenAI-compatible endpoint sends each parallel tool call whole in its own chunk without
// an index, so every call would decode as index 0 and be merged into one call downstream.
type toolCallIndexer struct {
	// next is the index of the next new tool call, per choice index.
	next map[int]int
	// lastID is the ID of the last tool call, per choice index.
	lastID map[int]string
}

func newToolCallIndexer() *toolCallIndexer {
	return &toolCallIndexer{next: make(map[int]int), lastID: make(map[int]string)}
}

// index returns the index of the tool call at choices[choicePos].delta.tool_calls[pos] of the raw chunk.
// A call without an index starts a new call when it has a new ID or a name, and otherwise continues the last call.
func (x *toolCallIndexer) index(data []byte, choicePos, choiceIndex, pos, index int, id, name string) int {
	if id != "" {
		defer func() { x.lastID[choiceIndex] = id }()
	}

	if gjson.GetBytes(data, fmt.Sprintf("choices.%d.delta.tool_calls.%d.index", choicePos, pos)).Exists() {
		x.next[choiceIndex] = max(x.next[choiceIndex], index+1)
		return index
	}

	continues := x.next[choiceIndex] > 0 && (id == "" && name == "" || id != "" && id == x.lastID[choiceIndex])
	if continues {
		return x.next[choiceIndex] - 1
	}

	index = x.next[choiceIndex]
	x.next[choiceIndex]++

	return index
}

// applyToResponse relies on resp keeping the raw chunk's choices and tool calls in order,
// as Response.ToLLMResponse and Message.ToLLMMessage map them one to one.
func (x *toolCallIndexer) applyToResponse(data []byte, resp *llm.Response) {
	for i := range resp.Choices {
		if resp.Choices[i].Delta == nil {
			continue
		}

		for j := range resp.Choices[i].Delta.ToolCalls {
			toolCall := &resp.Choices[i].Delta.ToolCalls[j]
			toolCall.Index = x.index(data, i, resp.Choices[i].Index, j, toolCall.Index, toolCall.ID, toolCall.Function.Name)
		}
	}
}

func (x *toolCallIndexer) applyToChunk(data []byte, chunk *Response) {
	for i := range chunk.Choices {
		if chunk.Choices[i].Delta == nil {
			continue
		}

		for j := range chunk.Choices[i].Delta.ToolCalls {
			toolCall := &chunk.Choices[i].Delta.ToolCalls[j]
			toolCall.Index = x.index(data, i, chunk.Choices[i].Index, j, toolCall.Index, toolCall.ID, toolCall.Function.Name)
		}
	}
}
