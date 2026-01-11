package orchestrator

import (
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
	ChannelService      *biz.ChannelService
	PromptProvider      PromptProvider
	RetryPolicyProvider RetryPolicyProvider
	CandidateSelector   CandidateSelector
	LoadBalancer        *LoadBalancer

	// Request state
	ModelMapper *ModelMapper
	// Proxy config, will be used to override channel's default proxy config.
	Proxy *httpclient.ProxyConfig

	// OriginalModel is the model after API key profile mapping, used for channel selection
	OriginalModel string
	RawRequest    *httpclient.Request
	LlmRequest    *llm.Request

	// Persistence state
	Request     *ent.Request
	RequestExec *ent.RequestExecution

	// ChannelModelCandidates is the primary state for channel selection
	ChannelModelCandidates []*ChannelModelCandidate

	// Candidate state - current candidate index of ChannelModelCandidates
	CandidateIndex int

	// CurrentCandidate is the currently selected candidate (includes channel and model info)
	CurrentCandidate *ChannelModelCandidate

	// CurrentChannel is kept for backward compatibility with code that uses it directly,
	// It should equal to CurrentCandidate.Channel
	CurrentChannel *biz.Channel

	Perf *biz.PerformanceRecord
}
