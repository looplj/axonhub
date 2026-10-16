package antigravity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// UserAgentVersionFallback is the hardcoded fallback version used when remote fetch fails.
	UserAgentVersionFallback = "1.3.6"

	// defaultChangelogURL publishes releases for each Antigravity surface.
	defaultChangelogURL = "https://antigravity.google/docs/changelog"

	// versionFetchTimeout is the maximum time allowed per fetch attempt.
	versionFetchTimeout = 5 * time.Second

	// changelogScanBytes is the number of bytes to read from the changelog page.
	changelogScanBytes = 2 * 1024 * 1024
)

// CLI and App versions are independent release streams.
var cliVersionRegex = regexp.MustCompile(`id="rel-cli-(\d+\.\d+\.\d+)"`)
var clientSemverRegex = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

var (
	versionMu      sync.RWMutex
	currentVersion = UserAgentVersionFallback
	initOnce       sync.Once
)

// GetUserAgent uses the CLI identity understood by the account model catalog.
// A generic antigravity/<App version> identity receives the IDE catalog, which
// excludes App/CLI-only models even with the same account and access token.
func GetUserAgent() string {
	versionMu.RLock()
	defer versionMu.RUnlock()

	return fmt.Sprintf("antigravity/cli/%s (aidev_client; os_type=%s; arch=%s; auth_method=consumer)", currentVersion, runtime.GOOS, runtime.GOARCH)
}

// GetVersion returns the current CLI version under the shared version lock.
func GetVersion() string {
	versionMu.RLock()
	defer versionMu.RUnlock()

	return currentVersion
}

// SetClientHeaders keeps model discovery and inference on the same client identity.
func SetClientHeaders(headers http.Header) {
	headers.Set("User-Agent", GetUserAgent())
}

// setVersion publishes the resolved version to all Antigravity request paths.
func setVersion(v string) {
	versionMu.Lock()
	defer versionMu.Unlock()

	currentVersion = v
}

type versionFetcher struct {
	manifestURL  string
	changelogURL string
	httpClient   *http.Client
}

var defaultFetcher = &versionFetcher{
	manifestURL:  fmt.Sprintf("https://antigravity-cli-auto-updater-974169037036.us-central1.run.app/manifests/%s_%s.json", runtime.GOOS, runtime.GOARCH),
	changelogURL: defaultChangelogURL,
	httpClient:   &http.Client{Timeout: versionFetchTimeout},
}

// InitVersion resolves the CLI version once per process, retaining the fallback on failure.
func InitVersion(ctx context.Context) {
	initOnce.Do(func() {
		defaultFetcher.init(ctx)
	})
}

// init selects a CLI release without downgrading below the supported fallback.
func (f *versionFetcher) init(ctx context.Context) {
	fallback := UserAgentVersionFallback

	if f.manifestURL != "" {
		if v := f.fetchVersion(ctx, f.manifestURL, changelogScanBytes); v != "" {
			setVersion(newerVersion(fallback, v))
			slog.InfoContext(ctx, "antigravity: CLI version resolved from release manifest", "version", GetVersion())
			return
		}
	}

	if v := f.fetchVersion(ctx, f.changelogURL, changelogScanBytes); v != "" {
		setVersion(newerVersion(fallback, v))
		slog.InfoContext(ctx, "antigravity: CLI version resolved", "version", GetVersion())
		return
	}

	slog.InfoContext(ctx, "antigravity: version fetch failed, using fallback", "fallback", fallback)
	setVersion(fallback)
}

// fetchVersion reads a bounded CLI release manifest or CLI changelog.
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
	var manifest struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(body, &manifest) == nil && clientSemverRegex.MatchString(manifest.Version) {
		return manifest.Version
	}
	if matches := cliVersionRegex.FindSubmatch(body); len(matches) == 2 {
		match = matches[1]
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
