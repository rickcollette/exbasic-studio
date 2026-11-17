package studio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"exbasicstudio/internal/appstate"
	"exbasicstudio/internal/project"
	"exbasicstudio/internal/watch"
)

// SearchResult is one matching line of a project search.
type SearchResult struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// Search limits keep a query over a large or hostile workspace bounded.
const (
	maxSearchResults        = 200
	maxSearchFileBytes      = 1 << 20
	maxSearchTotalBytes     = 64 << 20
	maxSearchQueryRunes     = 200
	maxSearchResultLineSize = 240
)

// TrustInfo tells the UI whether the workspace may run external programs.
type TrustInfo struct {
	Trusted bool `json:"trusted"`
	// Tools lists the programs the workspace is configured to launch, so the
	// trust prompt can show exactly what would run.
	Tools []string `json:"tools"`
}

func toolFingerprint(cfg project.Config) string {
	return appstate.ToolFingerprint(cfg.Compiler, cfg.Emulator, cfg.ATR.Tool)
}

func (s *StudioService) isTrusted(p *project.Project) bool {
	return s.state.IsTrusted(p.Root, toolFingerprint(p.Config))
}

// requireTrust guards everything that executes workspace-controlled
// programs or runs with the workspace's own configuration.
func (s *StudioService) requireTrust(p *project.Project, action string) error {
	if s.isTrusted(p) {
		return nil
	}
	return fmt.Errorf("restricted mode: trust this workspace to %s", action)
}

// WorkspaceTrust reports the trust state of the open workspace.
func (s *StudioService) WorkspaceTrust() (TrustInfo, error) {
	p, err := s.current()
	if err != nil {
		return TrustInfo{Tools: []string{}}, err
	}
	return TrustInfo{Trusted: s.isTrusted(p), Tools: []string{p.Config.Compiler, p.Config.Emulator, p.Config.ATR.Tool}}, nil
}

// TrustWorkspace records the user's explicit decision to trust the open
// workspace with its currently configured tools.
func (s *StudioService) TrustWorkspace() (TrustInfo, error) {
	p, err := s.current()
	if err != nil {
		return TrustInfo{Tools: []string{}}, err
	}
	if err = s.state.Trust(p.Root, toolFingerprint(p.Config)); err != nil {
		return TrustInfo{Tools: []string{}}, err
	}
	s.log.Info("workspace trusted: %s", p.Root)
	return s.WorkspaceTrust()
}

// RevokeWorkspaceTrust returns the workspace to restricted mode and closes
// anything that was started under the previous trust.
func (s *StudioService) RevokeWorkspaceTrust() (TrustInfo, error) {
	p, err := s.current()
	if err != nil {
		return TrustInfo{Tools: []string{}}, err
	}
	if err = s.state.Revoke(p.Root); err != nil {
		return TrustInfo{Tools: []string{}}, err
	}
	s.terminals.Shutdown()
	return s.WorkspaceTrust()
}

// activate makes p the open workspace: the previous workspace's processes
// are stopped, a watcher is started and the project joins the recent list.
//
// The folder is listed first. If that fails nothing changes: the previous
// workspace stays open, so the window and the backend can never disagree
// about which folder file operations apply to.
func (s *StudioService) activate(p *project.Project) (project.View, error) {
	view, err := p.View()
	if err != nil {
		return project.View{}, err
	}
	s.operations.CancelAll()
	s.terminals.Shutdown()
	_ = s.EmulatorStop()
	watcher, watchErr := watch.Start(p.Root, func(batch watch.Batch) { s.deliver(watch.EventName, batch) })
	if watchErr != nil {
		s.log.Warn("file watching unavailable for %s: %v", p.Root, watchErr)
	}
	s.mu.Lock()
	previous := s.watcher
	s.watcher = watcher
	s.active = p
	s.mu.Unlock()
	if previous != nil {
		previous.Stop()
	}
	if err := s.state.AddRecent(p.Root, p.Config.Name); err != nil {
		s.log.Warn("remember recent project: %v", err)
	}
	s.log.Info("opened workspace %s", p.Root)
	return view, nil
}

