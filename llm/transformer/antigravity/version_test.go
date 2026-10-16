package antigravity

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func resetVersionState(t *testing.T) {
	t.Helper()
	setVersion(UserAgentVersionFallback)
	initOnce = sync.Once{}
	t.Cleanup(func() { setVersion(UserAgentVersionFallback) })
}

func TestFetchVersion_PrefersCLIAndNeverDowngrades(t *testing.T) {
	for _, tc := range []struct{ name, changelog, expected string }{
		{"CLI release", `<article id="rel-cli-1.4.0"></article>`, "1.4.0"},
		{"mixed surfaces", `<article id="rel-ide-9.0.0"></article><article id="rel-hub-9.1.0"></article><article id="rel-cli-1.4.1"></article>`, "1.4.1"},
		{"IDE only", `<article id="rel-ide-9.0.0"></article>`, UserAgentVersionFallback},
		{"App only", `<article id="rel-hub-9.0.0"></article>`, UserAgentVersionFallback},
		{"old updater response", "9.99.0", UserAgentVersionFallback},
		{"stale CLI release", `<article id="rel-cli-1.0.6"></article>`, UserAgentVersionFallback},
		{"unparseable response", "no release here", UserAgentVersionFallback},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetVersionState(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tc.changelog) }))
			defer srv.Close()
			f := &versionFetcher{changelogURL: srv.URL, httpClient: srv.Client()}
			f.init(t.Context())
			require.Equal(t, tc.expected, GetVersion())
			headers := http.Header{}
			SetClientHeaders(headers)
			require.Equal(t, fmt.Sprintf("antigravity/cli/%s (aidev_client; os_type=%s; arch=%s; auth_method=consumer)", tc.expected, runtime.GOOS, runtime.GOARCH), GetUserAgent())
			require.Equal(t, GetUserAgent(), headers.Get("User-Agent"))
			require.Equal(t, "antigravity", headers.Get("X-Client-Name"))
			require.Equal(t, tc.expected, headers.Get("X-Client-Version"))
			require.Equal(t, ClientMetadata, headers.Get("Client-Metadata"))
			require.Equal(t, ApiClient, headers.Get("X-Goog-Api-Client"))
		})
	}
}

func TestFetchVersion_HardcodedFallback(t *testing.T) {
	resetVersionState(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer srv.Close()
	(&versionFetcher{changelogURL: srv.URL, httpClient: srv.Client()}).init(t.Context())
	require.Equal(t, UserAgentVersionFallback, GetVersion())
}

func TestFetchVersion_ReleaseManifest(t *testing.T) {
	for _, tc := range []struct{ name, manifest, expected string }{
		{"official manifest wins", `{"version":"1.4.0","url":"https://example.invalid/agy.exe"}`, "1.4.0"},
		{"malformed manifest uses CLI changelog", `{"version":"not-a-version"}`, "1.5.0"},
		{"stale manifest keeps supported fallback", `{"version":"1.0.0"}`, UserAgentVersionFallback},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetVersionState(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/manifest" {
					fmt.Fprint(w, tc.manifest)
					return
				}
				fmt.Fprint(w, `<article id="rel-hub-9.9.9"></article><article id="rel-cli-1.5.0"></article>`)
			}))
			defer srv.Close()
			(&versionFetcher{manifestURL: srv.URL + "/manifest", changelogURL: srv.URL + "/changelog", httpClient: srv.Client()}).init(t.Context())
			require.Equal(t, tc.expected, GetVersion())
		})
	}
}

func TestInitVersion_OnceGuard(t *testing.T) {
	resetVersionState(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, `<article id="rel-cli-1.50.0"></article>`)
	}))
	defer srv.Close()
	f := &versionFetcher{changelogURL: srv.URL, httpClient: srv.Client()}
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() { initOnce.Do(func() { f.init(t.Context()) }) })
	}
	wg.Wait()
	require.Equal(t, "1.50.0", GetVersion())
	require.EqualValues(t, 1, calls.Load())
}

func TestFetchVersion_ChangelogMaxBytes(t *testing.T) {
	resetVersionState(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, strings.Repeat("x", changelogScanBytes+100))
		fmt.Fprint(w, `<article id="rel-cli-9.9.9"></article>`)
	}))
	defer srv.Close()
	(&versionFetcher{changelogURL: srv.URL, httpClient: srv.Client()}).init(t.Context())
	require.Equal(t, UserAgentVersionFallback, GetVersion())
}

func TestNewerVersionMalformedInputs(t *testing.T) {
	for _, tc := range []struct{ current, candidate, expected string }{
		{"2.23.0", "2.23", "2.23.0"},
		{"2.23", "3.0.0", "2.23"},
		{"2.23.0", "3.0.bad", "2.23.0"},
		{"2.23.bad", "3.0.0", "2.23.bad"},
		{"2.23.0", "2.23.0-beta", "2.23.0"},
		{"2.23.0", "2.23.0.1", "2.23.0"},
		{"2.23.0", "-3.0.0", "2.23.0"},
		{"2.23.0", "", "2.23.0"},
		{"2.23.0", "2.100.0", "2.100.0"},
		{"2.23.0", "3.0.0", "3.0.0"},
		{"2.23.0", "2.23.1", "2.23.1"},
		{"2.23.0", "2.5.5", "2.23.0"},
	} {
		t.Run(tc.current+"/"+tc.candidate, func(t *testing.T) { require.Equal(t, tc.expected, newerVersion(tc.current, tc.candidate)) })
	}
}
