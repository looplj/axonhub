package pelican

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/scheduler"
)

// defaultDataDir keeps pelican data next to the AxonHub database file.
const defaultDataDir = "pelican-data"

// dataDirEnv overrides the data directory without touching AxonHub configuration.
const dataDirEnv = "AXONHUB_PELICAN_DATA_DIR"

// NewStore builds the file-backed store.
func NewStore() *Store {
	if dir := strings.TrimSpace(os.Getenv(dataDirEnv)); dir != "" {
		return NewStoreAt(dir)
	}
	return NewStoreAt(defaultDataDir)
}

// Status is the small summary the UI polls.
type Status struct {
	Running   bool     `json:"running"`
	Schedule  Schedule `json:"schedule"`
	NextRunAt string   `json:"nextRunAt"`
	DataDir   string   `json:"dataDir"`
	Efforts   []Effort `json:"efforts"`
}

// Service owns the module state: configuration, the runner and the hourly task.
type Service struct {
	store  *Store
	runner *Runner
	now    func() time.Time

	mu sync.Mutex
}

// newService is the seam used by tests; production wiring goes through NewService.
func newService(store *Store, runner *Runner, now func() time.Time) *Service {
	return &Service{store: store, runner: runner, now: now}
}

// NewService wires the service and registers the hourly task with AxonHub's scheduler.
func NewService(lifecycle fx.Lifecycle, store *Store, runner *Runner, tasks *scheduler.Scheduler) *Service {
	service := newService(store, runner, time.Now)

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Nothing is running yet, so attempts a previous process left behind were interrupted.
			service.failInterruptedResults(ctx)

			// The task fires every hour; whether a round actually runs is decided in runScheduled,
			// which also consumes the slot so a skipped hour is never backfilled.
			return tasks.Register(ctx, scheduler.TaskSpec{
				Name:        "pelican-round",
				Description: "Run the pelican drawing test for every configured model",
				CronExpr:    "0 * * * *",
				Timezone:    "UTC",
			}, service.runScheduled)
		},
	})
	return service
}

// failInterruptedResults closes attempts that a restart left open. The runner records an attempt as
// running before calling the model, so a restart mid-round would otherwise leave a spinner in the
// gallery that nothing can ever finish.
func (s *Service) failInterruptedResults(ctx context.Context) {
	err := s.store.Update(func(_ *Config, results *[]Result) error {
		for index := range *results {
			if (*results)[index].Status == StatusRunning {
				(*results)[index].Status = StatusFailed
				(*results)[index].Error = "the round was interrupted by a service restart"
			}
		}
		return nil
	})
	if err != nil {
		log.Error(ctx, "pelican could not close interrupted results", log.Cause(err))
	}
}

// Status reports whether a round is running plus the schedule bookkeeping.
func (s *Service) Status() (Status, error) {
	config, _, err := s.store.Load()
	if err != nil {
		return Status{}, err
	}
	return Status{
		Running:   s.runner.IsRunning(),
		Schedule:  config.Schedule,
		NextRunAt: config.Schedule.NextRunAt,
		DataDir:   s.store.Dir(),
		Efforts:   SupportedEfforts(),
	}, nil
}

// Config returns the current configuration.
func (s *Service) Config() (Config, error) {
	config, _, err := s.store.Load()
	return config, err
}

// SaveConfig stores the configuration. The project it belongs to is recorded so scheduled
// rounds can run without an HTTP request context.
func (s *Service) SaveConfig(projectID int, config Config) (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, _, err := s.store.Load()
	if err != nil {
		return Config{}, err
	}

	if config.Schedule.Enabled && !current.Schedule.Enabled {
		// Starting now means the next slot is the upcoming top of the hour.
		config.Schedule.NextRunAt = NextHour(s.now()).UTC().Format(time.RFC3339)
	}
	if !config.Schedule.Enabled {
		config.Schedule.NextRunAt = ""
	}
	if projectID > 0 {
		config.ProjectID = projectID
	}
	if err := s.store.SaveConfig(config); err != nil {
		return Config{}, err
	}
	return s.store.Config()
}

// Results returns the newest attempts first.
func (s *Service) Results() ([]Result, error) {
	_, results, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	// Newest first, matching the gallery order.
	ordered := make([]Result, 0, len(results))
	for index := len(results) - 1; index >= 0; index-- {
		ordered = append(ordered, results[index])
	}
	return ordered, nil
}

// FindResult locates one result by id.
func (s *Service) FindResult(id string) (Result, error) {
	_, results, err := s.store.Load()
	if err != nil {
		return Result{}, err
	}
	for _, result := range results {
		if result.ID == id {
			return result, nil
		}
	}
	return Result{}, ErrNotFound
}

// Artifact returns a generated document.
func (s *Service) Artifact(id, format string) (string, error) {
	return s.store.ReadArtifact(id, format)
}

// Conversation returns the stored exchange of one attempt.
func (s *Service) Conversation(id string) (Conversation, error) { return s.store.ReadConversation(id) }

// RunNow starts a round in the background and returns immediately: a round can take minutes,
// so the UI follows progress by polling Results.
func (s *Service) RunNow(ctx context.Context) error {
	// Validate before returning, otherwise the caller would be told the round started while it
	// silently failed in the background.
	config, _, err := s.store.Load()
	if err != nil {
		return err
	}
	if err := validateTargets(config.Targets); err != nil {
		return err
	}

	// The request context is canceled when the HTTP response is written, so the round gets a
	// detached context that outlives it.
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error(ctx, "pelican round panicked", log.Any("panic", recovered))
			}
		}()
		if _, err := s.runner.Run(context.WithoutCancel(ctx)); err != nil {
			log.Error(ctx, "pelican round failed", log.Cause(err))
		}
	}()
	return nil
}

// runScheduled is the hourly task body.
//
// Semantics: one round per hour at most, skipped (and recorded) while another round is still
// running, and never backfilled after downtime.
func (s *Service) runScheduled(ctx context.Context) {
	s.mu.Lock()
	config, _, err := s.store.Load()
	s.mu.Unlock()
	if err != nil {
		log.Error(ctx, "pelican schedule could not read the configuration", log.Cause(err))
		return
	}
	if !config.Schedule.Enabled {
		return
	}

	// Consume the slot before running, so a failure cannot cause an immediate retry loop.
	next := NextHour(s.now()).UTC().Format(time.RFC3339)
	if err := s.store.Update(func(current *Config, _ *[]Result) error {
		current.Schedule.NextRunAt = next
		return nil
	}); err != nil {
		log.Error(ctx, "pelican schedule could not advance the slot", log.Cause(err))
		return
	}

	if s.runner.IsRunning() {
		log.Info(ctx, "pelican schedule skipped: a round is still running")
		return
	}

	// The project only scopes project-level settings such as prompt injection, and admin requests
	// never carry one, so the round runs with the plain context when it was never recorded.
	roundCtx := ctx
	if config.ProjectID > 0 {
		roundCtx = withProject(ctx, config.ProjectID)
	}
	if _, err := s.runner.Run(roundCtx); err != nil {
		log.Error(ctx, "pelican scheduled round failed", log.Cause(err))
	}
}

// validateTargets is shared by the handler so bad input fails before touching the disk.
func validateTargets(targets []Target) error {
	if len(targets) == 0 {
		return errors.New("select at least one model")
	}
	for _, target := range targets {
		if strings.TrimSpace(target.Model) == "" {
			return errors.New("model must not be empty")
		}
		if !ValidEffort(target.Effort) {
			return fmt.Errorf("unsupported reasoning effort %q", target.Effort)
		}
	}
	return nil
}
