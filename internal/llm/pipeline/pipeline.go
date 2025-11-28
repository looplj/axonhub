package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// Retryable interface for transformers that support channel switching.
type Retryable interface {
	// HasMoreChannels returns true if there are more channels to switch to.
	HasMoreChannels() bool

	// NextChannel switches to the next channel.
	NextChannel(ctx context.Context) error
}

// ChannelRetryable interface for transformers that support same-channel retry.
type ChannelRetryable interface {
	// CanRetry returns true if the transformer can retry for current channel given the error that occurred.
	CanRetry(err error) bool

	// PrepareForRetry prepares the transformer for retry.
	PrepareForRetry(ctx context.Context) error
}

// ChannelCustomizedExecutor interface for channel need custom the process of request.
// The customized executor will be used to execute the request.
// e.g. the aws bedrock process need a custom executor to handle the request.
type ChannelCustomizedExecutor interface {
	CustomizeExecutor(Executor) Executor
}

// Option defines a pipeline configuration option.
type Option func(*pipeline)

// WithRetry configures both cross-channel and same-channel retry behavior for the pipeline.
func WithRetry(maxRetries, maxSameChannelRetries int, retryDelay time.Duration) Option {
	return func(p *pipeline) {
		p.maxRetries = maxRetries
		p.maxSameChannelRetries = maxSameChannelRetries
		p.retryDelay = retryDelay
	}
}

// WithMiddlewares configures decorators for the pipeline.
func WithMiddlewares(decorators ...Middleware) Option {
	return func(p *pipeline) {
		p.middlewares = append(p.middlewares, decorators...)
	}
}

// Factory creates pipeline instances.
type Factory struct {
	Executor Executor
}

// NewFactory creates a new pipeline factory.
func NewFactory(executor Executor) *Factory {
	return &Factory{
		Executor: executor,
	}
}

// Pipeline creates a new pipeline with options.
func (f *Factory) Pipeline(
	inbound transformer.Inbound,
	outbound transformer.Outbound,
	opts ...Option,
) *pipeline {
	p := &pipeline{
		Executor: f.Executor,
		Inbound:  inbound,
		Outbound: outbound,
	}

	// Apply options
	for _, opt := range opts {
		opt(p)
	}

	return p
}

// pipeline implements the main pipeline logic with retry capabilities.
type pipeline struct {
	Executor              Executor
	Inbound               transformer.Inbound
	Outbound              transformer.Outbound
	middlewares           []Middleware
	maxRetries            int
	maxSameChannelRetries int
	retryDelay            time.Duration
}

type Result struct {
	// Stream indicates whether the response is a stream
	Stream bool

	// Response is the final HTTP response, if Stream is false
	Response *httpclient.Response

	// EventStream is the stream of events, if Stream is true
	EventStream streams.Stream[*httpclient.StreamEvent]
}

func (p *pipeline) applyBeforeRequestMiddlewares(ctx context.Context, request *llm.Request) (*llm.Request, error) {
	var err error

	for _, dec := range p.middlewares {
		request, err = dec.OnInboundLlmRequest(ctx, request)
		if err != nil {
			return nil, err
		}
	}

	return request, nil
}

func (p *pipeline) applyInboundRawResponseMiddlewares(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error) {
	var err error

	for _, dec := range p.middlewares {
		response, err = dec.OnInboundRawResponse(ctx, response)
		if err != nil {
			return nil, err
		}
	}

	return response, nil
}

func (p *pipeline) applyRawRequestMiddlewares(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	var err error

	for _, dec := range p.middlewares {
		request, err = dec.OnOutboundRawRequest(ctx, request)
		if err != nil {
			return nil, err
		}
	}

	return request, nil
}

func (p *pipeline) applyRawResponseMiddlewares(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error) {
	var err error

	// Response middlewares should be applied in reverse order (last to first)
	for i := len(p.middlewares) - 1; i >= 0; i-- {
		response, err = p.middlewares[i].OnOutboundRawResponse(ctx, response)
		if err != nil {
			return nil, err
		}
	}

	return response, nil
}

func (p *pipeline) applyRawStreamMiddlewares(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error) {
	var err error

	// Stream middlewares should be applied in reverse order (last to first)
	for i := len(p.middlewares) - 1; i >= 0; i-- {
		stream, err = p.middlewares[i].OnOutboundRawStream(ctx, stream)
		if err != nil {
			return nil, err
		}
	}

	return stream, nil
}

func (p *pipeline) applyRawErrorResponseMiddlewares(ctx context.Context, err error) {
	// Error response middlewares should be applied in reverse order (last to first)
	for i := len(p.middlewares) - 1; i >= 0; i-- {
		p.middlewares[i].OnOutboundRawError(ctx, err)
	}
}

func (p *pipeline) applyLlmResponseMiddlewares(ctx context.Context, response *llm.Response) (*llm.Response, error) {
	var err error

	// LLM response middlewares should be applied in reverse order (last to first)
	for i := len(p.middlewares) - 1; i >= 0; i-- {
		response, err = p.middlewares[i].OnOutboundLlmResponse(ctx, response)
		if err != nil {
			return nil, err
		}
	}

	return response, nil
}

