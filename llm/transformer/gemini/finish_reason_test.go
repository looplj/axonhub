package gemini

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConvertGeminiFinishReasonToLLM_Language(t *testing.T) {
	got := convertGeminiFinishReasonToLLM("LANGUAGE", false)
	require.NotNil(t, got)
	require.Equal(t, "content_filter", *got)
}
