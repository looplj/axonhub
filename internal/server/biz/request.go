package biz

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
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
	SystemService   *SystemService
	UsageLogService *UsageLogService
}

// NewRequestService creates a new RequestService.
func NewRequestService(systemService *SystemService, usageLogService *UsageLogService) *RequestService {
	return &RequestService{
		SystemService:   systemService,
		UsageLogService: usageLogService,
	}
}

// CreateRequest creates a new request record.
func (s *RequestService) CreateRequest(
	ctx context.Context,
	user *ent.User,
	apiKey *ent.APIKey,
	llmRequest *llm.Request,
	httpRequest *httpclient.Request,
	format llm.APIFormat,
) (*ent.Request, error) {
	// Decide whether to store the original request body
	storeRequestBody := true
	if policy, err := s.SystemService.StoragePolicy(ctx); err == nil {
		storeRequestBody = policy.StoreRequestBody
	} else {
		log.Warn(ctx, "Failed to get storage policy, defaulting to store request body", log.Cause(err))
	}

	var requestBodyBytes objects.JSONRawMessage

	if storeRequestBody {
		b, err := xjson.Marshal(httpRequest.Body)
		if err != nil {
			log.Error(ctx, "Failed to serialize request body", log.Cause(err))
			return nil, err
		}

		requestBodyBytes = b
	} // else keep nil -> stored as JSON null

	// Get source from context, default to API if not present
	source := contexts.GetSourceOrDefault(ctx, request.SourceAPI)

	client := ent.FromContext(ctx)
	mut := client.Request.Create().
		SetUser(user).
		SetModelID(llmRequest.Model).
		SetFormat(string(format)).
		SetSource(source).
		SetStatus(request.StatusProcessing).
		SetRequestBody(requestBodyBytes)

	if apiKey != nil {
		mut = mut.SetAPIKeyID(apiKey.ID)
	}

	req, err := mut.Save(ctx)
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
	format llm.APIFormat,
) (*ent.RequestExecution, error) {
	// Decide whether to store the channel request body
	storeRequestBody := true
	if policy, err := s.SystemService.StoragePolicy(ctx); err == nil {
		storeRequestBody = policy.StoreRequestBody
	} else {
		log.Warn(ctx, "Failed to get storage policy, defaulting to store request body", log.Cause(err))
	}

	var requestBodyBytes objects.JSONRawMessage

	if storeRequestBody {
		b, err := xjson.Marshal(channelRequest.Body)
		if err != nil {
			log.Error(ctx, "Failed to marshal request body", log.Cause(err))
			return nil, err
		}

		requestBodyBytes = b
	}

	client := ent.FromContext(ctx)

	return client.RequestExecution.Create().
		SetFormat(string(format)).
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
	externalId string,
	responseBody any,
) error {
	// Decide whether to store the final response body
	storeResponseBody := true
	if policy, err := s.SystemService.StoragePolicy(ctx); err == nil {
		storeResponseBody = policy.StoreResponseBody
	} else {
		log.Warn(ctx, "Failed to get storage policy, defaulting to store response body", log.Cause(err))
	}

	client := ent.FromContext(ctx)

	upd := client.Request.UpdateOneID(requestID).
		SetStatus(request.StatusCompleted).
		SetExternalID(externalId)

	if storeResponseBody {
		responseBodyBytes, err := xjson.Marshal(responseBody)
		if err != nil {
			log.Error(ctx, "Failed to serialize response body", log.Cause(err))
			return err
		}

		upd = upd.SetResponseBody(responseBodyBytes)
	}

	_, err := upd.Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to update request status to completed", log.Cause(err))
		return err
	}

	return nil
}

// UpdateRequestExecutionCompleted updates request execution status to completed with response body.
func (s *RequestService) UpdateRequestExecutionCompleted(
	ctx context.Context,
	executionID int,
	externalId string,
	responseBody any,
) error {
	// Decide whether to store the final response body for execution
	storeResponseBody := true
	if policy, err := s.SystemService.StoragePolicy(ctx); err == nil {
		storeResponseBody = policy.StoreResponseBody
	} else {
		log.Warn(ctx, "Failed to get storage policy, defaulting to store response body", log.Cause(err))
	}

	client := ent.FromContext(ctx)

	upd := client.RequestExecution.UpdateOneID(executionID).
		SetStatus(requestexecution.StatusCompleted).
		SetExternalID(externalId)

	if storeResponseBody {
		responseBodyBytes, err := xjson.Marshal(responseBody)
		if err != nil {
			return err
		}

		upd = upd.SetResponseBody(responseBodyBytes)
	}

	_, err := upd.Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to update request execution status to completed", log.Cause(err))
		return err
	}

	return nil
}

