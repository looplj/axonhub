// 职责: 管理插件定时任务的设置、跨实例领取和执行记录。
// 关联文件:
//
//	internal/server/plugin/manager.go
//	internal/server/plugin/task_manifest.go
//	internal/server/plugin/invoke.go
//	internal/ent/schema/plugin_schedule.go
//	internal/server/scheduler/scheduler.go
//
// 注意事项:
//
//	数据库租约只保护任务领取。外部副作用应使用业务键实现幂等。

package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"time"

	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/pluginschedule"
	"github.com/looplj/axonhub/internal/ent/pluginschedulerun"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/scheduler"
)

const (
	scheduleTick      = 5 * time.Second
	scheduleTimeout   = 30 * time.Second
	scheduleLease     = time.Minute
	scheduleHistory   = 20
	scheduleWorkers   = 4
	taskSchedulerName = "plugin-tasks"
	runStatusRunning  = "running"
	runStatusUnknown  = "unknown"
)

var errTaskBusy = errors.New("plugin task is already running")

type TaskState struct {
	Key        string
	Cron       string
	Timezone   string
	Overridden bool
	Paused     bool
	NextRunAt  *time.Time
	Runs       []*ent.PluginScheduleRun
}

type pluginTaskRun struct {
	key          string
	installation int
	cancel       context.CancelFunc
}

func (m *Manager) registerTaskScheduler(ctx context.Context, schedulerService *scheduler.Scheduler) error {
	return schedulerService.Register(ctx, scheduler.TaskSpec{Name: taskSchedulerName, FixRate: scheduleTick}, m.scanTasks)
}