// CreateProject scaffolds a new project. A project the user just created
// from Studio's own template is trusted.
func (s *StudioService) CreateProject(parent, name string) (project.View, error) {
	s.opening.Lock()
	defer s.opening.Unlock()
	p, err := project.Create(parent, name)
	if err != nil {
		return project.View{}, err
	}
	if err = s.state.Trust(p.Root, toolFingerprint(p.Config)); err != nil {
		s.log.Warn("trust new project: %v", err)
	}
	return s.activate(p)
}

// OpenProject opens a folder. Opening and editing are always safe: nothing
// in the folder is executed and nothing is written to it.
func (s *StudioService) OpenProject(path string) (project.View, error) {
	s.opening.Lock()
	defer s.opening.Unlock()
	p, err := project.Open(path)
	if err != nil {
		return project.View{}, err
	}
	return s.activate(p)
}

// CloseProject returns to the empty workbench.
func (s *StudioService) CloseProject() error {
	s.operations.CancelAll()
	s.terminals.Shutdown()
	err := s.EmulatorStop()
	s.mu.Lock()
	watcher := s.watcher
	s.watcher, s.active = nil, nil
	s.mu.Unlock()
	if watcher != nil {
		watcher.Stop()
	}
	return err
}

func (s *StudioService) GetProject() (project.View, error) {
	p, err := s.current()
	if err != nil {
		return project.View{}, err
	}
	return p.View()
}

func (s *StudioService) ReadFile(rel string) (string, error) {
	p, err := s.current()
	if err != nil {
		return "", err
	}
	return p.Read(rel)
}

// SaveFile writes a file atomically and discards its crash-recovery copy.
func (s *StudioService) SaveFile(rel, content string) error {
	p, err := s.current()
	if err != nil {
		return err
	}
	if err = s.withEOSHostMutation([]string{rel}, func() error { return p.Save(rel, content) }); err != nil {
		return err
	}
	if clearErr := s.state.ClearRecovery(p.Root, rel); clearErr != nil {
		s.log.Warn("clear recovery copy of %s: %v", rel, clearErr)
	}
	return nil
}

func (s *StudioService) CreateFolder(rel string) (project.View, error) {
	p, err := s.current()
	if err != nil {
		return project.View{}, err
	}
	if err = s.withEOSHostMutation([]string{rel}, func() error { return p.CreateDirectory(rel) }); err != nil {
		return project.View{}, err
	}
	return p.View()
}

// ImportProjectPaths copies files or complete folders dropped from the host
// desktop into a project folder. Existing entries are never overwritten.
func (s *StudioService) ImportProjectPaths(destination string, paths []string) (project.View, error) {
	p, err := s.current()
	if err != nil {
		return project.View{}, err
	}
	if strings.TrimSpace(destination) == "" {
		destination = "."
	}
	affected := make([]string, 0, len(paths))
	for _, source := range paths {
		name := filepath.Base(filepath.Clean(source))
		if name != "." && name != string(filepath.Separator) {
			affected = append(affected, filepath.ToSlash(filepath.Join(destination, name)))
		}
	}
	if err = s.withEOSHostMutation(affected, func() error { return p.ImportPaths(destination, paths) }); err != nil {
		return project.View{}, err
	}
	return p.View()
}

func (s *StudioService) RenamePath(oldRel, newRel string) (project.View, error) {
	p, err := s.current()
	if err != nil {
		return project.View{}, err
	}
	err = s.withEOSHostMutation([]string{oldRel, newRel}, func() error {
		if renameErr := p.Rename(oldRel, newRel); renameErr != nil {
			return renameErr
		}
		oldSlash := strings.TrimSuffix(filepath.ToSlash(oldRel), "/")
		if p.HasConfig && (p.Config.Entry == oldSlash || strings.HasPrefix(p.Config.Entry, oldSlash+"/")) {
			p.Config.Entry = filepath.ToSlash(newRel) + strings.TrimPrefix(p.Config.Entry, oldSlash)
			return p.SaveConfig()
		}
		if p.Config.Entry == oldSlash || strings.HasPrefix(p.Config.Entry, oldSlash+"/") {
			// A plain folder has no metadata on disk; keep the in-memory entry current.
			p.Config.Entry = filepath.ToSlash(newRel) + strings.TrimPrefix(p.Config.Entry, oldSlash)
		}
		return nil
	})
	if err != nil {
		return project.View{}, err
	}
	return p.View()
}

