package studio

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"exbasicstudio/internal/atrexplorer"
	"exbasicstudio/internal/compiler"
	"exbasicstudio/internal/emulator"
	"exbasicstudio/internal/project"
)

func TestPrepareEOSHostDiskReservesD8AndSynchronizes(t *testing.T) {
	tool := filepath.Join("..", "..", "bin", "mkatr-go.exe")
	if _, err := os.Stat(tool); err != nil {
		t.Skipf("mkatr-go unavailable: %v", err)
	}
	root := t.TempDir()
	hostDir := filepath.Join(root, "H")
	if err := os.MkdirAll(hostDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostDir, "FROMHOST.TXT"), []byte("host"), 0o644); err != nil {
		t.Fatal(err)
	}
	drives := []emulator.DriveConfig{
		{Image: "one.atr", Enabled: true},
		{Image: "two.atr", Enabled: true},
		{Image: "three.atr", Enabled: true},
		{Image: "four.atr", Enabled: true},
	}
	launch, err := prepareEOSHostDisk(emulator.Launch{
		EOSCar:   "EOS.car",
		HostDir:  hostDir,
		HostTool: tool,
		Profile:  emulator.Profile{Drives: drives},
	})
	if err != nil {
		t.Fatal(err)
	}
	if launch.HostDeviceName != "D8:" {
		t.Fatalf("EOS project disk mapped as %q, want D8:", launch.HostDeviceName)
	}
	for index := range drives {
		if launch.Profile.Drives[index] != drives[index] {
			t.Fatalf("D%d configuration was changed: got %+v want %+v", index+1, launch.Profile.Drives[index], drives[index])
		}
	}
	view, err := atrexplorer.Catalog(tool, launch.HostDisk)
	if err != nil {
		t.Fatal(err)
	}
	if view.Filesystem != "eos" || len(view.Entries) != 1 || view.Entries[0].Path != "FROMHOST.TXT" {
		t.Fatalf("unexpected EOS project disk: %+v", view)
	}
	newFile := filepath.Join(root, "FROMEOS.TXT")
	if err = os.WriteFile(newFile, []byte("eos"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err = atrexplorer.Import(tool, launch.HostDisk, "", []string{newFile}); err != nil {
		t.Fatal(err)
	}
	if err = synchronizeEOSHostDisk(launch); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(hostDir, "FROMEOS.TXT"))
	if err != nil || string(data) != "eos" {
		t.Fatalf("EOS file was not synchronized to H: data=%q err=%v", data, err)
	}
}

func TestProjectHostPath(t *testing.T) {
	for _, path := range []string{"H", "H/file.txt", "h\\nested\\file.bas"} {
		if !isProjectHostPath(path) {
			t.Errorf("isProjectHostPath(%q) = false", path)
		}
	}
	for _, path := range []string{"", "src/H/file.txt", "H2/file.txt", "H/../file.txt"} {
		if isProjectHostPath(path) {
			t.Errorf("isProjectHostPath(%q) = true", path)
		}
	}
}

func TestRunningEOSHostMutationRefreshesD8(t *testing.T) {
	helper := compiler.ToolPath("atari800-studio")
	tool := compiler.ToolPath("mkatr-go")
	eosCar := compiler.AssetPath("EOS.car")
	for name, path := range map[string]string{"Atari800 bridge": helper, "ATR tool": tool, "EOS cartridge": eosCar} {
		if _, err := os.Stat(path); err != nil {
			t.Skipf("%s unavailable: %v", name, err)
		}
	}
	p, err := project.Create(t.TempDir(), "lived8")
	if err != nil {
		t.Fatal(err)
	}
	hostDir := filepath.Join(p.Root, "H")
	if err = os.MkdirAll(hostDir, 0o755); err != nil {
		t.Fatal(err)
	}
	launch, err := prepareEOSHostDisk(emulator.Launch{
		Helper: helper, EOSCar: eosCar, HostDir: hostDir, HostTool: tool,
		Profile: emulator.Profile{ID: "eos-test", Machine: "xl", Memory: "64", Video: "ntsc", SIOPatch: true, EOS: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := emulator.Start(launch)
	if err != nil {
		t.Fatal(err)
	}
	service := New()
	service.active = p
	service.emulator = session
	service.emulatorLaunch = launch
	t.Cleanup(func() { _ = service.EmulatorStop() })

	if err = service.SaveFile("H/LIVE.TXT", "visible without restarting"); err != nil {
		t.Fatal(err)
	}
	view, err := atrexplorer.Catalog(tool, launch.HostDisk)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range view.Entries {
		found = found || entry.Path == "LIVE.TXT"
	}
	if !found {
		t.Fatalf("refreshed D8 does not contain LIVE.TXT: %+v", view.Entries)
	}
	if _, err = session.Frame(); err != nil {
		t.Fatalf("emulator did not continue after D8 remount: %v", err)
	}

	// Model an EOS-side write while the machine is running and verify the
	// frame polling synchronization path publishes it into H immediately.
	if err = session.DismountHostDisk(); err != nil {
		t.Fatal(err)
	}
	fromEOS := filepath.Join(t.TempDir(), "FROMEOS.TXT")
	if err = os.WriteFile(fromEOS, []byte("created on D8"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err = atrexplorer.Import(tool, launch.HostDisk, "", []string{fromEOS}); err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().Add(2 * time.Second)
	if err = os.Chtimes(launch.HostDisk, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if err = session.MountHostDisk(); err != nil {
		t.Fatal(err)
	}
	service.mu.Lock()
	service.emulatorDiskCheck = time.Time{}
	service.mu.Unlock()
	changed, err := service.maybeSynchronizeEOSHostDisk(session, launch)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("D8 change was not reported to the workspace")
	}
	data, err := os.ReadFile(filepath.Join(hostDir, "FROMEOS.TXT"))
	if err != nil || string(data) != "created on D8" {
		t.Fatalf("live D8 file was not mirrored into H: data=%q err=%v", data, err)
	}
}
