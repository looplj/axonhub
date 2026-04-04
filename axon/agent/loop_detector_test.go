package agent

import (
	"testing"
)

func TestLoopDetector_NoLoopWithDifferentCalls(t *testing.T) {
	ld := newLoopDetector(DefaultLoopDetectorConfig())

	for i := range 10 {
		name := "read_file"
		input := `{"path":"file` + string(rune('0'+i)) + `.txt"}`

		d := ld.checkToolCall(name, input)
		if d.Detected {
			t.Fatalf("false positive on iteration %d", i)
		}
	}
}

func TestLoopDetector_DetectsConsecutiveIdenticalCalls(t *testing.T) {
	ld := newLoopDetector(DefaultLoopDetectorConfig())

	name := "read_file"
	input := `{"path":"same.txt"}`

	for i := range defaultToolCallLoopThreshold - 1 {
		d := ld.checkToolCall(name, input)
		if d.Detected {
			t.Fatalf("premature detection on iteration %d", i)
		}
	}

	d := ld.checkToolCall(name, input)
	if !d.Detected {
		t.Fatal("expected loop detection at threshold")
	}

	if d.Type != LoopTypeToolCall {
		t.Fatalf("unexpected type: %s", d.Type)
	}

	if d.Count != 1 {
		t.Fatalf("expected count 1, got %d", d.Count)
	}
}

func TestLoopDetector_ResetBreaksCount(t *testing.T) {
	ld := newLoopDetector(DefaultLoopDetectorConfig())

	name := "read_file"
	input := `{"path":"same.txt"}`

	// Call threshold-1 times.
	for range defaultToolCallLoopThreshold - 1 {
		ld.checkToolCall(name, input)
	}

	// Different call resets.
	ld.checkToolCall("write_file", `{"path":"other.txt"}`)

	// Same call again: should not trigger because counter reset.
	for i := range defaultToolCallLoopThreshold - 1 {
		d := ld.checkToolCall(name, input)
		if d.Detected {
			t.Fatalf("false positive after reset on iteration %d", i)
		}
	}
}

func TestLoopDetector_CanRecover(t *testing.T) {
	cfg := DefaultLoopDetectorConfig()
	cfg.MaxRecoveries = 2
	ld := newLoopDetector(cfg)

	name := "read_file"
	input := `{"path":"same.txt"}`

	// First detection.
	for i := 0; i < cfg.ToolCallThreshold; i++ {
		ld.checkToolCall(name, input)
	}

	if !ld.canRecover() {
		t.Fatal("should be able to recover after first detection")
	}

	// Second detection.
	for i := 0; i < cfg.ToolCallThreshold; i++ {
		ld.checkToolCall(name, input)
	}

	if !ld.canRecover() {
		t.Fatal("should be able to recover after second detection")
	}

	// Third detection: exhausted.
	for i := 0; i < cfg.ToolCallThreshold; i++ {
		ld.checkToolCall(name, input)
	}

	if ld.canRecover() {
		t.Fatal("should NOT be able to recover after third detection")
	}
}

func TestLoopDetector_Disabled(t *testing.T) {
	cfg := DefaultLoopDetectorConfig()
	cfg.Enabled = false
	ld := newLoopDetector(cfg)

	name := "read_file"
	input := `{"path":"same.txt"}`

	for range 100 {
		d := ld.checkToolCall(name, input)
		if d.Detected {
			t.Fatal("should never detect when disabled")
		}
	}
}

func TestLoopDetector_Reset(t *testing.T) {
	ld := newLoopDetector(DefaultLoopDetectorConfig())

	name := "read_file"
	input := `{"path":"same.txt"}`

	// Trigger a detection.
	for range defaultToolCallLoopThreshold {
		ld.checkToolCall(name, input)
	}

	ld.reset()

	// After reset, same pattern should not trigger until threshold again.
	for i := range defaultToolCallLoopThreshold - 1 {
		d := ld.checkToolCall(name, input)
		if d.Detected {
			t.Fatalf("detection after reset on iteration %d", i)
		}
	}
}

func TestLoopDetector_CustomThreshold(t *testing.T) {
	cfg := LoopDetectorConfig{
		Enabled:           true,
		ToolCallThreshold: 3,
		MaxRecoveries:     1,
	}
	ld := newLoopDetector(cfg)

	name := "bash"
	input := `{"command":"ls"}`

	for i := range 2 {
		d := ld.checkToolCall(name, input)
		if d.Detected {
			t.Fatalf("premature detection on iteration %d", i)
		}
	}

	d := ld.checkToolCall(name, input)
	if !d.Detected {
		t.Fatal("expected detection at custom threshold 3")
	}
}

func TestHashToolCall_Deterministic(t *testing.T) {
	h1 := hashToolCall("read_file", `{"path":"test.txt"}`)

	h2 := hashToolCall("read_file", `{"path":"test.txt"}`)
	if h1 != h2 {
		t.Fatal("hashes should be deterministic")
	}

	h3 := hashToolCall("read_file", `{"path":"other.txt"}`)
	if h1 == h3 {
		t.Fatal("different inputs should produce different hashes")
	}
}

func TestHashToolCall_NameSeparation(t *testing.T) {
	// Ensure "toolA" + "B" != "tool" + "AB"
	h1 := hashToolCall("toolA", "B")

	h2 := hashToolCall("tool", "AB")
	if h1 == h2 {
		t.Fatal("different name/input splits should produce different hashes")
	}
}
