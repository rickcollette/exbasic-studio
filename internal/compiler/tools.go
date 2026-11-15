package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ToolPath resolves Studio's bundled command-line tools while still allowing
// an explicit project path or a tool installed on PATH to override them.
func ToolPath(name string) string {
	if name == "" {
		return name
	}
	if filepath.IsAbs(name) || strings.ContainsAny(name, `/\`) {
		return name
	}
	exeName := name
	if runtime.GOOS == "windows" && filepath.Ext(exeName) == "" {
		exeName += ".exe"
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	var candidates []string
	if root := os.Getenv("EXBASIC_STUDIO_TOOLS"); root != "" {
		candidates = append(candidates, filepath.Join(root, exeName))
	}
	if executable, err := os.Executable(); err == nil {
		dir := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Join(dir, exeName),
			filepath.Join(dir, "tools", exeName),
			filepath.Join(dir, "tools", name, exeName),
			filepath.Join(dir, "toolchains", "exbasic", exeName),
		)
	}
	if wd, err := os.Getwd(); err == nil {
		root := sourceRoot(wd)
		candidates = append(candidates,
			filepath.Join(root, "bin", exeName),
			filepath.Join(root, "third_party", "exbasic", platform, "compiler", exeName),
			filepath.Join(root, "third_party", "atari800", "bin", platform, exeName),
		)
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	return name
}

// AssetPath resolves a runtime data file shipped beside the application or
// from the source tree during development.
func AssetPath(name string) string {
	var candidates []string
	if executable, err := os.Executable(); err == nil {
		dir := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Join(dir, "tools", name),
			filepath.Join(dir, name),
		)
	}
	if wd, err := os.Getwd(); err == nil {
		root := sourceRoot(wd)
		candidates = append(candidates, filepath.Join(root, "third_party", name))
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return name
}

func sourceRoot(start string) string {
	dir := start
	if strings.EqualFold(filepath.Base(dir), "bin") {
		dir = filepath.Dir(dir)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return start
		}
		dir = parent
	}
}
