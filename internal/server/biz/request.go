package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/looplj/axonhub/internal/contexts"
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
	SystemService      *SystemService
	UsageLogService    *UsageLogService
	DataStorageService *DataStorageService
}

// NewRequestService creates a new RequestService.
func NewRequestService(systemService *SystemService, usageLogService *UsageLogService, dataStorageService *DataStorageService) *RequestService {
	return &RequestService{
		SystemService:      systemService,
		UsageLogService:    usageLogService,
		DataStorageService: dataStorageService,
	}
}

// shouldUseExternalStorage checks if data should be saved to external storage.
// Returns true if the data storage is not primary (database).
func (s *RequestService) shouldUseExternalStorage(_ context.Context, ds *ent.DataStorage) bool {
	if ds == nil {
		return false
	}

	return !ds.Primary
}

// generateRequestBodyKey generates the storage key for request body.
func (s *RequestService) generateRequestBodyKey(projectID, requestID int) string {
	return fmt.Sprintf("/%d/requests/%d/request_body.json", projectID, requestID)
}

// generateResponseBodyKey generates the storage key for response body.
func (s *RequestService) generateResponseBodyKey(projectID, requestID int) string {
	return fmt.Sprintf("/%d/requests/%d/response_body.json", projectID, requestID)
}

// generateResponseChunksKey generates the storage key for response chunks.
func (s *RequestService) generateResponseChunksKey(projectID, requestID int) string {
	return fmt.Sprintf("/%d/requests/%d/response_chunks.json", projectID, requestID)
}

// generateExecutionRequestBodyKey generates the storage key for execution request body.
func (s *RequestService) generateExecutionRequestBodyKey(projectID, requestID, executionID int) string {
	return fmt.Sprintf("/%d/requests/%d/executions/%d/request_body.json", projectID, requestID, executionID)
}

// generateExecutionResponseBodyKey generates the storage key for execution response body.
func (s *RequestService) generateExecutionResponseBodyKey(projectID, requestID, executionID int) string {
	return fmt.Sprintf("/%d/requests/%d/executions/%d/response_body.json", projectID, requestID, executionID)
}

// generateExecutionResponseChunksKey generates the storage key for execution response chunks.
func (s *RequestService) generateExecutionResponseChunksKey(projectID, requestID, executionID int) string {
	return fmt.Sprintf("/%d/requests/%d/executions/%d/response_chunks.json", projectID, requestID, executionID)
}

