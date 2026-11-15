//go:build !windows

// Package process configures background child processes for desktop use.
package process

import "os/exec"

// Background is a no-op when child processes do not get a separate console.
func Background(cmd *exec.Cmd) *exec.Cmd { return cmd }
