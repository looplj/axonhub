// 职责: 把流式上游响应中聚合阶段会丢弃的元数据帧（provider、计费、native_finish_reason、非标准事件）保留到执行记录响应体的 _upstream_chunks 字段。
// 关联文件:
//
//	internal/server/biz/request.go
//	internal/server/biz/system.go
//	llm/httpclient/model.go
//
// 遵循的 Rules:
//
//	.agents/rules/go-general.md
//
// 注意事项:
//
//	只保留不含回答正文的帧；首帧即使带正文也会剥离 content/reasoning 后再保留。
//	仅在 store_upstream_meta 开启且 store_chunks 关闭时生效，避免同一份数据重复存储。
package biz

import (
	"bytes"
	"encoding/json"

	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm/httpclient"
)

const (
	// upstreamMetaChunksKey is the reserved key inside request_executions.response_body
	// that carries the upstream stream frames which are not represented in the
	// aggregated response (provider, usage cost, native finish reason, ...).
	upstreamMetaChunksKey = "_upstream_chunks"

	// maxUpstreamMetaBytes bounds the embedded frames to keep a single row sane.
	maxUpstreamMetaBytes = 1 << 20
)

// embedUpstreamMetaChunks keeps the upstream stream frames that are not part of
// the aggregated answer (provider, billing, native finish reason, non-standard
// events) inside the response body. Frames that only carry incremental answer
// text are skipped because the aggregated body already contains them.
func embedUpstreamMetaChunks(body []byte, chunks []*httpclient.StreamEvent) []byte {
	if len(body) == 0 || len(chunks) == 0 {
		return body
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil || obj == nil {
		return body
	}

	frames := make([]json.RawMessage, 0, 8)
	total := 0

	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}

		data := bytes.TrimSpace(chunk.Data)
		if len(data) == 0 || !json.Valid(data) {
			continue
		}

		// Always keep the first valid frame: it carries the upstream envelope
		// (id/model/provider) even when some providers also stream content there.
		// Strip the answer text from it so the aggregated body stays the only
		// copy of the answer.
		if chunkCarriesAnswerText(data) {
			if len(frames) > 0 {
				continue
			}

			slim, ok := stripAnswerText(data)
			if !ok {
				continue
			}

			data = slim
		}

		if total+len(data) > maxUpstreamMetaBytes {
			break
		}

		frames = append(frames, json.RawMessage(data))
		total += len(data)
	}

	if len(frames) == 0 {
		return body
	}

	encoded, err := json.Marshal(frames)
	if err != nil {
		return body
	}
	obj[upstreamMetaChunksKey] = encoded

	out, err := json.Marshal(obj)
	if err != nil {
		return body
	}

	return out
}

// chunkCarriesAnswerText reports whether a stream frame carries incremental
// answer text that the aggregator already merged into the final body.
func chunkCarriesAnswerText(data []byte) bool {
	choices := gjson.GetBytes(data, "choices")
	if !choices.IsArray() {
		return false
	}

	for _, choice := range choices.Array() {
		delta := choice.Get("delta")
		if !delta.Exists() {
			continue
		}

		for _, key := range []string{"content", "reasoning", "reasoning_content"} {
			if hasNonEmptyJSON(delta.Get(key)) {
				return true
			}
		}

		if audio := delta.Get("audio"); audio.Exists() {
			for _, key := range []string{"data", "transcript"} {
				if hasNonEmptyJSON(audio.Get(key)) {
					return true
				}
			}
		}

		for _, toolCall := range delta.Get("tool_calls").Array() {
			if hasNonEmptyJSON(toolCall.Get("function.arguments")) {
				return true
			}
		}
	}

	return false
}

// stripAnswerText removes the incremental answer fields from a single frame
// while keeping the rest of the upstream envelope (id/model/provider/choices).
func stripAnswerText(data []byte) ([]byte, bool) {
	var frame map[string]json.RawMessage
	if err := json.Unmarshal(data, &frame); err != nil {
		return nil, false
	}

	choicesRaw, ok := frame["choices"]
	if !ok {
		return data, true
	}

	var choices []map[string]json.RawMessage
	if err := json.Unmarshal(choicesRaw, &choices); err != nil {
		return nil, false
	}

	for _, choice := range choices {
		deltaRaw, ok := choice["delta"]
		if !ok {
			continue
		}

		var delta map[string]json.RawMessage
		if err := json.Unmarshal(deltaRaw, &delta); err != nil {
			return nil, false
		}

		for _, key := range []string{"content", "reasoning", "reasoning_content", "tool_calls", "audio"} {
			delete(delta, key)
		}

		encoded, err := json.Marshal(delta)
		if err != nil {
			return nil, false
		}

		choice["delta"] = encoded
	}

	encodedChoices, err := json.Marshal(choices)
	if err != nil {
		return nil, false
	}

	frame["choices"] = encodedChoices

	out, err := json.Marshal(frame)
	if err != nil {
		return nil, false
	}

	return out, true
}

func hasNonEmptyJSON(value gjson.Result) bool {
	if !value.Exists() {
		return false
	}

	switch value.Raw {
	case "", `""`, "null":
		return false
	default:
		return true
	}
}
