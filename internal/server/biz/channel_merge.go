package biz

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/looplj/axonhub/internal/objects"
)

const clearHeaderDirective = "__AXONHUB_CLEAR__"

// MergeOverrideHeaders merges existing headers with a template.
// - Header key comparison is case-insensitive (strings.EqualFold).
// - Template entries override existing ones with the same key.
// - Template entries with value "__AXONHUB_CLEAR__" remove the header.
// - Existing headers not mentioned in the template are preserved.
func MergeOverrideHeaders(existing, template []objects.HeaderEntry) []objects.HeaderEntry {
	result := make([]objects.HeaderEntry, 0, len(existing)+len(template))
	result = append(result, existing...)

	for _, header := range template {
		index := -1
		for i, item := range result {
			if strings.EqualFold(item.Key, header.Key) {
				index = i
				break
			}
		}

		if header.Value == clearHeaderDirective {
			if index >= 0 {
				result = append(result[:index], result[index+1:]...)
			}
			continue
		}

		if index >= 0 {
			result[index] = header
		} else {
			result = append(result, header)
		}
	}

	return result
}

// MergeOverrideParameters deep-merges two JSON object strings.
// - Both inputs must be JSON objects; otherwise, an error is returned.
// - Nested objects are merged recursively; scalars/arrays are overwritten by the template.
func MergeOverrideParameters(existing, template string) (string, error) {
	existingObj, err := parseJSONObject(existing)
	if err != nil {
		return "", fmt.Errorf("invalid existing override parameters: %w", err)
	}

	templateObj, err := parseJSONObject(template)
	if err != nil {
		return "", fmt.Errorf("invalid template override parameters: %w", err)
	}

	merged := deepMergeMap(existingObj, templateObj)

	bytes, err := json.Marshal(merged)
	if err != nil {
		return "", fmt.Errorf("failed to marshal merged override parameters: %w", err)
	}

	return string(bytes), nil
}

// NormalizeOverrideParameters converts empty or whitespace-only strings to "{}".
// This ensures consistent representation across the system.
func NormalizeOverrideParameters(params string) string {
	if strings.TrimSpace(params) == "" {
		return "{}"
	}
	return params
}

// ValidateOverrideParameters checks that params is a valid JSON object and
// that it does not contain the "stream" field (frontend parity).
func ValidateOverrideParameters(params string) error {
	trimmed := strings.TrimSpace(params)
	if trimmed == "" {
		return nil
	}

	obj, err := parseJSONObject(trimmed)
	if err != nil {
		return err
	}

	if _, exists := obj["stream"]; exists {
		return fmt.Errorf("override parameters cannot contain the field \"stream\"")
	}

	return nil
}

// ValidateOverrideHeaders ensures header keys are non-empty and unique (case-insensitive).
func ValidateOverrideHeaders(headers []objects.HeaderEntry) error {
	for i, header := range headers {
		if strings.TrimSpace(header.Key) == "" {
			return fmt.Errorf("header at index %d has an empty key", i)
		}

		for j := 0; j < i; j++ {
			if strings.EqualFold(headers[j].Key, header.Key) {
				return fmt.Errorf("duplicate header key (case-insensitive): %s", header.Key)
			}
		}
	}

	return nil
}

func parseJSONObject(input string) (map[string]any, error) {
	if strings.TrimSpace(input) == "" {
		return map[string]any{}, nil
	}

	var parsed any
	if err := json.Unmarshal([]byte(input), &parsed); err != nil {
		return nil, fmt.Errorf("must be valid JSON: %w", err)
	}

	obj, ok := parsed.(map[string]any)
	if !ok || obj == nil {
		return nil, fmt.Errorf("override parameters must be a JSON object")
	}

	return obj, nil
}

func deepMergeMap(base, override map[string]any) map[string]any {
	result := make(map[string]any, len(base)+len(override))

	for k, v := range base {
		result[k] = v
	}

	for k, overrideVal := range override {
		if baseVal, exists := result[k]; exists {
			baseMap, baseIsMap := baseVal.(map[string]any)
			overrideMap, overrideIsMap := overrideVal.(map[string]any)

			if baseIsMap && overrideIsMap {
				result[k] = deepMergeMap(baseMap, overrideMap)
				continue
			}
		}

		result[k] = overrideVal
	}

	return result
}
