package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/pkg/xcontext"
	"github.com/looplj/axonhub/internal/server/biz"
)

// OutboundPersistentStream wraps a stream and tracks all responses for final saving to database.
// It implements the streams.Stream interface and handles persistence in the Close method.
//
//nolint:containedctx // Checked.
type OutboundPersistentStream struct {
	ctx context.Context

	RequestService  *biz.RequestService
	UsageLogService *biz.UsageLogService

	stream      streams.Stream[*httpclient.StreamEvent]
	request     *ent.Request
	requestExec *ent.RequestExecution

	transformer    transformer.Outbound
	perf           *biz.PerformanceRecord
	responseChunks []*httpclient.StreamEvent
	closed         bool
}

var _ streams.Stream[*httpclient.StreamEvent] = (*OutboundPersistentStream)(nil)

func NewOutboundPersistentStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
	request *ent.Request,
	requestExec *ent.RequestExecution,
	requestService *biz.RequestService,
	usageLogService *biz.UsageLogService,
	outboundTransformer transformer.Outbound,
	perf *biz.PerformanceRecord,
) *OutboundPersistentStream {
	return &OutboundPersistentStream{
		ctx:             ctx,
		stream:          stream,
		request:         request,
		requestExec:     requestExec,
		RequestService:  requestService,
		UsageLogService: usageLogService,
		transformer:     outboundTransformer,
		perf:            perf,
		responseChunks:  make([]*httpclient.StreamEvent, 0),
		closed:          false,
	}
}

func (ts *OutboundPersistentStream) Next() bool {
	return ts.stream.Next()
}

func (ts *OutboundPersistentStream) Current() *httpclient.StreamEvent {
	event := ts.stream.Current()
	if event != nil {
		ts.responseChunks = append(ts.responseChunks, event)

		err := ts.RequestService.AppendRequestExecutionChunk(
			ts.ctx,
			ts.requestExec.ID,
			event,
		)
		if err != nil {
			log.Warn(ts.ctx, "Failed to append request execution chunk", log.Cause(err))
		}
	}

	return event
}

func (ts *OutboundPersistentStream) Err() error {
	return ts.stream.Err()
}

func (ts *OutboundPersistentStream) Close() error {
	if ts.closed {
		return nil
	}

	ts.closed = true
	ctx := ts.ctx

	log.Debug(ctx, "Closing persistent stream", log.Int("chunk_count", len(ts.responseChunks)))

	streamErr := ts.stream.Err()
	if streamErr != nil {
		// Use context without cancellation to ensure persistence even if client canceled
		persistCtx, cancel := xcontext.DetachWithTimeout(ctx, 10*time.Second)
		defer cancel()

		if ts.requestExec != nil {
			if err := ts.RequestService.UpdateRequestExecutionStatusFromError(persistCtx, ts.requestExec.ID, streamErr); err != nil {
				log.Warn(persistCtx, "Failed to update request execution status from error", log.Cause(err))
			}
		}

		return ts.stream.Close()
	}

	// Stream completed successfully - perform final persistence
	log.Debug(ctx, "Stream completed successfully, performing final persistence")

	ts.persistResponseChunks(ctx)

	return ts.stream.Close()
}

func (ts *OutboundPersistentStream) persistResponseChunks(ctx context.Context) {
	defer func() {
		if cause := recover(); cause != nil {
			log.Warn(ctx, "Failed to persist outbound response chunks", log.Any("cause", cause))
		}
	}()

	// Update request execution with aggregated chunks
	if ts.requestExec != nil {
		// Use context without cancellation to ensure persistence even if client canceled
		persistCtx, cancel := xcontext.DetachWithTimeout(ctx, 10*time.Second)
		defer cancel()

		responseBody, meta, err := ts.transformer.AggregateStreamChunks(persistCtx, ts.responseChunks)
		if err != nil {
			log.Warn(persistCtx, "Failed to aggregate chunks using transformer", log.Cause(err))
			return
		}

		// Build latency metrics from performance record
		var metrics *biz.LatencyMetrics

		if ts.perf != nil {
			firstTokenLatencyMs, requestLatencyMs, _ := ts.perf.Calculate()

			metrics = &biz.LatencyMetrics{
				LatencyMs: &requestLatencyMs,
			}
			if ts.perf.Stream && ts.perf.FirstTokenTime != nil {
				metrics.FirstTokenLatencyMs = &firstTokenLatencyMs
			}
		}

		err = ts.RequestService.UpdateRequestExecutionCompleted(
			persistCtx,
			ts.requestExec.ID,
			meta.ID,
			responseBody,
			metrics,
		)
		if err != nil {
			log.Warn(
				persistCtx,
				"Failed to update request execution with chunks, trying basic completion",
				log.Cause(err),
			)
		}

		// Try to create usage log from aggregated response
		if usage := meta.Usage; usage != nil {
			_, err = ts.UsageLogService.CreateUsageLogFromRequest(persistCtx, ts.request, ts.requestExec, usage)
			if err != nil {
				log.Warn(persistCtx, "Failed to create usage log from request", log.Cause(err))
			}
		}
	}
}

