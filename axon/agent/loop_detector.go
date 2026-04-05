package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
)

const (
	// defaultToolCallLoopThreshold is the number of consecutive identical
	// tool calls required to flag a potential loop.
	defaultToolCallLoopThreshold = 5

	// defaultMaxLoopRecoveries is the maximum number of times the agent
	// will attempt to recover from detected loops before hard-stopping.
	defaultMaxLoopRecoveries = 1
)

// LoopType classifies the kind of detected loop.
type LoopType string

const (
	// LoopTypeToolCall indicates consecutive identical tool calls.
	LoopTypeToolCall LoopType = "consecutive_identical_tool_calls"
)

// LoopDetection holds the result of a loop check.
type LoopDetection struct {
	// Detected is true if a loop was found.
	Detected bool
	// Type classifies the loop (only meaningful when Detected is true).
	Type LoopType
	// Detail is a human-readable description of the detection.
	Detail string
	// Count is the cumulative number of times a loop has been detected
	// within the current request (used for recovery decisions).
	Count int
}

// LoopDetectorConfig configures the loop detector.
type LoopDetectorConfig struct {
	// Enabled controls whether loop detection is active. Default true.
	Enabled bool
	// ToolCallThreshold is the number of consecutive identical tool calls
	// before flagging a loop. Default 5.
	ToolCallThreshold int
	// MaxRecoveries is the maximum number of recovery attempts before
	// the loop detector signals a hard stop. Default 1.
	MaxRecoveries int
}

// DefaultLoopDetectorConfig returns the default configuration.
func DefaultLoopDetectorConfig() LoopDetectorConfig {
	return LoopDetectorConfig{
		Enabled:           true,
		ToolCallThreshold: defaultToolCallLoopThreshold,
		MaxRecoveries:     defaultMaxLoopRecoveries,
	}
}

// loopDetector tracks tool call patterns to detect infinite loops.
type loopDetector struct {
	cfg LoopDetectorConfig

	mu sync.Mutex

	// Tool call repetition tracking.
	lastToolCallHash      string
	toolCallRepeatCount   int
	consecutiveDetections int
}

// newLoopDetector creates a loop detector with the given configuration.
func newLoopDetector(cfg LoopDetectorConfig) *loopDetector {
	if cfg.ToolCallThreshold <= 0 {
		cfg.ToolCallThreshold = defaultToolCallLoopThreshold
	}

	if cfg.MaxRecoveries < 0 {
		cfg.MaxRecoveries = defaultMaxLoopRecoveries
	}

	return &loopDetector{cfg: cfg}
}

// checkToolCall checks whether the given tool call forms a repetitive
// pattern with prior calls. Returns a LoopDetection indicating whether
// intervention is needed.
func (ld *loopDetector) checkToolCall(name, input string) LoopDetection {
	if !ld.cfg.Enabled {
		return LoopDetection{}
	}

	ld.mu.Lock()
	defer ld.mu.Unlock()

	hash := hashToolCall(name, input)

	if ld.lastToolCallHash == hash {
		ld.toolCallRepeatCount++
	} else {
		ld.lastToolCallHash = hash
		ld.toolCallRepeatCount = 1
	}

	if ld.toolCallRepeatCount >= ld.cfg.ToolCallThreshold {
		ld.consecutiveDetections++
		// Reset repeat counter so the next batch of identical calls
		// starts fresh after a recovery attempt.
		ld.toolCallRepeatCount = 0

		return LoopDetection{
			Detected: true,
			Type:     LoopTypeToolCall,
			Detail: fmt.Sprintf(
				"tool %q called %d consecutive times with identical arguments",
				name, ld.cfg.ToolCallThreshold,
			),
			Count: ld.consecutiveDetections,
		}
	}

	return LoopDetection{}
}

// canRecover returns true if the detector has not exhausted its recovery
// budget and the agent should attempt self-correction.
func (ld *loopDetector) canRecover() bool {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	return ld.consecutiveDetections <= ld.cfg.MaxRecoveries
}

// reset clears all tracking state (e.g. for a new request).
func (ld *loopDetector) reset() {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	ld.lastToolCallHash = ""
	ld.toolCallRepeatCount = 0
	ld.consecutiveDetections = 0
}

// hashToolCall produces a deterministic hash of a tool name + arguments.
func hashToolCall(name, input string) string {
	h := sha256.New()
	h.Write([]byte(name))
	h.Write([]byte{0}) // separator
	h.Write([]byte(input))

	return hex.EncodeToString(h.Sum(nil))
}
