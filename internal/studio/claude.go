package studio

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"exbasicstudio/internal/compiler"
	"exbasicstudio/internal/project"
	"exbasicstudio/internal/terminal"
	"exbasicstudio/internal/tools"
)

// ClaudeStatus tells the UI whether Claude Code can be started.
type ClaudeStatus struct {
	Available bool   `json:"available"`
	Path      string `json:"path,omitempty"`
	// Install is the command that installs Claude Code on this platform.
	Install string `json:"install"`
}

// claudeEnv names an explicit Claude Code executable, for unusual installs.
const claudeEnv = "EXBASIC_STUDIO_CLAUDE"

func claudeInstallCommand() string {
	if runtime.GOOS == "windows" {
		return "irm https://claude.ai/install.ps1 | iex"
	}
	return "curl -fsSL https://claude.ai/install.sh | bash"
}

// findClaude locates the Claude Code CLI. Applications started from a
// desktop launcher often get a shorter PATH than a shell, so the places the
// installers use are checked as well.
func findClaude() string {
	if explicit := strings.TrimSpace(os.Getenv(claudeEnv)); explicit != "" {
		if isFile(explicit) {
			return explicit
		}
		return ""
	}
	if path, err := exec.LookPath("claude"); err == nil {
		return path
	}
	home, _ := os.UserHomeDir()
	var candidates []string
	if runtime.GOOS == "windows" {
		candidates = []string{
			filepath.Join(home, ".local", "bin", "claude.exe"),
			filepath.Join(os.Getenv("APPDATA"), "npm", "claude.cmd"),
		}
	} else {
		candidates = []string{
			filepath.Join(home, ".local", "bin", "claude"),
			filepath.Join(home, ".claude", "local", "claude"),
			"/opt/homebrew/bin/claude",
			"/usr/local/bin/claude",
		}
	}
	for _, candidate := range candidates {
		if home != "" && isFile(candidate) {
			return candidate
		}
	}
	return ""
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// ClaudeStatus reports whether the Claude Code CLI is installed.
func (s *StudioService) ClaudeStatus() ClaudeStatus {
	path := findClaude()
	return ClaudeStatus{Available: path != "", Path: path, Install: claudeInstallCommand()}
}

// ClaudeOpen starts Claude Code in a terminal session in the project folder.
// It is the same interactive Claude Code as in a shell; Studio only tells it
// about exBASIC: where the bundled manual is and how this project builds.
func (s *StudioService) ClaudeOpen(cols, rows int) (terminal.Info, error) {
	p, err := s.current()
	if err != nil {
		return terminal.Info{}, err
	}
	if err = s.requireTrust(p, "start Claude Code in it"); err != nil {
		return terminal.Info{}, err
	}
	claude := findClaude()
	if claude == "" {
		return terminal.Info{}, errors.New("Claude Code is not installed")
	}
	contextFile, manual, err := s.writeClaudeContext(p)
	if err != nil {
		return terminal.Info{}, fmt.Errorf("prepare the exBASIC context for Claude Code: %w", err)
	}
	args := []string{"--add-dir", filepath.Dir(contextFile)}
	if manual != "" {
		args = append(args, "--add-dir", manual)
	}
	args = append(args, "--append-system-prompt",
		"You are working inside exBASIC Studio on an exBASIC program for Atari 8-bit computers. "+
			"Before writing or changing code, read "+contextFile+": it explains the exBASIC language, "+
			"where its manual is and how to build this project.")
	return s.terminals.Open(terminal.Options{Dir: p.Root, Shell: claude, Args: args, Cols: cols, Rows: rows})
}

// writeClaudeContext writes the project briefing into Studio's own state
// folder, never into the project, and returns its path and the manual folder.
func (s *StudioService) writeClaudeContext(p *project.Project) (string, string, error) {
	sum := sha256.Sum256([]byte(p.Root))
	dir := filepath.Join(s.state.Dir(), "claude", hex.EncodeToString(sum[:6]))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", err
	}
	manual := ""
	usage := ""
	if root, ok := tools.Root(); ok {
		if candidate := filepath.Join(root, "exbasic", "manual"); isDirectory(candidate) {
			manual = candidate
		}
		if candidate := filepath.Join(root, "exbasic", "USAGE.md"); isFile(candidate) {
			usage = candidate
		}
	}
	path := filepath.Join(dir, "EXBASIC-STUDIO.md")
	return path, manual, os.WriteFile(path, []byte(claudeContext(p, manual, usage)), 0o600)
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func shellQuote(arg string) string {
	if arg != "" && !strings.ContainsAny(arg, " \t\"'") {
		return arg
	}
	return `"` + strings.ReplaceAll(arg, `"`, `\"`) + `"`
}

func claudeContext(p *project.Project, manual, usage string) string {
	var b strings.Builder
	cfg := p.Config
	b.WriteString("# exBASIC Studio project\n\n")
	b.WriteString("The user is writing an exBASIC program in exBASIC Studio, an IDE for Atari 8-bit computers.\n\n")
	b.WriteString("## The language\n\n")
	b.WriteString("exBASIC is a compiled BASIC for the Atari 8-bit computers. It is not Atari BASIC, Turbo BASIC or ")
	b.WriteString("Microsoft BASIC: check keywords, syntax and limits in the manual instead of assuming them.\n\n")
	if manual != "" {
		fmt.Fprintf(&b, "The manual is in %s. Start with README.md (the table of contents). ", manual)
		b.WriteString("A-keyword-reference.md lists every keyword, B-error-messages.md explains compiler errors, ")
		b.WriteString("C-differences.md lists the differences from Atari BASIC, and 15-sound-and-controllers.md covers ")
		b.WriteString("STICK, STRIG, PADDLE and SOUND.\n")
	} else {
		b.WriteString("The bundled manual was not found; ask the user before relying on a keyword you are unsure of.\n")
	}
	if usage != "" {
		fmt.Fprintf(&b, "Compiler options are described in %s.\n", usage)
	}
	b.WriteString("\n## This project\n\n")
	fmt.Fprintf(&b, "- Folder: %s\n", p.Root)
	fmt.Fprintf(&b, "- Main file: %s\n", cfg.Entry)
	fmt.Fprintf(&b, "- Target: %s\n", cfg.Target)
	fmt.Fprintf(&b, "- Output: %s\n", cfg.Output)
	if len(cfg.IncludePaths) > 0 {
		fmt.Fprintf(&b, "- Include folders: %s\n", strings.Join(cfg.IncludePaths, ", "))
	}
	b.WriteString("- Settings: exbasic.project.json (it may be absent; then the defaults above apply)\n\n")
	b.WriteString("## Building\n\n")
	if compilerPath, args, err := compiler.CommandLine(p); err == nil {
		b.WriteString("Build from the project folder exactly as Studio does:\n\n```\n")
		parts := []string{shellQuote(compilerPath)}
		for _, arg := range args {
			parts = append(parts, shellQuote(arg))
		}
		b.WriteString(strings.Join(parts, " "))
		b.WriteString("\n```\n\n")
		b.WriteString("Compile after every change and fix the errors it reports (they name the file and line).\n")
	} else {
		fmt.Fprintf(&b, "The project settings are not valid yet (%v); the user can fix them in Studio.\n", err)
	}
	b.WriteString("In Studio the user presses F6 to build, Ctrl+F5 to build and run in the Atari800 emulator and F5 to debug. ")
	b.WriteString("You cannot see or control the emulator: ask the user to run the program and tell you what happens.\n\n")
	b.WriteString("## Emulator input\n\n")
	b.WriteString("Keys the user types on the emulator screen reach the Atari as the same characters. F5, F6 and F7 are ")
	b.WriteString("START, SELECT and OPTION; Home is CLEAR, End is HELP, Pause or F8 is BREAK. With the keyboard joystick on, ")
	b.WriteString("Q W E / A D / Z X C, the arrow keys or the numeric keypad move joystick 0 and Space, Numpad 0 or Ctrl fire. ")
	b.WriteString("STICK(0) is 15 centred, 14 up, 13 down, 11 left, 7 right, 10 up-left, 6 up-right, 9 down-left and 5 down-right; ")
	b.WriteString("STRIG(0) is 0 while fire is held.\n")
	return b.String()
}