// CreateRequest creates a new request record.
func (s *RequestService) CreateRequest(
	ctx context.Context,
	llmRequest *llm.Request,
	httpRequest *httpclient.Request,
	format llm.APIFormat,
) (*ent.Request, error) {
	// Get project ID from context.
	// If project ID is not found, use zero.
	// It will be not prsent in the admin pages,
	// e.g: test channel.
	projectID, _ := contexts.GetProjectID(ctx)

	// Decide whether to store the original request body
	storeRequestBody := true
	if policy, err := s.SystemService.StoragePolicy(ctx); err == nil {
		storeRequestBody = policy.StoreRequestBody
	} else {
		log.Warn(ctx, "Failed to get storage policy, defaulting to store request body", log.Cause(err))
	}

	var requestBodyBytes objects.JSONRawMessage = []byte("{}")

	if storeRequestBody {
		b, err := xjson.Marshal(httpRequest.Body)
		if err != nil {
			log.Error(ctx, "Failed to serialize request body", log.Cause(err))
			return nil, err
		}

		requestBodyBytes = b
	} // else keep nil -> stored as JSON null

	isStream := false
	if llmRequest.Stream != nil {
		isStream = *llmRequest.Stream
	}

	// Get default data storage
	dataStorage, err := s.DataStorageService.GetDefaultDataStorage(ctx)
	if err != nil {
		log.Warn(ctx, "Failed to get default data storage, request will be created without data storage", log.Cause(err))
	}

	client := ent.FromContext(ctx)
	mut := client.Request.Create().
		SetProjectID(projectID).
		SetModelID(llmRequest.Model).
		SetFormat(string(format)).
		SetSource(contexts.GetSourceOrDefault(ctx, request.SourceAPI)).
		SetStatus(request.StatusProcessing).
		SetStream(isStream)

	// Determine if we should store in database or external storage
	useExternalStorage := storeRequestBody && s.shouldUseExternalStorage(ctx, dataStorage)

	if useExternalStorage {
		// Set empty JSON for database, actual data will be in external storage
		mut = mut.SetRequestBody([]byte("{}"))
	} else {
		// Store in database
		mut = mut.SetRequestBody(requestBodyBytes)
	}

	if dataStorage != nil {
		mut = mut.SetDataStorageID(dataStorage.ID)
	}

	if apiKey, ok := contexts.GetAPIKey(ctx); ok && apiKey != nil {
		mut = mut.SetAPIKeyID(apiKey.ID)
	}

	if trace, ok := contexts.GetTrace(ctx); ok && trace != nil {
		mut = mut.SetTraceID(trace.ID)
	}

	// Create request
	req, err := mut.Save(ctx)
	if err != nil {
		return nil, err
	}

	// Save request body to external storage if needed
	if useExternalStorage {
		key := s.generateRequestBodyKey(projectID, req.ID)

		_, err := s.DataStorageService.SaveData(ctx, dataStorage, key, requestBodyBytes)
		if err != nil {
			log.Error(ctx, "Failed to save request body to external storage", log.Cause(err))
			// Continue anyway, don't fail the request creation
		}
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

	var requestBodyBytes objects.JSONRawMessage = []byte("{}")

	if storeRequestBody {
		if len(channelRequest.JSONBody) > 0 {
			requestBodyBytes = channelRequest.JSONBody
		} else {
			b, err := xjson.Marshal(channelRequest.Body)
			if err != nil {
				log.Error(ctx, "Failed to marshal request body", log.Cause(err))
				return nil, err
			}

			requestBodyBytes = b
		}
	}

	client := ent.FromContext(ctx)

	// Get data storage if set on request
	var dataStorage *ent.DataStorage

	if request.DataStorageID != 0 {
		var err error

		dataStorage, err = client.DataStorage.Get(ctx, request.DataStorageID)
		if err != nil {
			log.Warn(ctx, "Failed to get data storage for request execution", log.Cause(err))
		}
	}

	// Determine if we should store in database or external storage
	useExternalStorage := storeRequestBody && s.shouldUseExternalStorage(ctx, dataStorage)

	var requestBodyForDB objects.JSONRawMessage
	if useExternalStorage {
		// Set empty JSON for database, actual data will be in external storage
		requestBodyForDB = []byte("{}")
	} else {
		// Store in database
		requestBodyForDB = requestBodyBytes
	}

	mut := client.RequestExecution.Create().
		SetFormat(string(format)).
		SetRequestID(request.ID).
		SetProjectID(request.ProjectID).
		SetChannelID(channel.ID).
		SetModelID(modelID).
		SetRequestBody(requestBodyForDB).
		SetStatus(requestexecution.StatusProcessing)

	// Use the same data storage as the request
	if request.DataStorageID != 0 {
		mut = mut.SetDataStorageID(request.DataStorageID)
	}

	execution, err := mut.Save(ctx)
	if err != nil {
		return nil, err
	}

	// Save request body to external storage if needed
	if useExternalStorage {
		key := s.generateExecutionRequestBodyKey(request.ProjectID, request.ID, execution.ID)

		_, err := s.DataStorageService.SaveData(ctx, dataStorage, key, requestBodyBytes)
		if err != nil {
			log.Error(ctx, "Failed to save execution request body to external storage", log.Cause(err))
			// Continue anyway, don't fail the execution creation
		}
	}

	return execution, nil
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

	// Get the request to check data storage
	req, err := client.Request.Get(ctx, requestID)
	if err != nil {
		log.Error(ctx, "Failed to get request", log.Cause(err))
		return err
	}

	// Get data storage if set
	var dataStorage *ent.DataStorage
	if req.DataStorageID != 0 {
		dataStorage, err = client.DataStorage.Get(ctx, req.DataStorageID)
		if err != nil {
			log.Warn(ctx, "Failed to get data storage", log.Cause(err))
		}
	}

	upd := client.Request.UpdateOneID(requestID).
		SetStatus(request.StatusCompleted).
		SetExternalID(externalId)

	if storeResponseBody {
		responseBodyBytes, err := xjson.Marshal(responseBody)
		if err != nil {
			log.Error(ctx, "Failed to serialize response body", log.Cause(err))
			return err
		}

		// Check if we should use external storage
		if s.shouldUseExternalStorage(ctx, dataStorage) {
			// Save to external storage
			key := s.generateResponseBodyKey(req.ProjectID, requestID)

			_, err := s.DataStorageService.SaveData(ctx, dataStorage, key, responseBodyBytes)
			if err != nil {
				log.Error(ctx, "Failed to save response body to external storage", log.Cause(err))
				// Continue anyway
			}
		} else {
			// Store in database
			upd = upd.SetResponseBody(responseBodyBytes)
		}
	}

	_, err = upd.Save(ctx)
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

	// Get the execution to check data storage
	execution, err := client.RequestExecution.Get(ctx, executionID)
	if err != nil {
		log.Error(ctx, "Failed to get request execution", log.Cause(err))
		return err
	}

	// Get data storage if set
	var dataStorage *ent.DataStorage
	if execution.DataStorageID != 0 {
		dataStorage, err = client.DataStorage.Get(ctx, execution.DataStorageID)
		if err != nil {
			log.Warn(ctx, "Failed to get data storage", log.Cause(err))
		}
	}

	upd := client.RequestExecution.UpdateOneID(executionID).
		SetStatus(requestexecution.StatusCompleted).
		SetExternalID(externalId)

	if storeResponseBody {
		responseBodyBytes, err := xjson.Marshal(responseBody)
		if err != nil {
			return err
		}

		// Check if we should use external storage
		if s.shouldUseExternalStorage(ctx, dataStorage) {
			// Save to external storage
			key := s.generateExecutionResponseBodyKey(execution.ProjectID, execution.RequestID, executionID)

			_, err := s.DataStorageService.SaveData(ctx, dataStorage, key, responseBodyBytes)
			if err != nil {
				log.Error(ctx, "Failed to save execution response body to external storage", log.Cause(err))
				// Continue anyway
			}
		} else {
			// Store in database
			upd = upd.SetResponseBody(responseBodyBytes)
		}
	}

	_, err = upd.Save(ctx)
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

	// Get the execution to check data storage
	execution, err := client.RequestExecution.Get(ctx, executionID)
	if err != nil {
		log.Error(ctx, "Failed to get request execution", log.Cause(err))
		return err
	}

	// Get data storage if set
	var dataStorage *ent.DataStorage
	if execution.DataStorageID != 0 {
		dataStorage, err = client.DataStorage.Get(ctx, execution.DataStorageID)
		if err != nil {
			log.Warn(ctx, "Failed to get data storage", log.Cause(err))
		}
	}

	// Check if we should use external storage
	if s.shouldUseExternalStorage(ctx, dataStorage) {
		// For external storage, we need to read existing chunks, append, and save back
		// This is not ideal for streaming, but maintains consistency
		key := s.generateExecutionResponseChunksKey(execution.ProjectID, execution.RequestID, executionID)

		// Read existing chunks
		var existingChunks []objects.JSONRawMessage

		existingData, err := s.DataStorageService.LoadData(ctx, dataStorage, key)
		if err == nil && len(existingData) > 0 {
			// Parse existing chunks
			if err := json.Unmarshal(existingData, &existingChunks); err != nil {
				log.Warn(ctx, "Failed to unmarshal existing chunks, starting fresh", log.Cause(err))

				existingChunks = []objects.JSONRawMessage{}
			}
		}

		// Append new chunk
		existingChunks = append(existingChunks, chunkBytes)

		// Save back
		allChunksBytes, err := json.Marshal(existingChunks)
		if err != nil {
			log.Error(ctx, "Failed to marshal all chunks", log.Cause(err))
			return err
		}

		_, err = s.DataStorageService.SaveData(ctx, dataStorage, key, allChunksBytes)
		if err != nil {
			log.Error(ctx, "Failed to save chunks to external storage", log.Cause(err))
			return err
		}
	} else {
		// Store in database
		_, err = client.RequestExecution.UpdateOneID(executionID).
			AppendResponseChunks([]objects.JSONRawMessage{chunkBytes}).
			Save(ctx)
		if err != nil {
			log.Error(ctx, "Failed to append response chunk", log.Cause(err))
			return err
		}
	}

	return nil
}

func (s *RequestService) AppendRequestChunk(
	ctx context.Context,
	requestID int,
	chunk *httpclient.StreamEvent,
) error {
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

	// Get the request to check data storage
	req, err := client.Request.Get(ctx, requestID)
	if err != nil {
		log.Error(ctx, "Failed to get request", log.Cause(err))
		return err
	}

	// Get data storage if set
	var dataStorage *ent.DataStorage
	if req.DataStorageID != 0 {
		dataStorage, err = client.DataStorage.Get(ctx, req.DataStorageID)
		if err != nil {
			log.Warn(ctx, "Failed to get data storage", log.Cause(err))
		}
	}

	// Check if we should use external storage
	if s.shouldUseExternalStorage(ctx, dataStorage) {
		// For external storage, we need to read existing chunks, append, and save back
		key := s.generateResponseChunksKey(req.ProjectID, requestID)

		// Read existing chunks
		var existingChunks []objects.JSONRawMessage

		existingData, err := s.DataStorageService.LoadData(ctx, dataStorage, key)
		if err == nil && len(existingData) > 0 {
			// Parse existing chunks
			if err := json.Unmarshal(existingData, &existingChunks); err != nil {
				log.Warn(ctx, "Failed to unmarshal existing chunks, starting fresh", log.Cause(err))

				existingChunks = []objects.JSONRawMessage{}
			}
		}

		// Append new chunk
		existingChunks = append(existingChunks, chunkBytes)

		// Save back
		allChunksBytes, err := json.Marshal(existingChunks)
		if err != nil {
			log.Error(ctx, "Failed to marshal all chunks", log.Cause(err))
			return err
		}

		_, err = s.DataStorageService.SaveData(ctx, dataStorage, key, allChunksBytes)
		if err != nil {
			log.Error(ctx, "Failed to save chunks to external storage", log.Cause(err))
			return err
		}
	} else {
		// Store in database
		_, err = client.Request.UpdateOneID(requestID).
			AppendResponseChunks([]objects.JSONRawMessage{chunkBytes}).
			Save(ctx)
		if err != nil {
			log.Error(ctx, "Failed to append response chunk", log.Cause(err))
			return err
		}
	}

	return nil
}

// MarkRequestCanceled updates request status to canceled.
func (s *RequestService) MarkRequestCanceled(ctx context.Context, requestID int) error {
	return s.UpdateRequestStatus(ctx, requestID, request.StatusCanceled)
}

// MarkRequestFailed updates request status to failed.
func (s *RequestService) MarkRequestFailed(ctx context.Context, requestID int) error {
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
		return err
	}

	return nil
}

