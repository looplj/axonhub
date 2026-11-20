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
		{
			// Matches Monday 00:00; hourly matches run until Friday 23:00,
			// whose next match is the following Monday 00:00 — a 2940-minute
			// (49-hour) gap. The off window ends at Friday 23:00 + 1h, i.e.
			// the start of the weekend.
			name:    "weekly weekday cron: off window spans the weekend",
			cron:    "0 * * * 1-5",
			matched: base,
			want:    time.Date(2026, time.November, 21, 0, 0, 0, 0, time.UTC),
		},
		{
			// Every hour of January and February: the run of consecutive
			// matches lasts ~59 days. Disabled Jan 15, back one hour after
			// the February run ends: Mar 1 00:00.
			name:    "multi-month cron run resolves at the end of the run",
			cron:    "0 * * 1-2 *",
			matched: time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC),
			want:    time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			// Every hour of January through July: a ~199-day run, longer
			// than the pinned 5-day and ~59-day runs and shorter than the
			// 366-day bound. Disabled Jan 15, back one hour after the July
			// run ends: Aug 1 00:00.
			name:    "mid-year cron run resolves at the end of the run",
			cron:    "0 * * 1-7 *",
			matched: time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC),
			want:    time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			// supercronic's non-strict Parse accepts 7-field expressions:
			// every hour of 2026. The year list exhausts at the end of the
			// run, expr.Next returns the zero time, and the run must end
			// there: back one hour after the last 2026 match, Jan 1 2027.
			name:    "year-scoped run ends when the year list exhausts",
			cron:    "0 * * * * 2026",
			matched: time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC),
			want:    time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
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

func TestSweepScheduledDisableStaleScheduleDoesNotFire(t *testing.T) {
	// The sweep fires a guarded UPDATE for the channel row it read. Any admin
	// edit landing between the read and the write advances UpdatedAt, so the
	// UPDATE must affect 0 rows and the channel must not be disabled.
	now := time.Now().UTC().Truncate(time.Minute)
	client, ctx, svc, ch := withScheduledDisablePolicy(t, "stale-disable", &objects.ChannelScheduledDisable{
		DisableCron:  fmt.Sprintf("%d %d * * *", now.Minute(), now.Hour()),
		ReenableMode: "cron",
		ReenableCron: "0 0 1 1 *",
	})

	// The sweep's read is done (snapshot); an admin edit now lands before the
	// guarded write.
	snapshot, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	_, err = client.Channel.UpdateOneID(ch.ID).
		SetRemark("edited after sweep read").
		Save(ctx)
	require.NoError(t, err)

	disabled, err := svc.sweepScheduledDisable(ctx, snapshot, snapshot.Policies.ScheduledDisable, now)
	require.NoError(t, err)
	require.False(t, disabled, "a stale schedule must not disable the channel")

	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusEnabled, updated.Status)
	require.Nil(t, updated.AutoDisabledAt)
	require.Nil(t, updated.AutoDisableExpiresAt)
	require.Empty(t, updated.ErrorMessage)
	require.Equal(t, "edited after sweep read", lo.FromPtr(updated.Remark))
}

func TestSweepScheduledReenableStaleRowNotOverwritten(t *testing.T) {
	// Same compare-and-set on the recovery half: an edit after the sweep's
	// read must survive the re-enable UPDATE.
	now := time.Now()
	client, ctx, svc, ch := withScheduledDisablePolicy(t, "stale-reenable", &objects.ChannelScheduledDisable{
		DisableCron:  "0 0 1 1 *",
		ReenableMode: "cron",
		ReenableCron: "0 0 2 1 *",
	})

	_, err := client.Channel.UpdateOneID(ch.ID).
		SetStatus(channel.StatusDisabled).
		SetErrorMessage(scheduledDisableReason).
		SetAutoDisabledAt(now.Add(-time.Hour)).
		SetAutoDisableExpiresAt(now.Add(-time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	// The sweep's read is done (snapshot); an admin edit now lands before the
	// guarded write.
	snapshot, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	_, err = client.Channel.UpdateOneID(ch.ID).
		SetRemark("edited after sweep read").
		Save(ctx)
	require.NoError(t, err)

	recovered, err := svc.sweepScheduledReenable(ctx, snapshot, now)
	require.NoError(t, err)
	require.False(t, recovered, "a stale re-enable must not overwrite a concurrent edit")

	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusDisabled, updated.Status)
	require.NotNil(t, updated.AutoDisabledAt)
	require.NotNil(t, updated.AutoDisableExpiresAt)
	require.Equal(t, scheduledDisableReason, lo.FromPtr(updated.ErrorMessage))
	require.Equal(t, "edited after sweep read", lo.FromPtr(updated.Remark))
}

// withScheduledDisabledChannel stores a channel already disabled by its
// schedule, with the sweep's markers and a future re-enable expiry.
func withScheduledDisabledChannel(t *testing.T, name string, sd *objects.ChannelScheduledDisable, expiresAt time.Time) (*ent.Client, context.Context, *ChannelService, *ent.Channel) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	ch := createTestChannelWithAPIKeys(t, client, ctx, name, []string{"test-key"})

	_, err := client.Channel.UpdateOneID(ch.ID).
		SetPolicies(objects.ChannelPolicies{ScheduledDisable: sd}).
		SetStatus(channel.StatusDisabled).
		SetErrorMessage(scheduledDisableReason).
		SetAutoDisabledAt(time.Now().Add(-time.Hour)).
		SetAutoDisableExpiresAt(expiresAt).
		Save(ctx)
	require.NoError(t, err)

	return client, ctx, svc, ch
}

