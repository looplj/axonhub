package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/looplj/axonhub/axon/bus"
	axoncontext "github.com/looplj/axonhub/axon/context"
	clawcontext "github.com/looplj/axonhub/axon/context"
)

const (
	// TopicAgentRequest is the bus topic the agent listens on.
	TopicAgentRequest = "agent.request"
	// TopicAgentEvent is the bus topic the agent publishes lifecycle events on.
	TopicAgentEvent = "agent.event"

	// defaultMaxIterations is the default tool-call loop limit.
	defaultMaxIterations = 25
)

// AgentRequest is the payload published on the agent.request bus topic.
type AgentRequest struct {
	Content Content `json:"content"`
}

// Config holds agent configuration.
type Config struct {
	// Model is the LLM model identifier.
	Model string
	// MaxIterations limits tool-call loops (0 means unlimited).
	MaxIterations int
	// SystemPrompts is an array of system prompts that will be joined together.
	SystemPrompts []string
	// LoopDetector configures behavioral loop detection.
	// When nil, default settings are used (enabled, threshold=5).
	LoopDetector *LoopDetectorConfig
}

// Result holds the outcome of an agent run.
type Result struct {
	// Output is the final assistant text from the agent.
	Output string
	// Usage is the total token usage across all LLM calls.
	Usage Usage
}

// Agent orchestrates LLM interactions with tool execution.
// Each Agent instance corresponds to a single conversation; external
// callers that need thread persistence should subscribe to agent events
// via the bus (topic: agent.event) and persist messages externally.
type Agent struct {
	config atomic.Pointer[Config]

	provider        Provider
	contextManager  ContextManager
	bus             bus.EventBus
	tools           *ToolRegistry
	logger          *slog.Logger
	middlewares     []Middleware
	initialMessages []Message

	// roundIndex groups messages that belong to the same LLM call round.
	// Every LLM round assigns the same RoundIndex to all produced messages
	// (assistant text/thinking, tool_use, and tool result), so downstream
	// adapters can safely aggregate and compaction can keep them together.
	roundIndex atomic.Int64

	loopDetector *loopDetector

	steeringQueue []Message
	followUpQueue []Message
	queueMu       sync.Mutex

	running   atomic.Bool
	subID     bus.SubscriptionID
	cancelRun context.CancelFunc
}

// Option configures an Agent.
type Option func(*Agent)

// WithBus sets the event bus for the agent.
func WithBus(b bus.EventBus) Option {
	return func(a *Agent) {
		a.bus = b
	}
}

// WithLogger sets the logger for the agent.
func WithLogger(l *slog.Logger) Option {
	return func(a *Agent) {
		a.logger = l
	}
}

func WithMiddlewares(mws ...Middleware) Option {
	return func(a *Agent) {
		a.middlewares = append(a.middlewares, mws...)
	}
}

// Middlewares returns a snapshot of all middlewares currently registered
// on this agent. The returned slice is safe to read concurrently.
func (a *Agent) Middlewares() []Middleware {
	if len(a.middlewares) == 0 {
		return nil
	}

	out := make([]Middleware, len(a.middlewares))
	copy(out, a.middlewares)

	return out
}

func WithContextManager(cm ContextManager) Option {
	return func(a *Agent) {
		a.contextManager = cm
	}
}

func WithMessages(msgs []Message) Option {
	return func(a *Agent) {
		a.initialMessages = cloneMessages(msgs)
	}
}

// New creates a new Agent with the given config, provider, and options.
func New(config Config, provider Provider, opts ...Option) *Agent {
	if config.MaxIterations <= 0 {
		config.MaxIterations = defaultMaxIterations
	}

	ldCfg := DefaultLoopDetectorConfig()
	if config.LoopDetector != nil {
		ldCfg = *config.LoopDetector
	}

	a := &Agent{
		provider:     provider,
		tools:        NewToolRegistry(),
		logger:       slog.Default(),
		loopDetector: newLoopDetector(ldCfg),
	}
	a.config.Store(&config)

	for _, opt := range opts {
		opt(a)
	}

	if a.bus == nil {
		a.bus = bus.NewInProcess()
	}

	if a.contextManager == nil {
		a.contextManager = NewSimpleContextManager(a.initialMessages)
	} else if len(a.initialMessages) > 0 {
		a.contextManager.AddMessages(context.Background(), a.initialMessages...)
	}

	// Restore round index counter from persisted state.
	if ri := a.contextManager.Snapshot().RoundIndex; ri > 0 {
		a.roundIndex.Store(ri)
	}

	return a
}