// LoadRequestBody returns the stored request body, loading from external storage when necessary.
func (s *RequestService) LoadRequestBody(ctx context.Context, req *ent.Request) (objects.JSONRawMessage, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	dataStorage, err := s.getDataStorage(ctx, req.DataStorageID)
	if err != nil {
		log.Warn(ctx, "Failed to get data storage for request body", log.Cause(err), log.Int("request_id", req.ID))
		return req.RequestBody, nil
	}

	if !s.shouldUseExternalStorage(ctx, dataStorage) {
		return req.RequestBody, nil
	}

	key := s.generateRequestBodyKey(req.ProjectID, req.ID)

	data, err := s.DataStorageService.LoadData(ctx, dataStorage, key)
	if err != nil {
		log.Warn(ctx, "Failed to load request body", log.Cause(err), log.Int("request_id", req.ID))
		return req.RequestBody, nil
	}

	return objects.JSONRawMessage(data), nil
}

// LoadResponseBody returns the request response body, loading from external storage when necessary.
func (s *RequestService) LoadResponseBody(ctx context.Context, req *ent.Request) (objects.JSONRawMessage, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	dataStorage, err := s.getDataStorage(ctx, req.DataStorageID)
	if err != nil {
		log.Warn(ctx, "Failed to get data storage for request response body", log.Cause(err), log.Int("request_id", req.ID))
		return req.ResponseBody, nil
	}

	if !s.shouldUseExternalStorage(ctx, dataStorage) {
		return req.ResponseBody, nil
	}

	key := s.generateResponseBodyKey(req.ProjectID, req.ID)

	data, err := s.DataStorageService.LoadData(ctx, dataStorage, key)
	if err != nil {
		if os.IsNotExist(err) {
			return req.ResponseBody, nil
		}

		log.Warn(ctx, "Failed to load request response body", log.Cause(err), log.Int("request_id", req.ID))

		return req.ResponseBody, nil
	}

	return objects.JSONRawMessage(data), nil
}

