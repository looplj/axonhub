// 职责: 保存全站目录检查范围与每日时刻，并汇总待管理员确认的差异。
// 关联文件:
//
//	internal/server/biz/model_catalog_preview.go
//	internal/server/biz/system.go
//	internal/server/biz/fx_module.go
//
// 注意事项:
//
//	调度只记录检查结果。手动预览不会订阅服务商，也不会改变每日任务范围。

package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/system"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/server/scheduler"
)

const (
	catalogCheckSettingsKey = "model_catalog_check_settings"
	catalogCheckResultKey   = "model_catalog_check_result"
)

type CatalogCheckSettings struct {
	Enabled      bool     `json:"enabled"`
	Time         string   `json:"time"`
	Timezone     string   `json:"timezone"`
	Providers    []string `json:"providers"`
	IgnorePrefix bool     `json:"ignorePrefix"`
}

type CatalogCheckStatus struct {
	Settings  CatalogCheckSettings
	NextCheck *time.Time
	catalogCheckResult
}

type catalogCheckResult struct {
	LastAttempt  *time.Time `json:"LastAttempt"`
	LastSuccess  *time.Time `json:"LastSuccess"`
	Failed       bool       `json:"Failed"`
	Available    int        `json:"Available"`
	Changed      int        `json:"Changed"`
	ScheduledDay string     `json:"ScheduledDay,omitempty"`
}

func (s CatalogCheckSettings) next(now time.Time, scheduledDay string) (time.Time, error) {
	at, err := time.Parse("15:04", s.Time)
	if err != nil {
		return time.Time{}, xerrors.ValidationError("catalog check time must be in HH:MM format")
	}
	if s.Timezone == "" || s.Timezone == "Local" {
		return time.Time{}, xerrors.ValidationError("a site timezone is required")
	}
	zone, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return time.Time{}, xerrors.ValidationError("unknown catalog check timezone")
	}
	// Use the same occurrence for both the minute sweep and the displayed next
	// check. Date may normalize a nonexistent wall time across a DST gap.
	local := now.In(zone)
	for day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC); ; day = day.AddDate(0, 0, 1) {
		candidate := time.Date(day.Year(), day.Month(), day.Day(), at.Hour(), at.Minute(), 0, 0, zone)
		if day.Format(time.DateOnly) != scheduledDay && candidate.After(now) && candidate.Day() == day.Day() && candidate.Hour() == at.Hour() && candidate.Minute() == at.Minute() {
			return candidate, nil
		}
	}
}

func (svc *ModelService) CatalogCheckStatus(ctx context.Context) (*CatalogCheckStatus, error) {
	var state *CatalogCheckStatus
	err := svc.RunInTransaction(ctx, func(ctx context.Context) error {
		var err error
		state, _, err = svc.readCatalogCheck(ctx)
		return err
	})
	return state, err
}

func (svc *ModelService) readCatalogCheck(ctx context.Context) (*CatalogCheckStatus, string, error) {
	settings := CatalogCheckSettings{Time: "08:30", Providers: []string{}}
	value, err := svc.systemService.getSystemValue(ctx, catalogCheckSettingsKey)
	if err == nil {
		if err = json.Unmarshal([]byte(value), &settings); err != nil {
			return nil, "", err
		}
	} else if !ent.IsNotFound(err) {
		return nil, "", err
	}
	state := &CatalogCheckStatus{}
	value, err = svc.systemService.getSystemValue(ctx, catalogCheckResultKey)
	if err == nil {
		if err = json.Unmarshal([]byte(value), &state.catalogCheckResult); err != nil {
			return nil, "", err
		}
	} else if !ent.IsNotFound(err) {
		return nil, "", err
	}
	// The counts and timestamps describe the last completed check, including after scope changes.
	state.Settings = settings
	if settings.Enabled {
		next, err := settings.next(time.Now(), state.ScheduledDay)
		if err != nil {
			return nil, "", err
		}
		state.NextCheck = &next
	}
	return state, value, nil
}

func (svc *ModelService) SaveCatalogCheckSettings(ctx context.Context, settings CatalogCheckSettings) (*CatalogCheckStatus, error) {
	if _, err := settings.next(time.Now(), ""); err != nil {
		return nil, err
	}
	slices.Sort(settings.Providers)
	settings.Providers = slices.Compact(settings.Providers)
	data, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}
	if err = svc.systemService.setSystemValue(ctx, catalogCheckSettingsKey, string(data)); err != nil {
		return nil, err
	}
	return svc.CatalogCheckStatus(ctx)
}