// PersistentOutboundTransformer wraps an outbound transformer with enhanced capabilities.
type PersistentOutboundTransformer struct {
	wrapped transformer.Outbound
	state   *PersistenceState
}

// APIFormat returns the API format of the transformer.
func (p *PersistentOutboundTransformer) APIFormat() llm.APIFormat {
	return p.wrapped.APIFormat()
}

func (p *PersistentOutboundTransformer) TransformError(ctx context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	return p.wrapped.TransformError(ctx, rawErr)
}

// applyOverrideRequestBody creates a middleware that applies channel override parameters.
func applyOverrideRequestBody(outbound *PersistentOutboundTransformer) pipeline.Middleware {
	return pipeline.OnRawRequest("override-request-body", func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
		channel := outbound.GetCurrentChannel()
		if channel == nil {
			return request, nil
		}

		overrideParams := channel.GetOverrideParameters()
		if len(overrideParams) == 0 {
			return request, nil
		}

		// Apply each override parameter using sjson
		body := request.Body

		for key, value := range overrideParams {
			if strings.EqualFold(key, "stream") {
				log.Warn(ctx, "stream override parameter ignored",
					log.String("channel", channel.Name),
					log.Int("channel_id", channel.ID),
				)

				continue
			}

			var (
				overridedBody []byte
				err           error
			)

			if value == "__AXONHUB_CLEAR__" {
				overridedBody, err = sjson.DeleteBytes(body, key)
			} else {
				overridedBody, err = sjson.SetBytes(body, key, value)
			}

			if err != nil {
				log.Warn(ctx, "failed to apply override parameter",
					log.String("channel", channel.Name),
					log.String("key", key),
					log.Cause(err),
				)

				continue
			}

			body = overridedBody
		}

		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "applied override parameters",
				log.String("channel", channel.Name),
				log.Int("channel_id", channel.ID),
				log.Any("override_params", overrideParams),
				log.String("old_body", string(request.Body)),
				log.String("new_body", string(body)),
			)
		}

		request.Body = body

		return request, nil
	})
}

// applyOverrideRequestHeaders creates a middleware that applies channel override headers.
func applyOverrideRequestHeaders(outbound *PersistentOutboundTransformer) pipeline.Middleware {
	return pipeline.OnRawRequest("override-request-headers", func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
		channel := outbound.GetCurrentChannel()
		if channel == nil {
			return request, nil
		}

		overrideHeaders := channel.GetOverrideHeaders()
		if len(overrideHeaders) == 0 {
			return request, nil
		}

		// Apply each override header
		if request.Headers == nil {
			request.Headers = make(http.Header)
		}

		for _, entry := range overrideHeaders {
			if entry.Key == "" {
				log.Warn(ctx, "empty header key ignored",
					log.String("channel", channel.Name),
					log.Int("channel_id", channel.ID),
				)

				continue
			}

			// If value is __AXONHUB_CLEAR__, remove header.
			if entry.Value == "__AXONHUB_CLEAR__" {
				log.Debug(ctx, "cleared header",
					log.String("channel", channel.Name),
					log.Int("channel_id", channel.ID),
					log.String("key", entry.Key),
				)

				request.Headers.Del(entry.Key)

				continue
			}

			request.Headers.Set(entry.Key, entry.Value)

			if log.DebugEnabled(ctx) {
				log.Debug(ctx, "overrided header",
					log.String("channel", channel.Name),
					log.String("key", entry.Key),
					log.String("value", entry.Value),
				)
			}
		}

		return request, nil
	})
}

func (p *PersistentOutboundTransformer) TransformRequest(ctx context.Context, llmRequest *llm.Request) (*httpclient.Request, error) {
	// Candidates should already be selected by inbound transformer
	if len(p.state.ChannelModelCandidates) == 0 {
		return nil, errors.New("no candidates available: candidates should be selected by inbound transformer")
	}

	// Select current candidate for this attempt
	if p.state.CandidateIndex >= len(p.state.ChannelModelCandidates) {
		return nil, fmt.Errorf("%w: all candidates exhausted", biz.ErrInternal)
	}

	candidate := p.state.ChannelModelCandidates[p.state.CandidateIndex]
	p.state.CurrentCandidate = candidate
	p.state.CurrentChannel = candidate.Channel
	p.wrapped = p.state.CurrentChannel.Outbound

	// Restore original model if it was mapped.
	if p.state.OriginalModel != "" {
		llmRequest.Model = p.state.OriginalModel
	}

	log.Debug(ctx, "using candidate",
		log.String("channel", p.state.CurrentChannel.Name),
		log.String("model", llmRequest.Model),
		log.String("actual_model", candidate.ActualModel),
	)

	// Use the pre-resolved ActualModel from the candidate
	llmRequest.Model = candidate.ActualModel

	channelRequest, err := p.wrapped.TransformRequest(ctx, llmRequest)
	if err != nil {
		return nil, err
	}

	// Update request with channel ID after channel selection
	if p.state.Request != nil && p.state.Request.ChannelID == 0 {
		ctx, cancel := xcontext.DetachWithTimeout(ctx, 10*time.Second)
		defer cancel()

		err := p.state.RequestService.UpdateRequestChannelID(
			ctx,
			p.state.Request.ID,
			p.state.CurrentChannel.ID,
		)
		if err != nil {
			log.Warn(ctx, "Failed to update request channel ID", log.Cause(err))
			// Continue processing even if channel ID update fails
		}
	}

	return channelRequest, nil
}

