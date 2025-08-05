package biz

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/samber/lo"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// RequestService handles request and request execution operations.
type RequestService struct {
	EntClient     *ent.Client
	SystemService *SystemService
}

// NewRequestService creates a new RequestService.
func NewRequestService(entClient *ent.Client, systemService *SystemService) *RequestService {
	return &RequestService{
		EntClient:     entClient,
		SystemService: systemService,
	}
}

// CreateRequest creates a new request record.
func (s *RequestService) CreateRequest(
	ctx context.Context,
	apiKey *ent.APIKey,
	chatReq *llm.Request,
	requestBody any,
) (*ent.Request, error) {
	requestBodyBytes, err := Marshal(requestBody)
	if err != nil {
		log.Error(ctx, "Failed to serialize request body", log.Cause(err))
		return nil, err
	}

	// Create request record
	req, err := s.EntClient.Request.Create().
		// SetAPIKey(lo.TernaryF(apiKey != nil, func() *ent.APIKey { return apiKey }, func() *ent.APIKey { return nil })).
		SetUserID(lo.TernaryF(apiKey != nil, func() int { return apiKey.UserID }, func() int { return 0 })).
		SetModelID(chatReq.Model).
		SetStatus(request.StatusProcessing).
		SetRequestBody(requestBodyBytes).
		Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to create request", log.Cause(err))
		return nil, err
	}

	return req, nil
}

// CreateRequestExecution creates a new request execution record.
func (s *RequestService) CreateRequestExecution(
	ctx context.Context,
	channel *Channel,
	req *ent.Request,
	requestBody any,
) (*ent.RequestExecution, error) {
	requestBodyBytes, err := Marshal(requestBody)
	if err != nil {
		log.Error(ctx, "Failed to marshal request body", log.Cause(err))
		return nil, err
	}

	reqExec, err := s.EntClient.RequestExecution.Create().
		SetRequestID(req.ID).
		SetUserID(req.UserID).
		SetChannelID(channel.ID).
		SetModelID("TODO").
		SetRequestBody(requestBodyBytes).
		SetStatus(requestexecution.StatusProcessing).
		Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to create request execution", log.Cause(err))
		return nil, err
	}

	return reqExec, nil
}

// UpdateRequestCompleted updates request status to completed with response body.
func (s *RequestService) UpdateRequestCompleted(
	ctx context.Context,
	requestID int,
	responseBody any,
) error {
	responseBodyBytes, err := Marshal(responseBody)
	if err != nil {
		log.Error(ctx, "Failed to serialize response body", log.Cause(err))
		return err
	}

	_, err = s.EntClient.Request.UpdateOneID(requestID).
		SetStatus(request.StatusCompleted).
		SetResponseBody(responseBodyBytes).
		Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to update request status to completed", log.Cause(err))
		return err
	}

	return nil
}

// UpdateRequestFailed updates request status to failed.
func (s *RequestService) UpdateRequestFailed(ctx context.Context, requestID int) error {
	_, err := s.EntClient.Request.UpdateOneID(requestID).
		SetStatus(request.StatusFailed).
		Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to update request status to failed", log.Cause(err))
		return err
	}

	return nil
}

// UpdateRequestExecutionCompleted updates request execution status to completed with response body.
func (s *RequestService) UpdateRequestExecutionCompleted(
	ctx context.Context,
	executionID int,
	responseBody any,
) error {
	responseBodyBytes, err := Marshal(responseBody)
	if err != nil {
		return err
	}

	_, err = s.EntClient.RequestExecution.UpdateOneID(executionID).
		SetStatus(requestexecution.StatusCompleted).
		SetResponseBody(responseBodyBytes).
		Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to update request execution status to completed", log.Cause(err))
		return err
	}

	return nil
}

// UpdateRequestExecutionFailed updates request execution status to failed with error message.
func (s *RequestService) UpdateRequestExecutionFailed(
	ctx context.Context,
	executionID int,
	errorMsg string,
) error {
	_, err := s.EntClient.RequestExecution.UpdateOneID(executionID).
		SetStatus(requestexecution.StatusFailed).
		SetErrorMessage(errorMsg).
		Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to update request execution status to failed", log.Cause(err))
		return err
	}

	return nil
}

// AppendRequestExecutionChunk appends a response chunk to request execution.
// Only stores chunks if the system StoreChunks setting is enabled.
func (s *RequestService) AppendRequestExecutionChunk(
	ctx context.Context,
	executionID int,
	chunk any,
) error {
	// Check if chunk storage is enabled
	storeChunks, err := s.SystemService.StoreChunks(ctx)
	if err != nil {
		log.Warn(ctx, "Failed to get StoreChunks setting, defaulting to false", log.Cause(err))

		storeChunks = false
	}

	// Only store chunks if enabled
	if !storeChunks {
		return nil
	}

	chunkBytes, err := Marshal(chunk)
	if err != nil {
		log.Error(ctx, "Failed to marshal chunk", log.Cause(err))
		return err
	}

	_, err = s.EntClient.RequestExecution.UpdateOneID(executionID).
		AppendResponseChunks([]objects.JSONRawMessage{chunkBytes}).
		Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to append response chunk", log.Cause(err))
		return err
	}

	return nil
}

