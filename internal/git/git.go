// Package git drives the installed git command line for a local workspace.
//
// Studio never collects credentials: authentication is left to the user's
// existing credential helpers and SSH agent, and terminal prompts are
// disabled so a missing credential fails instead of hanging.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	processutil "exbasicstudio/internal/process"
	"exbasicstudio/internal/project"
)

// Repo is a workspace viewed as a Git working tree. Hooks only run when the
// workspace is trusted.
type Repo struct {
	Project *project.Project
	Trusted bool
}

type Status struct {
	Available bool     `json:"available"`
	Branch    string   `json:"branch"`
	Detached  bool     `json:"detached"`
	Unborn    bool     `json:"unborn"`
	Upstream  string   `json:"upstream"`
	Ahead     int      `json:"ahead"`
	Behind    int      `json:"behind"`
	Merging   bool     `json:"merging"`
	Changes   []Change `json:"changes"`
	Message   string   `json:"message,omitempty"`
}

// Change is one path from `git status`. Index and Worktree hold the single
// letter porcelain codes ("." for unchanged, "?" for untracked).
type Change struct {
	Path       string `json:"path"`
	OrigPath   string `json:"origPath,omitempty"`
	Index      string `json:"index"`
	Worktree   string `json:"worktree"`
	Staged     bool   `json:"staged"`
	Untracked  bool   `json:"untracked"`
	Conflicted bool   `json:"conflicted"`
}

type Branch struct {
	Name     string `json:"name"`
	Current  bool   `json:"current"`
	Upstream string `json:"upstream"`
}

type Remote struct {
	Name     string `json:"name"`
	FetchURL string `json:"fetchUrl"`
	PushURL  string `json:"pushUrl"`
}

// Conflict carries the three sides of an unmerged file. A side is absent
// when the file was added or deleted on that side, or is not text.
type Conflict struct {
	Path        string `json:"path"`
	Base        string `json:"base"`
	Current     string `json:"current"`
	Incoming    string `json:"incoming"`
	HasBase     bool   `json:"hasBase"`
	HasCurrent  bool   `json:"hasCurrent"`
	HasIncoming bool   `json:"hasIncoming"`
	Working     string `json:"working"`
	Binary      bool   `json:"binary"`
}

const localTimeout = 30 * time.Second

func (r Repo) baseArgs() []string {
	args := []string{"-C", r.Project.Root,
		// Never let repository configuration launch helper programs for
		// read-only queries, and never treat a path as pathspec magic.
		"-c", "core.fsmonitor=false",
		"--literal-pathspecs",
	}
	if !r.Trusted {
		args = append(args, "-c", "core.hooksPath="+os.DevNull)
	}
	return args
}

func (r Repo) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", append(r.baseArgs(), args...)...)
	processutil.Background(cmd)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0", // fail instead of prompting for credentials
		"GCM_INTERACTIVE=never",
		"GIT_OPTIONAL_LOCKS=0",
		"LC_ALL=C",
		"GIT_EDITOR=true",
	)
	cmd.WaitDelay = 3 * time.Second
	return cmd
}

// call runs a short local command and returns its combined output.
func (r Repo) call(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), localTimeout)
	defer cancel()
	out, err := r.command(ctx, args...).CombinedOutput()
	if err != nil {
		return string(out), describe(args, out, err)
	}
	return string(out), nil
}

// output runs a short local command and returns stdout only.
func (r Repo) output(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), localTimeout)
	defer cancel()
	cmd := r.command(ctx, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, describe(args, stderr.Bytes(), err)
	}
	return out, nil
}

func describe(args []string, output []byte, err error) error {
	message := strings.TrimSpace(string(output))
	if message == "" {
		message = err.Error()
	}
	if len(message) > 2000 {
		message = message[:2000] + "…"
	}
	return fmt.Errorf("git %s: %s", args[0], message)
}

