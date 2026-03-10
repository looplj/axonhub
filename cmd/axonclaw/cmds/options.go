package cmds

import "os"

type StdioOptions struct {
	Stdout *os.File
	Stderr *os.File
}
