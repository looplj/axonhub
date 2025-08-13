package pipeline

import (
	"context"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/decorator"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// ChannelRetryable interface for transformers that support channel switching.
type ChannelRetryable interface {
	NextChannel(ctx context.Context) error
	HasMoreChannels() bool
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
func WithRetry(maxRetries int, retryDelay time.Duration, retryableErrors ...string) Option {
	return func(p *pipeline) {
		p.maxRetries = maxRetries
		p.retryDelay = retryDelay
		p.retryableErrors = retryableErrors
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
	Executor        Executor
	Inbound         transformer.Inbound
	Outbound        transformer.Outbound
	decorators      []decorator.Decorator
	maxRetries      int
	retryDelay      time.Duration
	retryableErrors []string
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

	for attempt := range maxAttempts {
		if attempt > 0 {
			log.Debug(ctx, "retrying pipeline process", log.Any("attempt", attempt))

			// Try to switch to next channel if available
			if channelRetryable, ok := p.Outbound.(ChannelRetryable); ok {
				if channelRetryable.HasMoreChannels() {
					err := channelRetryable.NextChannel(ctx)
					if err != nil {
						log.Warn(ctx, "failed to switch to next channel", log.Cause(err))
						break
					}
				} else {
					log.Debug(ctx, "no more channels available for retry")
					break
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

		// Check if error is retryable
		if !p.isRetryableError(err) {
			log.Debug(ctx, "error is not retryable", log.Cause(err))
			break
		}

		log.Warn(ctx, "pipeline process failed, will retry",
			log.Cause(err),
			log.Any("attempt", attempt),
			log.Any("maxRetries", p.maxRetries))
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

// isRetryableError checks if an error is retryable based on configuration.
func (p *pipeline) isRetryableError(err error) bool {
	if len(p.retryableErrors) == 0 {
		return true // If no specific errors configured, retry all errors
	}

	errMsg := err.Error()
	for _, retryableErr := range p.retryableErrors {
		if strings.Contains(errMsg, retryableErr) {
			return true
		}
	}

	return false
}