func (a *Agent) Config() Config {
	return *a.config.Load()
}

func (a *Agent) SetConfig(next Config) {
	if next.MaxIterations <= 0 {
		next.MaxIterations = defaultMaxIterations
	}
	a.config.Store(&next)
}

func (a *Agent) UpdateConfig(update func(Config) Config) {
	for {
		cur := a.config.Load()
		var base Config
		if cur != nil {
			base = *cur
		}
		next := update(base)
		if next.MaxIterations <= 0 {
			next.MaxIterations = defaultMaxIterations
		}
		if a.config.CompareAndSwap(cur, &next) {
			return
		}
	}
}

// RegisterTool adds a tool to the agent's registry.
func (a *Agent) RegisterTool(tool Tool) {
	a.tools.Register(tool)
}

// RegisteredTools returns a snapshot of all tools currently registered
// on this agent. The returned slice is safe to read concurrently.
func (a *Agent) RegisteredTools() []Tool {
	names := a.tools.List()

	result := make([]Tool, 0, len(names))
	for _, name := range names {
		if t, ok := a.tools.Get(name); ok {
			result = append(result, t)
		}
	}

	return result
}

// emit publishes an event to the bus.
func (a *Agent) emit(ctx context.Context, event AgentEvent) {
	if event.RunID == "" {
		event.RunID = a.runIDFromContext(ctx)
	}

	if err := a.bus.Publish(ctx, bus.Event{
		Topic:   TopicAgentEvent,
		Type:    string(event.Type),
		Payload: event,
	}); err != nil {
		a.logger.Error("agent: failed to publish event", "error", err)
	}
}

type runIDContextKey struct{}

func withRunID(ctx context.Context, runID string) context.Context {
	return context.WithValue(ctx, runIDContextKey{}, runID)
}

func (a *Agent) runIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	runID, _ := ctx.Value(runIDContextKey{}).(string)

	return runID
}

func (a *Agent) newRunContext(ctx context.Context) (context.Context, string) {
	runID := uuid.NewString()

	return withRunID(ctx, runID), runID
}

// addMessage appends a message to the internal history and emits
// an EventMessageAdded event so external consumers can persist it.
func (a *Agent) addMessage(ctx context.Context, msgs ...Message) {
	a.contextManager.AddMessages(ctx, msgs...)

	for _, msg := range msgs {
		a.emit(ctx, AgentEvent{
			Type:    EventMessageAdded,
			Message: &msg,
		})
	}
}

// Messages returns a copy of the current message history.
func (a *Agent) Messages() []Message {
	return a.contextManager.Messages(context.Background())
}

// ClearMessages clears all messages from the agent's history.
func (a *Agent) ClearMessages() {
	a.contextManager.ClearMessages(context.Background())
}

// Inject inserts a message into the agent's history mid-process
// (e.g. an out-of-band system or user message).
func (a *Agent) Inject(ctx context.Context, msg Message) {
	a.addMessage(ctx, msg)
}

// Steer queues a message to interrupt the agent mid-tool-execution.
// The message is injected after the current tool finishes and remaining
// tool calls in the same response are skipped.
func (a *Agent) Steer(msg Message) {
	a.queueMu.Lock()
	defer a.queueMu.Unlock()
	a.steeringQueue = append(a.steeringQueue, msg)
}

// FollowUp queues a message to be processed after the agent finishes
// its current run (no more tool calls or steering). The agent loop
// will continue with another LLM turn.
func (a *Agent) FollowUp(msg Message) {
	a.queueMu.Lock()
	defer a.queueMu.Unlock()
	a.followUpQueue = append(a.followUpQueue, msg)
}

// ClearQueues empties both the steering and follow-up queues.
func (a *Agent) ClearQueues() {
	a.queueMu.Lock()
	defer a.queueMu.Unlock()
	a.steeringQueue = nil
	a.followUpQueue = nil
}

