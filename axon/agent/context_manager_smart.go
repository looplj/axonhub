package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// ContextManagerConfig controls context compaction behavior.
type ContextManagerConfig struct {
	Enabled           bool
	MaxRecentMessages int
	SoftTokenLimit    int
	SummaryMaxChars   int
	Summarizer        Summarizer
	Logger            *slog.Logger
}

func DefaultContextManagerConfig() ContextManagerConfig {
	return ContextManagerConfig{
		Enabled:           true,
		MaxRecentMessages: defaultContextMaxRecentMessages,
		SoftTokenLimit:    defaultContextSoftTokenLimit,
		SummaryMaxChars:   defaultContextSummaryMaxChars,
	}
}

// SmartContextManager is a decorator strategy that adds compaction
// and persisted summary state.
type SmartContextManager struct {
	ContextManager

	config ContextManagerConfig
	store  ContextManagerStore
	logger *slog.Logger

	mu    sync.RWMutex
	state ContextManagerState
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

func (m *SmartContextManager) BuildMessages(ctx context.Context) []Message {
	working := cloneMessages(m.ContextManager.BuildMessages(ctx))
	var archived []Message

	if !m.config.Enabled {
		m.logger.Debug("context manager: compaction disabled",
			"messages", len(working),
		)
		return working
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	keep := m.config.MaxRecentMessages
	if keep <= 0 {
		keep = defaultContextMaxRecentMessages
	}

	totalTokens := 0
	tokenLimitExceeded := false
	if m.config.SoftTokenLimit > 0 {
		totalTokens = EstimateMessagesTokens(working)
		tokenLimitExceeded = totalTokens > m.config.SoftTokenLimit
	}
	messageOverflow := len(working) > keep
	shouldCompact := messageOverflow || (tokenLimitExceeded && len(working) > keep/2)
	if m.logger.Enabled(ctx, slog.LevelDebug) {
		m.logger.Debug("context manager: build messages",
			"messages", len(working),
			"tokens", totalTokens,
			"max_recent_messages", keep,
			"soft_token_limit", m.config.SoftTokenLimit,
			"message_overflow", messageOverflow,
			"token_limit_exceeded", tokenLimitExceeded,
			"should_compact", shouldCompact,
		)
	}

	if shouldCompact {
		if keep > len(working) {
			keep = len(working)
		}

		cut := len(working) - keep
		cut = adjustCompactionCut(working, cut)
		overflow := working[:cut]
		if len(overflow) > 0 {
			m.logger.Debug("context manager: summarize start",
				"overflow_messages", len(overflow),
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

				m.logger.Debug("context manager: summarize complete",
					"overflow_messages", len(overflow),
					"retained_messages", len(retained),
					"summary_chars", len(summary),
					"compaction_count", m.state.CompactionCount,
				)

				m.ContextManager.SetMessages(ctx, working)
				archived = overflow
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
	m.saveLocked(ctx, working, archived)

	return cloneMessages(working)
}

func (m *SmartContextManager) Snapshot() ContextManagerState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return copyContextState(m.state)
}

func (m *SmartContextManager) saveLocked(ctx context.Context, messages []Message, archivedMessages []Message) {
	if m.store == nil {
		return
	}
	if err := m.store.Save(ctx, m.state, messages, archivedMessages); err != nil {
		m.logger.Debug("context manager: save failed", "error", err)
	}
}

func adjustCompactionCut(messages []Message, cut int) int {
	if cut <= 0 || cut >= len(messages) {
		return cut
	}

	overflowReqIndexes := map[int]struct{}{}
	overflowToolUseIDs := map[string]struct{}{}
	for i := 0; i < cut; i++ {
		msg := messages[i]
		if msg.RequestIndex != 0 {
			overflowReqIndexes[msg.RequestIndex] = struct{}{}
		}
		if msg.ToolUse != nil && msg.ToolUse.ID != "" {
			overflowToolUseIDs[msg.ToolUse.ID] = struct{}{}
		}
	}

	for cut < len(messages) {
		msg := messages[cut]

		if msg.RequestIndex != 0 {
			if _, ok := overflowReqIndexes[msg.RequestIndex]; ok {
				if msg.ToolUse != nil && msg.ToolUse.ID != "" {
					overflowToolUseIDs[msg.ToolUse.ID] = struct{}{}
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
	if cfg.SummaryMaxChars <= 0 {
		cfg.SummaryMaxChars = defaultContextSummaryMaxChars
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return cfg
}
