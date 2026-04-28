package provider_quota

import (
	"net/url"
	"strings"
)

type urlProviderEntry struct {
	hostPattern  string
	providerType string
}

// urlProviderMap maps host suffixes to provider types. More specific patterns first.
var urlProviderMap = []urlProviderEntry{
	{hostPattern: "wafer.ai", providerType: "wafer"},
	{hostPattern: "api.synthetic.new", providerType: "synthetic"},
	{hostPattern: "api.neuralwatt.com", providerType: "neuralwatt"},
}

func URLDetectedProviders() map[string]struct{} {
	providers := make(map[string]struct{}, len(urlProviderMap))
	for _, entry := range urlProviderMap {
		providers[entry.providerType] = struct{}{}
	}

	return providers
}

// DetectProviderFromURL returns the provider type for a base URL, or "" if unrecognized.
func DetectProviderFromURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return ""
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}

	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return ""
	}

	for _, entry := range urlProviderMap {
		pattern := strings.ToLower(entry.hostPattern)
		if host == pattern || strings.HasSuffix(host, "."+pattern) {
			return entry.providerType
		}
	}

	return ""
}
