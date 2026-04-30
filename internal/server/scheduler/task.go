package scheduler

import (
	"context"
	"sync"
	"time"
)

// task is the internal runtime representation of a scheduled task.
// mu guards lastRunAt and lastError, which are written by the executor
// goroutine (in schedule's wrapped closure) and read by List().
type task struct {
	mu sync.Mutex

	spec       TaskSpec
	cancelFunc context.CancelFunc
	lastRunAt  time.Time
	lastError  string
	fn         func(ctx context.Context)
}

// TaskSpec defines the static configuration of a scheduled task.
// Either CronExpr or FixRate must be set. If both are set, FixRate takes
// precedence.
type TaskSpec struct {
	Name        string        // unique identifier, e.g. "backup", "gc", "channel-probe"
	Description string        // human-readable description
	CronExpr    string        // cron expression, e.g. "0 2 * * *"; used when FixRate == 0
	FixRate     time.Duration // fixed-rate interval; used when CronExpr is empty
	Timezone    string        // IANA timezone name; empty defaults to UTC (cron only)
}

// TaskInfo represents the current runtime state of a task.
type TaskInfo struct {
	Spec      TaskSpec
	LastRunAt time.Time
	LastError string
}
