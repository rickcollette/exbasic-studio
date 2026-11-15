// Package remote provides OpenSSH-compatible remote workspaces.
package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"exbasicstudio/internal/compiler"
	"exbasicstudio/internal/diskimage"
	gitmodel "exbasicstudio/internal/git"
	processutil "exbasicstudio/internal/process"
	"exbasicstudio/internal/project"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Config is the effective, typed OpenSSH configuration returned by ssh -G.
// Raw retains unsupported/future OpenSSH keys without throwing information away.
type Config struct {
	Alias                 string              `json:"alias"`
	HostName              string              `json:"hostName"`
	User                  string              `json:"user"`
	Port                  int                 `json:"port"`
	ConfigFile            string              `json:"configFile"`
	IdentityFiles         []string            `json:"identityFiles"`
	UserKnownHostsFiles   []string            `json:"userKnownHostsFiles"`
	ProxyJump             string              `json:"proxyJump"`
	ProxyCommand          string              `json:"proxyCommand"`
	IdentityAgent         string              `json:"identityAgent"`
	StrictHostKeyChecking string              `json:"strictHostKeyChecking"`
	ForwardAgent          bool                `json:"forwardAgent"`
	ConnectTimeoutSeconds int                 `json:"connectTimeoutSeconds"`
	ServerAliveInterval   int                 `json:"serverAliveInterval"`
	ServerAliveCountMax   int                 `json:"serverAliveCountMax"`
	HostKeyAlgorithms     []string            `json:"hostKeyAlgorithms"`
	Raw                   map[string][]string `json:"raw"`
}

type Host struct {
	Alias      string `json:"alias"`
	ConfigFile string `json:"configFile"`
}

type Project struct {
	Alias       string
	Root        string
	Config      project.Config
	Resolved    Config
	sshPath     string
	configFile  string
	command     *exec.Cmd
	stdin       io.WriteCloser
	stdout      io.ReadCloser
	client      *sftp.Client
	stderr      bytes.Buffer
	terminalDir string
	mu          sync.Mutex
}

func sshBinary() (string, error) {
	value, err := exec.LookPath("ssh")
	if err != nil {
		return "", errors.New("OpenSSH client was not found in PATH")
	}
	return value, nil
}

func defaultConfigFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ssh", "config")
}

// Resolve asks the installed OpenSSH client to apply its complete configuration
// semantics instead of approximating Host/Match/Include rules in the IDE.
func Resolve(alias, configFile string) (Config, error) {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return Config{}, errors.New("enter an SSH host or alias")
	}
	if strings.HasPrefix(alias, "-") || strings.ContainsAny(alias, "\r\n\x00") {
		return Config{}, errors.New("invalid SSH host or alias")
	}
	binary, err := sshBinary()
	if err != nil {
		return Config{}, err
	}
	configFile = expandPath(strings.TrimSpace(configFile))
	args := []string{"-G"}
	if configFile != "" {
		args = append(args, "-F", configFile)
	}
	args = append(args, alias)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	processutil.Background(cmd)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		return Config{}, fmt.Errorf("resolve SSH configuration: %s", strings.TrimSpace(string(raw)))
	}
	cfg := Config{Alias: alias, Port: 22, ConfigFile: configFile, Raw: map[string][]string{}}
	if cfg.ConfigFile == "" {
		cfg.ConfigFile = defaultConfigFile()
	}
	for _, line := range strings.Split(string(raw), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok {
			continue
		}
		key, value = strings.ToLower(key), strings.TrimSpace(value)
		cfg.Raw[key] = append(cfg.Raw[key], value)
		switch key {
		case "hostname":
			cfg.HostName = value
		case "user":
			cfg.User = value
		case "port":
			cfg.Port, _ = strconv.Atoi(value)
		case "identityfile":
			cfg.IdentityFiles = append(cfg.IdentityFiles, expandPath(value))
		case "userknownhostsfile":
			for _, item := range strings.Fields(value) {
				cfg.UserKnownHostsFiles = append(cfg.UserKnownHostsFiles, expandPath(item))
			}
		case "proxyjump":
			cfg.ProxyJump = value
		case "proxycommand":
			cfg.ProxyCommand = value
		case "identityagent":
			cfg.IdentityAgent = expandPath(value)
		case "stricthostkeychecking":
			cfg.StrictHostKeyChecking = value
		case "forwardagent":
			cfg.ForwardAgent = value == "yes"
		case "connecttimeout":
			cfg.ConnectTimeoutSeconds, _ = strconv.Atoi(value)
		case "serveraliveinterval":
			cfg.ServerAliveInterval, _ = strconv.Atoi(value)
		case "serveralivecountmax":
			cfg.ServerAliveCountMax, _ = strconv.Atoi(value)
		case "hostkeyalgorithms":
			cfg.HostKeyAlgorithms = strings.Split(value, ",")
		}
	}
	if cfg.HostName == "" {
		cfg.HostName = alias
	}
	return cfg, nil
}

