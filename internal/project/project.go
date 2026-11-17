package project

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const ConfigName = "exbasic.project.json"
const MaxSourceBytes int64 = 2 << 20
const MaxProjectFiles = 4000

// maxEntryScan bounds the search for a default entry file in a folder that
// has no project configuration.
const maxEntryScan = 3 * MaxProjectFiles

// ErrTooLarge reports a folder that is too big to be a project.
var ErrTooLarge = fmt.Errorf("this folder holds more than %d files or folders, which is too many for a project; open the project's own folder instead", MaxProjectFiles)

// CurrentSchemaVersion is the exbasic.project.json format written by this
// release. Files without a version are the pre-1.0 alpha format.
const CurrentSchemaVersion = 1

// Import limits keep a drag-and-drop of an unexpectedly large tree from
// filling the workspace or freezing the application.
const (
	MaxImportFiles       = 5000
	MaxImportBytes int64 = 256 << 20
)

var namePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

type Config struct {
	SchemaVersion int       `json:"schemaVersion"`
	Name          string    `json:"name"`
	Entry         string    `json:"entry"`
	Target        string    `json:"target"`
	Output        string    `json:"output"`
	Compiler      string    `json:"compiler"`
	Emulator      string    `json:"emulator"`
	Native        string    `json:"native"`
	Debug         bool      `json:"debug"`
	IncludePaths  []string  `json:"includePaths"`
	Assembly      []string  `json:"assembly"`
	ATR           ATRConfig `json:"atr"`
}

// ATRConfig controls packaging compiler output into an SDFS or EOSFS ATR.
// The mkatr tool is a separate GPL-2.0 executable built alongside Studio.
type ATRConfig struct {
	Enabled     bool     `json:"enabled"`
	Tool        string   `json:"tool"`
	Filesystem  string   `json:"filesystem"`
	Output      string   `json:"output"`
	Boot        bool     `json:"boot"`
	ExactSize   bool     `json:"exactSize"`
	MinimumSize int      `json:"minimumSize"`
	BootPage    int      `json:"bootPage"`
	Files       []string `json:"files"`
}

type File struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}
type View struct {
	Root        string   `json:"root"`
	Config      Config   `json:"config"`
	Files       []File   `json:"files"`
	Directories []string `json:"directories"`
	// HasConfig is false for a plain folder opened without project metadata.
	HasConfig bool `json:"hasConfig"`
}

type Project struct {
	Root   string
	Config Config
	// HasConfig reports whether exbasic.project.json exists on disk.
	HasConfig bool
}

func DefaultConfig(name string) Config {
	return Config{
		SchemaVersion: CurrentSchemaVersion,
		Name:          name, Entry: "src/main.bas", Target: "atari", Output: "build/" + name + ".xex",
		Compiler: "exbasic", Emulator: "atari800-studio", Native: "auto", Debug: true,
		IncludePaths: []string{"src", "lib"}, Assembly: []string{},
		ATR: ATRConfig{Tool: "mkatr-go", Filesystem: "sparta", Output: "build/" + name + ".atr", Boot: true, BootPage: 7, Files: []string{}},
	}
}

// NormalizeConfig supplies fields introduced after the original project
// format, keeping existing exbasic.project.json files backward compatible.
func NormalizeConfig(c *Config) {
	legacyATR := c.ATR.Tool == "" && c.ATR.Filesystem == "" && c.ATR.Output == "" && c.ATR.BootPage == 0 && c.ATR.Files == nil
	if c.ATR.Tool == "" {
		c.ATR.Tool = "mkatr-go"
	}
	if c.ATR.Filesystem == "" {
		c.ATR.Filesystem = "sparta"
	}
	if c.ATR.Output == "" {
		c.ATR.Output = "build/" + c.Name + ".atr"
	}
	if c.ATR.BootPage == 0 {
		c.ATR.BootPage = 7
	}
	if c.ATR.Files == nil {
		c.ATR.Files = []string{}
	}
	if legacyATR {
		c.ATR.Boot = true
	}
}

