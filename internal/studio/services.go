package studio

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"exbasicstudio/internal/appstate"
	"exbasicstudio/internal/git"
	"exbasicstudio/internal/logging"
	"exbasicstudio/internal/manual"
	"exbasicstudio/internal/operations"
	"exbasicstudio/internal/terminal"
	"exbasicstudio/internal/version"
)

// ---- Terminal --------------------------------------------------------------

// TerminalOpen starts an interactive shell in the workspace root. Output and
// the exit status arrive as "terminal" events carrying the session ID.
func (s *StudioService) TerminalOpen(cols, rows int) (terminal.Info, error) {
	p, err := s.current()
	if err != nil {
		return terminal.Info{}, err
	}
	if err = s.requireTrust(p, "open a terminal in it"); err != nil {
		return terminal.Info{}, err
	}
	return s.terminals.Open(terminal.Options{Dir: p.Root, Cols: cols, Rows: rows})
}

// TerminalOpenClaude starts Claude Code interactively in the workspace root.
// Authentication remains entirely owned by the user's Claude installation.
func (s *StudioService) TerminalOpenClaude(cols, rows int) (terminal.Info, error) {
	p, err := s.current()
	if err != nil {
		return terminal.Info{}, err
	}
	if err = s.requireTrust(p, "run Claude Code in it"); err != nil {
		return terminal.Info{}, err
	}
	status := s.ClaudeCodeStatus()
	if !status.Available {
		return terminal.Info{}, errors.New(status.Message)
	}
	return s.terminals.Open(terminal.Options{Dir: p.Root, Shell: status.Path, Cols: cols, Rows: rows})
}

func (s *StudioService) TerminalWrite(id, data string) error { return s.terminals.Write(id, data) }

func (s *StudioService) TerminalResize(id string, cols, rows int) error {
	return s.terminals.Resize(id, cols, rows)
}

// TerminalClose ends a session and every process it started.
func (s *StudioService) TerminalClose(id string) error { return s.terminals.Close(id) }

func (s *StudioService) TerminalList() []terminal.Info { return s.terminals.List() }

// ---- Git -------------------------------------------------------------------

func (s *StudioService) repo() (git.Repo, error) {
	p, err := s.current()
	if err != nil {
		return git.Repo{}, err
	}
	return git.Repo{Project: p, Trusted: s.isTrusted(p)}, nil
}

// trustedRepo is required for anything that runs hooks or uses the network.
func (s *StudioService) trustedRepo(action string) (git.Repo, error) {
	repo, err := s.repo()
	if err != nil {
		return git.Repo{}, err
	}
	if !repo.Trusted {
		return git.Repo{}, fmt.Errorf("restricted mode: trust this workspace to %s", action)
	}
	return repo, nil
}

func (s *StudioService) GitStatus() (git.Status, error) {
	repo, err := s.repo()
	if err != nil {
		return git.Status{}, err
	}
	return repo.Status()
}

func (s *StudioService) GitInit() (string, error) {
	repo, err := s.repo()
	if err != nil {
		return "", err
	}
	return repo.Init()
}

func (s *StudioService) GitDiff(file string, staged bool) (string, error) {
	repo, err := s.repo()
	if err != nil {
		return "", err
	}
	return repo.Diff(file, staged)
}

func (s *StudioService) GitStage(file string) (string, error) {
	repo, err := s.repo()
	if err != nil {
		return "", err
	}
	return repo.Stage(file)
}

func (s *StudioService) GitStageAll() (string, error) {
	repo, err := s.repo()
	if err != nil {
		return "", err
	}
	return repo.StageAll()
}

func (s *StudioService) GitUnstage(file string) (string, error) {
	repo, err := s.repo()
	if err != nil {
		return "", err
	}
	return repo.Unstage(file)
}

func (s *StudioService) GitUnstageAll() (string, error) {
	repo, err := s.repo()
	if err != nil {
		return "", err
	}
	return repo.UnstageAll()
}

