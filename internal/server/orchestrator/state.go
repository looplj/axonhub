package orchestrator

import (
	"sync"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/events"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// PersistenceState holds shared state with channel management and retry capabilities.
// TODO: move the dependencies out of the state to make it a real state.
type PersistenceState struct {
	APIKey *ent.APIKey

	RequestService      *biz.RequestService
	UsageLogService     *biz.UsageLogService
	ChannelService      *biz.ChannelService
	PromptProvider      PromptProvider
	PromptProtecter     PromptProtecter
	CandidateSelector   CandidateSelector
	LoadBalancers       map[string]*LoadBalancer
	RoutingPolicy       EffectiveRoutingPolicy
	// Settings is the system settings snapshot taken when the request started.
	Settings *RequestSettings

	// Request state
	ModelMapper *ModelMapper
	// Proxy config, will be used to override channel's default proxy config.
	Proxy *httpclient.ProxyConfig

	// OriginalModel is the model after API key profile mapping, used for channel selection
	OriginalModel         string
	RawRequest            *httpclient.Request
	LlmRequest            *llm.Request
	ResponsesContinuation *responsesContinuation

	// OriginalRequestStream stores the client's original stream intent before any
	// candidate-specific forcing to provider-side streaming happens.
	OriginalRequestStream *bool

	// PromptProtectionMaskRules records the mask rules that changed this request.
	// Request-body pass-through uses it to patch the original JSON without dropping
	// provider-specific fields that are not represented by the unified request.
	PromptProtectionMaskRules []*ent.PromptProtectionRule

	// PromptProtectionBodyCheck verifies raw replay against the protected prompt
	// snapshot using the actual inbound mapping, including legacy protectors.
	PromptProtectionBodyCheck *promptProtectionBodyCheck

	// Persistence state
	Request     *ent.Request
	RequestExec *ent.RequestExecution
	// ExecutionHeaders holds the upstream response headers of the latest execution
	// until that execution's terminal update writes them.
	ExecutionHeaders *executionHeaderObserver

	// ChannelModelsCandidates is the primary state for channel selection
	ChannelModelsCandidates []*ChannelModelsCandidate
	// Candidate state - current candidate index of ChannelModelsCandidates
	CurrentCandidateIndex int
	// CurrentCandidate is the currently selected candidate of ChannelModelsCandidates
	CurrentCandidate *ChannelModelsCandidate
	// CurrentModelIndex is the current model index in CurrentCandidate.Models
	CurrentModelIndex int

	// Perf records channel health and latency for the current request. For stream
	// terminals, incomplete (token limit or content filtering) counts as channel
	// success: it must not trigger channel failure counts or auto-disable rules.
	Perf *biz.PerformanceRecord

	// StreamCompleted tracks a successful generation outcome, not channel health.
	// A recognized incomplete terminal keeps this false and is persisted as Failed
	// on both request and execution, even though Perf.Success is true. This does
	// not turn a valid protocol response (such as finish_reason=length) into an
	// HTTP/stream error for the client.
	StreamCompleted bool

	// DeliveryAcknowledged is set only after the downstream semantic boundary is
	// written successfully. AccountingFinal is independent from delivery and
	// usage values; downstream finalization owns when to publish either state.
	DeliveryAcknowledged bool
	AccountingFinal      bool
	AccountingSourceEnd  llm.SourceEndReason

	// OutboundStreamTerminal preserves the provider outcome across protocol
	// conversion, which may replace an abnormal terminal event with a generic stop.
	// Request/execution status and channel health intentionally interpret this
	// outcome differently, as described by StreamCompleted and Perf above.
	OutboundStreamTerminal streamTerminalState
	// CleanEOFCompletionEvidence is set only by a protocol transformer after it
	// has exhausted a clean source and emitted validated terminal evidence.
	CleanEOFCompletionEvidence bool

	// RawProviderResponse stores the raw provider response for non-stream response pass-through.
	RawProviderResponse *httpclient.Response

	// RawProviderRequest stores the actual outbound provider request for pass-through checks.
	RawProviderRequest *httpclient.Request

	// PassThroughRecorder retains the current attempt's raw provider stream events for
	// stream response pass-through. It is replaced by every attempt that records.
	PassThroughRecorder *rawEventRecorder

	// PassThroughApplied records whether the inbound request body was substituted during pass-through.
	PassThroughApplied bool

	// PluginOverloaded is set when an onAttempt hook cannot borrow a runtime.
	// HasMoreChannels returns false while it is set, so the pipeline does not switch channels.
	PluginOverloaded bool

	// EventBus publishes plugin events for this request. Nil skips publication.
	// It is copied from the orchestrator; it is not stored on context.Context.
	EventBus *events.Bus

	// OutboundAttempt is the 1-based count of outbound attempts started for this
	// request, including same-channel retries. Zero means no attempt has started.
	OutboundAttempt int

	requestFailedEvent sync.Once
}