func Create(parent, name string) (*Project, error) {
	if !namePattern.MatchString(name) {
		return nil, errors.New("project name must begin with a letter and contain only letters, numbers, _ or -")
	}
	if strings.TrimSpace(parent) == "" {
		return nil, errors.New("choose a parent directory")
	}
	base, err := filepath.Abs(parent)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(base)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("parent directory is unavailable: %s", base)
	}
	root := filepath.Join(base, name)
	if err := os.Mkdir(root, 0755); err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	p := &Project{Root: root, Config: DefaultConfig(name)}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(root, "lib"), 0755); err != nil {
		return nil, err
	}
	sample := "' " + name + " - exBASIC project\nGRAPHICS 0\nPRINT \"HELLO FROM exBASIC STUDIO!\"\n"
	if err := os.WriteFile(filepath.Join(root, "src", "main.bas"), []byte(sample), 0644); err != nil {
		return nil, err
	}
	if err := p.SaveConfig(); err != nil {
		return nil, err
	}
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte("build/\n*.o\n*.lst\n*.lbl\n"), 0644)
	return p, nil
}

func Open(path string) (*Project, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("empty project path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	root, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, err
	}
	stat, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, errors.New("path is not a directory")
	}
	raw, err := os.ReadFile(filepath.Join(root, ConfigName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Project{Root: root, Config: defaultWorkspaceConfig(root)}, nil
		}
		return nil, fmt.Errorf("read exBASIC Studio project configuration: %w", err)
	}
	cfg, err := ParseConfig(raw)
	if err != nil {
		return nil, err
	}
	return &Project{Root: root, Config: cfg, HasConfig: true}, nil
}

// ParseConfig decodes and migrates a project configuration in memory. The
// migrated form reaches disk only when the user saves settings.
func ParseConfig(raw []byte) (Config, error) {
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", ConfigName, err)
	}
	switch {
	case cfg.SchemaVersion < 0:
		return Config{}, fmt.Errorf("%s has an invalid schemaVersion", ConfigName)
	case cfg.SchemaVersion > CurrentSchemaVersion:
		return Config{}, fmt.Errorf("%s uses schema version %d; this exBASIC Studio supports up to %d", ConfigName, cfg.SchemaVersion, CurrentSchemaVersion)
	case cfg.SchemaVersion == 0:
		// Alpha projects carried no version. Their fields are a subset of
		// version 1, so normalization below is the whole migration.
		cfg.SchemaVersion = CurrentSchemaVersion
	}
	NormalizeConfig(&cfg)
	if err := ValidateConfig(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// defaultWorkspaceConfig lets Open Folder behave like an editor workspace.
// It is intentionally kept in memory; opening an ordinary folder must not
// create project metadata behind the user's back.
func defaultWorkspaceConfig(root string) Config {
	name := strings.TrimSpace(filepath.Base(root))
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "workspace"
	}
	cfg := DefaultConfig(name)
	candidates := make([]string, 0, 8)
	// The scan is bounded: a folder that is not a project (a home directory,
	// a whole drive) must not stall Open while it is walked.
	visited := 0
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if visited++; visited > maxEntryScan {
			return filepath.SkipAll
		}
		if entry.Type()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "build" || entry.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".bas") {
			if relative, err := filepath.Rel(root, path); err == nil {
				candidates = append(candidates, filepath.ToSlash(relative))
			}
		}
		return nil
	})
	sort.Slice(candidates, func(i, j int) bool {
		left, right := strings.ToLower(candidates[i]), strings.ToLower(candidates[j])
		leftRank, rightRank := workspaceEntryRank(left), workspaceEntryRank(right)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return left < right
	})
	if len(candidates) > 0 {
		cfg.Entry = candidates[0]
	}
	return cfg
}

