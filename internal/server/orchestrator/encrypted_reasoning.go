package orchestrator

import (
	"errors"
	"strings"

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

// dropOpaqueReasoningState applies recovery policy: strip issuer-bound opaque
// reasoning via llm.StripOpaqueReasoningState and disable pass-through replay
// of the pre-strip raw body for this attempt.
func (p *PersistentOutboundTransformer) dropOpaqueReasoningState() bool {
	if p == nil || p.state == nil {
		return false
	}

	if !llm.StripOpaqueReasoningState(p.state.LlmRequest) {
		return false
	}

	p.state.OpaqueReasoningStateDropped = true
	p.state.PassThroughApplied = false
	p.state.RawProviderResponse = nil

	return true
}

func hasOpaqueReasoningState(request *llm.Request) bool {
	return llm.HasOpaqueReasoningState(request)
}

func stripOpaqueReasoningState(request *llm.Request) bool {
	return llm.StripOpaqueReasoningState(request)
}
