package biz

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOverrideEnv_ReplacesExistingKey(t *testing.T) {
	base := []string{
		"A=1",
		"AXONCLAW_API_KEY=old",
		"B=2",
	}

	out := overrideEnv(base, "AXONCLAW_API_KEY", "new")

	require.Contains(t, out, "A=1")
	require.Contains(t, out, "B=2")
	require.Contains(t, out, "AXONCLAW_API_KEY=new")
	require.NotContains(t, out, "AXONCLAW_API_KEY=old")
}

func TestOverrideEnv_AddsWhenMissing(t *testing.T) {
	base := []string{"A=1"}

	out := overrideEnv(base, "AXONCLAW_NAME", "worker-1")

	require.Contains(t, out, "A=1")
	require.Contains(t, out, "AXONCLAW_NAME=worker-1")
}
