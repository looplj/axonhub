package antigravity

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// UserAgentVersionFallback is the hardcoded fallback version used when remote fetch fails.
	UserAgentVersionFallback = "2.5.5"

	// defaultVersionURL is the auto-updater endpoint that returns the latest Antigravity version as plain text.
	defaultVersionURL = "https://antigravity-auto-updater-974169037036.us-central1.run.app"

	// defaultChangelogURL is a fallback page to scrape the version from.
	defaultChangelogURL = "https://antigravity.google/docs/changelog"

	// versionFetchTimeout is the maximum time allowed per fetch attempt.
	versionFetchTimeout = 5 * time.Second

	// changelogScanBytes is the number of bytes to read from the changelog page.
	changelogScanBytes = 2 * 1024 * 1024
)

var versionRegex = regexp.MustCompile(`\d+\.\d+\.\d+`)

// Match IDE releases explicitly: the page also lists unrelated CLI, SDK and 2.0 versions.
var ideVersionRegex = regexp.MustCompile(`id="rel-ide-(\d+\.\d+\.\d+)"`)

var (
	versionMu      sync.RWMutex
	currentVersion = UserAgentVersionFallback
	initOnce       sync.Once
)

// GetUserAgent returns the resolved IDE client identity for OAuth requests.
func GetUserAgent() string {
	versionMu.RLock()
	defer versionMu.RUnlock()

	return "antigravity/" + currentVersion + " windows/amd64"
}

// GetVersion returns the current IDE version under the shared version lock.
func GetVersion() string {
	versionMu.RLock()
	defer versionMu.RUnlock()

	return currentVersion
}

// SetClientHeaders keeps model discovery and inference on the same client identity.
func SetClientHeaders(headers http.Header) {
	version := GetVersion()
	headers.Set("User-Agent", "antigravity/"+version+" windows/amd64")
	headers.Set("X-Client-Name", "antigravity")
	headers.Set("X-Client-Version", version)
	headers.Set("X-Goog-Api-Client", ApiClient)
	headers.Set("Client-Metadata", ClientMetadata)
}

// setVersion publishes the resolved version to all Antigravity request paths.
func setVersion(v string) {
	versionMu.Lock()
	defer versionMu.Unlock()

	currentVersion = v
}

type versionFetcher struct {
	versionURL   string
	changelogURL string
	httpClient   *http.Client
}

var defaultFetcher = &versionFetcher{
	versionURL:   defaultVersionURL,
	changelogURL: defaultChangelogURL,
	httpClient:   &http.Client{Timeout: versionFetchTimeout},
}

// InitVersion resolves the IDE version once per process, retaining the fallback on failure.
func InitVersion(ctx context.Context) {
	initOnce.Do(func() {
		defaultFetcher.init(ctx)
	})
}

// init selects an IDE release without downgrading below the supported fallback.
func (f *versionFetcher) init(ctx context.Context) {
	fallback := UserAgentVersionFallback

	// Prefer the official IDE changelog over the updater's potentially stale fixed version.
	if v := f.fetchVersion(ctx, f.changelogURL, changelogScanBytes); v != "" {
		setVersion(newerVersion(fallback, v))
		slog.InfoContext(ctx, "antigravity: IDE version resolved", "version", GetVersion())
		return
	}

	if v := f.fetchVersion(ctx, f.versionURL, 0); v != "" {
		setVersion(newerVersion(fallback, v))
		slog.InfoContext(ctx, "antigravity: version resolved from auto-updater", "version", GetVersion(), "reported", v)

		return
	}

	slog.InfoContext(ctx, "antigravity: version fetch failed, using fallback", "fallback", fallback)
	setVersion(fallback)
}

// fetchVersion reads a bounded IDE changelog or the updater's version response.
func (f *versionFetcher) fetchVersion(ctx context.Context, url string, maxBytes int) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		slog.DebugContext(ctx, "antigravity: failed to build version request", "url", url, "error", err)
		return ""
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		slog.DebugContext(ctx, "antigravity: version fetch error", "url", url, "error", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.DebugContext(ctx, "antigravity: version fetch non-200", "url", url, "status", resp.StatusCode)
		return ""
	}

	var body []byte

	if maxBytes > 0 {
		buf := make([]byte, maxBytes)
		n, _ := io.ReadFull(resp.Body, buf)
		body = buf[:n]
	} else {
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			slog.DebugContext(ctx, "antigravity: version read error", "url", url, "error", err)
			return ""
		}
	}

	var match []byte
	if maxBytes > 0 {
		if matches := ideVersionRegex.FindSubmatch(body); len(matches) == 2 {
			match = matches[1]
		}
	} else {
		match = versionRegex.Find(body)
	}
	if match == nil {
		slog.DebugContext(ctx, "antigravity: no version found in response", "url", url)
		return ""
	}

	return string(match)
}

// newerVersion preserves current for malformed versions and otherwise selects the newer release.
func newerVersion(current, candidate string) string {
	currentParts, candidateParts := strings.Split(current, "."), strings.Split(candidate, ".")
	if len(currentParts) != 3 || len(candidateParts) != 3 {
		return current
	}
	var currentNumbers, candidateNumbers [3]int
	for i := range 3 {
		var err error
		currentNumbers[i], err = strconv.Atoi(currentParts[i])
		if err != nil || currentNumbers[i] < 0 {
			return current
		}
		candidateNumbers[i], err = strconv.Atoi(candidateParts[i])
		if err != nil || candidateNumbers[i] < 0 {
			return current
		}
	}
	for i := range 3 {
		a, b := currentNumbers[i], candidateNumbers[i]
		if b > a {
			return candidate
		}
		if b < a {
			return current
		}
	}
	return current
}