func workspaceEntryRank(path string) int {
	switch path {
	case "src/main.bas":
		return 0
	case "main.bas":
		return 1
	default:
		return 2
	}
}

func ValidateConfig(c Config) error {
	NormalizeConfig(&c)
	if c.Name == "" {
		return errors.New("missing name")
	}
	if c.Target != "atari" && c.Target != "atari-cart" {
		return errors.New("unsupported target")
	}
	switch c.Native {
	case "auto", "hot", "all", "none":
	default:
		return errors.New("unsupported native mode")
	}
	for _, s := range append(append([]string{c.Entry, c.Output}, c.IncludePaths...), c.Assembly...) {
		if err := ValidRel(s); err != nil {
			return fmt.Errorf("invalid project path %q: %w", s, err)
		}
	}
	if c.Compiler == "" || c.Emulator == "" {
		return errors.New("compiler/emulator must be nonempty")
	}
	if c.ATR.Enabled {
		if c.Target != "atari" {
			return errors.New("ATR packaging requires the Atari XEX target")
		}
		if strings.TrimSpace(c.ATR.Tool) == "" {
			return errors.New("mkatr tool must be nonempty")
		}
		if c.ATR.Filesystem != "sparta" && c.ATR.Filesystem != "eos" {
			return errors.New("ATR filesystem must be sparta or eos")
		}
		if c.ATR.Filesystem == "eos" && c.ATR.Boot {
			return errors.New("EOSFS images cannot use the SDFS bootloader; disable ATR boot and mount the image under EOS")
		}
		if err := ValidRel(c.ATR.Output); err != nil {
			return fmt.Errorf("invalid ATR output path: %w", err)
		}
		if !strings.EqualFold(filepath.Ext(c.ATR.Output), ".atr") {
			return errors.New("ATR output must use the .atr extension")
		}
		if c.ATR.MinimumSize < 0 || c.ATR.MinimumSize > 16776576 {
			return errors.New("ATR minimum size must be between 0 and 16776576 bytes")
		}
		if c.ATR.BootPage <= 3 || c.ATR.BootPage >= 0xf0 {
			return errors.New("ATR boot page must be from 4 to 239")
		}
		for _, file := range c.ATR.Files {
			if err := ValidRel(file); err != nil {
				return fmt.Errorf("invalid ATR file %q: %w", file, err)
			}
		}
	}
	return nil
}

func ValidRel(rel string) error {
	if rel == "" || filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" {
		return errors.New("expected nonempty relative path")
	}
	normalized := strings.ReplaceAll(rel, "\\", "/")
	if strings.HasPrefix(normalized, "/") {
		return errors.New("absolute paths are forbidden")
	}
	for _, seg := range strings.Split(normalized, "/") {
		if seg == ".." {
			return errors.New("path traversal forbidden")
		}
	}
	if strings.ContainsRune(rel, 0) {
		return errors.New("NUL in path")
	}
	// On Windows a colon names an NTFS alternate data stream or a drive.
	if runtime.GOOS == "windows" && strings.ContainsRune(rel, ':') {
		return errors.New("':' is not allowed in workspace paths")
	}
	return nil
}

// Resolve restricts reads and writes to the selected project. Symbolic links
// and Windows reparse points (junctions, mount points) are rejected at every
// path component so no operation can be redirected outside the root.
func (p *Project) Resolve(rel string) (string, error) {
	if err := ValidRel(rel); err != nil {
		return "", err
	}
	path := filepath.Join(p.Root, filepath.FromSlash(rel))
	r, err := filepath.Rel(p.Root, path)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(os.PathSeparator)) {
		return "", errors.New("path outside project")
	}
	current := p.Root
	for _, part := range strings.Split(r, string(os.PathSeparator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err == nil && isLink(info) {
			return "", errors.New("symbolic links and reparse points are not allowed in workspace paths")
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	return path, nil
}

func (p *Project) SaveConfig() error {
	p.Config.SchemaVersion = CurrentSchemaVersion
	NormalizeConfig(&p.Config)
	if err := ValidateConfig(p.Config); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p.Config, "", "  ")
	if err != nil {
		return err
	}
	if err = atomicWrite(filepath.Join(p.Root, ConfigName), append(data, '\n')); err != nil {
		return err
	}
	p.HasConfig = true
	return nil
}
func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".exbasic-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(0644); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	// Surface rename errors directly rather than using an unsafe delete-first
	// fallback. On Windows a virus scanner or indexer that has the target open
	// for a moment makes the replace fail with a sharing error, so it is
	// retried briefly.
	for attempt := 0; ; attempt++ {
		err = os.Rename(f.Name(), path)
		if err == nil || attempt >= 9 || !transientReplaceError(err) {
			return err
		}
		time.Sleep(time.Duration(20*(attempt+1)) * time.Millisecond)
	}
}

