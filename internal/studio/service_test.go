package studio

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"exbasicstudio/internal/compiler"
	"exbasicstudio/internal/operations"
	"exbasicstudio/internal/project"
	"exbasicstudio/internal/terminal"
	"exbasicstudio/internal/tools"
	"exbasicstudio/internal/tools/toolstest"
	"exbasicstudio/internal/watch"
)

// events records everything the service emits to the frontend.
type events struct {
	mu   sync.Mutex
	all  []any
	wake chan struct{}
}

func TestEmulatorHelperMapsLegacyNamesToEx800(t *testing.T) {
	want := tools.Path("ex800-studio")
	for _, configured := range []string{"", "atari800", "atari800-headless", "atari800-studio", "ex800", "ex800-headless"} {
		if got := emulatorHelper(configured); got != want {
			t.Errorf("emulatorHelper(%q) = %q, want %q", configured, got, want)
		}
	}
}

func (e *events) emit(_ string, data any) {
	e.mu.Lock()
	e.all = append(e.all, data)
	e.mu.Unlock()
	select {
	case e.wake <- struct{}{}:
	default:
	}
}

// completion waits for the completion event of an operation.
func (e *events) completion(t *testing.T, id string) operations.Event {
	t.Helper()
	deadline := time.After(120 * time.Second)
	for {
		e.mu.Lock()
		for _, item := range e.all {
			if event, ok := item.(operations.Event); ok && event.ID == id && event.Type == "completed" {
				e.mu.Unlock()
				return event
			}
		}
		e.mu.Unlock()
		select {
		case <-e.wake:
		case <-time.After(50 * time.Millisecond):
		case <-deadline:
			t.Fatalf("operation %s did not complete", id)
		}
	}
}

func newService(t *testing.T) (*StudioService, *events) {
	t.Helper()
	s := NewWithDir(t.TempDir())
	recorder := &events{wake: make(chan struct{}, 1)}
	Attach(s, recorder.emit)
	t.Cleanup(func() { _ = s.ServiceShutdown() })
	return s, recorder
}

func TestRenamingEntryFolderUpdatesConfig(t *testing.T) {
	s, _ := newService(t)
	if _, err := s.CreateProject(t.TempDir(), "demo"); err != nil {
		t.Fatal(err)
	}
	view, err := s.RenamePath("src", "source")
	if err != nil {
		t.Fatal(err)
	}
	if view.Config.Entry != "source/main.bas" {
		t.Fatalf("entry = %q, want source/main.bas", view.Config.Entry)
	}
	if _, err := s.ReadFile(view.Config.Entry); err != nil {
		t.Fatal(err)
	}
}

func TestOpeningAFolderWritesNothingAndIsRestricted(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "game.bas"), []byte("PRINT 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ := newService(t)
	view, err := s.OpenProject(root)
	if err != nil {
		t.Fatal(err)
	}
	if view.Config.Entry != "game.bas" || view.HasConfig {
		t.Fatalf("unexpected workspace view: %+v", view)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Fatalf("opening the folder created files: %v", entries)
	}
	trust, err := s.WorkspaceTrust()
	if err != nil || trust.Trusted {
		t.Fatalf("a newly opened folder must be restricted: %+v %v", trust, err)
	}
	// Editing is allowed in restricted mode.
	if err = s.SaveFile("game.bas", "PRINT 2\n"); err != nil {
		t.Fatal(err)
	}
	// Everything that executes workspace-controlled programs is refused.
	if _, err = s.StartBuild(BuildRequest{}); err == nil || !strings.Contains(err.Error(), "restricted mode") {
		t.Fatalf("build allowed in restricted mode: %v", err)
	}
	if _, err = s.StartRun(RunRequest{}); err == nil {
		t.Fatal("run allowed in restricted mode")
	}
	if _, err = s.TerminalOpen(80, 24); err == nil {
		t.Fatal("terminal allowed in restricted mode")
	}
	if _, err = s.GitCommit("x"); err == nil {
		t.Fatal("commit (hooks) allowed in restricted mode")
	}
	if _, err = s.StartGitNetwork("fetch", ""); err == nil {
		t.Fatal("network action allowed in restricted mode")
	}
	lint, err := s.Lint("game.bas", "PRINT +\n", 7, nil)
	if err != nil || lint.Available || lint.Version != 7 {
		t.Fatalf("lint in restricted mode = %+v, %v", lint, err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "H")); statErr == nil {
		t.Fatal("the H directory was created before a trusted run")
	}
}

