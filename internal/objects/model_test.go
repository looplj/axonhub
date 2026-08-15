package objects

import (
	"encoding/json"
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

func TestModelSettingsUnsupportedImageFallbackJSONCompatibility(t *testing.T) {
	var legacy ModelSettings
	require.NoError(t, json.Unmarshal([]byte(`{"visionDelegation":{"enabled":false}}`), &legacy))
	require.False(t, legacy.UnsupportedImageFallback.Enabled)

	var configured ModelSettings
	require.NoError(t, json.Unmarshal([]byte(`{"unsupportedImageFallback":{"enabled":true}}`), &configured))
	require.True(t, configured.UnsupportedImageFallback.Enabled)
}