// GitCommit records staged changes. Commit hooks are repository-controlled
// programs, so committing requires a trusted workspace.
func (s *StudioService) GitCommit(message string) (string, error) {
	repo, err := s.trustedRepo("commit (commit hooks may run)")
	if err != nil {
		return "", err
	}
	return repo.Commit(message)
}

func (s *StudioService) GitBranches() ([]git.Branch, error) {
	repo, err := s.repo()
	if err != nil {
		return nil, err
	}
	return repo.Branches()
}

func (s *StudioService) GitCreateBranch(name string, checkout bool) (string, error) {
	repo, err := s.trustedRepo("create branches (checkout hooks may run)")
	if err != nil {
		return "", err
	}
	return repo.CreateBranch(name, checkout)
}

func (s *StudioService) GitSwitchBranch(name string) (string, error) {
	repo, err := s.trustedRepo("switch branches (checkout hooks may run)")
	if err != nil {
		return "", err
	}
	return repo.SwitchBranch(name)
}

func (s *StudioService) GitDeleteBranch(name string, force bool) (string, error) {
	repo, err := s.repo()
	if err != nil {
		return "", err
	}
	return repo.DeleteBranch(name, force)
}

func (s *StudioService) GitRemotes() ([]git.Remote, error) {
	repo, err := s.repo()
	if err != nil {
		return nil, err
	}
	return repo.Remotes()
}

func (s *StudioService) GitAddRemote(name, url string) (string, error) {
	repo, err := s.repo()
	if err != nil {
		return "", err
	}
	return repo.AddRemote(name, url)
}

func (s *StudioService) GitSetRemoteURL(name, url string) (string, error) {
	repo, err := s.repo()
	if err != nil {
		return "", err
	}
	return repo.SetRemoteURL(name, url)
}

func (s *StudioService) GitRenameRemote(oldName, newName string) (string, error) {
	repo, err := s.repo()
	if err != nil {
		return "", err
	}
	return repo.RenameRemote(oldName, newName)
}

func (s *StudioService) GitRemoveRemote(name string) (string, error) {
	repo, err := s.repo()
	if err != nil {
		return "", err
	}
	return repo.RemoveRemote(name)
}

// StartGitNetwork runs an explicit fetch, pull or push as a cancellable
// operation with streamed progress. Studio never contacts a remote on its
// own, and credentials come only from the user's Git configuration.
func (s *StudioService) StartGitNetwork(action, remote string) (operations.Info, error) {
	switch action {
	case "fetch", "pull", "push":
	default:
		return operations.Info{}, fmt.Errorf("unknown Git network action %q", action)
	}
	repo, err := s.trustedRepo(action + " (network access and hooks)")
	if err != nil {
		return operations.Info{}, err
	}
	return s.operations.Start("git", true, func(ctx context.Context, op *operations.Operation) (any, error) {
		networkErr := repo.Network(ctx, action, remote, op.Output)
		if networkErr != nil && ctx.Err() == nil {
			s.log.Warn("git %s failed: %v", action, networkErr)
		}
		status, _ := repo.Status()
		return status, networkErr
	})
}

func (s *StudioService) GitConflict(file string) (git.Conflict, error) {
	repo, err := s.repo()
	if err != nil {
		return git.Conflict{}, err
	}
	return repo.Conflict(file)
}

func (s *StudioService) GitResolveConflict(file, choice string) (string, error) {
	repo, err := s.repo()
	if err != nil {
		return "", err
	}
	return repo.ResolveConflict(file, choice)
}

func (s *StudioService) GitStageResolved(file string) (string, error) {
	repo, err := s.repo()
	if err != nil {
		return "", err
	}
	return repo.StageResolved(file)
}

func (s *StudioService) GitMergeContinue() (string, error) {
	repo, err := s.trustedRepo("conclude the merge (commit hooks may run)")
	if err != nil {
		return "", err
	}
	return repo.MergeContinue()
}

func (s *StudioService) GitMergeAbort() (string, error) {
	repo, err := s.repo()
	if err != nil {
		return "", err
	}
	return repo.MergeAbort()
}

// ---- Manual ----------------------------------------------------------------

