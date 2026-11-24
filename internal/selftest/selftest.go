// Package selftest verifies an installed copy of exBASIC Studio without
// opening a window: it locates the bundled tools exactly as the application
// does, compiles and boots a sample, stops on a source breakpoint, packages
// an ATR image and runs a terminal session.
//
// Release CI runs `exbasic-studio --self-test` against every packaged
// artifact after installing it.
package selftest

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"exbasicstudio/internal/compiler"
	"exbasicstudio/internal/debugger"
	"exbasicstudio/internal/operations"
	"exbasicstudio/internal/studio"
	"exbasicstudio/internal/terminal"
	"exbasicstudio/internal/tools"
	"exbasicstudio/internal/version"
)

const program = "SCORE = 0\nFOR I = 1 TO 3\n  SCORE = SCORE + I\nNEXT I\nPRINT SCORE\nDO\nLOOP\n"

type recorder struct {
	mu     sync.Mutex
	events []any
}

func (r *recorder) emit(_ string, data any) {
	r.mu.Lock()
	r.events = append(r.events, data)
	r.mu.Unlock()
}

func (r *recorder) find(match func(any) bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, event := range r.events {
		if match(event) {
			return true
		}
	}
	return false
}

func waitFor(timeout time.Duration, tick func(), done func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if done() {
			return true
		}
		if tick != nil {
			tick()
		} else {
			time.Sleep(20 * time.Millisecond)
		}
	}
	return done()
}

