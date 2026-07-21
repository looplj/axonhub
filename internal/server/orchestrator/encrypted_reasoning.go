package orchestrator

import (
	"errors"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

const (
	invalidEncryptedContentCode  = "invalid_encrypted_content"
	thinkingSignatureInvalidCode = "thinking_signature_invalid"
)

func isEncryptedReasoningProviderError(err error) bool {
	if err == nil {
		return false
	}

	var responseErr *llm.ResponseError
	if errors.As(err, &responseErr) {
		return isEncryptedReasoningFailure(responseErr.Detail.Code, responseErr.Detail.Type, responseErr.Detail.Message)
	}

	var httpErr *httpclient.Error
	if errors.As(err, &httpErr) {
		return isEncryptedReasoningFailure("", "", string(httpErr.Body))
	}

	return isEncryptedReasoningFailure("", "", err.Error())
}

func isEncryptedReasoningFailure(code, errorType, message string) bool {
	code = strings.ToLower(strings.TrimSpace(code))
	errorType = strings.ToLower(strings.TrimSpace(errorType))
	message = strings.ToLower(strings.TrimSpace(message))

	switch code {
	case invalidEncryptedContentCode, thinkingSignatureInvalidCode:
		return true
	}
	switch errorType {
	case invalidEncryptedContentCode, thinkingSignatureInvalidCode:
		return true
	}

	if strings.Contains(message, invalidEncryptedContentCode) || strings.Contains(message, thinkingSignatureInvalidCode) {
		return true
	}

	if !strings.Contains(message, "encrypted content") {
		return false
	}

	return strings.Contains(message, "could not be verified") ||
		strings.Contains(message, "could not be decrypted") ||
		strings.Contains(message, "could not be parsed") ||
		strings.Contains(message, "item_id did not match")
}

func (p *PersistentOutboundTransformer) dropOpaqueReasoningState() bool {
	if p == nil || p.state == nil {
		return false
	}

	if !stripOpaqueReasoningState(p.state.LlmRequest) {
		return false
	}

	p.state.OpaqueReasoningStateDropped = true
	p.state.PassThroughApplied = false
	p.state.RawProviderResponse = nil

	return true
}

func hasOpaqueReasoningState(request *llm.Request) bool {
	if request == nil {
		return false
	}

	return messagesHaveOpaqueReasoningState(request.Messages) ||
		(request.Compact != nil && messagesHaveOpaqueReasoningState(request.Compact.Input)) ||
		responsesRawInputHasOpaqueReasoningState(request)
}

func stripOpaqueReasoningState(request *llm.Request) bool {
	if request == nil {
		return false
	}

	changed := stripOpaqueReasoningFromMessages(request.Messages)
	if request.Compact != nil {
		changed = stripOpaqueReasoningFromMessages(request.Compact.Input) || changed
	}

	return stripOpaqueResponsesRawInputItems(request) || changed
}

func messagesHaveOpaqueReasoningState(messages []llm.Message) bool {
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

func stripOpaqueReasoningFromMessages(messages []llm.Message) bool {
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

		parts := make([]llm.MessageContentPart, 0, len(message.Content.MultipleContent))
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

func isOpaqueReasoningContentPart(part llm.MessageContentPart) bool {
	return part.Type == "compaction" || part.Type == "compaction_summary"
}

func responsesRawInputHasOpaqueReasoningState(request *llm.Request) bool {
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

func stripOpaqueResponsesRawInputItems(request *llm.Request) bool {
	if request == nil || request.ProviderExtensions == nil || request.ProviderExtensions.OpenAIResponses == nil ||
		request.ProviderExtensions.OpenAIResponses.Request == nil {
		return false
	}

	requestExt := request.ProviderExtensions.OpenAIResponses.Request
	fragments := requestExt.RawInputItems
	if len(fragments) == 0 {
		return false
	}

	kept := make([]llm.OpenAIResponsesRawFragment, 0, len(fragments))
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
