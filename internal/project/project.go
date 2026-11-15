package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const ConfigName = "exbasic.project.json"
const MaxSourceBytes int64 = 2 << 20
const MaxProjectFiles = 4000

var namePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

type Config struct {
	Name         string    `json:"name"`
	Entry        string    `json:"entry"`
	Target       string    `json:"target"`
	Output       string    `json:"output"`
	Compiler     string    `json:"compiler"`
	Emulator     string    `json:"emulator"`
	Native       string    `json:"native"`
	Debug        bool      `json:"debug"`
	IncludePaths []string  `json:"includePaths"`
	Assembly     []string  `json:"assembly"`
	ATR          ATRConfig `json:"atr"`
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
	Kind        string   `json:"kind"`
	RemoteHost  string   `json:"remoteHost,omitempty"`
	RemotePath  string   `json:"remotePath,omitempty"`
}

type Project struct {
	Root   string
	Config Config
}

func DefaultConfig(name string) Config {
	return Config{
		Name: name, Entry: "src/main.bas", Target: "atari", Output: "build/" + name + ".xex",
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
	var cfg Config
	if err = json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	NormalizeConfig(&cfg)
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}
	return &Project{Root: root, Config: cfg}, nil
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
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
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
	return nil
}

// Resolve restricts reads and writes to the selected project. Symlinks inside
// the workspace are rejected to avoid following a link outside the root.
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
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("symlink paths are not allowed")
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	return path, nil
}

func (p *Project) SaveConfig() error {
	NormalizeConfig(&p.Config)
	if err := ValidateConfig(p.Config); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p.Config, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(p.Root, ConfigName), append(data, '\n'))
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
	// Surface rename errors directly rather than using an unsafe delete-first fallback.
	return os.Rename(f.Name(), path)
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
	return string(b), err
}
func (p *Project) Save(rel, content string) error {
	if int64(len(content)) > MaxSourceBytes {
		return errors.New("maximum file size is 2 MiB")
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
		if sourceInfo.Mode()&os.ModeSymlink != 0 {
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
		if err = copyImportedPath(sourcePath, targetPath, sourceInfo); err != nil {
			_ = os.RemoveAll(targetPath)
			return err
		}
	}
	return nil
}

func copyImportedPath(source, target string, info os.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 {
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
			if err = copyImportedPath(filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name()), entryInfo); err != nil {
				return err
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("cannot import non-regular file %q", source)
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
	_, copyErr := io.Copy(out, in)
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
			return walkErr
		}
		if d.Type()&os.ModeSymlink != 0 {
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
			return errors.New("project has too many files to list")
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
			return walkErr
		}
		if d.Type()&os.ModeSymlink != 0 {
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
			return errors.New("project has too many directories to list")
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
		return View{Root: p.Root, Config: p.Config, Files: files, Kind: "local"}, err
	}
	directories, err := p.Directories()
	return View{Root: p.Root, Config: p.Config, Files: files, Directories: directories, Kind: "local"}, err
}
