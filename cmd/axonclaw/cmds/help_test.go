package cmds

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func noopRun(_ *cobra.Command, _ []string) {}

func TestHelpCommandSupportsNestedPath(t *testing.T) {
	root := &cobra.Command{Use: "axonclaw"}
	tasks := &cobra.Command{Use: "tasks", Short: "Manage tasks", Run: noopRun}
	add := &cobra.Command{Use: "add", Short: "Add task", Run: noopRun}
	tasks.AddCommand(add)
	root.AddCommand(tasks)

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetHelpCommand(NewHelpCommand(root))

	root.SetArgs([]string{"help", "tasks", "add"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Add task") {
		t.Fatalf("expected nested help output, got %q", got)
	}
}

func TestHelpCommandAllTargetsSelectedSubtree(t *testing.T) {
	root := &cobra.Command{Use: "axonclaw"}
	tasks := &cobra.Command{Use: "tasks", Short: "Manage tasks", Run: noopRun}
	add := &cobra.Command{Use: "add", Short: "Add task", Run: noopRun}
	del := &cobra.Command{Use: "delete", Short: "Delete task", Run: noopRun}
	tasks.AddCommand(add, del)
	root.AddCommand(tasks)

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetHelpCommand(NewHelpCommand(root))

	root.SetArgs([]string{"help", "tasks", "--all"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "tasks - Manage tasks") {
		t.Fatalf("expected subtree root in output, got %q", got)
	}

	if !strings.Contains(got, "add # Add task") {
		t.Fatalf("expected nested subcommand in output, got %q", got)
	}

	if strings.Contains(got, "axonclaw") {
		t.Fatalf("expected targeted subtree help, got %q", got)
	}
}