// Stream runs a command, forwarding output chunks as they arrive. Git writes
// progress with carriage returns, so chunks are forwarded unmodified.
func (r Repo) stream(ctx context.Context, sink func(stream, text string), args ...string) error {
	cmd := r.command(ctx, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	var wg sync.WaitGroup
	var tail bytes.Buffer
	var tailMu sync.Mutex
	pump := func(name string, reader io.Reader) {
		defer wg.Done()
		buffer := make([]byte, 4096)
		for {
			n, readErr := reader.Read(buffer)
			if n > 0 {
				if sink != nil {
					sink(name, string(buffer[:n]))
				}
				tailMu.Lock()
				if tail.Len() < 8192 {
					tail.Write(buffer[:n])
				}
				tailMu.Unlock()
			}
			if readErr != nil {
				return
			}
		}
	}
	wg.Add(2)
	go pump("stdout", stdout)
	go pump("stderr", stderr)
	wg.Wait()
	if err = cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return describe(args, tail.Bytes(), err)
	}
	return nil
}

// ---- Status ----------------------------------------------------------------

func (r Repo) Status() (Status, error) {
	out, err := r.output("status", "--porcelain=v2", "-z", "--branch", "--untracked-files=all")
	if err != nil {
		return Status{Available: false, Changes: []Change{}, Message: "No Git repository (or Git is unavailable)."}, nil
	}
	status, err := ParseStatus(out)
	if err != nil {
		return Status{}, err
	}
	if _, mergeErr := r.output("rev-parse", "-q", "--verify", "MERGE_HEAD"); mergeErr == nil {
		status.Merging = true
	}
	return status, nil
}

// ParseStatus decodes `git status --porcelain=v2 -z --branch`. NUL-separated
// records keep paths with spaces, quotes, newlines and non-ASCII characters
// intact, and renames carry their original path as a second field.
func ParseStatus(out []byte) (Status, error) {
	status := Status{Available: true, Changes: []Change{}}
	fields := bytes.Split(out, []byte{0})
	for i := 0; i < len(fields); i++ {
		record := string(fields[i])
		if record == "" {
			continue
		}
		switch record[0] {
		case '#':
			parseHeader(&status, record)
		case '1':
			// 1 XY sub mH mI mW hH hI path
			parts := strings.SplitN(record, " ", 9)
			if len(parts) != 9 || len(parts[1]) != 2 {
				return Status{}, fmt.Errorf("malformed status record %q", record)
			}
			status.Changes = append(status.Changes, newChange(parts[1], parts[8], ""))
		case '2':
			// 2 XY sub mH mI mW hH hI Xscore path NUL origPath
			parts := strings.SplitN(record, " ", 10)
			if len(parts) != 10 || len(parts[1]) != 2 || i+1 >= len(fields) || len(fields[i+1]) == 0 {
				return Status{}, fmt.Errorf("malformed rename record %q", record)
			}
			i++
			status.Changes = append(status.Changes, newChange(parts[1], parts[9], string(fields[i])))
		case 'u':
			// u XY sub m1 m2 m3 mW h1 h2 h3 path
			parts := strings.SplitN(record, " ", 11)
			if len(parts) != 11 || len(parts[1]) != 2 {
				return Status{}, fmt.Errorf("malformed unmerged record %q", record)
			}
			change := newChange(parts[1], parts[10], "")
			change.Conflicted, change.Staged = true, false
			status.Changes = append(status.Changes, change)
		case '?':
			if len(record) < 3 {
				return Status{}, fmt.Errorf("malformed untracked record %q", record)
			}
			status.Changes = append(status.Changes, Change{Path: record[2:], Index: "?", Worktree: "?", Untracked: true})
		case '!':
			// ignored entries are not requested; tolerate them
		default:
			return Status{}, fmt.Errorf("unknown status record %q", record)
		}
	}
	return status, nil
}

func newChange(xy, path, orig string) Change {
	return Change{Path: path, OrigPath: orig, Index: xy[:1], Worktree: xy[1:], Staged: xy[0] != '.'}
}

func parseHeader(status *Status, record string) {
	switch {
	case strings.HasPrefix(record, "# branch.oid "):
		status.Unborn = strings.TrimPrefix(record, "# branch.oid ") == "(initial)"
	case strings.HasPrefix(record, "# branch.head "):
		status.Branch = strings.TrimPrefix(record, "# branch.head ")
		if status.Branch == "(detached)" {
			status.Detached = true
		}
	case strings.HasPrefix(record, "# branch.upstream "):
		status.Upstream = strings.TrimPrefix(record, "# branch.upstream ")
	case strings.HasPrefix(record, "# branch.ab "):
		for _, part := range strings.Fields(strings.TrimPrefix(record, "# branch.ab ")) {
			value, err := strconv.Atoi(part[1:])
			if err != nil {
				continue
			}
			if part[0] == '+' {
				status.Ahead = value
			} else if part[0] == '-' {
				status.Behind = value
			}
		}
	}
}

