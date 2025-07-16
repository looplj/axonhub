package biz

import (
	"context"
	"encoding/json"

	"github.com/looplj/axonhub/ent"
	"github.com/looplj/axonhub/ent/request"
	"github.com/looplj/axonhub/ent/requestexecution"
	"github.com/looplj/axonhub/log"
	"github.com/looplj/axonhub/objects"
)

// RequestService handles request and request execution operations
type RequestService struct {
	EntClient *ent.Client
}

// NewRequestService creates a new RequestService
func NewRequestService(entClient *ent.Client) *RequestService {
	return &RequestService{
		EntClient: entClient,
	}
}

// CreateRequest creates a new request record
func (s *RequestService) CreateRequest(ctx context.Context, apiKey *ent.APIKey, requestBody any) (*ent.Request, error) {
	requestBodyBytes, err := Marshal(requestBody)
	if err != nil {
		log.Error(ctx, "Failed to serialize request body", log.Cause(err))
		return nil, err
	}

	// Create request record
	req, err := s.EntClient.Request.Create().
		SetAPIKey(apiKey).
		SetUserID(apiKey.UserID).
		SetStatus(request.StatusProcessing).
		SetRequestBody(requestBodyBytes).
		Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to create request", log.Cause(err))
		return nil, err
	}

	return req, nil
}

// CreateRequestExecution creates a new request execution record
func (s *RequestService) CreateRequestExecution(ctx context.Context, req *ent.Request, channel *ent.Channel, requestBody any) (*ent.RequestExecution, error) {
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

// UpdateRequestCompleted updates request status to completed with response body
func (s *RequestService) UpdateRequestCompleted(ctx context.Context, requestID int, responseBody any) error {
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

// UpdateRequestFailed updates request status to failed
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

// UpdateRequestExecutionCompleted updates request execution status to completed with response body
func (s *RequestService) UpdateRequestExecutionCompleted(ctx context.Context, executionID int, responseBody any) error {
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

// UpdateRequestExecutionFailed updates request execution status to failed with error message
func (s *RequestService) UpdateRequestExecutionFailed(ctx context.Context, executionID int, errorMsg string) error {
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
