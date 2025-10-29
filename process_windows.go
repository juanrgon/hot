//go:build windows

package main

import "os/exec"

func prepareCommand(cmd *exec.Cmd) {}

func terminateCommand(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	cmd.Process.Kill()
	cmd.Wait()
}
