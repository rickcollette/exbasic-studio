package emulator_test

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"exbasicstudio/internal/atrexplorer"
	"exbasicstudio/internal/compiler"
	"exbasicstudio/internal/emulator"
	"exbasicstudio/internal/project"
	"exbasicstudio/internal/tools/toolstest"
)

func TestBundledEmulatorHardwareProfile(t *testing.T) {
	helperPath := toolstest.Tool(t, "ex800-studio")
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	disk := filepath.Join(root, "third_party", "ex800", "ex800-1.0.0", "test", "acid800.atr")
	if _, err = os.Stat(disk); err != nil {
		t.Skipf("ex800 test disk unavailable: %v", err)
	}
	ide := filepath.Join(t.TempDir(), "ide-plus.img")
	file, err := os.Create(ide)
	if err != nil {
		t.Fatal(err)
	}
	if err = file.Truncate(2 << 20); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	drives := make([]emulator.DriveConfig, 4)
	for i := range drives {
		drives[i] = emulator.DriveConfig{Image: disk, Enabled: true, ReadOnly: true}
	}
	profile := emulator.Profile{
		Machine:  "xl",
		Memory:   "256",
		Video:    "ntsc",
		CPUSpeed: 150,
		SIOPatch: true,
		Drives:   drives,
		RDevice: emulator.RDeviceConfig{
			// Leave Modem false deliberately: R-Verter mode itself must activate
			// modem responses and carry the selected 19.2K line rate.
			Mode: "rverter", ListenPort: 19000, ConnectBaud: 19200,
		},
		IDE: emulator.IDEConfig{Enabled: true, Image: ide, CompactFlash: true},
	}
	session, err := emulator.Start(emulator.Launch{Helper: helperPath, Profile: profile})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Stop() })
	if core := session.Hello().Core; !strings.HasPrefix(strings.ToLower(core), "ex800") {
		t.Fatalf("unexpected emulator core %q", core)
	}
	frame, err := session.Frame()
	if err != nil {
		t.Fatal(err)
	}
	if frame.Number < 6 || !strings.HasPrefix(frame.Image, "data:image/png;base64,") {
		t.Fatalf("invalid hardware-profile frame: number=%d", frame.Number)
	}
}

func TestBundledToolchainRendersAFrame(t *testing.T) {
	toolstest.Tool(t, "exbasic")
	helperPath := toolstest.Tool(t, "ex800-studio")
	p, err := project.Create(t.TempDir(), "renderdemo")
	if err != nil {
		t.Fatal(err)
	}
	p.Config.Debug = false
	result, err := compiler.Build(context.Background(), p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("compile failed: %s", result.Output)
	}
	session, err := emulator.Start(emulator.Launch{Helper: helperPath, Artifact: result.Artifact("executable"), Profile: emulator.Profile{Machine: "xl", Video: "ntsc", SIOPatch: true}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Stop() })
	frame, err := session.Frame()
	if err != nil {
		t.Fatal(err)
	}
	if frame.Number < 1 || !strings.HasPrefix(frame.Image, "data:image/png;base64,") {
		t.Fatalf("invalid rendered frame: number=%d image-prefix=%q", frame.Number, frame.Image[:min(32, len(frame.Image))])
	}
}

