package llm

import "github.com/samber/lo"

// HasOpaqueReasoningState reports whether the request carries issuer-bound
// opaque reasoning (signatures, Responses reasoning item ids, compaction parts,
// or Responses raw input reasoning/compaction items).
func HasOpaqueReasoningState(request *Request) bool {
	if request == nil {
		return false
	}

	return messagesHaveOpaqueReasoningState(request.Messages) ||
		(request.Compact != nil && messagesHaveOpaqueReasoningState(request.Compact.Input)) ||
		responsesRawInputHasOpaqueReasoningState(request)
}

// StripOpaqueReasoningState removes issuer-bound opaque reasoning fields from
// messages, compact input, and Responses PE raw input items. Visible reasoning
// summary text is retained; Responses-native presence markers may remain as
// empty ResponseReasoningItemID when summary text is present.
func StripOpaqueReasoningState(request *Request) bool {
	if request == nil {
		return false
	}

	changed := stripOpaqueReasoningFromMessages(request.Messages)
	if request.Compact != nil {
		changed = stripOpaqueReasoningFromMessages(request.Compact.Input) || changed
	}

	return stripOpaqueResponsesRawInputItems(request) || changed
}

func messagesHaveOpaqueReasoningState(messages []Message) bool {
	for _, message := range messages {
		if (message.ReasoningSignature != nil && *message.ReasoningSignature != "") ||
			(message.ResponseReasoningItemID != nil && *message.ResponseReasoningItemID != "") {
			return true
		}
		for _, part := range message.Content.MultipleContent {
			if isOpaqueReasoningContentPart(part) {
				return true
			}
		}
	}

	return false
}

func stripOpaqueReasoningFromMessages(messages []Message) bool {
	changed := false
	for index := range messages {
		message := &messages[index]
		hasOpaqueReasoningID := message.ResponseReasoningItemID != nil && *message.ResponseReasoningItemID != ""
		if message.ReasoningSignature != nil {
			message.ReasoningSignature = nil
			changed = true
		}
		if hasOpaqueReasoningID {
			changed = true
		}

		// Keep visible reasoning summary text. A Responses-native summary must retain
		// its presence marker while omitting both id and encrypted_content. Signatures
		// from Anthropic/Gemini do not establish Responses provenance and therefore
		// must not invent a Responses reasoning item.
		if hasOpaqueReasoningID {
			if message.ReasoningContent != nil && *message.ReasoningContent != "" {
				message.ResponseReasoningItemID = lo.ToPtr("")
			} else {
				message.ResponseReasoningItemID = nil
			}
		}

		if len(message.Content.MultipleContent) == 0 {
			continue
		}

		parts := make([]MessageContentPart, 0, len(message.Content.MultipleContent))
		for _, part := range message.Content.MultipleContent {
			if isOpaqueReasoningContentPart(part) {
				changed = true
				continue
			}
			parts = append(parts, part)
		}
		if len(parts) != len(message.Content.MultipleContent) {
			message.Content.MultipleContent = parts
		}
	}

	return changed
}

func isOpaqueReasoningContentPart(part MessageContentPart) bool {
	return part.Type == "compaction" || part.Type == "compaction_summary"
}

func responsesRawInputHasOpaqueReasoningState(request *Request) bool {
	if request == nil || request.ProviderExtensions == nil || request.ProviderExtensions.OpenAIResponses == nil ||
		request.ProviderExtensions.OpenAIResponses.Request == nil {
		return false
	}

	for _, fragment := range request.ProviderExtensions.OpenAIResponses.Request.RawInputItems {
		if isOpaqueResponsesInputItemType(fragment.Type) {
			return true
		}
	}

	return false
}

func stripOpaqueResponsesRawInputItems(request *Request) bool {
	if request == nil || request.ProviderExtensions == nil || request.ProviderExtensions.OpenAIResponses == nil ||
		request.ProviderExtensions.OpenAIResponses.Request == nil {
		return false
	}

	requestExt := request.ProviderExtensions.OpenAIResponses.Request
	fragments := requestExt.RawInputItems
	if len(fragments) == 0 {
		return false
	}

	kept := make([]OpenAIResponsesRawFragment, 0, len(fragments))
	for _, fragment := range fragments {
		if isOpaqueResponsesInputItemType(fragment.Type) {
			continue
		}
		kept = append(kept, fragment)
	}
	if len(kept) == len(fragments) {
		return false
	}

	requestExt.RawInputItems = kept
	return true
}

func isOpaqueResponsesInputItemType(itemType string) bool {
	switch itemType {
	case "reasoning", "compaction", "compaction_summary":
		return true
	default:
		return false
	}
}
