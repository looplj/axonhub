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
