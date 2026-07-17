package shared

import (
	"encoding/json"
	"reflect"

	"github.com/looplj/axonhub/llm"
)

// Anthropic metadata keys owned by the Anthropic adapter. Duplicated here as
// string constants so OpenAI Chat/Responses target boundaries can diagnose
// Anthropic-native losses without importing the anthropic package.
const (
	AnthropicMetadataKeyContainer    = "anthropic_container"
	AnthropicMetadataKeyInferenceGeo = "anthropic_inference_geo"
	AnthropicMetadataKeyMCPServers   = "anthropic_mcp_servers"
	AnthropicMetadataKeyRawTools     = "anthropic_raw_tools"
)

// RecordAnthropicNativeLossyDowngradesForTarget records Anthropic-only request
// controls that have no safe OpenAI Chat/Responses equivalent. Callers must not
// invent target MCP/container/document wire fields from these sources.
func RecordAnthropicNativeLossyDowngradesForTarget(llmReq *llm.Request, targetProtocol llm.APIFormat) {
	if llmReq == nil || targetProtocol == "" || targetProtocol == llm.APIFormatAnthropicMessage {
		return
	}
	if llmReq.APIFormat != llm.APIFormatAnthropicMessage {
		return
	}

	meta := llmReq.TransformerMetadata
	hasMCPServers := metadataRawPresent(meta, AnthropicMetadataKeyMCPServers)
	hasContainer := metadataRawPresent(meta, AnthropicMetadataKeyContainer)
	hasInferenceGeo := metadataRawPresent(meta, AnthropicMetadataKeyInferenceGeo)
	hasMCPToolset := anthropicRawToolsContainType(meta, "mcp_toolset")
	hasDocument := requestHasContentPartType(llmReq, "document")

	llm.AddLossyDowngradeIfPresent(llmReq, llm.APIFormatAnthropicMessage, "mcp_servers", targetProtocol, hasMCPServers)
	llm.AddLossyDowngradeIfPresent(llmReq, llm.APIFormatAnthropicMessage, "tools[].type=mcp_toolset", targetProtocol, hasMCPToolset)
	llm.AddLossyDowngradeIfPresent(llmReq, llm.APIFormatAnthropicMessage, "container", targetProtocol, hasContainer)
	llm.AddLossyDowngradeIfPresent(llmReq, llm.APIFormatAnthropicMessage, "inference_geo", targetProtocol, hasInferenceGeo)
	llm.AddLossyDowngradeIfPresent(llmReq, llm.APIFormatAnthropicMessage, "content[].type=document", targetProtocol, hasDocument)
}

func metadataRawPresent(meta map[string]any, key string) bool {
	if meta == nil {
		return false
	}
	raw, ok := meta[key]
	if !ok || raw == nil {
		return false
	}
	switch v := raw.(type) {
	case json.RawMessage:
		return len(v) > 0
	case []byte:
		return len(v) > 0
	case string:
		return v != ""
	default:
		return true
	}
}

func anthropicRawToolsContainType(meta map[string]any, toolType string) bool {
	if meta == nil || toolType == "" {
		return false
	}
	raw, ok := meta[AnthropicMetadataKeyRawTools]
	if !ok || raw == nil {
		return false
	}

	rv := reflect.ValueOf(raw)
	for rv.Kind() == reflect.Interface || rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return false
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			if fragmentHasToolType(rv.Index(i).Interface(), toolType) {
				return true
			}
		}
	default:
		return fragmentHasToolType(raw, toolType)
	}
	return false
}

func fragmentHasToolType(fragment any, toolType string) bool {
	if fragment == nil {
		return false
	}
	switch frag := fragment.(type) {
	case json.RawMessage:
		return rawToolTypeEquals(frag, toolType)
	case []byte:
		return rawToolTypeEquals(json.RawMessage(frag), toolType)
	case string:
		return rawToolTypeEquals(json.RawMessage(frag), toolType)
	}

	rv := reflect.ValueOf(fragment)
	for rv.Kind() == reflect.Interface || rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return false
		}
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.Struct {
		field := rv.FieldByName("Raw")
		if field.IsValid() {
			switch field.Kind() {
			case reflect.Slice:
				if field.Type().Elem().Kind() == reflect.Uint8 {
					return rawToolTypeEquals(json.RawMessage(field.Bytes()), toolType)
				}
			}
		}
	}
	b, err := json.Marshal(fragment)
	if err != nil {
		return false
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(b, &obj); err == nil {
		if rawField, ok := obj["Raw"]; ok {
			return rawToolTypeEquals(rawField, toolType)
		}
		if rawField, ok := obj["raw"]; ok {
			return rawToolTypeEquals(rawField, toolType)
		}
	}
	return rawToolTypeEquals(b, toolType)
}

func rawToolTypeEquals(raw json.RawMessage, toolType string) bool {
	if len(raw) == 0 {
		return false
	}
	var obj struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return false
	}
	return obj.Type == toolType
}

func requestHasContentPartType(llmReq *llm.Request, partType string) bool {
	if llmReq == nil || partType == "" {
		return false
	}
	for _, msg := range llmReq.Messages {
		for _, part := range msg.Content.MultipleContent {
			if part.Type == partType {
				return true
			}
		}
	}
	return false
}