// UpdateRequestExecutionCompletedWithChunks updates request execution status to completed and aggregates chunks into response body.
func (s *RequestService) UpdateRequestExecutionCompletedWithChunks(
	ctx context.Context,
	executionID int,
	chunks []objects.JSONRawMessage,
	outboundTransformer transformer.Outbound,
) error {
	// Convert JSONRawMessage chunks to []*httpclient.StreamEvent for transformer
	streamEvents := make([]*httpclient.StreamEvent, len(chunks))
	for i, chunk := range chunks {
		streamEvents[i] = &httpclient.StreamEvent{
			Data: []byte(chunk),
		}
	}

	// Use outbound transformer to aggregate chunks
	chatResp, err := outboundTransformer.AggregateStreamChunks(ctx, streamEvents)
	if err != nil {
		log.Error(ctx, "Failed to aggregate chunks using transformer", log.Cause(err))
		return err
	}

	// Marshal the aggregated response
	aggregatedResponse, err := Marshal(chatResp)
	if err != nil {
		log.Error(ctx, "Failed to marshal aggregated response", log.Cause(err))
		return err
	}

	_, err = s.EntClient.RequestExecution.UpdateOneID(executionID).
		SetStatus(requestexecution.StatusCompleted).
		SetResponseBody(aggregatedResponse).
		Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to update request execution status to completed", log.Cause(err))
		return err
	}

	return nil
}

// AggregateChunksToResponseWithTransformer aggregates streaming chunks using the provided outbound transformer.
func (s *RequestService) AggregateChunksToResponseWithTransformer(
	ctx context.Context,
	chunks []objects.JSONRawMessage,
	outboundTransformer transformer.Outbound,
) (objects.JSONRawMessage, error) {
	// Convert JSONRawMessage chunks to []*httpclient.StreamEvent for transformer
	streamEvents := make([]*httpclient.StreamEvent, len(chunks))
	for i, chunk := range chunks {
		streamEvents[i] = &httpclient.StreamEvent{
			Data: []byte(chunk),
		}
	}

	// Use outbound transformer to aggregate chunks
	chatResp, err := outboundTransformer.AggregateStreamChunks(ctx, streamEvents)
	if err != nil {
		return nil, err
	}

	// Marshal the aggregated response
	return Marshal(chatResp)
}

// AggregateChunksToResponse aggregates streaming chunks into a complete LLM response
// Deprecated: Use AggregateChunksToResponseWithTransformer instead for better multi-platform support.
func (s *RequestService) AggregateChunksToResponse(
	chunks []objects.JSONRawMessage,
) (objects.JSONRawMessage, error) {
	if len(chunks) == 0 {
		return objects.JSONRawMessage("{}"), nil
	}

	// For OpenAI-style streaming, we need to aggregate the delta content from chunks
	// into a complete ChatCompletionResponse
	var (
		aggregatedContent strings.Builder
		lastChunk         map[string]interface{}
	)

	for _, chunk := range chunks {
		var chunkData map[string]interface{}
		if err := json.Unmarshal(chunk, &chunkData); err != nil {
			continue // Skip invalid chunks
		}

		// Extract content from choices[0].delta.content if it exists
		if choices, ok := chunkData["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				if delta, ok := choice["delta"].(map[string]interface{}); ok {
					if content, ok := delta["content"].(string); ok {
						aggregatedContent.WriteString(content)
					}
				}
			}
		}

		// Keep the last chunk for metadata
		lastChunk = chunkData
	}

	// Create a complete response using the last chunk as template
	if lastChunk != nil {
		// Convert streaming response to complete response
		if choices, ok := lastChunk["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				// Replace delta with complete message
				choice["message"] = map[string]interface{}{
					"role":    "assistant",
					"content": aggregatedContent.String(),
				}
				delete(choice, "delta")
				choice["finish_reason"] = "stop"
			}
		}
		// Change object type from chat.completion.chunk to chat.completion
		lastChunk["object"] = "chat.completion"
	}

	// Marshal the aggregated response
	responseBytes, err := json.Marshal(lastChunk)
	if err != nil {
		return nil, err
	}

	return objects.JSONRawMessage(responseBytes), nil
}

func Marshal(v any) (objects.JSONRawMessage, error) {
	switch v := v.(type) {
	case string:
		return objects.JSONRawMessage(v), nil
	case []byte:
		return objects.JSONRawMessage(v), nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}

		return objects.JSONRawMessage(b), nil
	}
}