// syncTasks runs in the same transaction as manifest changes.
func (m *Manager) syncTasks(ctx context.Context, id int, tasks map[string]objects.PluginTaskSpec) error {
	client := m.client(ctx)
	rows, err := client.PluginSchedule.Query().Where(pluginschedule.InstallationIDEQ(id)).All(ctx)
	if err != nil {
		return err
	}
	byKey := lo.KeyBy(rows, func(row *ent.PluginSchedule) string { return row.Key })
	for _, key := range slices.Sorted(maps.Keys(tasks)) {
		if byKey[key] != nil {
			delete(byKey, key)
			continue
		}
		if err := client.PluginSchedule.Create().SetInstallationID(id).SetKey(key).Exec(ctx); err != nil {
			return err
		}
	}
	for _, row := range byKey {
		if _, err := client.PluginScheduleRun.Delete().Where(pluginschedulerun.ScheduleIDEQ(row.ID)).Exec(ctx); err != nil {
			return err
		}
		if err := client.PluginSchedule.DeleteOneID(row.ID).Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

func effectiveSchedule(spec objects.PluginScheduleSpec, row *ent.PluginSchedule) (string, string) {
	cron, timezone := spec.Cron, scheduleTimezone(spec.Timezone)
	if row != nil && row.OverrideCron != "" {
		cron = row.OverrideCron
	}
	if row != nil && row.OverrideTimezone != "" {
		timezone = row.OverrideTimezone
	}
	return cron, timezone
}

func (m *Manager) ListTasks(ctx context.Context, installationID int) ([]TaskState, error) {
	var states []TaskState
	err := m.runInTx(ctx, func(ctx context.Context) error {
		current, err := m.oneRecord(ctx, installationID)
		if err != nil {
			return err
		}
		client := m.client(ctx)
		rows, err := client.PluginSchedule.Query().Where(pluginschedule.InstallationIDEQ(installationID)).All(ctx)
		if err != nil {
			return err
		}
		byKey := lo.KeyBy(rows, func(row *ent.PluginSchedule) string { return row.Key })
		states = make([]TaskState, 0, len(current.Manifest.Tasks))
		now := time.Now()
		for _, key := range slices.Sorted(maps.Keys(current.Manifest.Tasks)) {
			row := byKey[key]
			state := TaskState{Key: key, Runs: []*ent.PluginScheduleRun{}}
			state.Cron, state.Timezone = effectiveSchedule(current.Manifest.Tasks[key].Schedule, row)
			if row != nil {
				state.Paused = row.Paused
				state.Overridden = row.OverrideCron != "" || row.OverrideTimezone != ""
				state.Runs, err = client.PluginScheduleRun.Query().
					Where(pluginschedulerun.ScheduleIDEQ(row.ID)).
					Order(pluginschedulerun.ByID(entsql.OrderDesc())).Limit(scheduleHistory).All(ctx)
				if err != nil {
					return err
				}
				for _, run := range state.Runs {
					if run.Status == runStatusRunning && (row.ActiveRunID != run.ID || row.LeaseUntil <= now.Unix()) {
						run.Status = runStatusUnknown
					}
				}
			}
			if current.Enabled && !state.Paused {
				expr, loc, err := parsePluginSchedule(state.Cron, state.Timezone)
				if err != nil {
					return err
				}
				if next := expr.Next(now.In(loc)); !next.IsZero() {
					state.NextRunAt = lo.ToPtr(next.UTC())
				}
			}
			states = append(states, state)
		}
		return nil
	})
	return states, err
}

func (m *Manager) UpdateTask(ctx context.Context, installationID int, key string, paused bool, cron, timezone *string) (TaskState, error) {
	err := m.runInTx(ctx, func(ctx context.Context) error {
		current, err := m.oneRecord(ctx, installationID)
		if err != nil {
			return err
		}
		spec, ok := current.Manifest.Tasks[key]
		if !ok {
			return errors.New("plugin task not found")
		}
		client := m.client(ctx)
		row, err := client.PluginSchedule.Query().Where(
			pluginschedule.InstallationIDEQ(installationID), pluginschedule.KeyEQ(key),
		).Only(ctx)
		if err != nil {
			return err
		}
		if cron != nil {
			row.OverrideCron = *cron
		}
		if timezone != nil {
			row.OverrideTimezone = *timezone
		}
		effectiveCron, effectiveZone := effectiveSchedule(spec.Schedule, row)
		if _, _, err := parsePluginSchedule(effectiveCron, effectiveZone); err != nil {
			return err
		}
		return client.PluginSchedule.UpdateOneID(row.ID).SetPaused(paused).
			SetNillableOverrideCron(cron).SetNillableOverrideTimezone(timezone).Exec(ctx)
	})
	if err != nil {
		return TaskState{}, err
	}
	states, err := m.ListTasks(ctx, installationID)
	if err != nil {
		return TaskState{}, err
	}
	for _, state := range states {
		if state.Key == key {
			return state, nil
		}
	}
	return TaskState{}, errors.New("plugin task not found")
}

func activeTaskPlugin(gen *Generation, id int) *LoadedPlugin {
	if gen != nil {
		if loaded := gen.ByID(id); loaded != nil && loaded.Enabled && loaded.Status == RuntimeStatusActive {
			return loaded
		}
	}
	return nil
}

func (m *Manager) scanTasks(ctx context.Context) {
	if !m.scanningTasks.CompareAndSwap(false, true) {
		return
	}
	defer m.scanningTasks.Store(false)
	gen := m.Current()
	if gen == nil {
		return
	}
	now := time.Now().UTC()
	slot := now.Truncate(time.Minute)
	ids := make([]int, 0)
	for _, loaded := range gen.Plugins() {
		if activeTaskPlugin(gen, loaded.ID) != nil && len(loaded.Manifest.Tasks) > 0 {
			ids = append(ids, loaded.ID)
		}
	}
	if len(ids) == 0 {
		return
	}
	var rows []*ent.PluginSchedule
	err := m.runInTx(ctx, func(ctx context.Context) error {
		var err error
		rows, err = m.client(ctx).PluginSchedule.Query().Where(
			pluginschedule.InstallationIDIn(ids...), pluginschedule.PausedEQ(false),
			pluginschedule.LastSlotLT(slot.Unix()),
		).Order(pluginschedule.ByID()).All(ctx)
		return err
	})
	if err != nil {
		log.Warn(ctx, "scan plugin tasks", log.Cause(err))
		return
	}
	var workers errgroup.Group
	workers.SetLimit(scheduleWorkers)
	for _, row := range rows {
		loaded := gen.ByID(row.InstallationID)
		spec, ok := loaded.Manifest.Tasks[row.Key]
		if !ok || !scheduleMatches(spec.Schedule, row, slot) {
			continue
		}
		workers.Go(func() error {
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Error(ctx, "plugin task worker panicked", log.Any("panic", recovered), log.Stack("stack"))
				}
			}()
			if _, err := m.executeTask(ctx, row.InstallationID, row.Key, &slot); err != nil && !errors.Is(err, errTaskBusy) {
				log.Warn(ctx, "run plugin task", log.String("key", row.Key), log.Int("installation_id", row.InstallationID), log.Cause(err))
			}
			return nil
		})
	}
	_ = workers.Wait()
}