func TestScheduledDisableNotOverwrittenByAPIKeyOperations(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour)
	client, ctx, svc, ch := withScheduledDisabledChannel(t, "key-takeover", &objects.ChannelScheduledDisable{
		// A cron that cannot match the current minute, so the final sweep can
		// only recover the channel, never re-disable it mid-test.
		DisableCron:  "0 0 1 1 *",
		ReenableMode: "cron",
		ReenableCron: "0 1 1 1 *",
	}, expiresAt)

	// (a) Disabling the last key must not take the scheduled disable over:
	// the scheduled reason and the stored re-enable expiry survive.
	require.NoError(t, svc.DisableAPIKey(ctx, ch.ID, "test-key", 401, "key revoked"))

	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusDisabled, updated.Status)
	require.Equal(t, scheduledDisableReason, lo.FromPtr(updated.ErrorMessage),
		"the scheduled disable reason must not be overwritten by the key path")
	require.NotNil(t, updated.AutoDisabledAt)
	require.NotNil(t, updated.AutoDisableExpiresAt)
	require.True(t, updated.AutoDisableExpiresAt.Equal(expiresAt),
		"the stored re-enable expiry must survive: got %v want %v", updated.AutoDisableExpiresAt, expiresAt)

	// (b) Re-enabling the key must not resurrect the channel early; it stays
	// disabled until the stored expiry, which the sweep then clears.
	require.NoError(t, svc.EnableAPIKey(ctx, ch.ID, "test-key"))

	recovered, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusDisabled, recovered.Status,
		"key recovery must not re-enable a scheduled-disabled channel")
	require.Equal(t, scheduledDisableReason, lo.FromPtr(recovered.ErrorMessage))
	require.True(t, recovered.AutoDisableExpiresAt.Equal(expiresAt))

	// Once the stored expiry passes, the sweep re-enables the channel as planned.
	_, err = client.Channel.UpdateOneID(ch.ID).
		SetAutoDisableExpiresAt(time.Now().Add(-time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	svc.runScheduledDisableSweep(context.Background())

	final, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusEnabled, final.Status)
	require.Nil(t, final.AutoDisabledAt)
	require.Nil(t, final.AutoDisableExpiresAt)
	require.Empty(t, final.ErrorMessage)
}

// TestScheduledDisableSurvivesQuotaReconcile pins the same ownership rule on
// the quota side: a scheduled-disabled channel whose last key exhausts through
// the quota reconciler keeps the scheduled reason and the stored re-enable
// expiry, and a later available observation does not resurrect the channel
// before the expiry.
func TestScheduledDisableSurvivesQuotaReconcile(t *testing.T) {
	env := setupQuotaKeyTestEnv(t)
	ch := env.createOpenCodeChannel(t, quotaTestKeyA, quotaTestKeyB)

	// The channel is disabled by its schedule; the markers are exactly what
	// the sweep would store.
	expiresAt := time.Now().Add(time.Hour)
	_, err := env.client.Channel.UpdateOneID(ch.ID).
		SetPolicies(objects.ChannelPolicies{ScheduledDisable: &objects.ChannelScheduledDisable{
			DisableCron:  "0 0 1 1 *",
			ReenableMode: "cron",
			ReenableCron: "0 0 2 1 *",
		}}).
		SetStatus(channel.StatusDisabled).
		SetErrorMessage(scheduledDisableReason).
		SetAutoDisabledAt(time.Now().Add(-time.Hour)).
		SetAutoDisableExpiresAt(expiresAt).
		Save(env.ctx)
	require.NoError(t, err)

	// A scheduled-disabled channel is not pollable, so the scheduled pass
	// skips it; the reset-refresh path reaches it and must reconcile.
	check := func(t *testing.T) {
		t.Helper()
		current := env.reload(t, ch.ID)
		env.quota.mu.Lock()
		defer env.quota.mu.Unlock()
		env.quota.checkChannelQuota(env.ctx, quotaCheckGroup{
			channels:   []*ent.Channel{current},
			accountKey: quotaAccountKey("opencode_go", current),
		}, time.Now())
	}

	// The last key exhausts mid-window.
	env.server.usage(quotaTestKeyA, 100, 10, 10)
	env.server.usage(quotaTestKeyB, 100, 10, 10)
	check(t)

	kept := env.reload(t, ch.ID)
	require.Equal(t, channel.StatusDisabled, kept.Status)
	require.Equal(t, scheduledDisableReason, lo.FromPtr(kept.ErrorMessage),
		"the scheduled disable reason must not be overwritten by the quota takeover")
	require.NotNil(t, kept.AutoDisabledAt)
	require.NotNil(t, kept.AutoDisableExpiresAt)
	require.True(t, kept.AutoDisableExpiresAt.Equal(expiresAt),
		"the stored re-enable expiry must survive the mid-window exhaustion")
	require.ElementsMatch(t, []string{quotaTestKeyA, quotaTestKeyB}, quotaOwnedKeys(kept))

	// A later available observation parks no key, but must not resurrect the
	// channel: its recovery is owned by the stored expiry.
	env.server.usage(quotaTestKeyA, 5, 5, 5)
	env.server.usage(quotaTestKeyB, 5, 5, 5)
	check(t)

	recovered := env.reload(t, ch.ID)
	require.Equal(t, channel.StatusDisabled, recovered.Status,
		"key recovery must not re-enable a scheduled-disabled channel")
	require.Equal(t, scheduledDisableReason, lo.FromPtr(recovered.ErrorMessage))
	require.NotNil(t, recovered.AutoDisableExpiresAt)
	require.True(t, recovered.AutoDisableExpiresAt.Equal(expiresAt))
	require.Empty(t, quotaOwnedKeys(recovered))
}
