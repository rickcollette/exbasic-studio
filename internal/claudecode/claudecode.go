// Package claudecode integrates with the locally installed Claude Code CLI.
package claudecode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"exbasicstudio/internal/manual"
	processutil "exbasicstudio/internal/process"
)

const (
	pathOverride = "EXBASIC_CLAUDE_PATH"
	maxPromptLen = 8_000
	maxCodeLen   = 32_000
)

// Info describes the local Claude Code installation available to the app.
type Info struct {
	Available bool
	Version   string
}

// Result contains Claude's response and the session to resume on the next turn.
type Result struct {
	Text      string
	SessionID string
}

type cliResponse struct {
	IsError   bool   `json:"is_error"`
	Result    string `json:"result"`
	SessionID string `json:"session_id"`
}

// Status reports whether Claude Code can be launched. It never requires an API key.
func Status() Info {
	binary, err := findBinary()
	if err != nil {
		return Info{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "--version")
	processutil.Background(cmd)
	output, err := cmd.Output()
	if err != nil {
		return Info{}
	}
	return Info{Available: true, Version: strings.TrimSpace(string(output))}
}

// Ask sends one turn to Claude Code in the project directory. The mode is an
// explicit user choice: plan is read-only, edit accepts file edits, and auto
// delegates decisions to the local Claude Code permission configuration.
func Ask(projectRoot, prompt, filename, selectedCode, sessionID, mode string) (Result, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return Result{}, errors.New("enter a question for Claude Code")
	}
	if len(prompt) > maxPromptLen {
		return Result{}, errors.New("prompt too long (8 KB max)")
	}
	if len(selectedCode) > maxCodeLen {
		return Result{}, errors.New("selected code too long (32 KB max)")
	}

	binary, err := findBinary()
	if err != nil {
		return Result{}, err
	}

	permissionMode := "plan"
	switch mode {
	case "", "plan":
	case "edit":
		permissionMode = "acceptEdits"
	case "auto":
		permissionMode = "auto"
	default:
		return Result{}, errors.New("unsupported Claude permission mode")
	}

	args := []string{
		"--print",
		"--output-format", "json",
		"--permission-mode", permissionMode,
		"--permission-prompts", "none",
		"--append-system-prompt", systemPrompt(mode),
	}
	if sessionID != "" {
		args = append(args, "--resume", sessionID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	processutil.Background(cmd)
	cmd.Dir = projectRoot
	cmd.Env = os.Environ()
	cmd.Stdin = strings.NewReader(userPrompt(prompt, filename, selectedCode))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Result{}, errors.New("Claude Code timed out after 10 minutes")
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if detail == "" {
			detail = err.Error()
		}
		return Result{}, fmt.Errorf("Claude Code failed: %s", detail)
	}

	var response cliResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		return Result{}, fmt.Errorf("read Claude Code response: %w", err)
	}
	if response.IsError {
		return Result{}, fmt.Errorf("Claude Code: %s", strings.TrimSpace(response.Result))
	}
	if strings.TrimSpace(response.Result) == "" {
		return Result{}, errors.New("Claude Code returned an empty response")
	}
	return Result{Text: response.Result, SessionID: response.SessionID}, nil
}

func findBinary() (string, error) {
	if override := strings.TrimSpace(os.Getenv(pathOverride)); override != "" {
		info, err := os.Stat(override)
		if err != nil || info.IsDir() {
			return "", fmt.Errorf("%s does not point to a Claude Code executable", pathOverride)
		}
		return override, nil
	}

	if binary, err := exec.LookPath("claude"); err == nil {
		return binary, nil
	}

	home, err := os.UserHomeDir()
	if err == nil {
		if binary := findVSCodeBinary(home); binary != "" {
			return binary, nil
		}
	}

	return "", errors.New("Claude Code was not found; install the Claude Code VS Code extension or CLI, or set EXBASIC_CLAUDE_PATH")
}

func findVSCodeBinary(home string) string {
	filename := "claude"
	if filepath.Separator == '\\' {
		filename += ".exe"
	}

	roots := []string{
		filepath.Join(home, ".vscode", "extensions"),
		filepath.Join(home, ".vscode-insiders", "extensions"),
		filepath.Join(home, ".cursor", "extensions"),
		filepath.Join(home, ".windsurf", "extensions"),
	}
	var matches []string
	for _, root := range roots {
		pattern := filepath.Join(root, "anthropic.claude-code-*", "resources", "native-binary", filename)
		found, _ := filepath.Glob(pattern)
		for _, candidate := range found {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				matches = append(matches, candidate)
			}
		}
	}
	if len(matches) == 0 {
		return ""
	}
	sort.Slice(matches, func(i, j int) bool {
		left, _ := os.Stat(matches[i])
		right, _ := os.Stat(matches[j])
		if left.ModTime().Equal(right.ModTime()) {
			return matches[i] > matches[j]
		}
		return left.ModTime().After(right.ModTime())
	})
	return matches[0]
}

func manualContext(prompt string) string {
	manualContext := make([]string, 0, 5)
	seen := map[string]bool{}
	skip := map[string]bool{
		"what": true, "when": true, "where": true, "does": true,
		"with": true, "from": true, "this": true, "that": true,
		"have": true, "please": true, "basic": true, "code": true,
		"about": true, "why": true, "how": true,
	}
	for _, word := range strings.FieldsFunc(strings.ToLower(prompt), func(r rune) bool { return r < 'a' || r > 'z' }) {
		if len(word) < 4 || skip[word] {
			continue
		}
		for _, hit := range manual.Search(word, 2) {
			key := hit.File + hit.Snippet
			if seen[key] {
				continue
			}
			seen[key] = true
			manualContext = append(manualContext, hit.File+": "+hit.Snippet)
			if len(manualContext) >= 5 {
				break
			}
		}
		if len(manualContext) >= 5 {
			break
		}
	}

	return strings.Join(manualContext, "\n")
}

func systemPrompt(modes ...string) string {
	mode := "plan"
	if len(modes) > 0 {
		mode = modes[0]
	}
	base := "You are Claude Code running inside exBASIC Studio. The working directory is the open project. " +
		"Help with exBASIC syntax, compiler errors, project code, and Atari 8-bit hardware. " +
		"exBASIC differs from Atari BASIC and FastBasic. Distinguish documented facts from guesses. "
	if mode == "edit" || mode == "auto" {
		return base + "The user explicitly enabled workspace changes in the IDE. Make focused changes, explain them, and verify them when tools are available."
	}
	return base + "This integration is in plan mode: inspect files when useful, but do not modify the workspace."
}

func userPrompt(prompt, filename, selectedCode string) string {
	var text strings.Builder
	text.WriteString(prompt)
	if context := manualContext(prompt); context != "" {
		text.WriteString("\n\nRelevant bundled exBASIC manual excerpts:\n")
		text.WriteString(context)
	}
	if filename != "" && selectedCode != "" {
		text.WriteString("\n\nExplicitly selected source from ")
		text.WriteString(filename)
		text.WriteString(":\n```basic\n")
		text.WriteString(selectedCode)
		text.WriteString("\n```")
	}
	return text.String()
}
