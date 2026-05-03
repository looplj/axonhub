package responses

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type jsonRawObject map[string]json.RawMessage

func cloneRaw(src json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), src...)
}

func cloneRawMap(src map[string]json.RawMessage) map[string]json.RawMessage {
	if len(src) == 0 {
		return nil
	}

	cloned := make(map[string]json.RawMessage, len(src))
	for key, value := range src {
		cloned[key] = cloneRaw(value)
	}

	return cloned
}

func decodeRawObject(data []byte) (jsonRawObject, error) {
	var obj jsonRawObject
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, fmt.Errorf("json value is not an object")
	}

	return obj, nil
}

func collectExtraFields(data []byte, known map[string]struct{}) map[string]json.RawMessage {
	obj, err := decodeRawObject(data)
	if err != nil {
		return nil
	}

	extra := make(map[string]json.RawMessage)
	for key, value := range obj {
		if _, ok := known[key]; ok {
			continue
		}
		extra[key] = cloneRaw(value)
	}

	if len(extra) == 0 {
		return nil
	}

	return extra
}

func mergeExtraWithStructured(extra map[string]json.RawMessage, structured []byte) ([]byte, error) {
	if len(extra) == 0 {
		return structured, nil
	}

	structuredObj, err := decodeRawObject(structured)
	if err != nil {
		return structured, nil
	}

	merged := make(jsonRawObject, len(extra)+len(structuredObj))
	for key, value := range extra {
		merged[key] = cloneRaw(value)
	}
	for key, value := range structuredObj {
		merged[key] = cloneRaw(value)
	}

	return json.Marshal(merged)
}

func mergeRawObjects(raw map[string]json.RawMessage, structured map[string]json.RawMessage) ([]byte, error) {
	merged := make(jsonRawObject, len(raw)+len(structured))
	for key, value := range raw {
		merged[key] = cloneRaw(value)
	}
	for key, value := range structured {
		merged[key] = cloneRaw(value)
	}

	return json.Marshal(merged)
}

func marshalRaw(value any) (json.RawMessage, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(data), nil
}

func metadataStrings(data json.RawMessage) map[string]string {
	if len(data) == 0 {
		return nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil
	}

	result := make(map[string]string)
	for key, value := range obj {
		if !bytes.HasPrefix(bytes.TrimSpace(value), []byte(`"`)) {
			continue
		}

		var str string
		if err := json.Unmarshal(value, &str); err == nil {
			result[key] = str
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

func metadataExtra(data json.RawMessage) map[string]json.RawMessage {
	if len(data) == 0 {
		return nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil
	}

	return cloneRawMap(obj)
}

func rawField(raw json.RawMessage, key string) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}

	obj, err := decodeRawObject(raw)
	if err != nil {
		return nil
	}

	if value, ok := obj[key]; ok {
		return cloneRaw(value)
	}

	return nil
}

var requestKnownFields = map[string]struct{}{
	"model":                  {},
	"instructions":           {},
	"temperature":            {},
	"input":                  {},
	"tools":                  {},
	"parallel_tool_calls":    {},
	"background":             {},
	"stream":                 {},
	"store":                  {},
	"service_tier":           {},
	"safety_identifier":      {},
	"user":                   {},
	"metadata":               {},
	"max_output_tokens":      {},
	"max_tool_calls":         {},
	"text":                   {},
	"include":                {},
	"previous_response_id":   {},
	"prompt_cache_key":       {},
	"prompt_cache_retention": {},
	"reasoning":              {},
	"stream_options":         {},
	"tool_choice":            {},
	"truncation":             {},
	"top_logprobs":           {},
	"top_p":                  {},
}

var itemKnownFields = map[string]struct{}{
	"id":                {},
	"type":              {},
	"annotations":       {},
	"role":              {},
	"content":           {},
	"status":            {},
	"image_url":         {},
	"detail":            {},
	"text":              {},
	"background":        {},
	"output_format":     {},
	"quality":           {},
	"size":              {},
	"result":            {},
	"call_id":           {},
	"name":              {},
	"arguments":         {},
	"input":             {},
	"output":            {},
	"summary":           {},
	"reasoning_content": {},
	"encrypted_content": {},
	"created_by":        {},
}

var toolKnownFields = map[string]struct{}{
	"type":               {},
	"name":               {},
	"description":        {},
	"parameters":         {},
	"strict":             {},
	"format":             {},
	"background":         {},
	"input_fidelity":     {},
	"model":              {},
	"moderation":         {},
	"output_compression": {},
	"output_format":      {},
	"partial_images":     {},
	"quality":            {},
	"size":               {},
}

var toolChoiceKnownFields = map[string]struct{}{
	"mode":  {},
	"type":  {},
	"name":  {},
	"tools": {},
}
