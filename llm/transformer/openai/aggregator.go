package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// choiceAggregator is a helper struct to aggregate data for each choice.
type choiceAggregator struct {
	index            int
	content          strings.Builder
	reasoningContent strings.Builder
	toolCalls        map[int]*llm.ToolCall // Map to track tool calls by their index within the choice
	finishReason     *string
	role             string
	annotations      map[string]llm.Annotation // Map to track unique annotations by URL
}

// addAnnotations adds annotations from a message to the choice aggregator,
// deduplicating by URL.
func (ca *choiceAggregator) addAnnotations(msg *Message) {
	if msg == nil || len(msg.Annotations) == 0 {
		return
	}
	for _, annotation := range msg.Annotations {
		if annotation.URLCitation != nil && annotation.URLCitation.URL != "" {
			ca.annotations[annotation.URLCitation.URL] = annotation.ToLLMAnnotation()
		}
	}
}

type ChunkTransformFunc func(ctx context.Context, chunk *httpclient.StreamEvent) (*Response, error)

func DefaultTransformChunk(ctx context.Context, chunk *httpclient.StreamEvent) (*Response, error) {
	var response Response
	if err := json.Unmarshal(chunk.Data, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// AggregateStreamChunks aggregates OpenAI streaming response chunks into a complete response.
func AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent, chunkTransformer ChunkTransformFunc) ([]byte, llm.ResponseMeta, error) {
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
		// Map to track unique citations
		citationsMap = make(map[string]struct{})
	)

	for _, chunk := range chunks {
		// Skip [DONE] events
		if bytes.HasPrefix(chunk.Data, []byte("[DONE]")) {
			continue
		}

		chunk, err := chunkTransformer(ctx, chunk)
		if err != nil {
			continue // Skip invalid chunks
		}

		// Process each choice in the chunk
		for _, choice := range chunk.Choices {
			choiceIndex := choice.Index

			// Initialize choice aggregator if it doesn't exist
			if _, ok := choicesAggs[choiceIndex]; !ok {
				choicesAggs[choiceIndex] = &choiceAggregator{
					index:       choiceIndex,
					toolCalls:   make(map[int]*llm.ToolCall),
					annotations: make(map[string]llm.Annotation),
					role:        "assistant",
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

			// Handle annotations from Delta (streaming) and Message (non-streaming chunks)
			choiceAgg.addAnnotations(choice.Delta)
			choiceAgg.addAnnotations(choice.Message)

			// Capture finish reason
			if choice.FinishReason != nil {
				choiceAgg.finishReason = choice.FinishReason
			}
		}

		// Extract usage information if present
		if chunk.Usage != nil {
			usage = chunk.Usage
		}

		// Collect citations from chunk
		for _, citation := range chunk.Citations {
			citationsMap[citation] = struct{}{}
		}

		// Keep the first non-empty system fingerprint
		if systemFingerprint == "" && chunk.SystemFingerprint != "" {
			systemFingerprint = chunk.SystemFingerprint
		}

		// Keep the last chunk for metadata
		lastChunkResponse = chunk
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

		// Set content if available
		if choiceAgg.content.Len() > 0 {
			content := choiceAgg.content.String()
			message.Content = llm.MessageContent{Content: &content}
		}

		// Set tool calls if available (can coexist with content)
		if len(finalToolCalls) > 0 {
			message.ToolCalls = finalToolCalls
		}

		// Set annotations if available
		if len(choiceAgg.annotations) > 0 {
			message.Annotations = make([]llm.Annotation, 0, len(choiceAgg.annotations))
			for _, annotation := range choiceAgg.annotations {
				message.Annotations = append(message.Annotations, annotation)
			}
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

	// Add citations to response if any were collected
	if len(citationsMap) > 0 {
		citations := make([]string, 0, len(citationsMap))
		for citation := range citationsMap {
			citations = append(citations, citation)
		}
		sort.Strings(citations)

		if response.TransformerMetadata == nil {
			response.TransformerMetadata = make(map[string]any)
		}
		response.TransformerMetadata[TransformerMetadataKeyCitations] = citations
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
