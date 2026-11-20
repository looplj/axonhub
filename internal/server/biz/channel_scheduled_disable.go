// 职责: 调度化停用的校验、cron 时刻解析、opposite 重启用时刻推导与每分钟清扫任务。
// 关联文件:
//
//	internal/server/biz/channel.go
//	internal/server/biz/channel_apikey.go
//	internal/objects/channel.go
//
// 注意事项:
//
//	清扫任务的多实例安全依赖受 WHERE 谓词保护的 UPDATE，affected == 0 表示竞争失败。
//	恢复半段与 cleanupExpiredAutoDisabledChannels 依赖同一不变量：
//	status=disabled + AutoDisabledAt + AutoDisableExpiresAt 只在设置了过期时间的自动禁用下出现。
//	opposite 模式的重启用时刻在禁用时一次性算好并存储，恢复不依赖任何实例再次求值表达式；
//	每分钟都命中的 cron 无空闲时段，渠道保持禁用直到人工启用。

package biz

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aptible/supercronic/cronexpr"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
)

// scheduledDisableReason is the error message stored on a channel disabled by
// its wall-clock schedule.
const scheduledDisableReason = "Disabled by scheduled disable schedule"

// Re-enable modes stored in ChannelScheduledDisable.ReenableMode.
const (
	reenableModeCron     = "cron"
	reenableModeOpposite = "opposite"
)

// oppositeReenableScanMinutes is the opposite-mode scan window in minutes
// (366 days). A run of consecutive matches can last months, so the window has
// to cover a year. The walk stops once it has advanced this far in
// wall-clock time: only a cron matching every hour of every day is still
// within an hour of its next occurrence that far ahead, and that is the one
// case a nil result legitimately describes.
const oppositeReenableScanMinutes = 366 * 24 * 60

// NormalizeScheduledDisable trims the schedule's cron/timezone fields and
// validates them. A nil schedule is valid (no schedule configured); an
// all-empty struct is dropped to nil so clearing the schedule stores nothing.
// The disable cron must always parse; the re-enable cron, when present, must
// parse too. Returns an error for invalid input.
func NormalizeScheduledDisable(policies *objects.ChannelPolicies) error {
	if policies == nil {
		return nil
	}
	sd := policies.ScheduledDisable
	if sd == nil {
		return nil
	}

	sd.DisableCron = strings.TrimSpace(sd.DisableCron)
	sd.DisableTimezone = strings.TrimSpace(sd.DisableTimezone)
	sd.ReenableMode = strings.TrimSpace(sd.ReenableMode)
	sd.ReenableCron = strings.TrimSpace(sd.ReenableCron)
	sd.ReenableTimezone = strings.TrimSpace(sd.ReenableTimezone)

	// A re-enable cron without an explicit mode is a legacy "cron" schedule.
	if sd.ReenableMode == "" && sd.ReenableCron != "" {
		sd.ReenableMode = reenableModeCron
	}

	switch sd.ReenableMode {
	case "":
	case reenableModeCron:
		if sd.ReenableCron == "" {
			return fmt.Errorf("channel scheduled disable re-enable cron mode requires a cron expression")
		}
	case reenableModeOpposite:
		// The re-enable is derived from the disable cron; the cron fields
		// would be dead settings, so they are cleared.
		sd.ReenableCron = ""
		sd.ReenableTimezone = ""
	default:
		return fmt.Errorf("channel scheduled disable re-enable mode %q is not supported", sd.ReenableMode)
	}

	if sd.DisableCron == "" && sd.DisableTimezone == "" && sd.ReenableMode == "" && sd.ReenableCron == "" && sd.ReenableTimezone == "" {
		policies.ScheduledDisable = nil
		return nil
	}

	if sd.DisableCron == "" {
		return fmt.Errorf("channel scheduled disable requires a cron expression")
	}
	if _, err := cronexpr.Parse(sd.DisableCron); err != nil {
		return fmt.Errorf("channel scheduled disable has invalid cron expression %q: %w", sd.DisableCron, err)
	}
	if sd.DisableTimezone != "" {
		if _, err := time.LoadLocation(sd.DisableTimezone); err != nil {
			return fmt.Errorf("channel scheduled disable has invalid timezone %q: %w", sd.DisableTimezone, err)
		}
	}

	if sd.ReenableMode == reenableModeCron {
		if _, err := cronexpr.Parse(sd.ReenableCron); err != nil {
			return fmt.Errorf("channel scheduled disable re-enable has invalid cron expression %q: %w", sd.ReenableCron, err)
		}
	}
	if sd.ReenableTimezone != "" {
		if _, err := time.LoadLocation(sd.ReenableTimezone); err != nil {
			return fmt.Errorf("channel scheduled disable re-enable has invalid timezone %q: %w", sd.ReenableTimezone, err)
		}
	}

	return nil
}

