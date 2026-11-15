// Package diskimage invokes the separately licensed mkatr-go tool.
package diskimage

import (
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
)

type Result struct {
	Image   string
	Command string
	Output  string
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

func Create(p *project.Project, artifact string) (Result, error) {
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
	cmd := exec.Command(tool, args...)
	processutil.Background(cmd)
	cmd.Dir = p.Root
	start := time.Now()
	raw, err := cmd.CombinedOutput()
	command := commandLine(tool, args)
	text := string(raw)
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return Result{Image: output, Command: command, Output: text}, fmt.Errorf("mkatr-go exited with code %d", exit.ExitCode())
		}
		return Result{Image: output, Command: command, Output: text}, err
	}
	if _, err = os.Stat(output); err != nil {
		return Result{Image: output, Command: command, Output: text}, fmt.Errorf("mkatr-go did not create the image: %w", err)
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += fmt.Sprintf("ATR packaging completed in %d ms.\n", time.Since(start).Milliseconds())
	return Result{Image: output, Command: command, Output: text}, nil
}

// FindTool locates the separately built mkatr-go executable.
func FindTool(configured string) (string, error) {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		configured = "mkatr-go"
	}
	if configured == "mkatr-go" || configured == "mkatr-go.exe" {
		if executable, err := os.Executable(); err == nil {
			candidate := filepath.Join(filepath.Dir(executable), "mkatr-go")
			if filepath.Separator == '\\' {
				candidate += ".exe"
			}
			if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
				return candidate, nil
			}
		}
	}
	if tool, err := exec.LookPath(configured); err == nil {
		return tool, nil
	}
	return "", fmt.Errorf("mkatr-go was not found; expected it beside exBASIC Studio or on PATH (configured as %q)", configured)
}

func commandLine(tool string, args []string) string {
	parts := []string{strconv.Quote(tool)}
	for _, arg := range args {
		parts = append(parts, strconv.Quote(arg))
	}
	return strings.Join(parts, " ")
}
