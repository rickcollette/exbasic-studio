//go:build !windows

package terminal

import (
	"errors"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	creackpty "github.com/creack/pty"
)

// unixPTY hosts a shell as the leader of a new session on a pseudo-terminal.
type unixPTY struct {
	master    *os.File
	cmd       *exec.Cmd
	closeOnce sync.Once
	waitOnce  sync.Once
	exitCode  int
}

func startPTY(options Options) (pty, error) {
	cmd := exec.Command(options.Shell, options.Args...)
	cmd.Dir = options.Dir
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	// pty.StartWithSize makes the shell a session leader with the PTY as its
	// controlling terminal, so Ctrl+C and window-size changes behave normally.
	master, err := creackpty.StartWithSize(cmd, &creackpty.Winsize{Cols: uint16(options.Cols), Rows: uint16(options.Rows)})
	if err != nil {
		return nil, err
	}
	return &unixPTY{master: master, cmd: cmd}, nil
}

func (u *unixPTY) Read(p []byte) (int, error)  { return u.master.Read(p) }
func (u *unixPTY) Write(p []byte) (int, error) { return u.master.Write(p) }

func (u *unixPTY) Resize(cols, rows int) error {
	return creackpty.Setsize(u.master, &creackpty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

func (u *unixPTY) Wait() int {
	u.waitOnce.Do(func() {
		err := u.cmd.Wait()
		var exit *exec.ExitError
		switch {
		case err == nil:
			u.exitCode = 0
		case errors.As(err, &exit):
			u.exitCode = exit.ExitCode()
		default:
			u.exitCode = -1
		}
	})
	return u.exitCode
}

// Close hangs up the whole session, escalating to SIGKILL for processes that
// ignore the hangup.
func (u *unixPTY) Close() error {
	u.closeOnce.Do(func() {
		pid := u.cmd.Process.Pid
		_ = syscall.Kill(-pid, syscall.SIGHUP)
		exited := make(chan struct{})
		go func() { u.Wait(); close(exited) }()
		select {
		case <-exited:
		case <-time.After(2 * time.Second):
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			<-exited
		}
		_ = u.master.Close()
	})
	return nil
}
