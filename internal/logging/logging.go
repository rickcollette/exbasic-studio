// Package logging keeps rotating local diagnostic logs and builds a
// path-redacted support bundle on request. Nothing is ever uploaded.
package logging

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// MaxFileBytes is the size at which the active log is rotated.
	MaxFileBytes = 1 << 20
	// MaxFiles is the number of log files kept, including the active one.
	MaxFiles = 5

	activeName = "studio.log"
)

// Logger appends timestamped lines to a size-rotated file.
type Logger struct {
	dir  string
	mu   sync.Mutex
	file *os.File
	size int64
}

// New opens (or creates) the log directory. Logging failures never stop the
// application: a Logger whose file cannot be opened silently drops lines.
func New(dir string) *Logger {
	l := &Logger{dir: dir}
	_ = os.MkdirAll(dir, 0o700)
	l.open()
	return l
}

// Dir returns the directory holding the log files.
func (l *Logger) Dir() string { return l.dir }

func (l *Logger) open() {
	file, err := os.OpenFile(filepath.Join(l.dir, activeName), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		l.file = nil
		return
	}
	l.file = file
	if info, statErr := file.Stat(); statErr == nil {
		l.size = info.Size()
	}
}

// rotate shifts studio.log -> studio.1.log -> ... and drops the oldest.
func (l *Logger) rotate() {
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
	_ = os.Remove(filepath.Join(l.dir, fmt.Sprintf("studio.%d.log", MaxFiles-1)))
	for index := MaxFiles - 2; index >= 1; index-- {
		_ = os.Rename(filepath.Join(l.dir, fmt.Sprintf("studio.%d.log", index)), filepath.Join(l.dir, fmt.Sprintf("studio.%d.log", index+1)))
	}
	_ = os.Rename(filepath.Join(l.dir, activeName), filepath.Join(l.dir, "studio.1.log"))
	l.size = 0
	l.open()
}

// Logf writes one line. Newlines inside the message are escaped so a log
// entry can never forge a following entry.
func (l *Logger) Logf(level, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	message = strings.NewReplacer("\r", `\r`, "\n", `\n`).Replace(message)
	if len(message) > 8192 {
		message = message[:8192] + "…"
	}
	line := fmt.Sprintf("%s %-5s %s\n", time.Now().UTC().Format(time.RFC3339), level, message)
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}
	if l.size+int64(len(line)) > MaxFileBytes {
		l.rotate()
		if l.file == nil {
			return
		}
	}
	if n, err := l.file.WriteString(line); err == nil {
		l.size += int64(n)
	}
}

func (l *Logger) Info(format string, args ...any)  { l.Logf("INFO", format, args...) }
func (l *Logger) Warn(format string, args ...any)  { l.Logf("WARN", format, args...) }
func (l *Logger) Error(format string, args ...any) { l.Logf("ERROR", format, args...) }

// Close flushes and closes the active file.
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
}

// files lists the log files from newest to oldest.
func (l *Logger) files() []string {
	names := []string{activeName}
	for index := 1; index < MaxFiles; index++ {
		names = append(names, fmt.Sprintf("studio.%d.log", index))
	}
	return names
}

// ---- Redaction -------------------------------------------------------------

// Redactor replaces user-identifying path fragments with placeholders.
type Redactor struct {
	replacements []replacement
}

type replacement struct {
	pattern *regexp.Regexp
	with    string
}

// NewRedactor hides the home directory, the user name and each workspace
// root. Longer paths are replaced first so a workspace inside the home
// directory is reported as <workspace>, not <home>/....
func NewRedactor(workspaces ...string) *Redactor {
	type entry struct{ path, with string }
	entries := []entry{}
	for _, workspace := range workspaces {
		if strings.TrimSpace(workspace) != "" {
			entries = append(entries, entry{workspace, "<workspace>"})
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		entries = append(entries, entry{home, "<home>"})
	}
	if temp := os.TempDir(); temp != "" {
		entries = append(entries, entry{temp, "<temp>"})
	}
	sort.SliceStable(entries, func(i, j int) bool { return len(entries[i].path) > len(entries[j].path) })
	r := &Redactor{}
	for _, item := range entries {
		r.replacements = append(r.replacements, replacement{pathPattern(item.path), item.with})
	}
	if current, err := user.Current(); err == nil {
		name := current.Username
		if slash := strings.LastIndexAny(name, `\/`); slash >= 0 {
			name = name[slash+1:]
		}
		if len(name) >= 3 {
			r.replacements = append(r.replacements, replacement{regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(name) + `\b`), "<user>"})
		}
	}
	return r
}

// pathPattern matches a path with either separator and, on case-insensitive
// filesystems, any letter case.
func pathPattern(path string) *regexp.Regexp {
	path = strings.TrimRight(filepath.Clean(path), `\/`)
	parts := strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
	for index, part := range parts {
		parts[index] = regexp.QuoteMeta(part)
	}
	expression := strings.Join(parts, `[\\/]+`)
	if strings.HasPrefix(path, "/") {
		expression = "/" + expression
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		expression = "(?i)" + expression
	}
	return regexp.MustCompile(expression)
}

// Redact applies every replacement.
func (r *Redactor) Redact(text string) string {
	for _, item := range r.replacements {
		text = item.pattern.ReplaceAllString(text, item.with)
	}
	return text
}

// ---- Support bundle --------------------------------------------------------

// BundleInfo is the manifest stored inside a support bundle.
type BundleInfo struct {
	Created  time.Time         `json:"created"`
	Version  string            `json:"version"`
	OS       string            `json:"os"`
	Arch     string            `json:"arch"`
	Details  map[string]string `json:"details"`
	Redacted bool              `json:"redacted"`
}

// WriteBundle writes a zip archive containing the manifest and the redacted
// log files. Project sources are never included.
func (l *Logger) WriteBundle(out io.Writer, version string, details map[string]string, redactor *Redactor) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	archive := zip.NewWriter(out)
	info := BundleInfo{Created: time.Now().UTC(), Version: version, OS: runtime.GOOS, Arch: runtime.GOARCH, Details: map[string]string{}, Redacted: true}
	for key, value := range details {
		info.Details[key] = redactor.Redact(value)
	}
	entry, err := archive.Create("manifest.json")
	if err != nil {
		return err
	}
	// Placeholders such as <workspace> stay readable in the manifest.
	encoder := json.NewEncoder(entry)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(info); err != nil {
		return err
	}
	for _, name := range l.files() {
		raw, readErr := os.ReadFile(filepath.Join(l.dir, name))
		if readErr != nil {
			continue
		}
		if entry, err = archive.Create("logs/" + name); err != nil {
			return err
		}
		if _, err = io.Copy(entry, bytes.NewReader([]byte(redactor.Redact(string(raw))))); err != nil {
			return err
		}
	}
	return archive.Close()
}

// ExportBundle writes the bundle to path.
func (l *Logger) ExportBundle(path, version string, details map[string]string, redactor *Redactor) error {
	var buffer bytes.Buffer
	if err := l.WriteBundle(&buffer, version, details, redactor); err != nil {
		return err
	}
	return os.WriteFile(path, buffer.Bytes(), 0o600)
}
