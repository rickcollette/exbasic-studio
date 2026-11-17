package project

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAlphaConfigMigratesInMemoryOnly(t *testing.T) {
	root := t.TempDir()
	alpha := `{"name":"old","entry":"main.bas","target":"atari","output":"build/old.xex","compiler":"exbasic","emulator":"atari800","native":"auto","debug":true,"includePaths":[],"assembly":[]}`
	path := filepath.Join(root, ConfigName)
	if err := os.WriteFile(path, []byte(alpha), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if p.Config.SchemaVersion != CurrentSchemaVersion || !p.HasConfig {
		t.Fatalf("config not migrated in memory: %+v", p.Config)
	}
	if p.Config.ATR.Tool != "mkatr-go" || !p.Config.ATR.Boot {
		t.Fatalf("alpha ATR defaults missing: %+v", p.Config.ATR)
	}
	onDisk, _ := os.ReadFile(path)
	if string(onDisk) != alpha {
		t.Fatal("opening an alpha project rewrote its configuration")
	}
	if err = p.SaveConfig(); err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	onDisk, _ = os.ReadFile(path)
	if err = json.Unmarshal(onDisk, &saved); err != nil {
		t.Fatal(err)
	}
	if saved["schemaVersion"] != float64(CurrentSchemaVersion) {
		t.Fatalf("saved schemaVersion = %v", saved["schemaVersion"])
	}
}

func TestNewerSchemaIsRefused(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig("future")
	cfg.SchemaVersion = CurrentSchemaVersion + 1
	raw, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(root, ConfigName), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root); err == nil || !strings.Contains(err.Error(), "schema version") {
		t.Fatalf("newer schema was accepted: %v", err)
	}
}

func TestPlainFolderHasNoConfig(t *testing.T) {
	p, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if p.HasConfig {
		t.Fatal("plain folder reports project metadata")
	}
}

func FuzzValidRel(f *testing.F) {
	for _, seed := range []string{"src/main.bas", "../x", `..\x`, "/abs", `C:\x`, "a/../../b", "a\x00b", "a:stream", "", ".", "a//b"} {
		f.Add(seed)
	}
	root := f.TempDir()
	p := &Project{Root: root}
	f.Fuzz(func(t *testing.T, rel string) {
		path, err := p.Resolve(rel)
		if err != nil {
			return
		}
		inside, relErr := filepath.Rel(root, path)
		if relErr != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(os.PathSeparator)) || filepath.IsAbs(inside) {
			t.Fatalf("Resolve(%q) escaped the root: %q", rel, path)
		}
	})
}

// makeLink creates a directory link that needs no special privilege: a
// junction on Windows, a symbolic link elsewhere.
func makeLink(t *testing.T, target, link string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
			t.Skipf("cannot create a junction: %v %s", err, out)
		}
		return
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create a symlink: %v", err)
	}
}

func TestLinksAndReparsePointsAreRejectedAtEveryComponent(t *testing.T) {
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Create(t.TempDir(), "linked")
	if err != nil {
		t.Fatal(err)
	}
	makeLink(t, outside, filepath.Join(p.Root, "escape"))

	if _, err = p.Read("escape/secret.txt"); err == nil {
		t.Fatal("read through a link succeeded")
	}
	if err = p.Save("escape/new.bas", "PRINT 1\n"); err == nil {
		t.Fatal("write through a link succeeded")
	}
	if _, statErr := os.Stat(filepath.Join(outside, "new.bas")); statErr == nil {
		t.Fatal("a file was created outside the workspace")
	}
	if err = p.Delete("escape/secret.txt"); err == nil {
		t.Fatal("delete through a link succeeded")
	}
	if err = p.Rename("src/main.bas", "escape/main.bas"); err == nil {
		t.Fatal("rename through a link succeeded")
	}
	files, err := p.Files()
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasPrefix(file.Path, "escape") {
			t.Fatalf("listing followed a link: %q", file.Path)
		}
	}
	// Importing a tree that contains a link is refused and leaves nothing behind.
	source := filepath.Join(t.TempDir(), "tree")
	if err = os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	makeLink(t, outside, filepath.Join(source, "inner"))
	if err = p.ImportPaths("lib", []string{source}); err == nil {
		t.Fatal("import of a tree containing a link succeeded")
	}
	if _, statErr := os.Lstat(filepath.Join(p.Root, "lib", "tree")); statErr == nil {
		t.Fatal("a failed import left a partial copy")
	}
}

func TestBinaryAndInvalidTextIsNotEditable(t *testing.T) {
	p, err := Create(t.TempDir(), "textual")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(p.Root, "image.bin"), []byte{0x41, 0x00, 0x42}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(p.Root, "latin1.txt"), []byte{0xE9, 0x74, 0xE9}, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"image.bin", "latin1.txt"} {
		if _, err = p.Read(name); err == nil {
			t.Fatalf("%s was opened as text", name)
		}
	}
	if err = p.Save("src/bad.bas", "A\x00B"); err == nil {
		t.Fatal("content with NUL was saved")
	}
	if err = p.Save("src/ok.bas", "PRINT \"héllo\"\n"); err != nil {
		t.Fatal(err)
	}
	if got, readErr := p.Read("src/ok.bas"); readErr != nil || got != "PRINT \"héllo\"\n" {
		t.Fatalf("round trip = %q, %v", got, readErr)
	}
}

func TestSaveReplacesAtomicallyWithoutLeavingTemporaries(t *testing.T) {
	p, err := Create(t.TempDir(), "atomic")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if err = p.Save("src/main.bas", strings.Repeat("PRINT 1\n", i+1)); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(p.Root, "src"))
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("temporary file left behind: %s", entry.Name())
		}
	}
	if got, _ := p.Read("src/main.bas"); got != strings.Repeat("PRINT 1\n", 5) {
		t.Fatalf("unexpected content %q", got)
	}
}

func TestImportFileCountIsCapped(t *testing.T) {
	p, err := Create(t.TempDir(), "capped")
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "many")
	if err = os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		if err = os.WriteFile(filepath.Join(source, string(rune('a'+i))+".txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	budget := &importBudget{files: 5, bytes: MaxImportBytes}
	info, _ := os.Lstat(source)
	target := filepath.Join(p.Root, "many")
	if err = copyImportedPath(source, target, info, budget); err == nil {
		t.Fatal("import over the file limit succeeded")
	}
	budget = &importBudget{files: 100, bytes: 3}
	_ = os.RemoveAll(target)
	if err = copyImportedPath(source, target, info, budget); err == nil {
		t.Fatal("import over the byte limit succeeded")
	}
}
