package metrics

import (
	"testing"
)

func TestNewProvider_DisabledReturnsNoOp(t *testing.T) {
	cfg := Config{Enabled: false}

	provider, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNewProvider_UnsupportedExporterType(t *testing.T) {
	cfg := Config{
		Enabled: true,
		Exporter: ExporterConfig{
			Type: "",
		},
	}

	_, err := NewProvider(cfg)
	if err == nil {
		t.Fatal("expected error for empty exporter type, got nil")
	}
}

func TestNewProvider_UnknownExporterType(t *testing.T) {
	cfg := Config{
		Enabled: true,
		Exporter: ExporterConfig{
			Type: "unknown",
		},
	}

	_, err := NewProvider(cfg)
	if err == nil {
		t.Fatal("expected error for unknown exporter type, got nil")
	}
}
