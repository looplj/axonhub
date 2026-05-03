package responses

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPresenceOverlay_OnlyPresentStructuredFieldsOverwriteRaw(t *testing.T) {
	raw := map[string]any{
		"model":  "raw-model",
		"status": "completed",
		"usage":  map[string]any{"input_tokens": float64(1)},
		"input":  []any{"raw-input"},
		"tools":  []any{map[string]any{"type": "function"}},
	}

	overlay := NewPresenceOverlay(raw)
	overlay.Set("model", "")
	overlay.Set("tools", []any{})

	merged := overlay.Merge()
	require.Equal(t, "", merged["model"])
	require.Equal(t, []any{}, merged["tools"])
	require.Equal(t, "completed", merged["status"])
	require.Equal(t, map[string]any{"input_tokens": float64(1)}, merged["usage"])
	require.Equal(t, []any{"raw-input"}, merged["input"])
}
