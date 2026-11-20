package appstate

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRecentProjectsAreDeduplicatedAndCapped(t *testing.T) {
	s := NewStore(t.TempDir())
	for i := 0; i < MaxRecentProjects+5; i++ {
		if err := s.AddRecent(filepath.Join("root", fmt.Sprint(i)), fmt.Sprint(i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.AddRecent(filepath.Join("root", "10"), "ten"); err != nil {
		t.Fatal(err)
	}
	recent := NewStore(s.Dir()).Snapshot("").RecentProjects
	if len(recent) != MaxRecentProjects {
		t.Fatalf("got %d recent projects, want %d", len(recent), MaxRecentProjects)
	}
	if recent[0].Name != "ten" {
		t.Fatalf("most recent = %+v", recent[0])
	}
	seen := map[string]bool{}
	for _, item := range recent {
		if seen[item.Path] {
			t.Fatalf("duplicate recent project %q", item.Path)
		}
		seen[item.Path] = true
	}
	if err := s.RemoveRecent(""); err != nil {
		t.Fatal(err)
	}
	if got := s.Snapshot("").RecentProjects; len(got) != 0 {
		t.Fatalf("recent list not cleared: %+v", got)
	}
}

func TestWorkspaceStateRoundTrips(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "proj")
	s := NewStore(dir)
	want := Workspace{Tabs: []string{"src/main.bas", "lib/a.bas"}, ActiveEditor: "lib/a.bas", EmulatorProfile: "p1", Breakpoints: []Breakpoint{{Path: "src/main.bas", Line: 3, Enabled: true}}, Watches: []string{"SCORE", "$D40A"}}
	if err := s.SaveWorkspace(root, want); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveLayout(Layout{PrimaryOpen: false, PanelOpen: true, Activity: "source", PanelTab: "output", PrimaryWidth: 333, PanelHeight: 5}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPreference("theme", "high-contrast"); err != nil {
		t.Fatal(err)
	}
	snapshot := NewStore(dir).Snapshot(root)
	if fmt.Sprint(snapshot.Workspace) != fmt.Sprint(want) {
		t.Fatalf("workspace = %+v, want %+v", snapshot.Workspace, want)
	}
	if snapshot.Layout.Activity != "source" || snapshot.Layout.PrimaryWidth != 333 {
		t.Fatalf("layout = %+v", snapshot.Layout)
	}
	if snapshot.Layout.PanelHeight != 240 {
		t.Fatalf("out-of-range panel height was not reset: %d", snapshot.Layout.PanelHeight)
	}
	if snapshot.Preferences["theme"] != "high-contrast" {
		t.Fatalf("preferences = %+v", snapshot.Preferences)
	}
	other := NewStore(dir).Snapshot(filepath.Join(dir, "other"))
	if len(other.Workspace.Tabs) != 0 || other.Workspace.Tabs == nil {
		t.Fatalf("unknown workspace should have empty, non-nil state: %+v", other.Workspace)
	}
}

func TestCorruptAndNewerStateIsSetAside(t *testing.T) {
	for name, content := range map[string]string{"corrupt": "{not json", "newer": `{"schemaVersion": 99}`} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			s := NewStore(dir)
			snapshot := s.Snapshot("")
			if snapshot.Notice == "" || snapshot.SchemaVersion != SchemaVersion {
				t.Fatalf("expected a notice and defaults, got %+v", snapshot)
			}
			if again := s.Snapshot(""); again.Notice != "" {
				t.Fatal("notice was delivered twice")
			}
			matches, _ := filepath.Glob(filepath.Join(dir, "state.json.corrupt-*"))
			if len(matches) != 1 {
				t.Fatalf("damaged state was not preserved: %v", matches)
			}
			preserved, _ := os.ReadFile(matches[0])
			if string(preserved) != content {
				t.Fatal("preserved state differs from the original")
			}
			if err := s.AddRecent(dir, "x"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTrustIsBoundToToolFingerprint(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "proj")
	s := NewStore(dir)
	original := ToolFingerprint("exbasic", "ex800-studio", "mkatr-go")
	if s.IsTrusted(root, original) {
		t.Fatal("workspace trusted by default")
	}
	if err := s.Trust(root, original); err != nil {
		t.Fatal(err)
	}
	if !NewStore(dir).IsTrusted(root, original) {
		t.Fatal("trust did not persist")
	}
	if s.IsTrusted(root, ToolFingerprint("evil.exe", "ex800-studio", "mkatr-go")) {
		t.Fatal("changing a tool path must revoke trust")
	}
	if ToolFingerprint("a", "bc") == ToolFingerprint("ab", "c") {
		t.Fatal("fingerprint does not separate fields")
	}
	if err := s.Revoke(root); err != nil {
		t.Fatal(err)
	}
	if s.IsTrusted(root, original) {
		t.Fatal("trust survived Revoke")
	}
}

func TestRecoveryLifecycle(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "proj")
	s := NewStore(dir)
	if err := s.SaveRecovery(root, "src/main.bas", "PRINT 1\n"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRecovery(root, "src/main.bas", "PRINT 2\n"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRecovery(root, `lib\util.bas`, "REM\n"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRecovery(root, "big.bas", strings.Repeat("x", maxRecoveryBytes+1)); err == nil {
		t.Fatal("oversized recovery copy was accepted")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(s.recoveryPath(root, "src/main.bas"))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("recovery copy mode = %v, err=%v", info.Mode(), err)
		}
	}
	// A damaged entry is dropped, not offered.
	if err := os.WriteFile(filepath.Join(s.recoveryDir(root), "junk.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	list, err := NewStore(dir).Recoveries(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("got %d recoveries, want 2: %+v", len(list), list)
	}
	byPath := map[string]string{}
	for _, item := range list {
		byPath[item.Path] = item.Content
	}
	if byPath["src/main.bas"] != "PRINT 2\n" || byPath["lib/util.bas"] != "REM\n" {
		t.Fatalf("recoveries = %+v", byPath)
	}
	if other, _ := s.Recoveries(filepath.Join(dir, "other")); len(other) != 0 {
		t.Fatalf("recoveries leaked between workspaces: %+v", other)
	}
	if err = s.ClearRecovery(root, "src/main.bas"); err != nil {
		t.Fatal(err)
	}
	if err = s.ClearRecovery(root, "src/main.bas"); err != nil {
		t.Fatalf("clearing a missing copy failed: %v", err)
	}
	if list, _ = s.Recoveries(root); len(list) != 1 {
		t.Fatalf("got %d recoveries after clear, want 1", len(list))
	}
	if err = s.ClearAllRecovery(); err != nil {
		t.Fatal(err)
	}
	if list, _ = s.Recoveries(root); len(list) != 0 {
		t.Fatal("ClearAllRecovery left data behind")
	}
}
