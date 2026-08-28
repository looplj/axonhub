package orchestrator

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"syscall"

	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

const (
	// ErrTypeUpstreamError is the error type reported when the upstream connection breaks.
	ErrTypeUpstreamError = "upstream_error"
	// ErrCodeUpstreamStreamInterrupted is the stable code for an upstream connection that
	// ended before the response completed: EOF, reset, timeout or a missing terminal event.
	ErrCodeUpstreamStreamInterrupted = "upstream_stream_interrupted"
)

// IsUpstreamTransportError reports whether err is a transport-level failure of the
// upstream connection rather than an HTTP error body or a local cancellation: the
// connection ended early (io.EOF / io.ErrUnexpectedEOF), the stream never delivered a
// terminal event, or the network reported a reset or timeout.
func IsUpstreamTransportError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Already carries a status code or structured detail: nothing to classify.
	if _, ok := errors.AsType[*httpclient.Error](err); ok {
		return false
	}

	if _, ok := errors.AsType[*llm.ResponseError](err); ok {
		return false
	}

	if errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, llm.ErrStreamIncomplete) ||
		errors.Is(err, ErrStreamIncomplete) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE) {
		return true
	}

	var netErr net.Error

	return errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary())
}

// ClassifyUpstreamTransportError converts a transport-level upstream failure into a
// *llm.ResponseError with 502 semantics, a stable code and the original cause, so
// clients and the request log see a classifiable error instead of a bare
// "unexpected EOF". Any other error is returned unchanged.
func ClassifyUpstreamTransportError(err error) error {
	if !IsUpstreamTransportError(err) {
		return err
	}

	return &llm.ResponseError{
		StatusCode: http.StatusBadGateway,
		Detail: llm.ErrorDetail{
			Message: "Upstream provider closed the connection before the response completed: " + err.Error(),
			Type:    ErrTypeUpstreamError,
			Code:    ErrCodeUpstreamStreamInterrupted,
		},
		Cause: err,
	}
}

// persistRequestExecutionFailure marks an execution failed (or canceled) with a classified
// error message and status code, keeping any latency metrics captured before the failure.
func persistRequestExecutionFailure(
	ctx context.Context,
	requestService *biz.RequestService,
	executionID int,
	rawErr error,
	metrics *biz.LatencyMetrics,
) error {
	status := requestexecution.StatusFailed
	if errors.Is(rawErr, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		status = requestexecution.StatusCanceled
	}

	failure := ClassifyUpstreamTransportError(rawErr)

	return requestService.UpdateRequestExecutionStatusWithMetrics(
		ctx,
		executionID,
		status,
		ExtractErrorMessage(failure),
		ExtractErrorInfo(failure),
		metrics,
	)
}
