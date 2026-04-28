package transformer

import (
	"strings"
)

// NormalizeBaseURL normalizes the base URL for API endpoints.
// It ensures that the URL ends with the specified version and handles special cases:
// - URLs ending with "#" are treated as raw URLs (version not appended)
// - Trailing slashes are removed
// - Version is appended only if not already present.
func NormalizeBaseURL(url, version string) string {
	if url == "" {
		return ""
	}

	if before, ok := strings.CutSuffix(url, "#"); ok {
		normalized := strings.TrimRight(before, "/")
		return normalized
	}

	if version == "" {
		return strings.TrimRight(url, "/")
	}

	if strings.HasSuffix(url, "/"+version) {
		return strings.TrimRight(url, "/")
	}

	if strings.Contains(url, "/"+version+"/") {
		return strings.TrimRight(url, "/")
	}

	trimmed := strings.TrimRight(url, "/")
	if strings.HasSuffix(trimmed, "/") {
		return trimmed + "/" + version
	}

	return trimmed + "/" + version
}

// BuildRequestURL constructs the full request URL from base URL and path parameters.
// When endpointPath is set (non-empty), it overrides defaultPath and skips default version
// normalization on the base URL itself (but maintains compatibility with "#" and "##" behaviors).
//
// Parameters:
//   - baseURL: the channel's base URL (may contain "#" or "##" suffix)
//   - version: the API version to append (e.g., "v1")
//   - defaultPath: the default endpoint path (e.g., "/chat/completions")
//   - endpointPath: optional custom endpoint path override from channel endpoint config
//   - rawURL: when true, use baseURL as-is without appending any path
func BuildRequestURL(baseURL, version, defaultPath, endpointPath string, rawURL bool) string {
	if rawURL {
		return strings.TrimRight(baseURL, "/")
	}

	normalized := NormalizeBaseURL(baseURL, version)

	if endpointPath != "" {
		return normalized + endpointPath
	}

	return normalized + defaultPath
}