func expandPath(value string) string {
	value = os.ExpandEnv(value)
	if value == "~" || strings.HasPrefix(value, "~/") || strings.HasPrefix(value, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			value = filepath.Join(home, strings.TrimLeft(value[1:], `/\`))
		}
	}
	return value
}

// Hosts discovers concrete aliases for the connection picker. OpenSSH remains
// the authority for resolving each alias; wildcard-only stanzas are omitted.
func Hosts(configFile string) ([]Host, error) {
	if configFile == "" {
		configFile = defaultConfigFile()
	}
	configFile = expandPath(configFile)
	seenFiles, aliases := map[string]bool{}, map[string]Host{}
	var read func(string) error
	read = func(filename string) error {
		abs, err := filepath.Abs(expandPath(filename))
		if err != nil || seenFiles[abs] {
			return err
		}
		seenFiles[abs] = true
		data, err := os.ReadFile(abs)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		for _, raw := range strings.Split(string(data), "\n") {
			line := strings.TrimSpace(strings.SplitN(raw, "#", 2)[0])
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			switch strings.ToLower(fields[0]) {
			case "host":
				for _, alias := range fields[1:] {
					if !strings.ContainsAny(alias, "*!?") {
						aliases[alias] = Host{Alias: alias, ConfigFile: abs}
					}
				}
			case "include":
				for _, pattern := range fields[1:] {
					if !filepath.IsAbs(pattern) {
						pattern = filepath.Join(filepath.Dir(abs), pattern)
					}
					matches, _ := filepath.Glob(expandPath(pattern))
					sort.Strings(matches)
					for _, match := range matches {
						_ = read(match)
					}
				}
			}
		}
		return nil
	}
	if err := read(configFile); err != nil {
		return nil, err
	}
	out := make([]Host, 0, len(aliases))
	for _, item := range aliases {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Alias < out[j].Alias })
	return out, nil
}

func Open(alias, root, configFile string) (*Project, error) {
	configFile = expandPath(strings.TrimSpace(configFile))
	resolved, err := Resolve(alias, configFile)
	if err != nil {
		return nil, err
	}
	binary, err := sshBinary()
	if err != nil {
		return nil, err
	}
	args := []string{"-T", "-o", "BatchMode=yes"}
	if configFile != "" {
		args = append(args, "-F", configFile)
	}
	args = append(args, "-s", alias, "sftp")
	cmd := exec.Command(binary, args...)
	processutil.Background(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	client, err := sftp.NewClientPipe(stdout, stdin)
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("open SFTP for %s: %w: %s", alias, err, strings.TrimSpace(stderr.String()))
	}
	if root == "~" || strings.HasPrefix(root, "~/") {
		home, homeErr := client.RealPath(".")
		if homeErr != nil {
			client.Close()
			_ = cmd.Process.Kill()
			return nil, fmt.Errorf("resolve remote home: %w", homeErr)
		}
		root = path.Join(home, strings.TrimPrefix(root, "~/"))
	}
	canonical, err := client.RealPath(root)
	if err != nil {
		client.Close()
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("open remote path %q: %w", root, err)
	}
	p := &Project{Alias: alias, Root: path.Clean(canonical), Resolved: resolved, sshPath: binary, configFile: configFile, command: cmd, stdin: stdin, stdout: stdout, client: client, stderr: stderr, terminalDir: path.Clean(canonical)}
	raw, err := p.readPath(path.Join(p.Root, project.ConfigName))
	if err != nil {
		p.Close()
		return nil, fmt.Errorf("not an exBASIC Studio project on %s: %w", alias, err)
	}
	if err = json.Unmarshal(raw, &p.Config); err != nil {
		p.Close()
		return nil, err
	}
	project.NormalizeConfig(&p.Config)
	if err = project.ValidateConfig(p.Config); err != nil {
		p.Close()
		return nil, err
	}
	return p, nil
}

func (p *Project) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client != nil {
		_ = p.client.Close()
	}
	if p.command != nil && p.command.Process != nil {
		_ = p.command.Process.Kill()
	}
	p.client = nil
}
func (p *Project) Key() string         { return "ssh://" + p.Alias + p.Root }
func (p *Project) DisplayRoot() string { return p.Alias + ":" + p.Root }
func (p *Project) resolve(rel string) (string, error) {
	if err := project.ValidRel(rel); err != nil {
		return "", err
	}
	target := path.Clean(path.Join(p.Root, strings.ReplaceAll(rel, "\\", "/")))
	if target != p.Root && !strings.HasPrefix(target, p.Root+"/") {
		return "", errors.New("path outside remote project")
	}
	return target, nil
}
func (p *Project) readPath(filename string) ([]byte, error) {
	f, err := p.client.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > project.MaxSourceBytes {
		return nil, errors.New("remote file is not editable text or exceeds 2 MiB")
	}
	return io.ReadAll(io.LimitReader(f, project.MaxSourceBytes+1))
}
func (p *Project) Read(rel string) (string, error) {
	filename, err := p.resolve(rel)
	if err != nil {
		return "", err
	}
	b, err := p.readPath(filename)
	return string(b), err
}
func (p *Project) Save(rel, content string) error {
	if int64(len(content)) > project.MaxSourceBytes {
		return errors.New("maximum file size is 2 MiB")
	}
	filename, err := p.resolve(rel)
	if err != nil {
		return err
	}
	if filename == path.Join(p.Root, project.ConfigName) {
		return errors.New("use project settings to update configuration")
	}
	if err = p.client.MkdirAll(path.Dir(filename)); err != nil {
		return err
	}
	f, err := p.client.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	_, werr := io.WriteString(f, content)
	cerr := f.Close()
	if werr != nil {
		return werr
	}
	return cerr
}
func (p *Project) CreateDirectory(rel string) error {
	v, e := p.resolve(rel)
	if e != nil {
		return e
	}
	return p.client.MkdirAll(v)
}
func (p *Project) Rename(oldRel, newRel string) error {
	a, e := p.resolve(oldRel)
	if e != nil {
		return e
	}
	b, e := p.resolve(newRel)
	if e != nil {
		return e
	}
	if a == path.Join(p.Root, project.ConfigName) {
		return errors.New("configuration cannot be renamed")
	}
	if _, e = p.client.Stat(b); e == nil {
		return errors.New("destination already exists")
	}
	if e = p.client.MkdirAll(path.Dir(b)); e != nil {
		return e
	}
	return p.client.Rename(a, b)
}
func (p *Project) Delete(rel string) error {
	v, e := p.resolve(rel)
	if e != nil {
		return e
	}
	if v == path.Join(p.Root, project.ConfigName) {
		return errors.New("configuration cannot be deleted")
	}
	return p.client.RemoveAll(v)
}
func (p *Project) Files() ([]project.File, error) {
	files, _, err := p.workspaceEntries()
	return files, err
}

func (p *Project) workspaceEntries() ([]project.File, []string, error) {
	out := make([]project.File, 0, 64)
	directories := make([]string, 0, 16)
	var walk func(string) error
	walk = func(dir string) error {
		entries, e := p.client.ReadDir(dir)
		if e != nil {
			return e
		}
		for _, entry := range entries {
			name := entry.Name()
			full := path.Join(dir, name)
			if entry.Mode()&os.ModeSymlink != 0 {
				continue
			}
			if entry.IsDir() {
				if strings.HasPrefix(name, ".") || name == "build" || name == "node_modules" {
					continue
				}
				directories = append(directories, strings.TrimPrefix(full, p.Root+"/"))
				if len(directories) >= project.MaxProjectFiles {
					return errors.New("remote project has too many directories")
				}
				if e = walk(full); e != nil {
					return e
				}
				continue
			}
			if !entry.Mode().IsRegular() {
				continue
			}
			rel := strings.TrimPrefix(full, p.Root+"/")
			out = append(out, project.File{Path: rel, Size: entry.Size()})
			if len(out) >= project.MaxProjectFiles {
				return errors.New("remote project has too many files")
			}
		}
		return nil
	}
	e := walk(p.Root)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	sort.Strings(directories)
	return out, directories, e
}
func (p *Project) View() (project.View, error) {
	files, directories, e := p.workspaceEntries()
	return project.View{Root: p.DisplayRoot(), Config: p.Config, Files: files, Directories: directories, Kind: "ssh", RemoteHost: p.Alias, RemotePath: p.Root}, e
}
func (p *Project) SaveConfig() error {
	project.NormalizeConfig(&p.Config)
	if e := project.ValidateConfig(p.Config); e != nil {
		return e
	}
	b, e := json.MarshalIndent(p.Config, "", "  ")
	if e != nil {
		return e
	}
	f, e := p.client.OpenFile(path.Join(p.Root, project.ConfigName), os.O_WRONLY|os.O_TRUNC)
	if e != nil {
		return e
	}
	_, e = f.Write(append(b, '\n'))
	ce := f.Close()
	if e != nil {
		return e
	}
	return ce
}

func shellQuote(v string) string { return "'" + strings.ReplaceAll(v, "'", "'\\''") + "'" }
func (p *Project) commandArgs(script string) []string {
	args := []string{"-T", "-o", "BatchMode=yes"}
	if p.configFile != "" {
		args = append(args, "-F", p.configFile)
	}
	return append(args, p.Alias, "sh", "-lc", shellQuote(script))
}
func (p *Project) Command(ctx context.Context, script string) (string, int, error) {
	cmd := exec.CommandContext(ctx, p.sshPath, p.commandArgs(script)...)
	processutil.Background(cmd)
	raw, e := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return string(raw), -1, ctx.Err()
	}
	if e == nil {
		return string(raw), 0, nil
	}
	var exit *exec.ExitError
	if errors.As(e, &exit) {
		return string(raw), exit.ExitCode(), nil
	}
	return string(raw), -1, e
}
func (p *Project) Terminal(command string) (string, int, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	marker := "__EXBASIC_CWD__="
	script := "cd " + shellQuote(p.terminalDir) + " && { " + command + "; }; exbasic_exit=$?; printf '\\n" + marker + "%s\\n' \"$PWD\"; exit $exbasic_exit"
	out, code, e := p.Command(ctx, script)
	if i := strings.LastIndex(out, marker); i >= 0 {
		cwd := strings.TrimSpace(out[i+len(marker):])
		if line := strings.IndexByte(cwd, '\n'); line >= 0 {
			cwd = cwd[:line]
		}
		if strings.HasPrefix(cwd, "/") {
			p.terminalDir = cwd
		}
		out = strings.TrimRight(out[:i], "\r\n")
	}
	return out, code, p.terminalDir, e
}

func (p *Project) GitStatus() (gitmodel.Status, error) {
	ctx, c := context.WithTimeout(context.Background(), 20*time.Second)
	defer c()
	out, _, e := p.Command(ctx, "cd "+shellQuote(p.Root)+" && git status --porcelain=v1 --branch --untracked-files=normal")
	if e != nil {
		return gitmodel.Status{Available: false, Summary: "Git unavailable on remote host."}, nil
	}
	return gitmodel.ParseStatus(out), nil
}
func (p *Project) Git(args ...string) (string, error) {
	ctx, c := context.WithTimeout(context.Background(), 30*time.Second)
	defer c()
	parts := []string{"cd " + shellQuote(p.Root) + " && git"}
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	out, code, e := p.Command(ctx, strings.Join(parts, " "))
	if e != nil {
		return out, e
	}
	if code != 0 {
		return out, fmt.Errorf("remote git exited with code %d: %s", code, strings.TrimSpace(out))
	}
	return out, nil
}

func (p *Project) Build() (compiler.Result, error) {
	cfg := p.Config
	entry, _ := p.resolve(cfg.Entry)
	output, _ := p.resolve(cfg.Output)
	args := []string{"-t:" + cfg.Target}
	if cfg.Native == "hot" || cfg.Native == "all" {
		args = append(args, "-native:"+cfg.Native)
	} else if cfg.Native == "none" {
		args = append(args, "-no-native")
	}
	if cfg.Debug {
		args = append(args, "-g", "-keep")
	}
	for _, dir := range cfg.IncludePaths {
		v, _ := p.resolve(dir)
		args = append(args, "-I:"+v)
	}
	args = append(args, "-o", output, entry)
	for _, file := range cfg.Assembly {
		v, _ := p.resolve(file)
		args = append(args, v)
	}
	parts := []string{"cd " + shellQuote(p.Root) + " && mkdir -p " + shellQuote(path.Dir(output)) + " && " + shellQuote(cfg.Compiler)}
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	command := strings.Join(parts, " ")
	ctx, c := context.WithTimeout(context.Background(), 2*time.Minute)
	defer c()
	start := time.Now()
	out, code, e := p.Command(ctx, command)
	res := compiler.Result{Success: e == nil && code == 0, Command: command, Output: out, Artifact: p.Alias + ":" + output, DurationMS: time.Since(start).Milliseconds()}
	if e != nil || code != 0 || !cfg.ATR.Enabled {
		return res, e
	}
	atrOutput, resolveErr := p.resolve(cfg.ATR.Output)
	if resolveErr != nil {
		return res, resolveErr
	}
	extras := make([]string, 0, len(cfg.ATR.Files))
	for _, rel := range cfg.ATR.Files {
		value, extraErr := p.resolve(rel)
		if extraErr != nil {
			return res, extraErr
		}
		extras = append(extras, value)
	}
	atrArgs := diskimage.Arguments(cfg.ATR, atrOutput, output, extras)
	atrParts := []string{"cd " + shellQuote(p.Root) + " && mkdir -p " + shellQuote(path.Dir(atrOutput)) + " && " + shellQuote(cfg.ATR.Tool)}
	for _, arg := range atrArgs {
		atrParts = append(atrParts, shellQuote(arg))
	}
	atrCommand := strings.Join(atrParts, " ")
	atrOut, atrCode, atrErr := p.Command(ctx, atrCommand)
	res.Command += "\n" + atrCommand
	res.Output += "\n" + atrOut
	res.DiskImage = p.Alias + ":" + atrOutput
	res.Success = atrErr == nil && atrCode == 0
	if atrErr != nil {
		res.Output += "\nATR packaging failed: " + atrErr.Error() + "\n"
	} else if atrCode != 0 {
		res.Output += fmt.Sprintf("\nATR packaging failed with code %d. Install mkatr-go on the remote host or update the project ATR tool setting.\n", atrCode)
	}
	return res, nil
}

// Fingerprint is exposed for future trust UI and uses x/crypto/ssh's canonical format.
func Fingerprint(key ssh.PublicKey) string { return ssh.FingerprintSHA256(key) }
