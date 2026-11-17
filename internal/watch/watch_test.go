package watch

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type collector struct {
	mu      sync.Mutex
	changes map[string]Change
}

func (c *collector) sink(batch Batch) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, change := range batch.Changes {
		c.changes[change.Path] = change
	}
}

func (c *collector) wait(t *testing.T, path, kind string) Change {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		change, ok := c.changes[path]
		c.mu.Unlock()
		if ok && change.Kind == kind {
			return change
		}
		time.Sleep(25 * time.Millisecond)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	t.Fatalf("no %s change for %q; saw %+v", kind, path, c.changes)
	return Change{}
}

func (c *collector) has(path string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.changes[path]
	return ok
}

func TestWatcherReportsChangesAndIgnoresGeneratedDirectories(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"src", "build", ".git", "node_modules"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "src", "main.bas"), []byte("PRINT 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &collector{changes: map[string]Change{}}
	w, err := Start(root, c.sink)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	if err = os.WriteFile(filepath.Join(root, "src", "main.bas"), []byte("PRINT 22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if change := c.wait(t, "src/main.bas", "write"); change.Size != 9 || change.Directory {
		t.Fatalf("write change = %+v", change)
	}
	if err = os.WriteFile(filepath.Join(root, "src", "new.bas"), []byte("REM\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.wait(t, "src/new.bas", "create")

	// A new directory is watched too, including files created inside it.
	if err = os.Mkdir(filepath.Join(root, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if change := c.wait(t, "lib", "create"); !change.Directory {
		t.Fatalf("directory change = %+v", change)
	}
	time.Sleep(300 * time.Millisecond)
	if err = os.WriteFile(filepath.Join(root, "lib", "util.bas"), []byte("REM\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.wait(t, "lib/util.bas", "create")

	if err = os.Remove(filepath.Join(root, "src", "new.bas")); err != nil {
		t.Fatal(err)
	}
	c.wait(t, "src/new.bas", "remove")

	for _, ignored := range []string{"build/out.xex", ".git/index", "node_modules/x.js", "src/.exbasic-1.tmp"} {
		if err = os.WriteFile(filepath.Join(root, filepath.FromSlash(ignored)), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err = os.WriteFile(filepath.Join(root, "sentinel.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.wait(t, "sentinel.txt", "create")
	for _, ignored := range []string{"build/out.xex", ".git/index", "node_modules/x.js", "src/.exbasic-1.tmp", "build"} {
		if c.has(ignored) {
			t.Fatalf("ignored path %q was reported", ignored)
		}
	}
}

func TestStopIsIdempotentAndSilencesEvents(t *testing.T) {
	root := t.TempDir()
	c := &collector{changes: map[string]Change{}}
	w, err := Start(root, c.sink)
	if err != nil {
		t.Fatal(err)
	}
	w.Stop()
	w.Stop()
	if err = os.WriteFile(filepath.Join(root, "late.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	if c.has("late.txt") {
		t.Fatal("event delivered after Stop")
	}
}

func TestIgnored(t *testing.T) {
	for name, want := range map[string]bool{"build": true, "node_modules": true, "vendor": true, ".git": true, "src": false, "H": false, "builder": false} {
		if Ignored(name) != want {
			t.Errorf("Ignored(%q) = %v", name, !want)
		}
	}
}
