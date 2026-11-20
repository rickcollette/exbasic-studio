package studio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"exbasicstudio/internal/compiler"
	"exbasicstudio/internal/emulator"
	"exbasicstudio/internal/operations"
	"exbasicstudio/internal/project"
	"exbasicstudio/internal/tools"
)

// BuildRequest starts a build of the open project. The frontend saves (or
// explicitly confirms discarding) dirty sources before sending it.
type BuildRequest struct{}

// RunRequest builds the project and boots it in the embedded emulator.
type RunRequest struct {
	ProfileID string `json:"profileId"`
}

// RunResult is the completion result of a run operation.
type RunResult struct {
	Build    compiler.Result `json:"build"`
	Started  bool            `json:"started"`
	Emulator emulator.State  `json:"emulator"`
}

// LintResult echoes the document version it was computed for, so the editor
// can drop a result that arrives after the text changed again.
type LintResult struct {
	Path        string                `json:"path"`
	Version     int                   `json:"version"`
	Available   bool                  `json:"available"`
	Diagnostics []compiler.Diagnostic `json:"diagnostics"`
}

// StartBuild compiles the project in the background. Output and the final
// structured result arrive as "operation" events carrying the returned ID.
func (s *StudioService) StartBuild(_ BuildRequest) (operations.Info, error) {
	p, err := s.current()
	if err != nil {
		return operations.Info{}, err
	}
	if err = s.requireTrust(p, "build it"); err != nil {
		return operations.Info{}, err
	}
	snapshot := *p
	return s.operations.Start("build", true, func(ctx context.Context, op *operations.Operation) (any, error) {
		result, buildErr := compiler.Build(ctx, &snapshot, op.Output)
		s.logBuild("build", result, buildErr)
		return result, buildErr
	})
}

// StartRun builds and, on success, starts the emulator with the artifact.
func (s *StudioService) StartRun(request RunRequest) (operations.Info, error) {
	p, err := s.current()
	if err != nil {
		return operations.Info{}, err
	}
	if err = s.requireTrust(p, "run it"); err != nil {
		return operations.Info{}, err
	}
	profile, err := s.emulatorProfiles.Find(request.ProfileID)
	if err != nil {
		return operations.Info{}, err
	}
	snapshot := *p
	return s.operations.Start("build", true, func(ctx context.Context, op *operations.Operation) (any, error) {
		result := RunResult{}
		var buildErr error
		result.Build, buildErr = compiler.Build(ctx, &snapshot, op.Output)
		s.logBuild("run", result.Build, buildErr)
		if buildErr != nil || !result.Build.Success {
			return result, buildErr
		}
		if ctx.Err() != nil {
			result.Build.Cancelled, result.Build.Success = true, false
			return result, nil
		}
		launch, launchErr := s.launchFor(&snapshot, profile, result.Build)
		if launchErr != nil {
			return result, launchErr
		}
		if startErr := s.startEmulator(launch); startErr != nil {
			return result, startErr
		}
		op.Output("stdout", "\nex800 started in an editor tab.\n")
		s.mu.RLock()
		session, started := s.emulator, s.emulatorLaunch
		s.mu.RUnlock()
		if started.Program != "" {
			if startProgram(session, started.Program) {
				op.Output("stdout", "EOS is the operating system of this profile: the program was copied to D1: and started as "+started.Program+".\n")
			} else {
				op.Output("stdout", "EOS is the operating system of this profile: the program is on D1:. Type "+started.Program+" at the D1: prompt to start it.\n")
			}
		}
		result.Started = true
		result.Emulator = s.EmulatorStatus()
		return result, nil
	})
}

func (s *StudioService) logBuild(kind string, result compiler.Result, err error) {
	switch {
	case err != nil:
		s.log.Error("%s failed to run: %v", kind, err)
	case result.Cancelled:
		s.log.Info("%s cancelled after %d ms", kind, result.DurationMS)
	case result.Success:
		s.log.Info("%s succeeded in %d ms", kind, result.DurationMS)
	default:
		s.log.Info("%s failed with %d diagnostics in %d ms", kind, len(result.Diagnostics), result.DurationMS)
	}
}

// launchFor describes how to boot a build result in the emulator.
func (s *StudioService) launchFor(p *project.Project, profile emulator.Profile, build compiler.Result) (emulator.Launch, error) {
	artifact := build.Artifact("executable")
	eosCar := ""
	if disk := build.Artifact("disk-image"); disk != "" {
		artifact = disk
		if p.Config.ATR.Filesystem == "eos" {
			eosCar = eosCartridge()
		}
	}
	hostDir, err := s.ensureHostDir(p, profile.HostDir)
	if err != nil {
		return emulator.Launch{}, err
	}
	launch := emulator.Launch{Helper: emulatorHelper(p.Config.Emulator), Artifact: artifact, EOSCar: eosCar, HostDir: hostDir, HostDisk: s.hostDiskFor(profile), HostTool: p.Config.ATR.Tool, Profile: profile}
	if launch.EOSCar == "missing" {
		return emulator.Launch{}, errors.New("the bundled EOS cartridge (EOS.car) is missing; reinstall exBASIC Studio")
	}
	if err = applyCartridges(&launch, profile); err != nil {
		return emulator.Launch{}, err
	}
	return launch, nil
}

