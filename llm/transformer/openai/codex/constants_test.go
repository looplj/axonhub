package codex

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultModels(t *testing.T) {
	require.Equal(t, []string{
		"gpt-5.6-sol",
		"gpt-5.6-sol-fast",
		"gpt-5.6-terra",
		"gpt-5.6-terra-fast",
		"gpt-5.6-luna",
		"gpt-5.6-luna-fast",
		"gpt-6-astra",
		"gpt-6-astra-fast",
		"gpt-6-sol",
		"gpt-6-sol-fast",
		"gpt-6-luna",
		"gpt-6-luna-fast",
		"codex-auto-review",
	}, DefaultModels())
}
