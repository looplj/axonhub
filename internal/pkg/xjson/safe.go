package xjson

import (
	"encoding/json"

	"github.com/kaptinlin/jsonrepair"
)

// SafeJSONRawMessage tries to convert a string into a valid JSON RawMessage.
// Strategy:
// 1) If empty or only whitespace, return {}.
// 2) If valid JSON, use it directly.
// 3) Try jsonrepair; if repaired is valid JSON, use it.
// 4) Fallback to {}.
func SafeJSONRawMessage(s string) json.RawMessage {
	if len(s) == 0 {
		return json.RawMessage("{}")
	}

	if json.Valid([]byte(s)) {
		return json.RawMessage(s)
	}

	repaired, err := jsonrepair.JSONRepair(s)
	if err == nil && json.Valid([]byte(repaired)) {
		return json.RawMessage(repaired)
	}

	return json.RawMessage("{}")
}
