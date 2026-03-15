package claw

import "testing"

func TestPromptActionNormalizedModeDefaultsToIsolated(t *testing.T) {
	action := PromptAction{Message: "check in"}

	if got := action.NormalizedMode(); got != PromptModeIsolated {
		t.Fatalf("NormalizedMode() = %q, want %q", got, PromptModeIsolated)
	}
}

func TestPromptActionValidateRejectsUnknownMode(t *testing.T) {
	action := PromptAction{
		Message: "check in",
		Mode:    "shared",
	}

	if err := action.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}
