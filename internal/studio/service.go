package studio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"exbasicstudio/internal/atrexplorer"
	"exbasicstudio/internal/claudecode"
	"exbasicstudio/internal/compiler"
	"exbasicstudio/internal/diskimage"
	"exbasicstudio/internal/emulator"
	"exbasicstudio/internal/extensions"
	"exbasicstudio/internal/git"
	"exbasicstudio/internal/manual"
	processutil "exbasicstudio/internal/process"
	"exbasicstudio/internal/project"
	"exbasicstudio/internal/remote"
)

type Environment struct {
	WorkingDir      string `json:"workingDir"`
	ClaudeAvailable bool   `json:"claudeAvailable"`
	ClaudeVersion   string `json:"claudeVersion"`
	CompilerHint    string `json:"compilerHint"`
	EmulatorHint    string `json:"emulatorHint"`
	SSHAvailable    bool   `json:"sshAvailable"`
}

type SearchResult struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

type TerminalResult struct {
	Output     string `json:"output"`
	ExitCode   int    `json:"exitCode"`
	WorkingDir string `json:"workingDir"`
}

type StudioService struct {
	mu                sync.RWMutex
	eosSyncMu         sync.Mutex
	active            *project.Project
	remote            *remote.Project
	claudeSessions    map[string]string
	terminalDirs      map[string]string
	emulator          *emulator.Session
	emulatorLaunch    emulator.Launch
	emulatorDiskMod   time.Time
	emulatorDiskSize  int64
	emulatorDiskCheck time.Time
	emulatorProfiles  *emulator.ProfileStore
}

func New() *StudioService {
	return &StudioService{claudeSessions: make(map[string]string), terminalDirs: make(map[string]string), emulatorProfiles: emulator.DefaultProfileStore()}
}

// ServiceShutdown releases child processes and remote connections when Wails exits.
func (s *StudioService) ServiceShutdown() error {
	emulatorErr := s.EmulatorStop()
	s.mu.Lock()
	remoteProject := s.remote
	s.remote = nil
	s.mu.Unlock()
	if remoteProject != nil {
		remoteProject.Close()
	}
	return emulatorErr
}
func (s *StudioService) current() (*project.Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return nil, errors.New("open or create a project first")
	}
	return s.active, nil
}
func (s *StudioService) currentRemote() (*remote.Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.remote == nil {
		return nil, errors.New("open a remote project first")
	}
	return s.remote, nil
}
func (s *StudioService) isRemote() bool { s.mu.RLock(); defer s.mu.RUnlock(); return s.remote != nil }
func (s *StudioService) GetEnvironment() Environment {
	wd := usefulWorkingDirectory()
	claude := claudecode.Status()
	_, sshErr := exec.LookPath("ssh")
	return Environment{
		WorkingDir:      wd,
		ClaudeAvailable: claude.Available,
		ClaudeVersion:   claude.Version,
		CompilerHint:    compiler.ToolPath("exbasic"),
		EmulatorHint:    compiler.ToolPath("atari800-studio"),
		SSHAvailable:    sshErr == nil,
	}
}