func (s *StudioService) ManualFiles() []string                     { return manual.Files() }
func (s *StudioService) ManualChapter(file string) (string, error) { return manual.Chapter(file) }
func (s *StudioService) SearchManual(query string) []manual.Hit    { return manual.Search(query, 12) }

// ---- Application state, recovery and diagnostics ---------------------------

func (s *StudioService) activeRoot() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return ""
	}
	return s.active.Root
}

// AppState returns recent projects, layout, preferences and the remembered
// state of the open workspace.
func (s *StudioService) AppState() appstate.Snapshot { return s.state.Snapshot(s.activeRoot()) }

func (s *StudioService) SaveWorkspaceState(workspace appstate.Workspace) error {
	root := s.activeRoot()
	if root == "" {
		return nil
	}
	return s.state.SaveWorkspace(root, workspace)
}

func (s *StudioService) SaveLayout(layout appstate.Layout) error { return s.state.SaveLayout(layout) }

func (s *StudioService) SetPreference(key, value string) error {
	return s.state.SetPreference(key, value)
}

// RemoveRecentProject forgets one project, or all when path is empty.
func (s *StudioService) RemoveRecentProject(path string) (appstate.Snapshot, error) {
	if err := s.state.RemoveRecent(path); err != nil {
		return appstate.Snapshot{}, err
	}
	return s.AppState(), nil
}

// SaveRecovery stores a crash-recovery copy of an unsaved buffer.
func (s *StudioService) SaveRecovery(rel, content string) error {
	p, err := s.current()
	if err != nil {
		return err
	}
	if _, err = p.Resolve(rel); err != nil {
		return err
	}
	return s.state.SaveRecovery(p.Root, rel, content)
}

// DiscardRecovery removes the recovery copy of one file.
func (s *StudioService) DiscardRecovery(rel string) error {
	root := s.activeRoot()
	if root == "" {
		return nil
	}
	return s.state.ClearRecovery(root, rel)
}

// Recoveries lists recovery copies that differ from what is on disk. Copies
// identical to the saved file are stale and are removed silently.
func (s *StudioService) Recoveries() ([]appstate.Recovery, error) {
	p, err := s.current()
	if err != nil {
		return []appstate.Recovery{}, nil
	}
	all, err := s.state.Recoveries(p.Root)
	if err != nil {
		return nil, err
	}
	pending := []appstate.Recovery{}
	for _, item := range all {
		if _, resolveErr := p.Resolve(item.Path); resolveErr != nil {
			_ = s.state.ClearRecovery(p.Root, item.Path)
			continue
		}
		if onDisk, readErr := p.Read(item.Path); readErr == nil && onDisk == item.Content {
			_ = s.state.ClearRecovery(p.Root, item.Path)
			continue
		}
		pending = append(pending, item)
	}
	return pending, nil
}

// ClearRecoveryData deletes all crash-recovery copies (from Settings).
func (s *StudioService) ClearRecoveryData() error { return s.state.ClearAllRecovery() }

// LogFrontendError records a UI-side failure in the local diagnostic log.
func (s *StudioService) LogFrontendError(message string) {
	if len(message) > 4000 {
		message = message[:4000]
	}
	s.log.Error("frontend: %s", message)
}

// ExportSupportBundle writes a path-redacted zip of diagnostic logs to the
// chosen file. It contains no project sources and is never uploaded.
func (s *StudioService) ExportSupportBundle(path string) error {
	if strings.TrimSpace(path) == "" || !filepath.IsAbs(path) {
		return errors.New("choose where to save the support bundle")
	}
	if !strings.EqualFold(filepath.Ext(path), ".zip") {
		path += ".zip"
	}
	env := s.GetEnvironment()
	root := s.activeRoot()
	details := map[string]string{
		"platform":       env.Platform,
		"toolsAvailable": fmt.Sprint(env.ToolsAvailable),
		"compiler":       env.CompilerHint,
		"emulator":       env.EmulatorHint,
		"workspaceOpen":  fmt.Sprint(root != ""),
	}
	return s.log.ExportBundle(path, version.Display(), details, logging.NewRedactor(root, s.state.Dir()))
}
