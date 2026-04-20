package cacheidentity

import (
	"context"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/anthropic/claudecode"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

// Source classifies how the cache identity was resolved.
const (
	SourceNone               = "none"
	SourceClientProvided     = "client_provided"
	SourceSessionHeader      = "session_header"
	SourceSessionBodyField   = "session_body_field"
	SourceThreadIdentity     = "thread_identity"
	SourceConversationAnchor = "conversation_anchor"
)

// TransformerMetadata keys used to propagate resolved identity through the pipeline.
const (
	MetaResolvedSessionID       = "resolved_session_id"
	MetaResolvedPromptCacheKey  = "resolved_prompt_cache_key"
	MetaResolvedCacheSource     = "resolved_prompt_cache_source"
	MetaAllowPromptCacheKeyEmit = "allow_prompt_cache_key_emit"
)

// Result holds the output of an identity resolution pass.
type Result struct {
	SessionID      string
	PromptCacheKey string
	Source         string
}

// Resolver derives stable session IDs and prompt cache keys from a
// combination of inbound headers, body fields, thread context, and
// conversation-anchor hashing.
type Resolver struct {
	config       Config
	threadHeader string // from tracing config, e.g. "AH-Thread-Id"
}

// NewResolver creates a resolver with the given configuration.
func NewResolver(config Config, threadHeader string) *Resolver {
	if threadHeader == "" {
		threadHeader = "AH-Thread-Id"
	}

	return &Resolver{
		config:       config,
		threadHeader: threadHeader,
	}
}

// Resolve runs the full resolution precedence and returns the result.
// It reads from:
//   - headers on llmReq.RawRequest
//   - body fields on llmReq.RawRequest
//   - thread context via contexts.GetThread
//   - conversation anchor from llmReq.Messages
func (r *Resolver) Resolve(ctx context.Context, llmReq *llm.Request) Result {
	if !r.config.Enabled || llmReq == nil {
		return Result{Source: SourceNone}
	}

	// 1. If client already provided a non-empty prompt_cache_key, preserve it.
	if pck := ptrString(llmReq.PromptCacheKey); pck != "" {
		return Result{
			PromptCacheKey: pck,
			Source:         SourceClientProvided,
		}
	}

	// 2. Try to resolve a stable session identity from headers/body.
	sessionID, source := r.resolveSessionIdentity(ctx, llmReq)

	// 3. If we have a session identity, use it as both session and cache key.
	if sessionID != "" {
		return Result{
			SessionID:      sessionID,
			PromptCacheKey: sessionID,
			Source:         source,
		}
	}

	// 4. Try thread identity from context.
	if thread, ok := contexts.GetThread(ctx); ok && thread != nil && thread.ThreadID != "" {
		return Result{
			SessionID:      thread.ThreadID,
			PromptCacheKey: thread.ThreadID,
			Source:         SourceThreadIdentity,
		}
	}

	// 5. Derive conversation-anchor fingerprint if enabled.
	if r.config.DeriveFromConversationAnchor && len(llmReq.Messages) > 0 {
		projectID, _ := contexts.GetProjectID(ctx)
		apiKeyID := getAPIKeyID(ctx)

		anchor := DeriveAnchor(llmReq.Messages, projectID, apiKeyID, r.config.AnchorMaxBytes)
		if anchor != "" {
			return Result{
				PromptCacheKey: anchor,
				Source:         SourceConversationAnchor,
			}
		}
	}

	return Result{Source: SourceNone}
}

// resolveSessionIdentity checks built-in and configured header/body sources.
func (r *Resolver) resolveSessionIdentity(ctx context.Context, llmReq *llm.Request) (string, string) {
	var headers http.Header
	var rawBody []byte

	if llmReq.RawRequest != nil {
		headers = llmReq.RawRequest.Headers
		rawBody = llmReq.RawRequest.Body
	}

	// --- Header-based candidates ---

	// Built-in: Codex Session_id header.
	if sid := codex.GetSessionIDFromHeaders(headers); sid != "" {
		return sid, SourceSessionHeader
	}

	// Built-in: configured thread header (e.g. AH-Thread-Id).
	if headers != nil {
		if v := strings.TrimSpace(headers.Get(r.threadHeader)); v != "" {
			return v, SourceSessionHeader
		}

		// Built-in: Conversation_id header.
		if v := strings.TrimSpace(headers.Get("Conversation_id")); v != "" {
			return v, SourceSessionHeader
		}
	}

	// Configured extra session headers.
	for _, h := range r.config.ExtraSessionHeaders {
		if headers != nil {
			if v := strings.TrimSpace(headers.Get(h)); v != "" {
				if log.DebugEnabled(ctx) {
					log.Debug(ctx, "resolved session from extra header",
						log.String("header", h),
						log.String("source", SourceSessionHeader),
					)
				}

				return v, SourceSessionHeader
			}
		}
	}

	// --- Body-based candidates ---
	if len(rawBody) > 0 {
		// Built-in body fields.
		for _, field := range []string{"metadata.session_id", "metadata.conversation_id"} {
			if v := gjson.GetBytes(rawBody, field); v.Exists() && strings.TrimSpace(v.String()) != "" {
				return strings.TrimSpace(v.String()), SourceSessionBodyField
			}
		}

		// Configured extra session body fields.
		for _, field := range r.config.ExtraSessionBodyFields {
			if v := gjson.GetBytes(rawBody, field); v.Exists() && strings.TrimSpace(v.String()) != "" {
				if log.DebugEnabled(ctx) {
					log.Debug(ctx, "resolved session from extra body field",
						log.String("field", field),
						log.String("source", SourceSessionBodyField),
					)
				}

				return strings.TrimSpace(v.String()), SourceSessionBodyField
			}
		}
	}

	// Also check Claude Code metadata.user_id for session_id if present.
	// Claude Code encodes session identity in user_id using either:
	//   - v2 JSON: {"device_id":"...","session_id":"...","account_uuid":"..."}
	//   - Legacy:  user_<64hex>_account__session_<uuid>
	// Reuse the canonical parser to handle both formats correctly.
	if len(rawBody) > 0 {
		if v := gjson.GetBytes(rawBody, "metadata.user_id"); v.Exists() {
			if uid := claudecode.ParseUserID(v.String()); uid != nil && uid.SessionID != "" {
				return uid.SessionID, SourceSessionBodyField
			}
		}
	}

	return "", ""
}

// StoreOnRequest writes resolved identity into TransformerMetadata.
func StoreOnRequest(llmReq *llm.Request, result Result) {
	if llmReq.TransformerMetadata == nil {
		llmReq.TransformerMetadata = map[string]any{}
	}

	if result.SessionID != "" {
		llmReq.TransformerMetadata[MetaResolvedSessionID] = result.SessionID
	}

	if result.PromptCacheKey != "" {
		llmReq.TransformerMetadata[MetaResolvedPromptCacheKey] = result.PromptCacheKey
	}

	llmReq.TransformerMetadata[MetaResolvedCacheSource] = result.Source
}

// GetResolvedSessionID reads the resolved session ID from TransformerMetadata.
func GetResolvedSessionID(llmReq *llm.Request) string {
	if llmReq == nil || llmReq.TransformerMetadata == nil {
		return ""
	}

	v, _ := llmReq.TransformerMetadata[MetaResolvedSessionID].(string)

	return v
}

// GetResolvedPromptCacheKey reads the resolved prompt cache key from TransformerMetadata.
func GetResolvedPromptCacheKey(llmReq *llm.Request) string {
	if llmReq == nil || llmReq.TransformerMetadata == nil {
		return ""
	}

	v, _ := llmReq.TransformerMetadata[MetaResolvedPromptCacheKey].(string)

	return v
}

// GetResolvedCacheSource reads the source classification from TransformerMetadata.
func GetResolvedCacheSource(llmReq *llm.Request) string {
	if llmReq == nil || llmReq.TransformerMetadata == nil {
		return SourceNone
	}

	v, _ := llmReq.TransformerMetadata[MetaResolvedCacheSource].(string)
	if v == "" {
		return SourceNone
	}

	return v
}

func ptrString(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

func getAPIKeyID(ctx context.Context) int {
	key, ok := contexts.GetAPIKey(ctx)
	if !ok || key == nil {
		return 0
	}

	return key.ID
}

// IsPromptCacheKeyEmitAllowed checks the TransformerMetadata flag.
func IsPromptCacheKeyEmitAllowed(meta map[string]any) bool {
	if meta == nil {
		return false
	}

	v, ok := meta[MetaAllowPromptCacheKeyEmit].(bool)

	return ok && v
}

// SetPromptCacheKeyEmitAllowed sets the TransformerMetadata flag.
func SetPromptCacheKeyEmitAllowed(llmReq *llm.Request, allowed bool) {
	if llmReq.TransformerMetadata == nil {
		llmReq.TransformerMetadata = map[string]any{}
	}

	llmReq.TransformerMetadata[MetaAllowPromptCacheKeyEmit] = allowed
}

// InjectSessionIDToContext stores the resolved session ID into the context
// via shared.WithSessionID if not already present.
func InjectSessionIDToContext(ctx context.Context, result Result) context.Context {
	if result.SessionID == "" {
		return ctx
	}

	// Don't overwrite if the trace middleware already set a session-scoped value.
	if _, ok := shared.GetSessionID(ctx); ok {
		return ctx
	}

	return shared.WithSessionID(ctx, result.SessionID)
}
