package aisdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/looplj/axonhub/internal/llm"
	transformer "github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
)

// DataStreamTransformer implements the AI SDK Data Stream Protocol.
type DataStreamTransformer struct{}

// NewDataStreamTransformer creates a new AI SDK data stream transformer.
func NewDataStreamTransformer() *DataStreamTransformer {
	return &DataStreamTransformer{}
}

func (t *DataStreamTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatAiSDKDataStream
}

// TransformRequest transforms AI SDK request to LLM request.
func (t *DataStreamTransformer) TransformRequest(
	ctx context.Context,
	req *httpclient.Request,
) (*llm.Request, error) {
	// Parse JSON body
	var aiSDKReq Request

	err := json.Unmarshal(req.Body, &aiSDKReq)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to parse AI SDK request: %w", transformer.ErrInvalidRequest, err)
	}

	return convertToLLMRequest(&aiSDKReq)
}

// TransformResponse transforms LLM response to AI SDK response.
func (t *DataStreamTransformer) TransformResponse(
	ctx context.Context,
	resp *llm.Response,
) (*httpclient.Response, error) {
	// For data stream protocol, we don't use non-streaming responses
	// This should not be called in streaming mode
	return nil, fmt.Errorf("data stream protocol only supports streaming responses")
}

func (t *DataStreamTransformer) TransformStreamChunk(
	ctx context.Context,
	chunk *llm.Response,
) (*httpclient.StreamEvent, error) {
	var streamParts []string

	// Handle [DONE] marker
	if chunk.Object == "[DONE]" {
		streamParts = append(streamParts, "data: [DONE]\n")

		return &httpclient.StreamEvent{
			Data: []byte(strings.Join(streamParts, "") + "\n"),
		}, nil
	}

	// Process each choice
	for _, choice := range chunk.Choices {
		log.Debug(ctx, "Processing choice for ai data stream", log.Any("choice", choice))

		// Handle reasoning content (thinking) streaming
		if choice.Delta != nil && choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
			reasoningID := generateTextID()

			// Reasoning start part
			reasoningStart := UIMessagePart{
				Type: "reasoning-start",
				ID:   reasoningID,
			}
			reasoningStartJSON, _ := json.Marshal(reasoningStart)
			streamParts = append(streamParts, fmt.Sprintf("data: %s\n", string(reasoningStartJSON)))

			// Reasoning delta part
			reasoningDelta := UIMessagePart{
				Type:  "reasoning-delta",
				ID:    reasoningID,
				Delta: *choice.Delta.ReasoningContent,
			}
			reasoningDeltaJSON, _ := json.Marshal(reasoningDelta)
			streamParts = append(streamParts, fmt.Sprintf("data: %s\n", string(reasoningDeltaJSON)))

			// Reasoning end part
			reasoningEnd := UIMessagePart{
				Type: "reasoning-end",
				ID:   reasoningID,
			}
			reasoningEndJSON, _ := json.Marshal(reasoningEnd)
			streamParts = append(streamParts, fmt.Sprintf("data: %s\n", string(reasoningEndJSON)))
		}

		// Handle text content streaming
		if choice.Delta != nil && choice.Delta.Content.Content != nil &&
			*choice.Delta.Content.Content != "" {
			// Generate unique ID for this text block
			textID := generateTextID()

			// Text start part
			textStart := UIMessagePart{
				Type: "text-start",
				ID:   textID,
			}
			textStartJSON, _ := json.Marshal(textStart)
			streamParts = append(streamParts, fmt.Sprintf("data: %s\n", string(textStartJSON)))

			// Text delta part
			textDelta := UIMessagePart{
				Type:  "text-delta",
				ID:    textID,
				Delta: *choice.Delta.Content.Content,
			}
			textDeltaJSON, _ := json.Marshal(textDelta)
			streamParts = append(streamParts, fmt.Sprintf("data: %s\n", string(textDeltaJSON)))

			// Text end part
			textEnd := UIMessagePart{
				Type: "text-end",
				ID:   textID,
			}
			textEndJSON, _ := json.Marshal(textEnd)
			streamParts = append(streamParts, fmt.Sprintf("data: %s\n", string(textEndJSON)))
		}

		// Handle tool call streaming
		if choice.Delta != nil && len(choice.Delta.ToolCalls) > 0 {
			for _, toolCall := range choice.Delta.ToolCalls {
				toolCallID := toolCall.ID
				if toolCallID == "" {
					toolCallID = generateTextID()
				}

				// Tool input start part (only if we have a function name)
				if toolCall.Function.Name != "" {
					toolInputStart := UIMessagePart{
						Type:       "tool-input-start",
						ToolCallID: toolCallID,
						ToolName:   toolCall.Function.Name,
					}
					toolInputStartJSON, _ := json.Marshal(toolInputStart)
					streamParts = append(streamParts, fmt.Sprintf("data: %s\n", string(toolInputStartJSON)))
				}

				// Tool input delta part (only if we have arguments)
				if toolCall.Function.Arguments != "" {
					toolInputDelta := UIMessagePart{
						Type:           "tool-input-delta",
						ToolCallID:     toolCallID,
						InputTextDelta: toolCall.Function.Arguments,
					}
					toolInputDeltaJSON, _ := json.Marshal(toolInputDelta)
					streamParts = append(streamParts, fmt.Sprintf("data: %s\n", string(toolInputDeltaJSON)))
				}
			}
		}

		// Handle complete tool calls (tool-input-available)
		if choice.Message != nil && len(choice.Message.ToolCalls) > 0 {
			for _, toolCall := range choice.Message.ToolCalls {
				var input interface{}
				if toolCall.Function.Arguments != "" {
					err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input)
					if err != nil {
						return nil, fmt.Errorf("failed to unmarshal tool call arguments: %w", err)
					}
				}

				// Tool input available
				toolInputAvailable := UIMessagePart{
					Type:       "tool-input-available",
					ToolCallID: toolCall.ID,
					ToolName:   toolCall.Function.Name,
					Input:      input,
				}

				toolInputAvailableJSON, err := json.Marshal(toolInputAvailable)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal tool input available: %w", err)
				}

				streamParts = append(streamParts, fmt.Sprintf("data: %s\n", string(toolInputAvailableJSON)))
			}
		}

		// Handle finish reason
		if choice.FinishReason != nil {
			// Finish step part
			finishStep := UIMessagePart{
				Type: "finish-step",
			}
			finishStepJSON, _ := json.Marshal(finishStep)
			streamParts = append(streamParts, fmt.Sprintf("data: %s\n", string(finishStepJSON)))

			// Finish message part
			finishMessage := UIMessagePart{
				Type: "finish",
			}
			finishMessageJSON, _ := json.Marshal(finishMessage)
			streamParts = append(streamParts, fmt.Sprintf("data: %s\n", string(finishMessageJSON)))

			// Stream termination
			streamParts = append(streamParts, "data: [DONE]\n")
		}
	}

	// Return empty event if no stream parts were generated
	if len(streamParts) == 0 {
		return &httpclient.StreamEvent{
			Data: []byte("\n"),
		}, nil
	}

	// Join all stream parts with additional newline for SSE format
	eventData := strings.Join(streamParts, "") + "\n"

	return &httpclient.StreamEvent{
		Data: []byte(eventData),
	}, nil
}

