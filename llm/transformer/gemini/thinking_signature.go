package gemini

import (
	"encoding/base64"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

var (
	ContextEngineeringThoughtSignature = string(base64.StdEncoding.EncodeToString([]byte("context_engineering_is_the_way_to_go")))
)

const transformerMetadataKeyGoogleThoughtSignature = shared.TransformerMetadataKeyGoogleThoughtSignature

func setInboundToolCallThoughtSignature(toolCall *llm.ToolCall, signature string) {
	if toolCall == nil || signature == "" {
		return
	}

	if toolCall.TransformerMetadata == nil {
		toolCall.TransformerMetadata = map[string]any{}
	}

	toolCall.TransformerMetadata[transformerMetadataKeyGoogleThoughtSignature] = signature
}

func getInboundGeminiToolCallThoughtSignature(toolCall llm.ToolCall) *string {
	if toolCall.TransformerMetadata == nil {
		return nil
	}

	raw, ok := toolCall.TransformerMetadata[transformerMetadataKeyGoogleThoughtSignature].(string)
	if !ok || raw == "" {
		return nil
	}

	return &raw
}

func setOutboundToolCallThoughtSignature(toolCall *llm.ToolCall, signature string, scope shared.TransportScope) {
	if toolCall == nil || signature == "" {
		return
	}

	if toolCall.TransformerMetadata == nil {
		toolCall.TransformerMetadata = map[string]any{}
	}

	encoded := shared.EncodeGeminiThoughtSignatureInScope(&signature, scope)
	if encoded == nil {
		return
	}

	toolCall.TransformerMetadata[transformerMetadataKeyGoogleThoughtSignature] = *encoded
}

func getOutbountGeminiToolCallThoughtSignature(toolCall llm.ToolCall, scope shared.TransportScope) *string {
	if toolCall.TransformerMetadata == nil {
		return nil
	}

	raw, ok := toolCall.TransformerMetadata[transformerMetadataKeyGoogleThoughtSignature].(string)
	if !ok || raw == "" {
		return nil
	}

	if scope.Footprint() == "" {
		return &raw
	}

	return shared.DecodeGeminiThoughtSignatureInScope(&raw, scope)
}
