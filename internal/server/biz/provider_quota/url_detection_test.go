package provider_quota

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectProviderFromURL_Wafer(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
	}{
		{"exact domain", "https://wafer.ai"},
		{"subdomain", "https://pass.wafer.ai"},
		{"with path", "https://api.wafer.ai/v1/chat"},
		{"http scheme", "http://wafer.ai"},
		{"with port", "https://pass.wafer.ai:443"},
		{"with non-standard port", "https://pass.wafer.ai:8443"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectProviderFromURL(tt.baseURL)
			require.Equal(t, "wafer", result)
		})
	}
}

func TestDetectProviderFromURL_Synthetic(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
	}{
		{"exact domain", "https://api.synthetic.new"},
		{"subdomain", "https://us-east.api.synthetic.new"},
		{"with path", "https://api.synthetic.new/v1/chat/completions"},
		{"with port", "https://api.synthetic.new:443"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectProviderFromURL(tt.baseURL)
			require.Equal(t, "synthetic", result)
		})
	}
}

func TestDetectProviderFromURL_NeuralWatt(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
	}{
		{"exact domain", "https://api.neuralwatt.com"},
		{"subdomain", "https://us.api.neuralwatt.com"},
		{"with path", "https://api.neuralwatt.com/v1"},
		{"with port", "https://api.neuralwatt.com:443"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectProviderFromURL(tt.baseURL)
			require.Equal(t, "neuralwatt", result)
		})
	}
}

func TestDetectProviderFromURL_Unknown(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
	}{
		{"unknown domain", "https://api.unknown-provider.com"},
		{"openai domain", "https://api.openai.com"},
		{"generic domain", "https://example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectProviderFromURL(tt.baseURL)
			require.Equal(t, "", result)
		})
	}
}

func TestDetectProviderFromURL_Empty(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectProviderFromURL(tt.baseURL)
			require.Equal(t, "", result)
		})
	}
}

func TestDetectProviderFromURL_Malformed(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
	}{
		{"missing scheme", "wafer.ai"},
		{"just host no scheme", "api.synthetic.new"},
		{"garbage", "://invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectProviderFromURL(tt.baseURL)
			require.Equal(t, "", result)
		})
	}
}

func TestDetectProviderFromURL_FalsePositives(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
	}{
		{"evil wafer", "https://evilwafer.ai"},
		{"fake synthetic", "https://fakeapi.synthetic.new"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectProviderFromURL(tt.baseURL)
			require.Equal(t, "", result)
		})
	}
}
