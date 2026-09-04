package cc

import (
	"context"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

// FixMissingToolCalls returns a pipeline middleware that:
//  1. Inserts synthetic tool messages for any assistant tool_calls that lack a
//     corresponding tool response.
//  2. Adds dummy tool definitions for any function name referenced in tool_calls
//     that is missing from the request's tools array.
//
// This prevents downstream providers from rejecting requests due to incomplete
// tool call cycles or unknown tool definitions (e.g., Codex-specific tools like
// exec_command that are present in conversation history but not in the client's
// tool definitions).
func FixMissingToolCalls() pipeline.Middleware {
	return pipeline.OnLlmRequest("FixMissingToolCalls",
		func(ctx context.Context, request *llm.Request) (*llm.Request, error) {
			if request == nil || len(request.Messages) == 0 {
				return request, nil
			}

			// Step 1: Fix missing tool responses
			request.Messages = responses.FixMissingToolCallOutputs(request.Messages)

			// Step 2: Ensure all tool names in conversation history have
			// matching tool definitions. Downstream providers reject tool
			// calls whose function name is not in the tools array.
			knownTools := make(map[string]bool, len(request.Tools))
			for _, t := range request.Tools {
				if t.Type == "function" && t.Function.Name != "" {
					knownTools[t.Function.Name] = true
				}
			}

			for _, msg := range request.Messages {
				if msg.Role != "assistant" || len(msg.ToolCalls) == 0 {
					continue
				}
				for _, tc := range msg.ToolCalls {
					name := tc.Function.Name
					if name == "" || knownTools[name] {
						continue
					}
					// Add a minimal tool definition so downstream
					// providers don't reject the request.
					request.Tools = append(request.Tools, llm.Tool{
						Type: "function",
						Function: llm.Function{
							Name:        name,
							Description: name,
							Parameters:  []byte(`{"type":"object","properties":{}}`),
						},
					})
					knownTools[name] = true
				}
			}

			return request, nil
		},
	)
}
