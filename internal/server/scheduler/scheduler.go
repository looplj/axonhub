package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zhenzou/executors"

	"github.com/looplj/axonhub/internal/log"
)

type Scheduler struct {
	executor executors.ScheduledExecutor
	tasks    map[string]*task
	mu       sync.RWMutex
}

func New(exec executors.ScheduledExecutor) *Scheduler {
	return &Scheduler{
		executor: exec,
		tasks:    make(map[string]*task),
	}
}

// Register schedules a task and starts its cron. Returns an error if a task
// with the same name already exists.
func (s *Scheduler) Register(ctx context.Context, spec TaskSpec, fn func(ctx context.Context)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[spec.Name]; exists {
		return fmt.Errorf("task %q already registered", spec.Name)
	}

	t := &task{spec: spec, fn: fn}
	if err := s.schedule(ctx, t); err != nil {
		return fmt.Errorf("register task %q: %w", spec.Name, err)
	}

	s.tasks[spec.Name] = t

	if spec.FixRate > 0 {
		log.Info(ctx, "scheduler: registered task",
			log.String("name", spec.Name),
			log.Duration("fix_rate", spec.FixRate),
		)
	} else {
		log.Info(ctx, "scheduler: registered task",
			log.String("name", spec.Name),
			log.String("cron", spec.CronExpr),
			log.String("timezone", spec.Timezone),
		)
	}

	return nil
}

// Reschedule cancels the existing cron for the named task and re-creates it
// with the new spec. Use this when configuration changes (timezone, cron expr).
func (s *Scheduler) Reschedule(ctx context.Context, name string, newSpec TaskSpec) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, exists := s.tasks[name]
	if !exists {
		return fmt.Errorf("task %q not found", name)
	}

	if t.cancelFunc != nil {
		t.cancelFunc()
		t.cancelFunc = nil
	}

	t.spec = newSpec
	if err := s.schedule(ctx, t); err != nil {
		return fmt.Errorf("reschedule task %q: %w", name, err)
	}

	if newSpec.FixRate > 0 {
		log.Info(ctx, "scheduler: rescheduled task",
			log.String("name", name),
			log.Duration("fix_rate", newSpec.FixRate),
		)
	} else {
		log.Info(ctx, "scheduler: rescheduled task",
			log.String("name", name),
			log.String("cron", newSpec.CronExpr),
			log.String("timezone", newSpec.Timezone),
		)
	}

	return nil
}

// Unregister cancels and removes a task by name. No-op if not found.
func (s *Scheduler) Unregister(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, exists := s.tasks[name]
	if !exists {
		return
	}

	if t.cancelFunc != nil {
		t.cancelFunc()
	}

	delete(s.tasks, name)
}

// List returns a snapshot of all registered tasks and their runtime state.
func (s *Scheduler) List() []TaskInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]TaskInfo, 0, len(s.tasks))
	for _, t := range s.tasks {
		t.mu.Lock()
		result = append(result, TaskInfo{
			Spec:      t.spec,
			LastRunAt: t.lastRunAt,
			LastError: t.lastError,
		})
		t.mu.Unlock()
	}

	return result
}

// Shutdown cancels all running tasks. The executor is not shut down — the
// caller (fx lifecycle) owns that.
func (s *Scheduler) Shutdown(_ context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, t := range s.tasks {
		if t.cancelFunc != nil {
			t.cancelFunc()
		}
	}
}

func (s *Scheduler) schedule(ctx context.Context, t *task) error {
	wrapped := func(ctx context.Context) {
		t.mu.Lock()
		t.lastRunAt = time.Now()
		t.lastError = ""
		t.mu.Unlock()

		func() {
			defer func() {
				if r := recover(); r != nil {
					t.mu.Lock()
					t.lastError = fmt.Sprintf("panic: %v", r)
					t.mu.Unlock()
					log.Error(ctx, "scheduler: task panicked",
						log.String("name", t.spec.Name),
						log.Any("panic", r),
					)
				}
			}()

			t.fn(ctx)
		}()
	}

	var cancelFunc context.CancelFunc
	var err error

	if t.spec.FixRate > 0 {
		cancelFunc, err = s.executor.ScheduleFuncAtFixRate(wrapped, t.spec.FixRate)
	} else {
		tz := t.spec.Timezone
		if tz == "" {
			tz = "UTC"
		}
		cancelFunc, err = s.executor.ScheduleFuncAtCronRate(
			wrapped,
			executors.CRONRule{Expr: t.spec.CronExpr, Timezone: tz},
		)
	}

	if err != nil {
		return err
	}

	t.cancelFunc = cancelFunc
	return nil
}
