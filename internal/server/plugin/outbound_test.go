package plugin

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFetchMatchAndRedirect(t *testing.T) {
	exact := "https://example.com/v1/check"
	regex := `^https://api\.example\.com/v1/items/[0-9]+$`
	granted := []string{exact, "https://example.com:443/v1/port", regex}
	trip := &recordTripper{status: http.StatusOK, body: "ok"}
	manager := &Manager{outboundTransport: trip}

	result, err := manager.Fetch(context.Background(), granted, FetchRequest{URL: exact + "?x=1"})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, result.Status)
	require.Equal(t, "ok", result.Body)
	require.Equal(t, exact+"?x=1", trip.lastURL)
	require.Equal(t, http.MethodGet, trip.lastMethod)

	_, err = manager.Fetch(context.Background(), granted, FetchRequest{URL: "https://example.com/v1/check/child"})
	require.Equal(t, hostPermissionDenied, fetchCode(t, err))
	require.Equal(t, 1, trip.hits)

	result, err = manager.Fetch(context.Background(), granted, FetchRequest{URL: "https://EXAMPLE.com/v1/check"})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, result.Status)

	result, err = manager.Fetch(context.Background(), []string{"https://example.com/v1/port"}, FetchRequest{URL: "https://example.com:443/v1/port"})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, result.Status)

	result, err = manager.Fetch(context.Background(), granted, FetchRequest{URL: "https://api.example.com/v1/items/42?q=1"})
	require.NoError(t, err)
	require.Equal(t, "https://api.example.com/v1/items/42?q=1", trip.lastURL)

	_, err = manager.Fetch(context.Background(), granted, FetchRequest{URL: "https://api.example.com/v1/items/42/extra"})
	require.Equal(t, hostPermissionDenied, fetchCode(t, err))
	_, err = manager.Fetch(context.Background(), granted, FetchRequest{URL: "https://api.example.com/v1/items/"})
	require.Equal(t, hostPermissionDenied, fetchCode(t, err))

	before := trip.hits
	_, err = manager.Fetch(context.Background(), granted, FetchRequest{URL: "http://example.com/v1/check"})
	require.Equal(t, hostPermissionDenied, fetchCode(t, err))
	_, err = manager.Fetch(context.Background(), granted, FetchRequest{URL: "https://user:pw@example.com/v1/check"})
	require.Equal(t, hostPermissionDenied, fetchCode(t, err))
	require.Equal(t, before, trip.hits)

	trip.status = http.StatusFound
	trip.header = http.Header{"Location": []string{"https://example.com/secret"}}
	redirectHits := trip.maxHitsFor(exact)
	_, err = manager.Fetch(context.Background(), granted, FetchRequest{URL: exact})
	require.Equal(t, hostInvalidState, fetchCode(t, err))
	require.NotContains(t, err.Error(), "secret")
	require.NotContains(t, err.Error(), "Location")
	require.Equal(t, redirectHits+1, trip.maxHitsFor(exact))
}

func TestFetchRegexRequiresWholeURL(t *testing.T) {
	rule := `^https://api\.example\.com/v1|https://cdn\.example\.com/v2$`
	for _, pattern := range []string{rule, `^(?m:https://api\.example\.com/v1|https://cdn\.example\.com/v2)$`} {
		t.Run(pattern, func(t *testing.T) {
			trip := &recordTripper{body: "ok"}
			manager := &Manager{outboundTransport: trip}
			for _, target := range []string{"https://api.example.com/v1", "https://cdn.example.com/v2?query=1"} {
				_, err := manager.Fetch(context.Background(), []string{pattern}, FetchRequest{URL: target})
				require.NoError(t, err)
			}
			for _, target := range []string{
				"https://unapproved.example/https://cdn.example.com/v2",
				"https://api.example.com/v1/extra",
			} {
				_, err := manager.Fetch(context.Background(), []string{pattern}, FetchRequest{URL: target})
				require.Equal(t, hostPermissionDenied, fetchCode(t, err))
			}
			require.Equal(t, 2, trip.hits)
		})
	}
}

