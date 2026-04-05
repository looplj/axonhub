package runtime

import (
	"context"

	"github.com/looplj/axonhub/axon/agent"
	axonconf "github.com/looplj/axonhub/axon/conf"
	"github.com/looplj/axonhub/axon/provider/reloadable"
	cliconf "github.com/looplj/axonhub/cmd/axoncli/conf"
)

type Applier struct {
	agent          *agent.Agent
	reloadable     *reloadable.Provider
	defaultMaxIter int
	systemPrompt   string
}

func NewApplier(a *agent.Agent, p *reloadable.Provider, systemPrompt string, defaultMaxIter int) *Applier {
	return &Applier{
		agent:          a,
		reloadable:     p,
		defaultMaxIter: defaultMaxIter,
		systemPrompt:   systemPrompt,
	}
}

func (a *Applier) Apply(_ context.Context, oldV, newV cliconf.Config, changes axonconf.ChangeSet) (axonconf.ApplyResult, error) {
	if shouldRebuildProvider(changes) {
		a.reloadable.Swap(BuildProvider(newV))
	}

	if shouldUpdateAgent(changes) {
		a.agent.UpdateConfig(func(cfg agent.Config) agent.Config {
			cfg.Model = newV.Model
			if len(cfg.SystemPrompts) == 0 {
				cfg.SystemPrompts = []string{a.systemPrompt}
			}
			if cfg.MaxIterations == 0 {
				cfg.MaxIterations = a.defaultMaxIter
			}
			return cfg
		})
	}

	return axonconf.ApplyResult{
		EffectiveAt: "next_request",
		Attributes: map[string]string{
			"model": newV.Model,
		},
	}, nil
}

func shouldUpdateAgent(changes axonconf.ChangeSet) bool {
	return changes.IsAnyKeyChanged("model")
}

func shouldRebuildProvider(changes axonconf.ChangeSet) bool {
	return changes.IsAnyKeyChanged("base_url", "api_key", "trace_header", "thread_header")
}
