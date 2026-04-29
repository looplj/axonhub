package antigravity

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"sync"
	"time"
)

const (
	// UserAgentVersionFallback is the hardcoded fallback version used when remote fetch fails.
	UserAgentVersionFallback = "1.20.4"

	// defaultVersionURL is the auto-updater endpoint that returns the latest Antigravity version as plain text.
	defaultVersionURL = "https://antigravity-auto-updater-974169037036.us-central1.run.app"

	// defaultChangelogURL is a fallback page to scrape the version from.
	defaultChangelogURL = "https://antigravity.google/changelog"

	// versionFetchTimeout is the maximum time allowed per fetch attempt.
	versionFetchTimeout = 5 * time.Second

	// changelogScanBytes is the number of bytes to read from the changelog page.
	changelogScanBytes = 5000
)

var versionRegex = regexp.MustCompile(`\d+\.\d+\.\d+`)

var (
	versionMu      sync.RWMutex
	currentVersion = UserAgentVersionFallback
	initOnce       sync.Once
)

func GetUserAgent() string {
	versionMu.RLock()
	defer versionMu.RUnlock()

	return "antigravity/" + currentVersion + " windows/amd64"
}

func GetVersion() string {
	versionMu.RLock()
	defer versionMu.RUnlock()

	return currentVersion
}

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

func InitVersion(ctx context.Context) {
	initOnce.Do(func() {
		defaultFetcher.init(ctx)
	})
}

func (f *versionFetcher) init(ctx context.Context) {
	fallback := UserAgentVersionFallback

	if v := f.fetchVersion(ctx, f.versionURL, 0); v != "" {
		if v != fallback {
			slog.InfoContext(ctx, "antigravity: version updated from auto-updater", "version", v, "previous", fallback)
		} else {
			slog.DebugContext(ctx, "antigravity: version unchanged", "version", v, "source", "api")
		}

		setVersion(v)

		return
	}

	if v := f.fetchVersion(ctx, f.changelogURL, changelogScanBytes); v != "" {
		if v != fallback {
			slog.InfoContext(ctx, "antigravity: version updated from changelog", "version", v, "previous", fallback)
		} else {
			slog.DebugContext(ctx, "antigravity: version unchanged", "version", v, "source", "changelog")
		}

		setVersion(v)

		return
	}

	slog.InfoContext(ctx, "antigravity: version fetch failed, using fallback", "fallback", fallback)
	setVersion(fallback)
}

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

	match := versionRegex.Find(body)
	if match == nil {
		slog.DebugContext(ctx, "antigravity: no version found in response", "url", url)
		return ""
	}

	return string(match)
}
