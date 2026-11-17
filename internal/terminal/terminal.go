// Package terminal runs interactive shell sessions on a pseudo-terminal:
// ConPTY on Windows and a PTY on macOS and Linux.
package terminal

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"sync/atomic"
	"unicode/utf8"
)

// EventName is the frontend event channel for terminal sessions.
const EventName = "terminal"

// Event is one ordered update from a session.
type Event struct {
	ID       string `json:"id"`
	Seq      uint64 `json:"seq"`
	Type     string `json:"type"` // output | exit
	Data     string `json:"data,omitempty"`
	ExitCode int    `json:"exitCode"`
}

// Info describes an open session.
type Info struct {
	ID    string `json:"id"`
	Shell string `json:"shell"`
	Dir   string `json:"dir"`
	Cols  int    `json:"cols"`
	Rows  int    `json:"rows"`
}

// Options configure a new session.
type Options struct {
	Dir   string
	Shell string // empty selects the platform default
	// Args are passed to Shell. On Windows a .cmd or .bat program runs
	// through the command interpreter, so its arguments must not contain
	// characters cmd.exe treats specially; see CommandLine.
	Args []string
	Cols int
	Rows int
}

// pty is the platform-specific half of a session.
type pty interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Resize(cols, rows int) error
	// Wait blocks until the shell exits and returns its exit code.
	Wait() int
	// Close terminates the shell and everything it started.
	Close() error
}

// Session is one shell attached to a pseudo-terminal.
type Session struct {
	Info
	pty       pty
	seq       atomic.Uint64
	closeOnce sync.Once
	done      chan struct{}
}

// Manager owns every open session.
type Manager struct {
	sink     func(name string, data any)
	mu       sync.Mutex
	next     uint64
	sessions map[string]*Session
}

// MaxSessions bounds the number of simultaneously open terminals.
const MaxSessions = 8

func NewManager(sink func(name string, data any)) *Manager {
	return &Manager{sink: sink, sessions: make(map[string]*Session)}
}

func (m *Manager) SetSink(sink func(name string, data any)) {
	m.mu.Lock()
	m.sink = sink
	m.mu.Unlock()
}

// DefaultShell picks the user's interactive shell.
func DefaultShell() string {
	if runtime.GOOS == "windows" {
		for _, candidate := range []string{"pwsh.exe", "powershell.exe"} {
			if path, err := exec.LookPath(candidate); err == nil {
				return path
			}
		}
		if comspec := os.Getenv("COMSPEC"); comspec != "" {
			return comspec
		}
		return "cmd.exe"
	}
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}

func clampSize(cols, rows int) (int, int) {
	if cols < 2 {
		cols = 80
	}
	if rows < 2 {
		rows = 24
	}
	if cols > 500 {
		cols = 500
	}
	if rows > 300 {
		rows = 300
	}
	return cols, rows
}

// Open starts a shell in dir and begins streaming its output.
func (m *Manager) Open(options Options) (Info, error) {
	info, err := os.Stat(options.Dir)
	if err != nil || !info.IsDir() {
		return Info{}, fmt.Errorf("terminal working directory is unavailable: %s", options.Dir)
	}
	if options.Shell == "" {
		options.Shell = DefaultShell()
	}
	options.Cols, options.Rows = clampSize(options.Cols, options.Rows)

	m.mu.Lock()
	if len(m.sessions) >= MaxSessions {
		m.mu.Unlock()
		return Info{}, fmt.Errorf("at most %d terminals can be open", MaxSessions)
	}
	m.next++
	id := fmt.Sprintf("term-%d", m.next)
	m.mu.Unlock()

	device, err := startPTY(options)
	if err != nil {
		return Info{}, fmt.Errorf("start terminal: %w", err)
	}
	session := &Session{Info: Info{ID: id, Shell: options.Shell, Dir: options.Dir, Cols: options.Cols, Rows: options.Rows}, pty: device, done: make(chan struct{})}
	m.mu.Lock()
	m.sessions[id] = session
	m.mu.Unlock()
	go m.pump(session)
	return session.Info, nil
}

func (m *Manager) emit(session *Session, event Event) {
	event.ID, event.Seq = session.ID, session.seq.Add(1)
	m.mu.Lock()
	sink := m.sink
	m.mu.Unlock()
	if sink != nil {
		sink(EventName, event)
	}
}

// pump forwards output until the terminal closes, then reports the exit
// status exactly once and forgets the session.
func (m *Manager) pump(session *Session) {
	defer close(session.done)
	buffer := make([]byte, 16<<10)
	var pending []byte
	for {
		n, err := session.pty.Read(buffer)
		if n > 0 {
			pending = append(pending, buffer[:n]...)
			complete := completeUTF8(pending)
			if complete > 0 {
				m.emit(session, Event{Type: "output", Data: string(pending[:complete])})
				pending = append(pending[:0], pending[complete:]...)
			}
		}
		if err != nil {
			break
		}
	}
	if len(pending) > 0 {
		m.emit(session, Event{Type: "output", Data: string(pending)})
	}
	code := session.pty.Wait()
	session.closeOnce.Do(func() { _ = session.pty.Close() })
	m.mu.Lock()
	delete(m.sessions, session.ID)
	m.mu.Unlock()
	m.emit(session, Event{Type: "exit", ExitCode: code})
}

// completeUTF8 returns how many leading bytes can be emitted without
// splitting a multi-byte character across two events.
func completeUTF8(data []byte) int {
	end := len(data)
	for back := 1; back <= utf8.UTFMax && back <= len(data); back++ {
		b := data[len(data)-back]
		if utf8.RuneStart(b) {
			if b >= utf8.RuneSelf && !utf8.FullRune(data[len(data)-back:]) {
				end = len(data) - back
			}
			break
		}
	}
	return end
}

func (m *Manager) get(id string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session := m.sessions[id]
	if session == nil {
		return nil, errors.New("the terminal session has ended")
	}
	return session, nil
}

// Write sends keyboard input (including control characters such as Ctrl+C).
func (m *Manager) Write(id, data string) error {
	session, err := m.get(id)
	if err != nil {
		return err
	}
	if len(data) > 1<<20 {
		return errors.New("terminal input is too large")
	}
	_, err = session.pty.Write([]byte(data))
	return err
}

// Resize informs the shell of a new window size.
func (m *Manager) Resize(id string, cols, rows int) error {
	session, err := m.get(id)
	if err != nil {
		return err
	}
	cols, rows = clampSize(cols, rows)
	return session.pty.Resize(cols, rows)
}

// Close terminates a session and waits for its exit event to be delivered.
// Closing an already finished session is not an error.
func (m *Manager) Close(id string) error {
	session, err := m.get(id)
	if err != nil {
		return nil
	}
	var closeErr error
	session.closeOnce.Do(func() { closeErr = session.pty.Close() })
	<-session.done
	return closeErr
}

// List returns the open sessions.
func (m *Manager) List() []Info {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Info, 0, len(m.sessions))
	for _, session := range m.sessions {
		out = append(out, session.Info)
	}
	return out
}

// Shutdown closes every session.
func (m *Manager) Shutdown() {
	for _, info := range m.List() {
		_ = m.Close(info.ID)
	}
}