// Run executes the checks, writing one line per step to out. It returns an
// error describing the first failure.
func Run(out io.Writer) error {
	step := func(format string, args ...any) { fmt.Fprintf(out, "ok   "+format+"\n", args...) }
	fmt.Fprintf(out, "exBASIC Studio %s self-test on %s/%s\n", version.Display(), runtime.GOOS, runtime.GOARCH)

	root, ok := tools.Root()
	if !ok {
		return errors.New("the bundled tools directory was not found next to the application")
	}
	for _, name := range []string{"exbasic", "ex800-studio", "mkatr-go"} {
		if _, found := tools.Bundled(name); !found {
			return fmt.Errorf("bundled tool %q is missing from %s", name, root)
		}
	}
	if _, found := tools.Asset("EOS.car"); !found {
		return fmt.Errorf("EOS.car is missing from %s", root)
	}
	for _, license := range []string{"ex800-COPYING.txt", "exbasic-LICENSE.txt", "mkatr-go-LICENSE.txt"} {
		if _, err := os.Stat(filepath.Join(root, "licenses", license)); err != nil {
			return fmt.Errorf("license notice %s is not shipped with the tools", license)
		}
	}
	step("bundled tools and license notices found in %s", root)

	work, err := os.MkdirTemp("", "exbasic-selftest-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	events := &recorder{}
	service := studio.NewWithDir(filepath.Join(work, "state"))
	studio.Attach(service, events.emit)
	defer func() { _ = service.ServiceShutdown() }()

	view, err := service.CreateProject(work, "selftest")
	if err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	cfg := view.Config
	cfg.ATR.Enabled = true
	if _, err = service.UpdateConfig(cfg); err != nil {
		return fmt.Errorf("configure project: %w", err)
	}
	if err = service.SaveFile("src/main.bas", program); err != nil {
		return err
	}

	// Compile, with structured results and ATR packaging.
	info, err := service.StartBuild(studio.BuildRequest{})
	if err != nil {
		return fmt.Errorf("start build: %w", err)
	}
	var result compiler.Result
	built := waitFor(2*time.Minute, nil, func() bool {
		return events.find(func(event any) bool {
			completed, isOperation := event.(operations.Event)
			if !isOperation || completed.ID != info.ID || completed.Type != "completed" {
				return false
			}
			result, _ = completed.Result.(compiler.Result)
			return true
		})
	})
	if !built || !result.Success {
		return fmt.Errorf("the sample did not compile:\n%s", result.Output)
	}
	if !result.DebugAvailable {
		return fmt.Errorf("the bundled compiler produced no debug metadata: %s", result.DebugNote)
	}
	if result.Artifact("disk-image") == "" {
		return errors.New("the ATR image was not packaged")
	}
	step("compiled the sample with debug metadata and packaged an ATR image")

	atr, err := service.ATROpen(result.Artifact("disk-image"))
	if err != nil || len(atr.Entries) == 0 {
		return fmt.Errorf("read the packaged ATR image: %v", err)
	}
	step("read the ATR image (%d entries, %s)", len(atr.Entries), atr.Filesystem)

	// Boot under the debugger and stop on a BASIC source line.
	profiles, err := service.EmulatorProfiles()
	if err != nil || len(profiles) == 0 {
		return fmt.Errorf("load emulator profiles: %v", err)
	}
	if _, err = service.DebugStart(studio.DebugRequest{ProfileID: profiles[0].ID, Breakpoints: []debugger.Breakpoint{{Path: "src/main.bas", Line: 3, Enabled: true}}}); err != nil {
		return fmt.Errorf("start the debugger: %w", err)
	}
	var frameErr error
	var stop *studio.DebugStop
	stopped := waitFor(time.Minute, func() {
		if _, frameErr = service.EmulatorFrame(); frameErr != nil {
			time.Sleep(10 * time.Millisecond)
		}
	}, func() bool {
		return frameErr != nil || events.find(func(event any) bool {
			debugEvent, isDebug := event.(studio.DebugEvent)
			if isDebug && debugEvent.Type == "stopped" {
				stop = debugEvent.Stop
				return true
			}
			return false
		})
	})
	if frameErr != nil {
		return fmt.Errorf("the emulator helper failed: %w", frameErr)
	}
	if !stopped || stop == nil || stop.Path != "src/main.bas" || stop.Line != 3 {
		return fmt.Errorf("the debugger did not stop on the breakpoint (stop: %+v)", stop)
	}
	values, err := service.DebugEvaluate([]string{"SCORE"})
	if err != nil || len(values) != 1 || values[0].Error != "" {
		return fmt.Errorf("evaluate a symbol: %v %+v", err, values)
	}
	step("booted the sample and stopped at src/main.bas:3 (SCORE = %s)", values[0].Value)
	if err = service.DebugStop(); err != nil {
		return fmt.Errorf("stop the emulator: %w", err)
	}
	step("stopped the emulator helper")

	// Interactive terminal session.
	terminalInfo, err := service.TerminalOpen(80, 24)
	if err != nil {
		return fmt.Errorf("open a terminal: %w", err)
	}
	newline := "\n"
	if runtime.GOOS == "windows" {
		newline = "\r"
	}
	marker := "selftest-" + "terminal"
	if err = service.TerminalWrite(terminalInfo.ID, "echo "+marker+newline); err != nil {
		return err
	}
	echoed := waitFor(time.Minute, nil, func() bool {
		var output strings.Builder
		events.find(func(event any) bool {
			if terminalEvent, isTerminal := event.(terminal.Event); isTerminal && terminalEvent.ID == terminalInfo.ID {
				output.WriteString(terminalEvent.Data)
			}
			return false
		})
		return strings.Contains(output.String(), marker)
	})
	if !echoed {
		return errors.New("the terminal session produced no output")
	}
	if err = service.TerminalClose(terminalInfo.ID); err != nil {
		return err
	}
	step("ran a command in a terminal session (%s)", filepath.Base(terminalInfo.Shell))

	// Git is the user's own installation; report rather than require it.
	if _, lookErr := exec.LookPath("git"); lookErr == nil {
		if _, err = service.GitInit(); err != nil {
			return fmt.Errorf("git init: %w", err)
		}
		status, statusErr := service.GitStatus()
		if statusErr != nil || !status.Available || len(status.Changes) == 0 {
			return fmt.Errorf("git status: %v %+v", statusErr, status)
		}
		step("initialized a Git repository (%d changes)", len(status.Changes))
	} else {
		fmt.Fprintln(out, "skip Git is not installed; source control features need it")
	}

	fmt.Fprintln(out, "PASS")
	return nil
}
