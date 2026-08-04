package api

import (
	"context"
	"errors"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

type sseStreamReadResult struct {
	event *httpclient.StreamEvent
	err   error
	done  bool
}

// sseStreamReader keeps blocking stream reads away from the response writer loop.
type sseStreamReader struct {
	results chan sseStreamReadResult
	stop    chan struct{}
	done    chan struct{}
}

func newSSEStreamReader(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) *sseStreamReader {
	reader := &sseStreamReader{
		results: make(chan sseStreamReadResult),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}

	go reader.run(ctx, stream)

	return reader
}

// Results returns stream events and the terminal stream result.
func (reader *sseStreamReader) Results() <-chan sseStreamReadResult {
	return reader.results
}

// Stop stops delivering results and waits for the blocking reader to exit.
func (reader *sseStreamReader) Stop() {
	close(reader.stop)
	<-reader.done
}

func (reader *sseStreamReader) run(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) {
	defer close(reader.done)

	defer func() {
		if recovered := recover(); recovered != nil {
			log.Error(ctx, "Panic while reading SSE stream", log.Any("panic", recovered))
			reader.send(sseStreamReadResult{
				err:  errors.New("stream reader stopped unexpectedly"),
				done: true,
			})
		}
	}()

	for stream.Next() {
		if !reader.send(sseStreamReadResult{event: stream.Current()}) {
			return
		}
	}

	reader.send(sseStreamReadResult{err: stream.Err(), done: true})
}

func (reader *sseStreamReader) send(result sseStreamReadResult) bool {
	select {
	case reader.results <- result:
		return true
	case <-reader.stop:
		return false
	}
}
