package retry

import (
	"context"
	"errors"
	"time"

	"github.com/looplj/axonhub/axon/agent"
)

type Provider struct {
	inner      agent.Provider
	maxRetries int
	backoff    func(attempt int) time.Duration
}

type Option func(*Provider)

func WithMaxRetries(retries int) Option {
	return func(p *Provider) {
		if retries > 0 {
			p.maxRetries = retries
		}
	}
}

func WithBackoff(fn func(attempt int) time.Duration) Option {
	return func(p *Provider) {
		if fn != nil {
			p.backoff = fn
		}
	}
}

func DefaultBackoff(attempt int) time.Duration {
	return time.Second * time.Duration(attempt)
}

func New(p agent.Provider, opts ...Option) *Provider {
	r := &Provider{
		inner:      p,
		maxRetries: 3,
		backoff:    DefaultBackoff,
	}
	for _, opt := range opts {
		opt(r)
	}

	return r
}

func (p *Provider) Chat(ctx context.Context, model string, tools []agent.ToolDefinition, messages []agent.Message) (agent.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= p.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return agent.Response{}, ctx.Err()
			case <-time.After(p.backoff(attempt)):
			}
		}

		resp, err := p.inner.Chat(ctx, model, tools, messages)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		providerErr := &agent.ProviderError{}
		if errors.As(lastErr, &providerErr) {
			if !providerErr.IsRetryable() {
				break
			}
		}
	}

	return agent.Response{}, lastErr
}

func (p *Provider) ChatStream(ctx context.Context, model string, tools []agent.ToolDefinition, messages []agent.Message) (<-chan agent.StreamEvent, error) {
	events := make(chan agent.StreamEvent, 256)

	go func() {
		defer close(events)

		for attempt := 0; attempt <= p.maxRetries; attempt++ {
			if attempt > 0 {
				select {
				case <-ctx.Done():
					emitStreamError(events, ctx.Err())
					return
				case <-time.After(p.backoff(attempt)):
				}
			}

			stream, err := p.inner.ChatStream(ctx, model, tools, messages)
			if err != nil {
				providerErr := &agent.ProviderError{}
				if errors.As(err, &providerErr) {
					if !providerErr.IsRetryable() {
						emitStreamError(events, err)
						return
					}
				}

				continue
			}

			for ev := range stream {
				if ev.Type == agent.StreamEventError && ev.Error != nil {
					emitStreamError(events, ev.Error)
					return
				}

				events <- ev
			}

			return
		}
	}()

	return events, nil
}

func emitStreamError(events chan<- agent.StreamEvent, err error) {
	events <- agent.StreamEvent{
		Type:  agent.StreamEventError,
		Error: err,
	}
}
