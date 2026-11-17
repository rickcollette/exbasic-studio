package terminal

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

type capture struct {
	mu     sync.Mutex
	output strings.Builder
	seqs   []uint64
	exit   chan Event
}

func newCapture() *capture { return &capture{exit: make(chan Event, 4)} }

func (c *capture) sink(_ string, data any) {
	event := data.(Event)
	c.mu.Lock()
	c.seqs = append(c.seqs, event.Seq)
	if event.Type == "output" {
		c.output.WriteString(event.Data)
	}
	c.mu.Unlock()
	if event.Type == "exit" {
		c.exit <- event
	}
}

func (c *capture) text() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.output.String()
}

func (c *capture) waitFor(t *testing.T, fragment string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(c.text(), fragment) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("terminal never printed %q; output so far:\n%s", fragment, c.text())
}

func shellForTest() string {
	if runtime.GOOS == "windows" {
		return "cmd.exe"
	}
	return "/bin/sh"
}

func enter() string {
	if runtime.GOOS == "windows" {
		return "\r"
	}
	return "\n"
}

func TestSessionRunsCommandsKeepsDirectoryAndReportsExit(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "subdir-marker"), 0o755); err != nil {
		t.Fatal(err)
	}
	c := newCapture()
	m := NewManager(c.sink)
	info, err := m.Open(Options{Dir: dir, Shell: shellForTest(), Cols: 100, Rows: 30})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.List()) != 1 {
		t.Fatal("session not listed")
	}
	// The working directory persists between commands.
	if err = m.Write(info.ID, "cd subdir-marker"+enter()); err != nil {
		t.Fatal(err)
	}
	printDir := "pwd"
	if runtime.GOOS == "windows" {
		printDir = "cd"
	}
	if err = m.Write(info.ID, printDir+enter()); err != nil {
		t.Fatal(err)
	}
	c.waitFor(t, "subdir-marker")
	if err = m.Write(info.ID, "echo round-"+"trip"+enter()); err != nil {
		t.Fatal(err)
	}
	c.waitFor(t, "round-trip")
	if err = m.Resize(info.ID, 120, 40); err != nil {
		t.Fatal(err)
	}
	if err = m.Write(info.ID, "exit 7"+enter()); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-c.exit:
		if event.ExitCode != 7 {
			t.Fatalf("exit code = %d, want 7", event.ExitCode)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("no exit event")
	}
	c.mu.Lock()
	for index, seq := range c.seqs {
		if seq != uint64(index+1) {
			t.Fatalf("event %d has seq %d", index, seq)
		}
	}
	c.mu.Unlock()
	deadline := time.Now().Add(5 * time.Second)
	for len(m.List()) != 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if len(m.List()) != 0 {
		t.Fatal("finished session is still listed")
	}
	if err = m.Write(info.ID, "x"); err == nil {
		t.Fatal("write to a finished session succeeded")
	}
	if err = m.Close(info.ID); err != nil {
		t.Fatalf("closing a finished session failed: %v", err)
	}
}

func TestCtrlCInterruptsAndCloseKillsChildren(t *testing.T) {
	c := newCapture()
	m := NewManager(c.sink)
	info, err := m.Open(Options{Dir: t.TempDir(), Shell: shellForTest()})
	if err != nil {
		t.Fatal(err)
	}
	long := "sleep 600"
	if runtime.GOOS == "windows" {
		long = "ping -n 600 127.0.0.1"
	}
	if err = m.Write(info.ID, long+enter()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)
	if err = m.Write(info.ID, "\x03"); err != nil {
		t.Fatal(err)
	}
	// After the interrupt the shell accepts another command.
	time.Sleep(500 * time.Millisecond)
	if runtime.GOOS == "windows" {
		// cmd.exe may ask "Terminate batch job (Y/N)?" only for batch files; a
		// plain command returns straight to the prompt.
		_ = m.Write(info.ID, enter())
	}
	if err = m.Write(info.ID, "echo after-"+"interrupt"+enter()); err != nil {
		t.Fatal(err)
	}
	c.waitFor(t, "after-interrupt")

	// Close while a long command is running: it must return promptly.
	if err = m.Write(info.ID, long+enter()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	closed := make(chan error, 1)
	go func() { closed <- m.Close(info.ID) }()
	select {
	case err = <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("Close did not return while a child process was running")
	}
	select {
	case <-c.exit:
	case <-time.After(5 * time.Second):
		t.Fatal("no exit event after Close")
	}
}

func TestOpenValidatesDirectoryAndLimit(t *testing.T) {
	m := NewManager(nil)
	if _, err := m.Open(Options{Dir: filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("missing directory accepted")
	}
	dir := t.TempDir()
	for i := 0; i < MaxSessions; i++ {
		if _, err := m.Open(Options{Dir: dir, Shell: shellForTest()}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.Open(Options{Dir: dir, Shell: shellForTest()}); err == nil {
		t.Fatal("session limit not enforced")
	}
	m.Shutdown()
	if len(m.List()) != 0 {
		t.Fatal("Shutdown left sessions open")
	}
}

func TestCompleteUTF8HoldsBackPartialCharacters(t *testing.T) {
	euro := []byte("€") // three bytes
	cases := []struct {
		in   []byte
		want int
	}{
		{[]byte("abc"), 3},
		{append([]byte("ab"), euro...), 5},
		{append([]byte("ab"), euro[:1]...), 2},
		{append([]byte("ab"), euro[:2]...), 2},
		{[]byte{}, 0},
		{[]byte{0xff}, 1},
		{[]byte{'a', 0x80, 0x80, 0x80, 0x80, 0x80}, 6},
	}
	for _, tc := range cases {
		if got := completeUTF8(tc.in); got != tc.want {
			t.Errorf("completeUTF8(%v) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
