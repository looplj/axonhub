package task

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type Handler interface {
	HandleTask(ctx context.Context, t Task) error
}

type Scheduler struct {
	store   *Store
	handler Handler
	logger  *slog.Logger

	tickInterval time.Duration
	nowFunc      func() time.Time

	mu      sync.Mutex
	running map[string]struct{}

	cancel context.CancelFunc
}

type SchedulerOptions struct {
	TickInterval time.Duration
}

func NewScheduler(logger *slog.Logger, store *Store, handler Handler, opts SchedulerOptions) (*Scheduler, error) {
	if store == nil {
		return nil, fmt.Errorf("task store is required")
	}
	if handler == nil {
		return nil, fmt.Errorf("task handler is required")
	}

	tick := opts.TickInterval
	if tick <= 0 {
		tick = 1 * time.Minute
	}

	return &Scheduler{
		store:        store,
		handler:      handler,
		logger:       logger,
		tickInterval: tick,
		nowFunc:      time.Now,
		running:      map[string]struct{}{},
	}, nil
}

func (s *Scheduler) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	go s.run(ctx)
}

func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *Scheduler) run(ctx context.Context) {
	ticker := time.NewTicker(s.tickInterval)
	defer ticker.Stop()

	if err := s.tick(ctx); err != nil {
		s.logger.Error("scheduler tick failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.tick(ctx); err != nil {
				s.logger.Error("scheduler tick failed", "error", err)
			}
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) error {
	now := s.nowFunc()
	var toRun []Task

	err := s.store.Update(func(tasks []Task) ([]Task, bool, error) {
		changed := false
		for i := range tasks {
			t := &tasks[i]
			if !t.Enabled {
				continue
			}

			var next time.Time
			next, ok := t.Runtime.NextTime()
			if !ok {
				var err error
				next, err = nextRunTime(t.Trigger, now, true)
				if err != nil {
					t.Runtime.LastStatus = "failed"
					t.Runtime.LastError = err.Error()
					if t.Trigger.Type == TriggerTypeAt || t.Trigger.Type == TriggerTypeDelay {
						t.Enabled = false
						t.Runtime.Running = false
						t.Runtime.NextRunAt = ""
					}
					changed = true
					continue
				}
				t.Runtime.NextRunAt = next.Format(time.RFC3339)
				changed = true
				if next.After(now) {
					continue
				}
			}

			if next.After(now) {
				continue
			}
			if s.isRunning(t.ID) {
				t.Runtime.LastStatus = "skipped"
				t.Runtime.LastError = "task is already running"
				changed = true
				continue
			}

			nextNext, err := nextRunTime(t.Trigger, next, false)
			if err != nil {
				t.Runtime.LastStatus = "failed"
				t.Runtime.LastError = err.Error()
				changed = true
				continue
			}
			for !nextNext.IsZero() && !nextNext.After(now) {
				nextNext, err = nextRunTime(t.Trigger, nextNext, false)
				if err != nil {
					t.Runtime.LastStatus = "failed"
					t.Runtime.LastError = err.Error()
					changed = true
					break
				}
			}
			if err != nil {
				continue
			}

			t.Runtime.Running = true
			t.Runtime.LastStatus = "running"
			t.Runtime.LastError = ""
			if nextNext.IsZero() {
				t.Runtime.NextRunAt = ""
			} else {
				t.Runtime.NextRunAt = nextNext.Format(time.RFC3339)
			}
			changed = true

			taskSnapshot := *t
			toRun = append(toRun, taskSnapshot)
		}
		return tasks, changed, nil
	})
	if err != nil {
		return err
	}

	for i := range toRun {
		s.markRunning(toRun[i].ID, true)
		go s.executeTask(ctx, toRun[i])
	}
	return nil
}

func (s *Scheduler) executeTask(ctx context.Context, t Task) {
	defer s.markRunning(t.ID, false)

	current, err := s.store.Get(t.ID)
	if err != nil {
		s.logger.Warn("task not found in store, skipping execution", "task", t.ID, "error", err)
		return
	}

	if !current.Enabled {
		s.logger.Info("task is disabled, skipping execution", "task", t.ID)
		return
	}

	err = s.handler.HandleTask(ctx, t)
	now := s.nowFunc().Format(time.RFC3339)

	isOneShot := t.Trigger.Type == TriggerTypeAt || t.Trigger.Type == TriggerTypeDelay

	if isOneShot && err == nil {
		if moveErr := s.store.MoveToDeleted(t.ID, "completed"); moveErr == nil {
			return
		} else {
			s.logger.Error("failed to move completed task to deleted", "task", t.ID, "error", moveErr)
		}
	}

	if updateErr := s.store.Update(func(tasks []Task) ([]Task, bool, error) {
		for i := range tasks {
			if tasks[i].ID != t.ID {
				continue
			}
			tasks[i].Runtime.Running = false
			tasks[i].Runtime.LastRunAt = now
			if err != nil {
				tasks[i].Runtime.LastStatus = "failed"
				tasks[i].Runtime.LastError = err.Error()
			} else {
				tasks[i].Runtime.LastStatus = "success"
				tasks[i].Runtime.LastError = ""
				if isOneShot {
					tasks[i].Enabled = false
				}
			}
			return tasks, true, nil
		}
		return tasks, false, nil
	}); updateErr != nil {
		s.logger.Error("failed to update task status after execution", "task", t.ID, "error", updateErr)
	}
}

func (s *Scheduler) isRunning(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.running[id]
	return ok
}

func (s *Scheduler) markRunning(id string, running bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if running {
		s.running[id] = struct{}{}
		return
	}
	delete(s.running, id)
}

func nextRunTime(trigger Trigger, from time.Time, alignFromNow bool) (time.Time, error) {
	loc, err := parseLocation(trigger.Timezone)
	if err != nil {
		return time.Time{}, err
	}
	ref := from.In(loc)

	switch trigger.Type {
	case TriggerTypeCron:
		if trigger.Cron == "" {
			return time.Time{}, fmt.Errorf("cron trigger requires cron")
		}
		p := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		sched, err := p.Parse(trigger.Cron)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid cron: %w", err)
		}
		if alignFromNow {
			return sched.Next(ref).UTC(), nil
		}
		return sched.Next(ref).UTC(), nil
	case TriggerTypeInterval:
		if trigger.Interval == "" {
			return time.Time{}, fmt.Errorf("interval trigger requires interval")
		}
		d, err := time.ParseDuration(trigger.Interval)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid interval: %w", err)
		}
		if d <= 0 {
			return time.Time{}, fmt.Errorf("interval must be greater than 0")
		}
		return ref.Add(d).UTC(), nil
	case TriggerTypeAt:
		if trigger.At == "" {
			return time.Time{}, fmt.Errorf("at trigger requires at")
		}
		at, err := time.Parse(time.RFC3339, trigger.At)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid at: %w", err)
		}
		if alignFromNow && !at.After(ref) {
			return time.Time{}, fmt.Errorf("at time has already passed")
		}
		if !alignFromNow {
			return time.Time{}, nil
		}
		return at.UTC(), nil
	case TriggerTypeDelay:
		if trigger.Delay == "" {
			return time.Time{}, fmt.Errorf("delay trigger requires delay")
		}
		d, err := time.ParseDuration(trigger.Delay)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid delay: %w", err)
		}
		if d <= 0 {
			return time.Time{}, fmt.Errorf("delay must be greater than 0")
		}
		if !alignFromNow {
			return time.Time{}, nil
		}
		return ref.Add(d).UTC(), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported trigger type %q", trigger.Type)
	}
}

func parseLocation(name string) (*time.Location, error) {
	if name == "" {
		return time.Local, nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone %q: %w", name, err)
	}
	return loc, nil
}