// ---- Basic local operations ------------------------------------------------

func (r Repo) Init() (string, error) { return r.call("init") }

func (r Repo) checkPath(file string) error {
	if file == "" {
		return errors.New("select a file first")
	}
	_, err := r.Project.Resolve(file)
	return err
}

// Diff returns a read-only textual diff. External diff drivers and textconv
// filters configured by the repository are never executed.
func (r Repo) Diff(file string, staged bool) (string, error) {
	args := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-color"}
	if staged {
		args = append(args, "--cached")
	}
	args = append(args, "--")
	if file == "" {
		args = append(args, ".")
	} else {
		if err := r.checkPath(file); err != nil {
			return "", err
		}
		args = append(args, file)
	}
	out, err := r.output(args...)
	return string(out), err
}

func (r Repo) Stage(file string) (string, error) {
	if err := r.checkPath(file); err != nil {
		return "", err
	}
	return r.call("add", "--all", "--", file)
}

func (r Repo) StageAll() (string, error) { return r.call("add", "--all", "--", ".") }

func (r Repo) hasHead() bool {
	_, err := r.output("rev-parse", "-q", "--verify", "HEAD")
	return err == nil
}

func (r Repo) Unstage(file string) (string, error) {
	if err := r.checkPath(file); err != nil {
		return "", err
	}
	if !r.hasHead() {
		return r.call("rm", "--cached", "-r", "-q", "--", file)
	}
	return r.call("restore", "--staged", "--", file)
}

func (r Repo) UnstageAll() (string, error) {
	if !r.hasHead() {
		return r.call("rm", "-r", "--cached", "-q", "--", ".")
	}
	return r.call("restore", "--staged", "--", ".")
}

// Commit records the staged changes. Commit hooks run only in a trusted
// workspace.
func (r Repo) Commit(message string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", errors.New("commit message is empty")
	}
	return r.call("commit", "-m", message)
}

// ---- Branches --------------------------------------------------------------

func (r Repo) validRef(kind, name string) error {
	if name == "" || strings.HasPrefix(name, "-") {
		return fmt.Errorf("invalid %s name", kind)
	}
	if _, err := r.output("check-ref-format", "--branch", name); err != nil {
		return fmt.Errorf("invalid %s name %q", kind, name)
	}
	return nil
}

func (r Repo) Branches() ([]Branch, error) {
	out, err := r.output("for-each-ref", "--format=%(refname:short)%00%(HEAD)%00%(upstream:short)", "refs/heads")
	if err != nil {
		return nil, err
	}
	branches := []Branch{}
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		parts := strings.Split(line, "\x00")
		if len(parts) != 3 || parts[0] == "" {
			continue
		}
		branches = append(branches, Branch{Name: parts[0], Current: parts[1] == "*", Upstream: parts[2]})
	}
	return branches, nil
}

func (r Repo) CreateBranch(name string, checkout bool) (string, error) {
	if err := r.validRef("branch", name); err != nil {
		return "", err
	}
	if checkout {
		return r.call("switch", "-c", name)
	}
	return r.call("branch", "--", name)
}

func (r Repo) SwitchBranch(name string) (string, error) {
	if err := r.validRef("branch", name); err != nil {
		return "", err
	}
	return r.call("switch", "--", name)
}

// DeleteBranch refuses to delete unmerged work unless force is set.
func (r Repo) DeleteBranch(name string, force bool) (string, error) {
	if err := r.validRef("branch", name); err != nil {
		return "", err
	}
	flag := "-d"
	if force {
		flag = "-D"
	}
	return r.call("branch", flag, "--", name)
}

// ---- Remotes ---------------------------------------------------------------

func validRemoteName(name string) error {
	if name == "" || strings.HasPrefix(name, "-") || strings.ContainsAny(name, " \t\r\n\x00/\\:~^?*[") || strings.Contains(name, "..") {
		return errors.New("invalid remote name")
	}
	return nil
}

