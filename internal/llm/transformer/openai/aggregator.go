package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

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

		// Extract content from choices[0].delta.content if it exists
		if len(chunkResponse.Choices) > 0 {
			if chunkResponse.Choices[0].Delta != nil && chunkResponse.Choices[0].Delta.Content.Content != nil {
				aggregatedContent.WriteString(*chunkResponse.Choices[0].Delta.Content.Content)
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

	// Build the final response using llm.Response struct
	response := &llm.Response{
		ID:                lastChunkResponse.ID,
		Model:             lastChunkResponse.Model,
		Object:            "chat.completion", // Change from "chat.completion.chunk" to "chat.completion"
		Created:           lastChunkResponse.Created,
		SystemFingerprint: systemFingerprint,
		Choices: []llm.Choice{
			{
				Index: 0,
				Message: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: &[]string{aggregatedContent.String()}[0],
					},
				},
				FinishReason: &[]string{"stop"}[0],
			},
		},
		Usage: usage,
	}

	return json.Marshal(response)
}
