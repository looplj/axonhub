package claw

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/looplj/axonhub/axon/agent"
)

const (
	defaultContextTokenLimit      = 180_000
	defaultContextSummaryMaxChars = 16_000
)

type ContextManagerConfig struct {
	Enabled         bool
	TokenLimit      int
	SummaryMaxChars int
	Summarizer      Summarizer
	Logger          *slog.Logger
}

func DefaultContextManagerConfig() ContextManagerConfig {
	return ContextManagerConfig{
		Enabled:         true,
		TokenLimit:      defaultContextTokenLimit,
		SummaryMaxChars: defaultContextSummaryMaxChars,
	}
}

type SmartContextManager struct {
	agent.ContextManager

	config ContextManagerConfig
	store  ContextManagerStore
	logger *slog.Logger

	mu           sync.RWMutex
	state        agent.ContextManagerState
	onCompaction func()
}

func NewSmartContextManager(config ContextManagerConfig, store ContextManagerStore) (*SmartContextManager, error) {
	return NewSmartContextManagerWithNext(nil, config, store)
}

func NewSmartContextManagerWithNext(next agent.ContextManager, config ContextManagerConfig, store ContextManagerStore) (*SmartContextManager, error) {
	cfg := mergeDefaultContextManagerConfig(config)

	if next == nil {
		next = agent.NewSimpleContextManager(nil)
	}

	cm := &SmartContextManager{
		ContextManager: next,
		config:         cfg,
		store:          store,
		logger:         cfg.Logger,
		state:          agent.EmptyContextState(),
	}

	if store == nil {
		return cm, nil
	}

	loaded, messages, err := store.Load(context.Background())
	if err != nil {
		return nil, err
	}

	cfg.Logger.Info("context manager: store loaded",
		"loaded_messages", len(messages),
		"loaded_tokens", agent.EstimateMessagesTokens(messages),
		"compaction_count", loaded.CompactionCount,
		"round_index", loaded.RoundIndex,
	)

	cm.state = loaded
	if len(messages) > 0 {
		cm.ContextManager.SetMessages(context.Background(), messages)
	}

	return cm, nil
}

func (m *SmartContextManager) SetSummarizer(summarizer Summarizer) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.Summarizer = summarizer
}

func (m *SmartContextManager) AddMessages(ctx context.Context, msgs ...agent.Message) {
	roles := make([]string, len(msgs))
	for i, msg := range msgs {
		roles[i] = string(msg.Role)
	}

	beforeCount := len(m.ContextManager.Messages(ctx))
	m.ContextManager.AddMessages(ctx, msgs...)
	afterCount := len(m.ContextManager.Messages(ctx))
	m.logger.Info("context manager: AddMessages",
		"added_count", len(msgs),
		"added_roles", roles,
		"before_count", beforeCount,
		"after_count", afterCount,
	)
}

func (m *SmartContextManager) SetMessages(ctx context.Context, msgs []agent.Message) {
	beforeCount := len(m.ContextManager.Messages(ctx))
	m.ContextManager.SetMessages(ctx, msgs)
	m.logger.Info("context manager: SetMessages",
		"before_count", beforeCount,
		"new_count", len(msgs),
	)
}

func (m *SmartContextManager) Messages(ctx context.Context) []agent.Message {
	messages := m.ContextManager.Messages(ctx)
	m.logger.Info("context manager: Messages read",
		"total_messages", len(messages),
		"total_tokens", agent.EstimateMessagesTokens(messages),
	)

	return messages
}

func (m *SmartContextManager) ClearMessages(ctx context.Context) {
	m.ContextManager.ClearMessages(ctx)

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	m.state = agent.ContextManagerState{UpdatedAt: now}
	m.saveLocked(ctx, nil)
}

