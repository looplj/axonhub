package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNewProviderAcceptsPublishedOltpHTTPSpelling(t *testing.T) {
	var sawMetricsPath atomic.Bool
	var unexpectedPath atomic.Value

	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/metrics" {
			sawMetricsPath.Store(true)
		} else {
			unexpectedPath.Store(r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(collector.Close)

	provider, err := NewProvider(Config{
		Enabled: true,
		Exporter: ExporterConfig{
			Type:     "oltphttp",
			Endpoint: strings.TrimPrefix(collector.URL, "http://"),
			Insecure: true,
		},
	})
	if err != nil {
		t.Fatalf("NewProvider should accept the published oltphttp spelling: %v", err)
	}
	if provider == nil {
		t.Fatal("NewProvider returned nil provider")
	}

	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("failed to shutdown provider: %v", err)
	}
	if path, ok := unexpectedPath.Load().(string); ok {
		t.Fatalf("unexpected metrics export path: %s", path)
	}
	if !sawMetricsPath.Load() {
		t.Fatal("metrics exporter did not call /v1/metrics")
	}
}
