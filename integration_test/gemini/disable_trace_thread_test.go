package main

import (
	"os"
	"testing"

	"github.com/looplj/axonhub/gemini_test/internal/testutil"
)

func TestDisableTraceAndThread(t *testing.T) {
	// Test with both trace and thread disabled
	os.Setenv("TEST_DISABLE_TRACE", "true")
	os.Setenv("TEST_DISABLE_THREAD", "true")
	os.Setenv("TEST_AXONHUB_API_KEY", "test-key")

	defer func() {
		os.Unsetenv("TEST_DISABLE_TRACE")
		os.Unsetenv("TEST_DISABLE_THREAD")
		os.Unsetenv("TEST_AXONHUB_API_KEY")
	}()

	config := testutil.DefaultConfig()

	// Verify the configuration
	if !config.DisableTrace {
		t.Error("Expected DisableTrace to be true")
	}

	if !config.DisableThread {
		t.Error("Expected DisableThread to be true")
	}

	// Verify trace and thread IDs are empty
	if config.TraceID != "" {
		t.Errorf("Expected empty TraceID, got: %s", config.TraceID)
	}

	if config.ThreadID != "" {
		t.Errorf("Expected empty ThreadID, got: %s", config.ThreadID)
	}

	// Verify headers are empty
	headers := config.GetHeaders()
	if len(headers) != 0 {
		t.Errorf("Expected no headers, got: %v", headers)
	}

	// Verify validation passes
	err := config.ValidateConfig()
	if err != nil {
		t.Errorf("Expected validation to pass, got error: %v", err)
	}
}

func TestDisableTraceOnly(t *testing.T) {
	// Test with only trace disabled
	os.Setenv("TEST_DISABLE_TRACE", "true")
	os.Setenv("TEST_AXONHUB_API_KEY", "test-key")

	defer func() {
		os.Unsetenv("TEST_DISABLE_TRACE")
		os.Unsetenv("TEST_AXONHUB_API_KEY")
	}()

	config := testutil.DefaultConfig()

	// Verify the configuration
	if !config.DisableTrace {
		t.Error("Expected DisableTrace to be true")
	}

	if config.DisableThread {
		t.Error("Expected DisableThread to be false")
	}

	// Verify trace ID is empty but thread ID is not
	if config.TraceID != "" {
		t.Errorf("Expected empty TraceID, got: %s", config.TraceID)
	}

	if config.ThreadID == "" {
		t.Error("Expected non-empty ThreadID")
	}

	// Verify headers only contain thread ID
	headers := config.GetHeaders()
	if len(headers) != 1 {
		t.Errorf("Expected 1 header, got: %v", headers)
	}

	if _, exists := headers["AH-Trace-Id"]; exists {
		t.Error("Expected AH-Trace-Id header to be absent")
	}

	if _, exists := headers["AH-Thread-Id"]; !exists {
		t.Error("Expected AH-Thread-Id header to be present")
	}
}

func TestDisableThreadOnly(t *testing.T) {
	// Test with only thread disabled
	os.Setenv("TEST_DISABLE_THREAD", "true")
	os.Setenv("TEST_AXONHUB_API_KEY", "test-key")

	defer func() {
		os.Unsetenv("TEST_DISABLE_THREAD")
		os.Unsetenv("TEST_AXONHUB_API_KEY")
	}()

	config := testutil.DefaultConfig()

	// Verify the configuration
	if config.DisableTrace {
		t.Error("Expected DisableTrace to be false")
	}

	if !config.DisableThread {
		t.Error("Expected DisableThread to be true")
	}

	// Verify thread ID is empty but trace ID is not
	if config.ThreadID != "" {
		t.Errorf("Expected empty ThreadID, got: %s", config.ThreadID)
	}

	if config.TraceID == "" {
		t.Error("Expected non-empty TraceID")
	}

	// Verify headers only contain trace ID
	headers := config.GetHeaders()
	if len(headers) != 1 {
		t.Errorf("Expected 1 header, got: %v", headers)
	}

	if _, exists := headers["AH-Thread-Id"]; exists {
		t.Error("Expected AH-Thread-Id header to be absent")
	}

	if _, exists := headers["AH-Trace-Id"]; !exists {
		t.Error("Expected AH-Trace-Id header to be present")
	}
}