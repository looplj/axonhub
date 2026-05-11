package orchestrator

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
)

// isPassThroughEnabled returns true when the effective pass-through flag for the current
// channel is enabled and both the inbound and outbound API formats are identical.
//
// The effective flag is the channel-level PassThroughBody when set, otherwise it falls back
// to the global system setting. systemService may be nil; in that case only the channel-level
// setting is consulted (used by tests that exercise per-channel behavior in isolation).
func (p *PersistentOutboundTransformer) isPassThroughEnabled(ctx context.Context, systemService *biz.SystemService) bool {
	channel := p.GetCurrentChannel()
	if channel == nil {
		return false
	}

	rawReq := p.state.RawProviderRequest
	if rawReq == nil || rawReq.APIFormat == "" {
		return false
	}

	llmReq := p.state.LlmRequest
	if llmReq == nil || string(llmReq.APIFormat) != rawReq.APIFormat {
		return false
	}

	var enabled bool

	switch {
	case channel.Settings != nil && channel.Settings.PassThroughBody != nil:
		enabled = *channel.Settings.PassThroughBody
	case systemService != nil:
		global, err := systemService.PassThrough(ctx)
		if err != nil {
			log.Warn(ctx, "failed to get global pass-through setting", log.Cause(err))

			return false
		}

		enabled = global
	}

	return enabled
}

// applyPassThroughRequestBody creates a middleware that reuses the original inbound request body when
// the channel enables pass-through and the inbound and outbound API formats are identical.
// For formats that encode the selected model in the request body, the mapped llmReq.Model is
// written back into the copied raw payload so pass-through does not bypass model mapping.
// Save the actual outbound provider request so pass-through checks use the emitted API format.
func applyPassThroughRequestBody(outbound *PersistentOutboundTransformer, systemService *biz.SystemService) pipeline.Middleware {
	return pipeline.OnRawRequest("pass-through-request-body", func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
		outbound.state.RawProviderRequest = request

		if !outbound.isPassThroughEnabled(ctx, systemService) {
			return request, nil
		}

		channel := outbound.GetCurrentChannel()
		llmReq := outbound.state.LlmRequest

		log.Debug(ctx, "applying pass-through body",
			log.String("channel", channel.Name),
			log.String("api_format", request.APIFormat),
		)

		body, err := mergePassThroughRequestBody(llmReq.RawRequest.Body, llmReq.APIFormat, llmReq.Model)
		if err != nil {
			log.Warn(ctx, "failed to merge pass-through body, keeping outbound body",
				log.String("channel", channel.Name),
				log.Int("channel_id", channel.ID),
				log.Cause(err),
			)

			return request, nil
		}

		request.Body = body

		return request, nil
	})
}

func mergePassThroughRequestBody(rawBody []byte, apiFormat llm.APIFormat, model string) ([]byte, error) {
	body := append([]byte(nil), rawBody...)

	if !passThroughBodyNeedsModelPatch(apiFormat) {
		return body, nil
	}

	if model == "" {
		return body, nil
	}

	nextBody, err := sjson.SetBytes(body, "model", model)
	if err != nil {
		return nil, fmt.Errorf("set model in pass-through body: %w", err)
	}

	return nextBody, nil
}

func passThroughBodyNeedsModelPatch(apiFormat llm.APIFormat) bool {
	//nolint:exhaustive // ohter format do not need model field.
	switch apiFormat {
	case llm.APIFormatOpenAIChatCompletion,
		llm.APIFormatOpenAIResponse,
		llm.APIFormatOpenAIResponseCompact,
		llm.APIFormatOpenAIEmbedding,
		llm.APIFormatJinaEmbedding,
		llm.APIFormatJinaRerank,
		llm.APIFormatAnthropicMessage:
		return true
	default:
		return false
	}
}

