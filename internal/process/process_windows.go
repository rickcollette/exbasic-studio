//go:build windows

// Package process configures background child processes for desktop use.
package process

import (
	"os/exec"
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
