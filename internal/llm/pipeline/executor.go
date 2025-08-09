package pipeline

import (
	"context"

	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// Executor interface for making HTTP requests.
//
// The Executor interface defines the methods for executing HTTP requests and handling streaming responses.
type Executor interface {
	// Do executes a HTTP request and returns a HTTP response.
	Do(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error)

	// DoStream a HTTP request with streaming response
	DoStream(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error)
}
