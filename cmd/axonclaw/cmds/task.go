package cmds

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/looplj/axonhub/axon/task"
	"github.com/spf13/cobra"
)

type TaskOptions struct {
	Dir    string
	Stdout *os.File
	Stderr *os.File
}

func NewTaskCommand(opts TaskOptions) *cobra.Command {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	var dir string
	defaultDir := opts.Dir
	if defaultDir == "" {
		defaultDir = ".axonclaw"
	}

	var store *task.Store
	root := &cobra.Command{
		Use:   "tasks",
		Short: "Manage local scheduled tasks",
		Long: `Manage local scheduled tasks that can trigger actions at specific times.

Tasks are stored locally and can send messages to the agent when triggered.
This is useful for reminders, periodic checks, or scheduled notifications.

Available Commands:
  list     List all tasks
  get      Get details of a specific task
  add      Add a new scheduled task
  delete   Delete a task
  enable   Enable a disabled task
  disable  Disable an enabled task

Trigger Types:
  cron     - Cron expression (e.g., "0 9 * * *" for daily at 9:00)
  interval - Duration interval (e.g., "10m", "1h", "30s")
  at       - One-time execution at specific RFC3339 time
  delay    - One-time execution after a delay (e.g., "10m", "1h", "30s")

Action Types:
  send_agent_message - Send a message to the agent when triggered
    Required field: message (string)

Examples:
  # Add a daily reminder at 9:00 AM
  axonclaw tasks add --id daily-reminder --name "Daily Standup" \
    --trigger-type cron --cron "0 9 * * *" \
    --action '{"type":"send_agent_message","message":"Time for daily standup!"}'

  # Add a task that runs every 30 minutes
  axonclaw tasks add --id periodic-check --name "Periodic Check" \
    --trigger-type interval --interval "30m" \
    --action '{"type":"send_agent_message","message":"Check your progress!"}'

  # Add a one-time reminder
  axonclaw tasks add --id one-time --name "Meeting Reminder" \
    --trigger-type at --at "2024-01-15T14:30:00Z" \
    --action '{"type":"send_agent_message","message":"Meeting starts in 5 minutes"}'

  # Add a task that runs after a delay
  axonclaw tasks add --id delayed-task --name "Delayed Notification" \
    --trigger-type delay --delay "10m" \
    --action '{"type":"send_agent_message","message":"10 minutes have passed!"}'

  # List all tasks
  axonclaw tasks list

  # Disable a task
  axonclaw tasks disable daily-reminder

  # Enable a task
  axonclaw tasks enable daily-reminder

  # Delete a task
  axonclaw tasks delete daily-reminder
`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			s, err := task.NewStore(dir)
			if err != nil {
				return err
			}
			store = s
			return nil
		},
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().StringVar(&dir, "dir", defaultDir, "Task store directory")

	storeGetter := func() *task.Store { return store }
	root.AddCommand(newTaskListCmd(stdout, storeGetter))
	root.AddCommand(newTaskGetCmd(stdout, storeGetter))
	root.AddCommand(newTaskAddCmd(stdout, storeGetter))
	root.AddCommand(newTaskDeleteCmd(stdout, storeGetter))
	root.AddCommand(newTaskEnableCmd(stdout, storeGetter, true))
	root.AddCommand(newTaskEnableCmd(stdout, storeGetter, false))

	return root
}

func newTaskListCmd(out *os.File, store func() *task.Store) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			tasks, err := store().Load()
			if err != nil {
				return err
			}
			if len(tasks) == 0 {
				fmt.Fprintln(out, "No tasks found.")
				return nil
			}
			for _, t := range tasks {
				fmt.Fprintf(out, "%s\t%s\tenabled=%v\ttrigger=%s\tnext=%s\tstatus=%s\n",
					t.ID, t.Name, t.Enabled, t.Trigger.Type, t.Runtime.NextRunAt, t.Runtime.LastStatus)
			}
			return nil
		},
	}
}

func newTaskGetCmd(out *os.File, store func() *task.Store) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get one task as JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := store().Get(args[0])
			if err != nil {
				if errors.Is(err, task.ErrTaskNotFound) {
					return fmt.Errorf("task %q not found", args[0])
				}
				return err
			}
			data, err := json.MarshalIndent(t, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(out, string(data))
			return nil
		},
	}
}

