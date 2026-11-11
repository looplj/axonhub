package plugin

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
)

// applyHookReturn always applies patches to the untouched pre-call JSON.
func applyHookReturn(before, value json.RawMessage) (json.RawMessage, error) {
	value = bytes.TrimSpace(value)
	if len(value) == 0 {
		return nil, nil
	}
	if value[0] == '[' {
		patch, err := jsonpatch.DecodePatch(value)
		if err != nil {
			return nil, err
		}
		for _, op := range patch {
			path, err := op.Path()
			if err != nil || !validJSONPointer(path) {
				return nil, errors.New("invalid JSON Pointer path")
			}
			if op.Kind() == "move" || op.Kind() == "copy" {
				from, err := op.From()
				if err != nil || !validJSONPointer(from) {
					return nil, errors.New("invalid JSON Pointer from")
				}
			}
			if op.Kind() == "test" {
				if _, ok := op["value"]; !ok {
					return nil, errors.New("test requires value")
				}
			}
		}
		options := jsonpatch.NewApplyOptions()
		options.SupportNegativeIndices = false
		options.AccumulatedCopySizeLimit = hookInputLimit
		value, err = patch.ApplyWithOptions(before, options)
		if err != nil {
			return nil, err
		}
	}
	if !isJSONObject(value) {
		return nil, errors.New("plugin hook must return an object or JSON Patch array")
	}
	return value, nil
}

func validateHookReturn(value json.RawMessage) error {
	value = bytes.TrimSpace(value)
	if len(value) == 0 || isJSONObject(value) {
		return nil
	}
	if value[0] == '[' {
		_, err := jsonpatch.DecodePatch(value)
		return err
	}
	return errors.New("plugin hook must return an object or JSON Patch array")
}

func validJSONPointer(path string) bool {
	if path != "" && !strings.HasPrefix(path, "/") {
		return false
	}
	for i := 0; i < len(path); i++ {
		if path[i] == '~' {
			i++
			if i == len(path) || (path[i] != '0' && path[i] != '1') {
				return false
			}
		}
	}
	return true
}
