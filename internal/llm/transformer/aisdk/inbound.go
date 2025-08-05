package aisdk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/samber/lo"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// InboundTransformer implements the Inbound interface for AI SDK.
type InboundTransformer struct{}

// NewInboundTransformer creates a new AI SDK inbound transformer.
func NewInboundTransformer() *InboundTransformer {
	return &InboundTransformer{}
}

// AiSDKRequest represents the AI SDK request format.
type AiSDKRequest struct {
	Messages []AiSDKMessage `json:"messages"`
	Model    string         `json:"model,omitempty"`
	Stream   *bool          `json:"stream,omitempty"`
	Tools    []AiSDKTool    `json:"tools,omitempty"`
}

// AiSDKMessage represents a message in AI SDK format.
type AiSDKMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // can be string or array of content parts
	Name    *string     `json:"name,omitempty"`
}

// AiSDKTool represents a tool in AI SDK format.
type AiSDKTool struct {
	Type     string            `json:"type"`
	Function AiSDKToolFunction `json:"function"`
}

// AiSDKToolFunction represents a tool function in AI SDK format.
type AiSDKToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

// TransformRequest transforms AI SDK request to LLM request.
func (t *InboundTransformer) TransformRequest(
	ctx context.Context,
	req *httpclient.Request,
) (*llm.Request, error) {
	// Parse JSON body
	var aiSDKReq AiSDKRequest
	if err := json.Unmarshal(req.Body, &aiSDKReq); err != nil {
		return nil, fmt.Errorf("failed to parse AI SDK request: %w", err)
	}

	// Convert to LLM request
	llmReq := &llm.Request{
		Model:    aiSDKReq.Model,
		Messages: make([]llm.Message, len(aiSDKReq.Messages)),
		Stream:   lo.ToPtr(true),
	}

	// Convert messages
	for i, msg := range aiSDKReq.Messages {
		llmMsg := llm.Message{
			Role: msg.Role,
			Name: msg.Name,
		}

		// Handle content - can be string or array
		switch content := msg.Content.(type) {
		case string:
			llmMsg.Content = llm.MessageContent{
				Content: &content,
			}
		case []interface{}:
			// Handle multi-part content
			parts := make([]llm.MessageContentPart, len(content))
			for j, part := range content {
				if partMap, ok := part.(map[string]interface{}); ok {
					contentPart := llm.MessageContentPart{}
					if partType, exists := partMap["type"]; exists {
						//nolint:forcetypeassert // Will fix.
						contentPart.Type = partType.(string)
					}

					if text, exists := partMap["text"]; exists {
						//nolint:forcetypeassert // Will fix.
						textStr := text.(string)
						contentPart.Text = &textStr
					}

					if imageURL, exists := partMap["image_url"]; exists {
						if imageMap, ok := imageURL.(map[string]interface{}); ok {
							contentPart.ImageURL = &llm.ImageURL{}
							//nolint:forcetypeassert // Will fix.
							if url, exists := imageMap["url"]; exists {
								contentPart.ImageURL.URL = url.(string)
							}

							if detail, exists := imageMap["detail"]; exists {
								//nolint:forcetypeassert // Will fix.
								contentPart.ImageURL.Detail = detail.(string)
							}
						}
					}

					parts[j] = contentPart
				}
			}

			llmMsg.Content = llm.MessageContent{
				MultipleContent: parts,
			}
		}

		llmReq.Messages[i] = llmMsg
	}

	// Convert tools
	if len(aiSDKReq.Tools) > 0 {
		llmReq.Tools = make([]llm.Tool, len(aiSDKReq.Tools))
		for i, tool := range aiSDKReq.Tools {
			llmReq.Tools[i] = llm.Tool{
				Type: tool.Type,
				Function: llm.Function{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
				},
			}

			// Handle parameters
			if tool.Function.Parameters != nil {
				if paramsBytes, err := json.Marshal(tool.Function.Parameters); err == nil {
					llmReq.Tools[i].Function.Parameters = json.RawMessage(paramsBytes)
				}
			}
		}
	}

	return llmReq, nil
}

// TransformResponse transforms LLM response to AI SDK response.
func (t *InboundTransformer) TransformResponse(
	ctx context.Context,
	resp *llm.Response,
) (*httpclient.Response, error) {
	// Convert to AI SDK response format
	aiSDKResp := map[string]interface{}{
		"id":      resp.ID,
		"object":  resp.Object,
		"created": resp.Created,
		"model":   resp.Model,
		"choices": resp.Choices,
	}

	if resp.Usage != nil {
		aiSDKResp["usage"] = resp.Usage
	}

	// Marshal to JSON
	respBody, err := json.Marshal(aiSDKResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal AI SDK response: %w", err)
	}

	// Create response headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")

	return &httpclient.Response{
		StatusCode: 200,
		Headers:    headers,
		Body:       respBody,
	}, nil
}

func (t *InboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*llm.Response],
) (streams.Stream[*httpclient.StreamEvent], error) {
	return streams.MapErr(stream, func(chunk *llm.Response) (*httpclient.StreamEvent, error) {
		return t.TransformStreamChunk(ctx, chunk)
	}), nil
}

