package cmds

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/looplj/axonhub/axon/task"

	"github.com/looplj/axonhub/cmd/axonclaw/conf"
)

type StdioOptions struct {
	Stdout *os.File
	Stderr *os.File
}

func newTaskStore() (*task.Store, error) {
	runtimeDir, err := conf.RuntimeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve runtime directory: %w", err)
	}

	return task.NewStore(filepath.Join(runtimeDir, "tasks"))
}
