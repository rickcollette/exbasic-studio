package studio

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"

	"exbasicstudio/internal/compiler"
	"exbasicstudio/internal/debugger"
	"exbasicstudio/internal/emulator"
)

// DebugEventName is the frontend event channel for debugger updates.
const DebugEventName = "debugger"

// DebugRequest starts a source-level debugging session for the last build.
type DebugRequest struct {
	ProfileID   string                `json:"profileId"`
	Breakpoints []debugger.Breakpoint `json:"breakpoints"`
}

// DebugStop describes where and why execution halted.
type DebugStop struct {
	Reason      string                 `json:"reason"`
	Path        string                 `json:"path"`
	Line        int                    `json:"line"`
	Location    int                    `json:"location"`
	Bytecode    bool                   `json:"bytecode"`
	Registers   emulator.DebugState    `json:"registers"`
	Frames      []debugger.StackFrame  `json:"frames"`
	Disassembly []debugger.Instruction `json:"disassembly"`
}

// DebugEvent is an ordered debugger update.
type DebugEvent struct {
	Seq  uint64     `json:"seq"`
	Type string     `json:"type"` // started | running | stopped | exited
	Stop *DebugStop `json:"stop,omitempty"`
	// Message explains an exit caused by a failure.
	Message string `json:"message,omitempty"`
}

// DebugSession is the answer to DebugStart.
type DebugSession struct {
	Breakpoints []debugger.ResolvedBreakpoint `json:"breakpoints"`
	Symbols     []debugger.Symbol             `json:"symbols"`
	Emulator    emulator.State                `json:"emulator"`
}

// DebugSnapshot lets a reloaded frontend recover the session state.
type DebugSnapshot struct {
	Active  bool       `json:"active"`
	Stopped bool       `json:"stopped"`
	Stop    *DebugStop `json:"stop,omitempty"`
}

// MemoryBlock is a bounded read of emulated memory.
type MemoryBlock struct {
	Address int    `json:"address"`
	Data    string `json:"data"` // hexadecimal
}

type debugState struct {
	mu          sync.Mutex
	session     *emulator.Session
	sourceMap   *debugger.SourceMap
	breakpoints []debugger.Breakpoint
	stopped     bool
	stop        *DebugStop
	seq         uint64
}

func (s *StudioService) emitDebug(event DebugEvent) {
	s.debug.mu.Lock()
	s.debug.seq++
	event.Seq = s.debug.seq
	s.debug.mu.Unlock()
	s.deliver(DebugEventName, event)
}

// debugSession returns the active debug session and its source map.
func (s *StudioService) debugSession() (*emulator.Session, *debugger.SourceMap, error) {
	s.debug.mu.Lock()
	session, sourceMap := s.debug.session, s.debug.sourceMap
	s.debug.mu.Unlock()
	s.mu.RLock()
	current := s.emulator
	s.mu.RUnlock()
	if session == nil || session != current {
		return nil, nil, errors.New("no debug session is active")
	}
	return session, sourceMap, nil
}

