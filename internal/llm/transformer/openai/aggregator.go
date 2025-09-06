package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xjson"
)

// choiceAggregator is a helper struct to aggregate data for each choice.
type choiceAggregator struct {
	index            int
	content          strings.Builder
	reasoningContent strings.Builder
	toolCalls        map[int]*llm.ToolCall // Map to track tool calls by their index within the choice
	finishReason     *string
	role             string
}

// AggregateStreamChunks aggregates OpenAI streaming response chunks into a complete response.
func AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	if len(chunks) == 0 {
		data, err := json.Marshal(&llm.Response{})
		return data, llm.ResponseMeta{}, err
	}

	var (
		lastChunkResponse *Response
		usage             *Usage
		systemFingerprint string
		// Map to track choices by their index
		choicesAggs = make(map[int]*choiceAggregator)
	)

	for _, chunk := range chunks {
		// Skip [DONE] events
		if bytes.HasPrefix(chunk.Data, []byte("[DONE]")) {
			continue
		}

		chunk, err := xjson.To[Response](chunk.Data)
		if err != nil {
			continue // Skip invalid chunks
		}

		// Process each choice in the chunk
		for _, choice := range chunk.Choices {
			choiceIndex := choice.Index

			// Initialize choice aggregator if it doesn't exist
			if _, ok := choicesAggs[choiceIndex]; !ok {
				choicesAggs[choiceIndex] = &choiceAggregator{
					index:     choiceIndex,
					toolCalls: make(map[int]*llm.ToolCall),
					role:      "assistant",
				}
			}

			choiceAgg := choicesAggs[choiceIndex]

			if choice.Delta != nil {
				// Handle role
				if choice.Delta.Role != "" {
					choiceAgg.role = choice.Delta.Role
				}

				// Handle content
				if choice.Delta.Content.Content != nil {
					choiceAgg.content.WriteString(*choice.Delta.Content.Content)
				}

				// Handle reasoning content
				if choice.Delta.ReasoningContent != nil {
					choiceAgg.reasoningContent.WriteString(*choice.Delta.ReasoningContent)
				}

				// Handle tool calls
				if len(choice.Delta.ToolCalls) > 0 {
					for _, deltaToolCall := range choice.Delta.ToolCalls {
						// Use the index from the OpenAI delta tool call
						toolCallIndex := deltaToolCall.Index

						// Initialize tool call if it doesn't exist
						if _, ok := choiceAgg.toolCalls[toolCallIndex]; !ok {
							choiceAgg.toolCalls[toolCallIndex] = &llm.ToolCall{
								Index: toolCallIndex,
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
							choiceAgg.toolCalls[toolCallIndex].Function.Arguments += deltaToolCall.Function.Arguments
						}

						// Update function name if provided
						if deltaToolCall.Function.Name != "" {
							choiceAgg.toolCalls[toolCallIndex].Function.Name = deltaToolCall.Function.Name
						}

						// Update ID and type if provided
						if deltaToolCall.ID != "" {
							choiceAgg.toolCalls[toolCallIndex].ID = deltaToolCall.ID
						}

						if deltaToolCall.Type != "" {
							choiceAgg.toolCalls[toolCallIndex].Type = deltaToolCall.Type
						}
					}
				}
			}

			// Capture finish reason
			if choice.FinishReason != nil {
				choiceAgg.finishReason = choice.FinishReason
			}
		}

		// Extract usage information if present
		if chunk.Usage != nil {
			usage = chunk.Usage
		}

		// Keep the first non-empty system fingerprint
		if systemFingerprint == "" && chunk.SystemFingerprint != "" {
			systemFingerprint = chunk.SystemFingerprint
		}

		// Keep the last chunk for metadata
		lastChunkResponse = &chunk
	}

	// Create a complete ChatCompletionResponse based on the last chunk structure
	if lastChunkResponse == nil {
		data, err := json.Marshal(&llm.Response{})
		return data, llm.ResponseMeta{}, err
	}

	choices := make([]llm.Choice, len(choicesAggs))

	for choiceIndex := range choices {
		choiceAgg := choicesAggs[choiceIndex]

		var finalToolCalls []llm.ToolCall
		if len(choiceAgg.toolCalls) > 0 {
			finalToolCalls = make([]llm.ToolCall, len(choiceAgg.toolCalls))
			for index := range finalToolCalls {
				finalToolCalls[index] = *choiceAgg.toolCalls[index]
			}
		}

		// Build the message
		message := &llm.Message{
			Role: choiceAgg.role,
		}

		// Set reasoning content if available
		if choiceAgg.reasoningContent.Len() > 0 {
			reasoningContent := choiceAgg.reasoningContent.String()
			message.ReasoningContent = &reasoningContent
		}

		// Set content or tool calls
		if len(finalToolCalls) > 0 {
			message.ToolCalls = finalToolCalls
			// For tool calls, content should be null
			message.Content = llm.MessageContent{Content: nil}
		} else {
			content := choiceAgg.content.String()
			message.Content = llm.MessageContent{Content: &content}
		}

		// Determine finish reason
		finishReason := choiceAgg.finishReason
		if finishReason == nil {
			if len(finalToolCalls) > 0 {
				finishReason = lo.ToPtr("tool_calls")
			} else {
				finishReason = lo.ToPtr("stop")
			}
		}

		choices[choiceIndex] = llm.Choice{
			Index:        choiceIndex,
			Message:      message,
			FinishReason: finishReason,
		}
	}

	// Build the final response using llm.Response struct
	response := &llm.Response{
		ID:                lastChunkResponse.ID,
		Model:             lastChunkResponse.Model,
		Object:            "chat.completion", // Change from "chat.completion.chunk" to "chat.completion"
		Created:           lastChunkResponse.Created,
		SystemFingerprint: systemFingerprint,
		Choices:           choices,
		Usage:             usage.ToLLMUsage(),
	}

	data, err := json.Marshal(response)
	if err != nil {
		return nil, llm.ResponseMeta{}, err
	}

	return data, llm.ResponseMeta{
		ID:    response.ID,
		Usage: usage.ToLLMUsage(),
	}, nil
}
