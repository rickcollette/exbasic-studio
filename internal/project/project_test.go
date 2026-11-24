package project

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProjectLifecycle(t *testing.T) {
	parent := t.TempDir()
	p, err := Create(parent, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if p.Config.Entry != "src/main.bas" {
		t.Fatalf("wrong entry: %v", p.Config.Entry)
	}
	opened, err := Open(p.Root)
	if err != nil {
		t.Fatal(err)
	}
	text, err := opened.Read("src/main.bas")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "exBASIC STUDIO") {
		t.Fatal("sample not written")
	}
	if err := opened.Save("src/main.bas", "PRINT 123\n"); err != nil {
		t.Fatal(err)
	}
	text, err = opened.Read("src/main.bas")
	if err != nil || text != "PRINT 123\n" {
		t.Fatalf("write failed: %q %v", text, err)
	}
	view, err := opened.View()
	if err != nil || len(view.Files) < 2 {
		t.Fatalf("bad view %+v %v", view, err)
	}
}

func TestOpenOrdinaryFolderAsWorkspace(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "game.bas"), []byte("PRINT 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opened, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Config.Entry != "src/game.bas" {
		t.Fatalf("ordinary workspace entry = %q, want src/game.bas", opened.Config.Entry)
	}
	if _, err = os.Stat(filepath.Join(root, ConfigName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("opening a folder created project metadata: %v", err)
	}
	view, err := opened.View()
	if err != nil || len(view.Files) != 1 || view.Files[0].Path != "src/game.bas" {
		t.Fatalf("ordinary workspace view = %+v, err=%v", view, err)
	}
}

func TestOpenEmptyFolderAsWorkspace(t *testing.T) {
	opened, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = opened.View(); err != nil {
		t.Fatal(err)
	}
}

func TestViewIncludesEmptyDirectories(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"H", "src/screens", ".exbasic-studio/private", "build/output"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(rel)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	opened, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	view, err := opened.View()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".exbasic-studio", ".exbasic-studio/private", "H", "build", "build/output", "src", "src/screens"}
	if strings.Join(view.Directories, "|") != strings.Join(want, "|") {
		t.Fatalf("directories = %v, want %v", view.Directories, want)
	}
}

func TestExportFile(t *testing.T) {
	p, err := Create(t.TempDir(), "download")
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "main.bas")
	if err = p.ExportFile("src/main.bas", destination); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(destination)
	if err != nil || !strings.Contains(string(got), "download") {
		t.Fatalf("downloaded file = %q, err=%v", got, err)
	}
	if err = p.ExportFile("src/main.bas", "relative.bas"); err == nil {
		t.Fatal("relative download destination accepted")
	}
	if err = p.ExportFile("src/main.bas", filepath.Join(p.Root, "src", "main.bas")); err == nil {
		t.Fatal("original file accepted as its own download destination")
	}
}

func TestTraversalAndConfigProtection(t *testing.T) {
	p, err := Create(t.TempDir(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"../escape.bas", "src/../../escape.bas", "/etc/passwd", "..\\escape.bas"} {
		if _, err := p.Resolve(rel); err == nil {
			t.Errorf("accepted %q", rel)
		}
	}
	if err := p.Save("./"+ConfigName, "{}"); err == nil {
		t.Fatal("configuration overwritten through file writer")
	}
}
func TestRejectSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink privileges vary on Windows")
	}
	p, err := Create(t.TempDir(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "secret.bas")
	if err := os.WriteFile(dest, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dest, filepath.Join(p.Root, "link.bas")); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Read("link.bas"); err == nil {
		t.Fatal("symlink read was allowed")
	}
}
func TestInvalidConfig(t *testing.T) {
	cfg := DefaultConfig("demo")
	cfg.Output = "../../outside.xex"
	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("traversal output allowed")
	}
}