func TestBundledEmulatorStartsWithoutProjectMedia(t *testing.T) {
	helperPath := toolstest.Tool(t, "ex800-studio")
	session, err := emulator.Start(emulator.Launch{Helper: helperPath, HostDir: t.TempDir(), Profile: emulator.Profile{Machine: "xe", Video: "pal", Basic: true, SIOPatch: true}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Stop() })
	frame, err := session.Frame()
	if err != nil {
		t.Fatal(err)
	}
	if frame.Number < 1 || !strings.HasPrefix(frame.Image, "data:image/png;base64,") {
		t.Fatalf("invalid standalone emulator frame: number=%d", frame.Number)
	}
	for range 75 {
		if frame.HostDevice {
			break
		}
		frame, err = session.Frame()
		if err != nil {
			t.Fatal(err)
		}
	}
	if !frame.HostDevice {
		t.Fatal("project H1: host device was not installed in Atari HATABS")
	}

	state, err := session.KeyState("Shift", true)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Shift || state.KeyChar != 0 || state.KeyCode != 0 {
		t.Fatalf("Shift must be modifier-only, got %#v", state)
	}
	state, err = session.KeyState("A", true)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Shift || state.KeyChar != int('A') {
		t.Fatalf("shifted A was not preserved: %#v", state)
	}
	_, _ = session.KeyState("A", false)
	state, err = session.KeyState("Shift", false)
	if err != nil || state.Shift || state.KeyChar != 0 {
		t.Fatalf("Shift release failed: %#v err=%v", state, err)
	}
	state, err = session.KeyState("ArrowLeft", true)
	if err != nil || state.KeyCode != 0x86 || state.KeyChar != 0 {
		t.Fatalf("left arrow mapping failed: %#v err=%v", state, err)
	}
	_, _ = session.KeyState("ArrowLeft", false)
	state, err = session.KeyState("Control", true)
	if err != nil || !state.Control || state.KeyChar != 0 {
		t.Fatalf("Control must be modifier-only: %#v err=%v", state, err)
	}
	state, err = session.KeyState("a", true)
	if err != nil || !state.Control || state.KeyChar != int('a') {
		t.Fatalf("controlled character mapping failed: %#v err=%v", state, err)
	}
	_, _ = session.KeyState("a", false)
	_, _ = session.KeyState("Control", false)
	for _, key := range []string{"\"", "\\"} {
		state, err = session.KeyState(key, true)
		if err != nil || state.KeyChar != int(key[0]) {
			t.Fatalf("escaped key %q mapping failed: %#v err=%v", key, state, err)
		}
		_, _ = session.KeyState(key, false)
	}
	state, err = session.KeyState("PageUp", true)
	if err != nil || state.KeyChar != 0 || state.KeyCode != 0 {
		t.Fatalf("unknown named key must not become a character: %#v err=%v", state, err)
	}
	// Atari keys without a PC key: Home is CLEAR (Shift+<), End is HELP.
	for key, code := range map[string]int{"Home": 0x76, "End": 0x11} {
		state, err = session.KeyState(key, true)
		if err != nil || state.KeyChar != 0 || state.KeyCode != code {
			t.Fatalf("%s mapping failed: %#v err=%v", key, state, err)
		}
		_, _ = session.KeyState(key, false)
	}
	// BREAK has no key code of its own. Run an endless BASIC loop and stop
	// it with Pause: BASIC then prints STOPPED AT LINE 10.
	press := func(key string) {
		t.Helper()
		if err := session.Key(key, true); err != nil {
			t.Fatal(err)
		}
		if _, err := session.Frame(); err != nil {
			t.Fatal(err)
		}
		if err := session.Key(key, false); err != nil {
			t.Fatal(err)
		}
		if _, err := session.Frame(); err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range strings.Split("10 GOTO 10|RUN|", "") {
		if key == "|" {
			key = "Enter"
		}
		press(key)
	}
	press("Pause")
	screen := func() string {
		t.Helper()
		address, err := session.ReadMemory(0x58, 2) // SAVMSC
		if err != nil {
			t.Fatal(err)
		}
		data, err := session.ReadMemory(int(address[0])|int(address[1])<<8, 960)
		if err != nil {
			t.Fatal(err)
		}
		text := make([]byte, len(data)) // internal screen codes to ASCII
		for i, code := range data {
			switch code &= 0x7f; {
			case code < 0x40:
				text[i] = code + 0x20
			case code >= 0x60:
				text[i] = code
			default:
				text[i] = ' '
			}
		}
		return string(text)
	}
	if !strings.Contains(strings.ToUpper(screen()), "STOPPED AT LINE 10") {
		t.Fatalf("Pause did not press BREAK; screen: %q", screen())
	}
}

// The keyboard joystick drives joystick 0: the OS copies PORTA and TRIG0
// into the STICK0 ($0278) and STRIG0 ($0284) shadows every vertical blank.
func TestBundledEmulatorJoystick(t *testing.T) {
	helperPath := toolstest.Tool(t, "ex800-studio")
	session, err := emulator.Start(emulator.Launch{Helper: helperPath, HostDir: t.TempDir(), Profile: emulator.Profile{Machine: "xl", Video: "ntsc", Basic: true, SIOPatch: true}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Stop() })
	if !slices.Contains(session.Hello().Capabilities, "joystick") {
		t.Fatalf("helper does not advertise the joystick capability: %v", session.Hello().Capabilities)
	}
	shadows := func() (int, int) {
		t.Helper()
		if _, err := session.Frame(); err != nil {
			t.Fatal(err)
		}
		stick, err := session.ReadMemory(0x0278, 1)
		if err != nil {
			t.Fatal(err)
		}
		strig, err := session.ReadMemory(0x0284, 1)
		if err != nil {
			t.Fatal(err)
		}
		return int(stick[0]), int(strig[0])
	}
	// The shadows read 0 until the OS cold start has run its first VBI.
	stick, strig := shadows()
	for i := 0; i < 150 && stick == 0; i++ {
		stick, strig = shadows()
	}
	if stick != 15 || strig != 1 {
		t.Fatalf("idle joystick: STICK=%d STRIG=%d, want 15 and 1", stick, strig)
	}
	for _, want := range []int{14, 13, 11, 7, 10, 6, 9, 5} {
		if err := session.Joystick(want, want == 10); err != nil {
			t.Fatal(err)
		}
		stick, strig := shadows()
		wantStrig := 1
		if want == 10 {
			wantStrig = 0
		}
		if stick != want || strig != wantStrig {
			t.Fatalf("Joystick(%d): STICK=%d STRIG=%d, want %d and %d", want, stick, strig, want, wantStrig)
		}
	}
	if err := session.ReleaseKeys(); err != nil {
		t.Fatal(err)
	}
	if stick, strig := shadows(); stick != 15 || strig != 1 {
		t.Fatalf("released joystick: STICK=%d STRIG=%d, want 15 and 1", stick, strig)
	}
	if err := session.Joystick(16, false); err == nil {
		t.Fatal("an out-of-range STICK value was accepted")
	}
}

func TestEOSCartridgeBootsWithEOSFSDisk(t *testing.T) {
	toolstest.Tool(t, "exbasic")
	helperPath := toolstest.Tool(t, "ex800-studio")
	atrTool := toolstest.Tool(t, "mkatr-go")
	eosCar := toolstest.Asset(t, "EOS.car")
	p, err := project.Create(t.TempDir(), "eosdemo")
	if err != nil {
		t.Fatal(err)
	}
	p.Config.Debug = false
	p.Config.ATR.Enabled = true
	p.Config.ATR.Tool = atrTool
	p.Config.ATR.Filesystem = "eos"
	p.Config.ATR.Boot = false
	result, err := compiler.Build(context.Background(), p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("EOS build failed: %s", result.Output)
	}
	hostDisk := filepath.Join(t.TempDir(), "eos-host.atr")
	probe := filepath.Join(t.TempDir(), "PROBE.TXT")
	if err = os.WriteFile(probe, []byte("D8 project folder probe"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err = atrexplorer.Create(atrTool, hostDisk, "eos", []string{probe}); err != nil {
		t.Fatal(err)
	}
	session, err := emulator.Start(emulator.Launch{Helper: helperPath, Artifact: result.Artifact("disk-image"), EOSCar: eosCar, HostDir: t.TempDir(), HostDisk: hostDisk, HostDeviceName: "D8:", Profile: emulator.Profile{Machine: "xl", Video: "ntsc", SIOPatch: true, EOS: true}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Stop() })
	frame, err := session.Frame()
	if err != nil {
		t.Fatal(err)
	}
	if frame.Number < 1 || !strings.HasPrefix(frame.Image, "data:image/png;base64,") {
		t.Fatalf("invalid EOS frame: number=%d", frame.Number)
	}
	if !frame.HostDevice || frame.HostDeviceName != "D8:" {
		t.Fatalf("EOS project disk mapping not reported: %+v", frame)
	}
	if err = session.DismountHostDisk(); err != nil {
		t.Fatalf("dismount D8: %v", err)
	}
	if err = session.MountHostDisk(); err != nil {
		t.Fatalf("remount D8: %v", err)
	}
	for range 180 {
		if frame, err = session.Frame(); err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range []string{"D", "I", "R", " ", "D", "8", ":", "Enter"} {
		if err = session.Key(key, true); err != nil {
			t.Fatal(err)
		}
		if frame, err = session.Frame(); err != nil {
			t.Fatal(err)
		}
		if frame, err = session.Frame(); err != nil {
			t.Fatal(err)
		}
		if err = session.Key(key, false); err != nil {
			t.Fatal(err)
		}
		if frame, err = session.Frame(); err != nil {
			t.Fatal(err)
		}
		if frame, err = session.Frame(); err != nil {
			t.Fatal(err)
		}
	}
	for range 600 {
		if frame, err = session.Frame(); err != nil {
			t.Fatal(err)
		}
	}
	if capture := os.Getenv("EXBASIC_EOS_CAPTURE"); capture != "" {
		encoded := strings.TrimPrefix(frame.Image, "data:image/png;base64,")
		data, decodeErr := base64.StdEncoding.DecodeString(encoded)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if err = os.WriteFile(capture, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