// applyUserAgentPassThrough creates a middleware that applies the User-Agent pass-through setting.
func applyUserAgentPassThrough(outbound *PersistentOutboundTransformer, systemService *biz.SystemService) pipeline.Middleware {
	return pipeline.OnRawRequest("user-agent-pass-through", func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
		channel := outbound.GetCurrentChannel()
		if channel == nil {
			return request, nil
		}

		var passThroughEnabled bool
		if channel.Settings != nil && channel.Settings.PassThroughUserAgent != nil {
			passThroughEnabled = *channel.Settings.PassThroughUserAgent
		} else {
			globalPassThrough, err := systemService.UserAgentPassThrough(ctx)
			if err != nil {
				log.Warn(ctx, "failed to get global user agent pass through setting", log.Cause(err))

				passThroughEnabled = false
			} else {
				passThroughEnabled = globalPassThrough
			}
		}

		// Handle User-Agent header based on pass-through setting
		// This must be done here (before persistRequestExecution) to ensure
		// the correct User-Agent is logged in request execution records.
		if request.Headers == nil {
			request.Headers = make(http.Header)
		}

		if passThroughEnabled {
			// Pass-through enabled: use the original client's User-Agent
			if outbound.state.LlmRequest != nil && outbound.state.LlmRequest.RawRequest != nil {
				if clientUA := outbound.state.LlmRequest.RawRequest.Headers.Get("User-Agent"); clientUA != "" {
					request.Headers.Set("User-Agent", clientUA)
				}
			}
		} else {
			// Pass-through disabled: use AxonHub's default User-Agent
			request.Headers.Set("User-Agent", "axonhub/1.0")
		}

		return request, nil
	})
}

// captureRawProviderResponse stores the raw provider response on state for response pass-through.
func captureRawProviderResponse(outbound *PersistentOutboundTransformer, systemService *biz.SystemService) pipeline.Middleware {
	return pipeline.OnRawResponse("capture-raw-provider-response", func(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error) {
		if outbound.isPassThroughEnabled(ctx, systemService) {
			outbound.state.RawProviderResponse = response
		}

		return response, nil
	})
}

// applyPassThroughResponse replaces the transformed response with the raw provider response
// when PassThroughBody is enabled and the inbound/outbound API formats match.
func applyPassThroughResponse(outbound *PersistentOutboundTransformer, systemService *biz.SystemService) pipeline.Middleware {
	return pipeline.OnInboundRawResponse("pass-through-response", func(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error) {
		if !outbound.isPassThroughEnabled(ctx, systemService) {
			return response, nil
		}

		rawResp := outbound.state.RawProviderResponse
		if rawResp == nil {
			return response, nil
		}

		log.Debug(ctx, "applying pass-through response",
			log.String("channel", outbound.GetCurrentChannel().Name),
			log.String("api_format", outbound.state.RawProviderRequest.APIFormat),
		)

		return rawResp, nil
	})
}