func (m *SmartContextManager) BuildMessages(ctx context.Context) []agent.Message {
	working := cloneMessages(m.ContextManager.BuildMessages(ctx))

	if !m.config.Enabled {
		m.logger.Debug("context manager: compaction disabled",
			"messages", len(working),
		)

		return working
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	totalTokens := 0
	tokenLimitExceeded := false

	if m.config.TokenLimit > 0 {
		totalTokens = agent.EstimateMessagesTokens(working)
		tokenLimitExceeded = totalTokens > m.config.TokenLimit
	}

	shouldCompact := tokenLimitExceeded
	if m.logger.Enabled(ctx, slog.LevelDebug) {
		m.logger.Debug("context manager: build messages",
			"messages", len(working),
			"tokens", totalTokens,
			"token_limit", m.config.TokenLimit,
			"token_limit_exceeded", tokenLimitExceeded,
			"should_compact", shouldCompact,
		)
	}

	if shouldCompact {
		lastAssistantIdx := findLastAssistantMessageIndex(working)

		// Keep only the last assistant message and everything after it.
		cut := lastAssistantIdx
		if cut <= 0 {
			cut = len(working) - 1
		}

		m.logger.Info("context manager: compaction cut",
			"cut", cut,
			"last_assistant_idx", lastAssistantIdx,
			"total_messages", len(working),
		)

		overflow := cloneMessages(working[:cut])
		retained := cloneMessages(working[cut:])

		m.logger.Info("context manager: compaction split",
			"overflow_messages", len(overflow),
			"overflow_rounds", countUniqueRounds(overflow),
			"overflow_tokens", agent.EstimateMessagesTokens(overflow),
			"retained_messages", len(retained),
			"retained_rounds", countUniqueRounds(retained),
			"retained_tokens", agent.EstimateMessagesTokens(retained),
		)

		if len(overflow) > 0 {
			m.logger.Info("context manager: summarize start",
				"overflow_messages", len(overflow),
				"overflow_rounds", countUniqueRounds(overflow),
				"overflow_tokens", agent.EstimateMessagesTokens(overflow),
				"last_assistant_idx", lastAssistantIdx,
			)
			summary, err := m.config.Summarizer.Summarize(ctx, overflow)
			if err == nil {
				summary = truncatePlainText(strings.TrimSpace(summary), m.config.SummaryMaxChars)
			}
			if err == nil && summary != "" {
				summaryMsg := agent.Message{
					Role:    agent.RoleUser,
					Content: &agent.Content{Text: &summary},
				}
				working = append([]agent.Message{summaryMsg}, retained...)

				now := time.Now().UTC()
				m.state.Summary = ""
				m.state.CompactionCount++
				m.state.UpdatedAt = now

				m.logger.Info("context manager: summarize complete",
					"overflow_messages", len(overflow),
					"overflow_rounds", countUniqueRounds(overflow),
					"retained_messages", len(retained),
					"retained_rounds", countUniqueRounds(retained),
					"summary_chars", len(summary),
					"compaction_count", m.state.CompactionCount,
					"new_total_messages", len(working),
					"new_total_tokens", agent.EstimateMessagesTokens(working),
				)

				m.ContextManager.SetMessages(ctx, working)

				m.logger.Info("context manager: SetMessages after compaction",
					"set_messages_count", len(working),
				)

				if m.onCompaction != nil {
					m.onCompaction()
				}
			} else {
				m.logger.Warn("context manager: summarize failed or empty",
					"error", err,
					"summary_empty", summary == "",
					"overflow_messages", len(overflow),
				)
			}
		} else {
			m.logger.Warn("context manager: compaction skipped, empty overflow",
				"cut", cut,
				"last_assistant_idx", lastAssistantIdx,
			)
		}
	}

	m.logger.Info("context manager: BuildMessages returning",
		"returning_messages", len(working),
		"returning_tokens", agent.EstimateMessagesTokens(working),
		"compacted", shouldCompact,
	)

	m.saveLocked(ctx, working)

	return cloneMessages(working)
}

func (m *SmartContextManager) Snapshot() agent.ContextManagerState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return agent.CopyContextState(m.state)
}

func (m *SmartContextManager) OnCompaction(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.onCompaction = fn
}

func (m *SmartContextManager) saveLocked(ctx context.Context, messages []agent.Message) {
	if m.store == nil {
		return
	}

	var maxRI int64
	for i := range messages {
		if ri := int64(messages[i].RoundIndex); ri > maxRI {
			maxRI = ri
		}
	}

	m.state.RoundIndex = maxRI
	m.logger.Info("context manager: saving to store",
		"messages_count", len(messages),
		"round_index", maxRI,
		"compaction_count", m.state.CompactionCount,
	)

	if err := m.store.Save(ctx, m.state, messages); err != nil {
		m.logger.Warn("context manager: save failed", "error", err)
	}
}

func countUniqueRounds(messages []agent.Message) int {
	seen := make(map[int]struct{})

	for _, msg := range messages {
		if msg.RoundIndex != 0 {
			seen[msg.RoundIndex] = struct{}{}
		}
	}

	return len(seen)
}

func findLastAssistantMessageIndex(messages []agent.Message) int {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == agent.RoleAssistant {
			return i
		}
	}

	return -1
}

func mergeDefaultContextManagerConfig(cfg ContextManagerConfig) ContextManagerConfig {
	if cfg.TokenLimit <= 0 {
		cfg.TokenLimit = defaultContextTokenLimit
	}

	if cfg.SummaryMaxChars <= 0 {
		cfg.SummaryMaxChars = defaultContextSummaryMaxChars
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return cfg
}

func truncatePlainText(text string, maxChars int) string {
	if maxChars <= 0 || utf8.RuneCountInString(text) <= maxChars {
		return text
	}

	const suffix = "\n... (truncated)"

	budget := maxChars - utf8.RuneCountInString(suffix)
	if budget <= 0 {
		return truncateRunes(text, maxChars)
	}

	return truncateRunes(text, budget) + suffix
}

func truncateRunes(text string, limit int) string {
	if limit <= 0 {
		return ""
	}

	runeCount := 0
	for i := range text {
		if runeCount == limit {
			return text[:i]
		}

		runeCount++
	}

	return text
}