func (p *PersistentOutboundTransformer) TransformResponse(ctx context.Context, response *httpclient.Response) (*llm.Response, error) {
	return p.wrapped.TransformResponse(ctx, response)
}

func (p *PersistentOutboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	persistentStream := NewOutboundPersistentStream(
		ctx,
		stream,
		p.state.Request,
		p.state.RequestExec,
		p.state.RequestService,
		p.state.UsageLogService,
		p.wrapped, // Pass the wrapped outbound transformer for chunk aggregation
		p.state.Perf,
	)

	return p.wrapped.TransformStream(ctx, persistentStream)
}

func (p *PersistentOutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return p.wrapped.AggregateStreamChunks(ctx, chunks)
}

// GetRequestExecution returns the current request execution.
func (p *PersistentOutboundTransformer) GetRequestExecution() *ent.RequestExecution {
	return p.state.RequestExec
}

// GetRequest returns the current request.
func (p *PersistentOutboundTransformer) GetRequest() *ent.Request {
	return p.state.Request
}

// GetCurrentChannel returns the current channel.
func (p *PersistentOutboundTransformer) GetCurrentChannel() *biz.Channel {
	return p.state.CurrentChannel
}

// HasMoreChannels returns true if there are more candidates available for retry.
func (p *PersistentOutboundTransformer) HasMoreChannels() bool {
	return p.state.CandidateIndex+1 < len(p.state.ChannelModelCandidates)
}

// NextChannel moves to the next available candidate for retry.
func (p *PersistentOutboundTransformer) NextChannel(ctx context.Context) error {
	p.state.CandidateIndex++
	if p.state.CandidateIndex >= len(p.state.ChannelModelCandidates) {
		return errors.New("no more candidates available for retry")
	}

	// Reset request execution for the new candidate
	p.state.RequestExec = nil

	candidate := p.state.ChannelModelCandidates[p.state.CandidateIndex]
	p.state.CurrentCandidate = candidate
	p.state.CurrentChannel = candidate.Channel
	p.wrapped = p.state.CurrentChannel.Outbound

	log.Debug(ctx, "switching to next candidate for retry",
		log.String("channel", p.state.CurrentChannel.Name),
		log.String("model", candidate.ActualModel),
		log.Int("index", p.state.CandidateIndex))

	return nil
}

// CanRetry returns true if the current channel can be retried.
func (p *PersistentOutboundTransformer) CanRetry(err error) bool {
	return p.state.CurrentChannel != nil && isRetryableError(err)
}

// PrepareForRetry prepares for retrying the same channel.
// This creates a new request execution for the same channel without switching channels.
func (p *PersistentOutboundTransformer) PrepareForRetry(ctx context.Context) error {
	if p.state.CurrentChannel == nil {
		return errors.New("no current channel available for same-channel retry")
	}

	// Reset request execution for the same channel retry
	p.state.RequestExec = nil

	log.Debug(ctx, "prepared same channel retry",
		log.Any("channel", p.state.CurrentChannel.Name))

	return nil
}

// CustomizeExecutor customizes the executor for the current channel.
// If the current channel has an executor, it will be used.
// Otherwise, the default executor will be used.
//
// The customized executor will be used to execute the request.
// e.g. the aws bedrock process need a custom executor to handle the request.
func (p *PersistentOutboundTransformer) CustomizeExecutor(executor pipeline.Executor) pipeline.Executor {
	// Start with the default executor, then layer customizations.
	customizedExecutor := executor
	// 1. Apply proxy settings. Test proxy override takes precedence over channel settings.
	if p.state.Proxy != nil {
		customizedExecutor = httpclient.NewHttpClientWithProxy(p.state.Proxy)
	} else if p.state.CurrentChannel.HTTPClient != nil {
		// Use the channel's own HTTP client, which is pre-configured with its proxy settings.
		customizedExecutor = p.state.CurrentChannel.HTTPClient
	}
	// 2. Allow the specific outbound transformer (e.g., for AWS signing) to further customize the client.
	if custom, ok := p.state.CurrentChannel.Outbound.(pipeline.ChannelCustomizedExecutor); ok {
		return custom.CustomizeExecutor(customizedExecutor)
	}

	return customizedExecutor
}
