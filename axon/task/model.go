package task

import "time"

type TriggerType string

const (
	TriggerTypeCron     TriggerType = "cron"
	TriggerTypeInterval TriggerType = "interval"
	TriggerTypeAt       TriggerType = "at"
	TriggerTypeDelay    TriggerType = "delay"
)

type Task struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Enabled bool           `json:"enabled"`
	Trigger Trigger        `json:"trigger"`
	Action  map[string]any `json:"action"`
	Runtime TaskRuntime    `json:"runtime"`
}

type Trigger struct {
	Type     TriggerType `json:"type"`
	Cron     string      `json:"cron,omitempty"`
	Interval string      `json:"interval,omitempty"`
	At       string      `json:"at,omitempty"`
	Delay    string      `json:"delay,omitempty"`
	Timezone string      `json:"timezone,omitempty"`
}

type TaskRuntime struct {
	NextRunAt  string `json:"next_run_at,omitempty"`
	LastRunAt  string `json:"last_run_at,omitempty"`
	LastStatus string `json:"last_status,omitempty"`
	LastError  string `json:"last_error,omitempty"`
	Running    bool   `json:"running,omitempty"`
}

func (r TaskRuntime) NextTime() (time.Time, bool) {
	if r.NextRunAt == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, r.NextRunAt)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
