package reloadable

import (
	"context"
	"sync/atomic"

	"github.com/looplj/axonhub/axon/agent"
)

type Provider struct {
	current atomic.Value
}

func New(p agent.Provider) *Provider {
	r := &Provider{}
	r.current.Store(p)
	return r
}

func (p *Provider) Swap(next agent.Provider) {
	p.current.Store(next)
}

func (p *Provider) Current() agent.Provider {
	return p.current.Load().(agent.Provider)
}

func (p *Provider) Chat(ctx context.Context, model string, tools []agent.ToolDefinition, messages []agent.Message) (agent.Response, error) {
	return p.Current().Chat(ctx, model, tools, messages)
}

func (p *Provider) ChatStream(ctx context.Context, model string, tools []agent.ToolDefinition, messages []agent.Message) (<-chan agent.StreamEvent, error) {
	return p.Current().ChatStream(ctx, model, tools, messages)
}
