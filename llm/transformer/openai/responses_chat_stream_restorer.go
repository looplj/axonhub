package openai

import (
	"maps"
	"sort"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/streams"
	responsesapi "github.com/looplj/axonhub/llm/transformer/openai/responses"
)

type responsesChatToolChoiceState struct {
	finished bool
	byIndex  map[int]responsesChatToolMapping
	pending  map[int]llm.ToolCall
	ready    map[int]llm.ToolCall
	plain    map[int]struct{}
}

type responsesChatToolStreamRestorer struct {
	mappings map[string]responsesChatToolMapping
	catalog  map[string]struct{}
	prefixes *chatNamePrefixTree
	choices  map[int]*responsesChatToolChoiceState
}

type chatNamePrefixTree struct {
	children map[byte]*chatNamePrefixTree
}

// newResponsesChatToolStreamRestorer creates state for restoring fragmented Chat calls.
func newResponsesChatToolStreamRestorer(
	mappings map[string]responsesChatToolMapping,
	catalogs ...[]string,
) *responsesChatToolStreamRestorer {
	restorer := &responsesChatToolStreamRestorer{
		mappings: mappings,
		catalog:  map[string]struct{}{},
		prefixes: &chatNamePrefixTree{children: map[byte]*chatNamePrefixTree{}},
		choices:  map[int]*responsesChatToolChoiceState{},
	}
	for _, catalog := range catalogs {
		for _, name := range catalog {
			restorer.catalog[name] = struct{}{}
			restorer.prefixes.insert(name)
		}
	}
	for chatName, mapping := range mappings {
		if !mapping.HistoryOnly {
			restorer.prefixes.insert(chatName)
		}
	}
	return restorer
}

func (r *responsesChatToolStreamRestorer) choiceState(choiceIndex int) *responsesChatToolChoiceState {
	state, ok := r.choices[choiceIndex]
	if !ok {
		state = &responsesChatToolChoiceState{
			byIndex: map[int]responsesChatToolMapping{},
			pending: map[int]llm.ToolCall{},
			ready:   map[int]llm.ToolCall{},
			plain:   map[int]struct{}{},
		}
		r.choices[choiceIndex] = state
	}
	return state
}

func (tree *chatNamePrefixTree) insert(name string) {
	current := tree
	for i := range len(name) {
		if current.children == nil {
			current.children = map[byte]*chatNamePrefixTree{}
		}
		child, exists := current.children[name[i]]
		if !exists {
			child = &chatNamePrefixTree{}
			current.children[name[i]] = child
		}
		current = child
	}
}

func (tree *chatNamePrefixTree) hasLongerName() bool {
	return tree != nil && len(tree.children) > 0
}

// responsesChatToolFlushStream releases tool calls the restorer still buffers
// when the upstream stream ends without the finish chunk that would normally
// release them. Without this, providers that omit finish_reason (or emit
// [DONE] in its place) would silently drop every buffered call.
type responsesChatToolFlushStream struct {
	inner        streams.Stream[*llm.Response]
	restorer     *responsesChatToolStreamRestorer
	buffer       []*llm.Response
	current      *llm.Response
	flushed      bool
	strictFinish bool
}

func (s *responsesChatToolFlushStream) Next() bool {
	if len(s.buffer) > 0 {
		s.popBuffered()
		return true
	}

	if !s.inner.Next() {
		s.current = nil
		if s.inner.Err() == nil && s.syntheticFinishIfNeeded() {
			s.popBuffered()
			return true
		}
		if !s.strictFinish && !s.flushed && s.inner.Err() == nil {
			s.flushed = true
			s.buffer = s.restorer.flushBuffered()
			if len(s.buffer) > 0 {
				s.popBuffered()
				return true
			}
		}
		return false
	}

	// Insert flushed calls ahead of the [DONE] sentinel so downstream consumers
	// that stop at a terminal event still observe them.
	if current := s.inner.Current(); !s.flushed &&
		(current == llm.DoneResponse || (current != nil && current.Object == "[DONE]")) {
		if s.syntheticFinishIfNeeded() {
			s.buffer = append(s.buffer, current)
			s.popBuffered()
			return true
		}
		if !s.strictFinish {
			if flushed := s.restorer.flushBuffered(); len(flushed) > 0 {
				s.flushed = true
				s.buffer = append(flushed, current)
				s.popBuffered()
				return true
			}
		}
	}

	s.current = s.inner.Current()
	return true
}

func (s *responsesChatToolFlushStream) popBuffered() {
	s.current = s.buffer[0]
	s.buffer = s.buffer[1:]
}

