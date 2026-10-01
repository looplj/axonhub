package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestRetryTransportClassification(t *testing.T) {
	// Given errors as surfaced by HTTP execution, stream reads and error wrapping.
	reset := &url.Error{Op: "Post", URL: "https://example.invalid", Err: &net.OpError{
		Op: "read", Net: "tcp", Err: &os.SyscallError{Syscall: "read", Err: syscall.ECONNRESET},
	}}
	pipe := fmt.Errorf("write upstream request: %w", &net.OpError{
		Op: "write", Net: "tcp", Err: &os.SyscallError{Syscall: "write", Err: syscall.EPIPE},
	})
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"wrapped connection reset", reset, true},
		{"wrapped broken pipe", pipe, true},
		{"EOF", io.EOF, true},
		{"unexpected EOF", io.ErrUnexpectedEOF, true},
		{"LLM incomplete stream", llm.ErrStreamIncomplete, true},
		{"orchestrator incomplete stream", ErrStreamIncomplete, true},
		{"network timeout", &net.DNSError{IsTimeout: true}, true},
		{"temporary network error", &net.DNSError{IsTemporary: true}, true},
		{"wrapped cancellation", fmt.Errorf("read: %w", context.Canceled), false},
		{"wrapped deadline", &url.Error{Op: "Post", Err: context.DeadlineExceeded}, false},
		{"cancellation with reset", errors.Join(context.Canceled, reset), false},
		{"deadline with EOF", errors.Join(context.DeadlineExceeded, io.EOF), false},
		{"wrapped HTTP 400", fmt.Errorf("upstream: %w", &httpclient.Error{StatusCode: http.StatusBadRequest}), false},
		{"HTTP 400 with reset", errors.Join(&httpclient.Error{StatusCode: http.StatusBadRequest}, reset), false},
		{"response 400 with EOF cause", fmt.Errorf("upstream: %w", &llm.ResponseError{StatusCode: http.StatusBadRequest, Cause: io.EOF}), false},
		{"response 401 with pipe cause", fmt.Errorf("upstream: %w", &llm.ResponseError{StatusCode: http.StatusUnauthorized, Cause: pipe}), false},
		{"HTTP 503", &httpclient.Error{StatusCode: http.StatusServiceUnavailable}, true},
		{"classified reset 502", ClassifyUpstreamTransportError(reset), true},
		{"ordinary error", errors.New("invalid request"), false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// When / Then: both entry points use the same transport exclusions.
			require.Equal(t, tc.want, isRetryableError(tc.err))
			require.Equal(t, tc.want, isRetryableErrorForChannel(tc.err, nil))
			outbound := &PersistentOutboundTransformer{state: &PersistenceState{
				CurrentCandidate: &ChannelModelsCandidate{
					Channel: &biz.Channel{Channel: &ent.Channel{}},
					Models:  []biz.ChannelModelEntry{{ActualModel: "model"}},
				},
			}}
			require.Equal(t, tc.want, outbound.CanRetry(tc.err))
		})
	}
}

// The executor injects transport errors at the wire boundary while the production
// pipeline, protocol transformers and orchestrator retry methods run unchanged.
type transportRetryExecutor struct {
	failures    int
	err         error
	first       []*httpclient.StreamEvent
	attemptURLs []string
}

func (*transportRetryExecutor) Do(context.Context, *httpclient.Request) (*httpclient.Response, error) {
	return nil, errors.New("unexpected non-stream request")
}

func (e *transportRetryExecutor) DoStream(_ context.Context, req *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	e.attemptURLs = append(e.attemptURLs, req.URL)
	if len(e.attemptURLs) <= e.failures {
		return &errorAfterEventsStream{items: e.first, err: e.err}, nil
	}
	return streams.SliceStream([]*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_success","object":"response","model":"model","status":"in_progress","output":[]}}`)},
		{Type: "response.output_text.delta", Data: []byte(`{"type":"response.output_text.delta","item_id":"msg_success","output_index":0,"content_index":0,"delta":"recovered"}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_success","object":"response","model":"model","status":"completed","output":[]}}`)},
	}), nil
}

