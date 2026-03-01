//go:build windows

package biz

import (
	"os/exec"
)

func setProcessGroup(cmd *exec.Cmd) {
	// No-op for now, or use CreationFlags if needed
}
