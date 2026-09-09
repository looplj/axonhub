package biz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache/live"
)

func TestChannelService_BulkAddChannelTags(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	createChannel := func(name string, tags []string) *ent.Channel {
		ch, err := client.Channel.Create().
			SetType(channel.TypeOpenai).
			SetName(name).
			SetBaseURL("https://api.openai.com/v1").
			SetCredentials(objects.ChannelCredentials{APIKey: name}).
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SetTags(tags).
			Save(ctx)
		require.NoError(t, err)
		return ch
	}

	ch1 := createChannel("Bulk Tags 1", []string{"existing", "公益"})
	ch2 := createChannel("Bulk Tags 2", []string{"official"})
	ch3 := createChannel("Bulk Tags 3", nil)

	err := svc.BulkAddChannelTags(ctx, []int{ch1.ID, ch2.ID, ch3.ID}, []string{" 公益 ", "低价", "低价", " "})
	require.NoError(t, err)

	updated1, err := client.Channel.Get(ctx, ch1.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"existing", "公益", "低价"}, updated1.Tags)
	updated2, err := client.Channel.Get(ctx, ch2.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"official", "公益", "低价"}, updated2.Tags)
	updated3, err := client.Channel.Get(ctx, ch3.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"公益", "低价"}, updated3.Tags)

	err = svc.BulkAddChannelTags(ctx, []int{ch1.ID, 99999}, []string{"should-not-apply"})
	require.Error(t, err)
	unchanged, err := client.Channel.Get(ctx, ch1.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"existing", "公益", "低价"}, unchanged.Tags)
}

func TestChannelService_BulkRemoveChannelTags(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	createChannel := func(name string, tags []string) *ent.Channel {
		ch, err := client.Channel.Create().
			SetType(channel.TypeOpenai).
			SetName(name).
			SetBaseURL("https://api.openai.com/v1").
			SetCredentials(objects.ChannelCredentials{APIKey: name}).
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SetTags(tags).
			Save(ctx)
		require.NoError(t, err)
		return ch
	}

	ch1 := createChannel("Bulk Remove Tags 1", []string{"existing", "公益", "低价"})
	ch2 := createChannel("Bulk Remove Tags 2", []string{"official", "公益", "低价"})
	ch3 := createChannel("Bulk Remove Tags 3", []string{"公益", "低价", "other"})

	err := svc.BulkRemoveChannelTags(ctx, []int{ch1.ID, ch2.ID, ch3.ID}, []string{" 公益 ", "低价", "低价"})
	require.NoError(t, err)

	updated1, err := client.Channel.Get(ctx, ch1.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"existing"}, updated1.Tags)
	updated2, err := client.Channel.Get(ctx, ch2.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"official"}, updated2.Tags)
	updated3, err := client.Channel.Get(ctx, ch3.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"other"}, updated3.Tags)

	err = svc.BulkRemoveChannelTags(ctx, []int{ch1.ID, 99999}, []string{"should-not-apply"})
	require.Error(t, err)
	unchanged, err := client.Channel.Get(ctx, ch1.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"existing"}, unchanged.Tags)
}

func TestChannelService_BulkManageChannelTags(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	createChannel := func(name string, tags []string) *ent.Channel {
		ch, err := client.Channel.Create().
			SetType(channel.TypeOpenai).
			SetName(name).
			SetBaseURL("https://api.openai.com/v1").
			SetCredentials(objects.ChannelCredentials{APIKey: name}).
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SetTags(tags).
			Save(ctx)
		require.NoError(t, err)
		return ch
	}

	ch1 := createChannel("Bulk Manage Tags 1", []string{"existing", "公益"})
	ch2 := createChannel("Bulk Manage Tags 2", []string{"official", "公益"})

	err := svc.BulkManageChannelTags(ctx, []int{ch1.ID, ch2.ID}, []string{" 低价 ", "低价"}, []string{"公益"})
	require.NoError(t, err)

	updated1, err := client.Channel.Get(ctx, ch1.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"existing", "低价"}, updated1.Tags)
	updated2, err := client.Channel.Get(ctx, ch2.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"official", "低价"}, updated2.Tags)

	err = svc.BulkManageChannelTags(ctx, []int{ch1.ID, 99999}, []string{"should-not-apply"}, []string{"existing"})
	require.Error(t, err)
	unchanged, err := client.Channel.Get(ctx, ch1.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"existing", "低价"}, unchanged.Tags)
}

