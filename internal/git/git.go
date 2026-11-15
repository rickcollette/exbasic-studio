package git

import (
	"context"
	"errors"
	processutil "exbasicstudio/internal/process"
	"exbasicstudio/internal/project"
	"os/exec"
	"strings"
	"time"
)

type Status struct {
	Available bool     `json:"available"`
	Branch    string   `json:"branch"`
	Summary   string   `json:"summary"`
	Changes   []Change `json:"changes"`
}

type Change struct {
	Path     string `json:"path"`
	Index    string `json:"index"`
	Worktree string `json:"worktree"`
}

func call(root string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	processutil.Background(cmd)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
func GetStatus(p *project.Project) (Status, error) {
	out, err := call(p.Root, "status", "--porcelain=v1", "--branch", "--untracked-files=normal")
	if err != nil {
		return Status{Available: false, Summary: "No Git repository (or Git is unavailable)."}, nil
	}
	return ParseStatus(out), nil
}

// ParseStatus converts git's stable porcelain-v1 output into UI models. It is
// shared by local and SSH workspaces.
func ParseStatus(out string) Status {
	branch := ""
	if strings.HasPrefix(out, "## ") {
		branch = strings.SplitN(strings.TrimPrefix(strings.SplitN(out, "\n", 2)[0], "## "), "...", 2)[0]
		branch = strings.TrimPrefix(branch, "No commits yet on ")
		branch = strings.TrimPrefix(branch, "Initial commit on ")
	}
	changes := make([]Change, 0)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if len(line) < 4 || strings.HasPrefix(line, "## ") {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if arrow := strings.LastIndex(path, " -> "); arrow >= 0 {
			path = path[arrow+4:]
		}
		changes = append(changes, Change{Path: path, Index: string(line[0]), Worktree: string(line[1])})
	}
	return Status{Available: true, Branch: branch, Summary: out, Changes: changes}
}
func Init(p *project.Project) (string, error) { return call(p.Root, "init") }
func Diff(p *project.Project, file string, staged bool) (string, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--cached")
	}
	args = append(args, "--")
	if file == "" {
		args = append(args, ".")
	} else {
		if _, err := p.Resolve(file); err != nil {
			return "", err
		}
		args = append(args, file)
	}
	return call(p.Root, args...)
}
func Stage(p *project.Project, file string) (string, error) {
	if file == "" {
		return "", errors.New("select a file to stage")
	}
	if _, err := p.Resolve(file); err != nil {
		return "", err
	}
	return call(p.Root, "add", "--", file)
}
func StageAll(p *project.Project) (string, error) { return call(p.Root, "add", "--all", "--", ".") }
func Unstage(p *project.Project, file string) (string, error) {
	if file == "" {
		return "", errors.New("select a file to unstage")
	}
	if _, err := p.Resolve(file); err != nil {
		return "", err
	}
	if _, err := call(p.Root, "rev-parse", "--verify", "HEAD"); err != nil {
		return call(p.Root, "rm", "--cached", "--", file)
	}
	return call(p.Root, "restore", "--staged", "--", file)
}
func UnstageAll(p *project.Project) (string, error) {
	if _, err := call(p.Root, "rev-parse", "--verify", "HEAD"); err != nil {
		return call(p.Root, "rm", "-r", "--cached", "--", ".")
	}
	return call(p.Root, "restore", "--staged", "--", ".")
}
func Commit(p *project.Project, message string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", errors.New("commit message is empty")
	}
	return call(p.Root, "commit", "-m", message)
}