func TestATRDefaultsAndEOSFSValidation(t *testing.T) {
	cfg := DefaultConfig("demo")
	if cfg.ATR.Tool != "mkatr-go" || cfg.ATR.Filesystem != "sparta" || cfg.ATR.Output != "build/demo.atr" || !cfg.ATR.Boot {
		t.Fatalf("unexpected ATR defaults: %+v", cfg.ATR)
	}
	cfg.ATR.Enabled = true
	cfg.ATR.Filesystem = "eos"
	if err := ValidateConfig(cfg); err == nil || !strings.Contains(err.Error(), "disable ATR boot") {
		t.Fatalf("EOSFS SDFS bootloader was not rejected: %v", err)
	}
	cfg.ATR.Boot = false
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("valid EOSFS configuration was rejected: %v", err)
	}
	legacy := Config{Name: "old", Target: "atari", Entry: "src/main.bas", Output: "build/old.xex", Compiler: "exbasic", Emulator: "atari800", Native: "auto"}
	NormalizeConfig(&legacy)
	if !legacy.ATR.Boot || legacy.ATR.Filesystem != "sparta" || legacy.ATR.Output != "build/old.atr" {
		t.Fatalf("legacy project did not receive ATR defaults: %+v", legacy.ATR)
	}
}

func TestWorkspaceFileOperations(t *testing.T) {
	p, err := Create(t.TempDir(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.CreateDirectory("src/screens"); err != nil {
		t.Fatal(err)
	}
	if err := p.Save("src/screens/title.bas", "PRINT \"TITLE\"\n"); err != nil {
		t.Fatal(err)
	}
	if err := p.Rename("src/screens/title.bas", "src/screens/intro.bas"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Read("src/screens/intro.bas"); err != nil {
		t.Fatal(err)
	}
	if err := p.Delete("src/screens"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.Root, "src", "screens")); !os.IsNotExist(err) {
		t.Fatalf("deleted folder still exists: %v", err)
	}
	if err := p.Delete(ConfigName); err == nil {
		t.Fatal("project configuration deletion was allowed")
	}
}

func TestImportPathsCopiesFilesAndFoldersWithoutOverwriting(t *testing.T) {
	p, err := Create(t.TempDir(), "imports")
	if err != nil {
		t.Fatal(err)
	}
	if err = p.CreateDirectory("H"); err != nil {
		t.Fatal(err)
	}
	sourceRoot := t.TempDir()
	file := filepath.Join(sourceRoot, "README.TXT")
	folder := filepath.Join(sourceRoot, "assets")
	if err = os.WriteFile(file, []byte("desktop file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Join(folder, "screens"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(folder, "screens", "TITLE.DAT"), []byte("screen"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err = p.ImportPaths("H", []string{file, folder}); err != nil {
		t.Fatal(err)
	}
	for rel, want := range map[string]string{
		"H/README.TXT":               "desktop file",
		"H/assets/screens/TITLE.DAT": "screen",
	} {
		data, readErr := os.ReadFile(filepath.Join(p.Root, filepath.FromSlash(rel)))
		if readErr != nil || string(data) != want {
			t.Fatalf("imported %s = %q, err=%v; want %q", rel, data, readErr, want)
		}
	}
	if err = p.ImportPaths("H", []string{file}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("duplicate import was not rejected: %v", err)
	}
}

func TestBinaryReadAndSave(t *testing.T) {
	p, err := Create(t.TempDir(), "binary")
	if err != nil {
		t.Fatal(err)
	}
	data := []byte{0x00, 0xff, 0x18, 0x3c, 0x00, 0x7e}
	if err := p.SaveBinary("game/assets/SPRITES.PMG", data); err != nil {
		t.Fatal(err)
	}
	got, err := p.ReadBinary("game/assets/SPRITES.PMG")
	if err != nil || string(got) != string(data) {
		t.Fatalf("round trip: %v %v", got, err)
	}
	if _, err := p.Read("game/assets/SPRITES.PMG"); err == nil {
		t.Fatal("binary data must not open as text")
	}
	if err := p.SaveBinary("../outside.bin", data); err == nil {
		t.Fatal("wrote outside the workspace")
	}
	if err := p.SaveBinary(ConfigName, data); err == nil {
		t.Fatal("overwrote the project configuration")
	}
	if err := p.SaveBinary("big.bin", make([]byte, MaxBinaryBytes+1)); err == nil {
		t.Fatal("accepted an oversized file")
	}
	if _, err := p.ReadBinary("game/assets"); err == nil {
		t.Fatal("read a directory as a file")
	}
}