// LoadResponseChunks returns the request response chunks, loading from external storage when necessary.
func (s *RequestService) LoadResponseChunks(ctx context.Context, req *ent.Request) ([]objects.JSONRawMessage, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	dataStorage, err := s.getDataStorage(ctx, req.DataStorageID)
	if err != nil {
		log.Warn(ctx, "Failed to get data storage for request response chunks", log.Cause(err), log.Int("request_id", req.ID))
		return req.ResponseChunks, nil
	}

	if !s.shouldUseExternalStorage(ctx, dataStorage) {
		return req.ResponseChunks, nil
	}

	key := s.generateResponseChunksKey(req.ProjectID, req.ID)

	data, err := s.DataStorageService.LoadData(ctx, dataStorage, key)
	if err != nil {
		if os.IsNotExist(err) {
			return req.ResponseChunks, nil
		}

		log.Warn(ctx, "Failed to load request response chunks", log.Cause(err), log.Int("request_id", req.ID))

		return req.ResponseChunks, nil
	}

	if len(data) == 0 {
		return []objects.JSONRawMessage{}, nil
	}

	var chunks []objects.JSONRawMessage
	if err := json.Unmarshal(data, &chunks); err != nil {
		log.Warn(ctx, "Failed to unmarshal request response chunks", log.Cause(err), log.Int("request_id", req.ID))
		return req.ResponseChunks, nil
	}

	return chunks, nil
}

