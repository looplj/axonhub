package biz

import (
	"context"
	"encoding/json"

	"github.com/samber/lo"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xjson"
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
	llmRequest *llm.Request,
	httpRequest *httpclient.Request,
	format string,
) (*ent.Request, error) {
	requestBodyBytes, err := xjson.Marshal(httpRequest.Body)
	if err != nil {
		log.Error(ctx, "Failed to serialize request body", log.Cause(err))
		return nil, err
	}

	// Create request record
	req, err := s.EntClient.Request.Create().
		SetAPIKey(lo.TernaryF(apiKey != nil, func() *ent.APIKey { return apiKey }, func() *ent.APIKey { return nil })).
		SetUserID(lo.TernaryF(apiKey != nil, func() int { return apiKey.UserID }, func() int { return 0 })).
		SetModelID(llmRequest.Model).
		SetFormat(format).
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
	modelID string,
	request *ent.Request,
	channelRequest httpclient.Request,
	format string,
) (*ent.RequestExecution, error) {
	requestBodyBytes, err := xjson.Marshal(channelRequest.Body)
	if err != nil {
		log.Error(ctx, "Failed to marshal request body", log.Cause(err))
		return nil, err
	}

	return s.EntClient.RequestExecution.Create().
		SetFormat(format).
		SetRequestID(request.ID).
		SetUserID(request.UserID).
		SetChannelID(channel.ID).
		SetModelID(modelID).
		SetRequestBody(requestBodyBytes).
		SetStatus(requestexecution.StatusProcessing).
		Save(ctx)
}

// UpdateRequestCompleted updates request status to completed with response body.
func (s *RequestService) UpdateRequestCompleted(
	ctx context.Context,
	requestID int,
	responseBody any,
) error {
	responseBodyBytes, err := xjson.Marshal(responseBody)
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
	responseBodyBytes, err := xjson.Marshal(responseBody)
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

type jsonStreamEvent struct {
	LastEventID string          `json:"last_event_id,omitempty"`
	Type        string          `json:"event"`
	Data        json.RawMessage `json:"data"`
}

// AppendRequestExecutionChunk appends a response chunk to request execution.
// Only stores chunks if the system StoreChunks setting is enabled.
func (s *RequestService) AppendRequestExecutionChunk(
	ctx context.Context,
	executionID int,
	chunk *httpclient.StreamEvent,
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

	chunkBytes, err := xjson.Marshal(jsonStreamEvent{
		LastEventID: chunk.LastEventID,
		Type:        chunk.Type,
		Data:        chunk.Data,
	})
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

func (s *RequestService) AppendRequestChunk(
	ctx context.Context,
	requestID int,
	chunk *httpclient.StreamEvent,
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

	chunkBytes, err := xjson.Marshal(jsonStreamEvent{
		LastEventID: chunk.LastEventID,
		Type:        chunk.Type,
		Data:        chunk.Data,
	})
	if err != nil {
		log.Error(ctx, "Failed to marshal chunk", log.Cause(err))
		return err
	}

	_, err = s.EntClient.Request.UpdateOneID(requestID).
		AppendResponseChunks([]objects.JSONRawMessage{chunkBytes}).
		Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to append response chunk", log.Cause(err))
		return err
	}

	return nil
}

func (s *RequestService) UpdateRequestExecutionCompletd(
	ctx context.Context,
	executionID int,
	responseBody any,
) error {
	responseBodyBytes, err := xjson.Marshal(responseBody)
	if err != nil {
		log.Error(ctx, "Failed to marshal response body", log.Cause(err))
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
