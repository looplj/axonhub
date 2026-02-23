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
	// SystemPrompt is the base system prompt (deprecated, use SystemPrompts).
	SystemPrompt string
	// SystemPrompts is an array of system prompts that will be joined together.
	SystemPrompts []string
}

// Agent orchestrates LLM interactions with tool execution.
// Each Agent instance corresponds to a single conversation; external
// callers that need thread persistence should subscribe to agent events
// via the bus (topic: agent.event) and persist messages externally.
type Agent struct {
	config atomic.Pointer[Config]

	provider    Provider
	bus         bus.EventBus
	tools       *ToolRegistry
	logger      *slog.Logger
	middlewares []Middleware

	messages []Message
	msgMu    sync.RWMutex

	// requestIndex groups assistant messages that came from the same LLM call.
	// It is an agent-maintained field: every LLM turn must assign the same
	// RequestIndex to all assistant messages produced by that turn (text/thinking
	// blocks and tool_use blocks), so downstream adapters can safely aggregate
	// them into one "assistant message" at the provider API level.
	requestIndex atomic.Int64

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

func WithMessages(msgs []Message) Option {
	return func(a *Agent) {
		a.messages = make([]Message, len(msgs))
		copy(a.messages, msgs)

		// Continue request index numbering from persisted history to avoid collisions.
		var maxRequestIndex int64
		for i := range msgs {
			if int64(msgs[i].RequestIndex) > maxRequestIndex {
				maxRequestIndex = int64(msgs[i].RequestIndex)
			}
		}
		if maxRequestIndex > 0 {
			a.requestIndex.Store(maxRequestIndex)
		}
	}
}

// New creates a new Agent with the given config, provider, and options.
func New(config Config, provider Provider, opts ...Option) *Agent {
	if config.MaxIterations <= 0 {
		config.MaxIterations = defaultMaxIterations
	}

	a := &Agent{
		provider: provider,
		tools:    NewToolRegistry(),
		logger:   slog.Default(),
	}
	a.config.Store(&config)

	for _, opt := range opts {
		opt(a)
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

// emit publishes an event to the bus (if configured).
func (a *Agent) emit(ctx context.Context, event AgentEvent) {
	if a.bus == nil {
		return
	}

	payload, err := json.Marshal(event)
	if err != nil {
		a.logger.Error("agent: failed to marshal event", "error", err)
		return
	}

	if err := a.bus.Publish(ctx, bus.Event{
		Topic:   TopicAgentEvent,
		Type:    string(event.Type),
		Payload: payload,
	}); err != nil {
		a.logger.Error("agent: failed to publish event", "error", err)
	}
}

// addMessage appends a message to the internal history and emits
// an EventMessageAdded event so external consumers can persist it.
func (a *Agent) addMessage(ctx context.Context, msgs ...Message) {
	a.msgMu.Lock()
	a.messages = append(a.messages, msgs...)
	a.msgMu.Unlock()

	for _, msg := range msgs {
		a.emit(ctx, AgentEvent{
			Type:    EventMessageAdded,
			Message: &msg,
		})
	}
}

// Messages returns a copy of the current message history.
func (a *Agent) Messages() []Message {
	a.msgMu.RLock()
	defer a.msgMu.RUnlock()
	out := make([]Message, len(a.messages))
	copy(out, a.messages)
	return out
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
	if a.bus == nil {
		a.logger.Warn("agent: bus is nil, Start is a no-op; use Process directly")
		return
	}

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
				err := a.Process(runCtx, req.Content)
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
	if a.bus != nil && a.subID != "" {
		a.bus.Unsubscribe(a.subID)
		a.subID = ""
	}
	a.running.Store(false)
	a.logger.Info("agent stopped")
}

// Process processes a single user message synchronously and returns the
// assistant's final text response. It appends the user message to internal
// history, calls the LLM, executes tools in a loop, and returns the result.
func (a *Agent) Process(ctx context.Context, content Content) error {
	a.emit(ctx, AgentEvent{Type: EventAgentStart})
	defer a.emit(ctx, AgentEvent{Type: EventAgentEnd})

	cfg := a.Config()

	userMsg := Message{Role: RoleUser, Content: &content}
	a.addMessage(ctx, userMsg)
	a.emit(ctx, AgentEvent{
		Type:    EventMessageStart,
		Message: &userMsg,
	})

	err := a.runLoop(ctx, cfg)
	if err != nil {
		a.emit(ctx, AgentEvent{Type: EventError, Error: err})
		return err
	}

	return nil
}

// ProcessStream processes a single user message with streaming response.
// It returns a channel that emits AgentEvent for each streaming event.
// The channel is closed when processing completes or an error occurs.
func (a *Agent) ProcessStream(ctx context.Context, content Content) <-chan AgentEvent {
	events := make(chan AgentEvent, 256)

	go func() {
		defer close(events)

		emit := func(ev AgentEvent) {
			select {
			case events <- ev:
			case <-ctx.Done():
			}
			a.emit(ctx, ev)
		}

		emit(AgentEvent{Type: EventAgentStart})
		defer emit(AgentEvent{Type: EventAgentEnd})

		cfg := a.Config()

		userMsg := Message{Role: RoleUser, Content: &content}
		a.addMessage(ctx, userMsg)
		emit(AgentEvent{
			Type:    EventMessageStart,
			Message: &userMsg,
		})

		err := a.runLoopStream(ctx, cfg, events)
		if err != nil {
			emit(AgentEvent{Type: EventError, Error: err})
		}
	}()

	return events
}

// PublishRequest publishes an agent request onto the bus for asynchronous processing.
func (a *Agent) PublishRequest(ctx context.Context, content Content) error {
	if a.bus == nil {
		return fmt.Errorf("agent: bus is required for PublishRequest; use WithBus option")
	}

	payload, err := json.Marshal(AgentRequest{Content: content})
	if err != nil {
		return fmt.Errorf("agent: failed to marshal request: %w", err)
	}

	return a.bus.Publish(ctx, bus.Event{
		Topic:   TopicAgentRequest,
		Type:    "request",
		Payload: payload,
	})
}

// buildMessages constructs the message list for an LLM call, prepending
// the system prompts if configured.
func (a *Agent) buildMessages(cfg Config) []Message {
	a.msgMu.RLock()
	history := make([]Message, len(a.messages))
	copy(history, a.messages)
	a.msgMu.RUnlock()

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
// It supports both SystemPrompt (deprecated) and SystemPrompts.
// Returns a slice of non-empty system prompt strings.
func (a *Agent) buildSystemPrompts(cfg Config) []string {
	var prompts []string

	if cfg.SystemPrompt != "" {
		prompts = append(prompts, cfg.SystemPrompt)
	}

	for _, p := range cfg.SystemPrompts {
		if p != "" {
			prompts = append(prompts, p)
		}
	}

	return prompts
}

func (a *Agent) nextRequestIndex() int {
	// Start from 1 (0 means "unset" and may be treated specially).
	return int(a.requestIndex.Add(1))
}

func (a *Agent) ensureRequestIndex(msgs []Message, requestIndex int) {
	for i := range msgs {
		if msgs[i].Role != RoleAssistant {
			continue
		}
		if msgs[i].RequestIndex == 0 {
			msgs[i].RequestIndex = requestIndex
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
func (a *Agent) runLoop(ctx context.Context, cfg Config) error {
	toolDefs := a.tools.Definitions()

	a.emit(ctx, AgentEvent{Type: EventTraceStart})
	defer a.emit(ctx, AgentEvent{Type: EventTraceEnd})

	// Check for steering messages that arrived before the loop started.
	pendingSteering := a.dequeueSteering()

	iterations := 0

	// Outer loop: continues when follow-up messages arrive after the agent
	// would otherwise stop.
	for {
		hasMoreToolCalls := true

		// Inner loop: process LLM calls and tool execution with steering support.
		for hasMoreToolCalls {
			iterations++
			if iterations > cfg.MaxIterations {
				return fmt.Errorf("agent: max iterations (%d) reached", cfg.MaxIterations)
			}

			// Inject pending steering messages before the next LLM call.
			if len(pendingSteering) > 0 {
				a.addMessage(ctx, pendingSteering...)
				a.emit(ctx, AgentEvent{Type: EventSteeringApplied})
				pendingSteering = nil
			}

			messages := a.buildMessages(cfg)

			a.logger.Debug("agent: LLM call",
				"iteration", iterations,
				"message_count", len(messages),
			)

			resp, err := a.provider.Chat(ctx, cfg.Model, toolDefs, messages)
			if err != nil {
				return fmt.Errorf("agent: LLM call failed: %w", err)
			}
			requestIndex := a.nextRequestIndex()
			a.ensureRequestIndex(resp.Messages, requestIndex)

			// Separate tool-use messages from non-tool messages.
			var toolMsgs []Message
			for _, msg := range resp.Messages {
				if msg.ToolUse == nil {
					a.addMessage(ctx, msg)
				} else {
					toolMsgs = append(toolMsgs, msg)
				}
			}

			hasMoreToolCalls = len(toolMsgs) > 0

			// Execute tool calls one by one, checking for steering after each.
			steered := false
			for i, msg := range toolMsgs {
				toolResult := a.executeTool(ctx, *msg.ToolUse)

				var toolContent Content
				if toolResult.Error != nil {
					errMsg := fmt.Sprintf("error: %v", toolResult.Error)
					toolContent = Content{Text: &errMsg}
				} else {
					toolContent = toolResult.Content
				}

				toolMsg := Message{
					Role:      RoleTool,
					Content:   &toolContent,
					ToolUseID: &msg.ToolUse.ID,
				}
				a.addMessage(ctx, msg, toolMsg)

				// Check for steering after each tool execution.
				if steering := a.dequeueSteering(); len(steering) > 0 {
					// Skip remaining tool calls.
					for _, skipped := range toolMsgs[i+1:] {
						skippedIndex := requestIndex
						if skipped.RequestIndex != 0 {
							skippedIndex = skipped.RequestIndex
						}
						a.skipToolCall(ctx, *skipped.ToolUse, skippedIndex)
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

		return nil
	}
}

// executeTool runs a single tool call and returns the result.
// If the tool panics or is not found, Error is set in the result.
func (a *Agent) executeTool(ctx context.Context, tc ToolUse) (result ToolResult) {
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
		Workspace:  workspaceFromContext(ctx),
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

// skipToolCall emits a skipped-tool message pair (tool_use + tool result)
// so the conversation history stays consistent for the LLM.
func (a *Agent) skipToolCall(ctx context.Context, tc ToolUse, requestIndex int) {
	a.emit(ctx, AgentEvent{
		Type:     EventToolSkipped,
		ToolName: tc.Name,
	})

	errMsg := "Skipped due to steering message."
	toolMsg := Message{
		Role:      RoleTool,
		Content:   &Content{Text: &errMsg},
		ToolUseID: &tc.ID,
	}
	// Add original tool-use message + skipped result so history is valid.
	a.addMessage(ctx, Message{
		Role:         RoleAssistant,
		ToolUse:      &tc,
		RequestIndex: requestIndex,
	}, toolMsg)
}

// runLoopStream is the streaming version of runLoop.
// It processes streaming events from the LLM and emits them to the events channel.
func (a *Agent) runLoopStream(ctx context.Context, cfg Config, events chan AgentEvent) error {
	toolDefs := a.tools.Definitions()

	emit := func(ev AgentEvent) {
		select {
		case events <- ev:
		case <-ctx.Done():
		}
		a.emit(ctx, ev)
	}

	emit(AgentEvent{Type: EventTraceStart})
	defer emit(AgentEvent{Type: EventTraceEnd})

	pendingSteering := a.dequeueSteering()
	iterations := 0

	for {
		hasMoreToolCalls := true

		for hasMoreToolCalls {
			iterations++
			if iterations > cfg.MaxIterations {
				return fmt.Errorf("agent: max iterations (%d) reached", cfg.MaxIterations)
			}

			if len(pendingSteering) > 0 {
				a.addMessage(ctx, pendingSteering...)
				emit(AgentEvent{Type: EventSteeringApplied})
				pendingSteering = nil
			}

			messages := a.buildMessages(cfg)

			a.logger.Debug("agent: LLM stream call",
				"iteration", iterations,
				"message_count", len(messages),
			)

			stream, err := a.provider.ChatStream(ctx, cfg.Model, toolDefs, messages)
			if err != nil {
				return fmt.Errorf("agent: LLM stream call failed: %w", err)
			}

			requestIndex := a.nextRequestIndex()

			var textBuilder strings.Builder
			var thinkingBuilder strings.Builder
			var thinkingSignature string
			var toolCalls []ToolUse
			var toolCallBuilders map[string]*toolCallBuilder
			var usage *Usage

			for ev := range stream {
				if ctx.Err() != nil {
					return ctx.Err()
				}

				switch ev.Type {
				case StreamEventTextDelta:
					textBuilder.WriteString(ev.Text)
					emit(AgentEvent{Type: EventTextDelta, Delta: ev.Text})

				case StreamEventTextComplete:
					emit(AgentEvent{Type: EventTextComplete, Delta: ev.Text})

				case StreamEventThinkingDelta:
					if ev.Thinking != nil {
						thinkingBuilder.WriteString(ev.Thinking.Content)
						emit(AgentEvent{Type: EventThinkingDelta, Thinking: ev.Thinking.Content})
					}

				case StreamEventThinkingComplete:
					if ev.Thinking != nil {
						emit(AgentEvent{Type: EventThinkingComplete, Thinking: ev.Thinking.Content})
						thinkingSignature = ev.Thinking.Signature
					}

				case StreamEventToolCallDelta:
					if ev.ToolUse == nil {
						continue
					}
					if toolCallBuilders == nil {
						toolCallBuilders = make(map[string]*toolCallBuilder)
					}
					builder, ok := toolCallBuilders[ev.ToolUse.ID]
					if !ok {
						builder = &toolCallBuilder{
							id:   ev.ToolUse.ID,
							name: ev.ToolUse.Name,
						}
						toolCallBuilders[ev.ToolUse.ID] = builder
					}
					builder.jsonParts = append(builder.jsonParts, ev.Text)
					emit(AgentEvent{
						Type:      EventToolCallDelta,
						ToolName:  ev.ToolUse.Name,
						ToolInput: ev.Text,
					})

				case StreamEventToolCallComplete:
					if ev.ToolUse != nil {
						toolCalls = append(toolCalls, *ev.ToolUse)
						emit(AgentEvent{
							Type:      EventToolCallComplete,
							ToolName:  ev.ToolUse.Name,
							ToolInput: ev.ToolUse.Input,
						})
					}

				case StreamEventUsage:
					usage = ev.Usage
					emit(AgentEvent{Type: EventUsage, Usage: ev.Usage})

				case StreamEventError:
					return ev.Error

				case StreamEventDone:
				}
			}

			if len(toolCalls) == 0 && toolCallBuilders != nil {
				for _, builder := range toolCallBuilders {
					tc := ToolUse{
						ID:    builder.id,
						Name:  builder.name,
						Input: builder.buildJSON(),
					}
					toolCalls = append(toolCalls, tc)
					emit(AgentEvent{
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
					// Group content + tool-use blocks from this LLM call.
					RequestIndex: requestIndex,
				}
				a.addMessage(ctx, assistantMsg)
				emit(AgentEvent{Type: EventMessageAdded, Message: &assistantMsg})
			}

			hasMoreToolCalls = len(toolCalls) > 0

			steered := false
			for i, tc := range toolCalls {
				toolResult := a.executeToolStream(ctx, tc, events)

				var toolContent Content
				if toolResult.Error != nil {
					errMsg := fmt.Sprintf("error: %v", toolResult.Error)
					toolContent = Content{Text: &errMsg}
				} else {
					toolContent = toolResult.Content
				}

				toolMsg := Message{
					Role:      RoleTool,
					Content:   &toolContent,
					ToolUseID: &tc.ID,
				}
				a.addMessage(ctx, Message{Role: RoleAssistant, ToolUse: &tc, RequestIndex: requestIndex}, toolMsg)

				if steering := a.dequeueSteering(); len(steering) > 0 {
					for _, skipped := range toolCalls[i+1:] {
						a.skipToolCall(ctx, skipped, requestIndex)
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

// executeToolStream runs a tool and emits events to the stream channel.
func (a *Agent) executeToolStream(ctx context.Context, tc ToolUse, events chan AgentEvent) (result ToolResult) {
	emit := func(ev AgentEvent) {
		select {
		case events <- ev:
		case <-ctx.Done():
		}
		a.emit(ctx, ev)
	}

	defer func() {
		if r := recover(); r != nil {
			result.Error = fmt.Errorf("panic in tool %q: %v", tc.Name, r)
			a.logger.Error("agent: tool panic", "tool", tc.Name, "panic", r)
		}
	}()

	emit(AgentEvent{
		Type:      EventToolStart,
		ToolName:  tc.Name,
		ToolInput: tc.Input,
	})

	if err := a.tools.ValidateArguments(tc.Name, json.RawMessage(tc.Input)); err != nil {
		result.Error = fmt.Errorf("invalid arguments for tool %q: %w", tc.Name, err)
		emit(AgentEvent{
			Type:     EventToolEnd,
			ToolName: tc.Name,
			Result:   &result,
		})
		return result
	}

	tool, ok := a.tools.Get(tc.Name)
	if !ok {
		result.Error = fmt.Errorf("tool %q not found", tc.Name)
		emit(AgentEvent{
			Type:     EventToolEnd,
			ToolName: tc.Name,
			Result:   &result,
		})
		return result
	}

	a.logger.Debug("agent: executing tool", "tool", tc.Name)

	var executedMiddlewares []Middleware
	for _, mw := range a.middlewares {
		req := ToolRequest{
			ThreadID:   clawcontext.ThreadID(ctx),
			Workspace:  workspaceFromContext(ctx),
			ToolCallID: tc.ID,
			ToolName:   tc.Name,
			ToolInput:  tc.Input,
			StartedAt:  time.Now(),
		}
		if err := mw.BeforeTool(ctx, req); err != nil {
			result.Error = err
			emit(AgentEvent{
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

	emit(AgentEvent{
		Type:      EventToolEnd,
		ToolName:  tc.Name,
		ToolInput: tc.Input,
		Result:    &result,
	})

	return result
}

type toolCallBuilder struct {
	id        string
	name      string
	jsonParts []string
}

func (b *toolCallBuilder) buildJSON() string {
	result := ""
	for _, part := range b.jsonParts {
		result += part
	}
	return result
}

type ctxKey string

const workspaceCtxKey ctxKey = "workspace"

func WithWorkspace(ctx context.Context, workspace string) context.Context {
	return context.WithValue(ctx, workspaceCtxKey, workspace)
}

func workspaceFromContext(ctx context.Context) string {
	v, _ := ctx.Value(workspaceCtxKey).(string)
	return v
}

func (a *Agent) runAfterMiddlewares(ctx context.Context, tc ToolUse, toolErr error, mws []Middleware) {
	for i := len(mws) - 1; i >= 0; i-- {
		req := ToolRequest{
			ThreadID:   axoncontext.ThreadID(ctx),
			Workspace:  workspaceFromContext(ctx),
			ToolCallID: tc.ID,
			ToolName:   tc.Name,
			ToolInput:  tc.Input,
			StartedAt:  time.Now(),
		}
		_ = mws[i].AfterTool(ctx, req, toolErr)
	}
}
