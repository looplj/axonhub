package responses

import (
	"encoding/json"

	"github.com/looplj/axonhub/llm/internal/pkg/xjson"
)

// RawToolChoiceKind categorizes a wire-format Responses tool selector.
type RawToolChoiceKind string

const (
	RawToolChoiceKindString       RawToolChoiceKind = "string"
	RawToolChoiceKindModeObject   RawToolChoiceKind = "mode"
	RawToolChoiceKindNamed        RawToolChoiceKind = "named"
	RawToolChoiceKindAllowedTools RawToolChoiceKind = "allowed_tools"
	RawToolChoiceKindUnknown      RawToolChoiceKind = "unknown"
)

// RawToolChoiceClass is the shared classification used when preserving raw
// selectors and deciding whether Chat Completions can represent them.
type RawToolChoiceClass struct {
	Kind RawToolChoiceKind
	// SelectorType preserves the wire type for diagnostics.
	SelectorType string
	// FullyRepresented reports whether ToolChoice contains every wire field.
	FullyRepresented bool
}

// ClassifyRawToolChoice classifies selector syntax without discarding unknown
// extensions. Unknown selectors remain replayable as raw Responses payloads.
func ClassifyRawToolChoice(rawChoice json.RawMessage) RawToolChoiceClass {
	if len(rawChoice) == 0 {
		return RawToolChoiceClass{Kind: RawToolChoiceKindUnknown}
	}

	var mode string
	if json.Unmarshal(rawChoice, &mode) == nil {
		return RawToolChoiceClass{Kind: RawToolChoiceKindString, FullyRepresented: true}
	}

	var selector map[string]json.RawMessage
	if json.Unmarshal(rawChoice, &selector) != nil || selector == nil {
		return RawToolChoiceClass{Kind: RawToolChoiceKindUnknown}
	}

	selectorType := ""
	if rawType, ok := selector["type"]; ok {
		_ = json.Unmarshal(rawType, &selectorType)
	}

	if selectorType == ToolChoiceTypeAllowedTools {
		return RawToolChoiceClass{
			Kind:             RawToolChoiceKindAllowedTools,
			SelectorType:     selectorType,
			FullyRepresented: rawAllowedToolsSelectorFullyRepresented(selector),
		}
	}
	if xjson.ObjectHasOnlyFields(selector, "mode") {
		if _, ok := selector["mode"]; ok {
			return RawToolChoiceClass{Kind: RawToolChoiceKindModeObject, FullyRepresented: true}
		}
	}
	if xjson.ObjectHasOnlyFields(selector, "type", "name") {
		_, hasType := selector["type"]
		_, hasName := selector["name"]
		if hasType && hasName {
			return RawToolChoiceClass{
				Kind: RawToolChoiceKindNamed, SelectorType: selectorType, FullyRepresented: true,
			}
		}
	}
	return RawToolChoiceClass{Kind: RawToolChoiceKindUnknown, SelectorType: selectorType}
}

func rawAllowedToolsSelectorFullyRepresented(selector map[string]json.RawMessage) bool {
	if !xjson.ObjectHasOnlyFields(selector, "type", "mode", "tools") {
		return false
	}
	rawTools, ok := selector["tools"]
	if !ok {
		return false
	}
	var tools []map[string]json.RawMessage
	if json.Unmarshal(rawTools, &tools) != nil || tools == nil {
		return false
	}
	for _, tool := range tools {
		if !xjson.ObjectHasOnlyFields(tool, "type", "name") {
			return false
		}
		if _, hasType := tool["type"]; !hasType {
			return false
		}
		if _, hasName := tool["name"]; !hasName {
			return false
		}
	}
	return true
}
