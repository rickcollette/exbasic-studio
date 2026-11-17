package studio

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"exbasicstudio/internal/terminal"
)

// A stand-in for the Claude Code CLI that prints its arguments. On Windows
// it is a batch file, the form npm installs, which runs through cmd.exe.
func fakeClaude(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, "claude.cmd")
		if err := os.WriteFile(path, []byte("@echo FAKE-CLAUDE %*\r\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	path := filepath.Join(dir, "claude")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho FAKE-CLAUDE \"$@\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestClaudeStatus(t *testing.T) {
	s, _ := newService(t)
	t.Setenv(claudeEnv, filepath.Join(t.TempDir(), "missing"))
	if status := s.ClaudeStatus(); status.Available || status.Install == "" {
		t.Fatalf("a missing CLI must be unavailable with an install hint: %#v", status)
	}
	fake := fakeClaude(t)
	t.Setenv(claudeEnv, fake)
	if status := s.ClaudeStatus(); !status.Available || status.Path != fake {
		t.Fatalf("explicit CLI not found: %#v", status)
	}
}

func TestClaudeOpenStartsClaudeWithTheExbasicContext(t *testing.T) {
	s, recorder := newService(t)
	t.Setenv(claudeEnv, fakeClaude(t))
	root := t.TempDir()
	if _, err := s.CreateProject(root, "game"); err != nil {
		t.Fatal(err)
	}
	info, err := s.ClaudeOpen(100, 30)
	if err != nil {
		t.Fatal(err)
	}
	var output string
	deadline := time.Now().Add(60 * time.Second)
	for !strings.Contains(output, "EXBASIC-STUDIO.md") && time.Now().Before(deadline) {
		var b strings.Builder
		recorder.mu.Lock()
		for _, item := range recorder.all {
			if event, ok := item.(terminal.Event); ok && event.ID == info.ID {
				b.WriteString(event.Data)
			}
		}
		recorder.mu.Unlock()
		output = b.String()
		time.Sleep(50 * time.Millisecond)
	}
	flat := strings.Join(strings.Fields(output), " ") // the terminal wraps long lines
	for _, want := range []string{"FAKE-CLAUDE", "--add-dir", "--append-system-prompt", "exBASIC"} {
		if !strings.Contains(flat, want) {
			t.Fatalf("Claude Code was not started with %q; output: %q", want, output)
		}
	}
	// The briefing lives in Studio's state folder, not in the project.
	matches, _ := filepath.Glob(filepath.Join(s.state.Dir(), "claude", "*", "EXBASIC-STUDIO.md"))
	if len(matches) != 1 {
		t.Fatalf("context file not written: %v", matches)
	}
	text, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"not Atari BASIC", "-o ", "STICK(0) is 15", filepath.Join(root, "game")} {
		if !strings.Contains(string(text), want) {
			t.Fatalf("context lacks %q:\n%s", want, text)
		}
	}
	if _, err = os.Stat(filepath.Join(root, "game", "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatal("Studio must not write into the project")
	}
}

func TestClaudeOpenNeedsTheCLI(t *testing.T) {
	s, _ := newService(t)
	t.Setenv(claudeEnv, filepath.Join(t.TempDir(), "missing"))
	if _, err := s.CreateProject(t.TempDir(), "game"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClaudeOpen(80, 24); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("expected a not-installed error, got %v", err)
	}
}
