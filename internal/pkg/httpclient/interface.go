package httpclient

import (
	"context"

	"github.com/looplj/axonhub/internal/pkg/streams"
)

// HttpClient interface for making HTTP requests.
type HttpClient interface {
	// Do executes a HTTP request and returns a HTTP response.
	Do(ctx context.Context, request *Request) (*Response, error)

	// DoStream a HTTP request with streaming response
	DoStream(ctx context.Context, request *Request) (streams.Stream[*StreamEvent], error)
}
