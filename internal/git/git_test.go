package git

import (
	"os/exec"
	"testing"

	"exbasicstudio/internal/project"
)

func TestStatusStageAndUnstageWithoutCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	p, err := project.Create(t.TempDir(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Init(p); err != nil {
		t.Fatal(err)
	}
	status, err := GetStatus(p)
	if err != nil || !status.Available || len(status.Changes) == 0 {
		t.Fatalf("unexpected initial status: %+v, %v", status, err)
	}
	if _, err := Stage(p, "src/main.bas"); err != nil {
		t.Fatal(err)
	}
	status, err = GetStatus(p)
	if err != nil {
		t.Fatal(err)
	}
	var staged bool
	for _, change := range status.Changes {
		if change.Path == "src/main.bas" && change.Index != " " && change.Index != "?" {
			staged = true
		}
	}
	if !staged {
		t.Fatalf("file was not staged: %+v", status.Changes)
	}
	if _, err := Unstage(p, "src/main.bas"); err != nil {
		t.Fatal(err)
	}
}
