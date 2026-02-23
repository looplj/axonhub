package runner

import (
	"context"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/api"
	"github.com/looplj/axonhub/axon/tools"
)

type ReplyMessageTool struct {
	client     graphql.Client
	instanceID string
}

func NewReplyMessageTool(client graphql.Client, instanceID string) *ReplyMessageTool {
	return &ReplyMessageTool{
		client:     client,
		instanceID: instanceID,
	}
}

type replyMessageInput struct {
	Message string `json:"message"`
}

var replyMessageParameters = jsonschema.Schema{
	Schema: "https://json-schema.org/draft/2020-12/schema",
	Type:   "object",
	Properties: map[string]*jsonschema.Schema{
		"message": {
			Type:        "string",
			Description: "The message to send back to the user",
		},
	},
	Required: []string{"message"},
}

func (t *ReplyMessageTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "ReplyMessage",
		Description: "Send a message back to the user. Use this tool when you have completed the task and want to respond to the user. This should be the last tool you call after finishing your work.",
		Parameters:  replyMessageParameters,
	}
}

func (t *ReplyMessageTool) Execute(ctx context.Context, input replyMessageInput) agent.ToolResult {
	_, err := api.PushAgentMessage(ctx, t.client, &api.PushAgentMessageInput{
		InstanceID: t.instanceID,
		Text:       input.Message,
	})
	if err != nil {
		return tools.ErrorResult(err)
	}
	return tools.TextResult("Message sent successfully.")
}
