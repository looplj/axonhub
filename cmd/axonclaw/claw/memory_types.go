package claw

import (
	"encoding/json"
	"time"

	"github.com/looplj/axonhub/axon/agent"
)

type MemoryLayer string

const (
	MemoryLayerShort  MemoryLayer = "short"
	MemoryLayerMedium MemoryLayer = "medium"
	MemoryLayerLong   MemoryLayer = "long"
)

type MemoryEntry struct {
	ID        string      `json:"id"`
	Layer     MemoryLayer `json:"layer"`
	Content   string      `json:"content"`
	CreatedAt time.Time   `json:"created_at"`
	ExpiresAt *time.Time  `json:"expires_at,omitempty"`
	Metadata  MemoryMeta  `json:"metadata"`
}

type MemoryMeta struct {
	Type         string             `json:"type,omitempty"`
	Importance   int                `json:"importance,omitempty"`
	RoundRange   [2]int             `json:"round_range,omitempty"`
	TokenCount   int                `json:"token_count,omitempty"`
	SourceIDs    []string           `json:"source_ids,omitempty"`
	Entities     []EntityInfo       `json:"entities,omitempty"`
	Decisions    []DecisionInfo     `json:"decisions,omitempty"`
	FileChanges  []FileChangeInfo   `json:"file_changes,omitempty"`
	UserPrefs    []UserPrefInfo     `json:"user_prefs,omitempty"`
	TaskProgress []TaskProgressInfo `json:"task_progress,omitempty"`
}

type EntityInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Desc string `json:"desc,omitempty"`
}

type DecisionInfo struct {
	Topic    string `json:"topic"`
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
	Round    int    `json:"round"`
}

type FileChangeInfo struct {
	Path      string `json:"path"`
	Operation string `json:"operation"`
	Summary   string `json:"summary,omitempty"`
	Round     int    `json:"round"`
}

type UserPrefInfo struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Round int    `json:"round"`
}

type TaskProgressInfo struct {
	TaskID    string `json:"task_id"`
	Status    string `json:"status"`
	Progress  string `json:"progress,omitempty"`
	UpdatedAt int    `json:"updated_at"`
}

func (m *MemoryEntry) IsExpired() bool {
	if m.ExpiresAt == nil {
		return false
	}

	return time.Now().After(*m.ExpiresAt)
}

func (m *MemoryEntry) ToMessage() agent.Message {
	return agent.Message{
		Role:    agent.RoleUser,
		Content: &agent.Content{Text: &m.Content},
	}
}

type MemoryState struct {
	ShortTerm  []MemoryEntry `json:"short_term"`
	MediumTerm []MemoryEntry `json:"medium_term"`
	LongTerm   []MemoryEntry `json:"long_term"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

func newMemoryState() MemoryState {
	return MemoryState{
		ShortTerm:  make([]MemoryEntry, 0),
		MediumTerm: make([]MemoryEntry, 0),
		LongTerm:   make([]MemoryEntry, 0),
		UpdatedAt:  time.Now().UTC(),
	}
}

func (s *MemoryState) Clone() MemoryState {
	return MemoryState{
		ShortTerm:  cloneMemoryEntries(s.ShortTerm),
		MediumTerm: cloneMemoryEntries(s.MediumTerm),
		LongTerm:   cloneMemoryEntries(s.LongTerm),
		UpdatedAt:  s.UpdatedAt,
	}
}

func cloneMemoryEntries(in []MemoryEntry) []MemoryEntry {
	if in == nil {
		return nil
	}

	out := make([]MemoryEntry, len(in))
	copy(out, in)

	return out
}

func (s *MemoryState) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

func MemoryStateFromJSON(data []byte) (MemoryState, error) {
	var state MemoryState
	if err := json.Unmarshal(data, &state); err != nil {
		return state, err
	}

	return state, nil
}
