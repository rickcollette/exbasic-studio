package studio

import (
	"strings"
	"testing"

	"exbasicstudio/internal/debugger"
	"exbasicstudio/internal/emulator"
)

// eosProfile saves a profile with EOS as the operating system.
func eosProfile(t *testing.T, s *StudioService) emulator.Profile {
	t.Helper()
	profile, err := s.EmulatorSaveProfile(emulator.Profile{Name: "EOS test", Machine: "xe", Memory: "1088", Ultimate1MB: true, Video: "ntsc", CPUSpeed: 100, SIOPatch: true, EOS: true, Scale: 2})
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

// EOS loads programs from its own filesystem. Running a project with an EOS
// profile must put the program on a disk EOS can read and start it, not hand
// EOS a bare executable that shows up as "148 Unknown filesystem".
func TestRunStartsTheProgramUnderEOS(t *testing.T) {
	s, recorder, root := debugProject(t, "auto")
	// A program that prints and then waits, so its output stays on screen.
	if err := s.SaveFile("src/main.bas", "PRINT \"HELLO FROM EXBASIC\"\nGET K\n"); err != nil {
		t.Fatal(err)
	}
	profile := eosProfile(t, s)

	info, err := s.StartRun(RunRequest{ProfileID: profile.ID})
	if err != nil {
		t.Fatal(err)
	}
	completed := recorder.completion(t, info.ID)
	if completed.Error != "" {
		t.Fatalf("run failed: %s", completed.Error)
	}
	result := completed.Result.(RunResult)
	if !result.Started {
		t.Fatalf("the emulator did not start: %+v", result)
	}
	s.mu.RLock()
	session, launch := s.emulator, s.emulatorLaunch
	s.mu.RUnlock()
	if launch.Program != "DBG.XEX" || !strings.HasSuffix(launch.Artifact, "program.atr") {
		t.Fatalf("the program was not put on an EOS disk: %+v", launch)
	}
	if strings.HasPrefix(launch.Artifact, root) {
		t.Fatalf("the program disk was written into the project: %s", launch.Artifact)
	}
	screen := ""
	for attempt := 0; attempt < 60 && !strings.Contains(screen, "HELLO FROM EXBASIC"); attempt++ {
		for range 10 {
			if _, err = s.EmulatorFrame(); err != nil {
				t.Fatal(err)
			}
		}
		if screen, err = session.ScreenText(24); err != nil {
			t.Fatal(err)
		}
	}
	if !strings.Contains(screen, "HELLO FROM EXBASIC") {
		t.Fatalf("the program did not run under EOS; screen:\n%s", screen)
	}
	if strings.Contains(screen, "Unknown filesystem") {
		t.Fatalf("EOS could not read the program disk; screen:\n%s", screen)
	}
}

// Debugging with an EOS profile starts the program through EOS and still
// stops on a source breakpoint.
func TestDebuggingUnderEOSStopsOnABreakpoint(t *testing.T) {
	s, recorder, _ := debugProject(t, "auto")
	profile := eosProfile(t, s)
	session, err := s.DebugStart(DebugRequest{ProfileID: profile.ID, Breakpoints: []debugger.Breakpoint{{Path: "src/main.bas", Line: 3, Enabled: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(session.Breakpoints) != 1 || !session.Breakpoints[0].Verified {
		t.Fatalf("breakpoints = %+v", session.Breakpoints)
	}
	stopped := waitDebug(t, s, recorder, 0, "stopped")
	if stopped.Stop == nil || stopped.Stop.Reason != "breakpoint" || stopped.Stop.Path != "src/main.bas" || stopped.Stop.Line != 3 {
		t.Fatalf("stop under EOS = %+v", stopped.Stop)
	}
	if err = s.DebugStop(); err != nil {
		t.Fatal(err)
	}
}
