package studio

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"exbasicstudio/internal/atrexplorer"
	"exbasicstudio/internal/diskimage"
	"exbasicstudio/internal/emulator"
)

// prepareEOSHostDisk exposes the normal project H folder through a disk device
// EOS actually implements. ex800's inherited H: patch is an Atari OS CIO handler and
// is intentionally not advertised as an EOS device.
func prepareEOSHostDisk(launch emulator.Launch) (emulator.Launch, error) {
	if launch.EOSCar == "" || launch.HostDir == "" {
		if launch.HostDir != "" {
			launch.HostDeviceName = "H1:"
		}
		return launch, nil
	}
	tool, err := diskimage.FindTool(launch.HostTool)
	if err != nil {
		return launch, fmt.Errorf("prepare EOS project disk: %w", err)
	}
	if launch.HostDisk == "" {
		launch.HostDisk = filepath.Join(filepath.Dir(launch.HostDir), ".exbasic-studio", "eos-host.atr")
	}
	if err = os.MkdirAll(filepath.Dir(launch.HostDisk), 0o755); err != nil {
		return launch, fmt.Errorf("create EOS project disk cache: %w", err)
	}
	paths, err := hostDiskPaths(launch.HostDir)
	if err != nil {
		return launch, err
	}
	if _, err = atrexplorer.Create(tool, launch.HostDisk, "eos", paths); err != nil {
		return launch, fmt.Errorf("create EOS project disk: %w", err)
	}
	launch.HostTool = tool
	launch.HostDeviceName = "D8:"
	return launch, nil
}

// maxHostDiskEntries bounds the folder copied onto the EOS host disk, so a
// mistaken choice such as a whole drive fails at once.
const maxHostDiskEntries = 4096

func hostDiskPaths(root string) ([]string, error) {
	paths := []string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != root {
			if len(paths) >= maxHostDiskEntries {
				return fmt.Errorf("the folder %s holds more than %d files and folders, too many for an EOS disk; choose a smaller H: folder", root, maxHostDiskEntries)
			}
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read project H folder: %w", err)
	}
	return paths, nil
}

// synchronizeEOSHostDisk copies files created or changed by EOS back into H.
// Host-only files are retained so stopping the emulator can never erase work.
func synchronizeEOSHostDisk(launch emulator.Launch) error {
	if launch.HostDeviceName != "D8:" || launch.HostDisk == "" || launch.HostDir == "" || launch.HostTool == "" {
		return nil
	}
	view, err := atrexplorer.Catalog(launch.HostTool, launch.HostDisk)
	if err != nil {
		return fmt.Errorf("read EOS project disk: %w", err)
	}
	root := filepath.Clean(launch.HostDir)
	for _, entry := range view.Entries {
		relative := filepath.Clean(filepath.FromSlash(entry.Path))
		if relative == "." || relative == ".." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("EOS project disk contains unsafe path %q", entry.Path)
		}
		target := filepath.Join(root, relative)
		if entry.Directory {
			if err = os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err = atrexplorer.Extract(launch.HostTool, launch.HostDisk, entry.Path, target); err != nil {
			return fmt.Errorf("copy %s from EOS project disk: %w", entry.Path, err)
		}
	}
	return nil
}

// isProjectHostPath reports whether the project path rel is the folder the
// Atari sees as its host device, or lies inside it.
func isProjectHostPath(root, hostDir, rel string) bool {
	if root == "" || hostDir == "" {
		return false
	}
	host, err := filepath.Rel(root, hostDir)
	if err != nil {
		return false
	}
	host = filepath.ToSlash(filepath.Clean(host))
	if host == ".." || strings.HasPrefix(host, "../") || filepath.IsAbs(host) {
		return false // the host folder is outside the project
	}
	if host == "." {
		return true
	}
	clean := filepath.ToSlash(filepath.Clean(rel))
	return strings.EqualFold(clean, host) || (len(clean) > len(host) && strings.EqualFold(clean[:len(host)+1], host+"/"))
}

