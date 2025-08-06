package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// AggregateStreamChunks aggregates OpenAI streaming response chunks into a complete response.
func AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, error) {
	if len(chunks) == 0 {
		emptyResp := &llm.Response{}
		return json.Marshal(emptyResp)
	}

	// For OpenAI-style streaming, we need to aggregate the delta content from chunks
	// into a complete ChatCompletionResponse
	var (
		aggregatedContent strings.Builder
		lastChunk         map[string]any
	)

	for _, chunk := range chunks {
		// Skip [DONE] events
		if bytes.HasPrefix(chunk.Data, []byte("[DONE]")) {
			continue
		}

		var chunkData map[string]any

		err := json.Unmarshal(chunk.Data, &chunkData)
		if err != nil {
			continue // Skip invalid chunks
		}

		// Extract content from choices[0].delta.content if it exists
		if choices, ok := chunkData["choices"].([]any); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]any); ok {
				if delta, ok := choice["delta"].(map[string]any); ok {
					if content, ok := delta["content"].(string); ok {
						aggregatedContent.WriteString(content)
					}
				}
			}
		}

		// Keep the last chunk for metadata
		lastChunk = chunkData
	}

	// Create a complete ChatCompletionResponse based on the last chunk structure
	if lastChunk == nil {
		emptyResp := &llm.Response{}
		return json.Marshal(emptyResp)
	}

	// Build the final response
	finalResponse := map[string]interface{}{
		"object": "chat.completion", // Change from "chat.completion.chunk" to "chat.completion"
	}

	// Copy metadata from the last chunk
	for key, value := range lastChunk {
		if key != "choices" && key != "object" {
			finalResponse[key] = value
		}
	}

	// Create the final choices with aggregated content
	finalResponse["choices"] = []map[string]interface{}{
		{
			"index": 0,
			"message": map[string]interface{}{
				"role":    "assistant",
				"content": aggregatedContent.String(),
			},
			"finish_reason": "stop",
		},
	}

	// Marshal the final response directly
	finalJSON, err := json.Marshal(finalResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal final response: %w", err)
	}

	return finalJSON, nil
}
