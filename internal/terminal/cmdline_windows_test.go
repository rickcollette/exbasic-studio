package terminal

import (
	"strings"
	"testing"
)

func TestCommandLine(t *testing.T) {
	line, err := CommandLine(`C:\Program Files\claude.exe`, []string{"--add-dir", `C:\a b\manual`, `say "hi"`})
	if err != nil {
		t.Fatal(err)
	}
	if want := `"C:\Program Files\claude.exe" --add-dir "C:\a b\manual" "say \"hi\""`; line != want {
		t.Fatalf("got %s, want %s", line, want)
	}
	line, err = CommandLine(`C:\npm\claude.cmd`, []string{"--add-dir", `C:\a b`})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(line, ` /d /s /c ""C:\npm\claude.cmd" "--add-dir" "C:\a b""`) {
		t.Fatalf("batch command line: %s", line)
	}
	if _, err = CommandLine(`C:\npm\claude.cmd`, []string{"a & calc"}); err == nil {
		t.Fatal("a cmd.exe metacharacter was accepted for a batch file")
	}
}