func (t *InboundTransformer) TransformStreamChunk(
	ctx context.Context,
	chunk *llm.Response,
) (*httpclient.StreamEvent, error) {
	var streamData []string

	// Process each choice
	for _, choice := range chunk.Choices {
		// Handle text content - Format: 0:"text"\n
		if choice.Delta != nil && choice.Delta.Content.Content != nil &&
			*choice.Delta.Content.Content != "" {
			textJSON, _ := json.Marshal(*choice.Delta.Content.Content)
			streamData = append(streamData, fmt.Sprintf("0:%s\n", string(textJSON)))
		}

		// Handle tool call streaming start - Format: b:{"toolCallId":"id","toolName":"name"}\n
		if choice.Delta != nil && len(choice.Delta.ToolCalls) > 0 {
			for _, toolCall := range choice.Delta.ToolCalls {
				if toolCall.Function.Name != "" {
					// Tool call streaming start
					toolCallStart := map[string]interface{}{
						"toolCallId": toolCall.ID,
						"toolName":   toolCall.Function.Name,
					}
					toolCallJSON, _ := json.Marshal(toolCallStart)
					streamData = append(streamData, fmt.Sprintf("b:%s\n", string(toolCallJSON)))
				}

				if toolCall.Function.Arguments != "" {
					// Tool call delta - Format: c:{"toolCallId":"id","argsTextDelta":"delta"}\n
					toolCallDelta := map[string]interface{}{
						"toolCallId":    toolCall.ID,
						"argsTextDelta": toolCall.Function.Arguments,
					}
					toolCallJSON, _ := json.Marshal(toolCallDelta)
					streamData = append(streamData, fmt.Sprintf("c:%s\n", string(toolCallJSON)))
				}
			}
		}

		// Handle complete tool calls - Format: 9:{"toolCallId":"id","toolName":"name","args":{}}\n
		if choice.Message != nil && len(choice.Message.ToolCalls) > 0 {
			for _, toolCall := range choice.Message.ToolCalls {
				var args interface{}
				if toolCall.Function.Arguments != "" {
					err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
					if err != nil {
						return nil, fmt.Errorf("failed to unmarshal tool call arguments: %w", err)
					}
				}

				toolCallComplete := map[string]interface{}{
					"toolCallId": toolCall.ID,
					"toolName":   toolCall.Function.Name,
					"args":       args,
				}

				toolCallJSON, err := json.Marshal(toolCallComplete)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal tool call complete: %w", err)
				}

				streamData = append(streamData, fmt.Sprintf("9:%s\n", string(toolCallJSON)))
			}
		}

		// Handle finish reason and usage - Format: e:{"finishReason":"stop","usage":{}}\n
		if choice.FinishReason != nil {
			finishData := map[string]interface{}{
				"finishReason": *choice.FinishReason,
			}
			if chunk.Usage != nil {
				finishData["usage"] = chunk.Usage
			}

			finishJSON, _ := json.Marshal(finishData)
			streamData = append(streamData, fmt.Sprintf("e:%s\n", string(finishJSON)))
		}
	}

	// Join all stream data
	eventData := strings.Join(streamData, "")

	// Create headers for AI SDK data stream
	headers := make(http.Header)
	headers.Set("Content-Type", "text/plain; charset=utf-8")
	headers.Set("X-Vercel-Ai-Data-Stream", "v1")

	return &httpclient.StreamEvent{
		// Type: "data",
		Data: []byte(eventData),
	}, nil
}

func (t *InboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, error) {
	panic("unimplemented")
}

// // AggregateStreamChunks aggregates streaming response chunks into a complete response.
// func (t *InboundTransformer) AggregateStreamChunks(
// 	ctx context.Context,
// 	chunks []*llm.Response,
// ) ([]byte, error) {
// 	if len(chunks) == 0 {
// 		return []byte(""), nil
// 	}

// 	// For AI SDK inbound, we aggregate the unified response chunks into AI SDK data stream format
// 	var (
// 		aggregatedContent strings.Builder
// 		lastChunk         *llm.Response
// 	)

// 	for _, chunk := range chunks {
// 		if chunk == nil {
// 			continue
// 		}

// 		// Extract content from the chunk
// 		if len(chunk.Choices) > 0 && chunk.Choices[0].Message != nil {
// 			if chunk.Choices[0].Message.Content.Content != nil {
// 				aggregatedContent.WriteString(*chunk.Choices[0].Message.Content.Content)
// 			}
// 		} else if len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil {
// 			if chunk.Choices[0].Delta.Content.Content != nil {
// 				aggregatedContent.WriteString(*chunk.Choices[0].Delta.Content.Content)
// 			}
// 		}

// 		// Keep the last chunk for metadata
// 		lastChunk = chunk
// 	}

// 	// Create AI SDK data stream format for the complete response
// 	var streamData []string

// 	// Add the complete text content
// 	if aggregatedContent.Len() > 0 {
// 		textData := map[string]interface{}{
// 			"text": aggregatedContent.String(),
// 		}
// 		textJSON, _ := json.Marshal(textData)
// 		streamData = append(streamData, fmt.Sprintf("0:%s\n", string(textJSON)))
// 	}

// 	// Add finish reason and usage if available
// 	if lastChunk != nil {
// 		finishData := map[string]interface{}{
// 			"finishReason": "stop",
// 		}
// 		if lastChunk.Usage != nil {
// 			finishData["usage"] = lastChunk.Usage
// 		}

// 		finishJSON, _ := json.Marshal(finishData)
// 		streamData = append(streamData, fmt.Sprintf("e:%s\n", string(finishJSON)))
// 	}

// 	// Join all stream data
// 	eventData := strings.Join(streamData, "")

// 	return []byte(eventData), nil
// }