// nextScheduledCronOccurrence returns the first occurrence of cron strictly
// after from, evaluated in tz ("" ⇒ UTC). It is the scheduled-disable twin of
// nextAPIKeyRuleCronOccurrence (channel_auto_disable.go), which stays
// rule-specific.
func nextScheduledCronOccurrence(cron, tz string, from time.Time) (time.Time, error) {
	loc := time.UTC
	if tz != "" {
		parsed, err := time.LoadLocation(tz)
		if err != nil {
			return time.Time{}, fmt.Errorf("channel scheduled disable has invalid timezone %q: %w", tz, err)
		}
		loc = parsed
	}

	expr, err := cronexpr.Parse(cron)
	if err != nil {
		return time.Time{}, fmt.Errorf("channel scheduled disable has invalid cron expression %q: %w", cron, err)
	}

	next := expr.Next(from.In(loc))
	if next.IsZero() {
		return time.Time{}, fmt.Errorf("cron expression %q never fires again", cron)
	}

	return next, nil
}

// cronMatchesMinute reports whether minute (already truncated to the minute)
// is an occurrence of cron, evaluated in tz ("" ⇒ UTC). A minute is an
// occurrence iff the first occurrence strictly after the previous minute is
// exactly this minute.
func cronMatchesMinute(cron, tz string, minute time.Time) (bool, error) {
	loc := time.UTC
	if tz != "" {
		loaded, err := time.LoadLocation(tz)
		if err != nil {
			return false, fmt.Errorf("channel scheduled disable has invalid timezone %q: %w", tz, err)
		}
		loc = loaded
	}

	expr, err := cronexpr.Parse(cron)
	if err != nil {
		return false, fmt.Errorf("channel scheduled disable has invalid cron expression %q: %w", cron, err)
	}

	next := expr.Next(minute.In(loc).Add(-time.Minute))
	return !next.IsZero() && next.Equal(minute.In(loc)), nil
}

// oppositeReenableExpiry resolves the re-enable instant for the "opposite"
// mode. It walks the occurrences of the cron forward from the matched minute
// for the end of the off window the schedule describes: a matching minute P
// whose next occurrence is more than an hour away, and the channel re-enables
// at P + 1h. For "0 1-3,6-9 * * 1-5" matched at 06:00, the off window runs
// 06:00-09:00, so the channel is back at 10:00. Walking occurrence to
// occurrence instead of minute by minute keeps a year-long run of matches to
// at most a few thousand steps; the walk is still bounded by
// oppositeReenableScanMinutes in wall-clock terms: the walk stops once it has
// covered that many minutes. A nil result means the cron matches every hour
// of every day (every occurrence is at most an hour apart), and the channel
// stays disabled until an operator re-enables it.
func oppositeReenableExpiry(cron, tz string, matchedMinute time.Time) (*time.Time, error) {
	loc := time.UTC
	if tz != "" {
		loaded, err := time.LoadLocation(tz)
		if err != nil {
			return nil, fmt.Errorf("channel scheduled disable has invalid timezone %q: %w", tz, err)
		}
		loc = loaded
	}

	expr, err := cronexpr.Parse(cron)
	if err != nil {
		return nil, fmt.Errorf("channel scheduled disable has invalid cron expression %q: %w", cron, err)
	}

	// matchedMinute is a matching minute of the cron (the sweep only calls
	// this on a match), so it is the first occurrence to walk.
	minute := matchedMinute.In(loc)
	deadline := minute.Add(time.Duration(oppositeReenableScanMinutes) * time.Minute)
	for {
		next := expr.Next(minute)
		if next.IsZero() {
			// A year-scoped 7-field expression exhausts its year list and
			// Next returns the zero time: the run ends here, so the off
			// window starts now.
			expiry := minute.Add(time.Hour)
			return &expiry, nil
		}
		if next.After(minute.Add(time.Hour)) {
			expiry := minute.Add(time.Hour)
			return &expiry, nil
		}
		if !next.Before(deadline) {
			return nil, nil
		}
		minute = next
	}
}

