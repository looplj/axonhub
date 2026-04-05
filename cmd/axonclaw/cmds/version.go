package cmds

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/looplj/axonhub/cmd/axonclaw/build"
)

func NewVersionCommand(opts StdioOptions) *cobra.Command {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  `Print the version, build time, and git commit information for axonclaw.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(stdout, "Version:    %s\n", build.GetVersion())
			fmt.Fprintf(stdout, "Build Time: %s\n", build.GetBuildTime())
			fmt.Fprintf(stdout, "Git Commit: %s\n", build.GetGitCommit())
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	return cmd
}
