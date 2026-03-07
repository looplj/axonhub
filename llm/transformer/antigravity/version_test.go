package antigravity

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetVersionState resets the package-level version state between tests.
// Must be called at the start of each test that exercises InitVersion/fetchVersion.
func resetVersionState(t *testing.T) {
	t.Helper()
	versionMu.Lock()
	currentVersion = UserAgentVersionFallback
	versionMu.Unlock()
	initOnce = sync.Once{}
}

func newFetcher(versionSrv, changelogSrv *httptest.Server) *versionFetcher {
	vURL := ""
	if versionSrv != nil {
		vURL = versionSrv.URL
	}
	cURL := ""
	if changelogSrv != nil {
		cURL = changelogSrv.URL
	}
	return &versionFetcher{
		versionURL:   vURL,
		changelogURL: cURL,
		httpClient:   &http.Client{},
	}
}

func TestFetchVersion_AutoUpdaterSuccess(t *testing.T) {
	resetVersionState(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "1.99.0")
	}))
	defer srv.Close()

	f := newFetcher(srv, nil)
	f.init(context.Background())

	assert.Equal(t, "1.99.0", GetVersion())
	assert.Equal(t, "antigravity/1.99.0 windows/amd64", GetUserAgent())
}

// TestFetchVersion_ChangelogFallback verifies the changelog scrape path is used
// when the auto-updater endpoint is unavailable.
func TestFetchVersion_ChangelogFallback(t *testing.T) {
	resetVersionState(t)

	vSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer vSrv.Close()

	cSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html>...Antigravity 1.21.0 released...</html>`)
	}))
	defer cSrv.Close()

	f := newFetcher(vSrv, cSrv)
	f.init(context.Background())

	assert.Equal(t, "1.21.0", GetVersion())
}

// TestFetchVersion_HardcodedFallback verifies the hardcoded fallback is used when
// both remote endpoints are unavailable.
func TestFetchVersion_HardcodedFallback(t *testing.T) {
	resetVersionState(t)

	vSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer vSrv.Close()

	cSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer cSrv.Close()

	f := newFetcher(vSrv, cSrv)
	f.init(context.Background())

	assert.Equal(t, UserAgentVersionFallback, GetVersion())
}

// TestFetchVersion_NoSemverInResponse verifies that a response with no parseable
// semver falls through to the next source.
func TestFetchVersion_NoSemverInResponse(t *testing.T) {
	resetVersionState(t)

	vSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "no version here")
	}))
	defer vSrv.Close()

	cSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "release: 2.0.1")
	}))
	defer cSrv.Close()

	f := newFetcher(vSrv, cSrv)
	f.init(context.Background())

	assert.Equal(t, "2.0.1", GetVersion())
}

// TestInitVersion_OnceGuard verifies that InitVersion only initialises once even
// when called concurrently multiple times.
func TestInitVersion_OnceGuard(t *testing.T) {
	resetVersionState(t)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		fmt.Fprint(w, "1.50.0")
	}))
	defer srv.Close()

	f := &versionFetcher{
		versionURL:   srv.URL,
		changelogURL: srv.URL,
		httpClient:   &http.Client{},
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			initOnce.Do(func() { f.init(context.Background()) })
		}()
	}
	wg.Wait()

	assert.Equal(t, "1.50.0", GetVersion())
	// auto-updater should have been contacted exactly once
	require.Equal(t, 1, callCount, "version endpoint should only be called once")
}

// TestFetchVersion_ChangelogMaxBytes verifies that only the first changelogScanBytes
// of the changelog body are inspected.
func TestFetchVersion_ChangelogMaxBytes(t *testing.T) {
	resetVersionState(t)

	vSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer vSrv.Close()

	padding := make([]byte, changelogScanBytes+100)
	for i := range padding {
		padding[i] = 'x'
	}

	cSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(padding)
		fmt.Fprint(w, "9.9.9")
	}))
	defer cSrv.Close()

	f := newFetcher(vSrv, cSrv)
	f.init(context.Background())

	assert.Equal(t, UserAgentVersionFallback, GetVersion())
}
