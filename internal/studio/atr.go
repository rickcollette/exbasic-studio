package studio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"exbasicstudio/internal/atrexplorer"
	"exbasicstudio/internal/diskimage"
	"exbasicstudio/internal/project"
)

// atrTool returns the mkatr-go executable. A project-configured tool is
// honoured only in a trusted workspace; otherwise the bundled one is used.
func (s *StudioService) atrTool() (string, error) {
	configured := "mkatr-go"
	s.mu.RLock()
	active := s.active
	s.mu.RUnlock()
	if active != nil && strings.TrimSpace(active.Config.ATR.Tool) != "" && s.isTrusted(active) {
		configured = active.Config.ATR.Tool
	}
	return diskimage.FindTool(configured)
}

func checkImagePath(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("choose an ATR image")
	}
	if !filepath.IsAbs(path) {
		return errors.New("ATR image path must be absolute")
	}
	return nil
}

// ATROpen reads an arbitrary local SDFS or EOSFS ATR image.
func (s *StudioService) ATROpen(path string) (atrexplorer.View, error) {
	if err := checkImagePath(path); err != nil {
		return atrexplorer.View{}, err
	}
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
	if _, statErr := os.Stat(path); statErr != nil {
		return atrexplorer.View{}, fmt.Errorf("%s does not exist yet; build the project with ATR packaging enabled", p.Config.ATR.Output)
	}
	return s.ATROpen(path)
}

// ATRImportFiles imports native host paths, including directory trees.
func (s *StudioService) ATRImportFiles(image, destination string, paths []string) (atrexplorer.View, error) {
	if err := checkImagePath(image); err != nil {
		return atrexplorer.View{}, err
	}
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
	if err := checkImagePath(image); err != nil {
		return atrexplorer.View{}, err
	}
	tool, err := s.atrTool()
	if err != nil {
		return atrexplorer.View{}, err
	}
	return atrexplorer.Mkdir(tool, image, path)
}

// ATRRemove deletes an entry. The UI confirms with the user first.
func (s *StudioService) ATRRemove(image, path string) (atrexplorer.View, error) {
	if err := checkImagePath(image); err != nil {
		return atrexplorer.View{}, err
	}
	tool, err := s.atrTool()
	if err != nil {
		return atrexplorer.View{}, err
	}
	return atrexplorer.Remove(tool, image, path)
}

func (s *StudioService) ATRRename(image, oldPath, newPath string) (atrexplorer.View, error) {
	if err := checkImagePath(image); err != nil {
		return atrexplorer.View{}, err
	}
	tool, err := s.atrTool()
	if err != nil {
		return atrexplorer.View{}, err
	}
	return atrexplorer.Rename(tool, image, oldPath, newPath)
}

// atrExtract copies one entry to output. An existing destination is only
// replaced when overwrite is set, so the UI can prompt first.
func (s *StudioService) atrExtract(image, path, output string, overwrite bool) error {
	if err := checkImagePath(image); err != nil {
		return err
	}
	if _, err := os.Lstat(output); err == nil && !overwrite {
		return fmt.Errorf("exists: %s already exists", filepath.Base(output))
	}
	tool, err := s.atrTool()
	if err != nil {
		return err
	}
	return atrexplorer.Extract(tool, image, path, output)
}

// ATRExtractToDirectory extracts an entry into a host directory. It fails
// with an "exists:" error unless overwrite is set.
func (s *StudioService) ATRExtractToDirectory(image, path, directory string, overwrite bool) error {
	if strings.TrimSpace(directory) == "" || !filepath.IsAbs(directory) {
		return errors.New("choose an extraction directory")
	}
	name := filepath.Base(filepath.FromSlash(path))
	if name == "." || name == ".." || name == string(filepath.Separator) {
		return errors.New("invalid ATR entry name")
	}
	return s.atrExtract(image, path, filepath.Join(directory, name), overwrite)
}

// ATRExtractToProject copies an ATR entry into a workspace directory.
func (s *StudioService) ATRExtractToProject(image, path, destination string, overwrite bool) (project.View, error) {
	p, err := s.current()
	if err != nil {
		return project.View{}, err
	}
	name := filepath.Base(filepath.FromSlash(path))
	if name == "." || name == ".." || name == string(filepath.Separator) {
		return project.View{}, errors.New("invalid ATR entry name")
	}
	rel := filepath.ToSlash(filepath.Join(destination, name))
	target, err := p.Resolve(rel)
	if err != nil {
		return project.View{}, err
	}
	if err = s.withEOSHostMutation([]string{rel}, func() error { return s.atrExtract(image, path, target, overwrite) }); err != nil {
		return project.View{}, err
	}
	return p.View()
}
