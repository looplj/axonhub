package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/objects"
)

func TestChannelService_BulkEnableChannels(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create test channels with disabled status
	ch1, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Channel 1").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key1"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusDisabled).
		Save(ctx)
	require.NoError(t, err)

	ch2, err := client.Channel.Create().
		SetType(channel.TypeAnthropic).
		SetName("Channel 2").
		SetBaseURL("https://api.anthropic.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "key2"}).
		SetSupportedModels([]string{"claude-3-opus-20240229"}).
		SetDefaultTestModel("claude-3-opus-20240229").
		SetStatus(channel.StatusDisabled).
		Save(ctx)
	require.NoError(t, err)

	ch3, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Channel 3").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key3"}).
		SetSupportedModels([]string{"gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-3.5-turbo").
		SetStatus(channel.StatusDisabled).
		Save(ctx)
	require.NoError(t, err)

	tests := []struct {
		name    string
		ids     []int
		wantErr bool
		errMsg  string
	}{
		{
			name:    "enable multiple channels successfully",
			ids:     []int{ch1.ID, ch2.ID},
			wantErr: false,
		},
		{
			name:    "enable single channel successfully",
			ids:     []int{ch3.ID},
			wantErr: false,
		},
		{
			name:    "enable with non-existent channel ID",
			ids:     []int{ch1.ID, 99999},
			wantErr: true,
			errMsg:  "expected to find",
		},
		{
			name:    "enable with empty list",
			ids:     []int{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.BulkEnableChannels(ctx, tt.ids)

			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)

				// Verify channels are enabled if IDs were provided
				if len(tt.ids) > 0 {
					for _, id := range tt.ids {
						ch, err := client.Channel.Get(ctx, id)
						require.NoError(t, err)
						require.Equal(t, channel.StatusEnabled, ch.Status)
					}
				}
			}
		})
	}
}

func TestChannelService_BulkDisableChannels(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create test channels
	ch1, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Channel 1").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key1"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	ch2, err := client.Channel.Create().
		SetType(channel.TypeAnthropic).
		SetName("Channel 2").
		SetBaseURL("https://api.anthropic.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "key2"}).
		SetSupportedModels([]string{"claude-3-opus-20240229"}).
		SetDefaultTestModel("claude-3-opus-20240229").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	ch3, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Channel 3").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key3"}).
		SetSupportedModels([]string{"gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-3.5-turbo").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	tests := []struct {
		name    string
		ids     []int
		wantErr bool
		errMsg  string
	}{
		{
			name:    "disable multiple channels successfully",
			ids:     []int{ch1.ID, ch2.ID},
			wantErr: false,
		},
		{
			name:    "disable single channel successfully",
			ids:     []int{ch3.ID},
			wantErr: false,
		},
		{
			name:    "disable with non-existent channel ID",
			ids:     []int{ch1.ID, 99999},
			wantErr: true,
			errMsg:  "expected to find",
		},
		{
			name:    "disable with empty list",
			ids:     []int{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.BulkDisableChannels(ctx, tt.ids)

			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)

				// Verify channels are disabled if IDs were provided
				if len(tt.ids) > 0 {
					for _, id := range tt.ids {
						ch, err := client.Channel.Get(ctx, id)
						require.NoError(t, err)
						require.Equal(t, channel.StatusDisabled, ch.Status)
					}
				}
			}
		})
	}
}

func TestChannelService_BulkArchiveChannels(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := ent.NewContext(context.Background(), client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create test channels
	ch1, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Channel 1").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key1"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	ch2, err := client.Channel.Create().
		SetType(channel.TypeAnthropic).
		SetName("Channel 2").
		SetBaseURL("https://api.anthropic.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "key2"}).
		SetSupportedModels([]string{"claude-3-opus-20240229"}).
		SetDefaultTestModel("claude-3-opus-20240229").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	ch3, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Channel 3").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key3"}).
		SetSupportedModels([]string{"gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-3.5-turbo").
		SetStatus(channel.StatusDisabled).
		Save(ctx)
	require.NoError(t, err)

	tests := []struct {
		name    string
		ids     []int
		wantErr bool
		errMsg  string
	}{
		{
			name:    "archive multiple channels successfully",
			ids:     []int{ch1.ID, ch2.ID},
			wantErr: false,
		},
		{
			name:    "archive single channel successfully",
			ids:     []int{ch3.ID},
			wantErr: false,
		},
		{
			name:    "archive with non-existent channel ID",
			ids:     []int{ch1.ID, 99999},
			wantErr: true,
			errMsg:  "expected to find",
		},
		{
			name:    "archive with empty list",
			ids:     []int{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.BulkArchiveChannels(ctx, tt.ids)

			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)

				// Verify channels are archived if IDs were provided
				if len(tt.ids) > 0 {
					for _, id := range tt.ids {
						ch, err := client.Channel.Get(ctx, id)
						require.NoError(t, err)
						require.Equal(t, channel.StatusArchived, ch.Status)
					}
				}
			}
		})
	}
}