// LoadRequestExecutionRequestBody returns the execution request body, loading from external storage when necessary.
func (s *RequestService) LoadRequestExecutionRequestBody(ctx context.Context, exec *ent.RequestExecution) (objects.JSONRawMessage, error) {
	if exec == nil {
		return nil, fmt.Errorf("request execution is nil")
	}

	dataStorage, err := s.getDataStorage(ctx, exec.DataStorageID)
	if err != nil {
		log.Warn(ctx, "Failed to get data storage for execution request body", log.Cause(err), log.Int("execution_id", exec.ID))
		return exec.RequestBody, nil
	}

	if !s.shouldUseExternalStorage(ctx, dataStorage) {
		return exec.RequestBody, nil
	}

	key := s.generateExecutionRequestBodyKey(exec.ProjectID, exec.RequestID, exec.ID)

	data, err := s.DataStorageService.LoadData(ctx, dataStorage, key)
	if err != nil {
		if os.IsNotExist(err) {
			return exec.RequestBody, nil
		}

		log.Warn(ctx, "Failed to load request execution request body", log.Cause(err), log.Int("execution_id", exec.ID))

		return exec.RequestBody, nil
	}

	return objects.JSONRawMessage(data), nil
}

// LoadRequestExecutionResponseBody returns the execution response body, loading from external storage when necessary.
func (s *RequestService) LoadRequestExecutionResponseBody(ctx context.Context, exec *ent.RequestExecution) (objects.JSONRawMessage, error) {
	if exec == nil {
		return nil, fmt.Errorf("request execution is nil")
	}

	dataStorage, err := s.getDataStorage(ctx, exec.DataStorageID)
	if err != nil {
		log.Warn(ctx, "Failed to get data storage for execution response body", log.Cause(err), log.Int("execution_id", exec.ID))
		return exec.ResponseBody, nil
	}

	if !s.shouldUseExternalStorage(ctx, dataStorage) {
		return exec.ResponseBody, nil
	}

	key := s.generateExecutionResponseBodyKey(exec.ProjectID, exec.RequestID, exec.ID)

	data, err := s.DataStorageService.LoadData(ctx, dataStorage, key)
	if err != nil {
		if os.IsNotExist(err) {
			return exec.ResponseBody, nil
		}

		log.Warn(ctx, "Failed to load request execution response body", log.Cause(err), log.Int("execution_id", exec.ID))

		return exec.ResponseBody, nil
	}

	return objects.JSONRawMessage(data), nil
}

