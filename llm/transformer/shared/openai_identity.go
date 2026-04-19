package shared

import (
	"context"
	"strings"

	"github.com/looplj/axonhub/llm"
)

const (
	TransformerMetadataKeyOpenAIIdentityOwner = "openai_identity_owner"
)

type OpenAIIdentity struct {
	PromptCacheKey   *string
	SafetyIdentifier *string
	User             *string
	SessionID        *string
}

func DeriveOpenAIIdentity(ctx context.Context, req *llm.Request) OpenAIIdentity {
	if req == nil {
		return OpenAIIdentity{}
	}

	sessionID := normalizedSessionID(ctx, req)
	derivedCallerIdentity := normalizedStringPtr(req.User)
	if derivedCallerIdentity == nil {
		derivedCallerIdentity = metadataStringPtr(req.Metadata, "user_id")
	}
	if derivedCallerIdentity == nil && sessionID != nil {
		derivedCallerIdentity = cloneStringPtr(sessionID)
	}
	if derivedCallerIdentity == nil {
		derivedCallerIdentity = transformerMetadataStringPtr(req.TransformerMetadata, TransformerMetadataKeyOpenAIIdentityOwner)
	}

	promptCacheKey := normalizedStringPtr(req.PromptCacheKey)
	if promptCacheKey == nil {
		promptCacheKey = cloneStringPtr(sessionID)
	}

	safetyIdentifier := normalizedStringPtr(req.SafetyIdentifier)
	if safetyIdentifier == nil {
		safetyIdentifier = cloneStringPtr(derivedCallerIdentity)
	}

	user := normalizedStringPtr(req.User)
	if user == nil {
		user = cloneStringPtr(derivedCallerIdentity)
	}

	return OpenAIIdentity{
		PromptCacheKey:   promptCacheKey,
		SafetyIdentifier: safetyIdentifier,
		User:             user,
		SessionID:        sessionID,
	}
}

func normalizedSessionID(ctx context.Context, req *llm.Request) *string {
	if req != nil && req.RawRequest != nil && req.RawRequest.Headers != nil {
		if sessionID := strings.TrimSpace(req.RawRequest.Headers.Get("Session_id")); sessionID != "" {
			return &sessionID
		}
	}

	if req != nil {
		if sessionID := strings.TrimSpace(req.Metadata["session_id"]); sessionID != "" {
			return &sessionID
		}
	}

	if sessionID, ok := GetSessionID(ctx); ok {
		sessionID = strings.TrimSpace(sessionID)
		if sessionID != "" {
			return &sessionID
		}
	}

	return nil
}

func metadataStringPtr(metadata map[string]string, key string) *string {
	if metadata == nil {
		return nil
	}

	value := strings.TrimSpace(metadata[key])
	if value == "" {
		return nil
	}

	return &value
}

func transformerMetadataStringPtr(metadata map[string]any, key string) *string {
	if metadata == nil {
		return nil
	}

	value, ok := metadata[key].(string)
	if !ok {
		return nil
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	return &value
}

func normalizedStringPtr(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}

	cloned := *value
	return &cloned
}
