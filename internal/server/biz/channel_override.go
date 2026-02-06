package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
)

// GetBodyOverrideOperations returns the cached override operations for the channel.
// If the operations haven't been parsed yet, it parses and caches them.
// Supports both legacy map format and new operation array format.
//
// WARNING: The returned slice is internal cached state.
// DO NOT modify the returned slice or its contents.
// Modifications will not persist and may cause data inconsistency.
func (c *Channel) GetBodyOverrideOperations() []objects.OverrideOperation {
	if c.cachedOverrideOps != nil {
		return c.cachedOverrideOps
	}

	if c.Settings != nil {
		// New schema field takes precedence. Even an explicit empty slice means "no overrides".
		if c.Settings.BodyOverrideOperations != nil {
			c.cachedOverrideOps = c.Settings.BodyOverrideOperations
			return c.cachedOverrideOps
		}
	}

	if c.Settings == nil || c.Settings.OverrideParameters == "" {
		c.cachedOverrideOps = make([]objects.OverrideOperation, 0)
		return c.cachedOverrideOps
	}

	ops, err := objects.ParseOverrideOperations(c.Settings.OverrideParameters)
	if err != nil {
		log.Warn(context.Background(), "failed to parse override operations",
			log.String("channel", c.Name),
			log.Cause(err),
		)
		c.cachedOverrideOps = make([]objects.OverrideOperation, 0)

		return c.cachedOverrideOps
	}

	c.cachedOverrideOps = ops

	return c.cachedOverrideOps
}

// GetHeaderOverrideOperations returns the cached override headers for the channel.
// If the headers haven't been loaded yet, it loads and caches them.
//
// WARNING: The returned slice is internal cached state.
// DO NOT modify the returned slice or its elements.
// Modifications will not persist and may cause data inconsistency.
func (c *Channel) GetHeaderOverrideOperations() []objects.OverrideOperation {
	if c.cachedOverrideHeaders != nil {
		return c.cachedOverrideHeaders
	}

	if c.Settings != nil {
		// New schema field takes precedence. Even an explicit empty slice means "no overrides".
		if c.Settings.HeaderOverrideOperations != nil {
			c.cachedOverrideHeaders = c.Settings.HeaderOverrideOperations
			return c.cachedOverrideHeaders
		}
	}

	if c.Settings == nil || len(c.Settings.OverrideHeaders) == 0 {
		c.cachedOverrideHeaders = make([]objects.OverrideOperation, 0)
		return c.cachedOverrideHeaders
	}

	c.cachedOverrideHeaders = objects.HeaderEntriesToOverrideOperations(c.Settings.OverrideHeaders)

	return c.cachedOverrideHeaders
}

const ClearHeaderDirective = "__AXONHUB_CLEAR__"

// MergeOverrideHeaders merges existing header operations with a template.
// - For set/delete ops, matching is by Path (case-insensitive). Template overrides existing.
// - For rename/copy ops, they are always appended.
// - Existing ops not mentioned in the template are preserved.
func MergeOverrideHeaders(existing, template []objects.OverrideOperation) []objects.OverrideOperation {
	result := make([]objects.OverrideOperation, 0, len(existing)+len(template))
	result = append(result, existing...)

	for _, op := range template {
		if op.Op == objects.OverrideOpRename || op.Op == objects.OverrideOpCopy {
			result = append(result, op)
			continue
		}

		_, index, found := lo.FindIndexOf(result, func(item objects.OverrideOperation) bool {
			return (item.Op == objects.OverrideOpSet || item.Op == objects.OverrideOpDelete) &&
				strings.EqualFold(item.Path, op.Path)
		})
		if !found {
			result = append(result, op)
			continue
		}

		result[index] = op
	}

	return result
}

func MergeOverrideHeaderEntries(existing, template []objects.HeaderEntry) []objects.HeaderEntry {
	existingOps := objects.HeaderEntriesToOverrideOperations(existing)
	templateOps := objects.HeaderEntriesToOverrideOperations(template)
	mergedOps := MergeOverrideHeaders(existingOps, templateOps)

	return objects.OverrideOperationsToHeaderEntries(mergedOps)
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

// ValidateOverrideParameters checks that params is valid JSON (object or array)
// and that it does not contain the "stream" field (frontend parity).
func ValidateOverrideParameters(params string) error {
	trimmed := strings.TrimSpace(params)
	if trimmed == "" {
		return nil
	}

	ops, err := objects.ParseOverrideOperations(trimmed)
	if err != nil {
		return err
	}

	for _, op := range ops {
		if op.Op == objects.OverrideOpSet && strings.EqualFold(op.Path, "stream") {
			return fmt.Errorf("override parameters cannot contain the field \"stream\"")
		}
	}

	return nil
}

// ValidateBodyOverrideOperations validates body override operations.
// - set/delete: require non-empty Path
// - rename/copy: require non-empty From and To
// - set: cannot set the "stream" field.
func ValidateBodyOverrideOperations(ops []objects.OverrideOperation) error {
	for i, op := range ops {
		switch op.Op {
		case objects.OverrideOpSet:
			if strings.TrimSpace(op.Path) == "" {
				return fmt.Errorf("body operation at index %d (set) has an empty path", i)
			}

			if strings.EqualFold(op.Path, "stream") {
				return fmt.Errorf("override parameters cannot contain the field \"stream\"")
			}
		case objects.OverrideOpDelete:
			if strings.TrimSpace(op.Path) == "" {
				return fmt.Errorf("body operation at index %d (delete) has an empty path", i)
			}
		case objects.OverrideOpRename, objects.OverrideOpCopy:
			if strings.TrimSpace(op.From) == "" || strings.TrimSpace(op.To) == "" {
				return fmt.Errorf("body operation at index %d (%s) requires non-empty from and to", i, op.Op)
			}
		default:
			return fmt.Errorf("body operation at index %d has unknown op %q", i, op.Op)
		}
	}

	return nil
}

// ValidateOverrideHeaders validates override header operations.
// - set/delete: require non-empty Path
// - rename/copy: require non-empty From and To.
func ValidateOverrideHeaders(ops []objects.OverrideOperation) error {
	for i, op := range ops {
		switch op.Op {
		case objects.OverrideOpSet:
			if strings.TrimSpace(op.Path) == "" {
				return fmt.Errorf("header operation at index %d (set) has an empty path", i)
			}
		case objects.OverrideOpDelete:
			if strings.TrimSpace(op.Path) == "" {
				return fmt.Errorf("header operation at index %d (delete) has an empty path", i)
			}
		case objects.OverrideOpRename, objects.OverrideOpCopy:
			if strings.TrimSpace(op.From) == "" || strings.TrimSpace(op.To) == "" {
				return fmt.Errorf("header operation at index %d (%s) requires non-empty from and to", i, op.Op)
			}
		default:
			return fmt.Errorf("header operation at index %d has unknown op %q", i, op.Op)
		}
	}

	return nil
}

func ValidateOverrideHeaderEntries(headers []objects.HeaderEntry) error {
	ops := objects.HeaderEntriesToOverrideOperations(headers)
	return ValidateOverrideHeaders(ops)
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

	maps.Copy(result, base)

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
