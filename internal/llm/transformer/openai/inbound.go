package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/samber/lo"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// InboundTransformer implements transformer.Inbound for OpenAI format.
type InboundTransformer struct{}

// NewInboundTransformer creates a new OpenAI InboundTransformer.
func NewInboundTransformer() transformer.Inbound {
	return &InboundTransformer{}
}

// TransformRequest transforms HTTP request to ChatCompletionRequest.
func (t *InboundTransformer) TransformRequest(
	ctx context.Context,
	httpReq *httpclient.Request,
) (*llm.Request, error) {
	if httpReq == nil {
		return nil, fmt.Errorf("http request is nil")
	}

	if len(httpReq.Body) == 0 {
		return nil, fmt.Errorf("request body is empty")
	}

	// Check content type
	contentType := httpReq.Headers.Get("Content-Type")
	if contentType == "" {
		contentType = httpReq.Headers.Get("Content-Type")
	}

	if !strings.Contains(strings.ToLower(contentType), "application/json") {
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}

	var chatReq llm.Request
	if err := json.Unmarshal(httpReq.Body, &chatReq); err != nil {
		return nil, fmt.Errorf("failed to decode openai request: %w", err)
	}

	// Validate required fields
	if chatReq.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	if len(chatReq.Messages) == 0 {
		return nil, fmt.Errorf("messages are required")
	}

	return &chatReq, nil
}

// TransformResponse transforms ChatCompletionResponse to Response.
func (t *InboundTransformer) TransformResponse(
	ctx context.Context,
	chatResp *llm.Response,
) (*httpclient.Response, error) {
	if chatResp == nil {
		return nil, fmt.Errorf("chat completion response is nil")
	}

	body, err := json.Marshal(chatResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat completion response: %w", err)
	}

	// Create generic response
	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Headers: http.Header{
			"Content-Type":  []string{"application/json"},
			"Cache-Control": []string{"no-cache"},
		},
	}, nil
}

// TransformStreamChunk transforms ChatCompletionResponse to StreamEvent.
func (t *InboundTransformer) TransformStreamChunk(
	ctx context.Context,
	chatResp *llm.Response,
) (*httpclient.StreamEvent, error) {
	if chatResp == nil {
		return nil, fmt.Errorf("chat completion response is nil")
	}

	if chatResp.Object == "[DONE]" {
		return &httpclient.StreamEvent{
			Data: []byte("[DONE]"),
		}, nil
	}

	// For OpenAI, we keep the original response format as the event data
	eventData, err := json.Marshal(chatResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat completion response: %w", err)
	}

	return &httpclient.StreamEvent{
		Type: "",
		Data: eventData,
	}, nil
}

// AggregateStreamChunks aggregates streaming response chunks into a complete response.
func (t *InboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*llm.Response,
) ([]byte, error) {
	if len(chunks) == 0 {
		return json.Marshal(&llm.Response{})
	}

	// For OpenAI inbound, we aggregate the unified response chunks into a complete OpenAI response
	var (
		aggregatedContent strings.Builder
		lastChunk         *llm.Response
	)

	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}

		// Extract content from the chunk
		if len(chunk.Choices) > 0 && chunk.Choices[0].Message != nil {
			if chunk.Choices[0].Message.Content.Content != nil {
				aggregatedContent.WriteString(*chunk.Choices[0].Message.Content.Content)
			}
		} else if len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil {
			if chunk.Choices[0].Delta.Content.Content != nil {
				aggregatedContent.WriteString(*chunk.Choices[0].Delta.Content.Content)
			}
		}

		// Keep the last chunk for metadata
		lastChunk = chunk
	}

	// Create a complete response based on the last chunk
	if lastChunk == nil {
		return json.Marshal(&llm.Response{})
	}

	// Build the final response
	finalResponse := &llm.Response{
		ID:      lastChunk.ID,
		Object:  "chat.completion", // Change from "chat.completion.chunk" to "chat.completion"
		Created: lastChunk.Created,
		Model:   lastChunk.Model,
		Usage:   lastChunk.Usage,
		Choices: []llm.Choice{
			{
				Index: 0,
				Message: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: lo.ToPtr(aggregatedContent.String()),
					},
				},
				FinishReason: lastChunk.Choices[0].FinishReason,
			},
		},
	}

	return json.Marshal(finalResponse)
}
