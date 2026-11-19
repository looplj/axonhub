// 职责: 覆盖调度化停用的校验、cron 时刻解析与每分钟清扫任务的端到端行为。
// 关联文件:
//   internal/server/biz/channel_scheduled_disable.go
//   internal/server/biz/channel_auto_disable_test.go
// 注意事项:
//   清扫测试使用真实 cron 表达式计算期望值，避免硬编码与运行时刻相关的结果。
package biz

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/zhenzou/executors"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/scheduler"
)

func TestNormalizeScheduledDisable(t *testing.T) {
	t.Run("nil policies", func(t *testing.T) {
		require.NoError(t, NormalizeScheduledDisable(nil))
	})

	t.Run("all-empty struct dropped to nil", func(t *testing.T) {
		policies := &objects.ChannelPolicies{ScheduledDisable: &objects.ChannelScheduledDisable{}}
		require.NoError(t, NormalizeScheduledDisable(policies))
		require.Nil(t, policies.ScheduledDisable)
	})

	t.Run("valid disable and reenable", func(t *testing.T) {
		policies := &objects.ChannelPolicies{ScheduledDisable: &objects.ChannelScheduledDisable{
			DisableCron:      " 0 22 * * * ",
			DisableTimezone:  " Asia/Shanghai ",
			ReenableCron:     "0 2 * * *",
			ReenableTimezone: "UTC",
		}}
		require.NoError(t, NormalizeScheduledDisable(policies))
		sd := policies.ScheduledDisable
		require.NotNil(t, sd)
		require.Equal(t, "0 22 * * *", sd.DisableCron)
		require.Equal(t, "Asia/Shanghai", sd.DisableTimezone)
		require.Equal(t, "0 2 * * *", sd.ReenableCron)
		require.Equal(t, "UTC", sd.ReenableTimezone)
	})

	t.Run("blank fields only are dropped to nil", func(t *testing.T) {
		policies := &objects.ChannelPolicies{ScheduledDisable: &objects.ChannelScheduledDisable{DisableCron: "   "}}
		require.NoError(t, NormalizeScheduledDisable(policies))
		require.Nil(t, policies.ScheduledDisable)
	})

	t.Run("missing disable cron with other fields set", func(t *testing.T) {
		policies := &objects.ChannelPolicies{ScheduledDisable: &objects.ChannelScheduledDisable{DisableTimezone: "UTC"}}
		err := NormalizeScheduledDisable(policies)
		require.Error(t, err)
		require.Contains(t, err.Error(), "requires a cron expression")
	})

	t.Run("invalid disable cron", func(t *testing.T) {
		policies := &objects.ChannelPolicies{ScheduledDisable: &objects.ChannelScheduledDisable{DisableCron: "not a cron"}}
		err := NormalizeScheduledDisable(policies)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid cron")
	})

	t.Run("invalid reenable cron", func(t *testing.T) {
		policies := &objects.ChannelPolicies{ScheduledDisable: &objects.ChannelScheduledDisable{
			DisableCron:  "0 22 * * *",
			ReenableCron: "nope",
		}}
		err := NormalizeScheduledDisable(policies)
		require.Error(t, err)
		require.Contains(t, err.Error(), "re-enable has invalid cron")
	})

	t.Run("invalid disable timezone", func(t *testing.T) {
		policies := &objects.ChannelPolicies{ScheduledDisable: &objects.ChannelScheduledDisable{
			DisableCron:     "0 22 * * *",
			DisableTimezone: "No/Zone",
		}}
		err := NormalizeScheduledDisable(policies)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid timezone")
	})

	t.Run("invalid reenable timezone", func(t *testing.T) {
		policies := &objects.ChannelPolicies{ScheduledDisable: &objects.ChannelScheduledDisable{
			DisableCron:      "0 22 * * *",
			ReenableCron:     "0 2 * * *",
			ReenableTimezone: "No/Zone",
		}}
		err := NormalizeScheduledDisable(policies)
		require.Error(t, err)
		require.Contains(t, err.Error(), "re-enable has invalid timezone")
	})
}