// DebugStart boots the last build under the debugger. The build must carry
// debug metadata whose source hashes still match the files on disk.
func (s *StudioService) DebugStart(request DebugRequest) (DebugSession, error) {
	p, err := s.current()
	if err != nil {
		return DebugSession{}, err
	}
	if err = s.requireTrust(p, "debug it"); err != nil {
		return DebugSession{}, err
	}
	output, err := p.Resolve(p.Config.Output)
	if err != nil {
		return DebugSession{}, err
	}
	if _, statErr := os.Stat(output); statErr != nil {
		return DebugSession{}, errors.New("build the project before debugging")
	}
	meta, err := compiler.LoadMetadata(compiler.MetadataFile(output))
	if err != nil {
		return DebugSession{}, errors.New("this build has no debug metadata. Source-level debugging needs the bundled exBASIC compiler (or one that supports --metadata-json); build again with it")
	}
	sourceMap, err := debugger.NewSourceMap(p.Root, meta)
	if err != nil {
		return DebugSession{}, err
	}
	profile, err := s.emulatorProfiles.Find(request.ProfileID)
	if err != nil {
		return DebugSession{}, err
	}
	// The executable is loaded directly, so no loader or DOS code runs at the
	// program's addresses before it starts. The exception is an EOS profile:
	// there the program goes on a disk and is started at the EOS prompt after
	// the breakpoints are in place.
	build := compiler.Result{Artifacts: []compiler.Artifact{{Kind: "executable", Path: output}}}
	launch, err := s.launchFor(p, profile, build)
	if err != nil {
		return DebugSession{}, err
	}
	if err = s.startEmulator(launch); err != nil {
		return DebugSession{}, err
	}
	s.mu.RLock()
	session := s.emulator
	s.mu.RUnlock()
	if session == nil {
		return DebugSession{}, errors.New("the emulator stopped while starting the debugger")
	}
	resolved, addresses := sourceMap.ResolveAll(request.Breakpoints)
	setup := func() error {
		if err := session.SetInterpreter(sourceMap.Dispatch, sourceMap.Pointer); err != nil {
			return err
		}
		if err := session.SetLineStarts(sourceMap.LineStarts()); err != nil {
			return err
		}
		return session.SetBreakpoints(addresses)
	}
	if err = setup(); err != nil {
		_ = s.EmulatorStop()
		return DebugSession{}, fmt.Errorf("configure the debugger: %w", err)
	}
	s.mu.RLock()
	program := s.emulatorLaunch.Program
	s.mu.RUnlock()
	if program != "" && !startProgram(session, program) {
		s.log.Warn("could not start %s at the EOS prompt", program)
	}
	s.debug.mu.Lock()
	s.debug.session, s.debug.sourceMap = session, sourceMap
	s.debug.breakpoints = append([]debugger.Breakpoint{}, request.Breakpoints...)
	s.debug.stopped, s.debug.stop = false, nil
	s.debug.mu.Unlock()
	s.log.Info("debug session started with %d breakpoints", len(addresses))
	s.emitDebug(DebugEvent{Type: "started"})
	return DebugSession{Breakpoints: resolved, Symbols: sourceMap.Symbols(), Emulator: s.EmulatorStatus()}, nil
}

// describeStop gathers everything the UI shows when execution halts.
func (s *StudioService) describeStop(session *emulator.Session, sourceMap *debugger.SourceMap, state emulator.DebugState) *DebugStop {
	stop := &DebugStop{Reason: state.Reason, Location: state.Location, Bytecode: state.Bytecode, Registers: state, Frames: []debugger.StackFrame{}, Disassembly: []debugger.Instruction{}}
	if path, line, ok := sourceMap.Locate(state.Location); ok {
		stop.Path, stop.Line = path, line
	}
	if frames, err := debugger.Frames(session, state.S, sourceMap); err == nil {
		stop.Frames = frames
	}
	length := 48
	if state.PC+length > 0x10000 {
		length = 0x10000 - state.PC
	}
	if code, err := session.ReadMemory(state.PC, length); err == nil {
		stop.Disassembly = debugger.Disassemble(code, state.PC, 12)
	}
	return stop
}

// observeDebug is called with every frame of a debug session. It reports
// transitions between running and stopped exactly once each.
func (s *StudioService) observeDebug(session *emulator.Session, state *emulator.DebugState) {
	s.debug.mu.Lock()
	if s.debug.session != session || state == nil {
		s.debug.mu.Unlock()
		return
	}
	wasStopped, sourceMap := s.debug.stopped, s.debug.sourceMap
	s.debug.mu.Unlock()
	switch {
	case state.Stopped() && !wasStopped:
		stop := s.describeStop(session, sourceMap, *state)
		s.debug.mu.Lock()
		s.debug.stopped, s.debug.stop = true, stop
		s.debug.mu.Unlock()
		s.emitDebug(DebugEvent{Type: "stopped", Stop: stop})
	case !state.Stopped() && wasStopped:
		s.debug.mu.Lock()
		s.debug.stopped, s.debug.stop = false, nil
		s.debug.mu.Unlock()
		s.emitDebug(DebugEvent{Type: "running"})
	}
}

// endDebug closes the debug session of an emulator session that stopped.
// A helper failure is reported as an exit with the reason, so the UI leaves
// debugging cleanly and the user can start again.
func (s *StudioService) endDebug(session *emulator.Session, message string) {
	s.debug.mu.Lock()
	active := s.debug.session != nil && s.debug.session == session
	if active {
		s.debug.session, s.debug.sourceMap = nil, nil
		s.debug.stopped, s.debug.stop = false, nil
	}
	s.debug.mu.Unlock()
	if active {
		s.emitDebug(DebugEvent{Type: "exited", Message: message})
	}
}

