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

	RequestService  *biz.RequestService
	UsageLogService *biz.UsageLogService
	ChannelService  *biz.ChannelService
	ChannelSelector ChannelSelector
	LoadBalancer    *LoadBalancer

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

	// Channel state
	Channels       []*biz.Channel
	CurrentChannel *biz.Channel
	ChannelIndex   int

	Perf *biz.PerformanceRecord
}
