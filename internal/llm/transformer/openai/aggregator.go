package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/samber/lo"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// AggregateStreamChunks aggregates OpenAI streaming response chunks into a complete response.
func AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, error) {
	if len(chunks) == 0 {
		return json.Marshal(&llm.Response{})
	}

	// For OpenAI-style streaming, we need to aggregate the delta content from chunks
	// into a complete ChatCompletionResponse
	var (
		aggregatedContent strings.Builder
		lastChunkResponse *llm.Response
		usage             *llm.Usage
		systemFingerprint string
		toolCalls         = make(map[int]*llm.ToolCall) // Map to track tool calls by index
		finishReason      *string
	)

	for _, chunk := range chunks {
		// Skip [DONE] events
		if bytes.HasPrefix(chunk.Data, []byte("[DONE]")) {
			continue
		}

		var chunkResponse llm.Response

		err := json.Unmarshal(chunk.Data, &chunkResponse)
		if err != nil {
			continue // Skip invalid chunks
		}

		// Extract content and tool calls from choices[0].delta if it exists
		if len(chunkResponse.Choices) > 0 {
			choice := chunkResponse.Choices[0]

			if choice.Delta != nil {
				// Handle content
				if choice.Delta.Content.Content != nil {
					aggregatedContent.WriteString(*choice.Delta.Content.Content)
				}

				// Handle tool calls
				if len(choice.Delta.ToolCalls) > 0 {
					for _, deltaToolCall := range choice.Delta.ToolCalls {
						// Use the index from the OpenAI delta tool call
						index := deltaToolCall.Index

						// Initialize tool call if it doesn't exist
						if _, ok := toolCalls[index]; !ok {
							toolCalls[index] = &llm.ToolCall{
								Index: index,
								ID:    deltaToolCall.ID,
								Type:  deltaToolCall.Type,
								Function: llm.FunctionCall{
									Name:      deltaToolCall.Function.Name,
									Arguments: "",
								},
							}
						}

						// Aggregate function arguments
						if deltaToolCall.Function.Arguments != "" {
							toolCalls[index].Function.Arguments += deltaToolCall.Function.Arguments
						}

						// Update function name if provided
						if deltaToolCall.Function.Name != "" {
							toolCalls[index].Function.Name = deltaToolCall.Function.Name
						}

						// Update ID and type if provided
						if deltaToolCall.ID != "" {
							toolCalls[index].ID = deltaToolCall.ID
						}

						if deltaToolCall.Type != "" {
							toolCalls[index].Type = deltaToolCall.Type
						}
					}
				}
			}

			// Capture finish reason
			if choice.FinishReason != nil {
				finishReason = choice.FinishReason
			}
		}

		// Extract usage information if present
		if chunkResponse.Usage != nil {
			usage = chunkResponse.Usage
		}

		// Keep the first non-empty system fingerprint
		if systemFingerprint == "" && chunkResponse.SystemFingerprint != "" {
			systemFingerprint = chunkResponse.SystemFingerprint
		}

		// Keep the last chunk for metadata
		lastChunkResponse = &chunkResponse
	}

	// Create a complete ChatCompletionResponse based on the last chunk structure
	if lastChunkResponse == nil {
		return json.Marshal(&llm.Response{})
	}

	finalToolCalls := make([]llm.ToolCall, len(toolCalls))
	for i := range finalToolCalls {
		finalToolCalls[i] = *toolCalls[i]
	}

	// Build the message
	message := &llm.Message{
		Role: "assistant",
	}

	// Set content or tool calls
	if len(finalToolCalls) > 0 {
		message.ToolCalls = finalToolCalls
		// For tool calls, content should be null
		message.Content = llm.MessageContent{Content: nil}
	} else {
		content := aggregatedContent.String()
		message.Content = llm.MessageContent{Content: &content}
	}

	// Determine finish reason
	if finishReason == nil {
		if len(finalToolCalls) > 0 {
			finishReason = lo.ToPtr("tool_calls")
		} else {
			finishReason = lo.ToPtr("stop")
		}
	}

	// Build the final response using llm.Response struct
	response := &llm.Response{
		ID:                lastChunkResponse.ID,
		Model:             lastChunkResponse.Model,
		Object:            "chat.completion", // Change from "chat.completion.chunk" to "chat.completion"
		Created:           lastChunkResponse.Created,
		SystemFingerprint: systemFingerprint,
		Choices: []llm.Choice{
			{
				Index:        0,
				Message:      message,
				FinishReason: finishReason,
			},
		},
		Usage: usage,
	}

	return json.Marshal(response)
}
