package objects

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

func TestModelCardWithDelegatedVision(t *testing.T) {
	original := ModelCard{
		Modalities: ModelCardModalities{
			Input:  []string{"text"},
			Output: []string{"text"},
		},
	}
	settings := &ModelSettings{
		VisionDelegation: VisionDelegation{
			Enabled:       true,
			TargetModelID: lo.ToPtr("vision-model"),
		},
	}

	effective := original.WithDelegatedVision(settings)
	require.True(t, effective.Vision)
	require.Equal(t, []string{"text", "image"}, effective.Modalities.Input)
	require.False(t, original.Vision)
	require.Equal(t, []string{"text"}, original.Modalities.Input)

	settings.VisionDelegation.Enabled = false
	require.Equal(t, original, original.WithDelegatedVision(settings))
}

func TestModelCardSupportsVision(t *testing.T) {
	require.True(t, ModelCard{Vision: true}.SupportsVision())
	require.True(t, ModelCard{Modalities: ModelCardModalities{Input: []string{"text", "image"}}}.SupportsVision())
	require.False(t, ModelCard{Modalities: ModelCardModalities{Input: []string{"text"}}}.SupportsVision())
}
