package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

// GitHubRelease represents a GitHub release.
type GitHubRelease struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
}

// FetchLatestGitHubRelease fetches the latest stable release tag from GitHub.
// It skips beta, rc, and prerelease versions.
func FetchLatestGitHubRelease(ctx context.Context) (string, error) {
	const apiURL = "https://api.github.com/repos/looplj/axonhub/releases"

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

	// Find the latest stable release (not prerelease, not draft, not beta/rc)
	for _, release := range releases {
		if release.Draft || release.Prerelease {
			continue
		}

		if isPreReleaseTag(release.TagName) {
			continue
		}

		return release.TagName, nil
	}

	return "", fmt.Errorf("no stable release found")
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

// IsNewerVersion compares two semantic versions and returns true if latest is newer than current.
// Versions are expected to be in format "vX.Y.Z" or "X.Y.Z".
func IsNewerVersion(current, latest string) bool {
	vCurrent, err := semver.NewVersion(current)
	if err != nil {
		// Handle error, maybe log it and return false
		return false
	}

	vLatest, err := semver.NewVersion(latest)
	if err != nil {
		// Handle error, maybe log it and return false
		return false
	}

	return vLatest.GreaterThan(vCurrent)
}
