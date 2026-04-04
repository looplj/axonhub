package cmds

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/looplj/axonhub/axon/task"
	"github.com/spf13/cobra"

	"github.com/looplj/axonhub/cmd/axonclaw/claw"
)

func NewHeartbeatCommand(opts StdioOptions) *cobra.Command {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	root := &cobra.Command{
		Use:   "heartbeat",
		Short: "Manage the agentic heartbeat system",
		Long: `Manage the agentic heartbeat system.

The heartbeat periodically wakes the agent to check a HEARTBEAT.md checklist
and handle anything that needs proactive attention.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)

	root.AddCommand(newHeartbeatStatusCmd(stdout))
	root.AddCommand(newHeartbeatEnableCmd(stdout, true))
	root.AddCommand(newHeartbeatEnableCmd(stdout, false))
	root.AddCommand(newHeartbeatIntervalCmd(stdout))
	root.AddCommand(newHeartbeatModelCmd(stdout))

	return root
}

func newHeartbeatStatusCmd(out *os.File) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show heartbeat status",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := newTaskStore()
			if err != nil {
				return err
			}

			t, err := store.Get(claw.SystemTaskHeartbeatID)
			if err != nil && !errors.Is(err, task.ErrTaskNotFound) {
				return err
			}

			status := "disabled"
			if t != nil && t.Enabled {
				status = "enabled"
			}

			settings := claw.DefaultHeartbeatSettings()
			if t != nil {
				settings = claw.HeartbeatSettingsFromTask(*t)
			}

			fmt.Fprintf(out, "Heartbeat: %s\n", status)
			fmt.Fprintf(out, "Interval: %s\n", settings.Interval)
			fmt.Fprintf(out, "Active Hours: %s - %s\n", settings.ActiveStart, settings.ActiveEnd)

			if settings.Timezone != "" {
				fmt.Fprintf(out, "Timezone: %s\n", settings.Timezone)
			} else {
				fmt.Fprintf(out, "Timezone: (local)\n")
			}

			if settings.Model != "" {
				fmt.Fprintf(out, "Model: %s\n", settings.Model)
			} else {
				fmt.Fprintf(out, "Model: (default)\n")
			}

			fmt.Fprintf(out, "Light Context: %v\n", settings.LightContext)
			fmt.Fprintf(out, "Ack Max Chars: %d\n", settings.AckMaxChars)
			fmt.Fprintf(out, "Task ID: %s\n", claw.SystemTaskHeartbeatID)

			return nil
		},
	}
}

func newHeartbeatEnableCmd(out *os.File, enable bool) *cobra.Command {
	use := "enable"
	short := "Enable heartbeat"

	if !enable {
		use = "disable"
		short = "Disable heartbeat"
	}

	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := newTaskStore()
			if err != nil {
				return err
			}

			if err := store.SetEnabled(claw.SystemTaskHeartbeatID, enable); err != nil {
				if errors.Is(err, task.ErrTaskNotFound) {
					return fmt.Errorf("heartbeat task %q not found", claw.SystemTaskHeartbeatID)
				}

				return err
			}

			if enable {
				fmt.Fprintln(out, "Heartbeat enabled.")
			} else {
				fmt.Fprintln(out, "Heartbeat disabled.")
			}

			return nil
		},
	}
}

func newHeartbeatModelCmd(out *os.File) *cobra.Command {
	return &cobra.Command{
		Use:   "model [model]",
		Short: "Show or set heartbeat model",
		Long: `Show or set the model used for heartbeat tasks.

Examples:
  axonclaw heartbeat model
  axonclaw heartbeat model claude-sonnet-4-20250514`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := newTaskStore()
			if err != nil {
				return err
			}

			t, err := store.Get(claw.SystemTaskHeartbeatID)
			if err != nil {
				return err
			}

			if len(args) == 0 {
				model := claw.HeartbeatSettingsFromTask(*t).Model
				if model == "" {
					model = "(default)"
				}

				fmt.Fprintln(out, model)

				return nil
			}

			value := strings.TrimSpace(args[0])
			if err := claw.ApplyHeartbeatSetting(t, "model", value); err != nil {
				return err
			}

			if err := store.Update(func(tasks []task.Task) ([]task.Task, bool, error) {
				for i := range tasks {
					if tasks[i].ID != claw.SystemTaskHeartbeatID {
						continue
					}

					t.Runtime = tasks[i].Runtime
					tasks[i] = *t

					return tasks, true, nil
				}

				return tasks, false, fmt.Errorf("heartbeat task %q not found", claw.SystemTaskHeartbeatID)
			}); err != nil {
				return err
			}

			fmt.Fprintf(out, "Heartbeat model set to %s.\n", value)

			return nil
		},
	}
}

func newHeartbeatIntervalCmd(out *os.File) *cobra.Command {
	return &cobra.Command{
		Use:   "interval [duration]",
		Short: "Show or set heartbeat interval",
		Long: `Show or set heartbeat interval.

Examples:
  axonclaw heartbeat interval
  axonclaw heartbeat interval 1h`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := newTaskStore()
			if err != nil {
				return err
			}

			t, err := store.Get(claw.SystemTaskHeartbeatID)
			if err != nil {
				return err
			}

			if len(args) == 0 {
				fmt.Fprintln(out, claw.HeartbeatSettingsFromTask(*t).Interval)
				return nil
			}

			value := strings.TrimSpace(args[0])
			if err := claw.ApplyHeartbeatSetting(t, "interval", value); err != nil {
				return err
			}

			if err := store.Update(func(tasks []task.Task) ([]task.Task, bool, error) {
				for i := range tasks {
					if tasks[i].ID != claw.SystemTaskHeartbeatID {
						continue
					}

					t.Runtime = tasks[i].Runtime
					tasks[i] = *t

					return tasks, true, nil
				}

				return tasks, false, fmt.Errorf("heartbeat task %q not found", claw.SystemTaskHeartbeatID)
			}); err != nil {
				return err
			}

			fmt.Fprintf(out, "Heartbeat interval set to %s.\n", value)

			return nil
		},
	}
}