var errCheckClaimChanged = errors.New("another model catalog check is in progress")

// Compare the persisted result to the one read inside the claim transaction.
// A late completion cannot overwrite a newer manual check.
func (svc *ModelService) saveCheckResult(ctx context.Context, previous string, state catalogCheckResult) (string, error) {
	data, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	value := string(data)
	db := svc.entFromContext(ctx)
	if previous == "" {
		err = db.System.Create().SetKey(catalogCheckResultKey).SetValue(value).Exec(ctx)
		if ent.IsConstraintError(err) {
			return "", errCheckClaimChanged
		}
	} else {
		var count int
		count, err = db.System.Update().Where(system.KeyEQ(catalogCheckResultKey), system.ValueEQ(previous)).SetValue(value).Save(ctx)
		if err == nil && count == 0 {
			return "", errCheckClaimChanged
		}
	}
	if err != nil {
		return "", fmt.Errorf("save catalog check result: %w", err)
	}
	return value, nil
}

func (svc *ModelService) claimCatalogCheck(ctx context.Context, now time.Time, scheduled bool) (*CatalogCheckStatus, string, error) {
	var state *CatalogCheckStatus
	var token string
	for range 3 {
		err := svc.RunInTransaction(ctx, func(ctx context.Context) error {
			var old string
			var err error
			state, old, err = svc.readCatalogCheck(ctx)
			if err != nil {
				return err
			}
			if scheduled {
				if !state.Settings.Enabled {
					return nil
				}
				minute := now.Truncate(time.Minute)
				due, err := state.Settings.next(minute.Add(-time.Minute), state.ScheduledDay)
				if err != nil {
					return err
				}
				day := due.Format(time.DateOnly)
				if !due.Equal(minute) {
					return nil
				}
				state.ScheduledDay = day
			}
			state.LastAttempt = &now
			token, err = svc.saveCheckResult(ctx, old, state.catalogCheckResult)
			return err
		})
		if !errors.Is(err, errCheckClaimChanged) && !catalogWriteBusy(err) {
			return state, token, err
		}
	}
	return nil, "", errCheckClaimChanged
}

func (svc *ModelService) CheckModelCatalog(ctx context.Context) (*CatalogCheckStatus, error) {
	return svc.checkModelCatalog(ctx, time.Now().UTC(), false)
}

func (svc *ModelService) checkModelCatalog(ctx context.Context, now time.Time, scheduled bool) (*CatalogCheckStatus, error) {
	return authz.RunAsHost(ctx, svc.entFromContext(ctx), "check-model-catalog", func(ctx context.Context) (*CatalogCheckStatus, error) {
		state, token, err := svc.claimCatalogCheck(ctx, now, scheduled)
		if err != nil || token == "" {
			return state, err
		}
		cfg := state.Settings
		snapshot, err := svc.catalog.snapshot(ctx, false, true)
		var preview *CatalogImportPreview
		if err == nil && snapshot.Source == catalogSourceUpstream {
			preview, err = svc.catalogPreviewFromSnapshot(ctx, CatalogImportScope{Channels: true, Tracked: true, Providers: cfg.Providers, IgnorePrefix: cfg.IgnorePrefix}, snapshot)
		}
		state.Failed = err != nil || preview == nil
		if !state.Failed {
			success := time.Now().UTC()
			state.LastSuccess = &success
			state.Available, state.Changed = 0, 0
			for _, row := range preview.Rows {
				if row.Version == "" {
					state.Available++
				} else if row.Changed && !row.Archived {
					state.Changed++
				}
			}
		}
		_, saveErr := svc.saveCheckResult(ctx, token, state.catalogCheckResult)
		if saveErr != nil {
			return nil, saveErr
		}
		if err != nil {
			return nil, err
		}
		return state, nil
	})
}

func (svc *ModelService) registerCatalogCheck(ctx context.Context, s *scheduler.Scheduler) error {
	// Read the persisted wall time each tick, so changes take effect on every replica
	// and rollback of a settings mutation cannot leave an in-memory schedule behind.
	return s.Register(ctx, scheduler.TaskSpec{Name: "model-catalog-check", Description: "Check global model catalog changes", CronExpr: "* * * * *", Timezone: "UTC"}, func(ctx context.Context) {
		ctx = authz.NewSystemContext(ctx)
		_, err := svc.checkModelCatalog(ctx, time.Now().UTC(), true)
		if err != nil {
			log.Warn(ctx, "scheduled model catalog check failed", log.Cause(err))
		}
	})
}
