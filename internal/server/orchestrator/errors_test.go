package orchestrator

import "testing"

func TestQuotaExhaustedError(t *testing.T) {
	err := NewQuotaExhaustedError("gpt-4")
	msg := err.Error()
	expected := "all channels quota exhausted for model gpt-4"
	if msg != expected {
		t.Fatalf("unexpected message: got %q, want %q", msg, expected)
	}
	t.Log("PASS: error message correct:", msg)
}
