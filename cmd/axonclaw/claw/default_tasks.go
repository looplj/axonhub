package claw

import (
	"errors"
	"fmt"

	"github.com/looplj/axonhub/axon/task"
)

const (
	SystemTaskHeartbeatID    = "axonclaw-heartbeat"
	SystemTaskSelfEvolveID   = "axonclaw-self-evolve"
	SystemTaskNameHeartbeat  = "Heartbeat"
	SystemTaskNameSelfEvolve = "Self Evolution"
)

func EnsureDefaultTasks(store *task.Store) error {
	if store == nil {
		return fmt.Errorf("task store is required")
	}

	tasks := []task.Task{
		{
			ID:      SystemTaskHeartbeatID,
			Name:    SystemTaskNameHeartbeat,
			Type:    string(TaskTypeHeartbeat),
			System:  true,
			Enabled: true,
			Hidden:  true,
			Trigger: task.Trigger{
				Type:     task.TriggerTypeInterval,
				Interval: DefaultHeartbeatAction().Interval,
			},
			Action: map[string]any{
				"active_start":  DefaultHeartbeatAction().ActiveStart,
				"active_end":    DefaultHeartbeatAction().ActiveEnd,
				"timezone":      DefaultHeartbeatAction().Timezone,
				"light_context": DefaultHeartbeatAction().LightContext,
				"ack_max_chars": DefaultHeartbeatAction().AckMaxChars,
			},
		},
		{
			ID:      SystemTaskSelfEvolveID,
			Name:    SystemTaskNameSelfEvolve,
			Type:    string(TaskTypePrompt),
			System:  false,
			Enabled: false,
			Hidden:  false,
			Trigger: task.Trigger{
				Type: task.TriggerTypeCron,
				Cron: "0 23 * * *",
			},
			Action: map[string]any{
				"message": "Run the self-evolution task now. Reflect on recent work patterns, identify repetitive tasks or useful workflows, and create skills to improve over time.",
				"mode":    "isolated",
			},
		},
	}

	for _, t := range tasks {
		if _, err := store.Get(t.ID); err == nil {
			continue
		} else if !errors.Is(err, task.ErrTaskNotFound) {
			return err
		}

		if err := store.Add(t); err != nil {
			return err
		}
	}

	return nil
}