func TestTrustIsExplicitAndRevokedByToolChanges(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.bas"), []byte("PRINT 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ := newService(t)
	view, err := s.OpenProject(root)
	if err != nil {
		t.Fatal(err)
	}
	trust, err := s.TrustWorkspace()
	if err != nil || !trust.Trusted || len(trust.Tools) != 3 {
		t.Fatalf("trust = %+v, %v", trust, err)
	}
	cfg := view.Config
	cfg.Debug = !cfg.Debug
	if _, err = s.UpdateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if trust, _ = s.WorkspaceTrust(); !trust.Trusted {
		t.Fatal("changing an unrelated setting revoked trust")
	}
	cfg.Compiler = filepath.Join(root, "evil-compiler")
	if _, err = s.UpdateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if trust, _ = s.WorkspaceTrust(); trust.Trusted {
		t.Fatal("changing the compiler path did not revoke trust")
	}
	if _, err = s.StartBuild(BuildRequest{}); err == nil {
		t.Fatal("build ran with an untrusted compiler path")
	}
	// The same applies when the file is edited outside Studio.
	if _, err = s.TrustWorkspace(); err != nil {
		t.Fatal(err)
	}
	cfg.Emulator = "other-emulator"
	raw, _ := json.Marshal(cfg)
	if err = os.WriteFile(filepath.Join(root, project.ConfigName), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err = s.OpenProject(root); err != nil {
		t.Fatal(err)
	}
	if trust, _ = s.WorkspaceTrust(); trust.Trusted {
		t.Fatal("an externally edited tool path kept the workspace trusted")
	}
	if _, err = s.TrustWorkspace(); err != nil {
		t.Fatal(err)
	}
	if trust, _ = s.RevokeWorkspaceTrust(); trust.Trusted {
		t.Fatal("RevokeWorkspaceTrust left the workspace trusted")
	}
}

func TestCreatedProjectIsTrustedAndHostDirIsLazy(t *testing.T) {
	s, _ := newService(t)
	view, err := s.CreateProject(t.TempDir(), "hostdrive")
	if err != nil {
		t.Fatal(err)
	}
	if trust, _ := s.WorkspaceTrust(); !trust.Trusted {
		t.Fatal("a project created in Studio should be trusted")
	}
	if _, statErr := os.Stat(filepath.Join(view.Root, "H")); statErr == nil {
		t.Fatal("H was created before the first run")
	}
	p, _ := s.current()
	hostDir, err := s.ensureHostDir(p, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(filepath.Clean(hostDir), filepath.Join(view.Root, "H")) {
		t.Fatalf("host dir = %q", hostDir)
	}
	if _, err = s.RevokeWorkspaceTrust(); err != nil {
		t.Fatal(err)
	}
	private, err := s.ensureHostDir(p, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(strings.ToLower(private), strings.ToLower(view.Root)) {
		t.Fatalf("an untrusted workspace received a host dir inside itself: %q", private)
	}
}

func TestEmulatorProfileChoosesTheHostDir(t *testing.T) {
	s, _ := newService(t)
	view, err := s.CreateProject(t.TempDir(), "hostchoice")
	if err != nil {
		t.Fatal(err)
	}
	p, _ := s.current()
	same := func(got, want string) bool { return strings.EqualFold(filepath.Clean(got), filepath.Clean(want)) }

	chosen := t.TempDir()
	if got, err := s.ensureHostDir(p, chosen); err != nil || !same(got, chosen) {
		t.Fatalf("full path: %q, %v", got, err)
	}
	if got, err := s.ensureHostDir(nil, chosen); err != nil || !same(got, chosen) {
		t.Fatalf("full path without a project: %q, %v", got, err)
	}
	if _, err = s.ensureHostDir(p, filepath.Join(chosen, "missing")); err == nil {
		t.Fatal("a missing folder was accepted")
	}
	if _, statErr := os.Stat(filepath.Join(chosen, "missing")); statErr == nil {
		t.Fatal("a missing folder outside the project was created")
	}

	// A relative folder is taken from the project and created on first use.
	if got, err := s.ensureHostDir(p, "data"); err != nil || !same(got, filepath.Join(view.Root, "data")) {
		t.Fatalf("relative path: %q, %v", got, err)
	}
	if _, err = s.RevokeWorkspaceTrust(); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ensureHostDir(p, "other"); err == nil {
		t.Fatal("a relative folder was used in an untrusted workspace")
	}
	if _, statErr := os.Stat(filepath.Join(view.Root, "other")); statErr == nil {
		t.Fatal("a folder was created in an untrusted workspace")
	}
}

func TestImportProjectPathsRefreshesWorkspaceView(t *testing.T) {
	s, _ := newService(t)
	view, err := s.CreateProject(t.TempDir(), "dropdemo")
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "DROPPED.TXT")
	if err = os.WriteFile(source, []byte("dropped"), 0o644); err != nil {
		t.Fatal(err)
	}
	view, err = s.ImportProjectPaths("lib", []string{source})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, file := range view.Files {
		found = found || file.Path == "lib/DROPPED.TXT"
	}
	if !found {
		t.Fatalf("imported file is missing from workspace view: %+v", view.Files)
	}
	if _, err = s.ImportProjectPaths("lib", []string{source}); err == nil {
		t.Fatal("a second import silently overwrote the existing file")
	}
}

func TestSearchIsBoundedAndSkipsBinaryFiles(t *testing.T) {
	s, _ := newService(t)
	view, err := s.CreateProject(t.TempDir(), "search")
	if err != nil {
		t.Fatal(err)
	}
	var many strings.Builder
	for i := 0; i < maxSearchResults+50; i++ {
		many.WriteString("PRINT \"needle\"\n")
	}
	if err = s.SaveFile("src/many.bas", many.String()); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(view.Root, "blob.bin"), []byte("needle\x00needle"), 0o644); err != nil {
		t.Fatal(err)
	}
	results, err := s.SearchProject("NEEDLE")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != maxSearchResults {
		t.Fatalf("got %d results, want the cap of %d", len(results), maxSearchResults)
	}
	for _, result := range results {
		if result.Path == "blob.bin" {
			t.Fatal("a binary file was searched")
		}
	}
	if short, _ := s.SearchProject("n"); len(short) != 0 {
		t.Fatal("a one-character query returned results")
	}
	if _, err = s.SearchProject(strings.Repeat("x", maxSearchQueryRunes+1)); err == nil {
		t.Fatal("an oversized query was accepted")
	}
}

func TestRecoveryIsOfferedOnlyWhenItDiffersFromDisk(t *testing.T) {
	s, _ := newService(t)
	if _, err := s.CreateProject(t.TempDir(), "recover"); err != nil {
		t.Fatal(err)
	}
	onDisk, _ := s.ReadFile("src/main.bas")
	if err := s.SaveRecovery("src/main.bas", onDisk); err != nil {
		t.Fatal(err)
	}
	if list, err := s.Recoveries(); err != nil || len(list) != 0 {
		t.Fatalf("an identical recovery copy was offered: %+v %v", list, err)
	}
	if err := s.SaveRecovery("src/main.bas", "PRINT \"UNSAVED\"\n"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRecovery("src/never-saved.bas", "REM NEW\n"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRecovery("../escape.bas", "x"); err == nil {
		t.Fatal("recovery accepted a path outside the workspace")
	}
	list, err := s.Recoveries()
	if err != nil || len(list) != 2 {
		t.Fatalf("recoveries = %+v, %v", list, err)
	}
	// A successful save removes the copy.
	if err = s.SaveFile("src/main.bas", "PRINT \"UNSAVED\"\n"); err != nil {
		t.Fatal(err)
	}
	if list, _ = s.Recoveries(); len(list) != 1 || list[0].Path != "src/never-saved.bas" {
		t.Fatalf("after save: %+v", list)
	}
	if err = s.DiscardRecovery("src/never-saved.bas"); err != nil {
		t.Fatal(err)
	}
	if list, _ = s.Recoveries(); len(list) != 0 {
		t.Fatalf("after discard: %+v", list)
	}
	if err = s.SaveRecovery("src/main.bas", "again\n"); err != nil {
		t.Fatal(err)
	}
	if err = s.ClearRecoveryData(); err != nil {
		t.Fatal(err)
	}
	if list, _ = s.Recoveries(); len(list) != 0 {
		t.Fatal("ClearRecoveryData left copies behind")
	}
}

func TestAppStateRemembersRecentProjectsAndWorkspace(t *testing.T) {
	dir := t.TempDir()
	s := NewWithDir(dir)
	view, err := s.CreateProject(t.TempDir(), "remember")
	if err != nil {
		t.Fatal(err)
	}
	state := s.AppState()
	state.Workspace.Tabs = []string{"src/main.bas"}
	state.Workspace.ActiveEditor = "src/main.bas"
	if err = s.SaveWorkspaceState(state.Workspace); err != nil {
		t.Fatal(err)
	}
	_ = s.ServiceShutdown()

	reopened := NewWithDir(dir)
	t.Cleanup(func() { _ = reopened.ServiceShutdown() })
	before := reopened.AppState()
	if len(before.RecentProjects) != 1 || before.RecentProjects[0].Path != view.Root || before.RecentProjects[0].Name != "remember" {
		t.Fatalf("recent projects = %+v", before.RecentProjects)
	}
	if _, err = reopened.OpenProject(view.Root); err != nil {
		t.Fatal(err)
	}
	after := reopened.AppState()
	if after.Workspace.ActiveEditor != "src/main.bas" || len(after.Workspace.Tabs) != 1 {
		t.Fatalf("workspace state not restored: %+v", after.Workspace)
	}
	if trust, _ := reopened.WorkspaceTrust(); !trust.Trusted {
		t.Fatal("trust did not persist across restarts")
	}
	if _, err = os.Stat(filepath.Join(view.Root, "state.json")); err == nil {
		t.Fatal("application state was written inside the project")
	}
}

func TestWatcherEventsReachTheFrontend(t *testing.T) {
	s, recorder := newService(t)
	view, err := s.CreateProject(t.TempDir(), "watched")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(view.Root, "src", "external.bas"), []byte("REM\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		recorder.mu.Lock()
		for _, item := range recorder.all {
			if batch, ok := item.(watch.Batch); ok {
				for _, change := range batch.Changes {
					if change.Path == "src/external.bas" {
						recorder.mu.Unlock()
						return
					}
				}
			}
		}
		recorder.mu.Unlock()
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("an external file creation was not reported")
}

func TestTerminalSessionThroughTheService(t *testing.T) {
	s, recorder := newService(t)
	if _, err := s.CreateProject(t.TempDir(), "term"); err != nil {
		t.Fatal(err)
	}
	info, err := s.TerminalOpen(90, 25)
	if err != nil {
		t.Fatal(err)
	}
	newline := "\n"
	if runtime.GOOS == "windows" {
		newline = "\r"
	}
	if err = s.TerminalWrite(info.ID, "echo service-"+"terminal"+newline); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(60 * time.Second)
	seen := false
	for !seen && time.Now().Before(deadline) {
		var output strings.Builder
		recorder.mu.Lock()
		for _, item := range recorder.all {
			if event, ok := item.(terminal.Event); ok && event.ID == info.ID {
				output.WriteString(event.Data)
			}
		}
		recorder.mu.Unlock()
		seen = strings.Contains(output.String(), "service-terminal")
		time.Sleep(50 * time.Millisecond)
	}
	if !seen {
		t.Fatal("terminal output did not reach the frontend")
	}
	if err = s.TerminalResize(info.ID, 100, 30); err != nil {
		t.Fatal(err)
	}
	if err = s.TerminalClose(info.ID); err != nil {
		t.Fatal(err)
	}
	if len(s.TerminalList()) != 0 {
		t.Fatal("closed terminal still listed")
	}
}

func TestSupportBundleRedactsWorkspacePath(t *testing.T) {
	s, _ := newService(t)
	view, err := s.CreateProject(t.TempDir(), "bundleproject")
	if err != nil {
		t.Fatal(err)
	}
	s.LogFrontendError("failed to open " + filepath.Join(view.Root, "src", "main.bas"))
	target := filepath.Join(t.TempDir(), "support")
	if err = s.ExportSupportBundle(target); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(target + ".zip")
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	sawLog := false
	for _, file := range archive.File {
		reader, _ := file.Open()
		buffer := make([]byte, file.UncompressedSize64)
		_, _ = reader.Read(buffer)
		reader.Close()
		if strings.Contains(string(buffer), "bundleproject") {
			t.Fatalf("%s leaks the workspace path", file.Name)
		}
		if strings.HasPrefix(file.Name, "logs/") && strings.Contains(string(buffer), "frontend: failed to open <workspace>") {
			sawLog = true
		}
	}
	if !sawLog {
		t.Fatal("the redacted log entry is missing from the bundle")
	}
	if err = s.ExportSupportBundle("relative.zip"); err == nil {
		t.Fatal("a relative bundle path was accepted")
	}
}

func TestATRExtractionNeverOverwritesSilently(t *testing.T) {
	tool := toolstest.Tool(t, "mkatr-go")
	_ = tool
	s, _ := newService(t)
	view, err := s.CreateProject(t.TempDir(), "atr")
	if err != nil {
		t.Fatal(err)
	}
	image := filepath.Join(t.TempDir(), "disk.atr")
	source := filepath.Join(t.TempDir(), "DATA.TXT")
	if err = os.WriteFile(source, []byte("from image"), 0o644); err != nil {
		t.Fatal(err)
	}
	// mkatr-go creates the image when importing into a new file is not
	// supported, so build it through the explorer API used by the UI.
	if _, err = s.ATRImportFiles(image, "", []string{source}); err != nil {
		t.Skipf("cannot create a test image through import: %v", err)
	}
	if err = os.WriteFile(filepath.Join(view.Root, "lib", "DATA.TXT"), []byte("local work"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ATRExtractToProject(image, "DATA.TXT", "lib", false); err == nil || !strings.HasPrefix(err.Error(), "exists:") {
		t.Fatalf("extraction over an existing file: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(view.Root, "lib", "DATA.TXT")); string(data) != "local work" {
		t.Fatalf("existing file was modified: %q", data)
	}
	if _, err = s.ATRExtractToProject(image, "DATA.TXT", "lib", true); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(view.Root, "lib", "DATA.TXT")); string(data) != "from image" {
		t.Fatalf("confirmed overwrite did not replace the file: %q", data)
	}
	if _, err = s.ATROpen("relative.atr"); err == nil {
		t.Fatal("a relative image path was accepted")
	}
}

// ---- Real toolchain --------------------------------------------------------

func TestBuildOperationWithBundledToolchain(t *testing.T) {
	toolstest.Tool(t, "exbasic")
	s, recorder := newService(t)
	view, err := s.CreateProject(t.TempDir(), "opbuild")
	if err != nil {
		t.Fatal(err)
	}
	info, err := s.StartBuild(BuildRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.StartBuild(BuildRequest{}); err == nil {
		// The first build may already have finished on a fast machine.
		t.Log("second build was accepted; the first had already completed")
	}
	completed := recorder.completion(t, info.ID)
	result, ok := completed.Result.(compiler.Result)
	if !ok || completed.Error != "" {
		t.Fatalf("completion = %+v", completed)
	}
	if !result.Success {
		t.Fatalf("build failed:\n%s", result.Output)
	}
	if _, err = os.Stat(filepath.Join(view.Root, "build", "opbuild.xex")); err != nil {
		t.Fatal(err)
	}

	// A syntax error yields structured diagnostics, not an operation error.
	if err = s.SaveFile("src/main.bas", "PRINT +\n"); err != nil {
		t.Fatal(err)
	}
	info, err = s.StartBuild(BuildRequest{})
	if err != nil {
		t.Fatal(err)
	}
	completed = recorder.completion(t, info.ID)
	result = completed.Result.(compiler.Result)
	if result.Success || len(result.Diagnostics) == 0 || result.Diagnostics[0].Path != "src/main.bas" || result.Diagnostics[0].Line != 1 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
}

func TestRunOperationStartsAndStopsTheEmulator(t *testing.T) {
	toolstest.Tool(t, "exbasic")
	toolstest.Tool(t, "ex800-studio")
	toolstest.Tool(t, "mkatr-go")
	s, recorder := newService(t)
	view, err := s.CreateProject(t.TempDir(), "oprun")
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := s.EmulatorProfiles()
	if err != nil || len(profiles) == 0 {
		t.Fatalf("profiles: %v %v", profiles, err)
	}
	info, err := s.StartRun(RunRequest{ProfileID: profiles[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	completed := recorder.completion(t, info.ID)
	result, ok := completed.Result.(RunResult)
	if !ok || completed.Error != "" || !result.Started || !result.Emulator.Running {
		t.Fatalf("run completion = %+v", completed)
	}
	if _, statErr := os.Stat(filepath.Join(view.Root, "H")); statErr != nil {
		t.Fatal("the H directory was not created by the first trusted run")
	}
	frame, err := s.EmulatorFrame()
	if err != nil || !frame.Running || frame.Image == "" {
		t.Fatalf("frame = %+v, %v", frame.Number, err)
	}
	if err = s.EmulatorStop(); err != nil {
		t.Fatal(err)
	}
	if s.EmulatorStatus().Running {
		t.Fatal("emulator still running after stop")
	}
	if _, ok := tools.Root(); !ok {
		t.Fatal("tool root vanished")
	}
}