// dequeueSteering atomically drains the steering queue.
func (a *Agent) dequeueSteering() []Message {
	a.queueMu.Lock()
	defer a.queueMu.Unlock()
	if len(a.steeringQueue) == 0 {
		return nil
	}
	msgs := a.steeringQueue
	a.steeringQueue = nil
	return msgs
}

// dequeueFollowUp atomically drains the follow-up queue.
func (a *Agent) dequeueFollowUp() []Message {
	a.queueMu.Lock()
	defer a.queueMu.Unlock()
	if len(a.followUpQueue) == 0 {
		return nil
	}
	msgs := a.followUpQueue
	a.followUpQueue = nil
	return msgs
}

// Start begins the agent loop in a background goroutine, subscribing to the
// bus for agent.request events. It returns immediately. Call Stop to
// unsubscribe and terminate the loop.
func (a *Agent) Start(ctx context.Context) {
	runCtx, cancel := context.WithCancel(ctx)
	a.cancelRun = cancel
	a.running.Store(true)

	a.logger.Info("agent started", "model", a.Config().Model)

	requests := make(chan AgentRequest, 64)

	a.subID = a.bus.Subscribe(TopicAgentRequest, bus.TypedHandler(func(_ context.Context, _ bus.Event, req AgentRequest) error {
		select {
		case requests <- req:
		case <-runCtx.Done():
		}
		return nil
	}))

	go func() {
		defer a.running.Store(false)
		for {
			select {
			case <-runCtx.Done():
				a.logger.Info("agent stopped")
				return
			case req := <-requests:
				reqCtx, _ := a.newRunContext(runCtx)

				_, err := a.Process(reqCtx, req.Content)
				if err != nil {
					a.logger.Error("agent: process error", "error", err)
					continue
				}
				a.logger.Debug("agent: process complete")
			}
		}
	}()
}

// Stop gracefully stops the agent loop and unsubscribes from the bus.
func (a *Agent) Stop() {
	if a.cancelRun != nil {
		a.cancelRun()
	}

	if a.subID != "" {
		a.bus.Unsubscribe(a.subID)
		a.subID = ""
	}
	a.running.Store(false)
	a.logger.Info("agent stopped")
}

// Process processes a single user message synchronously and returns the
// assistant's final text response. It appends the user message to internal
// history, calls the LLM, executes tools in a loop, and returns the result.
func (a *Agent) Process(ctx context.Context, content Content) (*Result, error) {
	if a.runIDFromContext(ctx) == "" {
		ctx, _ = a.newRunContext(ctx)
	}

	a.emit(ctx, AgentEvent{Type: EventAgentStart})
	defer a.emit(ctx, AgentEvent{Type: EventAgentEnd})

	cfg := a.Config()

	roundIndex := a.nextRoundIndex()
	userMsg := Message{Role: RoleUser, Content: &content, RoundIndex: roundIndex}
	a.addMessage(ctx, userMsg)
	a.emit(ctx, AgentEvent{
		Type:    EventMessageStart,
		Message: &userMsg,
	})

	result, err := a.runLoop(ctx, cfg, roundIndex)
	if err != nil {
		a.emit(ctx, AgentEvent{Type: EventError, Error: err})
		return nil, err
	}

	return result, nil
}