// usefulWorkingDirectory prevents a packaged application from suggesting its
// executable directory (notably the development bin folder) as the workspace.
func usefulWorkingDirectory() string {
	wd, _ := os.Getwd()
	if strings.Contains(strings.ToLower(wd), `\windowsapps\`) {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.EqualFold(filepath.Base(wd), "bin") {
		parent := filepath.Dir(wd)
		if _, err := os.Stat(filepath.Join(parent, "go.mod")); err == nil {
			return parent
		}
	}
	return wd
}
func (s *StudioService) SSHHosts(configFile string) ([]remote.Host, error) {
	return remote.Hosts(configFile)
}
func (s *StudioService) ResolveSSHConfig(alias, configFile string) (remote.Config, error) {
	return remote.Resolve(alias, configFile)
}
func (s *StudioService) OpenRemoteProject(alias, remotePath, configFile string) (project.View, error) {
	p, err := remote.Open(alias, remotePath, configFile)
	if err != nil {
		return project.View{}, err
	}
	s.mu.Lock()
	if s.remote != nil {
		s.remote.Close()
	}
	s.remote = p
	s.active = nil
	s.mu.Unlock()
	return p.View()
}
func (s *StudioService) CreateProject(parent, name string) (project.View, error) {
	p, err := project.Create(parent, name)
	if err != nil {
		return project.View{}, err
	}
	if _, err = ensureProjectHostDir(p); err != nil {
		return project.View{}, err
	}
	s.mu.Lock()
	if s.remote != nil {
		s.remote.Close()
	}
	s.remote = nil
	s.active = p
	s.mu.Unlock()
	return p.View()
}
func (s *StudioService) OpenProject(path string) (project.View, error) {
	p, err := project.Open(path)
	if err != nil {
		return project.View{}, err
	}
	if _, err = ensureProjectHostDir(p); err != nil {
		return project.View{}, err
	}
	s.mu.Lock()
	if s.remote != nil {
		s.remote.Close()
	}
	s.remote = nil
	s.active = p
	s.mu.Unlock()
	return p.View()
}
func (s *StudioService) GetProject() (project.View, error) {
	if s.isRemote() {
		p, e := s.currentRemote()
		if e != nil {
			return project.View{}, e
		}
		return p.View()
	}
	p, err := s.current()
	if err != nil {
		return project.View{}, err
	}
	return p.View()
}
func (s *StudioService) ReadFile(rel string) (string, error) {
	if s.isRemote() {
		p, e := s.currentRemote()
		if e != nil {
			return "", e
		}
		return p.Read(rel)
	}
	p, err := s.current()
	if err != nil {
		return "", err
	}
	return p.Read(rel)
}
func (s *StudioService) SaveFile(rel, content string) error {
	if s.isRemote() {
		p, e := s.currentRemote()
		if e != nil {
			return e
		}
		return p.Save(rel, content)
	}
	p, err := s.current()
	if err != nil {
		return err
	}
	return s.withEOSHostMutation([]string{rel}, func() error { return p.Save(rel, content) })
}
func (s *StudioService) CreateFolder(rel string) (project.View, error) {
	if s.isRemote() {
		p, e := s.currentRemote()
		if e != nil {
			return project.View{}, e
		}
		if e = p.CreateDirectory(rel); e != nil {
			return project.View{}, e
		}
		return p.View()
	}
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
// desktop into a local project folder.
func (s *StudioService) ImportProjectPaths(destination string, paths []string) (project.View, error) {
	if s.isRemote() {
		return project.View{}, errors.New("desktop drops into remote projects are not supported")
	}
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
	if s.isRemote() {
		p, e := s.currentRemote()
		if e != nil {
			return project.View{}, e
		}
		if e = p.Rename(oldRel, newRel); e != nil {
			return project.View{}, e
		}
		oldPrefix := strings.TrimSuffix(filepath.ToSlash(oldRel), "/") + "/"
		if p.Config.Entry == oldRel || strings.HasPrefix(p.Config.Entry, oldPrefix) {
			p.Config.Entry = filepath.ToSlash(newRel) + strings.TrimPrefix(p.Config.Entry, filepath.ToSlash(oldRel))
			if e = p.SaveConfig(); e != nil {
				return project.View{}, e
			}
		}
		return p.View()
	}
	p, err := s.current()
	if err != nil {
		return project.View{}, err
	}
	err = s.withEOSHostMutation([]string{oldRel, newRel}, func() error {
		if renameErr := p.Rename(oldRel, newRel); renameErr != nil {
			return renameErr
		}
		oldPrefix := strings.TrimSuffix(filepath.ToSlash(oldRel), "/") + "/"
		if p.Config.Entry == oldRel || strings.HasPrefix(p.Config.Entry, oldPrefix) {
			p.Config.Entry = filepath.ToSlash(newRel) + strings.TrimPrefix(p.Config.Entry, filepath.ToSlash(oldRel))
			return p.SaveConfig()
		}
		return nil
	})
	if err != nil {
		return project.View{}, err
	}
	return p.View()
}
func (s *StudioService) DeletePath(rel string) (project.View, error) {
	if s.isRemote() {
		p, e := s.currentRemote()
		if e != nil {
			return project.View{}, e
		}
		if e = p.Delete(rel); e != nil {
			return project.View{}, e
		}
		return p.View()
	}
	p, err := s.current()
	if err != nil {
		return project.View{}, err
	}
	if err = s.withEOSHostMutation([]string{rel}, func() error { return p.Delete(rel) }); err != nil {
		return project.View{}, err
	}
	return p.View()
}
func (s *StudioService) SearchProject(query string) ([]SearchResult, error) {
	type searchable interface {
		Files() ([]project.File, error)
		Read(string) (string, error)
	}
	var p searchable
	if s.isRemote() {
		rp, e := s.currentRemote()
		if e != nil {
			return nil, e
		}
		p = rp
	} else {
		lp, e := s.current()
		if e != nil {
			return nil, e
		}
		p = lp
	}
	query = strings.TrimSpace(query)
	if len(query) < 2 {
		return []SearchResult{}, nil
	}
	files, err := p.Files()
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(query)
	results := make([]SearchResult, 0, 32)
	for _, file := range files {
		if file.Size > 1<<20 {
			continue
		}
		content, err := p.Read(file.Path)
		if err != nil || strings.IndexByte(content, 0) >= 0 {
			continue
		}
		for index, line := range strings.Split(content, "\n") {
			if !strings.Contains(strings.ToLower(line), needle) {
				continue
			}
			text := strings.TrimSpace(line)
			if len(text) > 240 {
				text = text[:240] + "…"
			}
			results = append(results, SearchResult{Path: file.Path, Line: index + 1, Text: text})
			if len(results) >= 200 {
				return results, nil
			}
		}
	}
	return results, nil
}

func (s *StudioService) RunTerminal(command string) (TerminalResult, error) {
	if s.isRemote() {
		p, e := s.currentRemote()
		if e != nil {
			return TerminalResult{}, e
		}
		command = strings.TrimSpace(command)
		if command == "" {
			return TerminalResult{}, errors.New("enter a terminal command")
		}
		out, code, cwd, e := p.Terminal(command)
		return TerminalResult{Output: out, ExitCode: code, WorkingDir: p.Alias + ":" + cwd}, e
	}
	p, err := s.current()
	if err != nil {
		return TerminalResult{}, err
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return TerminalResult{}, errors.New("enter a terminal command")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	s.mu.RLock()
	workingDir := s.terminalDirs[p.Root]
	s.mu.RUnlock()
	if workingDir == "" {
		workingDir = p.Root
	}
	const marker = "__EXBASIC_CWD__="
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		wrapped := `$exbasicOutput = (& { ` + command + ` } 2>&1 | Out-String); $exbasicExit=$LASTEXITCODE; if ($null -eq $exbasicExit) {$exbasicExit=0}; [Console]::Out.Write($exbasicOutput); [Console]::Out.WriteLine("` + marker + `$((Get-Location).Path)"); exit $exbasicExit`
		cmd = exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-Command", wrapped)
	} else {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		wrapped := command + `; exbasic_exit=$?; printf '\n` + marker + `%s\n' "$PWD"; exit $exbasic_exit`
		cmd = exec.CommandContext(ctx, shell, "-lc", wrapped)
	}
	processutil.Background(cmd)
	cmd.Dir = workingDir
	output, runErr := cmd.CombinedOutput()
	text := string(output)
	if index := strings.LastIndex(text, marker); index >= 0 {
		line := strings.TrimSpace(text[index+len(marker):])
		if end := strings.IndexAny(line, "\r\n"); end >= 0 {
			line = line[:end]
		}
		if info, statErr := os.Stat(line); statErr == nil && info.IsDir() {
			workingDir = line
			s.mu.Lock()
			s.terminalDirs[p.Root] = line
			s.mu.Unlock()
		}
		text = strings.TrimRight(text[:index], "\r\n")
	}
	result := TerminalResult{Output: text, WorkingDir: workingDir}
	if runErr == nil {
		return result, nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return TerminalResult{}, errors.New("terminal command timed out after 2 minutes")
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}
	return TerminalResult{}, runErr
}
func (s *StudioService) UpdateConfig(cfg project.Config) (project.View, error) {
	if s.isRemote() {
		p, e := s.currentRemote()
		if e != nil {
			return project.View{}, e
		}
		if e = project.ValidateConfig(cfg); e != nil {
			return project.View{}, e
		}
		p.Config = cfg
		if e = p.SaveConfig(); e != nil {
			return project.View{}, e
		}
		return p.View()
	}
	p, err := s.current()
	if err != nil {
		return project.View{}, err
	}
	if err := project.ValidateConfig(cfg); err != nil {
		return project.View{}, err
	}
	p.Config = cfg
	if err := p.SaveConfig(); err != nil {
		return project.View{}, err
	}
	return p.View()
}

func (s *StudioService) atrTool() (string, error) {
	configured := "mkatr-go"
	s.mu.RLock()
	if s.active != nil && strings.TrimSpace(s.active.Config.ATR.Tool) != "" {
		configured = s.active.Config.ATR.Tool
	}
	s.mu.RUnlock()
	return diskimage.FindTool(configured)
}

// ATROpen reads an arbitrary local SDFS or EOSFS ATR image.
func (s *StudioService) ATROpen(path string) (atrexplorer.View, error) {
	tool, err := s.atrTool()
	if err != nil {
		return atrexplorer.View{}, err
	}
	return atrexplorer.Catalog(tool, path)
}

// ATRConfigured opens the active project's configured ATR output.
func (s *StudioService) ATRConfigured() (atrexplorer.View, error) {
	p, err := s.current()
	if err != nil {
		return atrexplorer.View{}, err
	}
	path, err := p.Resolve(p.Config.ATR.Output)
	if err != nil {
		return atrexplorer.View{}, err
	}
	return s.ATROpen(path)
}

// ATRImportFiles imports native host paths, including directory trees.
func (s *StudioService) ATRImportFiles(image, destination string, paths []string) (atrexplorer.View, error) {
	tool, err := s.atrTool()
	if err != nil {
		return atrexplorer.View{}, err
	}
	return atrexplorer.Import(tool, image, destination, paths)
}

// ATRImportProject imports selected workspace paths into an image directory.
func (s *StudioService) ATRImportProject(image, destination string, paths []string) (atrexplorer.View, error) {
	p, err := s.current()
	if err != nil {
		return atrexplorer.View{}, err
	}
	hostPaths := make([]string, 0, len(paths))
	for _, rel := range paths {
		path, resolveErr := p.Resolve(rel)
		if resolveErr != nil {
			return atrexplorer.View{}, resolveErr
		}
		hostPaths = append(hostPaths, path)
	}
	return s.ATRImportFiles(image, destination, hostPaths)
}

func (s *StudioService) ATRMkdir(image, path string) (atrexplorer.View, error) {
	tool, err := s.atrTool()
	if err != nil {
		return atrexplorer.View{}, err
	}
	return atrexplorer.Mkdir(tool, image, path)
}

func (s *StudioService) ATRRemove(image, path string) (atrexplorer.View, error) {
	tool, err := s.atrTool()
	if err != nil {
		return atrexplorer.View{}, err
	}
	return atrexplorer.Remove(tool, image, path)
}

func (s *StudioService) ATRRename(image, oldPath, newPath string) (atrexplorer.View, error) {
	tool, err := s.atrTool()
	if err != nil {
		return atrexplorer.View{}, err
	}
	return atrexplorer.Rename(tool, image, oldPath, newPath)
}

func (s *StudioService) ATRExtract(image, path, output string) error {
	tool, err := s.atrTool()
	if err != nil {
		return err
	}
	return atrexplorer.Extract(tool, image, path, output)
}

func (s *StudioService) ATRExtractToDirectory(image, path, directory string) error {
	if strings.TrimSpace(directory) == "" {
		return errors.New("choose an extraction directory")
	}
	name := filepath.Base(filepath.FromSlash(path))
	return s.ATRExtract(image, path, filepath.Join(directory, name))
}

// ATRExtractToProject copies an ATR entry into a local workspace directory.
func (s *StudioService) ATRExtractToProject(image, path, destination string) (project.View, error) {
	p, err := s.current()
	if err != nil {
		return project.View{}, err
	}
	name := filepath.Base(filepath.FromSlash(path))
	rel := filepath.ToSlash(filepath.Join(destination, name))
	target, err := p.Resolve(rel)
	if err != nil {
		return project.View{}, err
	}
	if err = s.withEOSHostMutation([]string{rel}, func() error { return s.ATRExtract(image, path, target) }); err != nil {
		return project.View{}, err
	}
	return p.View()
}
func (s *StudioService) Build() (compiler.Result, error) {
	if s.isRemote() {
		p, e := s.currentRemote()
		if e != nil {
			return compiler.Result{}, e
		}
		return p.Build()
	}
	p, err := s.current()
	if err != nil {
		return compiler.Result{}, err
	}
	return compiler.Build(p)
}
func (s *StudioService) Lint(rel, content string) ([]compiler.Diagnostic, error) {
	if s.isRemote() {
		return nil, errors.New("live exBASIC linting is currently available for local projects")
	}
	p, err := s.current()
	if err != nil {
		return nil, err
	}
	return compiler.Lint(p, rel, content)
}
func (s *StudioService) Run() (compiler.Result, error) { return s.RunWithProfile("") }

func (s *StudioService) RunWithProfile(profileID string) (compiler.Result, error) {
	if s.isRemote() {
		p, e := s.currentRemote()
		if e != nil {
			return compiler.Result{}, e
		}
		res, e := p.Build()
		if e == nil && res.Success {
			res.Output += "\nRemote build completed. Launch the artifact from the remote host terminal.\n"
		}
		return res, e
	}
	p, err := s.current()
	if err != nil {
		return compiler.Result{}, err
	}
	res, err := compiler.Build(p)
	if err != nil || !res.Success {
		return res, err
	}
	artifact := res.Artifact
	eosCar := ""
	if res.DiskImage != "" {
		artifact = res.DiskImage
		if p.Config.ATR.Filesystem == "eos" {
			eosCar = compiler.AssetPath("EOS.car")
		}
	}
	profile, profileErr := s.emulatorProfiles.Find(profileID)
	if profileErr != nil {
		return res, profileErr
	}
	hostDir, hostErr := ensureProjectHostDir(p)
	if hostErr != nil {
		return res, hostErr
	}
	launch := emulator.Launch{Helper: emulatorHelper(p.Config.Emulator), Artifact: artifact, EOSCar: eosCar, HostDir: hostDir, HostTool: p.Config.ATR.Tool, Profile: profile}
	if profile.EOS && launch.EOSCar == "" {
		launch.EOSCar = compiler.AssetPath("EOS.car")
	}
	if startErr := s.startEmulator(launch); startErr != nil {
		return res, startErr
	}
	res.Output += "\nAtari800 started in an editor tab (headless libatari800 core).\n"
	return res, nil
}

func emulatorHelper(configured string) string {
	helper := configured
	base := filepath.Base(helper)
	if helper == "" || strings.EqualFold(base, "atari800") || strings.EqualFold(base, "atari800.exe") || strings.EqualFold(base, "atari800-headless") || strings.EqualFold(base, "atari800-headless.exe") {
		helper = "atari800-studio"
	}
	return compiler.ToolPath(helper)
}

func ensureProjectHostDir(p *project.Project) (string, error) {
	var hostDir string
	if p != nil {
		hostDir = filepath.Join(p.Root, "H")
	} else {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("locate standalone Atari H1 folder: %w", err)
		}
		hostDir = filepath.Join(configDir, "exbasic-studio", "H")
	}
	if err := os.MkdirAll(hostDir, 0o755); err != nil {
		return "", fmt.Errorf("create Atari H1 host folder: %w", err)
	}
	return hostDir, nil
}

func (s *StudioService) startEmulator(launch emulator.Launch) error {
	s.eosSyncMu.Lock()
	defer s.eosSyncMu.Unlock()
	s.mu.Lock()
	previous := s.emulator
	previousLaunch := s.emulatorLaunch
	s.emulator = nil
	s.mu.Unlock()
	if previous != nil {
		if stopErr := previous.Stop(); stopErr != nil {
			return stopErr
		}
		if syncErr := synchronizeEOSHostDisk(previousLaunch); syncErr != nil {
			return syncErr
		}
	}
	prepared, err := prepareEOSHostDisk(launch)
	if err != nil {
		return err
	}
	launch = prepared
	session, err := emulator.Start(launch)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.emulator = session
	s.emulatorLaunch = launch
	s.emulatorDiskMod = time.Time{}
	s.emulatorDiskSize = 0
	s.emulatorDiskCheck = time.Now()
	if info, statErr := os.Stat(launch.HostDisk); statErr == nil {
		s.emulatorDiskMod = info.ModTime()
		s.emulatorDiskSize = info.Size()
	}
	s.mu.Unlock()
	return nil
}

func (s *StudioService) EmulatorProfiles() ([]emulator.Profile, error) {
	return s.emulatorProfiles.Load()
}

func (s *StudioService) EmulatorSaveProfile(profile emulator.Profile) (emulator.Profile, error) {
	return s.emulatorProfiles.Save(profile)
}

func (s *StudioService) EmulatorDeleteProfile(id string) error {
	return s.emulatorProfiles.Delete(id)
}

func (s *StudioService) EmulatorStart(profileID, artifact string) (emulator.State, error) {
	profile, err := s.emulatorProfiles.Find(profileID)
	if err != nil {
		return emulator.State{}, err
	}
	if strings.TrimSpace(artifact) == "" {
		artifact = profile.Media
	}
	configured := ""
	var localProject *project.Project
	s.mu.RLock()
	if s.active != nil {
		configured = s.active.Config.Emulator
		localProject = s.active
	}
	s.mu.RUnlock()
	hostDir, err := ensureProjectHostDir(localProject)
	if err != nil {
		return emulator.State{}, err
	}
	hostTool := "mkatr-go"
	if localProject != nil {
		hostTool = localProject.Config.ATR.Tool
	}
	launch := emulator.Launch{Helper: emulatorHelper(configured), Artifact: artifact, HostDir: hostDir, HostTool: hostTool, Profile: profile}
	if profile.EOS {
		launch.EOSCar = compiler.AssetPath("EOS.car")
	}
	if err = s.startEmulator(launch); err != nil {
		return emulator.State{}, err
	}
	s.mu.RLock()
	deviceName := s.emulatorLaunch.HostDeviceName
	s.mu.RUnlock()
	return emulator.State{Running: true, ProfileID: profile.ID, Artifact: artifact, HostDeviceName: deviceName}, nil
}

func (s *StudioService) EmulatorRestart() (emulator.State, error) {
	s.mu.RLock()
	launch := s.emulatorLaunch
	s.mu.RUnlock()
	if launch.Helper == "" {
		return emulator.State{}, errors.New("start Atari800 before restarting it")
	}
	if err := s.startEmulator(launch); err != nil {
		return emulator.State{}, err
	}
	s.mu.RLock()
	deviceName := s.emulatorLaunch.HostDeviceName
	s.mu.RUnlock()
	return emulator.State{Running: true, ProfileID: launch.Profile.ID, Artifact: launch.Artifact, HostDeviceName: deviceName}, nil
}

func (s *StudioService) EmulatorStatus() emulator.State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return emulator.State{Running: s.emulator != nil, ProfileID: s.emulatorLaunch.Profile.ID, Artifact: s.emulatorLaunch.Artifact, HostDeviceName: s.emulatorLaunch.HostDeviceName}
}

func (s *StudioService) EmulatorFrame() (emulator.Frame, error) {
	s.mu.RLock()
	session := s.emulator
	launch := s.emulatorLaunch
	s.mu.RUnlock()
	if session == nil {
		return emulator.Frame{}, errors.New("Atari800 is not running")
	}
	frame, err := session.Frame()
	if err != nil {
		s.mu.Lock()
		if s.emulator == session {
			s.emulator = nil
		}
		s.mu.Unlock()
		_ = session.Stop()
		return frame, err
	}
	changed, syncErr := s.maybeSynchronizeEOSHostDisk(session, launch)
	frame.WorkspaceChanged = changed
	if syncErr != nil {
		frame.Error = syncErr.Error()
	}
	return frame, err
}

func (s *StudioService) EmulatorKey(key string, down bool) error {
	s.mu.RLock()
	session := s.emulator
	s.mu.RUnlock()
	if session == nil {
		return errors.New("Atari800 is not running")
	}
	return session.Key(key, down)
}

func (s *StudioService) EmulatorReleaseKeys() error {
	s.mu.RLock()
	session := s.emulator
	s.mu.RUnlock()
	if session == nil {
		return nil
	}
	return session.ReleaseKeys()
}

func (s *StudioService) EmulatorStop() error {
	s.eosSyncMu.Lock()
	defer s.eosSyncMu.Unlock()
	s.mu.Lock()
	session := s.emulator
	launch := s.emulatorLaunch
	s.emulator = nil
	s.mu.Unlock()
	if session == nil {
		return nil
	}
	if err := session.Stop(); err != nil {
		return err
	}
	err := synchronizeEOSHostDisk(launch)
	s.mu.Lock()
	s.emulatorDiskMod = time.Time{}
	s.emulatorDiskSize = 0
	s.emulatorDiskCheck = time.Time{}
	s.mu.Unlock()
	return err
}
func (s *StudioService) GitStatus() (git.Status, error) {
	if s.isRemote() {
		p, e := s.currentRemote()
		if e != nil {
			return git.Status{}, e
		}
		return p.GitStatus()
	}
	p, err := s.current()
	if err != nil {
		return git.Status{}, err
	}
	return git.GetStatus(p)
}
func (s *StudioService) GitInit() (string, error) {
	if s.isRemote() {
		p, e := s.currentRemote()
		if e != nil {
			return "", e
		}
		return p.Git("init")
	}
	p, err := s.current()
	if err != nil {
		return "", err
	}
	return git.Init(p)
}
func (s *StudioService) GitDiff(file string, staged bool) (string, error) {
	if s.isRemote() {
		p, e := s.currentRemote()
		if e != nil {
			return "", e
		}
		args := []string{"diff"}
		if staged {
			args = append(args, "--cached")
		}
		args = append(args, "--")
		if file == "" {
			file = "."
		}
		args = append(args, file)
		return p.Git(args...)
	}
	p, err := s.current()
	if err != nil {
		return "", err
	}
	return git.Diff(p, file, staged)
}
func (s *StudioService) GitStageAll() (string, error) {
	if s.isRemote() {
		p, e := s.currentRemote()
		if e != nil {
			return "", e
		}
		return p.Git("add", "--all", "--", ".")
	}
	p, err := s.current()
	if err != nil {
		return "", err
	}
	return git.StageAll(p)
}
func (s *StudioService) GitUnstage(file string) (string, error) {
	if s.isRemote() {
		p, e := s.currentRemote()
		if e != nil {
			return "", e
		}
		return p.Git("restore", "--staged", "--", file)
	}
	p, err := s.current()
	if err != nil {
		return "", err
	}
	return git.Unstage(p, file)
}
func (s *StudioService) GitUnstageAll() (string, error) {
	if s.isRemote() {
		p, e := s.currentRemote()
		if e != nil {
			return "", e
		}
		return p.Git("restore", "--staged", "--", ".")
	}
	p, err := s.current()
	if err != nil {
		return "", err
	}
	return git.UnstageAll(p)
}
func (s *StudioService) GitStage(file string) (string, error) {
	if s.isRemote() {
		p, e := s.currentRemote()
		if e != nil {
			return "", e
		}
		return p.Git("add", "--", file)
	}
	p, err := s.current()
	if err != nil {
		return "", err
	}
	return git.Stage(p, file)
}
func (s *StudioService) GitCommit(message string) (string, error) {
	if s.isRemote() {
		p, e := s.currentRemote()
		if e != nil {
			return "", e
		}
		return p.Git("commit", "-m", message)
	}
	p, err := s.current()
	if err != nil {
		return "", err
	}
	return git.Commit(p, message)
}
func (s *StudioService) ManualFiles() []string                     { return manual.Files() }
func (s *StudioService) ManualChapter(file string) (string, error) { return manual.Chapter(file) }
func (s *StudioService) SearchManual(query string) []manual.Hit    { return manual.Search(query, 12) }
func (s *StudioService) AskClaude(prompt, filename, selectedCode, mode string) (string, error) {
	if s.isRemote() {
		return "", errors.New("Claude Code remote sessions require Claude Code on the SSH host; use the remote terminal until remote Claude transport is configured")
	}
	p, err := s.current()
	if err != nil {
		return "", err
	}
	s.mu.RLock()
	sessionID := s.claudeSessions[p.Root]
	s.mu.RUnlock()

	result, err := claudecode.Ask(p.Root, prompt, filename, selectedCode, sessionID, mode)
	if err != nil {
		return "", err
	}
	if result.SessionID != "" {
		s.mu.Lock()
		s.claudeSessions[p.Root] = result.SessionID
		s.mu.Unlock()
	}
	return result.Text, nil
}
func (s *StudioService) ResetClaudeSession() error {
	p, err := s.current()
	if err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.claudeSessions, p.Root)
	s.mu.Unlock()
	return nil
}
func (s *StudioService) Extensions() ([]extensions.Entry, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return extensions.Discover(filepath.Join(base, "exbasic-studio", "extensions"))
}
