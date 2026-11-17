package studio

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"exbasicstudio/internal/compiler"
	"exbasicstudio/internal/debugger"
	"exbasicstudio/internal/tools/toolstest"
)

const debugProgram = "SCORE = 0\n" + // 1
	"FOR I = 1 TO 5\n" + // 2
	"  SCORE = SCORE + I\n" + // 3
	"NEXT I\n" + // 4
	"\n" + // 5 (blank: a breakpoint here moves to line 6)
	"PRINT SCORE\n" + // 6
	"DO\n" + // 7
	"LOOP\n" // 8

// waitDebug pumps emulator frames until a debugger event of the wanted type
// newer than afterSeq arrives.
func waitDebug(t *testing.T, s *StudioService, recorder *events, afterSeq uint64, want string) DebugEvent {
	t.Helper()
	for frame := 0; frame < 3000; frame++ {
		recorder.mu.Lock()
		for _, item := range recorder.all {
			if event, ok := item.(DebugEvent); ok && event.Seq > afterSeq && event.Type == want {
				recorder.mu.Unlock()
				return event
			}
		}
		recorder.mu.Unlock()
		if _, err := s.EmulatorFrame(); err != nil {
			t.Fatalf("emulator frame while waiting for %q: %v", want, err)
		}
	}
	t.Fatalf("no %q debugger event after sequence %d", want, afterSeq)
	return DebugEvent{}
}

// helperProcesses counts emulator helpers started by this test process.
// Other packages run their own helpers in parallel, so only our children
// are counted.
func helperProcesses(t *testing.T) int {
	t.Helper()
	pid := strconv.Itoa(os.Getpid())
	if runtime.GOOS == "windows" {
		query := "@(Get-CimInstance Win32_Process -Filter \"ParentProcessId=" + pid + " AND Name='atari800-studio.exe'\").Count"
		out, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", query).Output()
		if err != nil {
			t.Skipf("cannot list child processes: %v", err)
		}
		count, _ := strconv.Atoi(strings.TrimSpace(string(out)))
		return count
	}
	out, _ := exec.Command("pgrep", "-P", pid, "-x", "atari800-studio").Output()
	return len(strings.Fields(string(out)))
}

func debugProject(t *testing.T, native string) (*StudioService, *events, string) {
	t.Helper()
	toolstest.Tool(t, "exbasic")
	toolstest.Tool(t, "atari800-studio")
	toolstest.Tool(t, "mkatr-go")
	s, recorder := newService(t)
	view, err := s.CreateProject(t.TempDir(), "dbg")
	if err != nil {
		t.Fatal(err)
	}
	cfg := view.Config
	cfg.Native = native
	if _, err = s.UpdateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveFile("src/main.bas", debugProgram); err != nil {
		t.Fatal(err)
	}
	info, err := s.StartBuild(BuildRequest{})
	if err != nil {
		t.Fatal(err)
	}
	completed := recorder.completion(t, info.ID)
	result := completed.Result.(compiler.Result)
	if !result.Success || !result.DebugAvailable || result.MetadataPath == "" {
		t.Fatalf("debug build failed: %+v\n%s", result, result.Output)
	}
	return s, recorder, view.Root
}

