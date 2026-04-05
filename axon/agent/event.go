package agent

import (
	"encoding/json"
	"errors"
)

type AgentEvent struct {
	RunID     string
	Type      AgentEventType
	Message   *Message
	ToolName  string
	ToolInput string
	Result    *ToolResult
	Error     error
	Delta     string
	Thinking  string
	Usage     *Usage
}

type agentEventJSON struct {
	RunID     string         `json:"run_id,omitempty"`
	Type      AgentEventType `json:"type"`
	Message   *Message       `json:"message,omitempty"`
	ToolName  string         `json:"tool_name,omitempty"`
	ToolInput string         `json:"tool_input,omitempty"`
	Result    *ToolResult    `json:"result,omitempty"`
	Error     string         `json:"error,omitempty"`
	Delta     string         `json:"delta,omitempty"`
	Thinking  string         `json:"thinking,omitempty"`
	Usage     *Usage         `json:"usage,omitempty"`
}

func (e AgentEvent) MarshalJSON() ([]byte, error) {
	var errMsg string
	if e.Error != nil {
		errMsg = e.Error.Error()
	}
	return json.Marshal(agentEventJSON{
		RunID:     e.RunID,
		Type:      e.Type,
		Message:   e.Message,
		ToolName:  e.ToolName,
		ToolInput: e.ToolInput,
		Result:    e.Result,
		Error:     errMsg,
		Delta:     e.Delta,
		Thinking:  e.Thinking,
		Usage:     e.Usage,
	})
}

func (e *AgentEvent) UnmarshalJSON(data []byte) error {
	var tmp agentEventJSON
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	e.RunID = tmp.RunID
	e.Type = tmp.Type
	e.Message = tmp.Message
	e.ToolName = tmp.ToolName
	e.ToolInput = tmp.ToolInput
	e.Result = tmp.Result
	if tmp.Error != "" {
		e.Error = errors.New(tmp.Error)
	}
	e.Delta = tmp.Delta
	e.Thinking = tmp.Thinking
	e.Usage = tmp.Usage
	return nil
}

// AgentEventType identifies the kind of agent lifecycle event.
type AgentEventType string

const (
	EventAgentStart       AgentEventType = "agent_start"
	EventAgentEnd         AgentEventType = "agent_end"
	EventTraceStart       AgentEventType = "trace_start"
	EventTraceEnd         AgentEventType = "trace_end"
	EventMessageStart     AgentEventType = "message_start"
	EventMessageEnd       AgentEventType = "message_end"
	EventMessageAdded     AgentEventType = "message_added"
	EventToolStart        AgentEventType = "tool_start"
	EventToolEnd          AgentEventType = "tool_end"
	EventToolSkipped      AgentEventType = "tool_skipped"
	EventSteeringApplied  AgentEventType = "steering_applied"
	EventError            AgentEventType = "error"
	EventTextDelta        AgentEventType = "text_delta"
	EventTextComplete     AgentEventType = "text_complete"
	EventThinkingDelta    AgentEventType = "thinking_delta"
	EventThinkingComplete AgentEventType = "thinking_complete"
	EventToolCallDelta    AgentEventType = "tool_call_delta"
	EventToolCallComplete AgentEventType = "tool_call_complete"
	EventUsage            AgentEventType = "usage"
	EventLoopDetected     AgentEventType = "loop_detected"
	EventLoopRecovery     AgentEventType = "loop_recovery"
)
