package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func fakeExecutable(t *testing.T, path string) {
	t.Helper()
	touch(t, path)
	previous := executablePath
	executablePath = func() (string, error) { return path, nil }
	t.Cleanup(func() { executablePath = previous })
}

func TestOverrideDirectoryWins(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, exe("mkatr-go")))
	touch(t, filepath.Join(root, "exbasic", exe("exbasic")))
	touch(t, filepath.Join(root, "EOS.car"))
	t.Setenv(OverrideEnv, root)
	if got := Path("mkatr-go"); got != filepath.Join(root, exe("mkatr-go")) {
		t.Fatalf("Path(mkatr-go) = %q", got)
	}
	if got := Path("exbasic"); got != filepath.Join(root, "exbasic", exe("exbasic")) {
		t.Fatalf("Path(exbasic) = %q", got)
	}
	if got, ok := Asset("EOS.car"); !ok || got != filepath.Join(root, "EOS.car") {
		t.Fatalf("Asset = %q, %v", got, ok)
	}
	if _, ok := Asset("missing.car"); ok {
		t.Fatal("missing asset reported as present")
	}
}

func TestInvalidOverrideDoesNotFallBack(t *testing.T) {
	app := t.TempDir()
	fakeExecutable(t, filepath.Join(app, exe("exbasic-studio")))
	touch(t, filepath.Join(app, "tools", exe("mkatr-go")))
	t.Setenv(OverrideEnv, filepath.Join(app, "does-not-exist"))
	if _, ok := Root(); ok {
		t.Fatal("an invalid override silently fell back to packaged tools")
	}
}

func TestPackagedLayouts(t *testing.T) {
	t.Setenv(OverrideEnv, "")
	layouts := map[string][2]string{
		"windows-portable": {"exbasic-studio", "tools"},
		"macos-bundle":     {"Studio.app/Contents/MacOS/exbasic-studio", "Studio.app/Contents/Resources/tools"},
		"linux-package":    {"usr/bin/exbasic-studio", "usr/lib/exbasic-studio/tools"},
	}
	for name, layout := range layouts {
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			if resolved, err := filepath.EvalSymlinks(base); err == nil {
				base = resolved
			}
			fakeExecutable(t, filepath.Join(base, filepath.FromSlash(layout[0])))
			toolRoot := filepath.Join(base, filepath.FromSlash(layout[1]))
			touch(t, filepath.Join(toolRoot, exe("ex800-studio")))
			root, ok := Root()
			if !ok || root != toolRoot {
				t.Fatalf("Root() = %q, %v; want %q", root, ok, toolRoot)
			}
			if got, found := Bundled("ex800-studio"); !found || got != filepath.Join(toolRoot, exe("ex800-studio")) {
				t.Fatalf("Bundled = %q, %v", got, found)
			}
		})
	}
}

func TestNoSourceTreeFallbackAndExplicitPaths(t *testing.T) {
	t.Setenv(OverrideEnv, "")
	fakeExecutable(t, filepath.Join(t.TempDir(), exe("exbasic-studio")))
	if _, ok := Root(); ok {
		t.Fatal("tools were discovered without packaged resources or an override")
	}
	if got := Path("definitely-not-a-real-tool-xyz"); got != "definitely-not-a-real-tool-xyz" {
		t.Fatalf("unresolved name changed to %q", got)
	}
	explicit := filepath.Join(t.TempDir(), "custom", exe("exbasic"))
	if got := Path(explicit); got != explicit {
		t.Fatalf("explicit path changed to %q", got)
	}
	if !IsExplicitPath("./exbasic") || IsExplicitPath("exbasic") {
		t.Fatal("IsExplicitPath misclassified a name")
	}
}
