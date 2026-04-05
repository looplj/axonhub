package conf

import (
	"time"

	"github.com/samber/lo"
)

const TopicReloadEvent = "conf.reload"

type ReloadSource string

const (
	ReloadSourceManual ReloadSource = "manual"
	ReloadSourceFile   ReloadSource = "file"
)

type ReloadEventType string

const (
	ReloadEventRequested ReloadEventType = "reload_requested"
	ReloadEventStarted   ReloadEventType = "reload_started"
	ReloadEventNoop      ReloadEventType = "reload_noop"
	ReloadEventFailed    ReloadEventType = "reload_failed"
	ReloadEventApplied   ReloadEventType = "reload_applied"
)

type ChangeSet struct {
	ChangedKeys   []string `json:"changed_keys,omitempty"`
	SensitiveKeys []string `json:"sensitive_keys,omitempty"`
}

func (s ChangeSet) IsAnyKeyChanged(keys ...string) bool {
	if len(keys) == 0 {
		return len(s.ChangedKeys) > 0
	}
	return len(lo.Intersect(keys, s.ChangedKeys)) > 0
}

type ReloadEvent struct {
	RequestID      string            `json:"request_id"`
	Type           ReloadEventType   `json:"type"`
	Source         ReloadSource      `json:"source"`
	ConfigFile     string            `json:"config_file,omitempty"`
	OldVersion     uint64            `json:"old_version,omitempty"`
	NewVersion     uint64            `json:"new_version,omitempty"`
	ChangedKeys    []string          `json:"changed_keys,omitempty"`
	SensitiveKeys  []string          `json:"sensitive_keys,omitempty"`
	EffectiveAt    string            `json:"effective_at,omitempty"`
	Attributes     map[string]string `json:"attributes,omitempty"`
	Error          string            `json:"error,omitempty"`
	NeedsRebuild   bool              `json:"needs_rebuild,omitempty"`
	RequestedAt    time.Time         `json:"requested_at"`
	CompletedAt    time.Time         `json:"completed_at,omitempty"`
	ProcessingTime time.Duration     `json:"processing_time,omitempty"`
}