func TestChannelService_BulkEnableChannels(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

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

func TestChannelService_BulkRecoverChannels(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	errorMessage := "Unauthorized"

	ch1, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Recover Channel 1").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key1"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusDisabled).
		SetErrorMessage(errorMessage).
		Save(ctx)
	require.NoError(t, err)

	ch2, err := client.Channel.Create().
		SetType(channel.TypeAnthropic).
		SetName("Recover Channel 2").
		SetBaseURL("https://api.anthropic.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "key2"}).
		SetSupportedModels([]string{"claude-3-opus-20240229"}).
		SetDefaultTestModel("claude-3-opus-20240229").
		SetStatus(channel.StatusDisabled).
		SetErrorMessage(errorMessage).
		Save(ctx)
	require.NoError(t, err)

	err = svc.BulkRecoverChannels(ctx, []int{ch1.ID, ch2.ID})
	require.NoError(t, err)

	recovered1, err := client.Channel.Get(ctx, ch1.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusEnabled, recovered1.Status)
	require.Nil(t, recovered1.ErrorMessage)

	recovered2, err := client.Channel.Get(ctx, ch2.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusEnabled, recovered2.Status)
	require.Nil(t, recovered2.ErrorMessage)
}

func TestChannelService_BulkDisableChannels(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

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
	ctx = authz.WithTestBypass(ctx)

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

func createBulkOrderingTestChannel(t *testing.T, ctx context.Context, client *ent.Client, name string, weight int) *ent.Channel {
	t.Helper()

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName(name).
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key-" + name}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		SetOrderingWeight(weight).
		Save(ctx)
	require.NoError(t, err)

	return ch
}

// The GraphQL Transactioner wraps bulkUpdateChannelOrdering in a caller-owned
// transaction. The cache refresh must be published only after that transaction
// commits, otherwise the asynchronous reload can read a stale ordering_weight
// snapshot and keep failover chains on the old ordering (issue #2256).
func TestChannelService_BulkUpdateChannelOrdering_DefersReloadUntilCommit(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))

	ch1 := createBulkOrderingTestChannel(t, ctx, client, "Ordering Commit 1", 10)
	ch2 := createBulkOrderingTestChannel(t, ctx, client, "Ordering Commit 2", 20)

	notifier := &channelSyncNotifierSpy{}
	svc.channelNotifier = notifier
	previousAsyncReloadDisabled := asyncReloadDisabled
	asyncReloadDisabled = false
	t.Cleanup(func() {
		asyncReloadDisabled = previousAsyncReloadDisabled
	})

	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	txCtx := ent.NewTxContext(ctx, tx)
	txCtx = ent.NewContext(txCtx, tx.Client())

	updated, err := svc.BulkUpdateChannelOrdering(txCtx, []*ChannelOrderingItem{
		{ID: ch1.ID, OrderingWeight: 100},
		{ID: ch2.ID, OrderingWeight: 50},
	})
	require.NoError(t, err)
	require.Len(t, updated, 2)

	// Before commit no refresh may be published: an asynchronous reload at this
	// point could read the uncommitted transaction or the stale ordering
	// snapshot and keep failover chains on the old ordering.
	require.Zero(t, notifier.notifyCount, "cache refresh must not be published before the transaction commits")

	require.NoError(t, tx.Commit())

	// After commit exactly one force refresh is delivered.
	require.Equal(t, 1, notifier.notifyCount)

	committed1, err := client.Channel.Get(ctx, ch1.ID)
	require.NoError(t, err)
	require.Equal(t, 100, committed1.OrderingWeight)
	committed2, err := client.Channel.Get(ctx, ch2.ID)
	require.NoError(t, err)
	require.Equal(t, 50, committed2.OrderingWeight)
}

