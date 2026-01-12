package transformer

import (
	"strings"
)

// NormalizeBaseURL normalizes the base URL for the Anthropic API.
// It ensures that the URL ends with a slash and does not contain the version in the path.
// TODO: use this func to unify the base URL for all transformers.
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
