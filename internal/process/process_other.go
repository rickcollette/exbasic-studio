//go:build !windows

// Package process configures and runs background child processes.
package process

import (
	"os/exec"
	"syscall"
)

// Background is a no-op when child processes do not get a separate console.
func Background(cmd *exec.Cmd) *exec.Cmd { return cmd }

// prepareTree puts the child in its own process group so the whole group
// can be signalled.
func prepareTree(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killTree ends a process and every process it started.
func killTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}