func validRemoteURL(url string) error {
	url = strings.TrimSpace(url)
	if url == "" || strings.HasPrefix(url, "-") || strings.ContainsAny(url, "\r\n\x00") {
		return errors.New("invalid remote URL")
	}
	// The ext:: and fd:: transports run arbitrary commands.
	lower := strings.ToLower(url)
	if strings.HasPrefix(lower, "ext::") || strings.HasPrefix(lower, "fd::") {
		return errors.New("this remote transport is not supported")
	}
	return nil
}

func (r Repo) Remotes() ([]Remote, error) {
	out, err := r.output("remote")
	if err != nil {
		return nil, err
	}
	remotes := []Remote{}
	for _, name := range strings.Fields(string(out)) {
		remote := Remote{Name: name}
		if url, urlErr := r.output("remote", "get-url", "--", name); urlErr == nil {
			remote.FetchURL = strings.TrimSpace(string(url))
		}
		if url, urlErr := r.output("remote", "get-url", "--push", "--", name); urlErr == nil {
			remote.PushURL = strings.TrimSpace(string(url))
		}
		remotes = append(remotes, remote)
	}
	return remotes, nil
}

func (r Repo) AddRemote(name, url string) (string, error) {
	if err := validRemoteName(name); err != nil {
		return "", err
	}
	if err := validRemoteURL(url); err != nil {
		return "", err
	}
	return r.call("remote", "add", "--", name, strings.TrimSpace(url))
}

func (r Repo) SetRemoteURL(name, url string) (string, error) {
	if err := validRemoteName(name); err != nil {
		return "", err
	}
	if err := validRemoteURL(url); err != nil {
		return "", err
	}
	return r.call("remote", "set-url", "--", name, strings.TrimSpace(url))
}

func (r Repo) RenameRemote(oldName, newName string) (string, error) {
	if err := validRemoteName(oldName); err != nil {
		return "", err
	}
	if err := validRemoteName(newName); err != nil {
		return "", err
	}
	return r.call("remote", "rename", "--", oldName, newName)
}

func (r Repo) RemoveRemote(name string) (string, error) {
	if err := validRemoteName(name); err != nil {
		return "", err
	}
	return r.call("remote", "remove", "--", name)
}

// ---- Network operations ----------------------------------------------------

// Network runs fetch, pull or push with streamed progress. It is only ever
// started by an explicit user action and can be cancelled through ctx.
func (r Repo) Network(ctx context.Context, action, remote string, sink func(stream, text string)) error {
	if remote != "" {
		if err := validRemoteName(remote); err != nil {
			return err
		}
	}
	var args []string
	switch action {
	case "fetch":
		args = []string{"fetch", "--progress", "--prune"}
		if remote != "" {
			args = append(args, "--", remote)
		}
	case "pull":
		// A merge (never a rebase) so conflicts surface in the merge UI.
		args = []string{"pull", "--progress", "--no-rebase", "--no-edit"}
		if remote != "" {
			args = append(args, "--", remote)
		}
	case "push":
		args = []string{"push", "--progress"}
		status, err := r.Status()
		if err != nil {
			return err
		}
		if status.Upstream == "" {
			if status.Detached || status.Branch == "" {
				return errors.New("check out a branch before pushing")
			}
			if remote == "" {
				remote = "origin"
			}
			args = append(args, "--set-upstream", "--", remote, status.Branch)
		} else if remote != "" {
			args = append(args, "--", remote)
		}
	default:
		return fmt.Errorf("unknown network action %q", action)
	}
	return r.stream(ctx, sink, args...)
}

// ---- Merge conflicts -------------------------------------------------------

// stage returns one side of an unmerged path from the index.
func (r Repo) stage(number int, file string) ([]byte, bool) {
	out, err := r.output("show", fmt.Sprintf(":%d:%s", number, file))
	return out, err == nil
}

// Conflict loads the base, current (ours) and incoming (theirs) versions of
// an unmerged file together with the working-tree content.
func (r Repo) Conflict(file string) (Conflict, error) {
	if err := r.checkPath(file); err != nil {
		return Conflict{}, err
	}
	conflict := Conflict{Path: file}
	base, hasBase := r.stage(1, file)
	current, hasCurrent := r.stage(2, file)
	incoming, hasIncoming := r.stage(3, file)
	if !hasBase && !hasCurrent && !hasIncoming {
		return Conflict{}, fmt.Errorf("%s has no merge conflict", file)
	}
	for _, side := range [][]byte{base, current, incoming} {
		if len(side) > int(project.MaxSourceBytes) || !project.IsText(side) {
			conflict.Binary = true
		}
	}
	conflict.HasBase, conflict.HasCurrent, conflict.HasIncoming = hasBase, hasCurrent, hasIncoming
	if !conflict.Binary {
		conflict.Base, conflict.Current, conflict.Incoming = string(base), string(current), string(incoming)
		if working, err := r.Project.Read(file); err == nil {
			conflict.Working = working
		}
	}
	return conflict, nil
}

