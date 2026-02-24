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
	"github.com/looplj/axonhub/axon/pkg/search"
	"github.com/looplj/axonhub/axon/tools"
	"github.com/looplj/axonhub/cmd/axonclaw/bootstrap"
)

func registerTools(
	a *agent.Agent,
	threadWorkspace string,
	boot *bootstrap.Result,
	logger *slog.Logger,
	client graphql.Client,
	instanceID string,
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
		for _, name := range []string{"Read", "Write", "Edit", "Bash", "Grep", "Glob", "Skill"} {
			enabledBuiltin[name] = true
		}
	}

	if enabledBuiltin["Read"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewReadTool(threadWorkspace, true)))
	}
	if enabledBuiltin["Write"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewWriteTool(threadWorkspace, true)))
	}
	if enabledBuiltin["Edit"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewEditTool(threadWorkspace, true)))
	}
	if enabledBuiltin["Bash"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewBashTool(threadWorkspace, true)))
	}
	if enabledBuiltin["Grep"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewGrepTool(threadWorkspace, true)))
	}
	if enabledBuiltin["Glob"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewGlobTool(threadWorkspace, true)))
	}
	if enabledBuiltin["Skill"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewSkillTool(filepath.Join(threadWorkspace, "skills"), filepath.Join(threadWorkspace, "skills"))))
	}
	if enabledBuiltin["WebFetch"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewWebFetchTool()))
	}
	if enabledBuiltin["WebSearch"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewWebSearchTool(search.NewDuckDuckGoProvider())))
	}

	a.RegisterTool(tools.NewAgentTool(NewReplyMessageTool(client, instanceID)))
	a.RegisterTool(tools.NewAgentTool(NewAxonClawHelpTool()))

	known := map[string]struct{}{}
	for name := range enabledBuiltin {
		known[name] = struct{}{}
	}
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
	}
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
