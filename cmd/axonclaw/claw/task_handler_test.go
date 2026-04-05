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

func TestPromptActionValidateRejectsModelInMainMode(t *testing.T) {
	action := PromptAction{
		Message: "check in",
		Mode:    PromptModeMain,
		Model:   "gpt-4",
	}

	if err := action.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error for model in main mode")
	}
}

func TestPromptActionValidateAcceptsModelInIsolatedMode(t *testing.T) {
	action := PromptAction{
		Message: "check in",
		Mode:    PromptModeIsolated,
		Model:   "gpt-4",
	}

	if err := action.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestPromptActionValidateAcceptsEmptyModelInIsolatedMode(t *testing.T) {
	action := PromptAction{
		Message: "check in",
		Mode:    PromptModeIsolated,
	}

	if err := action.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}
