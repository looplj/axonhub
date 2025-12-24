package orchestrator

import (
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/server/biz"
)

// PersistenceState holds shared state with channel management and retry capabilities.
type PersistenceState struct {
	APIKey *ent.APIKey
	User   *ent.User

	RequestService      *biz.RequestService
	UsageLogService     *biz.UsageLogService
	ChannelService      *biz.ChannelService
	RetryPolicyProvider RetryPolicyProvider
	CandidateSelector   CandidateSelector
	LoadBalancer        *LoadBalancer

	// Request state
	ModelMapper *ModelMapper
	// Proxy config, will be used to override channel's default proxy config.
	Proxy         *objects.ProxyConfig
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
