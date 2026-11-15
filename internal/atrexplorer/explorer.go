// Package atrexplorer adapts the separately licensed mkatr-go executable for
// Studio's visual ATR explorer. No GPL implementation is linked into Studio.
package atrexplorer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	processutil "exbasicstudio/internal/process"
)

type Entry struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Size       int       `json:"size"`
	Directory  bool      `json:"directory"`
	Attributes byte      `json:"attributes"`
	Boot       bool      `json:"boot"`
	Modified   time.Time `json:"modified"`
}

type View struct {
	Path       string  `json:"path"`
	Filesystem string  `json:"filesystem"`
	SectorSize int     `json:"sectorSize"`
	Sectors    int     `json:"sectors"`
	Bytes      int     `json:"bytes"`
	FreeBytes  int     `json:"freeBytes"`
	Entries    []Entry `json:"entries"`
}

// Create writes a new ATR through the separately licensed mkatr-go process.
// Paths must be ordered with parent directories before their children.
func Create(tool, image, filesystem string, paths []string) (View, error) {
	if filesystem == "" {
		filesystem = "sparta"
	}
	args := []string{"-f", filesystem, image}
	args = append(args, paths...)
	if _, err := command(tool, args...); err != nil {
		return View{}, err
	}
	return Catalog(tool, image)
}

func Catalog(tool, image string) (View, error) {
	raw, err := command(tool, "catalog", image)
	if err != nil {
		return View{}, err
	}
	var view View
	if err := json.Unmarshal(raw, &view); err != nil {
		return View{}, fmt.Errorf("decode mkatr-go catalog: %w", err)
	}
	if view.Entries == nil {
		view.Entries = []Entry{}
	}
	return view, nil
}

func Import(tool, image, destination string, paths []string) (View, error) {
	if len(paths) == 0 {
		return View{}, errors.New("drop or choose at least one file")
	}
	args := []string{"put", image, destination}
	args = append(args, paths...)
	if _, err := command(tool, args...); err != nil {
		return View{}, err
	}
	return Catalog(tool, image)
}

func Mkdir(tool, image, path string) (View, error) {
	if _, err := command(tool, "mkdir", image, path); err != nil {
		return View{}, err
	}
	return Catalog(tool, image)
}

func Remove(tool, image, path string) (View, error) {
	if _, err := command(tool, "remove", image, path); err != nil {
		return View{}, err
	}
	return Catalog(tool, image)
}

func Rename(tool, image, oldPath, newPath string) (View, error) {
	if _, err := command(tool, "rename", image, oldPath, newPath); err != nil {
		return View{}, err
	}
	return Catalog(tool, image)
}

func Extract(tool, image, path, output string) error {
	_, err := command(tool, "extract", image, path, output)
	return err
}

func command(tool string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, tool, args...)
	processutil.Background(cmd)
	raw, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return raw, errors.New("ATR operation timed out after two minutes")
	}
	if err == nil {
		return raw, nil
	}
	message := strings.TrimSpace(string(raw))
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if message == "" {
			message = "mkatr-go exited with code " + strconv.Itoa(exit.ExitCode())
		}
		return raw, errors.New(message)
	}
	return raw, err
}
