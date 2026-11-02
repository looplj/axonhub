package safeguards

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
)

// mergeResults replaces all upstream dangerous_tool_use entries with local
// results while retaining decodable entries of other types. Passing nil results
// strips upstream tool verdicts before the local review is ready.
func mergeResults(raw json.RawMessage, results []Result) json.RawMessage {
	var existing []json.RawMessage
	_ = json.Unmarshal(raw, &existing)
	kept := make([]json.RawMessage, 0, len(existing)+len(results))
	for _, item := range existing {
		var v struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(item, &v) == nil && v.Type != "dangerous_tool_use" {
			kept = append(kept, item)
		}
	}
	for _, result := range results {
		value, _ := json.Marshal(result)
		kept = append(kept, value)
	}
	encoded, _ := json.Marshal(kept)
	return encoded
}

// decodeTool extracts tool-use fields without validating the ID, name or input.
// Evaluate validates completed calls after any streamed arguments are assembled.
func decodeTool(raw json.RawMessage) (ToolUse, bool) {
	var block struct {
		Type  string          `json:"type"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	}
	if json.Unmarshal(raw, &block) != nil || block.Type != "tool_use" {
		return ToolUse{}, false
	}
	return ToolUse{ID: block.ID, Name: block.Name, Input: block.Input}, true
}

// AnnotateResponse replaces dangerous-tool-use verdicts in a complete Anthropic
// response while preserving unrelated safeguards. A nil evaluator is a no-op;
// malformed response JSON returns an error rather than an approval.
func (e *Evaluator) AnnotateResponse(ctx context.Context, body []byte) ([]byte, error) {
	if e == nil {
		return body, nil
	}
	var response map[string]json.RawMessage
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode safeguard response: %w", err)
	}
	if response == nil {
		return nil, fmt.Errorf("expected safeguard response object")
	}
	var blocks []json.RawMessage
	_ = json.Unmarshal(response["content"], &blocks)
	var stop string
	_ = json.Unmarshal(response["stop_reason"], &stop)
	tools := make([]ToolUse, 0)
	for i, block := range blocks {
		if tool, ok := decodeTool(block); ok {
			tool.Truncated = stop == "max_tokens" && i == len(blocks)-1
			tools = append(tools, tool)
		}
	}
	response["safeguard_results"] = mergeResults(response["safeguard_results"], e.Evaluate(ctx, tools))
	return json.Marshal(response)
}

type streamedTool struct {
	tool    ToolUse
	partial []byte
	stopped bool
}

// Stream observes a single Anthropic message. It must not be shared across requests.
// It buffers only bounded tool inputs; text and thinking blocks pass through.
type Stream struct {
	evaluator *Evaluator
	tools     map[int]*streamedTool
	lastIndex int
	results   []Result
	bytes     int
}

// NewStream creates per-message tool-argument state. A nil evaluator leaves all
// events unchanged. Callers must not reuse the stream across messages.
func NewStream(e *Evaluator) *Stream {
	return &Stream{evaluator: e, tools: make(map[int]*streamedTool), lastIndex: -1}
}

// Observe consumes one Anthropic event, removing upstream tool-use verdicts and
// collecting bounded tool inputs. The final message_delta triggers one review;
// missing block stops or invalid arguments produce unavailable verdicts. The
// observer returns an error when required event fields cannot be decoded. The
// caller must validate stream completion before forwarding the reviewed delta.
func (s *Stream) Observe(ctx context.Context, body []byte) ([]byte, error) {
	if s.evaluator == nil {
		return body, nil
	}
	var event map[string]json.RawMessage
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, fmt.Errorf("decode safeguard stream event: %w", err)
	}
	if event == nil {
		return nil, fmt.Errorf("expected safeguard event object")
	}
	// The configured reviewer owns dangerous_tool_use results. Remove upstream
	// verdicts even from early message_start/nonterminal delta events so clients
	// cannot act on an approval before the local review has completed.
	changed := stripUpstreamResults(event)
	for _, key := range []string{"message", "delta"} {
		var nested map[string]json.RawMessage
		if json.Unmarshal(event[key], &nested) == nil && nested != nil && stripUpstreamResults(nested) {
			event[key], _ = json.Marshal(nested)
			changed = true
		}
	}
	if changed {
		body, _ = json.Marshal(event)
	}
	var kind string
	_ = json.Unmarshal(event["type"], &kind)
	var index int
	switch kind {
	case "content_block_start":
		if err := json.Unmarshal(event["index"], &index); err != nil {
			return nil, fmt.Errorf("decode safeguard content block index: %w", err)
		}
		s.lastIndex = index
		if tool, ok := decodeTool(event["content_block"]); ok {
			if _, exists := s.tools[index]; exists {
				s.tools[index].tool.Truncated = true
				return body, nil
			}
			s.bytes += len(tool.Input)
			if s.bytes > maxToolBytes {
				tool.Truncated = true
				tool.Input = nil
			}
			s.tools[index] = &streamedTool{tool: tool}
		}
	case "content_block_delta":
		if err := json.Unmarshal(event["index"], &index); err != nil {
			return nil, fmt.Errorf("decode safeguard content block index: %w", err)
		}
		current := s.tools[index]
		if current == nil {
			return body, nil
		}
		var delta struct {
			Type    string `json:"type"`
			Partial string `json:"partial_json"`
		}
		if err := json.Unmarshal(event["delta"], &delta); err != nil {
			return nil, fmt.Errorf("decode safeguard content block delta: %w", err)
		}
		if delta.Type != "input_json_delta" {
			return body, nil
		}
		if current.stopped {
			current.tool.Truncated = true
		}
		if current.tool.Truncated {
			return body, nil
		}
		s.bytes += len(delta.Partial)
		if s.bytes > maxToolBytes {
			current.tool.Truncated = true
			current.partial = nil
			return body, nil
		}
		current.partial = append(current.partial, delta.Partial...)
	case "content_block_stop":
		if json.Unmarshal(event["index"], &index) == nil && s.tools[index] != nil {
			s.tools[index].stopped = true
		}
	case "message_delta":
		var delta map[string]json.RawMessage
		if err := json.Unmarshal(event["delta"], &delta); err != nil {
			return nil, fmt.Errorf("decode safeguard message delta: %w", err)
		}
		if delta == nil {
			return body, nil
		}
		var stop string
		_ = json.Unmarshal(delta["stop_reason"], &stop)
		if stop == "" {
			return body, nil
		}
		if s.results == nil {
			indices := make([]int, 0, len(s.tools))
			for i := range s.tools {
				indices = append(indices, i)
			}
			sort.Ints(indices)
			tools := make([]ToolUse, 0, len(indices))
			for _, i := range indices {
				current := s.tools[i]
				tool := current.tool
				tool.Truncated = tool.Truncated || !current.stopped || (stop == "max_tokens" && i == s.lastIndex)
				if len(current.partial) > 0 {
					tool.Input = current.partial
					if !json.Valid(tool.Input) {
						tool.Truncated = true
					}
				}
				tools = append(tools, tool)
			}
			s.results = s.evaluator.Evaluate(ctx, tools)
		}
		delta["safeguard_results"] = mergeResults(delta["safeguard_results"], s.results)
		event["delta"], _ = json.Marshal(delta)
		return json.Marshal(event)
	}
	return body, nil
}

// stripUpstreamResults retains unrelated safeguard extensions while taking away
// upstream authority for dangerous_tool_use. Empty results are removed entirely.
func stripUpstreamResults(fields map[string]json.RawMessage) bool {
	raw, ok := fields["safeguard_results"]
	if !ok {
		return false
	}
	merged := mergeResults(raw, nil)
	if string(merged) == "[]" {
		delete(fields, "safeguard_results")
	} else {
		fields["safeguard_results"] = merged
	}
	return true
}
