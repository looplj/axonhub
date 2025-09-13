package aisdk

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
)

// ConvertToLLMRequestOptions provides options for message conversion.
type ConvertToLLMRequestOptions struct {
	// IgnoreIncompleteToolCalls filters out tool calls that are still streaming or not yet available
	IgnoreIncompleteToolCalls bool
}

func convertToLLMRequest(req *Request) (*llm.Request, error) {
	return convertToLLMRequestWithOptions(req, nil)
}

//nolint:maintidx
func convertToLLMRequestWithOptions(req *Request, options *ConvertToLLMRequestOptions) (*llm.Request, error) {
	// Base request
	llmReq := &llm.Request{
		Model:    req.Model,
		Messages: []llm.Message{},
		Stream:   lo.ToPtr(true),
	}

	// Apply options defaults
	if options == nil {
		options = &ConvertToLLMRequestOptions{}
	}

	// Helper: convert interface{} to json.RawMessage if possible
	anyToRaw := func(v interface{}) json.RawMessage {
		switch t := v.(type) {
		case nil:
			return nil
		case json.RawMessage:
			return t
		case []byte:
			return json.RawMessage(t)
		case string:
			return json.RawMessage([]byte(t))
		default:
			b, err := json.Marshal(t)
			if err != nil {
				return nil
			}

			return json.RawMessage(b)
		}
	}

	// Helper: map file part to LLM content part (image only for now)
	toContentPartFromFile := func(p UIMessagePart) *llm.MessageContentPart {
		if p.URL == "" {
			return nil
		}
		// Support images via image_url
		if strings.HasPrefix(strings.ToLower(p.MediaType), "image/") {
			return &llm.MessageContentPart{
				Type:     "image_url",
				ImageURL: &llm.ImageURL{URL: p.URL},
			}
		}

		return nil
	}

	// Helper: compact RawMessage to string
	rawToString := func(r json.RawMessage) string {
		if len(r) == 0 {
			return ""
		}

		var v interface{}
		if err := json.Unmarshal(r, &v); err != nil {
			return string(r)
		}

		switch t := v.(type) {
		case string:
			return t
		default:
			b, err := json.Marshal(t)
			if err != nil {
				return string(r)
			}

			return string(b)
		}
	}

	// Helper: determine tool name
	getToolName := func(p UIMessagePart) string {
		if p.ToolName != "" {
			return p.ToolName
		}

		if strings.HasPrefix(p.Type, "tool-") {
			return strings.TrimPrefix(p.Type, "tool-")
		}

		return ""
	}

	// Filter incomplete tool calls if requested
	messages := req.Messages
	if options.IgnoreIncompleteToolCalls {
		messages = make([]UIMessage, len(req.Messages))
		for i, msg := range req.Messages {
			messages[i] = msg
			if len(msg.Parts) > 0 {
				filteredParts := make([]UIMessagePart, 0, len(msg.Parts))
				for _, part := range msg.Parts {
					// Filter out incomplete tool calls
					if (part.Type == "dynamic-tool" || strings.HasPrefix(part.Type, "tool-")) &&
						(part.State == "input-streaming" || part.State == "input-available") {
						continue
					}

					filteredParts = append(filteredParts, part)
				}

				messages[i].Parts = filteredParts
			}
		}
	}

	// Process messages
	for _, msg := range messages {
		role := strings.ToLower(msg.Role)

		switch role {
		case "system":
			// Aggregate text parts and collect provider metadata
			var contentText string

			providerMetadata := make(map[string]interface{})

			if len(msg.Parts) > 0 {
				var sb strings.Builder

				for _, p := range msg.Parts {
					if p.Type == "text" && p.Text != "" {
						sb.WriteString(p.Text)
						// Merge provider metadata
						if len(p.ProviderMetadata) > 0 {
							var metadata map[string]interface{}
							if err := json.Unmarshal(p.ProviderMetadata, &metadata); err == nil {
								for k, v := range metadata {
									providerMetadata[k] = v
								}
							}
						}
					}
				}

				contentText = sb.String()
			} else if s, ok := msg.Content.(string); ok {
				contentText = s
			}

			if contentText != "" {
				systemMsg := llm.Message{
					Role: "system",
					Content: llm.MessageContent{
						Content: lo.ToPtr(contentText),
					},
				}
				// TODO: Add provider metadata support when LLM structs support it
				llmReq.Messages = append(llmReq.Messages, systemMsg)
			}

		case "user":
			// If parts exist, convert supported parts; else use content string
			var (
				parts       []llm.MessageContentPart
				contentText string
			)

			if len(msg.Parts) > 0 {
				for _, p := range msg.Parts {
					switch p.Type {
					case "text":
						if p.Text != "" {
							parts = append(parts, llm.MessageContentPart{Type: "text", Text: lo.ToPtr(p.Text)})
						}
					case "file":
						if cp := toContentPartFromFile(p); cp != nil {
							parts = append(parts, *cp)
						}
					}
				}
			} else if s, ok := msg.Content.(string); ok && s != "" {
				contentText = s
			}

			if len(parts) > 0 {
				llmReq.Messages = append(llmReq.Messages, llm.Message{
					Role:    "user",
					Content: llm.MessageContent{MultipleContent: parts},
				})
			} else if contentText != "" {
				llmReq.Messages = append(llmReq.Messages, llm.Message{
					Role:    "user",
					Content: llm.MessageContent{Content: lo.ToPtr(contentText)},
				})
			}

		case "assistant":
			// If no parts, handle as simple text assistant message
			if len(msg.Parts) == 0 {
				if s, ok := msg.Content.(string); ok && s != "" {
					llmReq.Messages = append(llmReq.Messages, llm.Message{
						Role:    "assistant",
						Content: llm.MessageContent{Content: lo.ToPtr(s)},
					})
				}

				break
			}

			// Block processing separated by step-start
			var block []UIMessagePart

			processBlock := func() {
				if len(block) == 0 {
					return
				}
				// Build assistant message content and tool calls
				var (
					contentParts []llm.MessageContentPart
					toolCalls    []llm.ToolCall
				)

				var toolMessages []llm.Message // tool role messages following assistant

				for _, p := range block {
					switch {
					case p.Type == "text" && p.Text != "":
						textPart := llm.MessageContentPart{Type: "text", Text: lo.ToPtr(p.Text)}
						// TODO: Add provider metadata support when LLM structs support it
						contentParts = append(contentParts, textPart)
					case p.Type == "file":
						if cp := toContentPartFromFile(p); cp != nil {
							// TODO: Add provider metadata support when LLM structs support it
							contentParts = append(contentParts, *cp)
						}
					case p.Type == "reasoning" && p.Text != "":
						// Map reasoning as a special content type - use text type for now
						reasoningPart := llm.MessageContentPart{Type: "text", Text: lo.ToPtr(p.Text)}
						// TODO: Add provider metadata support when LLM structs support it
						contentParts = append(contentParts, reasoningPart)
					case p.Type == "dynamic-tool" || strings.HasPrefix(p.Type, "tool-"):
						// Only include non-streaming tool input states
						if p.State == "input-streaming" {
							break
						}

						toolName := getToolName(p)
						// Build tool call (arguments from Input or RawInput for error state)
						var args json.RawMessage

						if p.State == "output-error" {
							// TS logic: input ?? rawInput (prefer structured input if available)
							if p.Input != nil {
								args = anyToRaw(p.Input)
							} else if len(p.RawInput) > 0 {
								args = p.RawInput
							}
						} else {
							args = anyToRaw(p.Input)
						}
						// Ensure arguments are valid JSON string
						argStr := rawToString(args)
						if argStr == "" {
							argStr = "{}"
						}

						toolCall := llm.ToolCall{
							ID:   p.ToolCallID,
							Type: "function",
							Function: llm.FunctionCall{
								Name:      toolName,
								Arguments: argStr,
							},
						}

						// TODO: Add provider metadata and execution info support when LLM structs support it

						toolCalls = append(toolCalls, toolCall)

						// Handle tool results - only for non-provider-executed tools or provider-executed with results
						if (p.State == "output-available" || p.State == "output-error") &&
							(p.ProviderExecuted == nil || !*p.ProviderExecuted ||
								(*p.ProviderExecuted && (p.State == "output-available" || p.State == "output-error"))) {
							var outputText string
							if p.State == "output-error" && p.ErrorText != "" {
								outputText = p.ErrorText
							} else {
								outputText = rawToString(anyToRaw(p.Output))
							}

							if outputText != "" {
								toolMessages = append(toolMessages, llm.Message{
									Role:       "tool",
									ToolCallID: lo.ToPtr(p.ToolCallID),
									Content:    llm.MessageContent{Content: lo.ToPtr(outputText)},
								})
							}
						}
					}
				}

				// Append assistant message if it has content or tool calls
				if len(contentParts) > 0 || len(toolCalls) > 0 {
					assistantMsg := llm.Message{
						Role:    "assistant",
						Content: llm.MessageContent{MultipleContent: contentParts},
						ToolCalls: func() []llm.ToolCall {
							if len(toolCalls) == 0 {
								return nil
							}
							return toolCalls
						}(),
					}
					llmReq.Messages = append(llmReq.Messages, assistantMsg)
				}
				// Append tool messages after the assistant message
				if len(toolMessages) > 0 {
					llmReq.Messages = append(llmReq.Messages, toolMessages...)
				}

				block = nil
			}

			for _, p := range msg.Parts {
				if p.Type == "step-start" {
					processBlock()
					continue
				}

				block = append(block, p)
			}

			processBlock()

		default:
			// Unsupported role -> error to surface invalid input similar to TS behavior
			return nil, fmt.Errorf("unsupported role: %s", role)
		}
	}

	// Convert tools
	if len(req.Tools) > 0 {
		tools := make([]llm.Tool, 0, len(req.Tools))
		for _, tool := range req.Tools {
			llmTool := llm.Tool{
				Type: tool.Type,
				Function: llm.Function{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
				},
			}
			if tool.Function.Parameters != nil {
				if paramsBytes, err := json.Marshal(tool.Function.Parameters); err == nil {
					llmTool.Function.Parameters = json.RawMessage(paramsBytes)
				}
			}

			tools = append(tools, llmTool)
		}

		llmReq.Tools = tools
	}

	return llmReq, nil
}