func TestPrevScheduledCronOccurrence(t *testing.T) {
	// 2026-11-14T14:37Z, not on a minute boundary.
	now := time.Date(2026, time.November, 14, 14, 37, 0, 0, time.UTC)

	tests := []struct {
		name  string
		cron  string
		tz    string
		want  time.Time
		isErr bool
	}{
		{name: "hourly", cron: "0 * * * *", want: now.Truncate(time.Hour)},
		{name: "exact previous", cron: "30 14 * * *", want: now.Add(-7 * time.Minute)},
		// 2025-11-15 00:00 UTC is more than 5000 minutes before now: the
		// bounded backward search must give up with an error.
		{name: "occurrence beyond the 5000-minute bound", cron: "0 0 15 11 *", isErr: true},
		{name: "every minute", cron: "* * * * *", want: now},
		{name: "invalid cron", cron: "bad", isErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := prevScheduledCronOccurrence(test.cron, test.tz, now)
			if test.isErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.True(t, got.Equal(test.want), "got %v want %v", got, test.want)
		})
	}
}

func TestNextScheduledCronOccurrence(t *testing.T) {
	// 2026-11-14T14:37:00Z, not on a minute boundary.
	now := time.Date(2026, time.November, 14, 14, 37, 0, 0, time.UTC)

	tests := []struct {
		name  string
		cron  string
		tz    string
		want  time.Time
		isErr bool
	}{
		{name: "hourly", cron: "0 * * * *", want: time.Date(2026, time.November, 14, 15, 0, 0, 0, time.UTC)},
		{name: "midnight UTC", cron: "0 0 * * *", want: time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC)},
		{
			name: "midnight in Tokyo is earlier in UTC",
			cron: "0 0 * * *",
			tz:   "Asia/Tokyo",
			// 2026-11-14T14:37Z is 2026-11-14T23:37 JST, so next 00:00 JST is 15:00 UTC the same day.
			want: time.Date(2026, time.November, 14, 15, 0, 0, 0, time.UTC),
		},
		{name: "invalid cron", cron: "bad", isErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := nextScheduledCronOccurrence(test.cron, test.tz, now)
			if test.isErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.True(t, got.Equal(test.want), "got %v want %v", got, test.want)
		})
	}
}

// withScheduledDisablePolicy returns a client, ctx, and service with one enabled
// channel carrying the given schedule.
func withScheduledDisablePolicy(t *testing.T, name string, sd *objects.ChannelScheduledDisable) (*ent.Client, context.Context, *ChannelService, *ent.Channel) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	ch := createTestChannelWithAPIKeys(t, client, ctx, name, []string{"test-key"})

	_, err := client.Channel.UpdateOneID(ch.ID).
		SetPolicies(objects.ChannelPolicies{ScheduledDisable: sd}).
		Save(ctx)
	require.NoError(t, err)

	return client, ctx, svc, ch
}

func TestRunScheduledDisableSweep(t *testing.T) {
	t.Run("non-matching cron leaves channel enabled", func(t *testing.T) {
		client, ctx, svc, ch := withScheduledDisablePolicy(t, "no-match", &objects.ChannelScheduledDisable{
			DisableCron:  "0 0 1 1 *",
			ReenableCron: "0 0 2 1 *",
		})

		svc.runScheduledDisableSweep(context.Background())

		updated, err := client.Channel.Get(ctx, ch.ID)
		require.NoError(t, err)
		require.Equal(t, channel.StatusEnabled, updated.Status)
		require.Nil(t, updated.AutoDisabledAt)
		require.Nil(t, updated.AutoDisableExpiresAt)
	})

	t.Run("matching cron disables until reenable cron", func(t *testing.T) {
		// The sweep evaluates the cron in UTC (empty timezone), so the matching
		// minute must be computed in UTC, not the host's local zone.
		now := time.Now().UTC()
		minute := now.Truncate(time.Minute)

		client, ctx, svc, ch := withScheduledDisablePolicy(t, "match", &objects.ChannelScheduledDisable{
			DisableCron:  fmt.Sprintf("%d %d * * *", minute.Minute(), minute.Hour()),
			ReenableCron: "0 0 2 1 *",
		})

		svc.runScheduledDisableSweep(context.Background())

		updated, err := client.Channel.Get(ctx, ch.ID)
		require.NoError(t, err)
		require.Equal(t, channel.StatusDisabled, updated.Status)
		require.NotNil(t, updated.AutoDisabledAt)
		require.NotNil(t, updated.AutoDisableExpiresAt)
		require.Equal(t, scheduledDisableReason, lo.FromPtr(updated.ErrorMessage))

		// Expected expiry: next Jan 2 00:00 UTC after now, computed with the same library.
		wantExpiry, err := nextScheduledCronOccurrence("0 0 2 1 *", "", now)
		require.NoError(t, err)
		require.True(t, updated.AutoDisableExpiresAt.Equal(wantExpiry),
			"got %v want %v", updated.AutoDisableExpiresAt, wantExpiry)

		// Idempotent: the channel is no longer enabled, so a second sweep is a no-op.
		before, err := client.Channel.Get(ctx, ch.ID)
		require.NoError(t, err)
		svc.runScheduledDisableSweep(context.Background())
		after, err := client.Channel.Get(ctx, ch.ID)
		require.NoError(t, err)
		require.Equal(t, before.UpdatedAt, after.UpdatedAt)
	})
}

