package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const (
	defaultContextMaxRecentMessages = 120
	defaultContextSoftTokenLimit    = 160_000
)

// ContextManagerConfig controls context compaction behavior.
type ContextManagerConfig struct {
	Enabled           bool
	MaxRecentMessages int // MaxRecentMessages is the maximum number of recent rounds to keep.
	SoftTokenLimit    int
	Summarizer        Summarizer
	Logger            *slog.Logger
}

func DefaultContextManagerConfig() ContextManagerConfig {
	return ContextManagerConfig{
		Enabled:           true,
		MaxRecentMessages: defaultContextMaxRecentMessages,
		SoftTokenLimit:    defaultContextSoftTokenLimit,
	}
}

// compactionCooldown prevents back-to-back summarization calls.
// After a successful compaction, the next BuildMessages call skips
// compaction to avoid an infinite summarize loop when retained
// messages still exceed the soft token limit.
const compactionCooldown = 30 * time.Second

// SmartContextManager is a decorator strategy that adds compaction
// and persisted summary state.
type SmartContextManager struct {
	ContextManager

	config ContextManagerConfig
	store  ContextManagerStore
	logger *slog.Logger

	mu              sync.RWMutex
	state           ContextManagerState
	lastCompactedAt time.Time
	onCompaction    func()
}

func NewSmartContextManager(config ContextManagerConfig, store ContextManagerStore) (*SmartContextManager, error) {
	return NewSmartContextManagerWithNext(nil, config, store)
}

func NewSmartContextManagerWithNext(next ContextManager, config ContextManagerConfig, store ContextManagerStore) (*SmartContextManager, error) {
	cfg := mergeDefaultContextManagerConfig(config)
	if cfg.Summarizer == nil {
		return nil, fmt.Errorf("context manager summarizer is required")
	}
	if next == nil {
		next = NewSimpleContextManager(nil)
	}

	cm := &SmartContextManager{
		ContextManager: next,
		config:         cfg,
		store:          store,
		logger:         cfg.Logger,
		state:          emptyContextState(),
	}

	if store == nil {
		return cm, nil
	}

	loaded, messages, err := store.Load(context.Background())
	if err != nil {
		return nil, err
	}
	cm.state = loaded
	if len(messages) > 0 {
		cm.ContextManager.SetMessages(context.Background(), messages)
	}
	return cm, nil
}

func (m *SmartContextManager) Messages(ctx context.Context) []Message {
	messages := m.ContextManager.Messages(ctx)
	if m.logger.Enabled(ctx, slog.LevelDebug) {
		m.logger.Debug("agent: messages updated",
			"total_messages", len(messages),
			"total_tokens", EstimateMessagesTokens(messages),
		)
	}
	return messages
}

func (m *SmartContextManager) ClearMessages(ctx context.Context) {
	m.ContextManager.ClearMessages(ctx)

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	m.state = ContextManagerState{UpdatedAt: now}
	m.lastCompactedAt = time.Time{}
	m.saveLocked(ctx, nil)
}

