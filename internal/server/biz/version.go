package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/looplj/axonhub/internal/build"
	"github.com/looplj/axonhub/internal/ent"
)

// Version retrieves the system version from system settings.
// Returns empty string if not set.
func (s *SystemService) Version(ctx context.Context) (string, error) {
	value, err := s.getSystemValue(ctx, SystemKeyVersion)
	if err != nil {
		if ent.IsNotFound(err) {
			return "", nil
		}

		return "", fmt.Errorf("failed to get system version: %w", err)
	}

	return value, nil
}

// SetVersion sets the system version.
func (s *SystemService) SetVersion(ctx context.Context, version string) error {
	return s.setSystemValue(ctx, SystemKeyVersion, version)
}

// VersionCheckResult contains the result of a version check.
type VersionCheckResult struct {
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	HasUpdate      bool   `json:"has_update"`
	ReleaseURL     string `json:"release_url"`
}

// CheckForUpdate checks if there is a newer version available on GitHub.
func (s *SystemService) CheckForUpdate(ctx context.Context, includeBeta bool) (*VersionCheckResult, error) {
	currentVersion := build.Version

	latestVersion, err := s.fetchLatestGitHubRelease(ctx, includeBeta)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}

	hasUpdate := s.isNewerVersion(currentVersion, latestVersion)
	releaseURL := fmt.Sprintf("https://github.com/looplj/axonhub/releases/tag/%s", latestVersion)

	return &VersionCheckResult{
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		HasUpdate:      hasUpdate,
		ReleaseURL:     releaseURL,
	}, nil
}

// fetchLatestGitHubRelease fetches the latest eligible release tag from GitHub.
func (s *SystemService) fetchLatestGitHubRelease(ctx context.Context, includeBeta bool) (string, error) {
	return FetchLatestGitHubRelease(ctx, includeBeta)
}

// isNewerVersion compares two semantic versions and returns true if latest is newer than current.
func (s *SystemService) isNewerVersion(current, latest string) bool {
	return IsNewerVersion(current, latest)
}

