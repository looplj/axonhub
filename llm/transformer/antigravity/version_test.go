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

// newFetcher directs both version sources to test servers.
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

// TestFetchVersion_AutoUpdaterSuccess checks the updater path when the IDE changelog is unavailable.
func TestFetchVersion_AutoUpdaterSuccess(t *testing.T) {
	resetVersionState(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "3.99.0")
	}))
	defer srv.Close()

	f := newFetcher(srv, nil)
	f.init(context.Background())

	assert.Equal(t, "3.99.0", GetVersion())
	assert.Equal(t, "antigravity/3.99.0 windows/amd64", GetUserAgent())
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
		fmt.Fprint(w, `<html><article id="rel-ide-3.21.0">IDE</article></html>`)
	}))
	defer cSrv.Close()

	f := newFetcher(vSrv, cSrv)
	f.init(context.Background())

	assert.Equal(t, "3.21.0", GetVersion())
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
		fmt.Fprint(w, `<article id="rel-ide-3.0.1">IDE</article>`)
	}))
	defer cSrv.Close()

	f := newFetcher(vSrv, cSrv)
	f.init(context.Background())

	assert.Equal(t, "3.0.1", GetVersion())
}

// TestInitVersion_OnceGuard verifies that InitVersion only initializes once even
// when called concurrently multiple times.
func TestInitVersion_OnceGuard(t *testing.T) {
	resetVersionState(t)

	callCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		fmt.Fprint(w, "3.50.0")
	}))
	defer srv.Close()

	f := &versionFetcher{
		versionURL:   srv.URL,
		changelogURL: srv.URL,
		httpClient:   &http.Client{},
	}

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			initOnce.Do(func() { f.init(context.Background()) })
		})
	}

	wg.Wait()

	assert.Equal(t, "3.50.0", GetVersion())
	// One initialization queries the changelog, then the updater when no IDE release is present.
	require.Equal(t, 2, callCount, "version sources should only be queried once each")
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
		fmt.Fprint(w, `<article id="rel-ide-9.9.9">IDE</article>`)
	}))
	defer cSrv.Close()

	f := newFetcher(vSrv, cSrv)
	f.init(context.Background())

	assert.Equal(t, UserAgentVersionFallback, GetVersion())
}

// TestFetchVersion_PrefersIDEAndNeverDowngrades excludes other clients and stale version sources.
func TestFetchVersion_PrefersIDEAndNeverDowngrades(t *testing.T) {
	for _, tc := range []struct {
		name, changelog, updater, expected string
	}{
		{"IDE only", `<article id="rel-agy-9.0.0"></article><article id="rel-ide-2.10.0"></article>`, "2.0.6", "2.10.0"},
		{"stale updater", `<article id="rel-cli-9.0.0"></article>`, "2.0.6", UserAgentVersionFallback},
		{"stale changelog", `<article id="rel-ide-1.23.2"></article>`, "2.0.6", UserAgentVersionFallback},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetVersionState(t)
			cSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tc.changelog) }))
			defer cSrv.Close()
			vSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tc.updater) }))
			defer vSrv.Close()
			newFetcher(vSrv, cSrv).init(t.Context())
			require.Equal(t, tc.expected, GetVersion())
			headers := http.Header{}
			SetClientHeaders(headers)
			require.Equal(t, GetUserAgent(), headers.Get("User-Agent"))
			require.Equal(t, tc.expected, headers.Get("X-Client-Version"))
		})
	}
}

// TestNewerVersionMalformedInputs checks startup safety and numeric comparison across release components.
func TestNewerVersionMalformedInputs(t *testing.T) {
	for _, tc := range []struct{ current, candidate, expected string }{
		{"2.5.5", "2.5", "2.5.5"},
		{"2.5", "3.0.0", "2.5"},
		{"2.5.5", "3.0.bad", "2.5.5"},
		{"2.5.bad", "3.0.0", "2.5.bad"},
		{"2.5.5", "2.5.5-beta", "2.5.5"},
		{"2.5.5", "2.5.5.1", "2.5.5"},
		{"2.5.5", "-3.0.0", "2.5.5"},
		{"2.5.5", "", "2.5.5"},
		{"2.5.5", "2.10.0", "2.10.0"},
		{"2.5.5", "3.0.0", "3.0.0"},
		{"2.5.5", "2.5.6", "2.5.6"},
		{"2.5.5", "2.0.6", "2.5.5"},
	} {
		t.Run(tc.current+"/"+tc.candidate, func(t *testing.T) {
			require.Equal(t, tc.expected, newerVersion(tc.current, tc.candidate))
		})
	}
}