// ProcessStream processes a single user message with streaming response.
// It returns a channel that emits AgentEvent for each streaming event.
// The channel is closed when processing completes or an error occurs.
func (a *Agent) ProcessStream(ctx context.Context, content Content) <-chan AgentEvent {
	events := make(chan AgentEvent, 256)
	runCtx, runID := a.newRunContext(ctx)

	var closed atomic.Bool

	subID := a.bus.Subscribe(TopicAgentEvent, func(_ context.Context, ev bus.Event) error {
		if closed.Load() {
			return nil
		}

		agentEv, ok := ev.Payload.(AgentEvent)
		if !ok {
			return nil
		}

		if agentEv.RunID != runID {
			return nil
		}

		select {
		case events <- agentEv:
		case <-runCtx.Done():
		}

		return nil
	})

	go func() {
		defer func() {
			closed.Store(true)
			a.bus.Unsubscribe(subID)
			close(events)
		}()

		a.emit(runCtx, AgentEvent{Type: EventAgentStart})
		defer a.emit(runCtx, AgentEvent{Type: EventAgentEnd})

		cfg := a.Config()

		roundIndex := a.nextRoundIndex()
		userMsg := Message{Role: RoleUser, Content: &content, RoundIndex: roundIndex}
		a.addMessage(runCtx, userMsg)
		a.emit(runCtx, AgentEvent{
			Type:    EventMessageStart,
			Message: &userMsg,
		})

		err := a.runLoopStream(runCtx, cfg, roundIndex)
		if err != nil {
			a.emit(runCtx, AgentEvent{Type: EventError, Error: err})
		}
	}()

	return events
}

// PublishRequest publishes an agent request onto the bus for asynchronous processing.
func (a *Agent) PublishRequest(ctx context.Context, content Content) error {
	return a.bus.Publish(ctx, bus.Event{
		Topic:   TopicAgentRequest,
		Type:    "request",
		Payload: AgentRequest{Content: content},
	})
}

// buildMessages constructs the message list for an LLM call, prepending
// the system prompts if configured.
func (a *Agent) buildMessages(ctx context.Context, cfg Config) []Message {
	history := a.contextManager.BuildMessages(ctx)

	systemPrompts := a.buildSystemPrompts(cfg)
	if len(systemPrompts) == 0 {
		return history
	}

	messages := make([]Message, 0, len(systemPrompts)+len(history))
	for _, prompt := range systemPrompts {
		messages = append(messages, Message{
			Role:    RoleSystem,
			Content: &Content{Text: &prompt},
		})
	}
	messages = append(messages, history...)
	return messages
}

// buildSystemPrompts builds the system prompts from Config.
// Returns a slice of non-empty system prompt strings.
func (a *Agent) buildSystemPrompts(cfg Config) []string {
	var prompts []string

	for _, p := range cfg.SystemPrompts {
		if p != "" {
			prompts = append(prompts, p)
		}
	}

	return prompts
}

func (a *Agent) nextRoundIndex() int {
	return int(a.roundIndex.Add(1))
}

func (a *Agent) ensureRoundIndex(msgs []Message, roundIndex int) {
	for i := range msgs {
		if msgs[i].RoundIndex == 0 {
			msgs[i].RoundIndex = roundIndex
		}
	}
}

