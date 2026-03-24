package objects

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelSettings_CloakingMode_JSON(t *testing.T) {
	t.Run("nil omitted", func(t *testing.T) {
		settings := ChannelSettings{}
		data, err := json.Marshal(settings)
		require.NoError(t, err)
		require.NotContains(t, string(data), "cloaking_mode")
	})

	t.Run("follow_global preserved", func(t *testing.T) {
		mode := "follow_global"
		settings := ChannelSettings{CloakingMode: &mode}
		data, err := json.Marshal(settings)
		require.NoError(t, err)
		require.Contains(t, string(data), `"cloaking_mode":"follow_global"`)

		var decoded ChannelSettings
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		require.NotNil(t, decoded.CloakingMode)
		require.Equal(t, "follow_global", *decoded.CloakingMode)
	})

	t.Run("auto preserved", func(t *testing.T) {
		mode := "auto"
		settings := ChannelSettings{CloakingMode: &mode}
		data, err := json.Marshal(settings)
		require.NoError(t, err)

		var decoded ChannelSettings
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		require.NotNil(t, decoded.CloakingMode)
		require.Equal(t, "auto", *decoded.CloakingMode)
	})

	t.Run("always preserved", func(t *testing.T) {
		mode := "always"
		settings := ChannelSettings{CloakingMode: &mode}
		data, err := json.Marshal(settings)
		require.NoError(t, err)

		var decoded ChannelSettings
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		require.NotNil(t, decoded.CloakingMode)
		require.Equal(t, "always", *decoded.CloakingMode)
	})

	t.Run("never preserved", func(t *testing.T) {
		mode := "never"
		settings := ChannelSettings{CloakingMode: &mode}
		data, err := json.Marshal(settings)
		require.NoError(t, err)

		var decoded ChannelSettings
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		require.NotNil(t, decoded.CloakingMode)
		require.Equal(t, "never", *decoded.CloakingMode)
	})
}
