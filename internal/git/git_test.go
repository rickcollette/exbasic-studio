package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"exbasicstudio/internal/project"
)

func newRepo(t *testing.T) Repo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	// Isolate from the developer's and CI's global configuration.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.invalid")
	if err := os.WriteFile(filepath.Join(home, "gitconfig"), []byte("[init]\n\tdefaultBranch = main\n[safe]\n\tdirectory = *\n[core]\n\tautocrlf = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := project.Create(t.TempDir(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	r := Repo{Project: p, Trusted: true}
	if _, err = r.Init(); err != nil {
		t.Fatal(err)
	}
	return r
}

func write(t *testing.T, r Repo, rel, content string) {
	t.Helper()
	path := filepath.Join(r.Project.Root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitAll(t *testing.T, r Repo, message string) {
	t.Helper()
	if _, err := r.StageAll(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Commit(message); err != nil {
		t.Fatal(err)
	}
}

func find(status Status, path string) *Change {
	for i := range status.Changes {
		if status.Changes[i].Path == path {
			return &status.Changes[i]
		}
	}
	return nil
}

func TestParseStatusV2(t *testing.T) {
	raw := strings.Join([]string{
		"# branch.oid 1234567890abcdef",
		"# branch.head feature/x",
		"# branch.upstream origin/feature/x",
		"# branch.ab +2 -3",
		"1 .M N... 100644 100644 100644 aaa bbb src/with space.bas",
		"1 A. N... 000000 100644 100644 000 ccc line\nbreak.txt",
		"2 R. N... 100644 100644 100644 ddd ddd R100 new name.bas",
		"old name.bas",
		"u UU N... 100644 100644 100644 100644 e1 e2 e3 conflict -> arrow.bas",
		"? ünïcode \"quoted\".txt",
		"",
	}, "\x00")
	status, err := ParseStatus([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if status.Branch != "feature/x" || status.Upstream != "origin/feature/x" || status.Ahead != 2 || status.Behind != 3 || status.Unborn || status.Detached {
		t.Fatalf("header = %+v", status)
	}
	if len(status.Changes) != 5 {
		t.Fatalf("got %d changes: %+v", len(status.Changes), status.Changes)
	}
	if c := find(status, "src/with space.bas"); c == nil || c.Staged || c.Worktree != "M" {
		t.Fatalf("modified = %+v", c)
	}
	if c := find(status, "line\nbreak.txt"); c == nil || !c.Staged || c.Index != "A" {
		t.Fatalf("added with newline = %+v", c)
	}
	if c := find(status, "new name.bas"); c == nil || c.OrigPath != "old name.bas" || c.Index != "R" {
		t.Fatalf("rename = %+v", c)
	}
	if c := find(status, "conflict -> arrow.bas"); c == nil || !c.Conflicted || c.Staged {
		t.Fatalf("conflict = %+v", c)
	}
	if c := find(status, "ünïcode \"quoted\".txt"); c == nil || !c.Untracked {
		t.Fatalf("untracked = %+v", c)
	}
	for _, bad := range []string{"1 .M short", "2 R. N... 1 2 3 a b R100 lonely", "x unknown"} {
		if _, err = ParseStatus([]byte(bad + "\x00")); err == nil {
			t.Fatalf("malformed record %q was accepted", bad)
		}
	}
	if s, _ := ParseStatus([]byte("# branch.oid (initial)\x00# branch.head main\x00")); !s.Unborn || s.Branch != "main" {
		t.Fatalf("unborn = %+v", s)
	}
	if s, _ := ParseStatus([]byte("# branch.head (detached)\x00")); !s.Detached {
		t.Fatalf("detached = %+v", s)
	}
}

func FuzzParseStatus(f *testing.F) {
	f.Add([]byte("# branch.head main\x001 .M N... 100644 100644 100644 a b f\x00? x\x00"))
	f.Add([]byte("2 R. N... 1 2 3 a b R100 n\x00o\x00"))
	f.Add([]byte("# branch.ab + -\x00"))
	f.Fuzz(func(t *testing.T, data []byte) {
		status, err := ParseStatus(data)
		if err == nil && status.Changes == nil {
			t.Fatal("nil change list")
		}
	})
}

func TestUnbornRepositoryStageUnstageCommit(t *testing.T) {
	r := newRepo(t)
	write(t, r, "src/odd name ü.bas", "PRINT 1\n")
	status, err := r.Status()
	if err != nil || !status.Available || !status.Unborn || status.Branch != "main" {
		t.Fatalf("status = %+v, %v", status, err)
	}
	if c := find(status, "src/odd name ü.bas"); c == nil || !c.Untracked {
		t.Fatalf("untracked non-ASCII file missing: %+v", status.Changes)
	}
	if _, err = r.Stage("src/odd name ü.bas"); err != nil {
		t.Fatal(err)
	}
	status, _ = r.Status()
	if c := find(status, "src/odd name ü.bas"); c == nil || !c.Staged {
		t.Fatalf("not staged: %+v", status.Changes)
	}
	if _, err = r.Unstage("src/odd name ü.bas"); err != nil {
		t.Fatal(err)
	}
	status, _ = r.Status()
	if c := find(status, "src/odd name ü.bas"); c == nil || c.Staged {
		t.Fatalf("not unstaged: %+v", status.Changes)
	}
	if _, err = r.Commit("  "); err == nil {
		t.Fatal("empty commit message accepted")
	}
	commitAll(t, r, "initial")
	status, _ = r.Status()
	if status.Unborn || len(status.Changes) != 0 {
		t.Fatalf("after commit: %+v", status)
	}
	if _, err = r.Stage("../outside"); err == nil {
		t.Fatal("path traversal accepted")
	}
}

func TestRenameAndDiff(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a file.bas", "PRINT 1\nPRINT 2\nPRINT 3\n")
	commitAll(t, r, "initial")
	if err := os.Rename(filepath.Join(r.Project.Root, "a file.bas"), filepath.Join(r.Project.Root, "b file.bas")); err != nil {
		t.Fatal(err)
	}
	if _, err := r.StageAll(); err != nil {
		t.Fatal(err)
	}
	status, _ := r.Status()
	if c := find(status, "b file.bas"); c == nil || c.OrigPath != "a file.bas" {
		t.Fatalf("rename not reported: %+v", status.Changes)
	}
	write(t, r, "b file.bas", "PRINT 1\nPRINT 22\nPRINT 3\n")
	diff, err := r.Diff("b file.bas", false)
	if err != nil || !strings.Contains(diff, "+PRINT 22") {
		t.Fatalf("diff = %q, %v", diff, err)
	}
	staged, err := r.Diff("", true)
	if err != nil || !strings.Contains(staged, "rename to") && !strings.Contains(staged, "b file.bas") {
		t.Fatalf("staged diff = %q, %v", staged, err)
	}
}

func TestDiffNeverRunsExternalDriversOrTextconv(t *testing.T) {
	r := newRepo(t)
	marker := filepath.Join(t.TempDir(), "ran")
	script := filepath.Join(t.TempDir(), "driver.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho ran > '"+filepath.ToSlash(marker)+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	driver := "sh '" + filepath.ToSlash(script) + "'"
	for _, kv := range [][2]string{{"diff.external", driver}, {"diff.evil.textconv", driver}, {"diff.evil.command", driver}} {
		if _, err := r.call("config", kv[0], kv[1]); err != nil {
			t.Fatal(err)
		}
	}
	write(t, r, ".gitattributes", "*.bas diff=evil\n")
	write(t, r, "x.bas", "PRINT 1\n")
	commitAll(t, r, "initial")
	write(t, r, "x.bas", "PRINT 2\n")
	diff, err := r.Diff("x.bas", false)
	if err != nil || !strings.Contains(diff, "+PRINT 2") {
		t.Fatalf("diff = %q, %v", diff, err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("a repository-configured diff program was executed")
	}
}

func TestHooksRunOnlyWhenTrusted(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Git for Windows runs hooks through its bundled sh; keep the check.
		if _, err := exec.LookPath("sh"); err != nil {
			t.Skip("no sh available for hooks")
		}
	}
	r := newRepo(t)
	marker := filepath.Join(r.Project.Root, "hook-ran")
	hook := filepath.Join(r.Project.Root, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho ran > hook-ran\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	untrusted := Repo{Project: r.Project, Trusted: false}
	write(t, r, "a.bas", "PRINT 1\n")
	if _, err := untrusted.Stage("a.bas"); err != nil {
		t.Fatal(err)
	}
	if _, err := untrusted.Commit("untrusted"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("hook ran in an untrusted workspace")
	}
	write(t, r, "a.bas", "PRINT 2\n")
	if _, err := r.Stage("a.bas"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Commit("trusted"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("hook did not run in a trusted workspace")
	}
}

func TestBranches(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a.bas", "PRINT 1\n")
	commitAll(t, r, "initial")
	if _, err := r.CreateBranch("feature/one", true); err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateBranch("idle", false); err != nil {
		t.Fatal(err)
	}
	branches, err := r.Branches()
	if err != nil {
		t.Fatal(err)
	}
	current := ""
	names := map[string]bool{}
	for _, branch := range branches {
		names[branch.Name] = true
		if branch.Current {
			current = branch.Name
		}
	}
	if current != "feature/one" || !names["main"] || !names["idle"] {
		t.Fatalf("branches = %+v", branches)
	}
	for _, bad := range []string{"", "-D", "a..b", "with space", "--force"} {
		if _, err = r.CreateBranch(bad, false); err == nil {
			t.Fatalf("branch name %q accepted", bad)
		}
	}
	write(t, r, "b.bas", "PRINT 2\n")
	commitAll(t, r, "feature work")
	if _, err = r.SwitchBranch("main"); err != nil {
		t.Fatal(err)
	}
	if _, err = r.DeleteBranch("feature/one", false); err == nil {
		t.Fatal("unmerged branch deleted without force")
	}
	if _, err = r.DeleteBranch("feature/one", true); err != nil {
		t.Fatal(err)
	}
	if _, err = r.DeleteBranch("main", true); err == nil {
		t.Fatal("deleted the checked-out branch")
	}
}

func TestRemotesAndNetwork(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a.bas", "PRINT 1\n")
	commitAll(t, r, "initial")
	bare := filepath.Join(t.TempDir(), "remote with space.git")
	if out, err := exec.Command("git", "init", "--bare", "-b", "main", bare).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	for _, bad := range [][2]string{{"-x", bare}, {"origin", "-u"}, {"origin", "ext::sh -c id"}, {"a b", bare}, {"", bare}} {
		if _, err := r.AddRemote(bad[0], bad[1]); err == nil {
			t.Fatalf("remote %q %q accepted", bad[0], bad[1])
		}
	}
	if _, err := r.AddRemote("origin", bare); err != nil {
		t.Fatal(err)
	}
	remotes, err := r.Remotes()
	if err != nil || len(remotes) != 1 || remotes[0].Name != "origin" || remotes[0].FetchURL != bare {
		t.Fatalf("remotes = %+v, %v", remotes, err)
	}
	var progress strings.Builder
	sink := func(_, text string) { progress.WriteString(text) }
	if err = r.Network(context.Background(), "push", "", sink); err != nil {
		t.Fatalf("push: %v\n%s", err, progress.String())
	}
	status, _ := r.Status()
	if status.Upstream != "origin/main" || status.Ahead != 0 {
		t.Fatalf("upstream not set by first push: %+v", status)
	}
	// A second clone pushes a change; fetch then pull bring it in.
	clone := filepath.Join(t.TempDir(), "clone")
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, runErr := cmd.CombinedOutput(); runErr != nil {
			t.Fatalf("git %v: %v %s", args, runErr, out)
		}
	}
	run("", "clone", "-q", bare, clone)
	if err = os.WriteFile(filepath.Join(clone, "remote.bas"), []byte("PRINT 9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(clone, "add", "-A")
	run(clone, "commit", "-q", "-m", "remote change")
	run(clone, "push", "-q")
	if err = r.Network(context.Background(), "fetch", "origin", sink); err != nil {
		t.Fatal(err)
	}
	if status, _ = r.Status(); status.Behind != 1 {
		t.Fatalf("after fetch: %+v", status)
	}
	if err = r.Network(context.Background(), "pull", "", sink); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(r.Project.Root, "remote.bas")); err != nil {
		t.Fatal("pull did not bring the remote file")
	}
	if _, err = r.SetRemoteURL("origin", bare+"2"); err != nil {
		t.Fatal(err)
	}
	if _, err = r.RenameRemote("origin", "upstream"); err != nil {
		t.Fatal(err)
	}
	if remotes, _ = r.Remotes(); len(remotes) != 1 || remotes[0].Name != "upstream" || remotes[0].FetchURL != bare+"2" {
		t.Fatalf("after edit: %+v", remotes)
	}
	// A missing remote fails cleanly and never prompts.
	if err = r.Network(context.Background(), "fetch", "upstream", sink); err == nil {
		t.Fatal("fetch from a missing remote succeeded")
	}
	if _, err = r.RemoveRemote("upstream"); err != nil {
		t.Fatal(err)
	}
	if remotes, _ = r.Remotes(); len(remotes) != 0 {
		t.Fatalf("remote not removed: %+v", remotes)
	}
	if err = r.Network(context.Background(), "clone", "", sink); err == nil {
		t.Fatal("unknown network action accepted")
	}
}

func TestNetworkIsCancellable(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a.bas", "PRINT 1\n")
	commitAll(t, r, "initial")
	if _, err := r.AddRemote("origin", filepath.Join(t.TempDir(), "missing.git")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.Network(ctx, "fetch", "origin", nil); err == nil {
		t.Fatal("cancelled fetch reported success")
	}
}

func TestRejectedAuthenticationFailsWithoutPrompting(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a.bas", "PRINT 1\n")
	commitAll(t, r, "initial")
	// Port 9 (discard) is closed or refuses HTTP; either way git must return
	// an error promptly rather than wait for a username.
	if _, err := r.AddRemote("origin", "http://user@127.0.0.1:9/repo.git"); err != nil {
		t.Fatal(err)
	}
	if err := r.Network(context.Background(), "fetch", "origin", nil); err == nil {
		t.Fatal("fetch against an unreachable authenticated remote succeeded")
	}
}

// conflicted builds a repository with one unmerged file.
func conflicted(t *testing.T) Repo {
	t.Helper()
	r := newRepo(t)
	write(t, r, "game.bas", "PRINT 1\nPRINT 2\nPRINT 3\n")
	commitAll(t, r, "base")
	if _, err := r.CreateBranch("feature", true); err != nil {
		t.Fatal(err)
	}
	write(t, r, "game.bas", "PRINT 1\nPRINT \"INCOMING\"\nPRINT 3\n")
	commitAll(t, r, "incoming")
	if _, err := r.SwitchBranch("main"); err != nil {
		t.Fatal(err)
	}
	write(t, r, "game.bas", "PRINT 1\nPRINT \"CURRENT\"\nPRINT 3\n")
	commitAll(t, r, "current")
	if _, err := r.call("merge", "--no-edit", "feature"); err == nil {
		t.Fatal("merge unexpectedly succeeded")
	}
	return r
}

func TestConflictDetectionAndThreeWayContent(t *testing.T) {
	r := conflicted(t)
	status, err := r.Status()
	if err != nil || !status.Merging {
		t.Fatalf("status = %+v, %v", status, err)
	}
	if c := find(status, "game.bas"); c == nil || !c.Conflicted {
		t.Fatalf("conflict not reported: %+v", status.Changes)
	}
	conflict, err := r.Conflict("game.bas")
	if err != nil {
		t.Fatal(err)
	}
	if !conflict.HasBase || !strings.Contains(conflict.Base, "PRINT 2") || !strings.Contains(conflict.Current, "CURRENT") || !strings.Contains(conflict.Incoming, "INCOMING") {
		t.Fatalf("conflict = %+v", conflict)
	}
	if !HasConflictMarkers([]byte(conflict.Working)) {
		t.Fatalf("working copy has no markers: %q", conflict.Working)
	}
	if _, err = r.MergeContinue(); err == nil {
		t.Fatal("merge continued with unresolved conflicts")
	}
	if _, err = r.StageResolved("game.bas"); err == nil {
		t.Fatal("file with conflict markers was staged as resolved")
	}
}

func TestConflictResolutionChoices(t *testing.T) {
	for choice, want := range map[string][]string{
		"current":  {"CURRENT"},
		"incoming": {"INCOMING"},
		"both":     {"CURRENT", "INCOMING"},
	} {
		t.Run(choice, func(t *testing.T) {
			r := conflicted(t)
			if _, err := r.ResolveConflict("game.bas", choice); err != nil {
				t.Fatal(err)
			}
			content, _ := os.ReadFile(filepath.Join(r.Project.Root, "game.bas"))
			for _, fragment := range want {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("resolved file lacks %q: %q", fragment, content)
				}
			}
			if HasConflictMarkers(content) {
				t.Fatalf("markers left behind: %q", content)
			}
			if _, err := r.MergeContinue(); err != nil {
				t.Fatal(err)
			}
			status, _ := r.Status()
			if status.Merging || len(status.Changes) != 0 {
				t.Fatalf("merge not concluded: %+v", status)
			}
		})
	}
}

func TestManualResolutionAndAbort(t *testing.T) {
	r := conflicted(t)
	write(t, r, "game.bas", "PRINT 1\nPRINT \"MANUAL\"\nPRINT 3\n")
	if _, err := r.StageResolved("game.bas"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.MergeContinue(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.MergeContinue(); err == nil {
		t.Fatal("MergeContinue succeeded with no merge in progress")
	}

	r = conflicted(t)
	if _, err := r.MergeAbort(); err != nil {
		t.Fatal(err)
	}
	status, _ := r.Status()
	content, _ := os.ReadFile(filepath.Join(r.Project.Root, "game.bas"))
	if status.Merging || len(status.Changes) != 0 || !strings.Contains(string(content), "CURRENT") {
		t.Fatalf("abort did not restore the tree: %+v %q", status, content)
	}
	if _, err := r.ResolveConflict("game.bas", "current"); err == nil {
		t.Fatal("resolved a file that has no conflict")
	}
}

func TestStatusOutsideRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(t.TempDir()))
	p, err := project.Create(t.TempDir(), "plain")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(p.Root))
	status, err := Repo{Project: p}.Status()
	if err != nil || status.Available || status.Changes == nil {
		t.Fatalf("status = %+v, %v", status, err)
	}
}
