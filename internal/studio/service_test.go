package studio

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUsefulWorkingDirectoryMovesOutOfDevelopmentBin(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0755); err != nil {
		t.Fatal(err)
	}
	previous, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(previous) })
	if err := os.Chdir(bin); err != nil {
		t.Fatal(err)
	}
	if got := usefulWorkingDirectory(); !strings.EqualFold(filepath.Clean(got), filepath.Clean(root)) {
		t.Fatalf("usefulWorkingDirectory() = %q, want %q", got, root)
	}
}

func TestTerminalPreservesWorkingDirectory(t *testing.T) {
	s := New()
	view, err := s.CreateProject(t.TempDir(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	command := "cd src"
	if runtime.GOOS == "windows" {
		command = "Set-Location src"
	}
	result, err := s.RunTerminal(command)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(view.Root, "src")
	if !strings.EqualFold(filepath.Clean(result.WorkingDir), filepath.Clean(want)) {
		t.Fatalf("terminal cwd = %q, want %q; output=%q", result.WorkingDir, want, result.Output)
	}
	result, err = s.RunTerminal("Get-Location")
	if runtime.GOOS != "windows" {
		result, err = s.RunTerminal("pwd")
	}
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(result.Output), strings.ToLower(filepath.Base(want))) {
		t.Fatalf("command did not run in preserved cwd: %q", result.Output)
	}
}

func TestRenamingEntryFolderUpdatesConfig(t *testing.T) {
	s := New()
	if _, err := s.CreateProject(t.TempDir(), "demo"); err != nil {
		t.Fatal(err)
	}
	view, err := s.RenamePath("src", "source")
	if err != nil {
		t.Fatal(err)
	}
	if view.Config.Entry != "source/main.bas" {
		t.Fatalf("entry = %q, want source/main.bas", view.Config.Entry)
	}
	if _, err := s.ReadFile(view.Config.Entry); err != nil {
		t.Fatal(err)
	}
}

func TestProjectHostDirectoryIsCreatedForAtariH1(t *testing.T) {
	s := New()
	view, err := s.CreateProject(t.TempDir(), "hostdrive")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(view.Root, "H")
	info, err := os.Stat(want)
	if err != nil || !info.IsDir() {
		t.Fatalf("H directory was not created with the project: info=%v err=%v", info, err)
	}
	p, err := s.current()
	if err != nil {
		t.Fatal(err)
	}
	hostDir, err := ensureProjectHostDir(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(filepath.Clean(hostDir), filepath.Clean(want)) {
		t.Fatalf("host dir = %q, want %q", hostDir, want)
	}
	info, err = os.Stat(want)
	if err != nil || !info.IsDir() {
		t.Fatalf("H directory was not created: info=%v err=%v", info, err)
	}
}

func TestOpenFolderWithoutProjectMetadata(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "game.bas"), []byte("PRINT 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New()
	view, err := s.OpenProject(root)
	if err != nil {
		t.Fatal(err)
	}
	if view.Root != root || view.Config.Entry != "game.bas" {
		t.Fatalf("unexpected workspace view: %+v", view)
	}
	if _, err = os.Stat(filepath.Join(root, "H")); err != nil {
		t.Fatalf("workspace H folder was not created: %v", err)
	}
	if _, err = os.Stat(filepath.Join(root, "exbasic.project.json")); !os.IsNotExist(err) {
		t.Fatalf("opening the folder unexpectedly created project metadata: %v", err)
	}
}

func TestImportProjectPathsRefreshesWorkspaceView(t *testing.T) {
	s := New()
	view, err := s.CreateProject(t.TempDir(), "dropdemo")
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "DROPPED.TXT")
	if err = os.WriteFile(source, []byte("dropped"), 0o644); err != nil {
		t.Fatal(err)
	}
	view, err = s.ImportProjectPaths("H", []string{source})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, file := range view.Files {
		found = found || file.Path == "H/DROPPED.TXT"
	}
	if !found {
		t.Fatalf("imported file is missing from workspace view: %+v", view.Files)
	}
	data, err := os.ReadFile(filepath.Join(view.Root, "H", "DROPPED.TXT"))
	if err != nil || string(data) != "dropped" {
		t.Fatalf("imported file = %q, err=%v", data, err)
	}
}