func TestRunScheduledDisableSweepReenable(t *testing.T) {
	now := time.Now()
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	ch := createTestChannelWithAPIKeys(t, client, ctx, "reenable", []string{"test-key"})

	disabledAt := now.Add(-time.Hour)
	expiresAt := now.Add(-time.Minute)
	_, err := client.Channel.UpdateOneID(ch.ID).
		SetPolicies(objects.ChannelPolicies{ScheduledDisable: &objects.ChannelScheduledDisable{
			DisableCron:  "0 0 1 1 *",
			ReenableCron: "0 0 1 1 *",
		}}).
		SetStatus(channel.StatusDisabled).
		SetErrorMessage(scheduledDisableReason).
		SetAutoDisabledAt(disabledAt).
		SetAutoDisableExpiresAt(expiresAt).
		Save(ctx)
	require.NoError(t, err)

	svc.runScheduledDisableSweep(context.Background())

	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusEnabled, updated.Status)
	require.Nil(t, updated.AutoDisabledAt)
	require.Nil(t, updated.AutoDisableExpiresAt)
	require.Empty(t, updated.ErrorMessage)
}

func TestRunScheduledDisableSweepOperatorDisableImmune(t *testing.T) {
	now := time.Now().Truncate(time.Minute)
	client, ctx, svc, ch := withScheduledDisablePolicy(t, "operator-disabled", &objects.ChannelScheduledDisable{
		DisableCron:  fmt.Sprintf("%d %d * * *", now.Minute(), now.Hour()),
		ReenableCron: "0 0 1 1 *",
	})

	// Operator disable: status flipped without the auto-disable markers.
	_, err := client.Channel.UpdateOneID(ch.ID).
		SetStatus(channel.StatusDisabled).
		Save(ctx)
	require.NoError(t, err)

	svc.runScheduledDisableSweep(context.Background())

	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusDisabled, updated.Status)
	require.Nil(t, updated.AutoDisabledAt)
	require.Nil(t, updated.AutoDisableExpiresAt)
}

func TestRunScheduledDisableSweepNoReenableStaysDisabled(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	client, ctx, svc, ch := withScheduledDisablePolicy(t, "no-reenable", &objects.ChannelScheduledDisable{
		DisableCron: fmt.Sprintf("%d %d * * *", now.Minute(), now.Hour()),
	})

	svc.runScheduledDisableSweep(context.Background())

	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusDisabled, updated.Status)
	require.NotNil(t, updated.AutoDisabledAt)
	require.Nil(t, updated.AutoDisableExpiresAt)
	require.Equal(t, scheduledDisableReason, lo.FromPtr(updated.ErrorMessage))

	// Second sweep: still disabled, no expiry to trigger a recovery.
	before, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	svc.runScheduledDisableSweep(context.Background())
	after, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusDisabled, after.Status)
	require.Equal(t, before.UpdatedAt, after.UpdatedAt)
}

func TestRegisterScheduledTasksInclScheduledDisable(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	svc := newTestChannelService(client)

	exec := executors.NewPoolScheduleExecutor()
	sched := scheduler.New(exec)
	defer sched.Shutdown(context.Background())
	defer exec.Shutdown(context.Background())

	require.NoError(t, svc.RegisterScheduledTasks(context.Background(), sched))

	var found bool
	for _, task := range sched.List() {
		if task.Spec.Name == "channel-scheduled-disable" {
			found = true
			require.Equal(t, "* * * * *", task.Spec.CronExpr)
		}
	}
	require.True(t, found, "channel-scheduled-disable task not registered")

	// The two pre-existing tasks must remain registered.
	names := map[string]bool{}
	for _, task := range sched.List() {
		names[task.Spec.Name] = true
	}
	require.True(t, names["channel-model-sync"])
	require.True(t, names["channel-disabled-api-key-cleanup"])
}
