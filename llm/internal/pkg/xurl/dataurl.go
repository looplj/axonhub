// Package xurl provides utilities for URL parsing and manipulation.
package xurl

import "strings"

// DataURL represents a parsed data URL with its components.
type DataURL struct {
	// MediaType is the MIME type (e.g., "image/png", "text/plain").
	MediaType string
	// Data is the base64-encoded or raw data portion.
	Data string
	// IsBase64 indicates whether the data is base64-encoded.
	IsBase64 bool
}

// ParseDataURL parses a data URL and returns its components.
// Returns nil if the URL is not a valid data URL.
//
// Data URL format: data:[<mediatype>][;base64],<data>
// Examples:
//   - data:image/png;base64,iVBORw0KGgo...
//   - data:text/plain,Hello%20World
func ParseDataURL(url string) *DataURL {
	if !strings.HasPrefix(url, "data:") {
		return nil
	}

	// Split into header and data parts
	parts := strings.SplitN(url, ",", 2)
	if len(parts) != 2 {
		return nil
	}

	header := parts[0]
	data := parts[1]

	// Parse header: data:[<mediatype>][;base64]
	headerParts := strings.Split(header, ";")
	if len(headerParts) == 0 {
		return nil
	}

	mediaType := strings.TrimPrefix(headerParts[0], "data:")
	if mediaType == "" {
		mediaType = "text/plain" // Default media type per RFC 2397
	}

	isBase64 := false

	for _, part := range headerParts[1:] {
		if strings.TrimSpace(part) == "base64" {
			isBase64 = true
			break
		}
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
