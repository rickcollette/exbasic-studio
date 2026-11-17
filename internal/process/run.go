package process

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"sync"
	"time"
)

// Sink receives output chunks as they are produced.
type Sink func(stream, text string)

// MaxCapturedOutput bounds the output retained for the caller. Streaming is
// not limited; only the returned copy is truncated.
const MaxCapturedOutput = 4 << 20

// Command builds a cancellable background command. Cancelling ctx ends the
// whole process tree, not just the direct child, so a cancelled build does
// not leave an assembler or linker running.
func Command(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	prepareTree(cmd)
	cmd.Cancel = func() error { return killTree(cmd) }
	cmd.WaitDelay = 5 * time.Second
	return cmd
}

// Run executes cmd, streaming stdout and stderr to sink and returning the
// interleaved output. The command is never passed through a shell.
func Run(cmd *exec.Cmd, sink Sink) (string, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err = cmd.Start(); err != nil {
		return "", err
	}
	var (
		mu       sync.Mutex
		captured bytes.Buffer
		wg       sync.WaitGroup
	)
	pump := func(name string, reader io.Reader) {
		defer wg.Done()
		buffer := make([]byte, 8192)
		for {
			n, readErr := reader.Read(buffer)
			if n > 0 {
				chunk := string(buffer[:n])
				mu.Lock()
				if room := MaxCapturedOutput - captured.Len(); room > 0 {
					if n > room {
						captured.WriteString(chunk[:room])
						captured.WriteString("\n[output truncated]\n")
					} else {
						captured.WriteString(chunk)
					}
				}
				mu.Unlock()
				if sink != nil {
					sink(name, chunk)
				}
			}
			if readErr != nil {
				return
			}
		}
	}
	wg.Add(2)
	go pump("stdout", stdout)
	go pump("stderr", stderr)
	wg.Wait()
	err = cmd.Wait()
	return captured.String(), err
}
