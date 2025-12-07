package xurl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDataURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected *DataURL
	}{
		{
			name: "valid base64 image png",
			url:  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
			expected: &DataURL{
				MediaType: "image/png",
				Data:      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
				IsBase64:  true,
			},
		},
		{
			name: "valid base64 image jpeg",
			url:  "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQABAAD",
			expected: &DataURL{
				MediaType: "image/jpeg",
				Data:      "/9j/4AAQSkZJRgABAQAAAQABAAD",
				IsBase64:  true,
			},
		},
		{
			name: "valid base64 image webp",
			url:  "data:image/webp;base64,UklGRh4AAABXRUJQVlA4TBEAAAAvAAAAAAfQ//73v/+BiOh/AAA=",
			expected: &DataURL{
				MediaType: "image/webp",
				Data:      "UklGRh4AAABXRUJQVlA4TBEAAAAvAAAAAAfQ//73v/+BiOh/AAA=",
				IsBase64:  true,
			},
		},
		{
			name: "valid base64 image gif",
			url:  "data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7",
			expected: &DataURL{
				MediaType: "image/gif",
				Data:      "R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7",
				IsBase64:  true,
			},
		},
		{
			name: "valid plain text without base64",
			url:  "data:text/plain,Hello%20World",
			expected: &DataURL{
				MediaType: "text/plain",
				Data:      "Hello%20World",
				IsBase64:  false,
			},
		},
		{
			name: "valid text with charset and base64",
			url:  "data:text/plain;charset=utf-8;base64,SGVsbG8gV29ybGQ=",
			expected: &DataURL{
				MediaType: "text/plain",
				Data:      "SGVsbG8gV29ybGQ=",
				IsBase64:  true,
			},
		},
		{
			name: "valid html content",
			url:  "data:text/html;base64,PGh0bWw+PC9odG1sPg==",
			expected: &DataURL{
				MediaType: "text/html",
				Data:      "PGh0bWw+PC9odG1sPg==",
				IsBase64:  true,
			},
		},
		{
			name: "valid json content",
			url:  "data:application/json;base64,eyJrZXkiOiJ2YWx1ZSJ9",
			expected: &DataURL{
				MediaType: "application/json",
				Data:      "eyJrZXkiOiJ2YWx1ZSJ9",
				IsBase64:  true,
			},
		},
		{
			name: "valid pdf content",
			url:  "data:application/pdf;base64,JVBERi0xLjQK",
			expected: &DataURL{
				MediaType: "application/pdf",
				Data:      "JVBERi0xLjQK",
				IsBase64:  true,
			},
		},
		{
			name: "valid audio content",
			url:  "data:audio/mp3;base64,//uQxAAAAAANIAAAAAExBTUUzLjEwMFVVVVVVVVVVVVVV",
			expected: &DataURL{
				MediaType: "audio/mp3",
				Data:      "//uQxAAAAAANIAAAAAExBTUUzLjEwMFVVVVVVVVVVVVVV",
				IsBase64:  true,
			},
		},
		{
			name: "default media type when empty",
			url:  "data:;base64,SGVsbG8=",
			expected: &DataURL{
				MediaType: "text/plain",
				Data:      "SGVsbG8=",
				IsBase64:  true,
			},
		},
		{
			name: "data with comma in content",
			url:  "data:text/plain,Hello,World",
			expected: &DataURL{
				MediaType: "text/plain",
				Data:      "Hello,World",
				IsBase64:  false,
			},
		},
		{
			name:     "not a data URL - http",
			url:      "http://example.com/image.png",
			expected: nil,
		},
		{
			name:     "not a data URL - https",
			url:      "https://example.com/image.png",
			expected: nil,
		},
		{
			name:     "not a data URL - file",
			url:      "file:///path/to/file.png",
			expected: nil,
		},
		{
			name:     "empty string",
			url:      "",
			expected: nil,
		},
		{
			name:     "data URL without comma",
			url:      "data:image/png;base64",
			expected: nil,
		},
		{
			name: "data URL with empty data",
			url:  "data:image/png;base64,",
			expected: &DataURL{
				MediaType: "image/png",
				Data:      "",
				IsBase64:  true,
			},
		},
		{
			name: "data URL with multiple semicolons",
			url:  "data:image/svg+xml;charset=utf-8;base64,PHN2Zz48L3N2Zz4=",
			expected: &DataURL{
				MediaType: "image/svg+xml",
				Data:      "PHN2Zz48L3N2Zz4=",
				IsBase64:  true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseDataURL(tt.url)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.MediaType, result.MediaType)
				assert.Equal(t, tt.expected.Data, result.Data)
				assert.Equal(t, tt.expected.IsBase64, result.IsBase64)
			}
		})
	}
}

func TestIsDataURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "valid data URL",
			url:      "data:image/png;base64,iVBORw0KGgo=",
			expected: true,
		},
		{
			name:     "data URL with text",
			url:      "data:text/plain,Hello",
			expected: true,
		},
		{
			name:     "http URL",
			url:      "http://example.com",
			expected: false,
		},
		{
			name:     "https URL",
			url:      "https://example.com",
			expected: false,
		},
		{
			name:     "empty string",
			url:      "",
			expected: false,
		},
		{
			name:     "data prefix only",
			url:      "data:",
			expected: true,
		},
		{
			name:     "DATA uppercase",
			url:      "DATA:image/png;base64,abc",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsDataURL(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractBase64FromDataURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "valid data URL with base64",
			url:      "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
			expected: "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
		},
		{
			name:     "valid data URL with plain text",
			url:      "data:text/plain,Hello%20World",
			expected: "Hello%20World",
		},
		{
			name:     "http URL returns unchanged",
			url:      "http://example.com/image.png",
			expected: "http://example.com/image.png",
		},
		{
			name:     "https URL returns unchanged",
			url:      "https://example.com/image.png",
			expected: "https://example.com/image.png",
		},
		{
			name:     "empty string returns unchanged",
			url:      "",
			expected: "",
		},
		{
			name:     "data URL without comma returns unchanged",
			url:      "data:image/png;base64",
			expected: "data:image/png;base64",
		},
		{
			name:     "data URL with empty data",
			url:      "data:image/png;base64,",
			expected: "",
		},
		{
			name:     "data URL with comma in data",
			url:      "data:text/plain,Hello,World",
			expected: "Hello,World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractBase64FromDataURL(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractMediaTypeFromDataURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "image/png",
			url:      "data:image/png;base64,iVBORw0KGgo=",
			expected: "image/png",
		},
		{
			name:     "image/jpeg",
			url:      "data:image/jpeg;base64,/9j/4AAQ=",
			expected: "image/jpeg",
		},
		{
			name:     "image/webp",
			url:      "data:image/webp;base64,UklGRh4=",
			expected: "image/webp",
		},
		{
			name:     "image/gif",
			url:      "data:image/gif;base64,R0lGODlh=",
			expected: "image/gif",
		},
		{
			name:     "image/svg+xml",
			url:      "data:image/svg+xml;base64,PHN2Zz4=",
			expected: "image/svg+xml",
		},
		{
			name:     "text/plain",
			url:      "data:text/plain,Hello",
			expected: "text/plain",
		},
		{
			name:     "text/html",
			url:      "data:text/html;base64,PGh0bWw+",
			expected: "text/html",
		},
		{
			name:     "application/json",
			url:      "data:application/json;base64,e30=",
			expected: "application/json",
		},
		{
			name:     "application/pdf",
			url:      "data:application/pdf;base64,JVBERi0=",
			expected: "application/pdf",
		},
		{
			name:     "audio/mp3",
			url:      "data:audio/mp3;base64,//uQxAA=",
			expected: "audio/mp3",
		},
		{
			name:     "default media type when empty",
			url:      "data:;base64,SGVsbG8=",
			expected: "text/plain",
		},
		{
			name:     "http URL returns empty",
			url:      "http://example.com",
			expected: "",
		},
		{
			name:     "empty string returns empty",
			url:      "",
			expected: "",
		},
		{
			name:     "invalid data URL returns empty",
			url:      "data:image/png;base64",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractMediaTypeFromDataURL(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}
