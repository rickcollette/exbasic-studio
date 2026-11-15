package compiler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"exbasicstudio/internal/diskimage"
	processutil "exbasicstudio/internal/process"
	"exbasicstudio/internal/project"
)

type Diagnostic struct {
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

var diagnosticPattern = regexp.MustCompile(`(?m)^(.+?):(\d+):(\d+):\s*(.+)$`)

// Lint runs the real exBASIC parser against unsaved editor text. The temporary
// source lives beside the original so relative INCLUDE statements behave just
// as they do during a normal build.
func Lint(p *project.Project, rel, content string) ([]Diagnostic, error) {
	source, err := p.Resolve(rel)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(source)
	temp, err := os.CreateTemp(dir, ".exbasic-lint-*.bas")
	if err != nil {
		return nil, err
	}
	tempName := temp.Name()
	asmName := strings.TrimSuffix(tempName, filepath.Ext(tempName)) + ".asm"
	defer os.Remove(tempName)
	defer os.Remove(asmName)
	if _, err = temp.WriteString(content); err != nil {
		temp.Close()
		return nil, err
	}
	if err = temp.Close(); err != nil {
		return nil, err
	}
	args := []string{"-t:" + p.Config.Target, "-c"}
	for _, include := range p.Config.IncludePaths {
		path, resolveErr := p.Resolve(include)
		if resolveErr != nil {
			return nil, resolveErr
		}
		args = append(args, "-I:"+path)
	}
	args = append(args, "-o", asmName, tempName)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ToolPath(p.Config.Compiler), args...)
	processutil.Background(cmd)
	cmd.Dir = p.Root
	output, _ := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, errors.New("exBASIC lint timed out")
	}
	matches := diagnosticPattern.FindAllStringSubmatch(string(output), -1)
	diagnostics := make([]Diagnostic, 0, len(matches))
	for _, match := range matches {
		line, _ := strconv.Atoi(match[2])
		column, _ := strconv.Atoi(match[3])
		diagnostics = append(diagnostics, Diagnostic{Path: rel, Line: line, Column: column, Severity: "error", Message: strings.TrimSpace(match[4])})
	}
	return diagnostics, nil
}

type Result struct {
	Success    bool   `json:"success"`
	Command    string `json:"command"`
	Output     string `json:"output"`
	Artifact   string `json:"artifact"`
	DiskImage  string `json:"diskImage,omitempty"`
	DurationMS int64  `json:"durationMs"`
}

func Build(p *project.Project) (Result, error) {
	cfg := p.Config
	if err := project.ValidateConfig(cfg); err != nil {
		return Result{}, err
	}
	entry, err := p.Resolve(cfg.Entry)
	if err != nil {
		return Result{}, err
	}
	output, err := p.Resolve(cfg.Output)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return Result{}, err
	}
	args := []string{"-t:" + cfg.Target}
	switch cfg.Native {
	case "hot", "all":
		args = append(args, "-native:"+cfg.Native)
	case "none":
		args = append(args, "-no-native")
	}
	if cfg.Debug {
		args = append(args, "-g", "-keep")
	}
	for _, dir := range cfg.IncludePaths {
		v, err := p.Resolve(dir)
		if err != nil {
			return Result{}, err
		}
		args = append(args, "-I:"+v)
	}
	// exBASIC -o applies to the immediately following BASIC input file.
	args = append(args, "-o", output, entry)
	for _, file := range cfg.Assembly {
		v, err := p.Resolve(file)
		if err != nil {
			return Result{}, err
		}
		args = append(args, v)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	start := time.Now()
	compilerPath := ToolPath(cfg.Compiler)
	cmd := exec.CommandContext(ctx, compilerPath, args...)
	processutil.Background(cmd)
	cmd.Dir = p.Root
	text, cmdErr := cmd.CombinedOutput()
	res := Result{Success: cmdErr == nil, Command: compilerPath + " " + strings.Join(args, " "), Output: string(text), Artifact: output, DurationMS: time.Since(start).Milliseconds()}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return res, fmt.Errorf("compiler timed out after two minutes")
	}
	// A compilation failure is reported as a result so the UI displays diagnostics.
	if cmdErr != nil {
		res.Output += fmt.Sprintf("\nCompiler failed: %v\n", cmdErr)
		return res, nil
	}
	if cfg.ATR.Enabled {
		disk, diskErr := diskimage.Create(p, output)
		res.Command += "\n" + disk.Command
		res.Output += "\n" + disk.Output
		res.DiskImage = disk.Image
		if diskErr != nil {
			res.Success = false
			res.Output += fmt.Sprintf("ATR packaging failed: %v\n", diskErr)
		}
	}
	return res, nil
}

