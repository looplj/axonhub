package subscription

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlanFromAccessToken(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		expected string
	}{
		{name: "heavy numeric tier", payload: `{"tier":5}`, expected: "SuperGrok Heavy"},
		{name: "lite numeric string", payload: `{"tier":"6"}`, expected: "SuperGrok Lite"},
		{name: "plus label", payload: `{"tier":"supergrok-plus"}`, expected: "SuperGrok Plus"},
		{name: "unknown tier", payload: `{"tier":"enterprise"}`, expected: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			token := "header." + base64.RawURLEncoding.EncodeToString([]byte(test.payload)) + ".signature"

			// When
			plan := PlanFromAccessToken(token)

			// Then
			require.Equal(t, test.expected, plan)
		})
	}
}

func TestPlanFromAccessTokenRejectsOpaqueToken(t *testing.T) {
	// Given
	token := "opaque-access-token"

	// When
	plan := PlanFromAccessToken(token)

	// Then
	require.Empty(t, plan)
}
