package antigravity

import (
	"regexp"
	"strings"

	"github.com/samber/lo"
)

// QuotaPreference represents which quota pool to use.
type QuotaPreference string

const (
	QuotaAntigravity QuotaPreference = "antigravity"
	QuotaGeminiCLI   QuotaPreference = "gemini-cli"
)

// suffixRegex matches explicit quota suffix like :antigravity or :gemini-cli.
var suffixRegex = regexp.MustCompile(`:(antigravity|gemini-cli)$`)

// prefixRegex matches explicit antigravity- prefix.
var prefixRegex = regexp.MustCompile(`^antigravity-(.+)`)

// DetermineQuotaPreference analyzes the model name to determine which quota pool to use.
// It follows this priority:
// 1. Explicit suffix (:antigravity or :gemini-cli)
// 2. Explicit antigravity- prefix
// 3. Model-specific rules (claude*, gpt*, image models, legacy gemini-3)
// 4. Default to gemini-cli for standard Gemini models.
func DetermineQuotaPreference(modelName string) QuotaPreference {
	// Normalize to lowercase for case-insensitive matching
	lowerModelName := strings.ToLower(modelName)

	// Step 1: Check for explicit suffix
	if match := suffixRegex.FindStringSubmatch(lowerModelName); match != nil {
		if match[1] == "antigravity" {
			return QuotaAntigravity
		}

		return QuotaGeminiCLI
	}

	// Step 2: Check for explicit antigravity- prefix
	if prefixRegex.MatchString(lowerModelName) {
		return QuotaAntigravity
	}

	// Step 3: Check for Antigravity-only models
	// Claude and GPT-OSS only exist on Antigravity
	if strings.HasPrefix(lowerModelName, "claude") || strings.HasPrefix(lowerModelName, "gpt") {
		return QuotaAntigravity
	}

	// Image generation models require Antigravity
	if strings.Contains(lowerModelName, "image") || strings.Contains(lowerModelName, "imagen") {
		return QuotaAntigravity
	}

	// Legacy Gemini 3 model names (backward compatibility)
	// These are the old naming convention before -preview suffix was added
	legacyGemini3Patterns := []string{
		"gemini-3-pro-low",
		"gemini-3-pro-high",
		"gemini-3-pro-medium",
		"gemini-3-flash",
		"gemini-3-flash-low",
		"gemini-3-flash-medium",
		"gemini-3-flash-high",
	}
	if lo.Contains(legacyGemini3Patterns, lowerModelName) {
		return QuotaAntigravity
	}

	// Step 4: Default to Gemini CLI quota for standard Gemini models
	// This includes: gemini-2.5-*, gemini-1.5-*, gemini-3-*-preview, etc.
	return QuotaGeminiCLI
}

// GetInitialEndpoint returns the initial endpoint based on quota preference.
// Always starts with Daily sandbox to match reference implementation behavior.
// Daily has the latest features and better quota availability.
func GetInitialEndpoint(quotaPreference QuotaPreference) string {
	// Both Antigravity and Gemini CLI quota should start with Daily
	// Reference implementation always tries Daily first for better quota distribution
	return EndpointDaily
}

// GetFallbackEndpoints returns all endpoints to try in order.
// Regardless of the initial endpoint, we try all three for maximum reliability.
func GetFallbackEndpoints() []string {
	return []string{
		EndpointDaily,    // Daily sandbox (latest features)
		EndpointAutopush, // Autopush sandbox (staging)
		EndpointProd,     // Production (most stable)
	}
}

// StripModelSuffix removes the quota preference suffix from a model name.
// For example: "gemini-2.5-pro:antigravity" -> "gemini-2.5-pro".
func StripModelSuffix(modelName string) string {
	return suffixRegex.ReplaceAllString(modelName, "")
}

// StripAntigravityPrefix removes the "antigravity-" prefix from a model name.
// For example: "antigravity-gemini-2.5-pro" -> "gemini-2.5-pro".
func StripAntigravityPrefix(modelName string) string {
	if match := prefixRegex.FindStringSubmatch(modelName); match != nil {
		return match[1]
	}

	return modelName
}

// NormalizeModelName strips both suffix and prefix to get the base model name.
func NormalizeModelName(modelName string) string {
	// First strip suffix, then strip prefix
	normalized := StripModelSuffix(modelName)
	normalized = StripAntigravityPrefix(normalized)

	return normalized
}

// ShouldRetryWithDifferentEndpoint determines if an HTTP status code indicates
// we should try a different endpoint.
func ShouldRetryWithDifferentEndpoint(statusCode int) bool {
	// Retry on:
	// - 429 Rate Limit (quota exhausted on this endpoint)
	// - 403 Forbidden (permissions/config issues)
	// - 404 Not Found (model not available on this endpoint)
	// - 5xx Server errors
	return statusCode == 429 || statusCode == 403 || statusCode == 404 || (statusCode >= 500 && statusCode < 600)
}

// transformModelForAntigravity transforms model names to match Antigravity API requirements.
// The Antigravity API requires tier suffixes for gemini-3-pro models (e.g., gemini-3-pro-low).
// Reference: opencode-antigravity-auth/src/plugin/transform/model-resolver.ts lines 196-217
func transformModelForAntigravity(modelName string) string {
	// First normalize to remove any existing prefixes/suffixes
	normalized := NormalizeModelName(modelName)
	lowerModel := strings.ToLower(normalized)

	// Check if this is gemini-3-pro without a tier suffix
	if strings.HasPrefix(lowerModel, "gemini-3-pro") {
		// Check if it already has a tier suffix
		hasTierSuffix := strings.HasSuffix(lowerModel, "-low") ||
			strings.HasSuffix(lowerModel, "-medium") ||
			strings.HasSuffix(lowerModel, "-high")

		if !hasTierSuffix {
			// Append default tier (-low) as per reference implementation
			return normalized + "-low"
		}
	}

	return normalized
}