// runLoop is the core agent loop. It sends messages to the LLM, executes any
// requested tools, appends results, and repeats until the model stops calling
// tools or MaxIterations is reached.
//
// The loop supports two interrupt mechanisms:
//   - Steering: checked after each tool execution; remaining tool calls are
//     skipped and steering messages are injected before the next LLM call.
//   - Follow-up: checked when the agent would otherwise stop (no more tool
//     calls); follow-up messages are injected and the loop continues.
func (a *Agent) runLoop(ctx context.Context, cfg Config, initialRoundIndex int) (*Result, error) {
	toolDefs := a.tools.Definitions()

	a.emit(ctx, AgentEvent{Type: EventTraceStart})
	defer a.emit(ctx, AgentEvent{Type: EventTraceEnd})

	// Reset loop detector state for this request.
	a.loopDetector.reset()

	// Check for steering messages that arrived before the loop started.
	pendingSteering := a.dequeueSteering()

	iterations := 0
	// Use the round index from the user message for the first LLM call,
	// so user + assistant messages share the same round.
	nextRound := initialRoundIndex

	var (
		totalUsage Usage
		lastOutput string
	)

	// Outer loop: continues when follow-up messages arrive after the agent
	// would otherwise stop.
	for {
		hasMoreToolCalls := true

		// Inner loop: process LLM calls and tool execution with steering support.
		for hasMoreToolCalls {
			iterations++
			if iterations > cfg.MaxIterations {
				return nil, fmt.Errorf("agent: max iterations (%d) reached", cfg.MaxIterations)
			}

			// Inject pending steering messages before the next LLM call.
			if len(pendingSteering) > 0 {
				a.addMessage(ctx, pendingSteering...)
				a.emit(ctx, AgentEvent{Type: EventSteeringApplied})
				pendingSteering = nil
			}

			messages := a.buildMessages(ctx, cfg)

			a.logger.Debug("agent: LLM call",
				"iteration", iterations,
				"message_count", len(messages),
			)

			resp, err := a.provider.Chat(ctx, cfg.Model, toolDefs, messages)
			if err != nil {
				return nil, fmt.Errorf("agent: LLM call failed: %w", err)
			}
			a.emit(ctx, AgentEvent{Type: EventUsage, Usage: &resp.Usage})
			totalUsage.InputTokens += resp.Usage.InputTokens
			totalUsage.OutputTokens += resp.Usage.OutputTokens

			roundIndex := nextRound
			nextRound = a.nextRoundIndex()
			a.ensureRoundIndex(resp.Messages, roundIndex)

			// Separate tool-use messages from non-tool messages.
			var toolMsgs []Message
			for _, msg := range resp.Messages {
				if msg.ToolCall == nil {
					a.addMessage(ctx, msg)

					if msg.Role == RoleAssistant && msg.Content != nil {
						if text := msg.Content.String(); text != "" {
							lastOutput = text
						}
					}
				} else {
					toolMsgs = append(toolMsgs, msg)
				}
			}

			hasMoreToolCalls = len(toolMsgs) > 0

			// Extract tool calls from messages for unified processing.
			toolCalls := make([]ToolCall, len(toolMsgs))
			for i, msg := range toolMsgs {
				toolCalls[i] = *msg.ToolCall
			}

			// Execute tool calls one by one, checking for steering after each.
			steered := false

			for i, tc := range toolCalls {
				toolResult := a.executeTool(ctx, tc)

				var toolContent Content
				var isError bool
				if toolResult.Error != nil {
					errMsg := fmt.Sprintf("error: %v", toolResult.Error)
					toolContent = Content{Text: &errMsg}
					isError = true
				} else {
					toolContent = toolResult.Content
				}

				toolMsg := Message{
					Role:       RoleTool,
					Content:    &toolContent,
					ToolUseID:  &tc.ID,
					IsError:    &isError,
					RoundIndex: roundIndex,
				}
				a.addMessage(ctx, toolMsgs[i], toolMsg)

				// Check for loop detection after each tool execution.
				steered, err = a.handleLoopDetection(ctx, tc, toolCalls[i+1:], roundIndex)
				if err != nil {
					return nil, err
				}

				if steered {
					break
				}

				// Check for steering after each tool execution.
				if steering := a.dequeueSteering(); len(steering) > 0 {
					for _, skipped := range toolCalls[i+1:] {
						a.skipToolCall(ctx, skipped, roundIndex)
					}
					pendingSteering = steering
					steered = true
					break
				}
			}

			if steered {
				// Continue inner loop: pending steering will be injected
				// before the next LLM call.
				hasMoreToolCalls = true
				continue
			}

			if !hasMoreToolCalls {
				// Also check steering that arrived during the last LLM call
				// (no tools to interrupt, but we can still inject before stopping).
				if steering := a.dequeueSteering(); len(steering) > 0 {
					pendingSteering = steering
					hasMoreToolCalls = true
				}
			}
		}

		// Agent would stop here. Check for follow-up messages.
		if followUp := a.dequeueFollowUp(); len(followUp) > 0 {
			a.addMessage(ctx, followUp...)
			continue
		}

		return &Result{Output: lastOutput, Usage: totalUsage}, nil
	}
}