func TestSourceLevelDebuggingWithBundledToolchain(t *testing.T) {
	for _, native := range []string{"auto", "none"} {
		t.Run("native-"+native, func(t *testing.T) {
			before := helperProcesses(t)
			s, recorder, root := debugProject(t, native)
			profiles, _ := s.EmulatorProfiles()

			session, err := s.DebugStart(DebugRequest{ProfileID: profiles[0].ID, Breakpoints: []debugger.Breakpoint{
				{Path: "src/main.bas", Line: 3, Enabled: true},
				{Path: "src/main.bas", Line: 5, Enabled: false},
			}})
			if err != nil {
				t.Fatal(err)
			}
			if len(session.Breakpoints) != 2 || !session.Breakpoints[0].Verified || session.Breakpoints[0].ResolvedLine != 3 {
				t.Fatalf("breakpoints = %+v", session.Breakpoints)
			}
			if session.Breakpoints[1].ResolvedLine != 6 {
				t.Fatalf("a breakpoint on a blank line did not move to the next statement: %+v", session.Breakpoints[1])
			}
			hasScore := false
			for _, symbol := range session.Symbols {
				hasScore = hasScore || (symbol.Name == "SCORE" && symbol.Type == "word")
			}
			if !hasScore {
				t.Fatalf("typed symbol SCORE missing: %+v", session.Symbols)
			}
			if snapshot := s.DebugState(); !snapshot.Active || snapshot.Stopped {
				t.Fatalf("snapshot after start = %+v", snapshot)
			}

			// Stop on the BASIC source breakpoint.
			stopped := waitDebug(t, s, recorder, 0, "stopped")
			if stopped.Stop == nil || stopped.Stop.Reason != "breakpoint" || stopped.Stop.Path != "src/main.bas" || stopped.Stop.Line != 3 {
				t.Fatalf("first stop = %+v", stopped.Stop)
			}
			if len(stopped.Stop.Disassembly) == 0 {
				t.Fatal("no disassembly at the stop")
			}
			values, err := s.DebugEvaluate([]string{"SCORE", "I", "score + 0", "NOPE"})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(values[0].Value, "0 ") || !strings.HasPrefix(values[1].Value, "1 ") {
				t.Fatalf("first iteration values = %+v", values)
			}
			if values[3].Error == "" {
				t.Fatal("unknown symbol evaluated")
			}
			if snapshot := s.DebugState(); !snapshot.Stopped || snapshot.Stop == nil || snapshot.Stop.Line != 3 {
				t.Fatalf("snapshot while stopped = %+v", snapshot)
			}

			// Continue: the same breakpoint is hit on the next iteration.
			if err = s.DebugCommand("continue"); err != nil {
				t.Fatal(err)
			}
			running := waitDebug(t, s, recorder, stopped.Seq, "running")
			second := waitDebug(t, s, recorder, running.Seq, "stopped")
			if second.Stop.Line != 3 {
				t.Fatalf("second stop = %+v", second.Stop)
			}
			values, _ = s.DebugEvaluate([]string{"SCORE", "I"})
			if !strings.HasPrefix(values[0].Value, "1 ") || !strings.HasPrefix(values[1].Value, "2 ") {
				t.Fatalf("second iteration values = %+v", values)
			}

			// Source-line step leaves line 3.
			if err = s.DebugCommand("stepLine"); err != nil {
				t.Fatal(err)
			}
			lineStep := waitDebug(t, s, recorder, second.Seq+1, "stopped")
			if lineStep.Stop.Reason != "step" || lineStep.Stop.Path != "src/main.bas" || lineStep.Stop.Line == 3 || lineStep.Stop.Line == 0 {
				t.Fatalf("line step = %+v", lineStep.Stop)
			}

			// A single 6502 instruction step moves the program counter.
			pcBefore := lineStep.Stop.Registers.PC
			if err = s.DebugCommand("stepInstruction"); err != nil {
				t.Fatal(err)
			}
			instruction := waitDebug(t, s, recorder, lineStep.Seq+1, "stopped")
			if instruction.Stop.Reason != "step" || instruction.Stop.Registers.PC == pcBefore {
				t.Fatalf("instruction step did not advance: %#x -> %+v", pcBefore, instruction.Stop.Registers)
			}
			if err = s.DebugCommand("stepOver"); err != nil {
				t.Fatal(err)
			}
			over := waitDebug(t, s, recorder, instruction.Seq+1, "stopped")
			if over.Stop.Reason != "step" {
				t.Fatalf("step over = %+v", over.Stop)
			}

			// Memory is readable in bounded blocks only.
			block, err := s.DebugReadMemory(0x0080, 16)
			if err != nil || len(block.Data) != 32 || block.Address != 0x80 {
				t.Fatalf("memory = %+v, %v", block, err)
			}
			for _, bad := range [][2]int{{0, 2000}, {0xFFFF, 2}, {-1, 4}, {0x10000, 1}, {0, 0}} {
				if _, err = s.DebugReadMemory(bad[0], bad[1]); err == nil {
					t.Fatalf("memory read %v was accepted", bad)
				}
			}

			// Without breakpoints the program runs to its idle loop; pause
			// stops it there.
			resolved, err := s.DebugSetBreakpoints([]debugger.Breakpoint{})
			if err != nil || len(resolved) != 0 {
				t.Fatalf("clearing breakpoints: %+v %v", resolved, err)
			}
			if err = s.DebugCommand("continue"); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 40; i++ {
				if _, err = s.EmulatorFrame(); err != nil {
					t.Fatal(err)
				}
			}
			if snapshot := s.DebugState(); snapshot.Stopped {
				t.Fatalf("the program stopped without breakpoints: %+v", snapshot.Stop)
			}
			values, _ = s.DebugEvaluate([]string{"SCORE"})
			if !strings.HasPrefix(values[0].Value, "15 ") {
				t.Fatalf("final SCORE = %+v", values[0])
			}
			recorder.mu.Lock()
			last := uint64(0)
			for _, item := range recorder.all {
				if event, ok := item.(DebugEvent); ok && event.Seq > last {
					last = event.Seq
				}
			}
			recorder.mu.Unlock()
			if err = s.DebugCommand("pause"); err != nil {
				t.Fatal(err)
			}
			paused := waitDebug(t, s, recorder, last, "stopped")
			if paused.Stop.Reason != "pause" {
				t.Fatalf("pause = %+v", paused.Stop)
			}
			// Run to a line, then step out of whatever routine we are in.
			if err = s.DebugRunToLine("src/main.bas", 8); err != nil {
				t.Fatal(err)
			}
			atLoop := waitDebug(t, s, recorder, paused.Seq+1, "stopped")
			if atLoop.Stop.Path != "src/main.bas" || (atLoop.Stop.Line != 7 && atLoop.Stop.Line != 8) {
				t.Fatalf("run to line = %+v", atLoop.Stop)
			}
			if err = s.DebugCommand("bogus"); err == nil {
				t.Fatal("unknown debugger command accepted")
			}

			// Editing a source makes the mapping stale: a new session is refused.
			if err = s.DebugStop(); err != nil {
				t.Fatal(err)
			}
			exited := waitExit(t, recorder, atLoop.Seq)
			if exited.Type != "exited" {
				t.Fatalf("no exit event: %+v", exited)
			}
			if s.DebugState().Active || s.EmulatorStatus().Running {
				t.Fatal("session still active after DebugStop")
			}
			if err = os.WriteFile(filepath.Join(root, "src", "main.bas"), []byte(debugProgram+"' edited\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err = s.DebugStart(DebugRequest{ProfileID: profiles[0].ID}); err == nil || !strings.Contains(err.Error(), "build again") {
				t.Fatalf("a stale source mapping was accepted: %v", err)
			}
			if _, err = s.DebugEvaluate([]string{"SCORE"}); err == nil {
				t.Fatal("evaluation without a session succeeded")
			}
			if after := helperProcesses(t); after > before {
				t.Fatalf("helper processes leaked: %d before, %d after", before, after)
			}
		})
	}
}

func waitExit(t *testing.T, recorder *events, afterSeq uint64) DebugEvent {
	t.Helper()
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	for _, item := range recorder.all {
		if event, ok := item.(DebugEvent); ok && event.Seq > afterSeq && event.Type == "exited" {
			return event
		}
	}
	return DebugEvent{}
}

func TestStructuredDiagnosticsFromBundledCompiler(t *testing.T) {
	toolstest.Tool(t, "exbasic")
	s, recorder := newService(t)
	if _, err := s.CreateProject(t.TempDir(), "diag"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveFile("src/main.bas", "PRINT 1\nFOR I = 1 TO\n"); err != nil {
		t.Fatal(err)
	}
	info, err := s.StartBuild(BuildRequest{})
	if err != nil {
		t.Fatal(err)
	}
	result := recorder.completion(t, info.ID).Result.(compiler.Result)
	if result.Success || len(result.Diagnostics) != 1 {
		t.Fatalf("result = %+v", result)
	}
	d := result.Diagnostics[0]
	if d.Path != "src/main.bas" || d.Line != 2 || d.Column != 13 || d.Code != "syntax" || d.Severity != "error" || !strings.Contains(d.Message, "parse error") {
		t.Fatalf("structured diagnostic = %+v", d)
	}
	if result.MetadataPath == "" {
		t.Fatal("metadata was not written for a failed build")
	}
	// Lint uses the same structured path for unsaved text.
	lint, err := s.Lint("src/main.bas", "PRINT +\n", 3, nil)
	if err != nil || !lint.Available || lint.Version != 3 || len(lint.Diagnostics) != 1 || lint.Diagnostics[0].Code != "syntax" || lint.Diagnostics[0].Path != "src/main.bas" {
		t.Fatalf("lint = %+v, %v", lint, err)
	}
}

func TestDebuggingRequiresDebugMetadata(t *testing.T) {
	toolstest.Tool(t, "exbasic")
	s, _ := newService(t)
	if _, err := s.CreateProject(t.TempDir(), "nometa"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DebugStart(DebugRequest{}); err == nil || !strings.Contains(err.Error(), "build the project") {
		t.Fatalf("debugging an unbuilt project: %v", err)
	}
	if _, err := s.DebugSetBreakpoints([]debugger.Breakpoint{{Path: "src/main.bas", Line: 1, Enabled: true}}); err != nil {
		t.Fatalf("setting breakpoints without a session failed: %v", err)
	}
	if snapshot := s.DebugState(); snapshot.Active {
		t.Fatal("a session is reported without one")
	}
}