func (p *pipeline) applyLlmStreamMiddlewares(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error) {
	var err error

	// LLM stream middlewares should be applied in reverse order (last to first)
	for i := len(p.middlewares) - 1; i >= 0; i-- {
		stream, err = p.middlewares[i].OnOutboundLlmStream(ctx, stream)
		if err != nil {
			return nil, err
		}
	}

	return stream, nil
}

func (p *pipeline) Process(ctx context.Context, request *httpclient.Request) (*Result, error) {
	// Step 1: Transform httpclient.Request to llm.Request using inbound transformer
	llmRequest, err := p.Inbound.TransformRequest(ctx, request)
	if err != nil {
		return nil, err
	}

	// Step 2: Apply before request middlewares
	llmRequest, err = p.applyBeforeRequestMiddlewares(ctx, llmRequest)
	if err != nil {
		return nil, err
	}

	llmRequest.RawRequest = request

	var lastErr error

	maxAttempts := p.maxRetries + 1 // maxRetries + initial attempt
	sameChannelAttempts := 0

	// Step 3: Process the request
	for attempt := range maxAttempts {
		if attempt > 0 {
			log.Debug(ctx, "retrying pipeline process", log.Any("attempt", attempt))

			// First try same-channel retry if available and not exhausted
			if channelRetryable, ok := p.Outbound.(ChannelRetryable); ok {
				if sameChannelAttempts < p.getMaxSameChannelRetries() && channelRetryable.CanRetry(lastErr) {
					err := channelRetryable.PrepareForRetry(ctx)
					if err != nil {
						log.Warn(ctx, "failed to prepare same channel retry", log.Cause(err))
					} else {
						sameChannelAttempts++
						log.Debug(ctx, "retrying same channel",
							log.Any("sameChannelAttempt", sameChannelAttempts),
							log.Any("maxSameChannelRetries", p.getMaxSameChannelRetries()))
					}
				} else {
					// Same-channel retries exhausted, try to switch to next channel
					if retryable, ok := p.Outbound.(Retryable); ok {
						if retryable.HasMoreChannels() {
							err := retryable.NextChannel(ctx)
							if err != nil {
								log.Warn(ctx, "failed to switch to next channel", log.Cause(err))
								break
							}

							sameChannelAttempts = 0 // Reset same-channel attempts for new channel
						} else {
							log.Debug(ctx, "no more channels available for retry")
							break
						}
					} else {
						log.Debug(ctx, "same channel retries exhausted and no channel switching available")
						break
					}
				}
			} else {
				// Fallback to channel switching if same-channel retry not supported
				if retryable, ok := p.Outbound.(Retryable); ok {
					if retryable.HasMoreChannels() {
						err := retryable.NextChannel(ctx)
						if err != nil {
							log.Warn(ctx, "failed to switch to next channel", log.Cause(err))
							break
						}
					} else {
						log.Debug(ctx, "no more channels available for retry")
						break
					}
				}
			}

			// Add retry delay if configured
			if p.retryDelay > 0 {
				time.Sleep(p.retryDelay)
			}
		}

		result, err := p.processRequest(ctx, llmRequest)
		if err == nil {
			return result, nil
		}

		lastErr = err

		log.Warn(ctx, "request process failed, will retry",
			log.Cause(err),
			log.Any("attempt", attempt),
			log.Any("maxRetries", p.maxRetries),
			log.Any("sameChannelAttempts", sameChannelAttempts))
	}

	return nil, lastErr
}

func (p *pipeline) processRequest(ctx context.Context, request *llm.Request) (*Result, error) {
	httpReq, err := p.Outbound.TransformRequest(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request: %w", err)
	}

	httpReq = httpclient.MergeInboundRequest(httpReq, request.RawRequest)

	httpReq, err = httpclient.FinalizeAuthHeaders(httpReq)
	if err != nil {
		return nil, fmt.Errorf("invalid authentication config: %w", err)
	}

	// Apply raw request middlewares
	httpReq, err = p.applyRawRequestMiddlewares(ctx, httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to apply raw request middlewares: %w", err)
	}

	executor := p.Executor
	if c, ok := p.Outbound.(ChannelCustomizedExecutor); ok {
		executor = c.CustomizeExecutor(executor)
	}

	var result *Result
	if request.Stream != nil && *request.Stream {
		result = &Result{
			Stream: true,
		}

		stream, err := p.stream(ctx, executor, httpReq)
		if err != nil {
			return nil, fmt.Errorf("failed to stream request: %w", err)
		}

		result.EventStream = stream
	} else {
		result = &Result{
			Stream: false,
		}

		response, err := p.notStream(ctx, executor, httpReq)
		if err != nil {
			return nil, err
		}

		result.Response = response
	}

	return result, nil
}

// getMaxSameChannelRetries returns the maximum number of same-channel retries.
func (p *pipeline) getMaxSameChannelRetries() int {
	return p.maxSameChannelRetries
}
