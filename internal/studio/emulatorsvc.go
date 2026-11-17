package studio

import (
	"errors"
	"exbasicstudio/internal/atrexplorer"
	"exbasicstudio/internal/diskimage"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"exbasicstudio/internal/emulator"
	"exbasicstudio/internal/project"
	"exbasicstudio/internal/tools"
)

// emulatorHelper maps the project's emulator setting to the helper program.
// Older projects named the stock emulator; those use the bundled helper.
func emulatorHelper(configured string) string {
	helper := configured
	base := strings.TrimSuffix(strings.ToLower(filepath.Base(helper)), ".exe")
	if helper == "" || base == "atari800" || base == "atari800-headless" {
		helper = "atari800-studio"
	}
	return tools.Path(helper)
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
			s.log.Warn("stop previous emulator: %v", stopErr)
		}
		s.endDebug(previous, "")
		if syncErr := synchronizeEOSHostDisk(previousLaunch); syncErr != nil {
			return syncErr
		}
	}
	prepared, err := prepareEOSHostDisk(launch)
	if err != nil {
		return err
	}
	launch = prepared
	if launch, err = s.prepareProgramDisk(launch); err != nil {
		return err
	}
	session, err := emulator.Start(launch)
	if err != nil {
		s.log.Error("start emulator: %v", err)
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

// hostDiskFor says where the EOS image of the host folder is kept. A folder
// chosen in the profile gets its image in Studio's own data directory, so
// nothing is written beside a folder Studio does not own. Blank leaves the
// image in the project's cache.
func (s *StudioService) hostDiskFor(profile emulator.Profile) string {
	if profile.HostDir == "" {
		return ""
	}
	return filepath.Join(s.state.Dir(), "run", "eos-host.atr")
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

// EmulatorStart boots the emulator with a profile and optional media. With
// an untrusted workspace open, the bundled helper and tools are used and
// the project's H folder is neither created nor mounted.
func (s *StudioService) EmulatorStart(profileID, artifact string) (emulator.State, error) {
	profile, err := s.emulatorProfiles.Find(profileID)
	if err != nil {
		return emulator.State{}, err
	}
	if strings.TrimSpace(artifact) == "" {
		artifact = profile.Media
	}
	var trusted *project.Project
	s.mu.RLock()
	active := s.active
	s.mu.RUnlock()
	if active != nil && s.isTrusted(active) {
		trusted = active
	}
	configured, hostTool := "", "mkatr-go"
	if trusted != nil {
		configured, hostTool = trusted.Config.Emulator, trusted.Config.ATR.Tool
	}
	hostDir, err := s.ensureHostDir(trusted, profile.HostDir)
	if err != nil {
		return emulator.State{}, err
	}
	launch := emulator.Launch{Helper: emulatorHelper(configured), Artifact: artifact, HostDir: hostDir, HostDisk: s.hostDiskFor(profile), HostTool: hostTool, Profile: profile}
	if err = applyCartridges(&launch, profile); err != nil {
		return emulator.State{}, err
	}
	if err = s.startEmulator(launch); err != nil {
		return emulator.State{}, err
	}
	return s.EmulatorStatus(), nil
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
	return s.EmulatorStatus(), nil
}

func (s *StudioService) EmulatorStatus() emulator.State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return emulator.State{Running: s.emulator != nil, ProfileID: s.emulatorLaunch.Profile.ID, Artifact: s.emulatorLaunch.Artifact, HostDeviceName: s.emulatorLaunch.HostDeviceName}
}

// EmulatorFrame advances the machine and returns the rendered screen. A
// helper failure stops the session cleanly: pressed keys are released, EOS
// storage is synchronized and the session can be restarted.
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
		s.log.Error("emulator helper failed: %v", err)
		s.mu.Lock()
		owned := s.emulator == session
		if owned {
			s.emulator = nil
		}
		s.mu.Unlock()
		_ = session.Stop()
		// A failed helper ends any debug session as a stop the UI can show.
		s.endDebug(session, "The emulator stopped unexpectedly: "+err.Error())
		if owned {
			s.eosSyncMu.Lock()
			if syncErr := synchronizeEOSHostDisk(launch); syncErr != nil {
				s.log.Warn("synchronize EOS storage after helper failure: %v", syncErr)
			}
			s.eosSyncMu.Unlock()
		}
		return frame, err
	}
	s.observeDebug(session, frame.Debug)
	changed, syncErr := s.maybeSynchronizeEOSHostDisk(session, launch)
	frame.WorkspaceChanged = changed
	if syncErr != nil {
		frame.Error = syncErr.Error()
	}
	return frame, nil
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

