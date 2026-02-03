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
