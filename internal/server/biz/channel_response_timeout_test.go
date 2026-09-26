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
			name: "inherit mode ignores stored values",
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
