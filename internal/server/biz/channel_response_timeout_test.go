package biz

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
)

func TestValidateResponseTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   *objects.ChannelResponseTimeoutSettings
		wantErr bool
	}{
		{
			name:  "nil is allowed",
			input: nil,
		},
		{
			name:  "zero value is allowed",
			input: &objects.ChannelResponseTimeoutSettings{},
		},
		{
			name: "inherit mode does not validate stored values",
			input: &objects.ChannelResponseTimeoutSettings{
				Mode:                           objects.ChannelResponseTimeoutModeInherit,
				StreamFirstEventTimeoutSeconds: lo.ToPtr(-1),
			},
		},
		{
			name: "custom with valid values is allowed",
			input: &objects.ChannelResponseTimeoutSettings{
				Mode:                            objects.ChannelResponseTimeoutModeCustom,
				StreamFirstEventTimeoutSeconds:  lo.ToPtr(0),
				NonStreamResponseTimeoutSeconds: lo.ToPtr(600),
			},
		},
		{
			name: "custom with nil fields is allowed",
			input: &objects.ChannelResponseTimeoutSettings{
				Mode: objects.ChannelResponseTimeoutModeCustom,
			},
		},
		{
			name: "custom rejects a negative stream timeout",
			input: &objects.ChannelResponseTimeoutSettings{
				Mode:                           objects.ChannelResponseTimeoutModeCustom,
				StreamFirstEventTimeoutSeconds: lo.ToPtr(-1),
			},
			wantErr: true,
		},
		{
			name: "custom rejects an oversized non-stream timeout",
			input: &objects.ChannelResponseTimeoutSettings{
				Mode:                            objects.ChannelResponseTimeoutModeCustom,
				NonStreamResponseTimeoutSeconds: lo.ToPtr(601),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateResponseTimeout(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestNormalizeResponseTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		settings *objects.ChannelSettings
		want     *objects.ChannelResponseTimeoutSettings
		wantErr  bool
	}{
		{
			name:     "nil settings is left alone",
			settings: nil,
		},
		{
			name:     "absent override stays absent",
			settings: &objects.ChannelSettings{},
		},
		{
			// Inherit mode ignores the overrides, so they must not be stored:
			// an out-of-range number would be rejected by the readers that
			// schema-check every channel, breaking the channel list.
			name: "inherit drops the stored override",
			settings: &objects.ChannelSettings{
				ResponseTimeout: &objects.ChannelResponseTimeoutSettings{
					Mode:                            objects.ChannelResponseTimeoutModeInherit,
					StreamFirstEventTimeoutSeconds:  lo.ToPtr(601),
					NonStreamResponseTimeoutSeconds: lo.ToPtr(-1),
				},
			},
		},
		{
			name: "custom override is kept",
			settings: &objects.ChannelSettings{
				ResponseTimeout: &objects.ChannelResponseTimeoutSettings{
					Mode:                           objects.ChannelResponseTimeoutModeCustom,
					StreamFirstEventTimeoutSeconds: lo.ToPtr(12),
				},
			},
			want: &objects.ChannelResponseTimeoutSettings{
				Mode:                           objects.ChannelResponseTimeoutModeCustom,
				StreamFirstEventTimeoutSeconds: lo.ToPtr(12),
			},
		},
		{
			name: "custom out of range is rejected",
			settings: &objects.ChannelSettings{
				ResponseTimeout: &objects.ChannelResponseTimeoutSettings{
					Mode:                           objects.ChannelResponseTimeoutModeCustom,
					StreamFirstEventTimeoutSeconds: lo.ToPtr(601),
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := NormalizeResponseTimeout(tt.settings)
			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)

			if tt.settings == nil {
				return
			}

			require.Equal(t, tt.want, tt.settings.ResponseTimeout)
		})
	}
}

func TestCreateChannel_PersistsResponseTimeoutOverride(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:channel_response_timeout?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := NewChannelServiceForTest(client)

	settings := &objects.ChannelSettings{
		ResponseTimeout: &objects.ChannelResponseTimeoutSettings{
			Mode:                            objects.ChannelResponseTimeoutModeCustom,
			StreamFirstEventTimeoutSeconds:  lo.ToPtr(12),
			NonStreamResponseTimeoutSeconds: lo.ToPtr(0),
		},
	}

	created, err := svc.createChannel(ctx, ent.CreateChannelInput{
		Type:             channel.TypeOpenai,
		Name:             "openai-response-timeout",
		Credentials:      objects.ChannelCredentials{APIKey: "test-key"},
		SupportedModels:  []string{"gpt-4"},
		DefaultTestModel: "gpt-4",
		Settings:         settings,
	})
	require.NoError(t, err)
	require.Equal(t, settings.ResponseTimeout, created.Settings.ResponseTimeout)

	// The override must survive Ent's JSON round-trip, including the explicit 0
	// that disables the non-stream timeout.
	stored, err := client.Channel.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, objects.ChannelResponseTimeoutModeCustom, stored.Settings.ResponseTimeout.Mode)
	require.Equal(t, 12, *stored.Settings.ResponseTimeout.StreamFirstEventTimeoutSeconds)
	require.Equal(t, 0, *stored.Settings.ResponseTimeout.NonStreamResponseTimeoutSeconds)
}

func TestCreateChannel_RejectsInvalidResponseTimeout(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:channel_response_timeout_invalid?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := NewChannelServiceForTest(client)

	_, err := svc.createChannel(ctx, ent.CreateChannelInput{
		Type:             channel.TypeOpenai,
		Name:             "openai-invalid-response-timeout",
		Credentials:      objects.ChannelCredentials{APIKey: "test-key"},
		SupportedModels:  []string{"gpt-4"},
		DefaultTestModel: "gpt-4",
		Settings: &objects.ChannelSettings{
			ResponseTimeout: &objects.ChannelResponseTimeoutSettings{
				Mode:                           objects.ChannelResponseTimeoutModeCustom,
				StreamFirstEventTimeoutSeconds: lo.ToPtr(601),
			},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid response timeout")
}

func TestUpdateChannel_RejectsInvalidResponseTimeout(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:channel_response_timeout_update?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := NewChannelServiceForTest(client)

	created, err := svc.createChannel(ctx, ent.CreateChannelInput{
		Type:             channel.TypeOpenai,
		Name:             "openai-response-timeout-update",
		Credentials:      objects.ChannelCredentials{APIKey: "test-key"},
		SupportedModels:  []string{"gpt-4"},
		DefaultTestModel: "gpt-4",
	})
	require.NoError(t, err)

	_, err = svc.UpdateChannel(ctx, created.ID, &ent.UpdateChannelInput{
		Settings: &objects.ChannelSettings{
			ResponseTimeout: &objects.ChannelResponseTimeoutSettings{
				Mode:                            objects.ChannelResponseTimeoutModeCustom,
				NonStreamResponseTimeoutSeconds: lo.ToPtr(-1),
			},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid response timeout")
}

// A channel that inherits the global timeouts must not persist override numbers.
// An out-of-range value stored in inherit mode is invisible to the service but
// fatal to readers that schema-check the 0..600 bounds, such as the channel list.
func TestCreateChannel_DropsInheritedResponseTimeoutValues(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:channel_response_timeout_inherit?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := NewChannelServiceForTest(client)

	created, err := svc.createChannel(ctx, ent.CreateChannelInput{
		Type:             channel.TypeOpenai,
		Name:             "openai-response-timeout-inherit",
		Credentials:      objects.ChannelCredentials{APIKey: "test-key"},
		SupportedModels:  []string{"gpt-4"},
		DefaultTestModel: "gpt-4",
		Settings: &objects.ChannelSettings{
			ResponseTimeout: &objects.ChannelResponseTimeoutSettings{
				Mode:                            objects.ChannelResponseTimeoutModeInherit,
				StreamFirstEventTimeoutSeconds:  lo.ToPtr(601),
				NonStreamResponseTimeoutSeconds: lo.ToPtr(42),
			},
		},
	})
	require.NoError(t, err)
	require.Nil(t, created.Settings.ResponseTimeout)

	stored, err := client.Channel.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Nil(t, stored.Settings.ResponseTimeout)
}

func TestUpdateChannel_DropsInheritedResponseTimeoutValues(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:channel_response_timeout_inherit_update?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := NewChannelServiceForTest(client)

	created, err := svc.createChannel(ctx, ent.CreateChannelInput{
		Type:             channel.TypeOpenai,
		Name:             "openai-response-timeout-inherit-update",
		Credentials:      objects.ChannelCredentials{APIKey: "test-key"},
		SupportedModels:  []string{"gpt-4"},
		DefaultTestModel: "gpt-4",
		Settings: &objects.ChannelSettings{
			ResponseTimeout: &objects.ChannelResponseTimeoutSettings{
				Mode:                           objects.ChannelResponseTimeoutModeCustom,
				StreamFirstEventTimeoutSeconds: lo.ToPtr(12),
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, created.Settings.ResponseTimeout)

	// Switching the channel back to inherit has to clear the stored override.
	updated, err := svc.UpdateChannel(ctx, created.ID, &ent.UpdateChannelInput{
		Settings: &objects.ChannelSettings{
			ResponseTimeout: &objects.ChannelResponseTimeoutSettings{
				Mode:                           objects.ChannelResponseTimeoutModeInherit,
				StreamFirstEventTimeoutSeconds: lo.ToPtr(601),
			},
		},
	})
	require.NoError(t, err)
	require.Nil(t, updated.Settings.ResponseTimeout)

	stored, err := client.Channel.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Nil(t, stored.Settings.ResponseTimeout)
}
