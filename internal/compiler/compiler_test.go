package compiler

import (
	"exbasicstudio/internal/project"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildPassesCorrectArguments(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub is POSIX-specific")
	}
	p, err := project.Create(t.TempDir(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(t.TempDir(), "mock-exbasic")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\"\n"
	if err := os.WriteFile(fake, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	p.Config.Compiler = fake
	result, err := Build(p)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("mock build failed: %s", result.Output)
	}
	for _, expected := range []string{"-t:atari", "-g", "-keep", "-I:", "-o", "main.bas"} {
		if !strings.Contains(result.Output, expected) {
			t.Errorf("missing %q in %s", expected, result.Output)
		}
	}
}
