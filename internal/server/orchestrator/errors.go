package orchestrator

import (
	"fmt"
	"net/http"

	"github.com/looplj/axonhub/llm"
)

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

// NewNoMatchingAssociationError is returned when a model has associations but
// none of their `when` conditions match the request.
func NewNoMatchingAssociationError(modelName string) error {
	return &llm.ResponseError{
		StatusCode: http.StatusUnprocessableEntity,
		Detail: llm.ErrorDetail{
			Message: fmt.Sprintf("no association of model %s matches this request", modelName),
			Type:    "invalid_request_error",
			Code:    "no_matching_association",
		},
	}
}
