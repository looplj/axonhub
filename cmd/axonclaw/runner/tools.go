package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/api"
	"github.com/looplj/axonhub/axon/mcp"
	"github.com/looplj/axonhub/axon/pkg/search"
	"github.com/looplj/axonhub/axon/subagent"
	"github.com/looplj/axonhub/axon/tools"

	"github.com/looplj/axonhub/cmd/axonclaw/bootstrap"
)

func registerTools(
	a *agent.Agent,
	workspace string,
	boot *bootstrap.Result,
	logger *slog.Logger,
	client graphql.Client,
	provider agent.Provider,
	mcpMgr *mcp.Manager,
) {
	enabledBuiltin := map[string]bool{}
	for _, t := range boot.BuiltinTools {
		if t.Name == "" {
			continue
		}
		if t.Enabled {
			enabledBuiltin[t.Name] = true
		}
	}

	if len(enabledBuiltin) == 0 {
		for _, name := range []string{"Read", "Write", "Edit", "Bash", "Grep", "Glob", "Skill", "SpawnAgent"} {
			enabledBuiltin[name] = true
		}
	}

	if enabledBuiltin["Read"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewReadTool(workspace, false)))
	}
	if enabledBuiltin["Write"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewWriteTool(workspace, false)))
	}
	if enabledBuiltin["Edit"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewEditTool(workspace, false)))
	}
	if enabledBuiltin["Bash"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewBashTool(workspace, false)))
	}
	if enabledBuiltin["Grep"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewGrepTool(workspace, false)))
	}
	if enabledBuiltin["Glob"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewGlobTool(workspace, false)))
	}
	if enabledBuiltin["Skill"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewSkillTool(filepath.Join(workspace, "skills"), filepath.Join(workspace, "skills"))))
	}
	if enabledBuiltin["WebFetch"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewWebFetchTool()))
	}
	if enabledBuiltin["WebSearch"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewWebSearchTool(search.NewDuckDuckGoProvider())))
	}

	a.RegisterTool(tools.NewAgentTool(NewSendMessageTool(client)))
	a.RegisterTool(tools.NewAgentTool(NewAxonClawHelpTool()))
	a.RegisterTool(tools.NewAgentTool(NewResetTool(ResetToolOptions{
		Client:    client,
		Agent:     a,
		Workspace: workspace,
		Boot:      boot,
		Logger:    logger,
	})))

	known := map[string]struct{}{}
	for name := range enabledBuiltin {
		known[name] = struct{}{}
	}

	known["SendMessage"] = struct{}{}
	known["AxonClawHelp"] = struct{}{}
	known["Reset"] = struct{}{}
	for _, t := range boot.Tools {
		if t.Name == "" {
			continue
		}
		if _, ok := known[t.Name]; ok {
			continue
		}

		def, err := convertRemoteToolDefinition(t)
		if err != nil {
			logger.Warn("skip invalid tool schema from bootstrap", "tool", t.Name, "error", err)
			continue
		}
		a.RegisterTool(&unimplementedTool{def: def})

		known[t.Name] = struct{}{}
	}

	// Register SpawnAgent tool last so it can enumerate all previously
	// registered tools via the agent's registry.
	if enabledBuiltin["SpawnAgent"] {
		agentDir := filepath.Join(workspace, ".agent", "subagents")

		subagentMgr := subagent.NewManagerFromPath(agentDir)
		if err := subagentMgr.Load(); err != nil {
			logger.Warn("failed to load subagent definitions", "error", err, "path", agentDir)
		}

		a.RegisterTool(tools.NewAgentTool(subagent.NewTool(subagent.ToolOptions{
			Manager:     subagentMgr,
			Provider:    provider,
			ToolSource:  &agentToolSource{agent: a},
			Model:       boot.Model,
			Middlewares: a.Middlewares(),
			Logger:      logger,
		})))
	}

	known[subagent.SpawnAgentToolName] = struct{}{}

	mcpMgr.RegisterTools(a, workspace, known)
}

// agentToolSource adapts an *agent.Agent to the subagent.ToolSource interface,
// allowing the SpawnAgent tool to enumerate all tools registered on the parent.
type agentToolSource struct {
	agent *agent.Agent
}

func (s *agentToolSource) AvailableTools() []agent.Tool {
	return s.agent.RegisteredTools()
}

func (s *agentToolSource) Middlewares() []agent.Middleware {
	return s.agent.Middlewares()
}

type unimplementedTool struct {
	def agent.ToolDefinition
}

func (t *unimplementedTool) Definition() agent.ToolDefinition { return t.def }

func (t *unimplementedTool) Execute(_ context.Context, _ json.RawMessage) agent.ToolResult {
	return agent.ToolResult{Error: fmt.Errorf("tool %q is not implemented in axonclaw", t.def.Name)}
}

func convertRemoteToolDefinition(in *api.AgentBootstrapAgentBootstrapToolsAgentToolDefinition) (agent.ToolDefinition, error) {
	var schema jsonschema.Schema
	if len(in.Parameters) > 0 && string(in.Parameters) != "null" {
		if err := json.Unmarshal(in.Parameters, &schema); err != nil {
			return agent.ToolDefinition{}, err
		}
	}

	if schema.Type == "" {
		schema = jsonschema.Schema{
			Schema:               "https://json-schema.org/draft/2020-12/schema",
			Type:                 "object",
			AdditionalProperties: &jsonschema.Schema{},
		}
	}

	return agent.ToolDefinition{
		Name:        in.Name,
		Description: in.Description,
		Parameters:  schema,
	}, nil
}
