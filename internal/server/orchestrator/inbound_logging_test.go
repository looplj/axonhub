package orchestrator

import "testing"

func TestURLForInboundLogInvalidURLDoesNotExposeRawValue(t *testing.T) {
	const raw = "https://example.invalid/path?token=secret%ZZ"

	if got := urlForInboundLog(raw); got != "<invalid-url>" {
		t.Fatalf("urlForInboundLog(%q) = %q, want <invalid-url>", raw, got)
	}
}