func (s *responsesChatToolFlushStream) syntheticFinishIfNeeded() bool {
	if !s.strictFinish || s.flushed {
		return false
	}
	choiceIndexes := s.restorer.unfinishedChoiceIndexes()
	if len(choiceIndexes) == 0 {
		return false
	}

	// A missing Chat finish marker is ambiguous, but several OpenAI-compatible
	// providers omit it after a complete stream. Treat the stream as successful
	// and release buffered calls before the synthetic terminal marker so they are
	// not lost when downstream consumers stop at response.completed.
	s.buffer = append(s.buffer, s.restorer.flushBuffered()...)
	for _, choiceIndex := range choiceIndexes {
		finishReason := "stop"
		s.buffer = append(s.buffer, &llm.Response{
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{{
				Index: choiceIndex, Delta: &llm.Message{}, FinishReason: &finishReason,
			}},
		})
	}
	s.flushed = true
	return true
}

func (s *responsesChatToolFlushStream) Current() *llm.Response { return s.current }

func (s *responsesChatToolFlushStream) Err() error { return s.inner.Err() }

func (s *responsesChatToolFlushStream) Close() error { return s.inner.Close() }

// restore restores specialized calls and tracks their names across stream chunks.
// All call state is scoped to the choice index supplied by the provider.
func (r *responsesChatToolStreamRestorer) restore(response *llm.Response) {
	if response == nil {
		return
	}
	for i := range response.Choices {
		state := r.choiceState(response.Choices[i].Index)
		if response.Choices[i].FinishReason != nil {
			state.finished = true
		}
	}
	if len(r.mappings) == 0 && len(r.catalog) == 0 {
		return
	}
	for i := range response.Choices {
		choice := &response.Choices[i]
		abnormalFinish := choice.FinishReason != nil && responsesapi.IsAbnormalChatFinishReason(*choice.FinishReason)
		message := choice.Delta
		if message == nil {
			message = choice.Message
		}
		state := r.choiceState(choice.Index)
		if message != nil {
			incoming := message.ToolCalls
			message.ToolCalls = make([]llm.ToolCall, 0, len(incoming))
			for j := range incoming {
				call := incoming[j]
				if buffered, ok := state.ready[call.Index]; ok {
					name := buffered.Function.Name
					call = mergeResponsesChatToolCallFragments(buffered, call)
					call.Function.Name = name
					state.ready[call.Index] = call
					continue
				}
				if mapping, ok := state.byIndex[call.Index]; ok {
					call.Function.Name = mapping.ChatName
					message.ToolCalls = append(message.ToolCalls, call)
					continue
				}
				if _, ok := state.plain[call.Index]; ok {
					message.ToolCalls = append(message.ToolCalls, call)
					continue
				}

				currentName := call.Function.Name
				if pending, ok := state.pending[call.Index]; ok {
					call = mergeResponsesChatToolCallFragments(pending, call)
					mergedNameValid := r.isKnownName(call.Function.Name) || r.isPotentialKnownName(call.Function.Name)
					currentNameValid := currentName != "" && (r.isKnownName(currentName) || r.isPotentialKnownName(currentName))
					if !mergedNameValid && currentNameValid {
						call.Function.Name = currentName
					}
				}
				potentialLongerName := r.isPotentialKnownName(call.Function.Name)
				if mapping, ok := r.mappings[call.Function.Name]; ok && !mapping.HistoryOnly {
					if call.ID == "" || potentialLongerName {
						state.pending[call.Index] = call
						continue
					}
					state.byIndex[call.Index] = mapping
					delete(state.pending, call.Index)
					state.ready[call.Index] = call
					continue
				}
				if _, exact := r.catalog[call.Function.Name]; exact && call.ID != "" && !potentialLongerName {
					delete(state.pending, call.Index)
					state.plain[call.Index] = struct{}{}
					state.ready[call.Index] = call
					continue
				}
				if call.ID == "" || potentialLongerName {
					state.pending[call.Index] = call
					continue
				}
				delete(state.pending, call.Index)
				state.plain[call.Index] = struct{}{}
				state.ready[call.Index] = call
			}
			if !abnormalFinish {
				r.releaseReady(state, message, false)
			}
		}

		if choice.FinishReason != nil {
			if abnormalFinish {
				// Abnormal finishes truncate only the in-flight call: buffered
				// calls whose identity and arguments already arrived stay
				// consistent with the non-streaming conversion, which keeps
				// every complete call. Drop only pending fragments.
				clear(state.pending)
				if len(state.ready) > 0 {
					if message == nil {
						message = &llm.Message{}
						choice.Delta = message
					}
					r.releaseReady(state, message, true)
				}
			} else {
				if message == nil {
					message = &llm.Message{}
					choice.Delta = message
				}
				for index, call := range state.pending {
					if mapping, ok := r.mappings[call.Function.Name]; ok && !mapping.HistoryOnly {
						state.byIndex[index] = mapping
					} else {
						state.plain[index] = struct{}{}
					}
					state.ready[index] = call
				}
				clear(state.pending)
				r.releaseReady(state, message, true)
			}
		}
		if message != nil {
			restoreResponsesChatMessage(message, r.mappings, false)
		}
	}
}