// executeTool runs a single tool call and returns the result.
// If the tool panics or is not found, Error is set in the result.
func (a *Agent) executeTool(ctx context.Context, tc ToolCall) (result ToolResult) {
	defer func() {
		if r := recover(); r != nil {
			result.Error = fmt.Errorf("panic in tool %q: %v", tc.Name, r)
			a.logger.Error("agent: tool panic",
				"tool", tc.Name,
				"panic", r,
			)
		}
	}()

	a.emit(ctx, AgentEvent{
		Type:      EventToolStart,
		ToolName:  tc.Name,
		ToolInput: tc.Input,
	})

	if err := a.tools.ValidateArguments(tc.Name, json.RawMessage(tc.Input)); err != nil {
		result.Error = fmt.Errorf("invalid arguments for tool %q: %w", tc.Name, err)
		a.emit(ctx, AgentEvent{
			Type:     EventToolEnd,
			ToolName: tc.Name,
			Result:   &result,
		})
		return result
	}

	tool, ok := a.tools.Get(tc.Name)
	if !ok {
		result.Error = fmt.Errorf("tool %q not found", tc.Name)
		a.emit(ctx, AgentEvent{
			Type:     EventToolEnd,
			ToolName: tc.Name,
			Result:   &result,
		})
		return result
	}

	a.logger.Debug("agent: executing tool",
		"tool", tc.Name,
	)

	req := ToolRequest{
		ThreadID:   clawcontext.ThreadID(ctx),
		Workspace:  axoncontext.Workspace(ctx),
		ToolCallID: tc.ID,
		ToolName:   tc.Name,
		ToolInput:  tc.Input,
		StartedAt:  time.Now(),
	}
	var executedMiddlewares []Middleware
	for _, mw := range a.middlewares {

		if err := mw.BeforeTool(ctx, req); err != nil {
			result.Error = err
			a.emit(ctx, AgentEvent{
				Type:      EventToolEnd,
				ToolName:  tc.Name,
				ToolInput: tc.Input,
				Result:    &result,
			})
			a.runAfterMiddlewares(ctx, tc, result.Error, executedMiddlewares)
			return result
		}
		executedMiddlewares = append(executedMiddlewares, mw)
	}

	result = tool.Execute(ctx, json.RawMessage(tc.Input))
	a.runAfterMiddlewares(ctx, tc, result.Error, executedMiddlewares)

	a.emit(ctx, AgentEvent{
		Type:      EventToolEnd,
		ToolName:  tc.Name,
		ToolInput: tc.Input,
		Result:    &result,
	})

	return result
}

// buildLoopRecoveryMessage creates a user message that nudges the LLM to
// break out of a detected loop.
func (a *Agent) buildLoopRecoveryMessage(detection LoopDetection, roundIndex int) Message {
	text := fmt.Sprintf(
		"System: Potential loop detected. Details: %s. "+
			"Please take a step back and confirm you are making forward progress. "+
			"If not, analyze your previous actions and try a different approach. "+
			"Avoid repeating the same tool calls or responses without meaningful new results.",
		detection.Detail,
	)

	return Message{
		Role:       RoleUser,
		Content:    &Content{Text: &text},
		RoundIndex: roundIndex,
	}
}

// skipToolCall emits a skipped-tool message pair (tool_use + tool result)
// so the conversation history stays consistent for the LLM.
func (a *Agent) skipToolCall(ctx context.Context, tc ToolCall, roundIndex int) {
	a.emit(ctx, AgentEvent{
		Type:     EventToolSkipped,
		ToolName: tc.Name,
	})

	errMsg := "Skipped due to steering message."
	isError := true
	toolMsg := Message{
		Role:       RoleTool,
		Content:    &Content{Text: &errMsg},
		ToolUseID:  &tc.ID,
		IsError:    &isError,
		RoundIndex: roundIndex,
	}
	// Add original tool-use message + skipped result so history is valid.
	a.addMessage(ctx, Message{
		Role:       RoleAssistant,
		ToolCall:   &tc,
		RoundIndex: roundIndex,
	}, toolMsg)
}

// handleLoopDetection checks for loop detection after a tool execution.
// It returns steered=true if a recovery was performed and remaining calls should be skipped,
// or an error if recovery is exhausted.
func (a *Agent) handleLoopDetection(ctx context.Context, tc ToolCall, remainingToolCalls []ToolCall, roundIndex int) (steered bool, err error) {
	detection := a.loopDetector.checkToolCall(tc.Name, tc.Input)
	if !detection.Detected {
		return false, nil
	}

	a.emit(ctx, AgentEvent{Type: EventLoopDetected, Delta: detection.Detail})
	a.logger.Warn("agent: loop detected",
		"type", detection.Type,
		"detail", detection.Detail,
		"count", detection.Count,
	)

	if a.loopDetector.canRecover() {
		a.emit(ctx, AgentEvent{Type: EventLoopRecovery, Delta: detection.Detail})
		recoveryMsg := a.buildLoopRecoveryMessage(detection, roundIndex)
		a.addMessage(ctx, recoveryMsg)

		for _, skipped := range remainingToolCalls {
			a.skipToolCall(ctx, skipped, roundIndex)
		}

		return true, nil
	}

	return false, fmt.Errorf("agent: loop detected and recovery exhausted: %s", detection.Detail)
}

