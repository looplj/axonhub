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

	RequestService      *biz.RequestService
	UsageLogService     *biz.UsageLogService
	SystemService       *biz.SystemService
	ChannelService      *biz.ChannelService
	PromptProvider      PromptProvider
	PromptProtecter     PromptProtecter
	RetryPolicyProvider RetryPolicyProvider
	CandidateSelector   CandidateSelector
	LoadBalancers       map[string]*LoadBalancer
	RoutingPolicy       EffectiveRoutingPolicy

	// Request state
	ModelMapper *ModelMapper
	// Proxy config, will be used to override channel's default proxy config.
	Proxy *httpclient.ProxyConfig

	// OriginalModel is the model after API key profile mapping, used for channel selection
	OriginalModel string
	RawRequest    *httpclient.Request
	LlmRequest    *llm.Request

	// OriginalRequestStream stores the client's original stream intent before any
	// candidate-specific forcing to provider-side streaming happens.
	OriginalRequestStream *bool

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

	// OutboundStreamTerminal preserves the provider outcome across protocol
	// conversion, which may replace an abnormal terminal event with a generic stop.
	// Request/execution status and channel health intentionally interpret this
	// outcome differently, as described by StreamCompleted and Perf above.
	OutboundStreamTerminal streamTerminalState

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

	// PassThroughApplied records whether the inbound request body was substituted during pass-through.
	PassThroughApplied bool
}
