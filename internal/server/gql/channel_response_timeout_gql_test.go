package gql

import (
	"bytes"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
)

func TestUnmarshalInputChannelSettingsInput_ResponseTimeout(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]any
		want    objects.ChannelResponseTimeoutSettings
		wantNil bool
	}{
		{
			name:    "missing field leaves the setting unset",
			input:   map[string]any{},
			wantNil: true,
		},
		{
			name: "custom mode with both overrides",
			input: map[string]any{
				"responseTimeout": map[string]any{
					"mode":                            "CUSTOM",
					"streamFirstEventTimeoutSeconds":  15,
					"nonStreamResponseTimeoutSeconds": 25,
				},
			},
			want: objects.ChannelResponseTimeoutSettings{
				Mode:                            objects.ChannelResponseTimeoutModeCustom,
				StreamFirstEventTimeoutSeconds:  lo.ToPtr(15),
				NonStreamResponseTimeoutSeconds: lo.ToPtr(25),
			},
		},
		{
			name: "inherit mode",
			input: map[string]any{
				"responseTimeout": map[string]any{
					"mode": "INHERIT",
				},
			},
			want: objects.ChannelResponseTimeoutSettings{
				Mode: objects.ChannelResponseTimeoutModeInherit,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := (&executionContext{}).unmarshalInputChannelSettingsInput(t.Context(), tt.input)
			require.NoError(t, err)

			if tt.wantNil {
				require.Nil(t, input.ResponseTimeout)
				return
			}

			require.NotNil(t, input.ResponseTimeout)
			require.Equal(t, tt.want, *input.ResponseTimeout)
		})
	}
}

func TestUnmarshalInputChannelSettingsInput_ResponseTimeoutRejectsUnknownMode(t *testing.T) {
	_, err := (&executionContext{}).unmarshalInputChannelSettingsInput(t.Context(), map[string]any{
		"responseTimeout": map[string]any{"mode": "SOMETHING_ELSE"},
	})
	require.Error(t, err)
}

func TestChannelResponseTimeoutModeGraphQLMarshal(t *testing.T) {
	var encoded bytes.Buffer
	objects.ChannelResponseTimeoutModeInherit.MarshalGQL(&encoded)
	require.Equal(t, `"INHERIT"`, encoded.String())

	encoded.Reset()
	objects.ChannelResponseTimeoutModeCustom.MarshalGQL(&encoded)
	require.Equal(t, `"CUSTOM"`, encoded.String())

	encoded.Reset()
	objects.ChannelResponseTimeoutMode("bogus").MarshalGQL(&encoded)
	require.Equal(t, `"INHERIT"`, encoded.String())

	var mode objects.ChannelResponseTimeoutMode
	require.NoError(t, mode.UnmarshalGQL("CUSTOM"))
	require.Equal(t, objects.ChannelResponseTimeoutModeCustom, mode)
	require.Error(t, mode.UnmarshalGQL("BOGUS"))
}