func TestChannelService_BulkUpdateChannelOrdering_NoReloadOnRollback(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))

	ch1 := createBulkOrderingTestChannel(t, ctx, client, "Ordering Rollback 1", 10)

	notifier := &channelSyncNotifierSpy{}
	svc.channelNotifier = notifier
	previousAsyncReloadDisabled := asyncReloadDisabled
	asyncReloadDisabled = false
	t.Cleanup(func() {
		asyncReloadDisabled = previousAsyncReloadDisabled
	})

	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	txCtx := ent.NewTxContext(ctx, tx)
	txCtx = ent.NewContext(txCtx, tx.Client())

	_, err = svc.BulkUpdateChannelOrdering(txCtx, []*ChannelOrderingItem{
		{ID: ch1.ID, OrderingWeight: 100},
	})
	require.NoError(t, err)
	require.Zero(t, notifier.notifyCount)

	require.NoError(t, tx.Rollback())

	// A rolled-back ordering change must not trigger any cache refresh, and
	// the database keeps the old weight.
	require.Zero(t, notifier.notifyCount)
	unchanged, err := client.Channel.Get(ctx, ch1.ID)
	require.NoError(t, err)
	require.Equal(t, 10, unchanged.OrderingWeight)
}

func TestChannelService_BulkStatusAndDelete_DefersReloadUntilCommit(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))

	ch1 := createBulkOrderingTestChannel(t, ctx, client, "Status Commit 1", 10)
	ch2 := createBulkOrderingTestChannel(t, ctx, client, "Status Commit 2", 20)

	notifier := &channelSyncNotifierSpy{}
	svc.channelNotifier = notifier
	previousAsyncReloadDisabled := asyncReloadDisabled
	asyncReloadDisabled = false
	t.Cleanup(func() {
		asyncReloadDisabled = previousAsyncReloadDisabled
	})

	// Bulk status change inside a caller-owned transaction.
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	txCtx := ent.NewTxContext(ctx, tx)
	txCtx = ent.NewContext(txCtx, tx.Client())

	require.NoError(t, svc.BulkDisableChannels(txCtx, []int{ch1.ID}))
	require.Zero(t, notifier.notifyCount, "bulk status change must not refresh before commit")

	require.NoError(t, tx.Commit())
	require.Equal(t, 1, notifier.notifyCount)

	// Bulk delete inside a caller-owned transaction.
	tx2, err := client.Tx(ctx)
	require.NoError(t, err)
	txCtx2 := ent.NewTxContext(ctx, tx2)
	txCtx2 = ent.NewContext(txCtx2, tx2.Client())

	require.NoError(t, svc.BulkDeleteChannels(txCtx2, []int{ch2.ID}))
	require.Equal(t, 1, notifier.notifyCount, "bulk delete must not refresh before commit")

	require.NoError(t, tx2.Commit())
	require.Equal(t, 2, notifier.notifyCount)
}

// After the ordering transaction commits, a cache reload must produce a
// snapshot ordered by the newly committed ordering_weight values.
func TestChannelService_BulkUpdateChannelOrdering_CacheSnapshotUsesCommittedWeights(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))

	ch1 := createBulkOrderingTestChannel(t, ctx, client, "Ordering Snapshot 1", 10)
	ch2 := createBulkOrderingTestChannel(t, ctx, client, "Ordering Snapshot 2", 20)

	// Install a real cache backed by reloadEnabledChannels so the snapshot is
	// rebuilt from the database on Load.
	svc.enabledChannelsCache.Stop()
	svc.enabledChannelsCache = live.NewCache(live.Options[[]*Channel]{
		Name:            "test_enabled_channels_ordering",
		InitialValue:    []*Channel{},
		RefreshInterval: time.Hour,
		RefreshFunc:     svc.reloadEnabledChannels,
		OnSwap:          svc.onEnabledChannelsSwap,
	})

	require.NoError(t, svc.enabledChannelsCache.Load(ctx, true))
	before := svc.GetEnabledChannels()
	require.Len(t, before, 2)
	require.Equal(t, ch2.ID, before[0].ID) // weight 20 ranks first
	require.Equal(t, ch1.ID, before[1].ID) // weight 10 ranks second

	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	txCtx := ent.NewTxContext(ctx, tx)
	txCtx = ent.NewContext(txCtx, tx.Client())

	_, err = svc.BulkUpdateChannelOrdering(txCtx, []*ChannelOrderingItem{
		{ID: ch1.ID, OrderingWeight: 100},
		{ID: ch2.ID, OrderingWeight: 50},
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	require.NoError(t, svc.enabledChannelsCache.Load(ctx, true))
	after := svc.GetEnabledChannels()
	require.Len(t, after, 2)
	require.Equal(t, ch1.ID, after[0].ID) // weight 100 now ranks first
	require.Equal(t, ch2.ID, after[1].ID) // weight 50 now ranks second
}
