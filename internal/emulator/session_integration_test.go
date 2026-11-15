package emulator_test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"exbasicstudio/internal/atrexplorer"
	"exbasicstudio/internal/compiler"
	"exbasicstudio/internal/emulator"
	"exbasicstudio/internal/project"
)

func TestBundledEmulatorHardwareProfile(t *testing.T) {
	helperPath := compiler.ToolPath("atari800-studio")
	if _, err := os.Stat(helperPath); err != nil {
		t.Skipf("Atari800 headless bridge unavailable: %v", err)
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	disk := filepath.Join(root, "third_party", "atari800", "atari800-7.2.1", "test", "acid800.atr")
	if _, err = os.Stat(disk); err != nil {
		t.Skipf("Atari800 test disk unavailable: %v", err)
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
	frame, err := session.Frame()
	if err != nil {
		t.Fatal(err)
	}
	if frame.Number < 6 || !strings.HasPrefix(frame.Image, "data:image/png;base64,") {
		t.Fatalf("invalid hardware-profile frame: number=%d", frame.Number)
	}
}

func TestBundledToolchainRendersAFrame(t *testing.T) {
	compilerPath := compiler.ToolPath("exbasic")
	helperPath := compiler.ToolPath("atari800-studio")
	if _, err := os.Stat(compilerPath); err != nil {
		t.Skipf("bundled exBASIC compiler unavailable: %v", err)
	}
	if _, err := os.Stat(helperPath); err != nil {
		t.Skipf("Atari800 headless bridge unavailable: %v", err)
	}
	p, err := project.Create(t.TempDir(), "renderdemo")
	if err != nil {
		t.Fatal(err)
	}
	p.Config.Debug = false
	result, err := compiler.Build(p)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("compile failed: %s", result.Output)
	}
	session, err := emulator.Start(emulator.Launch{Helper: helperPath, Artifact: result.Artifact, Profile: emulator.Profile{Machine: "xl", Video: "ntsc", SIOPatch: true}})
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
	helperPath := compiler.ToolPath("atari800-studio")
	if _, err := os.Stat(helperPath); err != nil {
		t.Skipf("Atari800 headless bridge unavailable: %v", err)
	}
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
	state, err = session.KeyState("Home", true)
	if err != nil || state.KeyChar != 0 || state.KeyCode != 0 {
		t.Fatalf("unknown named key must not become a character: %#v err=%v", state, err)
	}
}

func TestEOSCartridgeBootsWithEOSFSDisk(t *testing.T) {
	compilerPath := compiler.ToolPath("exbasic")
	helperPath := compiler.ToolPath("atari800-studio")
	atrTool := compiler.ToolPath("mkatr-go")
	eosCar := compiler.AssetPath("EOS.car")
	for name, path := range map[string]string{"compiler": compilerPath, "Atari800 bridge": helperPath, "ATR tool": atrTool, "EOS cartridge": eosCar} {
		if _, err := os.Stat(path); err != nil {
			t.Skipf("%s unavailable: %v", name, err)
		}
	}
	p, err := project.Create(t.TempDir(), "eosdemo")
	if err != nil {
		t.Fatal(err)
	}
	p.Config.Debug = false
	p.Config.ATR.Enabled = true
	p.Config.ATR.Tool = atrTool
	p.Config.ATR.Filesystem = "eos"
	p.Config.ATR.Boot = false
	result, err := compiler.Build(p)
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
	session, err := emulator.Start(emulator.Launch{Helper: helperPath, Artifact: result.DiskImage, EOSCar: eosCar, HostDir: t.TempDir(), HostDisk: hostDisk, HostDeviceName: "D8:", Profile: emulator.Profile{Machine: "xl", Video: "ntsc", SIOPatch: true, EOS: true}})
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