func (p *Project) Read(rel string) (string, error) {
	path, err := p.Resolve(rel)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxSourceBytes {
		return "", errors.New("file is not a regular editable text file or exceeds 2 MiB")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !IsText(b) {
		return "", errors.New("file is not UTF-8 text and cannot be edited")
	}
	return string(b), nil
}

// IsText reports whether data is valid UTF-8 without NUL bytes.
func IsText(data []byte) bool {
	return utf8.Valid(data) && bytes.IndexByte(data, 0) < 0
}

func (p *Project) Save(rel, content string) error {
	if int64(len(content)) > MaxSourceBytes {
		return errors.New("maximum file size is 2 MiB")
	}
	if !IsText([]byte(content)) {
		return errors.New("content is not valid UTF-8 text")
	}
	path, err := p.Resolve(rel)
	if err != nil {
		return err
	}
	if path == filepath.Join(p.Root, ConfigName) {
		return errors.New("use project settings to update the configuration")
	}
	return atomicWrite(path, []byte(content))
}

// CreateDirectory creates a folder inside the project workspace.
func (p *Project) CreateDirectory(rel string) error {
	path, err := p.Resolve(rel)
	if err != nil {
		return err
	}
	return os.MkdirAll(path, 0755)
}

// ImportPaths copies desktop files or folders into a workspace directory.
// Imported trees may not contain symlinks, and existing workspace entries are
// never overwritten implicitly by a drag and drop operation.
func (p *Project) ImportPaths(destination string, sources []string) error {
	if len(sources) == 0 {
		return errors.New("drop one or more files or folders")
	}
	if strings.TrimSpace(destination) == "" {
		destination = "."
	}
	destinationPath, err := p.Resolve(filepath.ToSlash(destination))
	if err != nil {
		return err
	}
	info, err := os.Stat(destinationPath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("drop destination is not a folder")
	}

	budget := &importBudget{files: MaxImportFiles, bytes: MaxImportBytes}
	for _, source := range sources {
		source = filepath.Clean(strings.TrimSpace(source))
		if source == "." || source == "" {
			return errors.New("invalid dropped path")
		}
		sourcePath, err := filepath.Abs(source)
		if err != nil {
			return err
		}
		sourceInfo, err := os.Lstat(sourcePath)
		if err != nil {
			return err
		}
		if isLink(sourceInfo) {
			return fmt.Errorf("cannot import symbolic link %q", source)
		}
		targetPath := filepath.Join(destinationPath, filepath.Base(sourcePath))
		if _, err = os.Lstat(targetPath); err == nil {
			return fmt.Errorf("%q already exists in the destination", filepath.Base(sourcePath))
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if sourceInfo.IsDir() {
			rel, relErr := filepath.Rel(sourcePath, targetPath)
			if relErr == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
				return errors.New("cannot copy a folder into itself")
			}
		}
		if err = copyImportedPath(sourcePath, targetPath, sourceInfo, budget); err != nil {
			_ = os.RemoveAll(targetPath)
			return err
		}
	}
	return nil
}

type importBudget struct {
	files int
	bytes int64
}

func copyImportedPath(source, target string, info os.FileInfo, budget *importBudget) error {
	if budget.files--; budget.files < 0 {
		return fmt.Errorf("import exceeds %d files and folders", MaxImportFiles)
	}
	if isLink(info) {
		return fmt.Errorf("cannot import symbolic link %q", source)
	}
	if info.IsDir() {
		if err := os.Mkdir(target, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			entryInfo, err := os.Lstat(filepath.Join(source, entry.Name()))
			if err != nil {
				return err
			}
			if err = copyImportedPath(filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name()), entryInfo, budget); err != nil {
				return err
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("cannot import non-regular file %q", source)
	}
	if budget.bytes -= info.Size(); budget.bytes < 0 {
		return fmt.Errorf("import exceeds %d MiB", MaxImportBytes>>20)
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	// The limit also covers a file that grows while it is being copied.
	written, copyErr := io.Copy(out, io.LimitReader(in, info.Size()+1))
	if copyErr == nil && written > info.Size() {
		copyErr = fmt.Errorf("%q changed while it was being imported", source)
	}
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// Rename moves a file or folder within the project workspace.
func (p *Project) Rename(oldRel, newRel string) error {
	oldPath, err := p.Resolve(oldRel)
	if err != nil {
		return err
	}
	newPath, err := p.Resolve(newRel)
	if err != nil {
		return err
	}
	if filepath.Clean(oldPath) == filepath.Join(p.Root, ConfigName) {
		return errors.New("the project configuration cannot be renamed")
	}
	if _, err := os.Stat(newPath); err == nil {
		return errors.New("the destination already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		return err
	}
	return os.Rename(oldPath, newPath)
}

// Delete removes a file or folder inside the project workspace. The caller is
// responsible for obtaining confirmation from the user before invoking it.
func (p *Project) Delete(rel string) error {
	path, err := p.Resolve(rel)
	if err != nil {
		return err
	}
	if filepath.Clean(path) == filepath.Join(p.Root, ConfigName) {
		return errors.New("the project configuration cannot be deleted")
	}
	if _, err := os.Lstat(path); err != nil {
		return err
	}
	return os.RemoveAll(path)
}
func (p *Project) Files() ([]File, error) {
	files := make([]File, 0, 32)
	err := filepath.WalkDir(p.Root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// An unreadable subfolder is left out; an unreadable root fails.
			if path != p.Root && d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return walkErr
		}
		if d.Type()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return nil
		}
		if d.IsDir() {
			if path != p.Root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "build" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(p.Root, path)
		if err != nil {
			return err
		}
		if len(files) >= MaxProjectFiles {
			return ErrTooLarge
		}
		stat, err := d.Info()
		if err != nil {
			return err
		}
		files = append(files, File{Path: filepath.ToSlash(rel), Size: stat.Size()})
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, err
}

// Directories returns every visible workspace directory, including empty
// directories. Files alone are not enough to build an editor-style explorer.
func (p *Project) Directories() ([]string, error) {
	directories := make([]string, 0, 16)
	err := filepath.WalkDir(p.Root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// An unreadable subfolder is left out; an unreadable root fails.
			if path != p.Root && d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return walkErr
		}
		if d.Type()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return nil
		}
		if !d.IsDir() || path == p.Root {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") || d.Name() == "build" || d.Name() == "node_modules" {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(p.Root, path)
		if err != nil {
			return err
		}
		if len(directories) >= MaxProjectFiles {
			return ErrTooLarge
		}
		directories = append(directories, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(directories)
	return directories, err
}

func (p *Project) View() (View, error) {
	files, err := p.Files()
	if err != nil {
		return View{Root: p.Root, Config: p.Config, Files: files, HasConfig: p.HasConfig}, err
	}
	directories, err := p.Directories()
	return View{Root: p.Root, Config: p.Config, Files: files, Directories: directories, HasConfig: p.HasConfig}, err
}
