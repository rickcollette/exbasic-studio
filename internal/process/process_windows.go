//go:build windows

// Package process configures and runs background child processes.
package process

import (
	"os/exec"
	"strconv"
	"syscall"
)

// CREATE_NO_WINDOW prevents command-line tools from flashing a console.
const createNoWindow uint32 = 0x08000000

// Background prevents command-line tools from opening a console window.
// Do not use it for GUI programs whose own window should be displayed.
func Background(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	return cmd
}

// killTree ends a process and every process it started.
func killTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	killer := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
	Background(killer)
	if err := killer.Run(); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}

func prepareTree(cmd *exec.Cmd) { Background(cmd) }
