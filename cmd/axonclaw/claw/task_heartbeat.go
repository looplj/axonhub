package claw

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/looplj/axonhub/axon/task"

	"github.com/looplj/axonhub/cmd/axonclaw/prompts"
)

const (
	HeartbeatOKToken = "HEARTBEAT_OK"
)

type HeartbeatSettings struct {
	Interval     string `json:"-"`
	ActiveStart  string `json:"active_start"`
	ActiveEnd    string `json:"active_end"`
	Timezone     string `json:"timezone"`
	LightContext bool   `json:"light_context"`
	AckMaxChars  int    `json:"ack_max_chars"`
}

func DefaultHeartbeatSettings() HeartbeatSettings {
	return HeartbeatSettings{
		Interval:     "30m",
		ActiveStart:  "08:00",
		ActiveEnd:    "23:00",
		Timezone:     "",
		LightContext: false,
		AckMaxChars:  300,
	}
}

func (s HeartbeatSettings) HeartbeatInterval() time.Duration {
	d, err := time.ParseDuration(s.Interval)
	if err != nil {
		return 30 * time.Minute
	}

	return d
}

func (s HeartbeatSettings) Location() *time.Location {
	if s.Timezone == "" {
		return time.Local
	}

	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return time.Local
	}

	return loc
}

func (s HeartbeatSettings) InActiveHours(now time.Time) bool {
	if s.ActiveStart == "" && s.ActiveEnd == "" {
		return true
	}

	loc := s.Location()
	localNow := now.In(loc)

	start, err := time.Parse("15:04", s.ActiveStart)
	if err != nil {
		return true
	}

	end, err := time.Parse("15:04", s.ActiveEnd)
	if err != nil {
		return true
	}

	current := localNow.Hour()*60 + localNow.Minute()
	startMin := start.Hour()*60 + start.Minute()
	endMin := end.Hour()*60 + end.Minute()

	if startMin <= endMin {
		return current >= startMin && current < endMin
	}

	return current >= startMin || current < endMin
}

func HeartbeatSettingsFromTask(t task.Task) HeartbeatSettings {
	settings := DefaultHeartbeatSettings()
	if strings.TrimSpace(t.Trigger.Interval) != "" {
		settings.Interval = strings.TrimSpace(t.Trigger.Interval)
	}

	actionSettings, err := parseAction[HeartbeatSettings](t.Action)
	if err != nil {
		return settings
	}

	if strings.TrimSpace(actionSettings.ActiveStart) != "" {
		settings.ActiveStart = strings.TrimSpace(actionSettings.ActiveStart)
	}

	if strings.TrimSpace(actionSettings.ActiveEnd) != "" {
		settings.ActiveEnd = strings.TrimSpace(actionSettings.ActiveEnd)
	}

	if strings.TrimSpace(actionSettings.Timezone) != "" {
		settings.Timezone = strings.TrimSpace(actionSettings.Timezone)
	}

	settings.LightContext = actionSettings.LightContext
	if actionSettings.AckMaxChars > 0 {
		settings.AckMaxChars = actionSettings.AckMaxChars
	}

	return settings
}

func ApplyHeartbeatSetting(t *task.Task, key, value string) error {
	if t == nil {
		return fmt.Errorf("heartbeat task is required")
	}

	if t.Action == nil {
		t.Action = map[string]any{}
	}

	switch key {
	case "enabled":
		return fmt.Errorf("enabled is managed via heartbeat enable/disable")
	case "interval":
		t.Trigger.Type = task.TriggerTypeInterval
		t.Trigger.Interval = value
	case "active_start":
		t.Action["active_start"] = value
	case "active_end":
		t.Action["active_end"] = value
	case "timezone":
		t.Action["timezone"] = value
	case "light_context":
		t.Action["light_context"] = value == "true" || value == "1"
	case "ack_max_chars":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid ack_max_chars value: %w", err)
		}

		t.Action["ack_max_chars"] = n
	default:
		return fmt.Errorf("unknown key %q (available: interval, active_start, active_end, timezone, light_context, ack_max_chars)", key)
	}

	return nil
}

func (h *TaskHandler) handleHeartbeat(ctx context.Context, t task.Task) error {
	h.logger.Info("execute heartbeat task", "task_id", t.ID, "task_name", t.Name)

	settings := HeartbeatSettingsFromTask(t)

	now := time.Now()
	if !settings.InActiveHours(now) {
		h.logger.Debug("heartbeat task not in active hours", "task_id", t.ID, "task_name", t.Name, "now", now)
		return nil
	}

	text, systemPrompts, err := buildHeartbeatPrompt(settings, now)
	if err != nil {
		return err
	}

	result, err := h.runner.ProcessIsolated(ctx, text, systemPrompts, "")
	if err != nil {
		return fmt.Errorf("process heartbeat task: %w", err)
	}

	if result == nil {
		return nil
	}

	output := strings.TrimSpace(result.Output)
	if output == "" {
		return nil
	}

	if _, isOK := ContainsHeartbeatOK(output); isOK {
		return nil
	}

	h.runner.FollowUP(ctx, output)

	return nil
}

func buildHeartbeatPrompt(settings HeartbeatSettings, now time.Time) (string, []string, error) {
	loc := settings.Location()
	localNow := now.In(loc)

	text := fmt.Sprintf("Run the heartbeat task now.\nCurrent time: %s", localNow.Format("2006-01-02 15:04:05 MST"))

	return text, prompts.BuildHeartbeatTaskSystemPrompts(), nil
}

func ContainsHeartbeatOK(message string) (stripped string, isOK bool) {
	trimmed := strings.TrimSpace(message)
	if trimmed == HeartbeatOKToken {
		return "", true
	}

	if strings.HasPrefix(trimmed, HeartbeatOKToken) {
		return strings.TrimSpace(trimmed[len(HeartbeatOKToken):]), true
	}

	if strings.HasSuffix(trimmed, HeartbeatOKToken) {
		return strings.TrimSpace(trimmed[:len(trimmed)-len(HeartbeatOKToken)]), true
	}

	return message, false
}
