package responses

import "encoding/json"

func cloneRaw(data []byte) json.RawMessage {
	if len(data) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), data...)
}

func extraFields(data []byte, known map[string]struct{}) map[string]json.RawMessage {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil || len(raw) == 0 {
		return nil
	}

	for key := range known {
		delete(raw, key)
	}
	if len(raw) == 0 {
		return nil
	}

	return raw
}

func mergeJSONObjects(raw json.RawMessage, extra map[string]json.RawMessage, structured []byte) ([]byte, error) {
	out := map[string]json.RawMessage{}

	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	for key, value := range extra {
		out[key] = value
	}

	var structuredMap map[string]json.RawMessage
	if err := json.Unmarshal(structured, &structuredMap); err != nil {
		return nil, err
	}
	for key, value := range structuredMap {
		out[key] = value
	}

	return json.Marshal(out)
}

func keys(names ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(names))
	for _, name := range names {
		result[name] = struct{}{}
	}
	return result
}

var requestJSONKeys = keys(
	"model",
	"instructions",
	"temperature",
	"input",
	"tools",
	"parallel_tool_calls",
	"background",
	"stream",
	"store",
	"service_tier",
	"safety_identifier",
	"user",
	"metadata",
	"max_output_tokens",
	"max_tool_calls",
	"text",
	"include",
	"previous_response_id",
	"prompt_cache_key",
	"prompt_cache_retention",
	"reasoning",
	"stream_options",
	"tool_choice",
	"truncation",
	"top_logprobs",
	"top_p",
)

var toolJSONKeys = keys(
	"type",
	"name",
	"description",
	"parameters",
	"strict",
	"format",
	"background",
	"input_fidelity",
	"model",
	"moderation",
	"output_compression",
	"output_format",
	"partial_images",
	"quality",
	"size",
)

var itemJSONKeys = keys(
	"id",
	"type",
	"annotations",
	"logprobs",
	"role",
	"content",
	"status",
	"image_url",
	"detail",
	"text",
	"background",
	"output_format",
	"quality",
	"size",
	"result",
	"call_id",
	"name",
	"arguments",
	"input",
	"output",
	"summary",
	"reasoning_content",
	"encrypted_content",
	"created_by",
)

var responseJSONKeys = keys(
	"object",
	"id",
	"error",
	"created_at",
	"model",
	"output",
	"status",
	"incomplete_details",
	"instructions",
	"metadata",
	"parallel_tool_calls",
	"temperature",
	"tool_choice",
	"tools",
	"top_p",
	"background",
	"conversation",
	"max_output_tokens",
	"max_tool_calls",
	"previous_response_id",
	"prompt",
	"prompt_cache_key",
	"prompt_cache_retention",
	"reasoning",
	"safety_identifier",
	"service_tier",
	"text",
	"top_logprobs",
	"truncation",
	"usage",
	"user",
)

var streamEventJSONKeys = keys(
	"type",
	"sequence_number",
	"response",
	"output_index",
	"item",
	"item_id",
	"content_index",
	"part",
	"delta",
	"text",
	"name",
	"call_id",
	"arguments",
	"input",
	"summary_index",
	"partial_image_b64",
	"partial_image_index",
	"code",
	"message",
	"param",
)
