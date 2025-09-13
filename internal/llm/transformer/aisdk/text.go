package aisdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/internal/llm"
	transformer "github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
)

// TextTransformer implements the Inbound interface for AI SDK.
type TextTransformer struct{}

// NewTextTransformer creates a new AI SDK inbound transformer.
func NewTextTransformer() *TextTransformer {
	return &TextTransformer{}
}

func (t *TextTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatAiSDKText
}

// TransformRequest transforms AI SDK request to LLM request.
func (t *TextTransformer) TransformRequest(
	ctx context.Context,
	req *httpclient.Request,
) (*llm.Request, error) {
	var aiSDKReq Request

	err := json.Unmarshal(req.Body, &aiSDKReq)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to parse AI SDK request: %w", transformer.ErrInvalidRequest, err)
	}

	return convertToLLMRequest(&aiSDKReq)
}

// TransformResponse transforms LLM response to AI SDK response.
func (t *TextTransformer) TransformResponse(
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

func (t *TextTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*llm.Response],
) (streams.Stream[*httpclient.StreamEvent], error) {
	return streams.MapErr(stream, func(chunk *llm.Response) (*httpclient.StreamEvent, error) {
		return t.TransformStreamChunk(ctx, chunk)
	}), nil
}

func (t *TextTransformer) TransformStreamChunk(
	ctx context.Context,
	chunk *llm.Response,
) (*httpclient.StreamEvent, error) {
	var streamData []string

	// Process each choice
	for _, choice := range chunk.Choices {
		log.Debug(ctx, "Processing choice for ai text", log.Any("choice", choice))

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
				var args any
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

	return &httpclient.StreamEvent{
		// Type: "data",
		Data: []byte(eventData),
	}, nil
}

func (t *TextTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	// TODO: support.
	return []byte(`{}`), llm.ResponseMeta{}, nil
}

func (t *TextTransformer) TransformError(ctx context.Context, rawErr error) *httpclient.Error {
	if rawErr == nil {
		return &httpclient.Error{
			StatusCode: http.StatusInternalServerError,
			Status:     http.StatusText(http.StatusInternalServerError),
			Body:       []byte(`{"message":"internal server error","type":"internal_server_error"}`),
		}
	}

	if httpErr, ok := xerrors.As[*httpclient.Error](rawErr); ok {
		return httpErr
	}

	// Handle validation errors
	if errors.Is(rawErr, transformer.ErrInvalidRequest) {
		return &httpclient.Error{
			StatusCode: http.StatusBadRequest,
			Status:     http.StatusText(http.StatusBadRequest),
			Body: []byte(
				fmt.Sprintf(`{"message":"%s","type":"invalid_request"}`, strings.TrimPrefix(rawErr.Error(), transformer.ErrInvalidRequest.Error()+": ")),
			),
		}
	}

	if llmErr, ok := xerrors.As[*llm.ResponseError](rawErr); ok {
		return &httpclient.Error{
			StatusCode: llmErr.StatusCode,
			Status:     http.StatusText(llmErr.StatusCode),
			Body:       []byte(fmt.Sprintf(`{"message":"%s","type":"%s"}`, llmErr.Detail.Message, llmErr.Detail.Type)),
		}
	}

	return &httpclient.Error{
		StatusCode: http.StatusInternalServerError,
		Status:     http.StatusText(http.StatusInternalServerError),
		Body:       []byte(fmt.Sprintf(`{"message":"%s","type":"internal_server_error"}`, rawErr.Error())),
	}
}
