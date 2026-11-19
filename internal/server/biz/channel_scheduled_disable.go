// 职责: 调度化停用的校验、下次/上次 cron 时刻解析与每分钟清扫任务。
// 关联文件:
//   internal/server/biz/channel.go
//   internal/server/biz/channel_apikey.go
//   internal/objects/channel.go
// 注意事项:
//   清扫任务的多实例安全依赖受 WHERE 谓词保护的 UPDATE，affected == 0 表示竞争失败。
//   恢复半段与 cleanupExpiredAutoDisabledChannels 依赖同一不变量：
//   status=disabled + AutoDisabledAt + AutoDisableExpiresAt 只在设置了过期时间的自动禁用下出现。
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
	sd.ReenableCron = strings.TrimSpace(sd.ReenableCron)
	sd.ReenableTimezone = strings.TrimSpace(sd.ReenableTimezone)

	if sd.DisableCron == "" && sd.DisableTimezone == "" && sd.ReenableCron == "" && sd.ReenableTimezone == "" {
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

	if sd.ReenableCron != "" {
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

// prevScheduledCronOccurrence returns the last occurrence of cron at or before
// t, evaluated in tz ("" ⇒ UTC). cronexpr has no Previous, so it walks back
// one minute at a time checking which minutes are occurrences; the 5000-minute
// bound errors for sparse expressions whose last occurrence is older (for
// example "0 0 1 1 *" from mid-year).
func prevScheduledCronOccurrence(cron, tz string, t time.Time) (time.Time, error) {
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

	local := t.In(loc)
	minute := local.Truncate(time.Minute)
	for range 5000 {
		// minute is an occurrence iff the first occurrence strictly after the
		// previous minute is exactly this minute.
		if expr.Next(minute.Add(-time.Minute)).Equal(minute) {
			return minute, nil
		}
		minute = minute.Add(-time.Minute)
	}

	return time.Time{}, fmt.Errorf("cron expression %q has no occurrence within 5000 minutes of %s", cron, local.Format(time.RFC3339))
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

	// Disable half: fire exactly once per matching minute. The zero-second
	// comparison gives minute precision, consistent with the task's minute tick.
	if ch.Status == channel.StatusEnabled {
		disabled, err := svc.sweepScheduledDisable(ctx, ch, sd, now)
		if err != nil {
			return changed, err
		}
		changed = disabled
	}

	// Re-enable half.
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
		changed = changed || recovered
	}

	return changed, nil
}

// sweepScheduledDisable disables one channel whose disable cron matches now.
func (svc *ChannelService) sweepScheduledDisable(ctx context.Context, ch *ent.Channel, sd *objects.ChannelScheduledDisable, now time.Time) (bool, error) {
	loc := time.UTC
	if sd.DisableTimezone != "" {
		loaded, err := time.LoadLocation(sd.DisableTimezone)
		if err != nil {
			return false, fmt.Errorf("invalid disable timezone %q: %w", sd.DisableTimezone, err)
		}
		loc = loaded
	}

	expr, err := cronexpr.Parse(sd.DisableCron)
	if err != nil {
		return false, fmt.Errorf("invalid disable cron expression %q: %w", sd.DisableCron, err)
	}

	t := now.In(loc)
	minute := t.Truncate(time.Minute)
	// The current minute matches iff it is an occurrence of the cron: the first
	// occurrence strictly after the previous minute is exactly this minute.
	next := expr.Next(minute.Add(-time.Minute))
	if next.IsZero() || !next.Equal(minute) {
		return false, nil
	}

	var expiresAt *time.Time
	if sd.ReenableCron != "" {
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
	}

	affected, err := svc.entFromContext(ctx).Channel.Update().
		Where(
			channel.IDEQ(ch.ID),
			channel.StatusEQ(channel.StatusEnabled),
		).
		SetStatus(channel.StatusDisabled).
		SetErrorMessage(scheduledDisableReason).
		SetAutoDisabledAt(now).
		SetNillableAutoDisableExpiresAt(expiresAt).
		Save(ctx)
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
	})
}
