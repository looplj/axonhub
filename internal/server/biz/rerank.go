package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/samber/lo"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xcontext"
)

// RerankChannelSelector defines the interface for selecting channels for rerank.
type RerankChannelSelector interface {
	Select(ctx context.Context, req *llm.RerankRequest) ([]*Channel, error)
}

// DefaultRerankChannelSelector selects channels that support the requested model.
type DefaultRerankChannelSelector struct {
	channelService *ChannelService
}

// NewDefaultRerankChannelSelector creates a basic selector that returns all enabled channels supporting the model.
func NewDefaultRerankChannelSelector(channelService *ChannelService) *DefaultRerankChannelSelector {
	return &DefaultRerankChannelSelector{
		channelService: channelService,
	}
}

// Select returns all enabled channels that support the given model.
func (s *DefaultRerankChannelSelector) Select(ctx context.Context, req *llm.RerankRequest) ([]*Channel, error) {
	var channels []*Channel

	for _, ch := range s.channelService.EnabledChannels {
		if ch.IsModelSupported(req.Model) {
			channels = append(channels, ch)
		}
	}

	return channels, nil
}

// LoadBalancedRerankChannelSelector wraps a selector and applies load balancing.
type LoadBalancedRerankChannelSelector struct {
	wrapped      RerankChannelSelector
	loadBalancer RerankLoadBalancer
}

// RerankLoadBalancer defines the interface for load balancing rerank channels.
type RerankLoadBalancer interface {
	Sort(ctx context.Context, channels []*Channel, model string) []*Channel
}

// NewLoadBalancedRerankChannelSelector creates a load-balanced selector.
func NewLoadBalancedRerankChannelSelector(wrapped RerankChannelSelector, loadBalancer RerankLoadBalancer) *LoadBalancedRerankChannelSelector {
	return &LoadBalancedRerankChannelSelector{
		wrapped:      wrapped,
		loadBalancer: loadBalancer,
	}
}

// Select returns channels sorted by load balancing priority.
func (s *LoadBalancedRerankChannelSelector) Select(ctx context.Context, req *llm.RerankRequest) ([]*Channel, error) {
	channels, err := s.wrapped.Select(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(channels) <= 1 {
		return channels, nil
	}

	// Apply load balancing to sort channels by priority
	sortedChannels := s.loadBalancer.Sort(ctx, channels, req.Model)

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "Load balanced rerank channels",
			log.String("model", req.Model),
			log.Int("total_channels", len(channels)),
			log.Int("selected_channels", len(sortedChannels)))
	}

	return sortedChannels, nil
}

type RerankServiceParams struct {
	fx.In

	ChannelService  *ChannelService
	RequestService  *RequestService
	SystemService   *SystemService
	ChannelSelector RerankChannelSelector `optional:"true"`
}

func NewRerankService(params RerankServiceParams) *RerankService {
	// Use default selector if not provided
	channelSelector := params.ChannelSelector
	if channelSelector == nil {
		channelSelector = NewDefaultRerankChannelSelector(params.ChannelService)
	}

	return &RerankService{
		channelService:  params.ChannelService,
		requestService:  params.RequestService,
		systemService:   params.SystemService,
		channelSelector: channelSelector,
	}
}

type RerankService struct {
	channelService  *ChannelService
	requestService  *RequestService
	systemService   *SystemService
	channelSelector RerankChannelSelector
}