// UpdateRequestExecutionCanceled updates request execution status to canceled with error message.
func (s *RequestService) UpdateRequestExecutionCanceled(
	ctx context.Context,
	executionID int,
	errorMsg string,
) error {
	return s.UpdateRequestExecutionStatus(ctx, executionID, requestexecution.StatusCanceled, errorMsg)
}

// UpdateRequestExecutionFailed updates request execution status to failed with error message.
func (s *RequestService) UpdateRequestExecutionFailed(
	ctx context.Context,
	executionID int,
	errorMsg string,
) error {
	return s.UpdateRequestExecutionStatus(ctx, executionID, requestexecution.StatusFailed, errorMsg)
}

// UpdateRequestExecutionStatus updates request execution status to the provided value (e.g., canceled or failed), with optional error message.
func (s *RequestService) UpdateRequestExecutionStatus(
	ctx context.Context,
	executionID int,
	status requestexecution.Status,
	errorMsg string,
) error {
	client := ent.FromContext(ctx)

	upd := client.RequestExecution.UpdateOneID(executionID).
		SetStatus(status)
	if errorMsg != "" {
		upd = upd.SetErrorMessage(errorMsg)
	}

	_, err := upd.Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to update request execution status", log.Cause(err), log.Any("status", status))
		return err
	}

	return nil
}

// UpdateRequestExecutionStatusFromError updates request execution status based on error type and sets error message.
func (s *RequestService) UpdateRequestExecutionStatusFromError(ctx context.Context, executionID int, rawErr error) error {
	status := requestexecution.StatusFailed
	if errors.Is(rawErr, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		status = requestexecution.StatusCanceled
	}

	return s.UpdateRequestExecutionStatus(ctx, executionID, status, rawErr.Error())
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
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
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

	client := ent.FromContext(ctx)

	_, err = client.RequestExecution.UpdateOneID(executionID).
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
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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

	client := ent.FromContext(ctx)

	_, err = client.Request.UpdateOneID(requestID).
		AppendResponseChunks([]objects.JSONRawMessage{chunkBytes}).
		Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to append response chunk", log.Cause(err))
		return err
	}

	return nil
}

// UpdateRequestCanceled updates request status to canceled.
func (s *RequestService) UpdateRequestCanceled(ctx context.Context, requestID int) error {
	return s.UpdateRequestStatus(ctx, requestID, request.StatusCanceled)
}

// UpdateRequestFailed updates request status to failed.
func (s *RequestService) UpdateRequestFailed(ctx context.Context, requestID int) error {
	return s.UpdateRequestStatus(ctx, requestID, request.StatusFailed)
}

// UpdateRequestStatus updates request status to the provided value (e.g., canceled or failed).
func (s *RequestService) UpdateRequestStatus(ctx context.Context, requestID int, status request.Status) error {
	client := ent.FromContext(ctx)

	_, err := client.Request.UpdateOneID(requestID).
		SetStatus(status).
		Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to update request status", log.Cause(err), log.Any("status", status))
		return err
	}

	return nil
}

// UpdateRequestStatusFromError updates request status based on error type: canceled if context canceled, otherwise failed.
func (s *RequestService) UpdateRequestStatusFromError(ctx context.Context, requestID int, rawErr error) error {
	if errors.Is(rawErr, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return s.UpdateRequestStatus(ctx, requestID, request.StatusCanceled)
	}

	return s.UpdateRequestStatus(ctx, requestID, request.StatusFailed)
}

// UpdateRequestChannelID updates request with channel ID after channel selection.
func (s *RequestService) UpdateRequestChannelID(ctx context.Context, requestID int, channelID int) error {
	client := ent.FromContext(ctx)

	_, err := client.Request.UpdateOneID(requestID).
		SetChannelID(channelID).
		Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to update request channel ID", log.Cause(err))
		return err
	}

	return nil
}