func scheduleMatches(spec objects.PluginScheduleSpec, row *ent.PluginSchedule, slot time.Time) bool {
	cron, timezone := effectiveSchedule(spec, row)
	expr, loc, err := parsePluginSchedule(cron, timezone)
	return err == nil && expr.Next(slot.Add(-time.Minute).In(loc)).Equal(slot)
}

func (m *Manager) RunTask(ctx context.Context, installationID int, key string) (*ent.PluginScheduleRun, error) {
	return m.executeTask(ctx, installationID, key, nil)
}

func (m *Manager) executeTask(ctx context.Context, installationID int, key string, slot *time.Time) (*ent.PluginScheduleRun, error) {
	select {
	case m.taskSlots <- struct{}{}:
		defer func() { <-m.taskSlots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	gen, err := acquireGeneration(ctx, m.current, nil)
	if err != nil {
		return nil, err
	}
	if gen == nil {
		return nil, errors.New("plugin is not active")
	}
	defer gen.Release()

	loaded := activeTaskPlugin(gen, installationID)
	if loaded == nil {
		return nil, errors.New("plugin is not active")
	}
	if _, declared := loaded.Manifest.Tasks[key]; !declared {
		return nil, errors.New("plugin task not found")
	}
	now := time.Now().UTC()
	if slot != nil && !now.Truncate(time.Minute).Equal(*slot) {
		return nil, nil
	}
	run, err := m.claimTask(ctx, installationID, key, slot, now)
	if err != nil || run == nil {
		return run, err
	}
	runCtx, cancel := context.WithTimeout(ctx, scheduleTimeout)
	defer cancel()
	if !m.trackTask(run.ID, pluginTaskRun{key: key, installation: installationID, cancel: cancel}) {
		cancel()
	}
	defer func() {
		m.taskMu.Lock()
		delete(m.taskRuns, run.ID)
		m.taskMu.Unlock()
	}()

	trigger := "scheduled"
	if run.ScheduledAt == nil {
		trigger = "manual"
	}
	args, _ := json.Marshal([]any{struct {
		Key         string     `json:"key"`
		RunID       int        `json:"runId"`
		Trigger     string     `json:"trigger"`
		ScheduledAt *time.Time `json:"scheduledAt"`
	}{key, run.ID, trigger, run.ScheduledAt}})

	_, invokeErr := m.Invoke(runCtx, InvokeRequest{
		InstallationID: loaded.ID, Export: pluginTaskExport, Args: args, Config: loaded.Config,
		Generation: gen, BorrowWait: time.Second, backgroundTask: true,
	})
	status := "succeeded"
	switch {
	case errors.Is(runCtx.Err(), context.DeadlineExceeded):
		status = "timed_out"
	case errors.Is(runCtx.Err(), context.Canceled):
		status = "canceled"
	case invokeErr != nil:
		status = "failed"
	}
	if invokeErr != nil {
		log.Warn(ctx, "plugin task invocation failed", log.String("plugin_id", loaded.PluginID),
			log.Int("installation_id", loaded.ID), log.String("key", key),
			log.Int("run_id", run.ID), log.String("status", status))
	}
	finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer finishCancel()
	finished := time.Now().UTC()
	err = m.finishTask(finishCtx, run, status, finished)
	run.Status, run.FinishedAt = status, &finished
	return run, err
}

func (m *Manager) claimTask(ctx context.Context, installationID int, key string, slot *time.Time, now time.Time) (*ent.PluginScheduleRun, error) {
	var run *ent.PluginScheduleRun
	err := m.runInTx(ctx, func(ctx context.Context) error {
		current, err := m.oneRecord(ctx, installationID)
		if err != nil {
			return err
		}
		if !current.Enabled {
			return ErrGenerationClosed
		}
		spec, ok := current.Manifest.Tasks[key]
		if !ok {
			return errors.New("plugin task not found")
		}
		client := m.client(ctx)
		row, err := client.PluginSchedule.Query().Where(
			pluginschedule.InstallationIDEQ(installationID), pluginschedule.KeyEQ(key),
		).Only(ctx)
		if err != nil {
			return err
		}
		if slot != nil && (row.Paused || row.LastSlot >= slot.Unix() || !scheduleMatches(spec.Schedule, row, *slot)) {
			return nil
		}
		claim := client.PluginSchedule.Update().Where(
			pluginschedule.IDEQ(row.ID), pluginschedule.LeaseUntilEQ(row.LeaseUntil),
			pluginschedule.ActiveRunIDEQ(row.ActiveRunID), pluginschedule.LastSlotEQ(row.LastSlot),
			pluginschedule.OverrideCronEQ(row.OverrideCron), pluginschedule.OverrideTimezoneEQ(row.OverrideTimezone),
			pluginschedule.PausedEQ(row.Paused),
		)
		if slot != nil {
			claim.SetLastSlot(slot.Unix())
		}
		if row.LeaseUntil > now.Unix() {
			if slot == nil {
				return errTaskBusy
			}
			// Consume the tick while another run holds the lease.
			_, err := claim.Save(ctx)
			return err
		}
		updated, err := claim.SetLeaseUntil(now.Add(scheduleLease).Unix()).Save(ctx)
		if err != nil {
			return err
		}
		if updated != 1 {
			return errTaskBusy
		}
		if row.ActiveRunID != 0 {
			if _, err := client.PluginScheduleRun.Update().Where(
				pluginschedulerun.IDEQ(row.ActiveRunID), pluginschedulerun.StatusEQ(runStatusRunning),
			).SetStatus(runStatusUnknown).SetFinishedAt(now).Save(ctx); err != nil {
				return err
			}
		}
		run, err = client.PluginScheduleRun.Create().SetScheduleID(row.ID).
			SetNillableScheduledAt(slot).SetStartedAt(now).SetStatus(runStatusRunning).Save(ctx)
		if err != nil {
			return err
		}
		return client.PluginSchedule.UpdateOneID(row.ID).SetActiveRunID(run.ID).Exec(ctx)
	})
	if err != nil {
		return nil, err
	}
	return run, nil
}

func (m *Manager) finishTask(ctx context.Context, run *ent.PluginScheduleRun, status string, finished time.Time) error {
	return m.runInTx(ctx, func(ctx context.Context) error {
		client := m.client(ctx)
		released, err := client.PluginSchedule.Update().Where(
			pluginschedule.IDEQ(run.ScheduleID), pluginschedule.ActiveRunIDEQ(run.ID),
		).SetActiveRunID(0).SetLeaseUntil(0).Save(ctx)
		if err != nil || released == 0 {
			return err
		}
		if err := client.PluginScheduleRun.UpdateOneID(run.ID).SetStatus(status).SetFinishedAt(finished).Exec(ctx); err != nil {
			return err
		}
		keep, err := client.PluginScheduleRun.Query().Where(pluginschedulerun.ScheduleIDEQ(run.ScheduleID)).
			Order(pluginschedulerun.ByID(entsql.OrderDesc())).Limit(scheduleHistory).IDs(ctx)
		if err != nil {
			return err
		}
		_, err = client.PluginScheduleRun.Delete().Where(
			pluginschedulerun.ScheduleIDEQ(run.ScheduleID), pluginschedulerun.IDNotIn(keep...),
		).Exec(ctx)
		return err
	})
}

func (m *Manager) trackTask(id int, run pluginTaskRun) bool {
	m.taskMu.Lock()
	defer m.taskMu.Unlock()
	if m.tasksStopped || !run.matches(m.Current()) {
		return false
	}
	m.taskRuns[id] = run
	return true
}

func (run pluginTaskRun) matches(gen *Generation) bool {
	loaded := activeTaskPlugin(gen, run.installation)
	if loaded == nil {
		return false
	}
	_, exists := loaded.Manifest.Tasks[run.key]
	return exists
}

func (m *Manager) cancelTasks(gen *Generation, stopping bool) {
	m.taskMu.Lock()
	defer m.taskMu.Unlock()
	m.tasksStopped = m.tasksStopped || stopping
	for _, run := range m.taskRuns {
		if stopping || !run.matches(gen) {
			run.cancel()
		}
	}
}
