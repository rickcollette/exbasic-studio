package studio

import (
	"os"
	"strings"

	"exbasicstudio/internal/atrexplorer"
	"exbasicstudio/internal/emulator"
	"exbasicstudio/internal/tools/toolstest"
	"path/filepath"
	"testing"
)

func TestProjectPathsInsideTheHostFolder(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	outside := t.TempDir()
	cases := []struct {
		name, hostDir, rel string
		want               bool
	}{
		{"the default H folder", filepath.Join(root, "H"), "H", true},
		{"a file in H", filepath.Join(root, "H"), "h/DATA.TXT", true},
		{"a neighbour with the same prefix", filepath.Join(root, "H"), "HELLO.BAS", false},
		{"a chosen folder in the project", filepath.Join(root, "data", "disk"), "data/disk/A.TXT", true},
		{"the old H folder once another is chosen", filepath.Join(root, "data"), "H/A.TXT", false},
		{"a chosen folder outside the project", outside, "H/A.TXT", false},
		{"the project folder itself", root, "main.bas", true},
		{"no host folder", "", "H/A.TXT", false},
	}
	for _, c := range cases {
		if got := isProjectHostPath(root, c.hostDir, c.rel); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// A folder chosen in the profile is what the machine gets as H1:, and under
// EOS as D8:, with the disk image kept away from that folder.
func TestEmulatorStartsWithTheChosenHostFolder(t *testing.T) {
	toolstest.Tool(t, "ex800-studio")
	toolstest.Tool(t, "mkatr-go")
	s, _ := newService(t)
	parent := t.TempDir()
	chosen := filepath.Join(parent, "shared")
	if err := os.MkdirAll(chosen, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chosen, "HELLO.TXT"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	plain, err := s.EmulatorSaveProfile(emulator.Profile{Name: "Plain", Machine: "xl", SIOPatch: true, HostDir: " " + chosen + " "})
	if err != nil {
		t.Fatal(err)
	}
	if plain.HostDir != chosen {
		t.Fatalf("stored folder = %q", plain.HostDir)
	}
	state, err := s.EmulatorStart(plain.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if s.emulatorLaunch.HostDir != chosen || state.HostDeviceName != "H1:" {
		t.Fatalf("launch = %+v, state = %+v", s.emulatorLaunch, state)
	}

	eos, err := s.EmulatorSaveProfile(emulator.Profile{Name: "EOS", Machine: "xl", SIOPatch: true, HostDir: chosen, Cartridge: emulator.CartridgeSlot{Kind: emulator.CartEOS}})
	if err != nil {
		t.Fatal(err)
	}
	if state, err = s.EmulatorStart(eos.ID, ""); err != nil {
		t.Fatal(err)
	}
	launch := s.emulatorLaunch
	if launch.HostDir != chosen || state.HostDeviceName != "D8:" || !strings.HasPrefix(launch.HostDisk, s.state.Dir()) {
		t.Fatalf("launch = %+v, state = %+v", launch, state)
	}
	view, err := atrexplorer.Catalog(launch.HostTool, launch.HostDisk)
	if err != nil || len(view.Entries) != 1 || !strings.EqualFold(view.Entries[0].Path, "HELLO.TXT") {
		t.Fatalf("D8: holds %+v, %v", view.Entries, err)
	}
	if err = s.EmulatorStop(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(parent)
	if len(entries) != 1 {
		t.Fatalf("Studio wrote beside the chosen folder: %v", entries)
	}

	if _, err = s.EmulatorSaveProfile(emulator.Profile{Name: "Gone", HostDir: filepath.Join(parent, "gone")}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.EmulatorStart("gone", ""); err == nil || !strings.Contains(err.Error(), "not a folder") {
		t.Fatalf("a missing folder: %v", err)
	}
}