// runScheduledDisableSweep runs once per minute. It disables channels whose
// disable cron matches the current minute and re-enables scheduled-disabled
// channels whose re-enable cron has passed. Both sweeps are guarded
// conditional UPDATEs (affected == 0 means another instance or an operator
// won), so concurrent instances are safe; the process-local apiKeyOpsLock is
// not used here. The recovery half overlaps cleanupExpiredAutoDisabledChannels
// on the same final state (enabled, markers cleared) — both are guarded the
// same way by the WHERE predicates.
func (svc *ChannelService) runScheduledDisableSweep(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	ctx = authz.WithSystemBypass(ctx, "channel-scheduled-disable")

	channelIDs, err := svc.entFromContext(ctx).Channel.Query().
		Where(channel.StatusIn(channel.StatusEnabled, channel.StatusDisabled)).
		IDs(ctx)
	if err != nil {
		log.Error(ctx, "Failed to query channels for scheduled disable sweep", log.Cause(err))
		return
	}

	now := time.Now()
	changed := false
	for _, channelID := range channelIDs {
		changedForChannel, err := svc.sweepChannelScheduledDisable(ctx, channelID, now)
		if err != nil {
			log.Error(ctx, "Failed to sweep channel scheduled disable",
				log.Int("channel_id", channelID),
				log.Cause(err),
			)
			continue
		}
		if changedForChannel {
			changed = true
		}
	}

	if changed {
		svc.asyncReloadChannels()
	}
}

// sweepChannelScheduledDisable applies both schedule halves to one channel.
// Returns true when the channel state changed (for a single cache reload).
func (svc *ChannelService) sweepChannelScheduledDisable(ctx context.Context, channelID int, now time.Time) (bool, error) {
	ch, err := svc.entFromContext(ctx).Channel.Get(ctx, channelID)
	if err != nil {
		return false, err
	}
	sd := ch.Policies.ScheduledDisable
	if sd == nil {
		return false, nil
	}

	changed := false

	// Re-enable half runs before the disable half: when an "opposite"
	// re-enable and the next disable landing on the same minute collide, the
	// channel flips off again instead of the re-enable being swallowed.
	//
	// Load-bearing invariant: a channel reaches status=disabled +
	// AutoDisabledAt + AutoDisableExpiresAt != nil only via a disable that sets
	// an expiry — this sweep, a temporary keyless auto-disable rule, or a
	// disable_until_cron rule. All three re-enable at their expiry by design,
	// so re-enabling here is correct for all of them (it is the same state
	// cleanupExpiredAutoDisabledChannels already re-enables). Operator disables
	// and all-keys disables leave AutoDisableExpiresAt nil and are immune.
	if ch.Status == channel.StatusDisabled &&
		ch.AutoDisabledAt != nil &&
		ch.AutoDisableExpiresAt != nil &&
		now.After(*ch.AutoDisableExpiresAt) {
		recovered, err := svc.sweepScheduledReenable(ctx, ch, now)
		if err != nil {
			return changed, err
		}
		changed = recovered
		ch, err = svc.entFromContext(ctx).Channel.Get(ctx, channelID)
		if err != nil {
			return changed, err
		}
		if ch.Policies.ScheduledDisable == nil {
			return changed, nil
		}
		sd = ch.Policies.ScheduledDisable
	}

	// Disable half: fire exactly once per matching minute, consistent with the
	// task's minute tick.
	if ch.Status == channel.StatusEnabled {
		disabled, err := svc.sweepScheduledDisable(ctx, ch, sd, now)
		if err != nil {
			return changed, err
		}
		changed = changed || disabled
	}

	return changed, nil
}

