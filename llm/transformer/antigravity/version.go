package antigravity

import (
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

	// versionURL is the auto-updater endpoint that returns the latest Antigravity version as plain text.
	versionURL = "https://antigravity-auto-updater-974169037036.us-central1.run.app"

	// changelogURL is a fallback page to scrape the version from.
	changelogURL = "https://antigravity.google/changelog"

	// versionFetchTimeout is the maximum time allowed per fetch attempt.
	versionFetchTimeout = 5 * time.Second

	// changelogScanBytes is the number of bytes to read from the changelog page.
	changelogScanBytes = 5000
)

var versionRegex = regexp.MustCompile(`\d+\.\d+\.\d+`)

var (
	mu              sync.RWMutex
	currentVersion  = UserAgentVersionFallback
	versionResolved = false
)

func GetUserAgent() string {
	mu.RLock()
	defer mu.RUnlock()
	return "antigravity/" + currentVersion + " windows/amd64"
}

func GetVersion() string {
	mu.RLock()
	defer mu.RUnlock()
	return currentVersion
}

func setVersion(v string) {
	mu.Lock()
	defer mu.Unlock()
	if versionResolved {
		return
	}
	currentVersion = v
	versionResolved = true
}

func InitVersion() {
	fallback := UserAgentVersionFallback

	if v := fetchVersion(versionURL, 0); v != "" {
		if v != fallback {
			slog.Info("antigravity: version updated from auto-updater", "version", v, "previous", fallback)
		} else {
			slog.Debug("antigravity: version unchanged", "version", v, "source", "api")
		}
		setVersion(v)
		return
	}

	if v := fetchVersion(changelogURL, changelogScanBytes); v != "" {
		if v != fallback {
			slog.Info("antigravity: version updated from changelog", "version", v, "previous", fallback)
		} else {
			slog.Debug("antigravity: version unchanged", "version", v, "source", "changelog")
		}
		setVersion(v)
		return
	}

	slog.Info("antigravity: version fetch failed, using fallback", "fallback", fallback)
	setVersion(fallback)
}

func fetchVersion(url string, maxBytes int) string {
	client := &http.Client{Timeout: versionFetchTimeout}
	resp, err := client.Get(url) //nolint:noctx
	if err != nil {
		slog.Debug("antigravity: version fetch error", "url", url, "error", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Debug("antigravity: version fetch non-200", "url", url, "status", resp.StatusCode)
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
			slog.Debug("antigravity: version read error", "url", url, "error", err)
			return ""
		}
	}

	match := versionRegex.Find(body)
	if match == nil {
		slog.Debug("antigravity: no version found in response", "url", url)
		return ""
	}
	return string(match)
}