// GitHubRelease represents a GitHub release.
type GitHubRelease struct {
	TagName     string    `json:"tag_name"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
	PublishedAt time.Time `json:"published_at"`
}

// releaseCooldownDuration is the time to wait after a release is published before considering it available.
// This accounts for build and upload time.
const releaseCooldownDuration = 30 * time.Minute

// FetchLatestGitHubRelease fetches the latest eligible release tag from GitHub for the axonhub service.
// Beta versions are included when includeBeta is true; other prereleases are always skipped.
// It waits for a cooldown period after release.
// In monorepo mode, it only considers tags matching "vX.Y.Z" (no service prefix).
func FetchLatestGitHubRelease(ctx context.Context, includeBeta bool) (string, error) {
	baseURL := "https://api.github.com/repos/looplj/axonhub/releases"

	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	q := u.Query()
	q.Set("per_page", "100")
	q.Set("page", "1")
	u.RawQuery = q.Encode()
	apiURL := u.String()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "AxonHub-Version-Checker")

	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch releases: %w", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var releases []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return "", fmt.Errorf("failed to decode releases: %w", err)
	}

	return selectLatestGitHubRelease(releases, includeBeta, time.Now().UTC())
}

// selectLatestGitHubRelease returns the highest semantic version among eligible releases.
func selectLatestGitHubRelease(releases []GitHubRelease, includeBeta bool, now time.Time) (string, error) {
	var latestVersion *semver.Version
	latestTag := ""

	for _, release := range releases {
		if release.Draft {
			continue
		}

		// Only consider axonhub tags (vX.Y.Z format, skip service-prefixed tags like "axonclaw/v1.0.0")
		if !isAxonHubTag(release.TagName) {
			continue
		}

		if release.Prerelease || isPreReleaseTag(release.TagName) {
			if !includeBeta || !isBetaReleaseTag(release.TagName) {
				continue
			}
		}

		// Check if the release has passed the cooldown period
		if now.Sub(release.PublishedAt) < releaseCooldownDuration {
			continue
		}

		version, err := semver.NewVersion(release.TagName)
		if err != nil {
			continue
		}

		if latestVersion == nil || compareVersions(version, latestVersion) > 0 {
			latestVersion = version
			latestTag = release.TagName
		}
	}

	if latestTag != "" {
		return latestTag, nil
	}

	return "", fmt.Errorf("no eligible release found")
}

// isAxonHubTag returns true if the tag is an axonhub version tag (vX.Y.Z format).
// Tags with a service prefix (e.g., "axonclaw/v1.0.0") are not axonhub tags.
func isAxonHubTag(tag string) bool {
	// axonhub tags start with "v", other services use "service/vX.Y.Z" format
	return strings.HasPrefix(tag, "v")
}

// isPreReleaseTag checks if a version tag contains beta, rc, alpha, or similar prerelease indicators.
func isPreReleaseTag(tag string) bool {
	lowerTag := strings.ToLower(tag)
	preReleasePatterns := []string{"-beta", "-rc", "-alpha", "-dev", "-preview", "-snapshot"}

	for _, pattern := range preReleasePatterns {
		if strings.Contains(lowerTag, pattern) {
			return true
		}
	}

	return false
}

// isBetaReleaseTag reports whether tag identifies a beta prerelease.
func isBetaReleaseTag(tag string) bool {
	return strings.Contains(strings.ToLower(tag), "-beta")
}

// IsNewerVersion compares two semantic versions and returns true if latest is newer than current.
// Versions are expected to be in format "vX.Y.Z" or "X.Y.Z".
func IsNewerVersion(current, latest string) bool {
	result, err := CompareVersions(latest, current)
	if err != nil {
		// Handle error, maybe log it and return false
		return false
	}

	return result > 0
}

// CompareVersions compares two version strings and reports whether a is less than,
// equal to, or greater than b, returning -1, 0, or 1 respectively.
// Versions are expected to be in format "vX.Y.Z" or "X.Y.Z", optionally with a
// prerelease suffix. An error is returned when either version cannot be parsed.
func CompareVersions(a, b string) (int, error) {
	vA, err := ParseVersion(a)
	if err != nil {
		return 0, err
	}

	vB, err := ParseVersion(b)
	if err != nil {
		return 0, err
	}

	return compareVersions(vA, vB), nil
}

// ParseVersion parses a version string in the format "vX.Y.Z" or "X.Y.Z".
func ParseVersion(version string) (*semver.Version, error) {
	v, err := semver.NewVersion(version)
	if err != nil {
		return nil, fmt.Errorf("failed to parse version %q: %w", version, err)
	}

	return v, nil
}

// compareVersions compares the release numbers first and falls back to
// comparePrerelease, which understands prerelease identifiers such as "beta10"
// that glue a name to a counter without a dot separator.
func compareVersions(a, b *semver.Version) int {
	if result := compareUint64(a.Major(), b.Major()); result != 0 {
		return result
	}

	if result := compareUint64(a.Minor(), b.Minor()); result != 0 {
		return result
	}

	if result := compareUint64(a.Patch(), b.Patch()); result != 0 {
		return result
	}

	return comparePrerelease(a.Prerelease(), b.Prerelease())
}

// comparePrerelease compares two prerelease strings following the SemVer
// precedence rules, except that each identifier is further split into digit and
// non-digit runs. That makes the counter in tags like "beta9" and "beta10"
// compare numerically, and treats "beta10" as equal to "beta.10".
// A release without a prerelease outranks any prerelease of the same version.
func comparePrerelease(a, b string) int {
	if a == b {
		return 0
	}

	if a == "" {
		return 1
	}

	if b == "" {
		return -1
	}

	tokensA := prereleaseTokens(a)
	tokensB := prereleaseTokens(b)

	for i := 0; i < len(tokensA) && i < len(tokensB); i++ {
		if result := comparePrereleaseToken(tokensA[i], tokensB[i]); result != 0 {
			return result
		}
	}

	// A larger set of prerelease tokens takes precedence when every shared token is equal.
	return compareInt(len(tokensA), len(tokensB))
}

// prereleaseTokens splits a prerelease string on dots and then into digit and
// non-digit runs, e.g. "beta10" and "beta.10" both become []string{"beta", "10"}.
func prereleaseTokens(s string) []string {
	var tokens []string

	for identifier := range strings.SplitSeq(s, ".") {
		tokens = append(tokens, splitDigitRuns(identifier)...)
	}

	return tokens
}

// comparePrereleaseToken compares a single flattened prerelease token.
func comparePrereleaseToken(a, b string) int {
	numericA, numericB := isDigitRun(a), isDigitRun(b)

	switch {
	case numericA && numericB:
		return compareNumericRuns(a, b)
	case numericA != numericB:
		// Numeric identifiers always have lower precedence than alphanumeric ones.
		if numericA {
			return -1
		}

		return 1
	default:
		return strings.Compare(a, b)
	}
}

// splitDigitRuns splits s into consecutive runs of digits and non-digits,
// e.g. "beta10" becomes []string{"beta", "10"}.
func splitDigitRuns(s string) []string {
	if s == "" {
		return nil
	}

	var runs []string

	start := 0
	digits := isASCIIDigit(s[0])

	for i := 1; i < len(s); i++ {
		if isASCIIDigit(s[i]) == digits {
			continue
		}

		runs = append(runs, s[start:i])
		start = i
		digits = isASCIIDigit(s[i])
	}

	return append(runs, s[start:])
}

// compareNumericRuns compares two digit runs by value without parsing them,
// so arbitrarily long counters cannot overflow.
func compareNumericRuns(a, b string) int {
	trimmedA := strings.TrimLeft(a, "0")
	trimmedB := strings.TrimLeft(b, "0")

	if result := compareInt(len(trimmedA), len(trimmedB)); result != 0 {
		return result
	}

	return strings.Compare(trimmedA, trimmedB)
}

func isDigitRun(s string) bool {
	return s != "" && isASCIIDigit(s[0])
}

func isASCIIDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func compareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func compareUint64(a, b uint64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