// flushBuffered releases every call still held when the upstream stream ends
// without a finish chunk that would normally release it. Providers that omit
// finish_reason (or emit [DONE] in its place) would otherwise silently lose
// each buffered call. Results retain their original choice index.
func (r *responsesChatToolStreamRestorer) flushBuffered() []*llm.Response {
	if len(r.choices) == 0 {
		return nil
	}

	choiceIndexes := make([]int, 0, len(r.choices))
	for choiceIndex := range r.choices {
		choiceIndexes = append(choiceIndexes, choiceIndex)
	}
	sort.Ints(choiceIndexes)

	responses := make([]*llm.Response, 0, len(choiceIndexes))
	for _, choiceIndex := range choiceIndexes {
		state := r.choices[choiceIndex]
		indexes := make([]int, 0, len(state.pending)+len(state.ready))
		for index := range state.ready {
			indexes = append(indexes, index)
		}
		for index := range state.pending {
			if _, buffered := state.ready[index]; !buffered {
				indexes = append(indexes, index)
			}
		}
		if len(indexes) == 0 {
			continue
		}
		sort.Ints(indexes)

		calls := make([]llm.ToolCall, 0, len(indexes))
		for _, index := range indexes {
			call, buffered := state.ready[index]
			if !buffered {
				call = state.pending[index]
			}
			delete(state.ready, index)
			delete(state.pending, index)
			calls = append(calls, call)
		}
		message := &llm.Message{ToolCalls: calls}
		restoreResponsesChatMessage(message, r.mappings, false)
		responses = append(responses, &llm.Response{
			Choices: []llm.Choice{{
				Index: choiceIndex,
				Delta: message,
			}},
		})
	}
	if len(responses) == 0 {
		return nil
	}
	return responses
}

// unfinishedChoiceIndexes returns every observed choice that lacks a finish
// marker. A stream with no observed choices still emits choice zero because
// Chat Completions defines that as the default single-choice stream.
func (r *responsesChatToolStreamRestorer) unfinishedChoiceIndexes() []int {
	if len(r.choices) == 0 {
		return []int{0}
	}

	choiceIndexes := make([]int, 0, len(r.choices))
	for choiceIndex, state := range r.choices {
		if !state.finished {
			choiceIndexes = append(choiceIndexes, choiceIndex)
		}
	}
	sort.Ints(choiceIndexes)
	return choiceIndexes
}

// releaseReady starts observed calls in ascending index order. A provider that
// first introduces a lower index after a higher index was already released
// cannot be reordered without buffering every tool call until stream finish.
func (r *responsesChatToolStreamRestorer) releaseReady(
	state *responsesChatToolChoiceState,
	message *llm.Message,
	force bool,
) {
	minPending := 0
	hasPending := false
	for index := range state.pending {
		if hasPending && index >= minPending {
			continue
		}
		minPending = index
		hasPending = true
	}

	readyIndexes := make([]int, 0, len(state.ready))
	for index := range state.ready {
		if force || !hasPending || index < minPending {
			readyIndexes = append(readyIndexes, index)
		}
	}
	sort.Ints(readyIndexes)
	for _, index := range readyIndexes {
		message.ToolCalls = append(message.ToolCalls, state.ready[index])
		delete(state.ready, index)
	}
}

// isKnownName reports whether a name exactly identifies a declared callable tool.
func (r *responsesChatToolStreamRestorer) isKnownName(name string) bool {
	if mapping, ok := r.mappings[name]; ok && !mapping.HistoryOnly {
		return true
	}
	_, ok := r.catalog[name]
	return ok
}

// isPotentialKnownName reports whether a partial name can become a declared tool name.
func (r *responsesChatToolStreamRestorer) isPotentialKnownName(name string) bool {
	current := r.prefixes
	for i := range len(name) {
		current = current.children[name[i]]
		if current == nil {
			return false
		}
	}
	return current.hasLongerName()
}

// mergeResponsesChatToolCallFragments accumulates identity, name, and arguments deltas.
func mergeResponsesChatToolCallFragments(pending, current llm.ToolCall) llm.ToolCall {
	merged := pending
	if current.ID != "" {
		merged.ID = current.ID
	}
	if current.Type != "" {
		merged.Type = current.Type
	}
	merged.Index = current.Index
	merged.Function.Name += current.Function.Name
	merged.Function.Arguments += current.Function.Arguments
	if current.Function.Namespace != "" {
		merged.Function.Namespace = current.Function.Namespace
	}
	if len(current.TransformerMetadata) > 0 {
		if merged.TransformerMetadata == nil {
			merged.TransformerMetadata = map[string]any{}
		}
		maps.Copy(merged.TransformerMetadata, current.TransformerMetadata)
	}
	return merged
}