// captureRawProviderStream fans out raw provider stream events to both the pipeline
// (for transforms and LLM middlewares like connection tracking, performance recording)
// and a pass-through channel. The pipeline receives events via pipelineCh, while
// raw events are stored on state.RawStreamCh for pass-through delivery.
func captureRawProviderStream(outbound *PersistentOutboundTransformer, systemService *biz.SystemService) pipeline.Middleware {
	return pipeline.OnRawStream("capture-raw-provider-stream", func(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error) {
		if !outbound.isPassThroughEnabled(ctx, systemService) {
			return stream, nil
		}

		channel := outbound.GetCurrentChannel()

		pipelineCh := make(chan *httpclient.StreamEvent, 64)
		rawStreamCh := make(chan *httpclient.StreamEvent, 64)
		outbound.state.RawStreamCh = rawStreamCh

		// Per-attempt local error storage: each attempt writes to its own variable so
		// concurrent defers from an abandoned goroutine and the new attempt's goroutine
		// never touch the same memory location, eliminating the data race on retries.
		var rawStreamErr error

		outbound.state.RawStreamErrRef = &rawStreamErr

		// Per-attempt cancelable context: PrepareForRetry / NextChannel call this cancel
		// to unblock the goroutine's channel sends and release the upstream HTTP connection
		// before the next attempt starts, preventing goroutine leaks.
		attemptCtx, cancel := context.WithCancel(ctx)
		var closeStreamOnce sync.Once
		closeStream := func() {
			closeStreamOnce.Do(func() {
				cancel()
				_ = stream.Close()
			})
		}
		outbound.state.RawStreamCancel = closeStream

		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Warn(ctx, "captureRawProviderStream goroutine panicked, recovering",
						log.Any("panic", r),
						log.String("channel", channel.Name),
					)
					rawStreamErr = fmt.Errorf("passthrough stream panic: %v", r)
				} else {
					rawStreamErr = stream.Err()
				}

				close(pipelineCh)
				close(rawStreamCh)
			}()
			// Ensure the context is cleaned up when the goroutine exits, regardless of
			// whether it finished naturally or was canceled by a retry.
			defer closeStream()

			for {
				select {
				case <-attemptCtx.Done():
					log.Debug(ctx, "context canceled before reading pass-through stream",
						log.String("channel", channel.Name))

					return
				default:
				}

				if !stream.Next() {
					return
				}

				event := stream.Current()
				// Use blocking sends so events are not silently dropped when a
				// consumer is slower than the upstream provider. Bail out on
				// attempt cancellation (retry) or request cancellation to avoid
				// blocking forever.
				select {
				case pipelineCh <- event:
				case <-attemptCtx.Done():
					log.Debug(ctx, "context canceled while sending pipeline event",
						log.String("channel", channel.Name))

					return
				}

				select {
				case rawStreamCh <- event:
				case <-attemptCtx.Done():
					log.Debug(ctx, "context canceled while sending pass-through event",
						log.String("channel", channel.Name))

					return
				}
			}
		}()

		return &passThroughChannelStream{ctx: ctx, ch: pipelineCh, errRef: &rawStreamErr, cancel: closeStream}, nil
	})
}

// applyPassThroughStream returns a stream of raw provider events when PassThroughBody is enabled.
// A goroutine drains the transformed pipeline stream so that LLM middlewares (connection tracking,
// performance recording, rate limit tracking) still process events.
func applyPassThroughStream(outbound *PersistentOutboundTransformer, systemService *biz.SystemService) pipeline.Middleware {
	return pipeline.OnInboundRawStream("pass-through-response-stream", func(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error) {
		if !outbound.isPassThroughEnabled(ctx, systemService) {
			return stream, nil
		}

		rawCh := outbound.state.RawStreamCh
		if rawCh == nil {
			return stream, nil
		}

		// Snapshot the current attempt's error reference. If a future retry replaces
		// state.RawStreamErrRef, this stream still reads from the correct variable.
		errRef := outbound.state.RawStreamErrRef
		cancel := outbound.state.RawStreamCancel

		channel := outbound.GetCurrentChannel()

		log.Debug(ctx, "applying pass-through stream",
			log.String("channel", channel.Name),
		)

		go func() {
			for stream.Next() {
				_ = stream.Current()
			}

			stream.Close()
		}()

		return &passThroughChannelStream{ctx: ctx, ch: rawCh, errRef: errRef, cancel: cancel}, nil
	})
}

// passThroughChannelStream wraps a channel as a Stream.
//
//nolint:containedctx // Required so Next() can observe request cancellation.
type passThroughChannelStream struct {
	ctx     context.Context
	ch      <-chan *httpclient.StreamEvent
	current *httpclient.StreamEvent
	errRef  *error
	cancel  context.CancelFunc
	once    sync.Once
}

func (s *passThroughChannelStream) Next() bool {
	if s.ctx != nil {
		select {
		case ev, ok := <-s.ch:
			if !ok {
				return false
			}

			s.current = ev

			return true
		case <-s.ctx.Done():
			_ = s.Close()

			return false
		}
	}

	ev, ok := <-s.ch
	if !ok {
		return false
	}

	s.current = ev

	return true
}

func (s *passThroughChannelStream) Current() *httpclient.StreamEvent { return s.current }

func (s *passThroughChannelStream) Err() error {
	if s.errRef != nil {
		return *s.errRef
	}

	return nil
}

func (s *passThroughChannelStream) Close() error {
	s.once.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
	})

	return nil
}
