package orchestrator

import "fmt"

// QuotaExhaustedError is returned when all channels are quota exhausted for a model.
type QuotaExhaustedError struct {
	ModelName string
}

func (e *QuotaExhaustedError) Error() string {
	return fmt.Sprintf("all channels quota exhausted for model %s", e.ModelName)
}

func NewQuotaExhaustedError(modelName string) error {
	return &QuotaExhaustedError{ModelName: modelName}
}

// StreamPolicyConflictError indicates that matching channels require streaming
// for a request type that cannot be auto-aggregated.
type StreamPolicyConflictError struct {
	ModelName string
}

func (e *StreamPolicyConflictError) Error() string {
	return fmt.Sprintf(
		"no eligible channel for model %s: matching channels require streaming, but Decisions requests are non-streaming",
		e.ModelName,
	)
}

func NewStreamPolicyConflictError(modelName string) error {
	return &StreamPolicyConflictError{ModelName: modelName}
}
