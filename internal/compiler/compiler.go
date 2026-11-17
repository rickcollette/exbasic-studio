// Package compiler drives the exBASIC compiler as a child process and turns
// its output into structured build results.
package compiler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"exbasicstudio/internal/diskimage"
	processutil "exbasicstudio/internal/process"
	"exbasicstudio/internal/project"
	"exbasicstudio/internal/tools"
)

// Diagnostic is a compiler message positioned in a workspace file. Lines
// and columns are 1-based; an End of zero means "to the end of the token".
type Diagnostic struct {
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	EndLine   int    `json:"endLine"`
	EndColumn int    `json:"endColumn"`
	Severity  string `json:"severity"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

// Artifact is a file produced by a build.
type Artifact struct {
	Kind string `json:"kind"` // executable | disk-image | labels | listing | assembly | metadata
	Path string `json:"path"`
}

// Step records one external command run during a build, as an argument
// vector: nothing is ever joined into a shell command line.
type Step struct {
	Name string   `json:"name"`
	Argv []string `json:"argv"`
}

// Result is the structured outcome of a build.
type Result struct {
	Success     bool         `json:"success"`
	Cancelled   bool         `json:"cancelled"`
	Steps       []Step       `json:"steps"`
	Output      string       `json:"output"`
	Artifacts   []Artifact   `json:"artifacts"`
	Diagnostics []Diagnostic `json:"diagnostics"`
	// MetadataPath locates the compiler metadata document, when produced.
	MetadataPath string `json:"metadataPath"`
	// DebugAvailable is true when the build carries source-level debug data.
	DebugAvailable bool `json:"debugAvailable"`
	// DebugNote explains why source debugging is unavailable.
	DebugNote  string `json:"debugNote"`
	DurationMS int64  `json:"durationMs"`
}

// Artifact returns the path of the first artifact of a kind.
func (r Result) Artifact(kind string) string {
	for _, artifact := range r.Artifacts {
		if artifact.Kind == kind {
			return artifact.Path
		}
	}
	return ""
}

// fallbackPattern matches the human-readable "file:line:column: message"
// lines printed by every exBASIC compiler.
var fallbackPattern = regexp.MustCompile(`(?m)^(.+?):(\d+):(\d+):\s*(.+?)\r?$`)

// fallbackLinePattern matches "file:line: message" (no column).
var fallbackLinePattern = regexp.MustCompile(`(?m)^(.+?\.[A-Za-z0-9]+):(\d+):\s+(.+?)\r?$`)

// ParseFallbackDiagnostics extracts diagnostics from plain compiler output.
// It is used for compilers that cannot write structured metadata.
func ParseFallbackDiagnostics(output string, rewrite func(path string) string) []Diagnostic {
	diagnostics := []Diagnostic{}
	seen := map[string]bool{}
	add := func(path string, line, column int, message string) {
		if rewrite != nil {
			path = rewrite(path)
		}
		key := fmt.Sprintf("%s:%d:%d:%s", path, line, column, message)
		if seen[key] {
			return
		}
		seen[key] = true
		diagnostics = append(diagnostics, Diagnostic{Path: path, Line: line, Column: column, Severity: "error", Message: strings.TrimSpace(message)})
	}
	for _, match := range fallbackPattern.FindAllStringSubmatch(output, -1) {
		line, _ := strconv.Atoi(match[2])
		column, _ := strconv.Atoi(match[3])
		// The compiler reports a 0-based byte offset.
		add(match[1], line, column+1, match[4])
	}
	for _, match := range fallbackLinePattern.FindAllStringSubmatch(output, -1) {
		if fallbackPattern.MatchString(match[0]) {
			continue
		}
		line, _ := strconv.Atoi(match[2])
		add(match[1], line, 1, match[3])
	}
	return diagnostics
}

// diagnosticsFromMetadata converts structured diagnostics to workspace paths.
func diagnosticsFromMetadata(meta *Metadata, rewrite func(path string) string) []Diagnostic {
	out := make([]Diagnostic, 0, len(meta.Diagnostics))
	for _, item := range meta.Diagnostics {
		path := item.Path
		if rewrite != nil {
			path = rewrite(path)
		}
		out = append(out, Diagnostic{
			Path: path, Line: item.Range.Start.Line, Column: item.Range.Start.Column,
			EndLine: item.Range.End.Line, EndColumn: item.Range.End.Column,
			Severity: item.Severity, Code: item.Code, Message: item.Message,
		})
	}
	return out
}

func compilerArgs(p *project.Project, includeNative bool) ([]string, error) {
	cfg := p.Config
	args := []string{"-t:" + cfg.Target}
	if includeNative {
		switch cfg.Native {
		case "hot", "all":
			args = append(args, "-native:"+cfg.Native)
		case "none":
			args = append(args, "-no-native")
		}
		if cfg.Debug {
			args = append(args, "-g", "-keep")
		}
	}
	for _, dir := range cfg.IncludePaths {
		path, err := p.Resolve(dir)
		if err != nil {
			return nil, err
		}
		args = append(args, "-I:"+path)
	}
	return args, nil
}

// MetadataFile is where a build's metadata document is written.
func MetadataFile(output string) string {
	return strings.TrimSuffix(output, filepath.Ext(output)) + ".exbmeta.json"
}

// Limits on the unsaved text of other files that a lint may take into
// account.
const (
	maxLintOverlays     = 32
	maxLintOverlayBytes = 4 << 20
)

// Lint runs the real exBASIC parser against unsaved editor text.
//
// overlays holds the unsaved text of other open files, keyed by
// workspace-relative path, so an edited INCLUDE file is checked as it is on
// screen, not as it was last saved. The text being checked and the overlays
// are written to a private mirror of the workspace layout outside the
// project. The compiler looks for an INCLUDE beside the including file first
// and then in the include paths, so the mirror directories are searched
// before the real ones and every file without unsaved changes is still read
// from the project.
func Lint(ctx context.Context, p *project.Project, rel, content string, overlays map[string]string) ([]Diagnostic, error) {
	source, err := p.Resolve(rel)
	if err != nil {
		return nil, err
	}
	rel = filepath.ToSlash(filepath.Clean(filepath.FromSlash(rel)))
	mirror, err := os.MkdirTemp("", "exbasic-lint-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(mirror)
	if resolved, resolveErr := filepath.EvalSymlinks(mirror); resolveErr == nil {
		mirror = resolved
	}
	place := func(name, text string) (string, error) {
		target := filepath.Join(mirror, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return "", err
		}
		return target, os.WriteFile(target, []byte(text), 0o600)
	}
	tempName, err := place(rel, content)
	if err != nil {
		return nil, err
	}
	// Real directories that hold a file with unsaved changes: files beside
	// it that are unchanged must still be found.
	realDirs := []string{filepath.Dir(source)}
	budget, count := maxLintOverlayBytes, 0
	for name, text := range overlays {
		if project.ValidRel(name) != nil {
			continue
		}
		clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(name)))
		if clean == rel || count >= maxLintOverlays || len(text) > budget {
			continue
		}
		real, resolveErr := p.Resolve(clean)
		if resolveErr != nil {
			continue
		}
		if _, err = place(clean, text); err != nil {
			return nil, err
		}
		count++
		budget -= len(text)
		realDirs = append(realDirs, filepath.Dir(real))
	}
	base := strings.TrimSuffix(tempName, filepath.Ext(tempName))
	asmName, metaName := base+".lint.asm", base+".lint.exbmeta.json"

	projectArgs, err := compilerArgs(p, false)
	if err != nil {
		return nil, err
	}
	// Include paths: the mirror of each project include directory first,
	// then the project's own, then the real directories noted above.
	args := make([]string, 0, len(projectArgs)*2+len(realDirs)+6)
	var includeArgs []string
	for _, arg := range projectArgs {
		dir, isInclude := strings.CutPrefix(arg, "-I:")
		if !isInclude {
			args = append(args, arg)
			continue
		}
		if inside, relErr := filepath.Rel(p.Root, dir); relErr == nil && filepath.IsLocal(inside) {
			if mirrored := filepath.Join(mirror, inside); isDirectory(mirrored) {
				args = append(args, "-I:"+mirrored)
			}
		}
		includeArgs = append(includeArgs, arg)
	}
	args = append(args, includeArgs...)
	seen := map[string]bool{}
	for _, dir := range realDirs {
		if !seen[dir] {
			seen[dir] = true
			args = append(args, "-I:"+dir)
		}
	}
	compilerPath := tools.Path(p.Config.Compiler)
	caps := DetectCapabilities(ctx, compilerPath)
	args = append(args, "-c")
	if caps.Has(FeatureMetadata) {
		args = append(args, "--metadata-json", metaName)
	}
	args = append(args, "-o", asmName, tempName)
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := processutil.Command(ctx, compilerPath, args...)
	cmd.Dir = p.Root
	output, runErr := processutil.Run(cmd, nil)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, errors.New("exBASIC lint timed out")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	var exit interface{ ExitCode() int }
	if runErr != nil && !errors.As(runErr, &exit) {
		return nil, fmt.Errorf("run exBASIC compiler: %w", runErr)
	}
	// Messages about a mirrored copy belong to the workspace file it stands
	// for.
	rewrite := func(path string) string {
		candidate := filepath.FromSlash(path)
		if filepath.IsAbs(candidate) {
			if inside, relErr := filepath.Rel(mirror, candidate); relErr == nil && filepath.IsLocal(inside) {
				return filepath.ToSlash(inside)
			}
		}
		if resolved := ResolveSource(p.Root, path); resolved != "" {
			return resolved
		}
		return filepath.ToSlash(path)
	}
	if caps.Has(FeatureMetadata) {
		if meta, metaErr := LoadMetadata(metaName); metaErr == nil {
			return diagnosticsFromMetadata(meta, rewrite), nil
		}
	}
	return ParseFallbackDiagnostics(output, rewrite), nil
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// Build compiles the project, streaming compiler output to sink. A failed
// compilation is a Result (so the UI can show diagnostics), not an error.
func Build(ctx context.Context, p *project.Project, sink processutil.Sink) (Result, error) {
	cfg := p.Config
	result := Result{Steps: []Step{}, Artifacts: []Artifact{}, Diagnostics: []Diagnostic{}}
	if err := project.ValidateConfig(cfg); err != nil {
		return result, err
	}
	entry, err := p.Resolve(cfg.Entry)
	if err != nil {
		return result, err
	}
	output, err := p.Resolve(cfg.Output)
	if err != nil {
		return result, err
	}
	if err = os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return result, err
	}
	args, err := compilerArgs(p, true)
	if err != nil {
		return result, err
	}
	compilerPath := tools.Path(cfg.Compiler)
	caps := DetectCapabilities(ctx, compilerPath)
	metadataPath := MetadataFile(output)
	_ = os.Remove(metadataPath)
	if caps.Has(FeatureMetadata) {
		args = append(args, "--metadata-json", metadataPath)
	}
	// exBASIC -o applies to the immediately following BASIC input file.
	args = append(args, "-o", output, entry)
	for _, file := range cfg.Assembly {
		path, resolveErr := p.Resolve(file)
		if resolveErr != nil {
			return result, resolveErr
		}
		args = append(args, path)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	start := time.Now()
	result.Steps = append(result.Steps, Step{Name: "compile", Argv: append([]string{compilerPath}, args...)})
	cmd := processutil.Command(ctx, compilerPath, args...)
	cmd.Dir = p.Root
	progress := newProgressFilter(sink)
	text, runErr := processutil.Run(cmd, progress.write)
	progress.flush()
	result.Output = text
	result.DurationMS = time.Since(start).Milliseconds()

	rewrite := func(path string) string {
		if resolved := ResolveSource(p.Root, path); resolved != "" {
			return resolved
		}
		return filepath.ToSlash(path)
	}
	var meta *Metadata
	if caps.Has(FeatureMetadata) {
		if loaded, metaErr := LoadMetadata(metadataPath); metaErr == nil {
			meta = loaded
			result.MetadataPath = metadataPath
			result.Diagnostics = diagnosticsFromMetadata(meta, rewrite)
		} else if runErr == nil {
			result.DebugNote = "The compiler did not produce usable metadata: " + metaErr.Error()
		}
	}
	if meta == nil {
		result.Diagnostics = ParseFallbackDiagnostics(text, rewrite)
		if result.DebugNote == "" {
			result.DebugNote = "This compiler does not provide build metadata, so completion metadata and source-level debugging are unavailable. Builds still work."
		}
	}

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return result, errors.New("compiler timed out after five minutes")
	}
	if ctx.Err() != nil {
		result.Cancelled = true
		return result, nil
	}
	if runErr != nil {
		var exit interface{ ExitCode() int }
		if !errors.As(runErr, &exit) {
			return result, fmt.Errorf("run exBASIC compiler %q: %w", compilerPath, runErr)
		}
		if len(result.Diagnostics) == 0 {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{Path: filepath.ToSlash(cfg.Entry), Line: 1, Column: 1, Severity: "error", Message: "The compiler reported a failure; see the build output."})
		}
		return result, nil
	}
	if _, statErr := os.Stat(output); statErr != nil {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Path: filepath.ToSlash(cfg.Entry), Line: 1, Column: 1, Severity: "error", Message: "The compiler finished without creating " + filepath.ToSlash(cfg.Output)})
		return result, nil
	}
	result.Success = true
	result.Artifacts = append(result.Artifacts, Artifact{Kind: "executable", Path: output})
	base := strings.TrimSuffix(output, filepath.Ext(output))
	for kind, path := range map[string]string{"labels": base + ".lbl", "assembly": base + ".asm", "listing": base + ".lst"} {
		if _, statErr := os.Stat(path); statErr == nil {
			result.Artifacts = append(result.Artifacts, Artifact{Kind: kind, Path: path})
		}
	}
	if meta != nil {
		result.Artifacts = append(result.Artifacts, Artifact{Kind: "metadata", Path: metadataPath})
		switch {
		case meta.Debug == nil || len(meta.Debug.Lines) == 0:
			result.DebugNote = "This build has no debug information. Enable \"Debug symbols\" in the project settings to debug at source level."
		default:
			result.DebugAvailable = true
		}
	}

	if cfg.ATR.Enabled {
		disk, diskErr := diskimage.Create(ctx, p, output, sink)
		result.Steps = append(result.Steps, Step{Name: "package", Argv: disk.Argv})
		result.Output += disk.Output
		result.DurationMS = time.Since(start).Milliseconds()
		if ctx.Err() != nil && !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			result.Success, result.Cancelled = false, true
			return result, nil
		}
		if diskErr != nil {
			result.Success = false
			message := "ATR packaging failed: " + diskErr.Error()
			result.Output += message + "\n"
			if sink != nil {
				sink("stderr", message+"\n")
			}
			result.Diagnostics = append(result.Diagnostics, Diagnostic{Path: project.ConfigName, Line: 1, Column: 1, Severity: "error", Code: "atr-packaging", Message: message})
			return result, nil
		}
		result.Artifacts = append(result.Artifacts, Artifact{Kind: "disk-image", Path: disk.Image})
	}
	sortArtifacts(result.Artifacts)
	return result, nil
}

func sortArtifacts(artifacts []Artifact) {
	order := map[string]int{"executable": 0, "disk-image": 1, "metadata": 2, "labels": 3, "listing": 4, "assembly": 5}
	for i := 1; i < len(artifacts); i++ {
		for j := i; j > 0 && order[artifacts[j].Kind] < order[artifacts[j-1].Kind]; j-- {
			artifacts[j], artifacts[j-1] = artifacts[j-1], artifacts[j]
		}
	}
}

// progressFilter forwards compiler output line by line and drops repeated
// progress lines. The compiler builds a program several ways and keeps the
// smallest, printing the same "BAS compile / ASM assemble / LINK" lines for
// every attempt; once is enough for the reader.
type progressFilter struct {
	sink    processutil.Sink
	pending map[string]string
	seen    map[string]bool
}

func newProgressFilter(sink processutil.Sink) *progressFilter {
	return &progressFilter{sink: sink, pending: map[string]string{}, seen: map[string]bool{}}
}

func (f *progressFilter) write(stream, chunk string) {
	if f.sink == nil {
		return
	}
	text := f.pending[stream] + chunk
	for {
		end := strings.IndexByte(text, '\n')
		if end < 0 {
			break
		}
		f.line(stream, text[:end+1])
		text = text[end+1:]
	}
	f.pending[stream] = text
}

func (f *progressFilter) line(stream, line string) {
	trimmed := strings.TrimSpace(line)
	for _, prefix := range []string{"BAS compile ", "ASM assemble ", "LINK "} {
		if strings.HasPrefix(trimmed, prefix) {
			if f.seen[trimmed] {
				return
			}
			f.seen[trimmed] = true
			break
		}
	}
	f.sink(stream, line)
}

func (f *progressFilter) flush() {
	if f.sink == nil {
		return
	}
	for stream, rest := range f.pending {
		if rest != "" {
			f.line(stream, rest)
		}
		delete(f.pending, stream)
	}
}
