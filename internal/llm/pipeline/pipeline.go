package pipeline

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/decorator"
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

// WithRetry configures retry behavior for the pipeline.
func WithRetry(maxRetries int, retryDelay time.Duration) Option {
	return func(p *pipeline) {
		p.maxRetries = maxRetries
		p.retryDelay = retryDelay
	}
}

// WithSameChannelRetry configures same-channel retry behavior for the pipeline.
func WithSameChannelRetry(maxSameChannelRetries int) Option {
	return func(p *pipeline) {
		p.maxSameChannelRetries = maxSameChannelRetries
	}
}

// WithDecorators configures decorators for the pipeline.
func WithDecorators(decorators ...decorator.Decorator) Option {
	return func(p *pipeline) {
		p.decorators = append(p.decorators, decorators...)
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
	decorators            []decorator.Decorator
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

func (p *pipeline) Process(ctx context.Context, request *httpclient.Request) (*Result, error) {
	// Transform httpclient.Request to llm.Request using inbound transformer
	llmRequest, err := p.Inbound.TransformRequest(ctx, request)
	if err != nil {
		return nil, err
	}

	var lastErr error

	maxAttempts := p.maxRetries + 1 // maxRetries + initial attempt
	sameChannelAttempts := 0

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

		log.Warn(ctx, "pipeline process failed, will retry",
			log.Cause(err),
			log.Any("attempt", attempt),
			log.Any("maxRetries", p.maxRetries),
			log.Any("sameChannelAttempts", sameChannelAttempts))
	}

	return nil, lastErr
}

func (p *pipeline) processRequest(ctx context.Context, request *llm.Request) (*Result, error) {
	var result *Result
	if request.Stream != nil && *request.Stream {
		result = &Result{
			Stream: true,
		}

		stream, err := p.stream(ctx, request)
		if err != nil {
			return nil, err
		}

		result.EventStream = stream
	} else {
		result = &Result{
			Stream: false,
		}

		response, err := p.notStream(ctx, request)
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