func (m *SmartContextManager) BuildMessages(ctx context.Context) []Message {
	working := cloneMessages(m.ContextManager.BuildMessages(ctx))

	if !m.config.Enabled {
		m.logger.Debug("context manager: compaction disabled",
			"messages", len(working),
		)
		return working
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	keepRounds := m.config.MaxRecentMessages
	if keepRounds <= 0 {
		keepRounds = defaultContextMaxRecentMessages
	}

	totalTokens := 0
	tokenLimitExceeded := false
	if m.config.SoftTokenLimit > 0 {
		totalTokens = EstimateMessagesTokens(working)
		tokenLimitExceeded = totalTokens > m.config.SoftTokenLimit
	}

	totalRounds := countUniqueRounds(working)
	roundOverflow := totalRounds > keepRounds
	inCooldown := !m.lastCompactedAt.IsZero() && time.Since(m.lastCompactedAt) < compactionCooldown

	shouldCompact := (roundOverflow || (tokenLimitExceeded && totalRounds > keepRounds/2)) && !inCooldown
	if m.logger.Enabled(ctx, slog.LevelDebug) {
		m.logger.Debug("context manager: build messages",
			"messages", len(working),
			"rounds", totalRounds,
			"tokens", totalTokens,
			"max_recent_rounds", keepRounds,
			"soft_token_limit", m.config.SoftTokenLimit,
			"round_overflow", roundOverflow,
			"token_limit_exceeded", tokenLimitExceeded,
			"in_cooldown", inCooldown,
			"should_compact", shouldCompact,
		)
	}

	if shouldCompact {
		cut := findCutIndexForRounds(working, keepRounds)
		cut = adjustCompactionCut(working, cut)

		overflow := cloneMessages(working[:cut])
		if len(overflow) > 0 {
			m.logger.Debug("context manager: summarize start",
				"overflow_messages", len(overflow),
				"overflow_rounds", countUniqueRounds(overflow),
				"overflow_tokens", EstimateMessagesTokens(overflow),
			)
			summary, err := m.config.Summarizer.Summarize(ctx, overflow)
			summary = strings.TrimSpace(summary)
			if err == nil && summary != "" {
				summaryMsg := Message{
					Role:    RoleUser,
					Content: &Content{Text: &summary},
				}
				retained := cloneMessages(working[cut:])
				working = append([]Message{summaryMsg}, retained...)

				now := time.Now().UTC()
				m.state.Summary = ""
				m.state.CompactionCount++
				m.state.UpdatedAt = now
				m.lastCompactedAt = now

				m.logger.Debug("context manager: summarize complete",
					"overflow_messages", len(overflow),
					"overflow_rounds", countUniqueRounds(overflow),
					"retained_messages", len(retained),
					"retained_rounds", countUniqueRounds(retained),
					"summary_chars", len(summary),
					"compaction_count", m.state.CompactionCount,
				)

				m.ContextManager.SetMessages(ctx, working)

				if m.onCompaction != nil {
					m.onCompaction()
				}
			} else {
				m.logger.Debug("context manager: summarize skipped",
					"error", err,
					"summary_empty", summary == "",
					"overflow_messages", len(overflow),
				)
			}
		} else {
			m.logger.Debug("context manager: summarize skipped",
				"reason", "empty_overflow_after_adjust",
			)
		}
	}

	m.saveLocked(ctx, working)

	return cloneMessages(working)
}

func (m *SmartContextManager) Snapshot() ContextManagerState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return copyContextState(m.state)
}

func (m *SmartContextManager) OnCompaction(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.onCompaction = fn
}

func (m *SmartContextManager) saveLocked(ctx context.Context, messages []Message) {
	if m.store == nil {
		return
	}
	// Derive max RoundIndex from current messages so it survives restarts.
	var maxRI int64
	for i := range messages {
		if ri := int64(messages[i].RoundIndex); ri > maxRI {
			maxRI = ri
		}
	}

	m.state.RoundIndex = maxRI
	if err := m.store.Save(ctx, m.state, messages); err != nil {
		m.logger.Debug("context manager: save failed", "error", err)
	}
}

func adjustCompactionCut(messages []Message, cut int) int {
	if cut <= 0 || cut >= len(messages) {
		return cut
	}

	overflowRoundIndexes := map[int]struct{}{}
	overflowToolUseIDs := map[string]struct{}{}
	for i := 0; i < cut; i++ {
		msg := messages[i]
		if msg.RoundIndex != 0 {
			overflowRoundIndexes[msg.RoundIndex] = struct{}{}
		}

		if msg.ToolCall != nil && msg.ToolCall.ID != "" {
			overflowToolUseIDs[msg.ToolCall.ID] = struct{}{}
		}
	}

	for cut < len(messages) {
		msg := messages[cut]

		if msg.RoundIndex != 0 {
			if _, ok := overflowRoundIndexes[msg.RoundIndex]; ok {
				if msg.ToolCall != nil && msg.ToolCall.ID != "" {
					overflowToolUseIDs[msg.ToolCall.ID] = struct{}{}
				}
				cut++
				continue
			}
		}

		if msg.Role == RoleTool && msg.ToolUseID != nil {
			if _, ok := overflowToolUseIDs[*msg.ToolUseID]; ok {
				cut++
				continue
			}
		}

		break
	}

	return cut
}

func mergeDefaultContextManagerConfig(cfg ContextManagerConfig) ContextManagerConfig {
	if cfg.MaxRecentMessages <= 0 {
		cfg.MaxRecentMessages = defaultContextMaxRecentMessages
	}
	if cfg.SoftTokenLimit <= 0 {
		cfg.SoftTokenLimit = defaultContextSoftTokenLimit
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return cfg
}