func TestFetchHeadersAndBodies(t *testing.T) {
	trip := &recordTripper{
		status: http.StatusCreated,
		body:   "created",
		header: http.Header{
			"Content-Type": []string{"text/plain"},
			"X-One":        []string{"a", "b"},
			"Connection":   []string{"close, X-Drop"},
			"X-Drop":       []string{"gone"},
			"Keep-Alive":   []string{"timeout=1"},
		},
	}
	manager := &Manager{outboundTransport: trip}
	granted := []string{"https://example.com/v1/check"}
	result, err := manager.Fetch(context.Background(), granted, FetchRequest{
		URL:    "https://example.com/v1/check",
		Method: "post",
		Headers: map[string]string{
			"Authorization": "plugin-secret",
			"Host":          "evil.example",
			"Connection":    "X-Drop",
			"X-Drop":        "nope",
			"Cookie":        "user=jwt",
			"X-Trace":       "1",
		},
		Body: `{"text":"hi"}`,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, result.Status)
	require.Equal(t, "created", result.Body)
	require.Equal(t, "text/plain", result.Headers["content-type"])
	require.Equal(t, "a, b", result.Headers["x-one"])
	require.NotContains(t, result.Headers, "connection")
	require.NotContains(t, result.Headers, "x-drop")
	require.NotContains(t, result.Headers, "keep-alive")
	require.Equal(t, "plugin-secret", trip.lastHeader.Get("Authorization"))
	require.Equal(t, "1", trip.lastHeader.Get("X-Trace"))
	require.Empty(t, trip.lastHeader.Get("Host"))
	require.Equal(t, "user=jwt", trip.lastHeader.Get("Cookie"))
	require.Empty(t, trip.lastHeader.Get("X-Drop"))
	require.Equal(t, http.MethodPost, trip.lastMethod)
	require.Equal(t, `{"text":"hi"}`, trip.lastBody)

	trip.status = http.StatusBadGateway
	trip.body = "upstream"
	trip.header = http.Header{}
	result, err = manager.Fetch(context.Background(), granted, FetchRequest{URL: "https://example.com/v1/check"})
	require.NoError(t, err)
	require.Equal(t, http.StatusBadGateway, result.Status)
	require.Equal(t, "upstream", result.Body)

	trip.body = string([]byte{0xff, 0xfe})
	_, err = manager.Fetch(context.Background(), granted, FetchRequest{URL: "https://example.com/v1/check"})
	require.Equal(t, hostInvalidState, fetchCode(t, err))

	trip.body = strings.Repeat("a", maxFetchResponseBody+1)
	_, err = manager.Fetch(context.Background(), granted, FetchRequest{URL: "https://example.com/v1/check"})
	require.Equal(t, hostLimitExceeded, fetchCode(t, err))

	_, err = manager.Fetch(context.Background(), granted, FetchRequest{
		URL:  "https://example.com/v1/check",
		Body: strings.Repeat("b", maxFetchRequestBody+1),
	})
	require.Equal(t, hostLimitExceeded, fetchCode(t, err))

	_, err = manager.Fetch(context.Background(), granted, FetchRequest{URL: "https://example.com/v1/check", Method: "TRACE"})
	require.Equal(t, hostInvalidArgument, fetchCode(t, err))

	trip.err = io.ErrUnexpectedEOF
	trip.body = "ok"
	_, err = manager.Fetch(context.Background(), granted, FetchRequest{URL: "https://example.com/v1/check"})
	require.Equal(t, hostUnavailable, fetchCode(t, err))

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	time.Sleep(2 * time.Millisecond)
	_, err = manager.Fetch(ctx, granted, FetchRequest{URL: "https://example.com/v1/check"})
	require.Equal(t, hostDeadlineExceeded, fetchCode(t, err))
}

func fetchCode(t *testing.T, err error) string {
	t.Helper()
	require.Error(t, err)
	var kind *FetchError
	require.ErrorAs(t, err, &kind)
	return kind.Code
}

type recordTripper struct {
	mu         sync.Mutex
	hits       int
	byURL      map[string]int
	status     int
	body       string
	header     http.Header
	err        error
	lastURL    string
	lastMethod string
	lastHeader http.Header
	lastBody   string
}

func (r *recordTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
		_ = req.Body.Close()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hits++
	if r.byURL == nil {
		r.byURL = map[string]int{}
	}
	r.byURL[req.URL.String()]++
	r.lastURL = req.URL.String()
	r.lastMethod = req.Method
	r.lastHeader = req.Header.Clone()
	r.lastBody = string(body)
	if r.err != nil {
		return nil, r.err
	}
	status := r.status
	if status == 0 {
		status = http.StatusOK
	}
	header := r.header.Clone()
	if header == nil {
		header = make(http.Header)
	}
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(r.body)),
		Request:    req,
	}, nil
}

func (r *recordTripper) maxHitsFor(raw string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.byURL[raw]
}
