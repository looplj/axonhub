package shared

import (
	"errors"
	"testing"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/stretchr/testify/require"
)

func TestResponsesRecoveryPreservesHTTPError(t *testing.T) {
	// Given
	err := ResponsesRecoveryError()
	// When
	var wire *httpclient.Error
	matched := errors.As(err, &wire)
	// Then
	require.ErrorIs(t, err, ErrResponsesContinuationUnavailable)
	require.True(t, matched)
	require.Equal(t, 400, wire.StatusCode)
}

func TestResponsesBudgetRejectsAggregateOverflow(t *testing.T) {
	// Given
	budget := NewResponsesBudget(64 << 20)
	require.True(t, budget.Replace(0, 63<<20))
	// When
	accepted := budget.Replace(0, (1<<20)+1)
	// Then
	require.False(t, accepted)
	require.True(t, budget.Replace(63<<20, 0))
	require.True(t, budget.Replace(0, 64<<20))
}

func TestResponsesProviderErrorIsRecovery(t *testing.T) {
	// Given
	err := &httpclient.Error{StatusCode: 400, Body: []byte(`{"error":{"code":"previous_response_not_found"}}`)}
	// When
	recovery := IsResponsesRecoveryError(err)
	// Then
	require.True(t, recovery)
}
