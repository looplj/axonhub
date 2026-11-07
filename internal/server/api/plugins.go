package api

import (
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/internal/server/plugin"
)

// orchestratorPlugins passes the process plugin manager into chat orchestrators.
// A nil manager stays a nil interface so requests without a manager do not grow a middleware.
func orchestratorPlugins(manager *plugin.Manager) orchestrator.PluginIntegration {
	if manager == nil {
		return orchestrator.PluginIntegration{}
	}
	return orchestrator.PluginIntegration{Hooks: manager}
}
