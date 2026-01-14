package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

// Retryable interface for transformers that support channel switching.
type Retryable interface {
	// HasMoreChannels returns true if there are more channels to switch to.
	// It will only be called if the attempt count is less than maxRetries.
	HasMoreChannels() bool

	// NextChannel switches to the next channel.
	// It will be called if HasMoreChannels returns true.
	NextChannel(ctx context.Context) error
}

// ChannelRetryable interface for transformers that support same-channel retry.
type ChannelRetryable interface {
	// CanRetry returns true if the transformer can retry for current channel given the error that occurred.
	// It will only be called if the attempt count is less than maxSameChannelRetries.
	CanRetry(err error) bool

	// PrepareForRetry prepares the transformer for retry.
	// It will be called if CanRetry returns true.
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
// maxChannelRetries: the maximum number of times to switch to the next channel.
// maxSameChannelRetries: the maximum number of times to retry the same channel.
// retryDelay: the delay between retries.
func WithRetry(maxChannelRetries, maxSameChannelRetries int, retryDelay time.Duration) Option {
	return func(p *pipeline) {
		p.maxChannelRetries = maxChannelRetries
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
	maxChannelRetries     int
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

	channelSwitches := 0
	sameChannelRetries := 0

	// Step 3: Process the request
	for {
		result, err := p.processRequest(ctx, llmRequest)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Stop retrying if the context is canceled or the deadline is exceeded.
		if ctx.Err() != nil {
			return nil, lastErr
		}

		// Determine retry strategy
		canRetry := false

		// 1. Try same-channel retry first if supported
		if channelRetryable, ok := p.Outbound.(ChannelRetryable); ok {
			if sameChannelRetries < p.getMaxSameChannelRetries() && channelRetryable.CanRetry(lastErr) {
				if err := channelRetryable.PrepareForRetry(ctx); err == nil {
					sameChannelRetries++
					canRetry = true

					log.Debug(ctx, "retrying same channel",
						log.Int("same_channel_attempt", sameChannelRetries),
						log.Int("max_same_channel_retries", p.getMaxSameChannelRetries()),
					)
				} else {
					log.Warn(ctx, "failed to prepare same channel retry, will try channel switch", log.Cause(err))
				}
			}
		}

		// 2. If same-channel retry not possible/exhausted, try channel switching
		if !canRetry {
			if retryable, ok := p.Outbound.(Retryable); ok {
				if channelSwitches < p.maxChannelRetries && retryable.HasMoreChannels() {
					if err := retryable.NextChannel(ctx); err == nil {
						channelSwitches++
						sameChannelRetries = 0 // Reset same-channel attempts for new channel
						canRetry = true

						log.Debug(ctx, "switched to next channel",
							log.Int("channel_switch_attempt", channelSwitches),
							log.Int("max_channel_retries", p.maxChannelRetries),
						)
					} else {
						log.Warn(ctx, "failed to switch to next channel", log.Cause(err))
					}
				}
			}
		}

		// If no retry strategy worked, break and return last error
		if !canRetry {
			break
		}

		// Add retry delay if configured
		if p.retryDelay > 0 {
			time.Sleep(p.retryDelay)
		}

		log.Warn(ctx, "request process failed, retrying...",
			log.Cause(lastErr),
			log.Int("channel_switches", channelSwitches),
			log.Int("same_channel_retries", sameChannelRetries),
		)
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
