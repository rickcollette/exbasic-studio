package studio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"exbasicstudio/internal/project"
)

// A folder that cannot be opened must leave the previous workspace in place.
// Otherwise the window keeps showing one folder while saves land in another.
func TestFailedOpenKeepsThePreviousWorkspace(t *testing.T) {
	service, _ := newService(t)
	good := t.TempDir()
	if err := os.WriteFile(filepath.Join(good, "main.bas"), []byte("PRINT 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	opened, err := service.OpenProject(good)
	if err != nil {
		t.Fatal(err)
	}

	huge := t.TempDir()
	for i := 0; i <= project.MaxProjectFiles; i++ {
		if err := os.WriteFile(filepath.Join(huge, fmt.Sprintf("f%05d.txt", i)), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = service.OpenProject(huge); !errors.Is(err, project.ErrTooLarge) {
		t.Fatalf("opening an oversized folder: got %v, want ErrTooLarge", err)
	}

	current, err := service.GetProject()
	if err != nil {
		t.Fatal(err)
	}
	if current.Root != opened.Root {
		t.Fatalf("active workspace changed to %s after a failed open; want %s", current.Root, opened.Root)
	}
	if err = service.SaveFile("after.inc", "x\n"); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(good, "after.inc")); err != nil {
		t.Fatalf("the save did not land in the open workspace: %v", err)
	}
	if _, err = os.Stat(filepath.Join(huge, "after.inc")); err == nil {
		t.Fatal("the save landed in the folder that failed to open")
	}
	for _, recent := range service.AppState().RecentProjects {
		if recent.Path == huge {
			t.Fatal("a folder that failed to open was added to the recent projects")
		}
	}
}