// DeletePath removes a file or folder. The UI confirms with the user first.
func (s *StudioService) DeletePath(rel string) (project.View, error) {
	p, err := s.current()
	if err != nil {
		return project.View{}, err
	}
	if err = s.withEOSHostMutation([]string{rel}, func() error { return p.Delete(rel) }); err != nil {
		return project.View{}, err
	}
	_ = s.state.ClearRecovery(p.Root, rel)
	return p.View()
}

// SearchProject finds a case-insensitive substring in text files.
func (s *StudioService) SearchProject(query string) ([]SearchResult, error) {
	p, err := s.current()
	if err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 2 {
		return []SearchResult{}, nil
	}
	if len([]rune(query)) > maxSearchQueryRunes {
		return nil, errors.New("search text is too long")
	}
	files, err := p.Files()
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(query)
	results := make([]SearchResult, 0, 32)
	var scanned int64
	for _, file := range files {
		if file.Size > maxSearchFileBytes {
			continue
		}
		if scanned += file.Size; scanned > maxSearchTotalBytes {
			break
		}
		// Read rejects binary and non-UTF-8 files.
		content, readErr := p.Read(file.Path)
		if readErr != nil {
			continue
		}
		for index, line := range strings.Split(content, "\n") {
			if !strings.Contains(strings.ToLower(line), needle) {
				continue
			}
			text := strings.TrimSpace(line)
			if len(text) > maxSearchResultLineSize {
				cut := maxSearchResultLineSize
				for cut > 0 && !isRuneStart(text[cut]) {
					cut--
				}
				text = text[:cut] + "…"
			}
			results = append(results, SearchResult{Path: file.Path, Line: index + 1, Text: text})
			if len(results) >= maxSearchResults {
				return results, nil
			}
		}
	}
	return results, nil
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }

// UpdateConfig saves project settings. This is the only path that writes
// exbasic.project.json, and therefore the only one that persists a schema
// migration. Changing a tool path changes the trust fingerprint, which
// returns the workspace to restricted mode until the user trusts it again.
func (s *StudioService) UpdateConfig(cfg project.Config) (project.View, error) {
	p, err := s.current()
	if err != nil {
		return project.View{}, err
	}
	if err = project.ValidateConfig(cfg); err != nil {
		return project.View{}, err
	}
	wasTrusted := s.isTrusted(p)
	previous := p.Config
	p.Config = cfg
	if err = p.SaveConfig(); err != nil {
		p.Config = previous
		return project.View{}, err
	}
	if wasTrusted && !s.isTrusted(p) {
		s.log.Info("tool paths changed; workspace trust revoked: %s", p.Root)
		s.terminals.Shutdown()
	}
	return p.View()
}

// ensureHostDir returns the Atari H1: folder. The project's H directory is
// created lazily, on the first trusted run; an untrusted or absent project
// uses a private folder in the application's own data directory.
//
// A folder chosen in the emulator profile takes the place of either. The
// profile belongs to the user, not to the project, so it is honoured without
// workspace trust; a relative choice needs a trusted project to start from.
func (s *StudioService) ensureHostDir(p *project.Project, configured string) (string, error) {
	trusted := p != nil && s.isTrusted(p)
	if configured != "" {
		if filepath.IsAbs(configured) {
			hostDir := filepath.Clean(configured)
			if info, err := os.Stat(hostDir); err != nil || !info.IsDir() {
				return "", fmt.Errorf("the H: folder of this emulator profile is not a folder: %s", hostDir)
			}
			return hostDir, nil
		}
		if !trusted {
			return "", fmt.Errorf("the H: folder %q of this emulator profile is relative to a project; open and trust a project, or choose a full path", configured)
		}
		hostDir := filepath.Join(p.Root, configured)
		if err := os.MkdirAll(hostDir, 0o755); err != nil {
			return "", fmt.Errorf("create Atari H1 host folder: %w", err)
		}
		return hostDir, nil
	}
	var hostDir string
	if trusted {
		hostDir = filepath.Join(p.Root, "H")
	} else {
		hostDir = filepath.Join(s.state.Dir(), "H")
	}
	if err := os.MkdirAll(hostDir, 0o755); err != nil {
		return "", fmt.Errorf("create Atari H1 host folder: %w", err)
	}
	return hostDir, nil
}
