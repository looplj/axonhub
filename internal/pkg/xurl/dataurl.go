// Package xurl provides utilities for URL parsing and manipulation.
package xurl

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// DefaultMaxDataURL is the default maximum allowed size for decoded data URL content (10MB).
const DefaultMaxDataURL = 10 * 1024 * 1024

// DataURL represents a parsed data URL with its components.
type DataURL struct {
	// MediaType is the MIME type (e.g., "image/png", "text/plain").
	MediaType string
	// Data is the base64-encoded or raw data portion.
	Data string
	// IsBase64 indicates whether the data is base64-encoded.
	IsBase64 bool
}

// allowedMediaTypes defines the permitted top-level media types for data URLs.
// Only image/*, text/plain, and application/json are allowed to prevent
// injection of executable content (e.g., text/html, application/javascript).
var allowedMediaTypes = map[string]bool{
	"image":            true,
	"text/plain":       true,
	"application/json": true,
}

// validateMediaType checks whether the given MIME type is in the allowlist.
// It matches exact types (e.g., "text/plain") and top-level wildcards (e.g., "image/*").
func validateMediaType(mediaType string) error {
	if mediaType == "" {
		return nil // default text/plain is allowed
	}
	// Exact match
	if allowedMediaTypes[mediaType] {
		return nil
	}
	// Wildcard match for image/*, audio/*, video/* etc.
	topLevel, _, _ := strings.Cut(mediaType, "/")
	if allowedMediaTypes[topLevel] {
		return nil
	}
	return fmt.Errorf("data URL media type %q is not allowed", mediaType)
}

// ParseDataURL parses a data URL and returns its components.
// Returns nil if the URL is not a valid data URL.
// Rejects data URLs whose decoded content exceeds DefaultMaxDataURL bytes.
//
// Data URL format: data:[<mediatype>][;base64],<data>
// Examples:
//   - data:image/png;base64,iVBORw0KGgo...
//   - data:text/plain,Hello%20World
func ParseDataURL(url string) *DataURL {
	return ParseDataURLWithLimit(url, DefaultMaxDataURL)
}

// ParseDataURLWithLimit parses a data URL with an explicit size limit.
// Returns nil if the URL is not valid or exceeds the size limit.
func ParseDataURLWithLimit(url string, maxDataSize int) *DataURL {
	if !strings.HasPrefix(url, "data:") {
		return nil
	}

	parts := strings.SplitN(url, ",", 2)
	if len(parts) != 2 {
		return nil
	}

	header := parts[0]
	data := parts[1]

	// Enforce size limit on the raw data portion.
	if len(data) > maxDataSize {
		return nil
	}

	headerParts := strings.Split(header, ";")
	if len(headerParts) == 0 {
		return nil
	}

	mediaType := strings.TrimPrefix(headerParts[0], "data:")
	if mediaType == "" {
		mediaType = "text/plain"
	}

	isBase64 := false

	for _, part := range headerParts[1:] {
		if strings.TrimSpace(part) == "base64" {
			isBase64 = true
			break
		}
	}

	// For base64, verify decoded size does not exceed limit.
	if isBase64 {
		decodedLen := base64.StdEncoding.DecodedLen(len(data))
		if decodedLen > maxDataSize {
			return nil
		}
	}

	// Reject unsupported media types
	if err := validateMediaType(mediaType); err != nil {
		return nil
	}

	return &DataURL{
		MediaType: mediaType,
		Data:      data,
		IsBase64:  isBase64,
	}
}

// IsDataURL checks if the given URL is a data URL.
func IsDataURL(url string) bool {
	return strings.HasPrefix(url, "data:")
}

// ExtractBase64FromDataURL extracts the base64 data from a data URL.
// If the URL is not a data URL, returns the original URL unchanged.
func ExtractBase64FromDataURL(url string) string {
	if !strings.HasPrefix(url, "data:") {
		return url
	}

	parts := strings.SplitN(url, ",", 2)
	if len(parts) == 2 {
		return parts[1]
	}

	return url
}

// ExtractMediaTypeFromDataURL extracts the media type from a data URL.
// Returns empty string if the URL is not a valid data URL.
func ExtractMediaTypeFromDataURL(url string) string {
	parsed := ParseDataURL(url)
	if parsed == nil {
		return ""
	}

	return parsed.MediaType
}

// BuildDataURL constructs a data URL from media type and data.
// If isBase64 is true, adds ";base64" to the URL.
// If mediaType is empty, uses "text/plain" as default per RFC 2397.
func BuildDataURL(mediaType string, data string, isBase64 bool) string {
	if mediaType == "" {
		mediaType = "text/plain"
	}

	if isBase64 {
		return "data:" + mediaType + ";base64," + data
	}

	return "data:" + mediaType + "," + data
}

// ValidateDataURLSize checks whether a data URL's decoded content fits within the given limit.
// Returns an error if the data URL exceeds maxDataSize bytes.
func ValidateDataURLSize(url string, maxDataSize int) error {
	if !IsDataURL(url) {
		return nil
	}

	parts := strings.SplitN(url, ",", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid data URL format")
	}

	data := parts[1]
	if len(data) > maxDataSize {
		return fmt.Errorf("data URL exceeds size limit (%d bytes > %d bytes)", len(data), maxDataSize)
	}

	header := parts[0]
	isBase64 := strings.Contains(header, ";base64")
	if isBase64 {
		decodedLen := base64.StdEncoding.DecodedLen(len(data))
		if decodedLen > maxDataSize {
			return fmt.Errorf("data URL decoded content exceeds size limit (%d bytes > %d bytes)", decodedLen, maxDataSize)
		}
	}

	return nil
}