// withEOSHostMutation keeps the project's H folder and the EOS D8 image in
// lockstep while the emulator is running. Atari-side writes are pulled back
// first, then the host mutation is applied, and finally a fresh image is
// mounted without resetting the emulated machine.
func (s *StudioService) withEOSHostMutation(paths []string, action func() error) (err error) {
	s.mu.RLock()
	root, hostDir := "", s.emulatorLaunch.HostDir
	if s.active != nil {
		root = s.active.Root
	}
	s.mu.RUnlock()
	usesHostFolder := false
	for _, rel := range paths {
		if isProjectHostPath(root, hostDir, rel) {
			usesHostFolder = true
			break
		}
	}
	if !usesHostFolder {
		return action()
	}

	s.eosSyncMu.Lock()
	defer s.eosSyncMu.Unlock()
	s.mu.RLock()
	session := s.emulator
	launch := s.emulatorLaunch
	s.mu.RUnlock()
	if session == nil || launch.HostDeviceName != "D8:" || launch.HostDisk == "" {
		return action()
	}
	if err = session.DismountHostDisk(); err != nil {
		return fmt.Errorf("dismount EOS project D8: %w", err)
	}
	remounted := false
	defer func() {
		if !remounted {
			_ = session.MountHostDisk()
		}
	}()
	if err = synchronizeEOSHostDisk(launch); err != nil {
		return err
	}
	if err = action(); err != nil {
		return err
	}
	launch, err = prepareEOSHostDisk(launch)
	if err != nil {
		return err
	}
	if err = session.MountHostDisk(); err != nil {
		return fmt.Errorf("remount EOS project D8: %w", err)
	}
	remounted = true
	s.mu.Lock()
	if s.emulator == session {
		s.emulatorLaunch = launch
		if info, statErr := os.Stat(launch.HostDisk); statErr == nil {
			s.emulatorDiskMod = info.ModTime()
			s.emulatorDiskSize = info.Size()
		}
		s.emulatorDiskCheck = time.Now()
	}
	s.mu.Unlock()
	return nil
}

// maybeSynchronizeEOSHostDisk mirrors Atari-side D8 changes into the normal H
// folder without stopping or resetting the machine. The Atari core flushes
// sector writes, so the image timestamp lets us avoid needless dismounts.
func (s *StudioService) maybeSynchronizeEOSHostDisk(session *emulator.Session, launch emulator.Launch) (bool, error) {
	if launch.HostDeviceName != "D8:" || launch.HostDisk == "" || launch.HostDir == "" {
		return false, nil
	}
	now := time.Now()
	s.mu.Lock()
	if s.emulator != session || (!s.emulatorDiskCheck.IsZero() && now.Sub(s.emulatorDiskCheck) < 750*time.Millisecond) {
		s.mu.Unlock()
		return false, nil
	}
	s.emulatorDiskCheck = now
	baselineMod, baselineSize := s.emulatorDiskMod, s.emulatorDiskSize
	s.mu.Unlock()

	info, err := os.Stat(launch.HostDisk)
	if err != nil {
		return false, fmt.Errorf("inspect EOS project D8: %w", err)
	}
	if info.ModTime().Equal(baselineMod) && info.Size() == baselineSize {
		return false, nil
	}

	s.eosSyncMu.Lock()
	defer s.eosSyncMu.Unlock()
	s.mu.RLock()
	current := s.emulator == session
	baselineMod, baselineSize = s.emulatorDiskMod, s.emulatorDiskSize
	s.mu.RUnlock()
	if !current {
		return false, nil
	}
	info, err = os.Stat(launch.HostDisk)
	if err != nil {
		return false, fmt.Errorf("inspect EOS project D8: %w", err)
	}
	if info.ModTime().Equal(baselineMod) && info.Size() == baselineSize {
		return false, nil
	}
	if err = session.DismountHostDisk(); err != nil {
		return false, fmt.Errorf("dismount changed EOS project D8: %w", err)
	}
	remounted := false
	defer func() {
		if !remounted {
			_ = session.MountHostDisk()
		}
	}()
	if err = synchronizeEOSHostDisk(launch); err != nil {
		return false, err
	}
	if err = session.MountHostDisk(); err != nil {
		return false, fmt.Errorf("remount synchronized EOS project D8: %w", err)
	}
	remounted = true
	info, err = os.Stat(launch.HostDisk)
	if err != nil {
		return false, fmt.Errorf("inspect synchronized EOS project D8: %w", err)
	}
	s.mu.Lock()
	if s.emulator == session {
		s.emulatorDiskMod = info.ModTime()
		s.emulatorDiskSize = info.Size()
		s.emulatorDiskCheck = time.Now()
	}
	s.mu.Unlock()
	return true, nil
}
