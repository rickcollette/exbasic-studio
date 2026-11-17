package logging

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogRotationKeepsBoundedHistory(t *testing.T) {
	dir := t.TempDir()
	l := New(dir)
	defer l.Close()
	line := strings.Repeat("x", 1000)
	for i := 0; i < (MaxFiles+3)*(MaxFileBytes/1000); i++ {
		l.Info("%s", line)
	}
	l.Close()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != MaxFiles {
		t.Fatalf("got %d log files, want %d", len(entries), MaxFiles)
	}
	for _, entry := range entries {
		info, _ := entry.Info()
		if info.Size() > MaxFileBytes {
			t.Fatalf("%s is %d bytes, over the limit", entry.Name(), info.Size())
		}
	}
}

func TestLogLinesCannotBeForged(t *testing.T) {
	dir := t.TempDir()
	l := New(dir)
	l.Error("first\n2099-01-01T00:00:00Z INFO  forged")
	l.Close()
	raw, _ := os.ReadFile(filepath.Join(dir, activeName))
	if strings.Count(string(raw), "\n") != 1 {
		t.Fatalf("message produced more than one line: %q", raw)
	}
}

func TestLoggerSurvivesUnwritableDirectory(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	l := New(filepath.Join(blocker, "logs"))
	l.Info("dropped")
	l.Close()
}

func TestRedactorHidesWorkspaceAndHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	workspace := filepath.Join(home, "projects", "secret-game")
	r := NewRedactor(workspace)
	text := "opened " + workspace + string(filepath.Separator) + "src" + string(filepath.Separator) + "main.bas and " + filepath.Join(home, "other.txt") +
		" and " + filepath.ToSlash(workspace) + "/lib"
	got := r.Redact(text)
	if strings.Contains(got, "secret-game") || strings.Contains(got, home) || strings.Contains(got, filepath.ToSlash(home)) {
		t.Fatalf("redaction leaked a path: %q", got)
	}
	if !strings.Contains(got, "<workspace>") || !strings.Contains(got, "<home>") {
		t.Fatalf("placeholders missing: %q", got)
	}
	if !strings.Contains(got, "main.bas") {
		t.Fatalf("relative file name should survive: %q", got)
	}
}

func TestSupportBundleIsRedactedAndExcludesSources(t *testing.T) {
	dir := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "my-workspace")
	l := New(dir)
	l.Info("build started in %s", workspace)
	l.Error("compiler failed: %s", filepath.Join(workspace, "src", "main.bas"))
	var buffer bytes.Buffer
	details := map[string]string{"workspace": workspace, "compiler": "exbasic 1.0"}
	if err := l.WriteBundle(&buffer, "1.0.0-test", details, NewRedactor(workspace)); err != nil {
		t.Fatal(err)
	}
	l.Close()
	archive, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, file := range archive.File {
		reader, _ := file.Open()
		content, _ := io.ReadAll(reader)
		reader.Close()
		names[file.Name] = string(content)
		if strings.Contains(string(content), "my-workspace") {
			t.Fatalf("%s leaks the workspace path: %s", file.Name, content)
		}
	}
	if !strings.Contains(names["manifest.json"], `"version": "1.0.0-test"`) || !strings.Contains(names["manifest.json"], "<workspace>") {
		t.Fatalf("manifest = %s", names["manifest.json"])
	}
	if !strings.Contains(names["logs/"+activeName], "compiler failed: <workspace>") {
		t.Fatalf("log = %s", names["logs/"+activeName])
	}
	for name := range names {
		if name != "manifest.json" && !strings.HasPrefix(name, "logs/") {
			t.Fatalf("unexpected bundle entry %q", name)
		}
	}
}