// EmulatorJoystick sets joystick 0 of the running machine from the keyboard
// joystick: stick is the Atari STICK() value (15 centred) and trigger is
// true while fire is held.
func (s *StudioService) EmulatorJoystick(stick int, trigger bool) error {
	s.mu.RLock()
	session := s.emulator
	s.mu.RUnlock()
	if session == nil {
		return errors.New("Atari800 is not running")
	}
	return session.Joystick(stick, trigger)
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
	stopErr := session.Stop()
	s.endDebug(session, "")
	// Storage is synchronized even when the helper exited abnormally.
	syncErr := synchronizeEOSHostDisk(launch)
	s.mu.Lock()
	s.emulatorDiskMod = time.Time{}
	s.emulatorDiskSize = 0
	s.emulatorDiskCheck = time.Time{}
	s.mu.Unlock()
	if syncErr != nil {
		return syncErr
	}
	if stopErr != nil {
		s.log.Warn("emulator helper exit: %v", stopErr)
	}
	return nil
}

// isMediaImage reports artifacts the emulator mounts or inserts as they are.
func isMediaImage(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".atr", ".xfd", ".car", ".rom":
		return true
	}
	return false
}

var plainProgramName = regexp.MustCompile(`^[A-Z0-9]{1,8}(\.[A-Z0-9]{1,3})?$`)

// prepareProgramDisk puts a bare executable on a disk when EOS is the
// operating system. EOS starts from its cartridge and loads programs from
// its own filesystem, so an executable handed to the emulator directly would
// appear as an unreadable disk in D1:. The disk lives in the application's
// own directory, never in the project.
func (s *StudioService) prepareProgramDisk(launch emulator.Launch) (emulator.Launch, error) {
	if launch.EOSCar == "" || launch.Artifact == "" || isMediaImage(launch.Artifact) {
		return launch, nil
	}
	tool, err := diskimage.FindTool(launch.HostTool)
	if err != nil {
		return launch, fmt.Errorf("prepare the EOS program disk: %w", err)
	}
	dir := filepath.Join(s.state.Dir(), "run")
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return launch, fmt.Errorf("prepare the EOS program disk: %w", err)
	}
	disk := filepath.Join(dir, "program.atr")
	_ = os.Remove(disk)
	view, err := atrexplorer.Create(tool, disk, "eos", []string{launch.Artifact})
	if err != nil {
		return launch, fmt.Errorf("create the EOS program disk: %w", err)
	}
	launch.Artifact = disk
	if len(view.Entries) == 1 {
		launch.Program = view.Entries[0].Name
	}
	return launch, nil
}

// startProgram types the program's name at the EOS prompt, as a user would,
// so that Run and Debug start the program instead of stopping at the
// prompt. It reports whether the program was started; when it was not, the
// program is still on D1: for the user to start.
func startProgram(session *emulator.Session, name string) bool {
	name = strings.ToUpper(name)
	if session == nil || !plainProgramName.MatchString(name) {
		return false
	}
	run := func(frames int) bool {
		for range frames {
			if _, err := session.Frame(); err != nil {
				return false
			}
		}
		return true
	}
	ready := false
	for attempt := 0; attempt < 90 && !ready; attempt++ {
		if !run(10) {
			return false
		}
		text, err := session.ScreenText(24)
		if err != nil {
			return false
		}
		ready = strings.Contains(text, "D1:")
	}
	if !ready {
		return false
	}
	for _, key := range append(strings.Split(strings.ToLower(name), ""), "Enter") {
		if session.Key(key, true) != nil || !run(3) || session.Key(key, false) != nil || !run(3) {
			return false
		}
	}
	return true
}
