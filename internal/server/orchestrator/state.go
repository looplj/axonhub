package orchestrator

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// PersistenceState holds shared state with channel management and retry capabilities.
// TODO: move the dependencies out of the state to make it a real state.
type PersistenceState struct {
	APIKey *ent.APIKey

	RequestService                *biz.RequestService
	UsageLogService               *biz.UsageLogService
	ChannelService                *biz.ChannelService
	PromptProvider                PromptProvider
	PromptProtecter               PromptProtecter
	RetryPolicyProvider           RetryPolicyProvider
	CompatibilitySettingsProvider LLMCompatibilitySettingsProvider
	CandidateSelector             CandidateSelector
	LoadBalancer                  *LoadBalancer

	// Request state
	ModelMapper *ModelMapper
	// Proxy config, will be used to override channel's default proxy config.
	Proxy *httpclient.ProxyConfig

	// OriginalModel is the model after API key profile mapping, used for channel selection
	OriginalModel string
	RawRequest    *httpclient.Request

	// InboundParsedRequest is the first semantic request emitted by the inbound transformer.
	InboundParsedRequest *llm.Request
	// EffectiveSemanticRequest is the semantic baseline after inbound middlewares such as prompt injection/protection.
	EffectiveSemanticRequest *llm.Request
	// CurrentAttemptRequest is the mutable deep copy used by the current outbound attempt.
	CurrentAttemptRequest *llm.Request
	// CurrentAttemptRawProviderRequest is the provider request emitted by the current outbound attempt.
	CurrentAttemptRawProviderRequest *httpclient.Request
	// CurrentAttemptSanitizeResult records Responses-only data cleanup/rejection for the current attempt.
	CurrentAttemptSanitizeResult AttemptSanitizeResult
	// CurrentAttemptRequestBodyPassThroughEnabled records the final request-body pass-through decision.
	CurrentAttemptRequestBodyPassThroughEnabled bool
	// CurrentAttemptTransformOptionsChanged is true when channel transform options changed the emitted request shape.
	CurrentAttemptTransformOptionsChanged bool

	// RequestDirty records semantic fragments changed after inbound parsing.
	RequestDirty *RequestDirtySet
	// PromptProtection records prompt protection outcomes relevant to replay gates.
	PromptProtection PromptProtectionState

	// LlmRequest is a compatibility alias for EffectiveSemanticRequest.
	LlmRequest *llm.Request

	// Persistence state
	Request     *ent.Request
	RequestExec *ent.RequestExecution

	// ChannelModelsCandidates is the primary state for channel selection
	ChannelModelsCandidates []*ChannelModelsCandidate
	// Candidate state - current candidate index of ChannelModelsCandidates
	CurrentCandidateIndex int
	// CurrentCandidate is the currently selected candidate of ChannelModelsCandidates
	CurrentCandidate *ChannelModelsCandidate
	// CurrentModelIndex is the current model index in CurrentCandidate.Models
	CurrentModelIndex int

	// Perf is the performance record for the current request.
	Perf *biz.PerformanceRecord

	// StreamCompleted tracks whether the stream has response successfully completed.
	// This is used to distinguish between a stream that was canceled mid-way
	// versus a stream that completed successfully but the client disconnected
	// immediately after receiving the last chunk.
	StreamCompleted bool

	// RawProviderResponse stores the raw provider response for non-stream response pass-through.
	RawProviderResponse *httpclient.Response

	// RawProviderRequest stores the actual outbound provider request for pass-through checks.
	RawProviderRequest *httpclient.Request

	// RawStreamCh receives raw provider stream events for stream response pass-through.
	RawStreamCh chan *httpclient.StreamEvent

	// RawStreamErrRef points to the current attempt's local error variable used by the
	// captureRawProviderStream fan-out goroutine. Using a per-attempt pointer (instead of
	// a single shared field) prevents data races when retries spawn a new goroutine before
	// the previous one has exited.
	RawStreamErrRef *error

	// RawStreamCancel cancels the current attempt's fan-out goroutine started by
	// captureRawProviderStream. Must be called in PrepareForRetry and NextChannel so the
	// abandoned goroutine exits promptly and releases its upstream HTTP connection.
	RawStreamCancel context.CancelFunc
}

type AttemptSanitizeResult struct {
	Changed                         bool
	Rejected                        bool
	Reason                          string
	Policy                          biz.ResponsesOnlyDataPolicy
	OutboundAPIFormat               string
	DroppedSafeExtraKeysCount       int
	DroppedSemanticExtraKeysCount   int
	DroppedMetadataExtraKeysCount   int
	DroppedRawInputItemsCount       int
	DroppedExecutableToolsCount     int
	DroppedCustomToolMessagesCount  int
	DroppedTransformerMetadataCount int
	RejectedCategories              []string
}

type RequestDirtyScope string

