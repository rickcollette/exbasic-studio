// Package diskimage invokes the separately licensed mkatr-go tool.
package diskimage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	processutil "exbasicstudio/internal/process"
	"exbasicstudio/internal/project"
	"exbasicstudio/internal/tools"
)

type Result struct {
	Image  string
	Argv   []string
	Output string
}

// Arguments returns mkatr-compatible arguments. Paths must already be in the
// coordinate system of the process that will execute the command.
func Arguments(cfg project.ATRConfig, output, artifact string, extras []string) []string {
	args := make([]string, 0, 8+len(extras))
	if cfg.Filesystem != "" && cfg.Filesystem != "sparta" {
		args = append(args, "-f", cfg.Filesystem)
	}
	if cfg.ExactSize {
		args = append(args, "-x")
	}
	if cfg.MinimumSize > 0 {
		args = append(args, "-s", strconv.Itoa(cfg.MinimumSize))
	}
	if cfg.BootPage != 7 {
		args = append(args, "-B", strconv.Itoa(cfg.BootPage))
	}
	args = append(args, output)
	if cfg.Boot {
		args = append(args, "-b")
	}
	args = append(args, artifact)
	args = append(args, extras...)
	return args
}

// Create packages a compiled program into an ATR image, streaming tool
// output to sink. The tool is invoked with an argument vector, never a shell.
func Create(ctx context.Context, p *project.Project, artifact string, sink processutil.Sink) (Result, error) {
	cfg := p.Config.ATR
	tool, err := FindTool(cfg.Tool)
	if err != nil {
		return Result{}, err
	}
	output, err := p.Resolve(cfg.Output)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return Result{}, err
	}
	extras := make([]string, 0, len(cfg.Files))
	for _, rel := range cfg.Files {
		value, resolveErr := p.Resolve(rel)
		if resolveErr != nil {
			return Result{}, resolveErr
		}
		if filepath.Clean(value) == filepath.Clean(artifact) {
			return Result{}, fmt.Errorf("ATR extra file %q duplicates compiler output", rel)
		}
		extras = append(extras, value)
	}
	args := Arguments(cfg, output, artifact, extras)
	result := Result{Image: output, Argv: append([]string{tool}, args...)}
	cmd := processutil.Command(ctx, tool, args...)
	cmd.Dir = p.Root
	start := time.Now()
	text, err := processutil.Run(cmd, sink)
	result.Output = text
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return result, fmt.Errorf("mkatr-go exited with code %d", exit.ExitCode())
		}
		return result, err
	}
	if _, err = os.Stat(output); err != nil {
		return result, fmt.Errorf("mkatr-go did not create the image: %w", err)
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		result.Output += "\n"
	}
	summary := fmt.Sprintf("ATR packaging completed in %d ms.\n", time.Since(start).Milliseconds())
	result.Output += summary
	if sink != nil {
		sink("stdout", summary)
	}
	return result, nil
}

// FindTool locates the separately built mkatr-go executable: the bundled
// copy for the default name, otherwise the configured path or PATH entry.
func FindTool(configured string) (string, error) {
	configured = strings.TrimSpace(configured)
	if configured == "" || configured == "mkatr-go.exe" {
		configured = "mkatr-go"
	}
	path := tools.Path(configured)
	if tools.IsExplicitPath(path) {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("mkatr-go was not found; expected it with the bundled tools or on PATH (configured as %q)", configured)
}