func newTaskAddCmd(out *os.File, store func() *task.Store) *cobra.Command {
	var (
		id        string
		name      string
		enabled   bool
		trigType  string
		cronExpr  string
		interval  string
		at        string
		delay     string
		timezone  string
		actionRaw string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a task",
		Long: `Add a new scheduled task.

Required flags:
  --id           Unique task identifier
  --trigger-type Type of trigger (cron|interval|at|delay)
  --action       Action JSON with type and parameters

Trigger-specific flags:
  --cron     Cron expression (required for --trigger-type=cron)
  --interval Duration string like "10m", "1h" (required for --trigger-type=interval)
  --at       RFC3339 timestamp (required for --trigger-type=at)
  --delay    Duration string like "10m", "1h" (required for --trigger-type=delay)

Optional flags:
  --name     Human-readable task name
  --enabled  Enable task immediately (default: true)
  --timezone IANA timezone for schedule (e.g., "Asia/Shanghai")

Action format:
  {"type":"send_agent_message","message":"your message here"}

Examples:
  axonclaw tasks add --id my-task --trigger-type cron --cron "0 9 * * *" \
    --action '{"type":"send_agent_message","message":"Good morning!"}'
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id = strings.TrimSpace(id)
			if id == "" {
				return fmt.Errorf("--id is required")
			}
			if strings.TrimSpace(actionRaw) == "" {
				return fmt.Errorf("--action is required")
			}

			var action map[string]any
			if err := json.Unmarshal([]byte(actionRaw), &action); err != nil {
				return fmt.Errorf("invalid --action json: %w", err)
			}
			if _, ok := action["type"].(string); !ok {
				return fmt.Errorf("action.type is required and must be string")
			}

			trigger := task.Trigger{
				Type:     task.TriggerType(strings.TrimSpace(trigType)),
				Cron:     strings.TrimSpace(cronExpr),
				Interval: strings.TrimSpace(interval),
				At:       strings.TrimSpace(at),
				Delay:    strings.TrimSpace(delay),
				Timezone: strings.TrimSpace(timezone),
			}
			if err := validateTrigger(trigger); err != nil {
				return err
			}

			t := task.Task{
				ID:      id,
				Name:    strings.TrimSpace(name),
				Enabled: enabled,
				Trigger: trigger,
				Action:  action,
			}
			if err := store().Add(t); err != nil {
				if errors.Is(err, task.ErrTaskExists) {
					return fmt.Errorf("task %q already exists", id)
				}
				return err
			}
			fmt.Fprintf(out, "task added: %s\n", id)
			return nil
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Task ID")
	cmd.Flags().StringVar(&name, "name", "", "Task name")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "Enable task")
	cmd.Flags().StringVar(&trigType, "trigger-type", "", "Trigger type: cron|interval|at|delay")
	cmd.Flags().StringVar(&cronExpr, "cron", "", "Cron expression (for trigger-type=cron)")
	cmd.Flags().StringVar(&interval, "interval", "", "Duration, e.g. 10m (for trigger-type=interval)")
	cmd.Flags().StringVar(&at, "at", "", "RFC3339 time (for trigger-type=at)")
	cmd.Flags().StringVar(&delay, "delay", "", "Duration, e.g. 10m (for trigger-type=delay)")
	cmd.Flags().StringVar(&timezone, "timezone", "", "IANA timezone, e.g. Asia/Shanghai")
	cmd.Flags().StringVar(&actionRaw, "action", "", `Action JSON, e.g. {"type":"send_agent_message","message":"hi"}`)
	return cmd
}

func newTaskDeleteCmd(out *os.File, store func() *task.Store) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := store().Delete(args[0]); err != nil {
				if errors.Is(err, task.ErrTaskNotFound) {
					return fmt.Errorf("task %q not found", args[0])
				}
				return err
			}
			fmt.Fprintf(out, "task deleted: %s\n", args[0])
			return nil
		},
	}
}

func newTaskEnableCmd(out *os.File, store func() *task.Store, enabled bool) *cobra.Command {
	use := "enable"
	short := "Enable a task"
	if !enabled {
		use = "disable"
		short = "Disable a task"
	}
	return &cobra.Command{
		Use:   use + " <id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := store().SetEnabled(args[0], enabled); err != nil {
				if errors.Is(err, task.ErrTaskNotFound) {
					return fmt.Errorf("task %q not found", args[0])
				}
				return err
			}
			fmt.Fprintf(out, "task %s: %s\n", use, args[0])
			return nil
		},
	}
}

func validateTrigger(trigger task.Trigger) error {
	switch trigger.Type {
	case task.TriggerTypeCron:
		if strings.TrimSpace(trigger.Cron) == "" {
			return fmt.Errorf("--cron is required when --trigger-type=cron")
		}
	case task.TriggerTypeInterval:
		if strings.TrimSpace(trigger.Interval) == "" {
			return fmt.Errorf("--interval is required when --trigger-type=interval")
		}
		if _, err := time.ParseDuration(trigger.Interval); err != nil {
			return fmt.Errorf("invalid --interval: %w", err)
		}
	case task.TriggerTypeAt:
		if strings.TrimSpace(trigger.At) == "" {
			return fmt.Errorf("--at is required when --trigger-type=at")
		}
		if _, err := time.Parse(time.RFC3339, trigger.At); err != nil {
			return fmt.Errorf("invalid --at: %w", err)
		}
	case task.TriggerTypeDelay:
		if strings.TrimSpace(trigger.Delay) == "" {
			return fmt.Errorf("--delay is required when --trigger-type=delay")
		}
		if _, err := time.ParseDuration(trigger.Delay); err != nil {
			return fmt.Errorf("invalid --delay: %w", err)
		}
	default:
		return fmt.Errorf("--trigger-type must be one of cron|interval|at|delay")
	}

	if strings.TrimSpace(trigger.Timezone) != "" {
		if _, err := time.LoadLocation(trigger.Timezone); err != nil {
			return fmt.Errorf("invalid --timezone: %w", err)
		}
	}
	return nil
}
