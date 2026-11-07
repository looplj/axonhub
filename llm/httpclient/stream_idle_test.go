package httpclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStreamIdleTimeoutBeforeHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	ctx = WithStreamIdleTimeout(ctx, 50*time.Millisecond)
	_, err := NewHttpClient().DoStream(ctx, &Request{Method: http.MethodGet, URL: server.URL})
	require.ErrorIs(t, err, ErrStreamIdleTimeout)
	require.NotErrorIs(t, err, context.Canceled)
	require.NoError(t, ctx.Err(), "watchdog must cancel only the upstream attempt")
}

func TestStreamIdleTimeoutBody(t *testing.T) {
	for _, contentType := range []string{"text/event-stream", "application/octet-stream", "error-response"} {
		t.Run(contentType, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if contentType == "error-response" {
					w.WriteHeader(http.StatusBadGateway)
				} else {
					w.Header().Set("Content-Type", contentType)
				}
				w.(http.Flusher).Flush()
				<-r.Context().Done()
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			stream, err := NewHttpClient().DoStream(WithStreamIdleTimeout(ctx, 50*time.Millisecond), &Request{Method: http.MethodGet, URL: server.URL})
			if contentType == "error-response" {
				require.ErrorIs(t, err, ErrStreamIdleTimeout)
				return
			}
			require.NoError(t, err)
			defer stream.Close()
			require.False(t, stream.Next())
			require.ErrorIs(t, stream.Err(), ErrStreamIdleTimeout)
			require.NotErrorIs(t, stream.Err(), context.Canceled)
			require.False(t, stream.Next())
			require.ErrorIs(t, stream.Err(), ErrStreamIdleTimeout)
		})
	}
}

func TestStreamIdleTimeoutCountsHeartbeatAndPartialBodyBytes(t *testing.T) {
	for name, chunks := range map[string][]string{
		"heartbeats":    {": ping\n\n", ": ping\n\n", ": ping\n\n", ": ping\n\n", "data: alive\n\n"},
		"partial-event": {"data: ", "still", " ", "alive", "\n\n"},
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.(http.Flusher).Flush()
				for _, chunk := range chunks {
					select {
					case <-r.Context().Done():
						return
					case <-time.After(30 * time.Millisecond):
					}
					_, _ = fmt.Fprint(w, chunk)
					w.(http.Flusher).Flush()
				}
				<-r.Context().Done()
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			stream, err := NewHttpClient().DoStream(WithStreamIdleTimeout(ctx, 100*time.Millisecond), &Request{Method: http.MethodGet, URL: server.URL})
			require.NoError(t, err)
			defer stream.Close()
			require.True(t, stream.Next(), "body activity must survive longer than one idle interval")
			require.Contains(t, string(stream.Current().Data), "alive")
			require.False(t, stream.Next(), "stream must time out once upstream activity stops")
			require.ErrorIs(t, stream.Err(), ErrStreamIdleTimeout)
		})
	}
}

func TestStreamIdleTimeoutDisabledAndNonStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(60 * time.Millisecond):
		}
		_, _ = fmt.Fprint(w, "data: done\n\n")
	}))
	defer server.Close()
	client := NewHttpClient()
	stream, err := client.DoStream(WithStreamIdleTimeout(t.Context(), 0), &Request{Method: http.MethodGet, URL: server.URL})
	require.NoError(t, err)
	defer stream.Close()
	require.True(t, stream.Next())
	require.False(t, stream.Next())
	require.NoError(t, stream.Err())
	require.False(t, stream.Next())
	require.NoError(t, stream.Err())
	_, err = client.Do(WithStreamIdleTimeout(t.Context(), time.Millisecond), &Request{Method: http.MethodGet, URL: server.URL})
	require.NoError(t, err, "non-streaming requests do not use the streaming inactivity policy")
}

func TestStreamIdleTimeoutParentCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(t.Context())
	stream, err := NewHttpClient().DoStream(WithStreamIdleTimeout(ctx, time.Second), &Request{Method: http.MethodGet, URL: server.URL})
	require.NoError(t, err)
	defer stream.Close()
	cancel()
	require.False(t, stream.Next())
	require.ErrorIs(t, stream.Err(), context.Canceled)
	require.NotErrorIs(t, stream.Err(), ErrStreamIdleTimeout)
}

func TestStreamIdleWatchdogStopAndReset(t *testing.T) {
	ctx, activity, stop := NewStreamIdleContext(WithStreamIdleTimeout(t.Context(), 30*time.Millisecond))
	stop()
	activity()
	stop()
	<-ctx.Done()
	require.ErrorIs(t, context.Cause(ctx), context.Canceled)
	require.NotErrorIs(t, context.Cause(ctx), ErrStreamIdleTimeout)
}

func TestStreamIdleTimeoutResetsAtResponseHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for phase := range 2 {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(60 * time.Millisecond):
			}
			if phase == 0 {
				w.Header().Set("Content-Type", "text/event-stream")
				w.(http.Flusher).Flush()
			}
		}
		_, _ = fmt.Fprint(w, "data: alive\n\n")
	}))
	defer server.Close()
	stream, err := NewHttpClient().DoStream(WithStreamIdleTimeout(t.Context(), 100*time.Millisecond), &Request{Method: http.MethodGet, URL: server.URL})
	require.NoError(t, err)
	defer stream.Close()
	require.True(t, stream.Next())
	require.False(t, stream.Next())
	require.NoError(t, stream.Err())
}

func TestStreamIdleTimeoutExcludesConsumerDelay(t *testing.T) {
	for _, contentType := range []string{"text/event-stream", "application/octet-stream"} {
		t.Run(contentType, func(t *testing.T) {
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", contentType)
				_, _ = fmt.Fprint(w, "data: first\n\n")
				w.(http.Flusher).Flush()
				select {
				case <-release:
					_, _ = fmt.Fprint(w, "data: second\n\n")
				case <-r.Context().Done():
				}
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			stream, err := NewHttpClient().DoStream(WithStreamIdleTimeout(ctx, 50*time.Millisecond), &Request{Method: http.MethodGet, URL: server.URL})
			require.NoError(t, err)
			defer stream.Close()
			time.Sleep(120 * time.Millisecond)
			require.True(t, stream.Next(), "delay before the first read must not consume idle allowance")
			require.Contains(t, string(stream.Current().Data), "first")
			time.Sleep(120 * time.Millisecond)
			close(release)
			require.True(t, stream.Next(), "consumer backpressure must not trigger upstream idle timeout")
			require.Contains(t, string(stream.Current().Data), "second")
			for stream.Next() {
			}
			require.NoError(t, stream.Err())
		})
	}
}

func TestStreamParentCustomCause(t *testing.T) {
	for _, timeout := range []time.Duration{0, time.Second} {
		t.Run(timeout.String(), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.(http.Flusher).Flush()
				<-r.Context().Done()
			}))
			defer server.Close()
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			stream, err := NewHttpClient().DoStream(WithStreamIdleTimeout(ctx, timeout), &Request{Method: http.MethodGet, URL: server.URL})
			require.NoError(t, err)
			defer stream.Close()
			cancel(errors.New("caller-specific cause"))
			require.False(t, stream.Next())
			require.ErrorIs(t, stream.Err(), context.Canceled)
		})
	}
}

func TestStreamContextErrorCustomDeadlineCause(t *testing.T) {
	ctx, cancel := context.WithDeadlineCause(t.Context(), time.Now().Add(-time.Second), errors.New("caller deadline"))
	defer cancel()
	require.ErrorIs(t, StreamContextError(ctx), context.DeadlineExceeded)
}
