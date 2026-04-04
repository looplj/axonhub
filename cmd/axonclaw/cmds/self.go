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

func NewSelfCommand(opts StdioOptions) *cobra.Command {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	root := &cobra.Command{
		Use:           "self",
		Short:         "Manage self-related agent tasks",
		Long:          `Manage self-related agent tasks such as self-evolution.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)

	root.AddCommand(newSelfEvolutionCmd(stdout))

	return root
}

func newSelfEvolutionCmd(out *os.File) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "evolution",
		Short: "Manage the self-evolution task",
		Long: `Manage the self-evolution task.

The self-evolution task periodically runs to reflect on recent work patterns,
identify repetitive tasks or useful workflows, and create skills to improve over time.`,
	}

	cmd.AddCommand(newSelfEvolutionStatusCmd(out))
	cmd.AddCommand(newSelfEvolutionEnableCmd(out, true))
	cmd.AddCommand(newSelfEvolutionEnableCmd(out, false))
	cmd.AddCommand(newSelfEvolutionModelCmd(out))

	return cmd
}

func newSelfEvolutionStatusCmd(out *os.File) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show self-evolution task status",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := newTaskStore()
			if err != nil {
				return err
			}

			t, err := store.Get(claw.SystemTaskSelfEvolveID)
			if err != nil && !errors.Is(err, task.ErrTaskNotFound) {
				return err
			}

			status := "disabled"
			if t != nil && t.Enabled {
				status = "enabled"
			}

			fmt.Fprintf(out, "Self Evolution: %s\n", status)

			if t != nil {
				if t.Trigger.Cron != "" {
					fmt.Fprintf(out, "Schedule: %s\n", t.Trigger.Cron)
				}

				model := actionString(t.Action, "model")
				if model != "" {
					fmt.Fprintf(out, "Model: %s\n", model)
				} else {
					fmt.Fprintf(out, "Model: (default)\n")
				}

				mode := actionString(t.Action, "mode")
				if mode == "" {
					mode = "isolated"
				}

				fmt.Fprintf(out, "Mode: %s\n", mode)
			}

			fmt.Fprintf(out, "Task ID: %s\n", claw.SystemTaskSelfEvolveID)

			return nil
		},
	}
}

func newSelfEvolutionEnableCmd(out *os.File, enable bool) *cobra.Command {
	use := "enable"
	short := "Enable self-evolution task"

	if !enable {
		use = "disable"
		short = "Disable self-evolution task"
	}

	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := newTaskStore()
			if err != nil {
				return err
			}

			if err := store.SetEnabled(claw.SystemTaskSelfEvolveID, enable); err != nil {
				if errors.Is(err, task.ErrTaskNotFound) {
					return fmt.Errorf("self-evolution task %q not found", claw.SystemTaskSelfEvolveID)
				}

				return err
			}

			if enable {
				fmt.Fprintln(out, "Self-evolution task enabled.")
			} else {
				fmt.Fprintln(out, "Self-evolution task disabled.")
			}

			return nil
		},
	}
}

func newSelfEvolutionModelCmd(out *os.File) *cobra.Command {
	return &cobra.Command{
		Use:   "model [model]",
		Short: "Show or set self-evolution model",
		Long: `Show or set the model used for self-evolution tasks.

Examples:
  axonclaw self evolution model
  axonclaw self evolution model claude-sonnet-4-20250514`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := newTaskStore()
			if err != nil {
				return err
			}

			t, err := store.Get(claw.SystemTaskSelfEvolveID)
			if err != nil {
				if errors.Is(err, task.ErrTaskNotFound) {
					return fmt.Errorf("self-evolution task %q not found", claw.SystemTaskSelfEvolveID)
				}

				return err
			}

			if len(args) == 0 {
				model := actionString(t.Action, "model")
				if model == "" {
					model = "(default)"
				}

				fmt.Fprintln(out, model)

				return nil
			}

			value := strings.TrimSpace(args[0])

			if t.Action == nil {
				t.Action = map[string]any{}
			}

			t.Action["model"] = value

			if err := store.Update(func(tasks []task.Task) ([]task.Task, bool, error) {
				for i := range tasks {
					if tasks[i].ID != claw.SystemTaskSelfEvolveID {
						continue
					}

					t.Runtime = tasks[i].Runtime
					tasks[i] = *t

					return tasks, true, nil
				}

				return tasks, false, fmt.Errorf("self-evolution task %q not found", claw.SystemTaskSelfEvolveID)
			}); err != nil {
				return err
			}

			fmt.Fprintf(out, "Self-evolution model set to %s.\n", value)

			return nil
		},
	}
}

func actionString(action map[string]any, key string) string {
	if action == nil {
		return ""
	}

	v, ok := action[key]
	if !ok || v == nil {
		return ""
	}

	s, ok := v.(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(s)
}