// Rerank performs document reranking using the specified model and channel.
func (s *RerankService) Rerank(ctx context.Context, req *llm.RerankRequest) (*llm.RerankResponse, int, error) {
	startTime := time.Now()

	// Validate request
	if err := s.validateRequest(req); err != nil {
		return nil, http.StatusBadRequest, err
	}

	// Select channels using the channel selector
	channels, err := s.channelSelector.Select(ctx, req)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to select channels: %w", err)
	}

	if len(channels) == 0 {
		return nil, http.StatusBadRequest, fmt.Errorf("no channel supports model: %s", req.Model)
	}

	// Get retry policy from system settings
	retryPolicy := s.systemService.RetryPolicyOrDefault(ctx)

	maxRetries := 1
	if retryPolicy.Enabled {
		maxRetries = lo.Min([]int{retryPolicy.MaxChannelRetries, len(channels)})
	}

	// Create request record for logging
	reqBody := marshalJSONWithFallback(ctx, req)
	llmRequest := &llm.Request{
		Model:  req.Model,
		Stream: lo.ToPtr(false),
	}
	httpRequest := &httpclient.Request{
		Body: reqBody,
	}

	requestRecord, err := s.requestService.CreateRequest(ctx, llmRequest, httpRequest, llm.APIFormatOpenAIRerank)
	if err != nil {
		log.Warn(ctx, "failed to create request record", log.Cause(err))
	}

	var (
		lastErr        error
		lastStatusCode int
	)

	// Try channels with retry mechanism

	for attempt := range maxRetries {
		channelIndex := attempt % len(channels)
		selectedChannel := channels[channelIndex]

		resp, statusCode, err := s.tryChannel(ctx, req, selectedChannel, requestRecord)
		if err == nil {
			// Success - update request status
			s.updateRequestSuccess(ctx, requestRecord, resp, startTime)

			return resp, http.StatusOK, nil
		}

		lastErr = err
		lastStatusCode = statusCode

		log.Warn(ctx, "rerank attempt failed",
			log.Int("attempt", attempt+1),
			log.Int("channel_id", selectedChannel.ID),
			log.String("channel_name", selectedChannel.Name),
			log.Cause(err))

		// Don't retry on client errors (4xx)
		if statusCode >= 400 && statusCode < 500 {
			break
		}

		// Add delay between retries
		if attempt < maxRetries-1 && retryPolicy.RetryDelayMs > 0 {
			time.Sleep(time.Duration(retryPolicy.RetryDelayMs) * time.Millisecond)
		}
	}

	// All attempts failed - update request status
	s.updateRequestFailed(ctx, requestRecord, lastErr)

	return nil, lastStatusCode, lastErr
}

// validateRequest validates the rerank request parameters.
func (s *RerankService) validateRequest(req *llm.RerankRequest) error {
	if req == nil {
		return fmt.Errorf("rerank request is nil")
	}

	if req.Model == "" {
		return fmt.Errorf("model is required")
	}

	if req.Query == "" {
		return fmt.Errorf("query is required")
	}

	if len(req.Documents) == 0 {
		return fmt.Errorf("documents are required")
	}

	// Validate top_n if provided
	if req.TopN != nil {
		if *req.TopN <= 0 {
			return fmt.Errorf("top_n must be a positive integer")
		}

		if *req.TopN > len(req.Documents) {
			return fmt.Errorf("top_n (%d) cannot exceed the number of documents (%d)", *req.TopN, len(req.Documents))
		}
	}

	// Validate documents are not empty strings
	for i, doc := range req.Documents {
		if doc == "" {
			return fmt.Errorf("document at index %d is empty", i)
		}
	}

	return nil
}

