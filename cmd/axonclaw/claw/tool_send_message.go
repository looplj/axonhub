package claw

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/api"
	"github.com/looplj/axonhub/axon/tools"
)

type SendMessageTool struct {
	client graphql.Client
	logger *slog.Logger
}

type SendMessageToolOptions struct {
	Client graphql.Client
	Logger *slog.Logger
}

func NewSendMessageTool(opts SendMessageToolOptions) *SendMessageTool {
	return &SendMessageTool{
		client: opts.Client,
		logger: opts.Logger,
	}
}

type sendMessageInput struct {
	Target           string  `json:"target"`
	TargetAgentID    string  `json:"target_agent_id,omitempty"`
	TargetInstanceID *string `json:"target_agent_instance_id,omitempty"`
	Message          string  `json:"message"`
	ReplyToMessageID *string `json:"reply_to_message_id,omitempty"`
}

var sendMessageParameters = jsonschema.Schema{
	Schema: "https://json-schema.org/draft/2020-12/schema",
	Type:   "object",
	Properties: map[string]*jsonschema.Schema{
		"target": {
			Type:        "string",
			Enum:        []any{"user", "peer"},
			Description: "The target of the message: 'user' to reply to the user, 'peer' to send to another agent",
		},
		"target_agent_id": {
			Type:        "string",
			Description: "The agent ID to send the message to, required when target is 'peer'",
		},
		"target_agent_instance_id": {
			Type:        "string",
			Description: "The specific agent instance ID to send the message to, optional and will send to all instances if not provided.",
		},
		"message": {
			Type:        "string",
			Description: "The message content to send",
		},
		"reply_to_message_id": {
			Type:        "string",
			Description: `The agent message ID (gid://axonhub/AgentMessage/<id>) to reply to. If provided, the platform reply will be attached to the external message referenced by that inbound message.`,
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
		return t.sendToUser(ctx, input.Message, input.ReplyToMessageID)
	case "peer":
		return t.sendToPeer(ctx, input)
	default:
		return tools.ErrorResult(fmt.Errorf("invalid target %q: must be 'user' or 'peer'", input.Target))
	}
}

func (t *SendMessageTool) sendToUser(ctx context.Context, message string, replyToMessageID *string) agent.ToolResult {
	if message == HeartbeatOKToken {
		return tools.TextResult("Heartbeat OK")
	}
	if replyToMessageID == nil || *replyToMessageID == "" {
		replyToMessageID = nil
	}

	_, err := api.ReplyMessage(ctx, t.client, &api.ReplyMessageInput{
		Text:             message,
		ReplyToMessageID: replyToMessageID,
	})
	if err != nil {
		return tools.ErrorResult(err)
	}
	return tools.TextResult("Message sent successfully to user.")
}

func (t *SendMessageTool) sendToPeer(ctx context.Context, input sendMessageInput) agent.ToolResult {
	if input.TargetAgentID == "" {
		return tools.ErrorResult(fmt.Errorf("target_agent_id is required when target is 'peer'"))
	}

	apiInput := &api.SendAgentMessageInput{
		TargetAgentID:         input.TargetAgentID,
		TargetAgentInstanceID: input.TargetInstanceID,
		Text:                  input.Message,
	}

	_, err := api.SendAgentMessage(ctx, t.client, apiInput)
	if err != nil {
		return tools.ErrorResult(err)
	}
	return tools.TextResult("Message sent successfully")
}