// applyCartridges turns the profile's cartridge slots into launch settings.
// A project packaged as an EOSFS disk already selected the EOS cartridge for
// the primary slot; that choice is kept.
func applyCartridges(launch *emulator.Launch, profile emulator.Profile) error {
	resolve := func(slot emulator.CartridgeSlot, name string) (string, int, error) {
		switch slot.Kind {
		case emulator.CartEOS:
			path := eosCartridge()
			if path == "missing" {
				return "", 0, errors.New("the bundled EOS cartridge (EOS.car) is missing; reinstall exBASIC Studio")
			}
			return path, 0, nil
		case emulator.CartSDX, emulator.CartImage:
			kind, err := cartridgeType(slot)
			if err != nil {
				return "", 0, fmt.Errorf("%s cartridge slot: %w", name, err)
			}
			return slot.Image, kind, nil
		}
		return "", 0, nil // empty, or BASIC, which is a machine switch
	}
	primary, primaryType, err := resolve(profile.Cartridge, "primary")
	if err != nil {
		return err
	}
	secondary, secondaryType, err := resolve(profile.Cartridge2, "secondary")
	if err != nil {
		return err
	}
	if launch.EOSCar == "" {
		if profile.Cartridge.Kind == emulator.CartEOS {
			launch.EOSCar = primary
		} else {
			launch.Cart, launch.CartType = primary, primaryType
		}
	}
	launch.Cart2, launch.Cart2Type = secondary, secondaryType
	return nil
}

// cartridgeType decides the ex800 type number of a cartridge image. A
// .car file names its own type in its header, which ex800 reads (zero is
// returned). A raw image has no header: the chosen type is used, or one
// implied by the file size when the size allows only one sensible answer.
func cartridgeType(slot emulator.CartridgeSlot) (int, error) {
	file, err := os.Open(slot.Image)
	if err != nil {
		return 0, fmt.Errorf("open the cartridge image: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		return 0, fmt.Errorf("%s is not a cartridge image", slot.Image)
	}
	header := make([]byte, 4)
	if n, _ := file.Read(header); n == 4 && string(header) == "CART" {
		return 0, nil
	}
	if slot.Type > 0 {
		return slot.Type, nil
	}
	const (
		standard8K, standard16K = 1, 2
		sdx64K, sdx128K         = 11, 43
	)
	switch size := info.Size(); {
	case slot.Kind == emulator.CartSDX && size == 64<<10:
		return sdx64K, nil
	case slot.Kind == emulator.CartSDX && size == 128<<10:
		return sdx128K, nil
	case slot.Kind == emulator.CartImage && size == 8<<10:
		return standard8K, nil
	case slot.Kind == emulator.CartImage && size == 16<<10:
		return standard16K, nil
	}
	return 0, fmt.Errorf("%s has no cartridge header; choose its cartridge type in the profile", filepath.Base(slot.Image))
}

// eosCartridge returns the bundled cartridge path, or "missing".
func eosCartridge() string {
	if path, ok := tools.Asset("EOS.car"); ok {
		return path
	}
	return "missing"
}

// CancelOperation requests cancellation of a running operation. The
// operation still delivers exactly one completion event.
func (s *StudioService) CancelOperation(id string) bool {
	return s.operations.Cancel(id)
}

// RunningOperations lists operations that have not completed, so a reloaded
// frontend can reattach to them.
func (s *StudioService) RunningOperations() []operations.Info {
	return s.operations.Running()
}

// Lint checks unsaved editor text with the real compiler. overlays carries
// the unsaved text of other open files so included files are checked as
// they appear on screen. In restricted mode it reports Available=false
// instead of running anything.
func (s *StudioService) Lint(rel, content string, version int, overlays map[string]string) (LintResult, error) {
	result := LintResult{Path: rel, Version: version, Diagnostics: []compiler.Diagnostic{}}
	p, err := s.current()
	if err != nil {
		return result, err
	}
	if !s.isTrusted(p) {
		return result, nil
	}
	diagnostics, err := compiler.Lint(context.Background(), p, rel, content, overlays)
	if err != nil {
		return result, err
	}
	result.Available = true
	result.Diagnostics = diagnostics
	return result, nil
}
