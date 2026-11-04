package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestPassThroughReasoningEffortMappingChanges(t *testing.T) {
	dropNone := []llm.ReasoningEffortMapping{{From: llm.ReasoningEffortNone, To: ""}}

	require.True(t, passThroughReasoningEffortMappingChanges([]byte(`{"reasoning_effort":"none"}`), dropNone))
	require.False(t, passThroughReasoningEffortMappingChanges([]byte(`{"reasoning_effort":"high"}`), dropNone))
	require.False(t, passThroughReasoningEffortMappingChanges([]byte(`{"messages":[]}`), dropNone))
	require.False(t, passThroughReasoningEffortMappingChanges([]byte(`{"reasoning_effort":"none"}`), nil))
	require.False(t, passThroughReasoningEffortMappingChanges(nil, dropNone))
}

func TestPassThroughReasoningEffortMappingRewritesValue(t *testing.T) {
	rewrite := []llm.ReasoningEffortMapping{{From: llm.ReasoningEffortNone, To: llm.ReasoningEffortLow}}

	require.True(t, passThroughReasoningEffortMappingChanges([]byte(`{"reasoning_effort":"none"}`), rewrite))
}
