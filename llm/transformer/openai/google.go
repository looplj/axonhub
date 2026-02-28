package openai

import "github.com/looplj/axonhub/llm/transformer/shared"

// TransformerMetadataKeyGoogleThoughtSignature 用于在 ToolCall TransformerMetadata 中保存 Gemini thought signature。
const TransformerMetadataKeyGoogleThoughtSignature = shared.TransformerMetadataKeyGoogleThoughtSignature

// ToolCallGoogleExtraContent represents Google-specific extension fields for tool calls.
type ToolCallGoogleExtraContent struct {
	ThoughtSignature string `json:"thought_signature,omitempty"`
}

// stripUnsupportedToolCallExtraContent strips unsupported extra content fields from tool calls.
func stripUnsupportedToolCallExtraContent(req *Request) {
	if req == nil {
		return
	}

	for i := range req.Messages {
		for j := range req.Messages[i].ToolCalls {
			tc := req.Messages[i].ToolCalls[j]

			var changed bool
			if tc.ExtraContent != nil {
				// Keep empty google extra content (does not serialize thought_signature) for compatibility,
				// but strip when it carries an actual thought_signature value.
				if tc.ExtraContent.Google == nil || tc.ExtraContent.Google.ThoughtSignature != "" {
					tc.ExtraContent = nil
					changed = true
				}
			}

			if tc.ExtraFields != nil {
				if tc.ExtraFields.ExtraContent == nil ||
					tc.ExtraFields.ExtraContent.Google == nil ||
					tc.ExtraFields.ExtraContent.Google.ThoughtSignature != "" {
					tc.ExtraFields = nil
					changed = true
				}
			}

			if changed {
				req.Messages[i].ToolCalls[j] = tc
			}
		}
	}
}
