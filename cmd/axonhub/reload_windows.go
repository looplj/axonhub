//go:build windows

package main

import "errors"

func reloadRunningServer(pidFile string) error {
	return errors.New("reload is not available on Windows — use a Unix-like OS and send SIGHUP manually")
}
