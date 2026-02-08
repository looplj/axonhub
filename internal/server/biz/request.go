package biz

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/eko/gocache/lib/v4/store"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/pkg/xjson"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// RequestService handles request and request execution operations.
type RequestService struct {
	*AbstractService

	SystemService      *SystemService
	UsageLogService    *UsageLogService
	DataStorageService *DataStorageService
	channelCache       xcache.Cache[int]
}

// NewRequestService creates a new RequestService.
func NewRequestService(ent *ent.Client, systemService *SystemService, usageLogService *UsageLogService, dataStorageService *DataStorageService) *RequestService {
	return &RequestService{
		AbstractService: &AbstractService{
			db: ent,
		},
		SystemService:      systemService,
		UsageLogService:    usageLogService,
		DataStorageService: dataStorageService,
		channelCache: xcache.NewFromConfig[int](xcache.Config{
			Mode: xcache.ModeMemory,
			Memory: xcache.MemoryConfig{
				Expiration: 30 * time.Minute,
			},
		}),
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

// _InvalidRequestBodyJSON returns a JSON object indicating invalid text.
var _InvalidRequestBodyJSON = objects.JSONRawMessage(`{"message":"invalid text"}`)

// GenerateRequestBodyKey generates the storage key for request body.
func GenerateRequestBodyKey(projectID, requestID int) string {
	return fmt.Sprintf("/%d/requests/%d/request_body.json", projectID, requestID)
}

// GenerateResponseBodyKey generates the storage key for response body.
func GenerateResponseBodyKey(projectID, requestID int) string {
	return fmt.Sprintf("/%d/requests/%d/response_body.json", projectID, requestID)
}

// GenerateResponseChunksKey generates the storage key for response chunks.
func GenerateResponseChunksKey(projectID, requestID int) string {
	return fmt.Sprintf("/%d/requests/%d/response_chunks.json", projectID, requestID)
}

// GenerateRequestDirKey generates the storage key for request.
func GenerateRequestDirKey(projectID, requestID int) string {
	return fmt.Sprintf("/%d/requests/%d", projectID, requestID)
}

func GenerateRequestExecutionsDirKey(projectID, requestID int) string {
	return fmt.Sprintf("/%d/requests/%d/executions", projectID, requestID)
}

// GenerateExecutionRequestBodyKey generates the storage key for execution request body.
func GenerateExecutionRequestBodyKey(projectID, requestID, executionID int) string {
	return fmt.Sprintf("/%d/requests/%d/executions/%d/request_body.json", projectID, requestID, executionID)
}

// GenerateExecutionResponseBodyKey generates the storage key for execution response body.
func GenerateExecutionResponseBodyKey(projectID, requestID, executionID int) string {
	return fmt.Sprintf("/%d/requests/%d/executions/%d/response_body.json", projectID, requestID, executionID)
}

// GenerateExecutionResponseChunksKey generates the storage key for execution response chunks.
func GenerateExecutionResponseChunksKey(projectID, requestID, executionID int) string {
	return fmt.Sprintf("/%d/requests/%d/executions/%d/response_chunks.json", projectID, requestID, executionID)
}

// GenerateExecutionRequestDirKey generates the storage key for execution request.
func GenerateExecutionRequestDirKey(projectID, requestID, executionID int) string {
	return fmt.Sprintf("/%d/requests/%d/executions/%d", projectID, requestID, executionID)
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

	var (
		requestBodyBytes    objects.JSONRawMessage = []byte("{}")
		requestHeadersBytes objects.JSONRawMessage = []byte("{}")
	)

	if storeRequestBody {
		b, err := xjson.Marshal(httpRequest.Body)
		if err != nil {
			log.Error(ctx, "Failed to serialize request body", log.Cause(err))
			return nil, err
		}

		requestBodyBytes = b

		if httpRequest != nil && len(httpRequest.Headers) > 0 {
			requestHeadersBytes, _ = xjson.Marshal(httpclient.MaskSensitiveHeaders(httpRequest.Headers))
		}
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

	client := s.entFromContext(ctx)
	mut := client.Request.Create().
		SetProjectID(projectID).
		SetModelID(llmRequest.Model).
		SetFormat(string(format)).
		SetSource(contexts.GetSourceOrDefault(ctx, request.SourceAPI)).
		SetStatus(request.StatusProcessing).
		SetStream(isStream).
		SetRequestHeaders(requestHeadersBytes)

	if httpRequest != nil && httpRequest.ClientIP != "" {
		mut = mut.SetClientIP(httpRequest.ClientIP)
	}

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
		if !useExternalStorage {
			log.Warn(ctx, "Failed to save request body due to error, retrying with placeholder", log.Cause(err))

			mut = mut.SetRequestBody(_InvalidRequestBodyJSON)

			req, err = mut.Save(ctx)
			if err != nil {
				log.Error(ctx, "Failed to save request even with placeholder", log.Cause(err))
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Save request body to external storage if needed
	if useExternalStorage {
		key := GenerateRequestBodyKey(projectID, req.ID)

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

	var (
		requestBodyBytes    objects.JSONRawMessage = []byte("{}")
		requestHeadersBytes objects.JSONRawMessage = []byte("{}")
	)

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

		if len(channelRequest.Headers) > 0 {
			requestHeadersBytes, _ = xjson.Marshal(httpclient.MaskSensitiveHeaders(channelRequest.Headers))
		}
	}

	client := s.entFromContext(ctx)

	// Get data storage if set on request
	var dataStorage *ent.DataStorage

	if request.DataStorageID != 0 {
		var err error

		dataStorage, err = s.DataStorageService.GetDataStorageByID(ctx, request.DataStorageID)
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
		SetStatus(requestexecution.StatusProcessing).
		SetStream(request.Stream).
		SetRequestHeaders(requestHeadersBytes)

	// Use the same data storage as the request
	if request.DataStorageID != 0 {
		mut = mut.SetDataStorageID(request.DataStorageID)
	}

	execution, err := mut.Save(ctx)
	if err != nil {
		if useExternalStorage {
			return nil, err
		}

		log.Warn(ctx, "Failed to save execution request body due to error, retrying with placeholder", log.Cause(err))

		mut = mut.SetRequestBody(_InvalidRequestBodyJSON)

		execution, err = mut.Save(ctx)
		if err != nil {
			log.Error(ctx, "Failed to save execution request even with placeholder", log.Cause(err))
			return nil, err
		}
	}

	// Save request body to external storage if needed
	if useExternalStorage {
		key := GenerateExecutionRequestBodyKey(request.ProjectID, request.ID, execution.ID)

		_, err := s.DataStorageService.SaveData(ctx, dataStorage, key, requestBodyBytes)
		if err != nil {
			log.Error(ctx, "Failed to save execution request body to external storage", log.Cause(err))
			// Continue anyway, don't fail the execution creation
		}
	}

	return execution, nil
}

// LatencyMetrics holds latency metrics for a request.
type LatencyMetrics struct {
	LatencyMs           *int64
	FirstTokenLatencyMs *int64
}

// UpdateRequestCompleted updates request status to completed with response body.
func (s *RequestService) UpdateRequestCompleted(
	ctx context.Context,
	requestID int,
	externalId string,
	responseBody any,
	metrics *LatencyMetrics,
) error {
	// Decide whether to store the final response body
	storeResponseBody := true
	if policy, err := s.SystemService.StoragePolicy(ctx); err == nil {
		storeResponseBody = policy.StoreResponseBody
	} else {
		log.Warn(ctx, "Failed to get storage policy, defaulting to store response body", log.Cause(err))
	}

	client := s.entFromContext(ctx)

	// Get the request to check data storage
	req, err := client.Request.Get(ctx, requestID)
	if err != nil {
		log.Error(ctx, "Failed to get request", log.Cause(err))
		return err
	}

	// Get data storage if set
	var dataStorage *ent.DataStorage
	if req.DataStorageID != 0 {
		dataStorage, err = s.DataStorageService.GetDataStorageByID(ctx, req.DataStorageID)
		if err != nil {
			log.Warn(ctx, "Failed to get data storage", log.Cause(err))
		}
	}

	upd := client.Request.UpdateOneID(requestID).
		SetStatus(request.StatusCompleted).
		SetExternalID(externalId)

	// Set latency metrics if provided
	if metrics != nil {
		if metrics.LatencyMs != nil {
			upd = upd.SetMetricsLatencyMs(*metrics.LatencyMs)
		}

		if metrics.FirstTokenLatencyMs != nil {
			upd = upd.SetMetricsFirstTokenLatencyMs(*metrics.FirstTokenLatencyMs)
		}
	}

	if storeResponseBody {
		responseBodyBytes, err := xjson.Marshal(responseBody)
		if err != nil {
			log.Error(ctx, "Failed to serialize response body", log.Cause(err))
			return err
		}

		// Check if we should use external storage
		if s.shouldUseExternalStorage(ctx, dataStorage) {
			// Save to external storage
			key := GenerateResponseBodyKey(req.ProjectID, requestID)

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
	metrics *LatencyMetrics,
) error {
	// Decide whether to store the final response body for execution
	storeResponseBody := true
	if policy, err := s.SystemService.StoragePolicy(ctx); err == nil {
		storeResponseBody = policy.StoreResponseBody
	} else {
		log.Warn(ctx, "Failed to get storage policy, defaulting to store response body", log.Cause(err))
	}

	client := s.entFromContext(ctx)

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

	// Set latency metrics if provided
	if metrics != nil {
		if metrics.LatencyMs != nil {
			upd = upd.SetMetricsLatencyMs(*metrics.LatencyMs)
		}

		if metrics.FirstTokenLatencyMs != nil {
			upd = upd.SetMetricsFirstTokenLatencyMs(*metrics.FirstTokenLatencyMs)
		}
	}

	if storeResponseBody {
		responseBodyBytes, err := xjson.Marshal(responseBody)
		if err != nil {
			return err
		}

		// Check if we should use external storage
		if s.shouldUseExternalStorage(ctx, dataStorage) {
			// Save to external storage
			key := GenerateExecutionResponseBodyKey(execution.ProjectID, execution.RequestID, executionID)

			_, err := s.DataStorageService.SaveData(ctx, dataStorage, key, responseBodyBytes)
			if err != nil {
				log.Error(ctx, "Failed to save execution response body to external storage", log.Cause(err))
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
	client := s.entFromContext(ctx)

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

// SaveRequestExecutionChunks saves all response chunks to request execution at once.
// Only stores chunks if the system StoreChunks setting is enabled.
func (s *RequestService) SaveRequestExecutionChunks(
	ctx context.Context,
	executionID int,
	chunks []*httpclient.StreamEvent,
) error {
	if len(chunks) == 0 {
		return nil
	}

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

	// Convert chunks to JSON format, filtering out done events
	var chunkBytes []objects.JSONRawMessage

	for _, chunk := range chunks {
		if bytes.Equal(chunk.Data, llm.DoneStreamEvent.Data) {
			continue
		}

		b, err := xjson.Marshal(jsonStreamEvent{
			LastEventID: chunk.LastEventID,
			Type:        chunk.Type,
			Data:        chunk.Data,
		})
		if err != nil {
			log.Warn(ctx, "Failed to marshal chunk, skipping", log.Cause(err))

			continue
		}

		chunkBytes = append(chunkBytes, b)
	}

	if len(chunkBytes) == 0 {
		return nil
	}

	client := s.entFromContext(ctx)

	// Get the execution to check data storage
	execution, err := client.RequestExecution.Get(ctx, executionID)
	if err != nil {
		return fmt.Errorf("failed to get request execution: %w", err)
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
		key := GenerateExecutionResponseChunksKey(execution.ProjectID, execution.RequestID, executionID)

		allChunksBytes, err := json.Marshal(chunkBytes)
		if err != nil {
			return fmt.Errorf("failed to marshal all chunks: %w", err)
		}

		_, err = s.DataStorageService.SaveData(ctx, dataStorage, key, allChunksBytes)
		if err != nil {
			return fmt.Errorf("failed to save chunks to external storage: %w", err)
		}
	} else {
		// Store in database
		_, err = client.RequestExecution.UpdateOneID(executionID).
			SetResponseChunks(chunkBytes).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to save response chunks: %w", err)
		}
	}

	return nil
}

// SaveRequestChunks saves all response chunks to request at once.
// Only stores chunks if the system StoreChunks setting is enabled.
func (s *RequestService) SaveRequestChunks(
	ctx context.Context,
	requestID int,
	chunks []*httpclient.StreamEvent,
) error {
	if len(chunks) == 0 {
		return nil
	}

	storeChunks, err := s.SystemService.StoreChunks(ctx)
	if err != nil {
		log.Warn(ctx, "Failed to get StoreChunks setting, defaulting to false", log.Cause(err))

		storeChunks = false
	}

	// Only store chunks if enabled
	if !storeChunks {
		return nil
	}

	// Convert chunks to JSON format, filtering out done events
	var chunkBytes []objects.JSONRawMessage

	for _, chunk := range chunks {
		if bytes.Equal(chunk.Data, llm.DoneStreamEvent.Data) {
			continue
		}

		b, err := xjson.Marshal(jsonStreamEvent{
			LastEventID: chunk.LastEventID,
			Type:        chunk.Type,
			Data:        chunk.Data,
		})
		if err != nil {
			log.Warn(ctx, "Failed to marshal chunk, skipping", log.Cause(err))

			continue
		}

		chunkBytes = append(chunkBytes, b)
	}

	if len(chunkBytes) == 0 {
		return nil
	}

	client := s.entFromContext(ctx)

	// Get the request to check data storage
	req, err := client.Request.Get(ctx, requestID)
	if err != nil {
		return fmt.Errorf("failed to get request: %w", err)
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
		key := GenerateResponseChunksKey(req.ProjectID, requestID)

		allChunksBytes, err := json.Marshal(chunkBytes)
		if err != nil {
			return fmt.Errorf("failed to marshal all chunks: %w", err)
		}

		_, err = s.DataStorageService.SaveData(ctx, dataStorage, key, allChunksBytes)
		if err != nil {
			return fmt.Errorf("failed to save chunks to external storage: %w", err)
		}
	} else {
		// Store in database
		_, err = client.Request.UpdateOneID(requestID).
			SetResponseChunks(chunkBytes).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to save response chunks: %w", err)
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
	client := s.entFromContext(ctx)

	_, err := client.Request.UpdateOneID(requestID).
		SetStatus(status).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to update request status: %w", err)
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
	client := s.entFromContext(ctx)

	request, err := client.Request.UpdateOneID(requestID).
		SetChannelID(channelID).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to update request channel ID: %w", err)
	}

	// Reset channel cache for this trace when request completes
	if request.TraceID != 0 {
		s.setLastSuccessfulChannelID(ctx, request.TraceID, channelID)
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

	key := GenerateRequestBodyKey(req.ProjectID, req.ID)

	data, err := s.DataStorageService.LoadData(ctx, dataStorage, key)
	if err != nil {
		if os.IsNotExist(err) {
			return req.RequestBody, nil
		}

		log.Warn(ctx, "Failed to load request body", log.Cause(err), log.Int("request_id", req.ID))

		return xjson.EmptyJSONRawMessage, nil
	}

	if json.Valid(data) {
		return objects.JSONRawMessage(data), nil
	}

	return xjson.EmptyJSONRawMessage, nil
}

// LoadResponseBody returns the request response body, loading from external storage when necessary.
func (s *RequestService) LoadResponseBody(ctx context.Context, req *ent.Request) (objects.JSONRawMessage, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	// Only load response body if request is completed
	if req.Status != request.StatusCompleted {
		return nil, nil
	}

	dataStorage, err := s.getDataStorage(ctx, req.DataStorageID)
	if err != nil {
		log.Warn(ctx, "Failed to get data storage for request response body", log.Cause(err), log.Int("request_id", req.ID))
		return req.ResponseBody, nil
	}

	if !s.shouldUseExternalStorage(ctx, dataStorage) {
		return req.ResponseBody, nil
	}

	key := GenerateResponseBodyKey(req.ProjectID, req.ID)

	data, err := s.DataStorageService.LoadData(ctx, dataStorage, key)
	if err != nil {
		if os.IsNotExist(err) {
			return req.ResponseBody, nil
		}

		log.Warn(ctx, "Failed to load request response body", log.Cause(err), log.Int("request_id", req.ID))

		return req.ResponseBody, nil
	}

	if json.Valid(data) {
		return objects.JSONRawMessage(data), nil
	}

	return xjson.EmptyJSONRawMessage, nil
}

// LoadResponseChunks returns the request response chunks, loading from external storage when necessary.
func (s *RequestService) LoadResponseChunks(ctx context.Context, req *ent.Request) ([]objects.JSONRawMessage, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}
	// Only load response chunks if request is completed and streaming.
	if !req.Stream || req.Status != request.StatusCompleted {
		return nil, nil
	}

	dataStorage, err := s.getDataStorage(ctx, req.DataStorageID)
	if err != nil {
		log.Warn(ctx, "Failed to get data storage for request response chunks", log.Cause(err), log.Int("request_id", req.ID))
		return req.ResponseChunks, nil
	}

	if !s.shouldUseExternalStorage(ctx, dataStorage) {
		return req.ResponseChunks, nil
	}

	key := GenerateResponseChunksKey(req.ProjectID, req.ID)

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

	key := GenerateExecutionRequestBodyKey(exec.ProjectID, exec.RequestID, exec.ID)

	data, err := s.DataStorageService.LoadData(ctx, dataStorage, key)
	if err != nil {
		if os.IsNotExist(err) {
			return exec.RequestBody, nil
		}

		log.Warn(ctx, "Failed to load request execution request body", log.Cause(err), log.Int("execution_id", exec.ID))

		return exec.RequestBody, nil
	}

	if json.Valid(data) {
		return objects.JSONRawMessage(data), nil
	}

	return xjson.EmptyJSONRawMessage, nil
}

// LoadRequestExecutionResponseBody returns the execution response body, loading from external storage when necessary.
func (s *RequestService) LoadRequestExecutionResponseBody(ctx context.Context, exec *ent.RequestExecution) (objects.JSONRawMessage, error) {
	if exec == nil {
		return nil, fmt.Errorf("request execution is nil")
	}

	// Only load response body if execution is completed
	if exec.Status != requestexecution.StatusCompleted {
		return nil, nil
	}

	dataStorage, err := s.getDataStorage(ctx, exec.DataStorageID)
	if err != nil {
		log.Warn(ctx, "Failed to get data storage for execution response body", log.Cause(err), log.Int("execution_id", exec.ID))
		return exec.ResponseBody, nil
	}

	if !s.shouldUseExternalStorage(ctx, dataStorage) {
		return exec.ResponseBody, nil
	}

	key := GenerateExecutionResponseBodyKey(exec.ProjectID, exec.RequestID, exec.ID)

	data, err := s.DataStorageService.LoadData(ctx, dataStorage, key)
	if err != nil {
		if os.IsNotExist(err) {
			return exec.ResponseBody, nil
		}

		log.Warn(ctx, "Failed to load request execution response body", log.Cause(err), log.Int("execution_id", exec.ID))

		return exec.ResponseBody, nil
	}

	if json.Valid(data) {
		return objects.JSONRawMessage(data), nil
	}

	return xjson.EmptyJSONRawMessage, nil
}

// LoadRequestExecutionResponseChunks returns the execution response chunks, loading from external storage when necessary.
func (s *RequestService) LoadRequestExecutionResponseChunks(ctx context.Context, exec *ent.RequestExecution) ([]objects.JSONRawMessage, error) {
	if exec == nil {
		return nil, fmt.Errorf("request execution is nil")
	}

	// Only load response body if execution is completed
	if !exec.Stream || exec.Status != requestexecution.StatusCompleted {
		return nil, nil
	}

	dataStorage, err := s.getDataStorage(ctx, exec.DataStorageID)
	if err != nil {
		log.Warn(ctx, "Failed to get data storage for execution response chunks", log.Cause(err), log.Int("execution_id", exec.ID))
		return exec.ResponseChunks, nil
	}

	if !s.shouldUseExternalStorage(ctx, dataStorage) {
		return exec.ResponseChunks, nil
	}

	key := GenerateExecutionResponseChunksKey(exec.ProjectID, exec.RequestID, exec.ID)

	data, err := s.DataStorageService.LoadData(ctx, dataStorage, key)
	if err != nil {
		if os.IsNotExist(err) {
			return exec.ResponseChunks, nil
		}

		log.Warn(ctx, "Failed to load request execution response chunks", log.Cause(err), log.Int("execution_id", exec.ID))

		return exec.ResponseChunks, nil
	}

	if json.Valid(data) {
		var chunks []objects.JSONRawMessage
		if err := json.Unmarshal(data, &chunks); err != nil {
			log.Warn(ctx, "Failed to unmarshal request execution response chunks", log.Cause(err), log.Int("execution_id", exec.ID))
			return exec.ResponseChunks, nil
		}

		return chunks, nil
	}

	return []objects.JSONRawMessage{}, nil
}

func (s *RequestService) GetTraceFirstRequest(ctx context.Context, traceID int) (*ent.Request, error) {
	client := s.entFromContext(ctx)
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

// GetLastSuccessfulChannelID retrieves the last successful channel ID from a trace.
// Returns 0 if no successful channel is found.
func (s *RequestService) GetLastSuccessfulChannelID(ctx context.Context, traceID int) (int, error) {
	// Try cache first
	cacheKey := buildLastChannelCacheKey(traceID)
	if channelID, err := s.channelCache.Get(ctx, cacheKey); err == nil {
		return channelID, nil
	}

	req, err := s.entFromContext(ctx).Request.Query().
		Where(
			request.TraceIDEQ(traceID),
			// Only successful requests
			request.StatusEQ(request.StatusCompleted),
			// Must have a channel
			request.ChannelIDNotNil(),
		).
		Order(ent.Desc(request.FieldCreatedAt)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			// Cache the zero result
			_ = s.channelCache.Set(ctx, cacheKey, 0, store.WithExpiration(5*time.Second))
			return 0, nil
		}

		return 0, fmt.Errorf("failed to query last successful request: %w", err)
	}

	// Cache the result
	s.setLastSuccessfulChannelID(ctx, traceID, req.ChannelID)

	return req.ChannelID, nil
}

func (s *RequestService) setLastSuccessfulChannelID(ctx context.Context, traceID, channelID int) {
	cacheKey := buildLastChannelCacheKey(traceID)
	_ = s.channelCache.Set(ctx, cacheKey, channelID, store.WithExpiration(1*time.Minute))
}

func buildLastChannelCacheKey(traceID int) string {
	return fmt.Sprintf("last_channel:%d", traceID)
}