func (t *DataStreamTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	// Aggregate AI SDK data stream events into a final UIMessage JSON.
	// The transformer emits JSON events per chunk (see convert_stream.go),
	// and we reconstruct high-level parts:
	// - text: aggregate between text-start/text-end
	// - reasoning: aggregate between reasoning-start/reasoning-end
	// - tool inputs: currently ignored for final aggregation (can be added later)
	var (
		result        UIMessage
		meta          llm.ResponseMeta
		currentText   strings.Builder
		textOpen      bool
		currentReason strings.Builder
		reasoningOpen bool
		parts         []UIMessagePart
	)

	// Always assistant for aggregated assistant output
	result.Role = "assistant"

	for _, ev := range chunks {
		if ev == nil || len(ev.Data) == 0 {
			continue
		}

		// Skip [DONE] marker lines
		if string(ev.Data) == "[DONE]" {
			continue
		}

		// Events produced by TransformStream (convert_stream.go) are raw JSON of StreamEvent
		var se StreamEvent
		if err := json.Unmarshal(ev.Data, &se); err != nil {
			// If it's not valid JSON (e.g., SSE formatted), skip for now
			// since current tests use JSON events.
			continue
		}

		switch se.Type {
		case "start":
			// Capture message ID
			result.ID = se.MessageID
			meta.ID = se.MessageID

		case "text-start":
			// Close any open text block defensively
			if textOpen {
				parts = append(parts, UIMessagePart{Type: "text", Text: currentText.String()})
				currentText.Reset()
			}

			textOpen = true

		case "text-delta":
			if textOpen {
				currentText.WriteString(se.Delta)
			}

		case "text-end":
			if textOpen {
				parts = append(parts, UIMessagePart{Type: "text", Text: currentText.String()})
				currentText.Reset()

				textOpen = false
			}

		case "reasoning-start":
			if reasoningOpen {
				parts = append(parts, UIMessagePart{Type: "reasoning", Text: currentReason.String()})
				currentReason.Reset()
			}

			reasoningOpen = true

		case "reasoning-delta":
			if reasoningOpen {
				currentReason.WriteString(se.Delta)
			}

		case "reasoning-end":
			if reasoningOpen {
				parts = append(parts, UIMessagePart{Type: "reasoning", Text: currentReason.String()})
				currentReason.Reset()

				reasoningOpen = false
			}

		case "finish-step", "finish":
			// Nothing to aggregate; markers for UI flows.
		case "tool-input-start", "tool-input-delta", "tool-input-available":
			// For now we don't include tool inputs in the aggregated UIMessage parts.
			// Can be added later if needed by consumers.
			continue
		default:
			// Ignore unknown types in aggregation
		}
	}

	// Close any dangling blocks
	if textOpen {
		parts = append(parts, UIMessagePart{Type: "text", Text: currentText.String()})
	}

	if reasoningOpen {
		parts = append(parts, UIMessagePart{Type: "reasoning", Text: currentReason.String()})
	}

	result.Parts = parts

	b, err := json.Marshal(result)
	if err != nil {
		return nil, llm.ResponseMeta{}, fmt.Errorf("failed to marshal aggregated UIMessage: %w", err)
	}

	return b, meta, nil
}

func (t *DataStreamTransformer) TransformError(ctx context.Context, rawErr error) *httpclient.Error {
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

// generateTextID generates a unique ID for text blocks.
func generateTextID() string {
	return "msg_" + strings.ReplaceAll(uuid.New().String(), "-", "")
}

// SetDataStreamHeaders sets the required headers for AI SDK data stream protocol.
func SetDataStreamHeaders(headers http.Header) {
	headers.Set("X-Vercel-Ai-Ui-Message-Stream", "v1")
	headers.Set("Content-Type", "text/event-stream")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Connection", "keep-alive")
}