// LoadRequestExecutionResponseChunks returns the execution response chunks, loading from external storage when necessary.
func (s *RequestService) LoadRequestExecutionResponseChunks(ctx context.Context, exec *ent.RequestExecution) ([]objects.JSONRawMessage, error) {
	if exec == nil {
		return nil, fmt.Errorf("request execution is nil")
	}

	dataStorage, err := s.getDataStorage(ctx, exec.DataStorageID)
	if err != nil {
		log.Warn(ctx, "Failed to get data storage for execution response chunks", log.Cause(err), log.Int("execution_id", exec.ID))
		return exec.ResponseChunks, nil
	}

	if !s.shouldUseExternalStorage(ctx, dataStorage) {
		return exec.ResponseChunks, nil
	}

	key := s.generateExecutionResponseChunksKey(exec.ProjectID, exec.RequestID, exec.ID)

	data, err := s.DataStorageService.LoadData(ctx, dataStorage, key)
	if err != nil {
		if os.IsNotExist(err) {
			return exec.ResponseChunks, nil
		}

		log.Warn(ctx, "Failed to load request execution response chunks", log.Cause(err), log.Int("execution_id", exec.ID))

		return exec.ResponseChunks, nil
	}

	if len(data) == 0 {
		return []objects.JSONRawMessage{}, nil
	}

	var chunks []objects.JSONRawMessage
	if err := json.Unmarshal(data, &chunks); err != nil {
		log.Warn(ctx, "Failed to unmarshal request execution response chunks", log.Cause(err), log.Int("execution_id", exec.ID))
		return exec.ResponseChunks, nil
	}

	return chunks, nil
}

func (s *RequestService) getDataStorage(ctx context.Context, dataStorageID int) (*ent.DataStorage, error) {
	if dataStorageID == 0 {
		return s.DataStorageService.GetPrimaryDataStorage(ctx)
	}

	dataStorage, err := s.DataStorageService.GetDataStorageByID(ctx, dataStorageID)
	if err != nil {
		return nil, err
	}

	return dataStorage, nil
}

func (s *RequestService) GetTraceFirstRequest(ctx context.Context, traceID int) (*ent.Request, error) {
	client := ent.FromContext(ctx)
	if client == nil {
		return nil, fmt.Errorf("ent client not found in context")
	}

	request, err := client.Request.Query().
		Where(request.TraceIDEQ(traceID), request.StatusEQ(request.StatusCompleted)).
		Order(ent.Asc(request.FieldCreatedAt)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to get first request for trace: %w", err)
	}

	return request, nil
}

func (s *RequestService) GetTraceFirstSegment(ctx context.Context, traceID int) (*Segment, error) {
	request, err := s.GetTraceFirstRequest(ctx, traceID)
	if err != nil {
		return nil, err
	}

	if request == nil {
		return nil, nil
	}

	body, err := s.LoadRequestBody(ctx, request)
	if err != nil {
		return nil, err
	}

	request.RequestBody = body

	body, err = s.LoadResponseBody(ctx, request)
	if err != nil {
		return nil, err
	}

	request.ResponseBody = body

	return requestToSegment(ctx, request)
}
