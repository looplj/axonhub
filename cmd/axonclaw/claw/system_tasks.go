package claw

import (
	"errors"
	"fmt"

	"github.com/looplj/axonhub/axon/task"
)

const (
	SystemTaskHeartbeatID   = "axonclaw-heartbeat"
	SystemTaskSelfReflectID = "axonclaw-self-reflect"
	SystemTaskSelfEvolveID  = "axonclaw-self-evolution"
	SystemTaskNameHeartbeat = "Heartbeat"
	SystemTaskNameReflect   = "Self Reflection"
	SystemTaskNameEvolution = "Self Evolution"
)

func EnsureSystemTasks(store *task.Store) error {
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
				Interval: DefaultHeartbeatSettings().Interval,
			},
			Action: map[string]any{
				"active_start":  DefaultHeartbeatSettings().ActiveStart,
				"active_end":    DefaultHeartbeatSettings().ActiveEnd,
				"timezone":      DefaultHeartbeatSettings().Timezone,
				"light_context": DefaultHeartbeatSettings().LightContext,
				"ack_max_chars": DefaultHeartbeatSettings().AckMaxChars,
			},
		},
		{
			ID:      SystemTaskSelfReflectID,
			Name:    SystemTaskNameReflect,
			Type:    string(TaskTypeSelfReflect),
			System:  true,
			Enabled: true,
			Hidden:  true,
			Trigger: task.Trigger{
				Type: task.TriggerTypeCron,
				Cron: "0 22 * * *",
			},
		},
		{
			ID:      SystemTaskSelfEvolveID,
			Name:    SystemTaskNameEvolution,
			Type:    string(TaskTypeSelfEvolve),
			System:  true,
			Enabled: true,
			Hidden:  true,
			Trigger: task.Trigger{
				Type: task.TriggerTypeCron,
				Cron: "0 23 * * *",
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
