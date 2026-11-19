// 职责: 覆盖调度化停用的校验、cron 时刻解析与每分钟清扫任务的端到端行为。
// 关联文件:
//
//	internal/server/biz/channel_scheduled_disable.go
//	internal/server/biz/channel_auto_disable_test.go
//
// 注意事项:
//
//	清扫测试使用真实 cron 表达式计算期望值，避免硬编码与运行时刻相关的结果。

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
		// A re-enable cron without an explicit mode is normalized to "cron".
		require.Equal(t, "cron", sd.ReenableMode)
		require.Equal(t, "0 2 * * *", sd.ReenableCron)
		require.Equal(t, "UTC", sd.ReenableTimezone)
	})

	t.Run("opposite mode clears reenable cron fields", func(t *testing.T) {
		policies := &objects.ChannelPolicies{ScheduledDisable: &objects.ChannelScheduledDisable{
			DisableCron:      "0 22 * * *",
			ReenableMode:     "opposite",
			ReenableCron:     "0 2 * * *",
			ReenableTimezone: "UTC",
		}}
		require.NoError(t, NormalizeScheduledDisable(policies))
		sd := policies.ScheduledDisable
		require.NotNil(t, sd)
		require.Equal(t, "opposite", sd.ReenableMode)
		require.Empty(t, sd.ReenableCron)
		require.Empty(t, sd.ReenableTimezone)
	})

	t.Run("cron mode without a cron is rejected", func(t *testing.T) {
		policies := &objects.ChannelPolicies{ScheduledDisable: &objects.ChannelScheduledDisable{
			DisableCron:  "0 22 * * *",
			ReenableMode: "cron",
		}}
		err := NormalizeScheduledDisable(policies)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cron mode requires a cron expression")
	})

	t.Run("unsupported reenable mode is rejected", func(t *testing.T) {
		policies := &objects.ChannelPolicies{ScheduledDisable: &objects.ChannelScheduledDisable{
			DisableCron:  "0 22 * * *",
			ReenableMode: "sometimes",
		}}
		err := NormalizeScheduledDisable(policies)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not supported")
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

func TestCronMatchesMinute(t *testing.T) {
	base := time.Date(2026, time.November, 14, 0, 0, 0, 0, time.UTC)
	at := func(minute int) time.Time { return base.Add(time.Duration(minute) * time.Minute) }

	tests := []struct {
		name   string
		cron   string
		tz     string
		minute time.Time
		want   bool
	}{
		{name: "matches", cron: "0 14 * * *", minute: at(14 * 60), want: true},
		{name: "one minute before", cron: "0 14 * * *", minute: at(14*60 - 1), want: false},
		{name: "every minute", cron: "* * * * *", minute: at(1), want: true},
		{name: "weekday restriction excludes Sunday", cron: "0 0 * * 1-5", minute: time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC), want: false},
		{
			name:   "tokyo midnight matches 15:00 UTC previous day",
			cron:   "0 0 * * *",
			tz:     "Asia/Tokyo",
			minute: time.Date(2026, time.November, 13, 15, 0, 0, 0, time.UTC),
			want:   true,
		},
		{
			// 2027-03-14 is the US spring-forward day; 12:00 noon lands on
			// EDT (16:00 UTC), so matching must resolve in the local zone.
			name:   "DST spring forward in New York",
			cron:   "0 12 * * *",
			tz:     "America/New_York",
			minute: time.Date(2027, time.March, 14, 16, 0, 0, 0, time.UTC),
			want:   true,
		},
		{name: "invalid cron", cron: "bad", minute: at(0), want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := cronMatchesMinute(test.cron, test.tz, test.minute)
			if test.cron == "bad" {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestOppositeReenableExpiry(t *testing.T) {
	base := time.Date(2026, time.November, 16, 0, 0, 0, 0, time.UTC) // Monday

	tests := []struct {
		name    string
		cron    string
		tz      string
		matched time.Time
		want    time.Time
		wantNil bool
		wantErr bool
	}{
		{
			// 06:00, 07:00, 08:00, 09:00 match; 10:00 is the last quiet hour of the off window.
			name:    "business morning cluster ends at last matching minute plus one hour",
			cron:    "0 1-3,6-9 * * 1-5",
			matched: time.Date(2026, time.November, 16, 6, 0, 0, 0, time.UTC),
			want:    time.Date(2026, time.November, 16, 10, 0, 0, 0, time.UTC),
		},
		{
			name:    "single daily match",
			cron:    "0 22 * * *",
			matched: time.Date(2026, time.November, 16, 22, 0, 0, 0, time.UTC),
			want:    time.Date(2026, time.November, 16, 23, 0, 0, 0, time.UTC),
		},
		{
			name:    "two-hourly cadence flips back after one hour",
			cron:    "0 */2 * * *",
			matched: time.Date(2026, time.November, 16, 4, 0, 0, 0, time.UTC),
			want:    time.Date(2026, time.November, 16, 5, 0, 0, 0, time.UTC),
		},
		{
			// Monday 00:00 matches, the next match is the following Monday, so
			// the channel is back one hour after the matched minute.
			name:    "weekly cron: back one hour after the matched minute",
			cron:    "0 0 * * 1",
			matched: base,
			want:    time.Date(2026, time.November, 16, 1, 0, 0, 0, time.UTC),
		},
		{name: "every-minute cron has no off window", cron: "* * * * *", matched: base, wantNil: true},
		{
			// Same business cluster evaluated in Tokyo: matched at 06:00 JST
			// (21:00 UTC the prior day), off window ends 09:00 JST, back on at
			// 10:00 JST = 01:00 UTC. Verifies the location load and the
			// absolute +1h stored as an instant.
			name:    "timezone: business cluster in Tokyo",
			cron:    "0 1-3,6-9 * * 1-5",
			tz:      "Asia/Tokyo",
			matched: time.Date(2026, time.November, 15, 21, 0, 0, 0, time.UTC),
			want:    time.Date(2026, time.November, 16, 1, 0, 0, 0, time.UTC),
		},
		{
			// 2027-03-14 is the US spring-forward day. Matched at 08:00 EDT
			// (12:00 UTC); the scan runs to 14:00 EDT (18:00 UTC), whose next
			// occurrence is 06:00 EDT the next day (10:00 UTC) — a 16-hour
			// gap, so the channel is back one absolute hour later at 19:00 UTC.
			name:    "DST spring forward in New York",
			cron:    "0 6-14 * * *",
			tz:      "America/New_York",
			matched: time.Date(2027, time.March, 14, 12, 0, 0, 0, time.UTC),
			want:    time.Date(2027, time.March, 14, 19, 0, 0, 0, time.UTC),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := oppositeReenableExpiry(test.cron, test.tz, test.matched)
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if test.wantNil {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.True(t, got.Equal(test.want), "got %v want %v", *got, test.want)
		})
	}
}

func TestRunScheduledDisableSweepOppositeReenable(t *testing.T) {
	now := time.Now().UTC()
	minute := now.Truncate(time.Minute)

	client, ctx, svc, ch := withScheduledDisablePolicy(t, "opposite", &objects.ChannelScheduledDisable{
		DisableCron:  fmt.Sprintf("%d %d * * *", minute.Minute(), minute.Hour()),
		ReenableMode: "opposite",
	})

	svc.runScheduledDisableSweep(context.Background())

	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusDisabled, updated.Status)
	require.NotNil(t, updated.AutoDisabledAt)
	// The matching minute is a single-match cluster, so the channel comes back
	// one hour later.
	require.NotNil(t, updated.AutoDisableExpiresAt)
	require.True(t, updated.AutoDisableExpiresAt.Equal(minute.Add(time.Hour)),
		"got %v want %v", updated.AutoDisableExpiresAt, minute.Add(time.Hour))
	require.Equal(t, scheduledDisableReason, lo.FromPtr(updated.ErrorMessage))

	// After the stored expiry, the sweep re-enables the channel.
	_, err = client.Channel.UpdateOneID(ch.ID).
		SetAutoDisableExpiresAt(now.Add(-time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	svc.runScheduledDisableSweep(context.Background())

	recovered, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	// The disable cron may fire again within the minute; when it does the
	// re-enabled channel is immediately disabled with a fresh expiry, which is
	// the correct steady state. Only assert that recovery happened if the
	// minute did not re-match.
	if recovered.Status == channel.StatusEnabled {
		require.Nil(t, recovered.AutoDisabledAt)
		require.Nil(t, recovered.AutoDisableExpiresAt)
	} else {
		require.Equal(t, channel.StatusDisabled, recovered.Status)
		require.NotNil(t, recovered.AutoDisableExpiresAt)
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
			ReenableMode: "cron",
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
			ReenableMode: "cron",
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
			ReenableMode: "cron",
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
		ReenableMode: "cron",
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
