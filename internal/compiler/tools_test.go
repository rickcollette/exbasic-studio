package compiler

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"exbasicstudio/internal/project"
)

func TestToolPathUsesConfiguredToolDirectory(t *testing.T) {
	root := t.TempDir()
	name := "exbasic"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	tool := filepath.Join(root, name)
	if err := os.WriteFile(tool, []byte("tool"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EXBASIC_STUDIO_TOOLS", root)
	if got := ToolPath("exbasic"); got != tool {
		t.Fatalf("ToolPath(exbasic) = %q, want %q", got, tool)
	}
}

func TestBundledCompilerProducesLintDiagnostic(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the checked-in compiler distribution in this workspace is Windows amd64")
	}
	compiler := ToolPath("exbasic")
	if _, err := os.Stat(compiler); err != nil {
		t.Skipf("bundled compiler unavailable: %v", err)
	}
	p, err := project.Create(t.TempDir(), "lintdemo")
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Lint(p, p.Config.Entry, "PRINT +\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 || diagnostics[0].Line != 1 || diagnostics[0].Column < 1 {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}
}