// tryChannel attempts to execute rerank on a specific channel.
func (s *RerankService) tryChannel(
	ctx context.Context,
	req *llm.RerankRequest,
	channel *Channel,
	requestRecord *ent.Request,
) (*llm.RerankResponse, int, error) {
	// Check if the transformer supports Rerank
	rerankTransformer, ok := channel.Outbound.(transformer.Transformer)
	if !ok {
		return nil, http.StatusInternalServerError, fmt.Errorf("channel transformer does not support rerank operation")
	}

	// Apply model mapping
	actualModel, err := channel.ChooseModel(req.Model)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("model mapping failed: %w", err)
	}

	// Create a copy of request with mapped model
	mappedReq := &llm.RerankRequest{
		Model:     actualModel,
		Query:     req.Query,
		Documents: req.Documents,
		TopN:      req.TopN,
	}

	log.Debug(ctx, "selected channel for rerank",
		log.String("requested_model", req.Model),
		log.String("actual_model", actualModel),
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name))

	// Update request channel_id
	if requestRecord != nil {
		persistCtx, cancel := xcontext.DetachWithTimeout(ctx, time.Second*5)
		defer cancel()

		if err := s.requestService.UpdateRequestChannelID(persistCtx, requestRecord.ID, channel.ID); err != nil {
			log.Warn(persistCtx, "failed to update request channel_id", log.Cause(err))
		}
	}

	// Create execution record
	var executionRecord *ent.RequestExecution

	if requestRecord != nil {
		// Marshal the mapped request body to record the actual request sent to the channel
		mappedReqBody := marshalJSONWithFallback(ctx, mappedReq)
		channelRequest := httpclient.Request{
			Body: mappedReqBody,
		}

		executionRecord, err = s.requestService.CreateRequestExecution(
			ctx,
			channel,
			actualModel,
			requestRecord,
			channelRequest,
			llm.APIFormatOpenAIRerank,
		)
		if err != nil {
			log.Warn(ctx, "failed to create request execution record", log.Cause(err))
		}
	}

	// Call the transformer's Rerank method with channel's HTTP client
	resp, err := rerankTransformer.Rerank(ctx, mappedReq, channel.HTTPClient)
	if err != nil {
		statusCode := http.StatusInternalServerError

		// Extract status code from RerankError
		var rerankErr *openai.RerankError
		if errors.As(err, &rerankErr) {
			statusCode = rerankErr.StatusCode
		}

		// Update execution status
		if executionRecord != nil {
			persistCtx, cancel := xcontext.DetachWithTimeout(ctx, time.Second*5)
			defer cancel()

			if updateErr := s.requestService.UpdateRequestExecutionStatusFromError(persistCtx, executionRecord.ID, err); updateErr != nil {
				log.Warn(persistCtx, "failed to update execution status", log.Cause(updateErr))
			}
		}

		return nil, statusCode, err
	}

	// Update execution status to completed
	if executionRecord != nil {
		persistCtx, cancel := xcontext.DetachWithTimeout(ctx, time.Second*5)
		defer cancel()

		if updateErr := s.requestService.UpdateRequestExecutionCompleted(persistCtx, executionRecord.ID, "", resp, nil); updateErr != nil {
			log.Warn(persistCtx, "failed to update execution completed", log.Cause(updateErr))
		}
	}

	return resp, http.StatusOK, nil
}

// updateRequestSuccess updates request record on successful completion.
func (s *RerankService) updateRequestSuccess(
	ctx context.Context,
	requestRecord *ent.Request,
	resp *llm.RerankResponse,
	startTime time.Time,
) {
	latency := time.Since(startTime)

	log.Debug(ctx, "rerank request completed",
		log.Int("num_results", len(resp.Results)),
		log.Duration("latency", latency))

	if requestRecord != nil {
		persistCtx, cancel := xcontext.DetachWithTimeout(ctx, time.Second*5)
		defer cancel()

		if err := s.requestService.UpdateRequestCompleted(persistCtx, requestRecord.ID, "", resp, &LatencyMetrics{
			LatencyMs: lo.ToPtr(latency.Milliseconds()),
		}); err != nil {
			log.Warn(persistCtx, "failed to update request completed", log.Cause(err))
		}
	}
}

// updateRequestFailed updates request record on failure.
func (s *RerankService) updateRequestFailed(ctx context.Context, requestRecord *ent.Request, err error) {
	if requestRecord != nil {
		persistCtx, cancel := xcontext.DetachWithTimeout(ctx, time.Second*5)
		defer cancel()

		if updateErr := s.requestService.UpdateRequestStatusFromError(persistCtx, requestRecord.ID, err); updateErr != nil {
			log.Warn(persistCtx, "failed to update request status", log.Cause(updateErr))
		}
	}
}

// marshalJSONWithFallback marshals v to JSON bytes. It returns an empty JSON object
// and logs a warning if an error occurs.
func marshalJSONWithFallback(ctx context.Context, v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		log.Warn(ctx, "failed to marshal JSON, falling back to empty object", log.Cause(err))
		return []byte("{}")
	}

	return data
}