// runLoopStream is the streaming version of runLoop.
// It processes streaming events from the LLM and emits them via the bus.
//
//nolint:maintidx // Checked.
func (a *Agent) runLoopStream(ctx context.Context, cfg Config, initialRoundIndex int) error {
	toolDefs := a.tools.Definitions()

	a.emit(ctx, AgentEvent{Type: EventTraceStart})
	defer a.emit(ctx, AgentEvent{Type: EventTraceEnd})

	// Reset loop detector state for this request.
	a.loopDetector.reset()

	pendingSteering := a.dequeueSteering()
	iterations := 0
	nextRound := initialRoundIndex

	for {
		hasMoreToolCalls := true

		for hasMoreToolCalls {
			iterations++
			if iterations > cfg.MaxIterations {
				return fmt.Errorf("agent: max iterations (%d) reached", cfg.MaxIterations)
			}

			if len(pendingSteering) > 0 {
				a.addMessage(ctx, pendingSteering...)
				a.emit(ctx, AgentEvent{Type: EventSteeringApplied})
				pendingSteering = nil
			}

			messages := a.buildMessages(ctx, cfg)

			a.logger.Debug("agent: LLM stream call",
				"iteration", iterations,
				"message_count", len(messages),
			)

			stream, err := a.provider.ChatStream(ctx, cfg.Model, toolDefs, messages)
			if err != nil {
				return fmt.Errorf("agent: LLM stream call failed: %w", err)
			}

			roundIndex := nextRound
			nextRound = a.nextRoundIndex()

			var (
				textBuilder       strings.Builder
				thinkingBuilder   strings.Builder
				thinkingSignature string
				toolCalls         []ToolCall
				toolCallBuilders  map[string]*toolCallBuilder
				toolCallOrder     []string
				usage             *Usage
			)

			for ev := range stream {
				if ctx.Err() != nil {
					return ctx.Err()
				}

				switch ev.Type {
				case StreamEventTextDelta:
					textBuilder.WriteString(ev.Text)
					a.emit(ctx, AgentEvent{Type: EventTextDelta, Delta: ev.Text})

				case StreamEventTextComplete:
					a.emit(ctx, AgentEvent{Type: EventTextComplete, Delta: ev.Text})

				case StreamEventThinkingDelta:
					if ev.Thinking != nil {
						thinkingBuilder.WriteString(ev.Thinking.Content)
						a.emit(ctx, AgentEvent{Type: EventThinkingDelta, Thinking: ev.Thinking.Content})
					}

				case StreamEventThinkingComplete:
					if ev.Thinking != nil {
						a.emit(ctx, AgentEvent{Type: EventThinkingComplete, Thinking: ev.Thinking.Content})
						thinkingSignature = ev.Thinking.Signature
					}

				case StreamEventToolCallDelta:
					if ev.ToolCall == nil {
						continue
					}
					if toolCallBuilders == nil {
						toolCallBuilders = make(map[string]*toolCallBuilder)
					}

					builder, ok := toolCallBuilders[ev.ToolCall.ID]
					if !ok {
						builder = &toolCallBuilder{
							Id:   ev.ToolCall.ID,
							Name: ev.ToolCall.Name,
						}
						toolCallBuilders[ev.ToolCall.ID] = builder
						toolCallOrder = append(toolCallOrder, ev.ToolCall.ID)
					}

					builder.InputDelta = append(builder.InputDelta, ev.Text)
					a.emit(ctx, AgentEvent{
						Type:      EventToolCallDelta,
						ToolName:  ev.ToolCall.Name,
						ToolInput: ev.Text,
					})

				case StreamEventToolCallComplete:
					if ev.ToolCall != nil {
						toolCalls = append(toolCalls, *ev.ToolCall)
						a.emit(ctx, AgentEvent{
							Type:      EventToolCallComplete,
							ToolName:  ev.ToolCall.Name,
							ToolInput: ev.ToolCall.Input,
						})
					}

				case StreamEventUsage:
					usage = ev.Usage
					a.emit(ctx, AgentEvent{Type: EventUsage, Usage: ev.Usage})

				case StreamEventError:
					return ev.Error

				case StreamEventDone:
				}
			}

			if len(toolCalls) == 0 && toolCallBuilders != nil {
				for _, toolCallID := range toolCallOrder {
					builder := toolCallBuilders[toolCallID]
					if builder == nil {
						continue
					}

					tc := ToolCall{
						ID:    builder.Id,
						Name:  builder.Name,
						Input: builder.buildJSON(),
					}
					toolCalls = append(toolCalls, tc)
					a.emit(ctx, AgentEvent{
						Type:      EventToolCallComplete,
						ToolName:  tc.Name,
						ToolInput: tc.Input,
					})
				}
			}

			var contentParts []ContentPart
			thinkingText := thinkingBuilder.String()
			if len(thinkingText) > 0 {
				contentParts = append(contentParts, ContentPart{
					Type:              ContentPartThinking,
					Thinking:          thinkingText,
					ThinkingSignature: thinkingSignature,
				})
			}
			textText := textBuilder.String()
			if len(textText) > 0 {
				contentParts = append(contentParts, ContentPart{
					Type: ContentPartText,
					Text: textText,
				})
			}

			var assistantMsg Message
			if len(contentParts) > 0 {
				assistantMsg = Message{
					Role:    RoleAssistant,
					Content: &Content{Parts: contentParts},
					// Group content + tool-use blocks from this LLM call round.
					RoundIndex: roundIndex,
				}
				a.addMessage(ctx, assistantMsg)
			}

			hasMoreToolCalls = len(toolCalls) > 0

			steered := false
			for i, tc := range toolCalls {
				toolResult := a.executeTool(ctx, tc)

				var toolContent Content
				var isError bool
				if toolResult.Error != nil {
					errMsg := fmt.Sprintf("error: %v", toolResult.Error)
					toolContent = Content{Text: &errMsg}
					isError = true
				} else {
					toolContent = toolResult.Content
				}

				toolMsg := Message{
					Role:       RoleTool,
					Content:    &toolContent,
					ToolUseID:  &tc.ID,
					IsError:    &isError,
					RoundIndex: roundIndex,
				}
				a.addMessage(ctx, Message{Role: RoleAssistant, ToolCall: &tc, RoundIndex: roundIndex}, toolMsg)

				steered, err = a.handleLoopDetection(ctx, tc, toolCalls[i+1:], roundIndex)
				if err != nil {
					return err
				}

				if steered {
					break
				}

				if steering := a.dequeueSteering(); len(steering) > 0 {
					for _, skipped := range toolCalls[i+1:] {
						a.skipToolCall(ctx, skipped, roundIndex)
					}
					pendingSteering = steering
					steered = true
					break
				}
			}

			if steered {
				hasMoreToolCalls = true
				continue
			}

			if !hasMoreToolCalls {
				if steering := a.dequeueSteering(); len(steering) > 0 {
					pendingSteering = steering
					hasMoreToolCalls = true
				}
			}

			_ = usage
		}

		if followUp := a.dequeueFollowUp(); len(followUp) > 0 {
			a.addMessage(ctx, followUp...)
			continue
		}

		return nil
	}
}

func (a *Agent) runAfterMiddlewares(ctx context.Context, tc ToolCall, toolErr error, mws []Middleware) {
	for i := len(mws) - 1; i >= 0; i-- {
		req := ToolRequest{
			ThreadID:   axoncontext.ThreadID(ctx),
			Workspace:  axoncontext.Workspace(ctx),
			ToolCallID: tc.ID,
			ToolName:   tc.Name,
			ToolInput:  tc.Input,
			StartedAt:  time.Now(),
		}
		_ = mws[i].AfterTool(ctx, req, toolErr)
	}
}