const (
	RequestDirtyMessages              RequestDirtyScope = "messages"
	RequestDirtyInstructions          RequestDirtyScope = "instructions"
	RequestDirtyInputItems            RequestDirtyScope = "input_items"
	RequestDirtyTools                 RequestDirtyScope = "tools"
	RequestDirtyToolChoice            RequestDirtyScope = "tool_choice"
	RequestDirtyMetadata              RequestDirtyScope = "metadata"
	RequestDirtyTopLevelSemanticExtra RequestDirtyScope = "top_level_semantic_extra"
)

type RequestDirtySet struct {
	scopes map[RequestDirtyScope]struct{}
}

func NewRequestDirtySet() *RequestDirtySet {
	return &RequestDirtySet{
		scopes: make(map[RequestDirtyScope]struct{}),
	}
}

func (d *RequestDirtySet) Mark(scopes ...RequestDirtyScope) {
	if d == nil {
		return
	}

	if d.scopes == nil {
		d.scopes = make(map[RequestDirtyScope]struct{})
	}

	for _, scope := range scopes {
		d.scopes[scope] = struct{}{}
	}
}

func (d *RequestDirtySet) Has(scope RequestDirtyScope) bool {
	if d == nil || d.scopes == nil {
		return false
	}

	_, ok := d.scopes[scope]
	return ok
}

func (d *RequestDirtySet) HasAny(scopes ...RequestDirtyScope) bool {
	for _, scope := range scopes {
		if d.Has(scope) {
			return true
		}
	}

	return false
}

func (d *RequestDirtySet) Scopes() []RequestDirtyScope {
	if d == nil || len(d.scopes) == 0 {
		return nil
	}

	scopes := make([]RequestDirtyScope, 0, len(d.scopes))
	for scope := range d.scopes {
		scopes = append(scopes, scope)
	}

	return scopes
}

type PromptProtectionFragmentStatus string

const (
	PromptProtectionFragmentUnscanned                  PromptProtectionFragmentStatus = "unscanned"
	PromptProtectionFragmentScannedClean               PromptProtectionFragmentStatus = "scanned_clean"
	PromptProtectionFragmentMatchedUnchanged           PromptProtectionFragmentStatus = "matched_unchanged"
	PromptProtectionFragmentMatchedChangedReplayable   PromptProtectionFragmentStatus = "matched_changed_replayable"
	PromptProtectionFragmentMatchedChangedUnreplayable PromptProtectionFragmentStatus = "matched_changed_unreplayable"
	PromptProtectionFragmentRejected                   PromptProtectionFragmentStatus = "rejected"
)

type PromptProtectionFragmentResult struct {
	Scope  string
	Status PromptProtectionFragmentStatus
}

type PromptProtectionState struct {
	Changed   bool
	Rejected  bool
	Fragments []PromptProtectionFragmentResult
}

func (p PromptProtectionState) HasUnsafeFragmentsForReplay() bool {
	for _, fragment := range p.Fragments {
		switch fragment.Status {
		case PromptProtectionFragmentUnscanned,
			PromptProtectionFragmentMatchedChangedUnreplayable,
			PromptProtectionFragmentRejected:
			return true
		}
	}

	return false
}

func (s *PersistenceState) MarkDirty(scopes ...RequestDirtyScope) {
	if s == nil {
		return
	}

	if s.RequestDirty == nil {
		s.RequestDirty = NewRequestDirtySet()
	}

	s.RequestDirty.Mark(scopes...)
}

func (s *PersistenceState) SetEffectiveSemanticRequest(req *llm.Request) {
	if s == nil {
		return
	}

	s.EffectiveSemanticRequest = req
	s.LlmRequest = req
}

func (s *PersistenceState) effectiveSemanticRequest() *llm.Request {
	if s == nil {
		return nil
	}

	if s.EffectiveSemanticRequest != nil {
		return s.EffectiveSemanticRequest
	}

	return s.LlmRequest
}

func (s *PersistenceState) currentAttemptRawProviderRequest() *httpclient.Request {
	if s == nil {
		return nil
	}

	if s.CurrentAttemptRawProviderRequest != nil {
		return s.CurrentAttemptRawProviderRequest
	}

	return s.RawProviderRequest
}

func (s *PersistenceState) currentAttemptPassThroughBodyApplied() bool {
	return s != nil && s.CurrentAttemptRequestBodyPassThroughEnabled
}

func (s *PersistenceState) resetCurrentAttemptState() {
	if s == nil {
		return
	}

	s.CurrentAttemptRequest = nil
	s.CurrentAttemptRawProviderRequest = nil
	s.CurrentAttemptSanitizeResult = AttemptSanitizeResult{}
	s.CurrentAttemptRequestBodyPassThroughEnabled = false
	s.CurrentAttemptTransformOptionsChanged = false
	s.RawProviderRequest = nil
	s.RawProviderResponse = nil
}