// DebugCommand controls execution: continue, pause, stepInstruction,
// stepLine, stepOver or stepOut.
func (s *StudioService) DebugCommand(command string) error {
	session, _, err := s.debugSession()
	if err != nil {
		return err
	}
	if command == "pause" {
		return session.Pause()
	}
	if err = session.Resume(command); err != nil {
		return err
	}
	s.debug.mu.Lock()
	changed := s.debug.stopped
	s.debug.stopped, s.debug.stop = false, nil
	s.debug.mu.Unlock()
	if changed {
		s.emitDebug(DebugEvent{Type: "running"})
	}
	return nil
}

// DebugRunToLine continues until execution reaches a source line.
func (s *StudioService) DebugRunToLine(path string, line int) error {
	session, sourceMap, err := s.debugSession()
	if err != nil {
		return err
	}
	resolved := sourceMap.Resolve(debugger.Breakpoint{Path: path, Line: line, Enabled: true})
	if !resolved.Verified || len(resolved.Addresses) == 0 {
		return fmt.Errorf("%s:%d has no executable code", path, line)
	}
	if err = session.RunUntil(resolved.Addresses[0]); err != nil {
		return err
	}
	s.debug.mu.Lock()
	changed := s.debug.stopped
	s.debug.stopped, s.debug.stop = false, nil
	s.debug.mu.Unlock()
	if changed {
		s.emitDebug(DebugEvent{Type: "running"})
	}
	return nil
}

// DebugSetBreakpoints replaces the breakpoint set and reports where each
// breakpoint was placed. Without a session the breakpoints are returned
// unverified; they are bound when debugging starts.
func (s *StudioService) DebugSetBreakpoints(breakpoints []debugger.Breakpoint) ([]debugger.ResolvedBreakpoint, error) {
	if len(breakpoints) > 500 {
		return nil, errors.New("too many breakpoints")
	}
	session, sourceMap, err := s.debugSession()
	if err != nil {
		return debugger.Unresolved(breakpoints), nil
	}
	resolved, addresses := sourceMap.ResolveAll(breakpoints)
	if err = session.SetBreakpoints(addresses); err != nil {
		return nil, err
	}
	s.debug.mu.Lock()
	s.debug.breakpoints = append([]debugger.Breakpoint{}, breakpoints...)
	s.debug.mu.Unlock()
	return resolved, nil
}

// DebugReadMemory returns up to 1024 bytes of emulated memory. Memory is
// read-only in this release.
func (s *StudioService) DebugReadMemory(address, length int) (MemoryBlock, error) {
	session, _, err := s.debugSession()
	if err != nil {
		return MemoryBlock{}, err
	}
	data, err := session.ReadMemory(address, length)
	if err != nil {
		return MemoryBlock{}, err
	}
	return MemoryBlock{Address: address, Data: hex.EncodeToString(data)}, nil
}

// DebugEvaluate reads watch expressions: symbols, decimal or hexadecimal
// addresses, and symbol ± constant.
func (s *StudioService) DebugEvaluate(expressions []string) ([]debugger.WatchValue, error) {
	if len(expressions) > 100 {
		return nil, errors.New("too many watch expressions")
	}
	session, sourceMap, err := s.debugSession()
	if err != nil {
		return nil, err
	}
	values := make([]debugger.WatchValue, 0, len(expressions))
	for _, expression := range expressions {
		values = append(values, debugger.Evaluate(expression, sourceMap, session))
	}
	return values, nil
}

// DebugSymbols lists the program symbols of the session's build.
func (s *StudioService) DebugSymbols() ([]debugger.Symbol, error) {
	_, sourceMap, err := s.debugSession()
	if err != nil {
		return nil, err
	}
	return sourceMap.Symbols(), nil
}

// DebugState returns the session state for a frontend that (re)connects.
func (s *StudioService) DebugState() DebugSnapshot {
	if _, _, err := s.debugSession(); err != nil {
		return DebugSnapshot{}
	}
	s.debug.mu.Lock()
	defer s.debug.mu.Unlock()
	return DebugSnapshot{Active: true, Stopped: s.debug.stopped, Stop: s.debug.stop}
}

// DebugStop ends the debug session and the emulated machine.
func (s *StudioService) DebugStop() error {
	return s.EmulatorStop()
}
