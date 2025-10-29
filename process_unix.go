//go:build !windows

package main

import (
	"os/exec"
	"syscall"
	"time"
)

func prepareCommand(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminateCommand(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil {
		done := make(chan struct{})
		go func() {
			cmd.Wait()
			close(done)
		}()

		syscall.Kill(-pgid, syscall.SIGTERM)

		select {
		case <-done:
			return
		case <-time.After(2 * time.Second):
			syscall.Kill(-pgid, syscall.SIGKILL)
			<-done
			return
		}
	}

	cmd.Process.Kill()
	cmd.Wait()
}