// sweepScheduledDisable disables one channel whose disable cron matches now.
func (svc *ChannelService) sweepScheduledDisable(ctx context.Context, ch *ent.Channel, sd *objects.ChannelScheduledDisable, now time.Time) (bool, error) {
	matched, err := cronMatchesMinute(sd.DisableCron, sd.DisableTimezone, now.Truncate(time.Minute))
	if err != nil {
		return false, err
	}
	if !matched {
		return false, nil
	}

	// The re-enable expiry is stored on the disable, so recovery does not
	// depend on any instance re-evaluating the schedule later.
	var expiresAt *time.Time
	switch sd.ReenableMode {
	case reenableModeCron:
		exp, err := nextScheduledCronOccurrence(sd.ReenableCron, sd.ReenableTimezone, now)
		if err != nil {
			// Do not disable on a schedule whose recovery cannot be resolved.
			log.Warn(ctx, "Skipping scheduled disable with unresolvable re-enable cron",
				log.Int("channel_id", ch.ID),
				log.Cause(err),
			)
			return false, nil
		}
		expiresAt = &exp
	case reenableModeOpposite:
		// A nil result means the disable cron matches every minute: there is
		// no off window, so the channel stays disabled until re-enabled by an
		// operator.
		exp, err := oppositeReenableExpiry(sd.DisableCron, sd.DisableTimezone, now.Truncate(time.Minute))
		if err != nil {
			log.Warn(ctx, "Skipping scheduled disable with unresolvable opposite re-enable",
				log.Int("channel_id", ch.ID),
				log.Cause(err),
			)
			return false, nil
		}
		expiresAt = exp
	}

	update := svc.entFromContext(ctx).Channel.Update().
		Where(
			channel.IDEQ(ch.ID),
			channel.StatusEQ(channel.StatusEnabled),
			// The schedule may have been removed or changed between the read
			// above and this write: any edit advances UpdatedAt, so a stale
			// schedule must not fire (affected == 0 skips the channel).
			channel.UpdatedAtEQ(ch.UpdatedAt),
		).
		SetStatus(channel.StatusDisabled).
		SetErrorMessage(scheduledDisableReason).
		SetAutoDisabledAt(now)
	// SetNillableAutoDisableExpiresAt(nil) is a no-op in ent; a nil expiry
	// ("stays disabled") must clear any stale value, matching the sibling
	// auto-disable writer.
	if expiresAt != nil {
		update = update.SetAutoDisableExpiresAt(*expiresAt)
	} else {
		update = update.ClearAutoDisableExpiresAt()
	}

	affected, err := update.Save(ctx)
	if err != nil {
		return false, err
	}
	if affected == 0 {
		return false, nil
	}

	log.Info(ctx, "Disabled channel by scheduled disable schedule",
		log.Int("channel_id", ch.ID),
		log.String("channel_name", ch.Name),
	)
	svc.publishChannelAutoDisabledWebhook(ctx, ch.ID)

	return true, nil
}

// sweepScheduledReenable re-enables one channel whose stored auto-disable
// expiry has passed.
func (svc *ChannelService) sweepScheduledReenable(ctx context.Context, ch *ent.Channel, now time.Time) (bool, error) {
	affected, err := svc.entFromContext(ctx).Channel.Update().
		Where(
			channel.IDEQ(ch.ID),
			channel.StatusEQ(channel.StatusDisabled),
			channel.AutoDisabledAtNotNil(),
			channel.AutoDisableExpiresAtNotNil(),
			channel.AutoDisableExpiresAtLT(now),
			// Same compare-and-set as the disable half: an edit after the
			// sweep's read (e.g. the schedule was removed) must not be
			// overwritten by a stale re-enable (affected == 0 skips it).
			channel.UpdatedAtEQ(ch.UpdatedAt),
		).
		SetStatus(channel.StatusEnabled).
		ClearErrorMessage().
		ClearAutoDisabledAt().
		ClearAutoDisableExpiresAt().
		Save(ctx)
	if err != nil {
		return false, err
	}
	if affected == 0 {
		return false, nil
	}

	log.Info(ctx, "Re-enabled channel by scheduled disable schedule",
		log.Int("channel_id", ch.ID),
		log.String("channel_name", ch.Name),
	)
	svc.publishChannelAutoEnabled(ctx, ch.ID)

	return true, nil
}

// publishChannelAutoDisabledWebhook notifies webhook targets that a channel
// was disabled by a scheduled disable. It reuses the channel.auto_disabled
// event (no new event name) with trigger type "scheduled"; webhook render
// templates see trigger.type == "scheduled" and an empty status code.
func (svc *ChannelService) publishChannelAutoDisabledWebhook(ctx context.Context, channelID int) {
	updatedChannel, err := svc.entFromContext(ctx).Channel.Get(ctx, channelID)
	if err != nil {
		log.Error(ctx, "Failed to fetch disabled channel for webhook notification",
			log.Int("channel_id", channelID),
			log.Cause(err),
		)
		return
	}

	svc.asyncNotifyChannelAutoDisabled(ctx, ChannelAutoDisabledEvent{
		ChannelID:       updatedChannel.ID,
		ChannelName:     updatedChannel.Name,
		ChannelProvider: updatedChannel.Type.String(),
		ChannelBaseURL:  updatedChannel.BaseURL,
		ChannelStatus:   updatedChannel.Status.String(),
		Reason:          scheduledDisableReason,
		OccurredAt:      time.Now(),
		TriggerType:     "scheduled",
	})
}