func TestRetryTransportPipeline(t *testing.T) {
	metadata := &httpclient.StreamEvent{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_failed","object":"response","model":"model","status":"in_progress","output":[]}}`)}
	content := &httpclient.StreamEvent{Type: "response.output_text.delta", Data: []byte(`{"type":"response.output_text.delta","item_id":"msg_partial","output_index":0,"content_index":0,"delta":"partial"}`)}
	cases := []struct {
		name       string
		err        error
		failures   int
		switches   int
		content    bool
		wantError  bool
		wantRoutes []string
	}{
		{"same channel reset before content", syscall.ECONNRESET, 1, 0, false, false, []string{"first", "first"}},
		{"same channel pipe before content", syscall.EPIPE, 1, 0, false, false, []string{"first", "first"}},
		{"cross channel reset after same channel exhaustion", syscall.ECONNRESET, 2, 1, false, false, []string{"first", "first", "second"}},
		{"cross channel pipe after same channel exhaustion", syscall.EPIPE, 2, 1, false, false, []string{"first", "first", "second"}},
		{"reset after content", syscall.ECONNRESET, 1, 1, true, true, []string{"first"}},
		{"pipe after content", syscall.EPIPE, 1, 1, true, true, []string{"first"}},
		{"local cancellation", context.Canceled, 1, 0, false, true, []string{"first"}},
		{"local deadline", context.DeadlineExceeded, 1, 0, false, true, []string{"first"}},
		{"nonretryable HTTP with EOF cause", &llm.ResponseError{StatusCode: http.StatusBadRequest, Cause: io.EOF}, 1, 0, false, true, []string{"first"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Given two real Responses transformers and the production retry state.
			state := &PersistenceState{OriginalModel: "model"}
			for _, host := range []string{"first", "second"} {
				out, err := responses.NewOutboundTransformer("https://"+host+".invalid", "test-key")
				require.NoError(t, err)
				state.ChannelModelsCandidates = append(state.ChannelModelsCandidates, &ChannelModelsCandidate{
					Channel:   &biz.Channel{Channel: &ent.Channel{Name: host}, Outbound: out},
					APIFormat: string(llm.APIFormatOpenAIResponse),
					Models:    []biz.ChannelModelEntry{{RequestModel: "model", ActualModel: "model"}},
				})
			}
			executor := &transportRetryExecutor{failures: tc.failures, err: fmt.Errorf("upstream read: %w", tc.err), first: []*httpclient.StreamEvent{metadata}}
			if tc.content {
				executor.first = append(executor.first, content)
			}
			outbound := &PersistentOutboundTransformer{state: state}
			p := pipeline.NewFactory(executor).Pipeline(openai.NewInboundTransformer(), outbound, pipeline.WithRetry(tc.switches, 1, 0))

			// When: process and consume the actual client event stream.
			result, processErr := p.Process(context.Background(), buildTestRequest("model", "hello", true))
			var events []*httpclient.StreamEvent
			var streamErr error
			if processErr == nil {
				events, streamErr = streams.All(result.EventStream)
				require.NoError(t, result.EventStream.Close())
			}

			// Then: retry only before content, discard failed metadata and preserve causes.
			var payloads string
			for _, event := range events {
				payloads += string(event.Data)
			}
			if tc.wantError {
				require.ErrorIs(t, errors.Join(processErr, streamErr), tc.err)
				if tc.content {
					require.NoError(t, processErr)
					require.Contains(t, payloads, "partial")
					require.NotContains(t, payloads, "recovered")
				}
			} else {
				require.NoError(t, processErr)
				require.NoError(t, streamErr)
				require.Equal(t, 1, strings.Count(payloads, "recovered"))
				require.Equal(t, 1, strings.Count(payloads, "[DONE]"))
				require.NotContains(t, payloads, "resp_failed")
			}
			var routes []string
			for _, rawURL := range executor.attemptURLs {
				u, err := url.Parse(rawURL)
				require.NoError(t, err)
				routes = append(routes, strings.TrimSuffix(u.Hostname(), ".invalid"))
			}
			require.Equal(t, tc.wantRoutes, routes)
		})
	}
}
