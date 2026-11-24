// Package studio is the Wails-facing facade of exBASIC Studio. It holds the
// active workspace and delegates to focused services: project I/O, the
// operation manager (builds, runs, Git network), terminal sessions, the
// filesystem watcher, application state/recovery/trust, the emulator and
// the diagnostic log.
package studio

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"exbasicstudio/internal/appstate"
	"exbasicstudio/internal/emulator"
	"exbasicstudio/internal/logging"
	"exbasicstudio/internal/operations"
	"exbasicstudio/internal/project"
	"exbasicstudio/internal/terminal"
	"exbasicstudio/internal/tools"
	"exbasicstudio/internal/version"
	"exbasicstudio/internal/watch"
)

// Environment describes the running application to the frontend.
type Environment struct {
	Version        string `json:"version"`
	Platform       string `json:"platform"`
	WorkingDir     string `json:"workingDir"`
	CompilerHint   string `json:"compilerHint"`
	EmulatorHint   string `json:"emulatorHint"`
	ToolsAvailable bool   `json:"toolsAvailable"`
	ToolsNote      string `json:"toolsNote"`
}

// ClaudeCodeInfo describes the locally installed Claude Code CLI. exBASIC
// Studio uses the user's existing Claude login and never stores API keys or
// credentials itself.
type ClaudeCodeInfo struct {
	Available bool   `json:"available"`
	Path      string `json:"path"`
	Message   string `json:"message"`
}

// ClaudeCodeStatus finds Claude Code on the user's PATH.
func (s *StudioService) ClaudeCodeStatus() ClaudeCodeInfo {
	path, err := exec.LookPath("claude")
	if err != nil {
		return ClaudeCodeInfo{Message: "Claude Code was not found on PATH. Install it, then restart exBASIC Studio."}
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	return ClaudeCodeInfo{Available: true, Path: path, Message: "Claude Code is ready."}
}

// StudioService is bound to the frontend. Every exported method is callable
// from the UI, so each one validates its own arguments.
type StudioService struct {
	mu        sync.RWMutex
	eosSyncMu sync.Mutex
	// opening serializes folder switches so two overlapping requests cannot
	// interleave their teardown and activation.
	opening sync.Mutex
	active  *project.Project
	watcher *watch.Watcher

	state      *appstate.Store
	log        *logging.Logger
	operations *operations.Manager
	terminals  *terminal.Manager
	emit       func(name string, data any)

	emulator          *emulator.Session
	emulatorLaunch    emulator.Launch
	emulatorDiskMod   time.Time
	emulatorDiskSize  int64
	emulatorDiskCheck time.Time
	emulatorProfiles  *emulator.ProfileStore

	debug debugState
}

// New creates the service using the per-user configuration directory.
func New() *StudioService { return NewWithDir(appstate.DefaultDir()) }

// NewWithDir creates the service with all persistent state under dir. Tests
// use it to stay out of the real user profile.
func NewWithDir(dir string) *StudioService {
	s := &StudioService{
		state:            appstate.NewStore(dir),
		log:              logging.New(filepath.Join(dir, "logs")),
		emulatorProfiles: emulator.NewProfileStore(filepath.Join(dir, "atari800-profiles.json")),
	}
	s.operations = operations.NewManager(s.deliver)
	s.terminals = terminal.NewManager(s.deliver)
	s.log.Info("exBASIC Studio %s starting on %s/%s", version.Display(), runtime.GOOS, runtime.GOARCH)
	return s
}

// Attach connects backend events to the frontend. It is called once by the
// application bootstrap. It is a package function, not a method, so it is
// not exposed to the frontend as a binding.
func Attach(s *StudioService, emit func(name string, data any)) {
	s.mu.Lock()
	s.emit = emit
	s.mu.Unlock()
}

func (s *StudioService) deliver(name string, data any) {
	s.mu.RLock()
	emit := s.emit
	s.mu.RUnlock()
	if emit != nil {
		emit(name, data)
	}
}

// ServiceShutdown releases every child process when Wails exits.
func (s *StudioService) ServiceShutdown() error {
	s.operations.Shutdown()
	s.terminals.Shutdown()
	err := s.EmulatorStop()
	s.mu.Lock()
	watcher := s.watcher
	s.watcher = nil
	s.mu.Unlock()
	if watcher != nil {
		watcher.Stop()
	}
	if err != nil {
		s.log.Error("emulator shutdown: %v", err)
	}
	s.log.Info("exBASIC Studio stopped")
	s.log.Close()
	return err
}

func (s *StudioService) current() (*project.Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return nil, errors.New("open or create a project first")
	}
	return s.active, nil
}

// GetEnvironment reports versions and bundled tool availability.
func (s *StudioService) GetEnvironment() Environment {
	env := Environment{
		Version:      version.Display(),
		Platform:     runtime.GOOS + "/" + runtime.GOARCH,
		WorkingDir:   usefulWorkingDirectory(),
		CompilerHint: tools.Path("exbasic"),
		EmulatorHint: tools.Path("ex800-studio"),
	}
	missing := []string{}
	for _, tool := range []struct{ name, label string }{{"exbasic", "exBASIC compiler"}, {"ex800-studio", "ex800 helper"}, {"mkatr-go", "mkatr-go"}} {
		if _, ok := tools.Bundled(tool.name); !ok {
			missing = append(missing, tool.label)
		}
	}
	env.ToolsAvailable = len(missing) == 0
	if !env.ToolsAvailable {
		sort.Strings(missing)
		env.ToolsNote = "Bundled tools are missing (" + strings.Join(missing, ", ") + "). Reinstall exBASIC Studio, or configure tool paths in the project settings."
	}
	return env
}

// usefulWorkingDirectory suggests a starting folder for the open dialog. A
// packaged application's own directory is never a sensible workspace.
func usefulWorkingDirectory() string {
	wd, _ := os.Getwd()
	lower := strings.ToLower(wd)
	packaged := strings.Contains(lower, `\windowsapps\`) || strings.Contains(lower, `\program files`) ||
		strings.Contains(wd, ".app/Contents") || strings.HasPrefix(wd, "/usr/") || strings.HasPrefix(wd, "/opt/") || wd == "/"
	if executable, err := os.Executable(); err == nil && strings.EqualFold(filepath.Dir(executable), wd) {
		packaged = true
	}
	if packaged {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	return wd
}
