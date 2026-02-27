package runner

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/api"
	"github.com/looplj/axonhub/axon/tools"
)

type SendMessageTool struct {
	client     graphql.Client
	instanceID string
}

func NewSendMessageTool(client graphql.Client, instanceID string) *SendMessageTool {
	return &SendMessageTool{
		client:     client,
		instanceID: instanceID,
	}
}

type sendMessageInput struct {
	Target           string `json:"target"`
	TargetAgentID    string `json:"target_agent_id,omitempty"`
	TargetInstanceID string `json:"target_instance_id,omitempty"`
	Message          string `json:"message"`
}

var sendMessageParameters = jsonschema.Schema{
	Schema: "https://json-schema.org/draft/2020-12/schema",
	Type:   "object",
	Properties: map[string]*jsonschema.Schema{
		"target": {
			Type:        "string",
			Enum:        []interface{}{"user", "peer"},
			Description: "The target of the message: 'user' to reply to the user, 'peer' to send to another agent",
		},
		"target_agent_id": {
			Type:        "string",
			Description: "The agent ID of the target agent (required when target is 'peer', obtained from discover command)",
		},
		"target_instance_id": {
			Type:        "string",
			Description: "The instance ID of the target agent instance (required when target is 'peer', obtained from discover command)",
		},
		"message": {
			Type:        "string",
			Description: "The message content to send",
		},
	},
	Required: []string{"target", "message"},
}

func (t *SendMessageTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "SendMessage",
		Description: "Send a message to the user or to another agent. Use target='user' to reply to the user (this should be your final action after completing a task). Use target='peer' to communicate with other agents (run 'axonclaw discover' first to find available agents).",
		Parameters:  sendMessageParameters,
	}
}

func (t *SendMessageTool) Execute(ctx context.Context, input sendMessageInput) agent.ToolResult {
	switch input.Target {
	case "user":
		return t.sendToUser(ctx, input.Message)
	case "peer":
		return t.sendToPeer(ctx, input)
	default:
		return tools.ErrorResult(fmt.Errorf("invalid target %q: must be 'user' or 'peer'", input.Target))
	}
}

func (t *SendMessageTool) sendToUser(ctx context.Context, message string) agent.ToolResult {
	_, err := api.ReplyMessage(ctx, t.client, &api.ReplyMessageInput{
		InstanceID: t.instanceID,
		Text:       message,
	})
	if err != nil {
		return tools.ErrorResult(err)
	}
	return tools.TextResult("Message sent successfully to user.")
}

func (t *SendMessageTool) sendToPeer(ctx context.Context, input sendMessageInput) agent.ToolResult {
	if input.TargetAgentID == "" {
		return tools.ErrorResult(fmt.Errorf("targetAgentID is required when target is 'peer'"))
	}
	if input.TargetInstanceID == "" {
		return tools.ErrorResult(fmt.Errorf("targetInstanceID is required when target is 'peer'"))
	}

	_, err := api.SendAgentMessage(ctx, t.client, &api.SendAgentMessageInput{
		TargetAgentID:    input.TargetAgentID,
		TargetInstanceID: input.TargetInstanceID,
		Text:             input.Message,
	})
	if err != nil {
		return tools.ErrorResult(err)
	}
	return tools.TextResult(fmt.Sprintf("Message sent successfully to agent %s (instance: %s)", input.TargetAgentID, input.TargetInstanceID))
}
