// Package tools locates the separately licensed command-line programs and
// data files that ship with exBASIC Studio: the exBASIC compiler
// distribution, the ex800 helper, mkatr-go and the EOS cartridge.
//
// Packaged builds keep them in the application's resource directory. The
// EXBASIC_STUDIO_TOOLS environment variable is an explicit override for
// development and tests; there is no implicit source-tree lookup.
package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// OverrideEnv names the development/test override for the tool directory.
const OverrideEnv = "EXBASIC_STUDIO_TOOLS"

// executablePath is replaceable in tests.
var executablePath = os.Executable

// resourceCandidates lists where a packaged application keeps its tools,
// relative to the directory holding the main executable.
func resourceCandidates(exeDir string) []string {
	return []string{
		// Windows installer and portable ZIP, and plain Linux archives.
		filepath.Join(exeDir, "tools"),
		// macOS bundle: Contents/MacOS/<exe> -> Contents/Resources/tools.
		filepath.Join(exeDir, "..", "Resources", "tools"),
		// Linux packages and AppImage: usr/bin/<exe> -> usr/lib/exbasic-studio/tools.
		filepath.Join(exeDir, "..", "lib", "exbasic-studio", "tools"),
	}
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// Root returns the directory holding the bundled tools.
func Root() (string, bool) {
	if override := strings.TrimSpace(os.Getenv(OverrideEnv)); override != "" {
		if abs, err := filepath.Abs(override); err == nil && isDir(abs) {
			return abs, true
		}
		return "", false
	}
	executable, err := executablePath()
	if err != nil {
		return "", false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		executable = resolved
	}
	for _, candidate := range resourceCandidates(filepath.Dir(executable)) {
		if isDir(candidate) {
			return filepath.Clean(candidate), true
		}
	}
	return "", false
}

func executableName(name string) string {
	if runtime.GOOS == "windows" && filepath.Ext(name) == "" {
		return name + ".exe"
	}
	return name
}

// Bundled returns the path of a tool shipped with Studio. A tool lives
// either directly in the tool root or in a directory named after it (the
// compiler distribution is tools/exbasic/exbasic).
func Bundled(name string) (string, bool) {
	root, ok := Root()
	if !ok {
		return "", false
	}
	base := strings.TrimSuffix(name, ".exe")
	exe := executableName(base)
	for _, candidate := range []string{filepath.Join(root, exe), filepath.Join(root, base, exe)} {
		if isFile(candidate) {
			return candidate, true
		}
	}
	return "", false
}

// IsExplicitPath reports whether a configured tool is a path rather than a
// bare program name.
func IsExplicitPath(name string) bool {
	return filepath.IsAbs(name) || strings.ContainsAny(name, `/\`)
}

// Path resolves a configured tool: an explicit path is used as written, a
// bare name prefers the bundled copy and then PATH. An unresolved name is
// returned unchanged so the eventual error names what was configured.
func Path(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || IsExplicitPath(name) {
		return name
	}
	if bundled, ok := Bundled(name); ok {
		return bundled
	}
	if found, err := exec.LookPath(name); err == nil {
		return found
	}
	return name
}

// Asset resolves a bundled data file such as EOS.car.
func Asset(name string) (string, bool) {
	root, ok := Root()
	if !ok {
		return "", false
	}
	candidate := filepath.Join(root, name)
	return candidate, isFile(candidate)
}
