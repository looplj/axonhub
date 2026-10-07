package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestValidateEndpoints_AllowsManualDecisionsEndpoint(t *testing.T) {
	// Given a manually configured Decisions endpoint
	endpoints := []objects.ChannelEndpoint{{APIFormat: llm.APIFormatOpenAIDecisions.String()}}

	// When endpoint configuration is validated
	err := ValidateEndpoints(endpoints)

	// Then the opt-in format is accepted without becoming a default
	require.NoError(t, err)
}