// ResolveConflict settles an unmerged file by taking one side ("current" or
// "incoming") or both, then stages the result.
func (r Repo) ResolveConflict(file, choice string) (string, error) {
	if err := r.checkPath(file); err != nil {
		return "", err
	}
	_, hasBase := r.stage(1, file)
	current, hasCurrent := r.stage(2, file)
	incoming, hasIncoming := r.stage(3, file)
	if !hasCurrent && !hasIncoming {
		return "", fmt.Errorf("%s has no merge conflict", file)
	}
	take := func(content []byte, present bool) (string, error) {
		if !present {
			// That side deleted the file.
			return r.call("rm", "-q", "--", file)
		}
		if err := r.writeWorking(file, content); err != nil {
			return "", err
		}
		return r.call("add", "--", file)
	}
	switch choice {
	case "current":
		return take(current, hasCurrent)
	case "incoming":
		return take(incoming, hasIncoming)
	case "both":
		if !hasCurrent || !hasIncoming {
			return "", errors.New("one side deleted the file; choose current or incoming")
		}
		merged, err := r.union(file, hasBase)
		if err != nil {
			return "", err
		}
		return take(merged, true)
	default:
		return "", fmt.Errorf("unknown resolution %q", choice)
	}
}

// union keeps the lines of both sides using git's own three-way merge.
func (r Repo) union(file string, hasBase bool) ([]byte, error) {
	dir, err := os.MkdirTemp("", "exbasic-merge-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	names := map[int]string{1: "base", 2: "current", 3: "incoming"}
	for number, name := range names {
		content, ok := r.stage(number, file)
		if !ok && number != 1 {
			return nil, fmt.Errorf("%s has no merge conflict", file)
		}
		if err = os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			return nil, err
		}
	}
	_ = hasBase // a missing base is merged against an empty file
	return r.output("merge-file", "-p", "--union",
		filepath.Join(dir, "current"), filepath.Join(dir, "base"), filepath.Join(dir, "incoming"))
}

func (r Repo) writeWorking(file string, content []byte) error {
	path, err := r.Project.Resolve(file)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".exbasic-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if _, err = temp.Write(content); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}

// StageResolved marks a manually edited conflict as resolved. It refuses a
// file that still contains conflict markers.
func (r Repo) StageResolved(file string) (string, error) {
	if err := r.checkPath(file); err != nil {
		return "", err
	}
	path, _ := r.Project.Resolve(file)
	if content, err := os.ReadFile(path); err == nil && HasConflictMarkers(content) {
		return "", fmt.Errorf("%s still contains conflict markers", file)
	}
	return r.call("add", "--", file)
}

// HasConflictMarkers reports whether content contains a complete
// <<<<<<< / ======= / >>>>>>> marker sequence at line starts.
func HasConflictMarkers(content []byte) bool {
	state := 0
	for _, line := range bytes.Split(content, []byte{'\n'}) {
		switch {
		case state == 0 && bytes.HasPrefix(line, []byte("<<<<<<< ")):
			state = 1
		case state == 1 && bytes.HasPrefix(line, []byte("=======")):
			state = 2
		case state == 2 && bytes.HasPrefix(line, []byte(">>>>>>> ")):
			return true
		}
	}
	return false
}

// MergeContinue concludes a merge once every conflict is staged.
func (r Repo) MergeContinue() (string, error) {
	status, err := r.Status()
	if err != nil {
		return "", err
	}
	if !status.Merging {
		return "", errors.New("no merge is in progress")
	}
	for _, change := range status.Changes {
		if change.Conflicted {
			return "", fmt.Errorf("resolve %s before continuing the merge", change.Path)
		}
	}
	return r.call("commit", "--no-edit")
}

// MergeAbort restores the state from before the merge.
func (r Repo) MergeAbort() (string, error) { return r.call("merge", "--abort") }
