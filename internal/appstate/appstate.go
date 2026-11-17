// Package appstate persists per-user Studio state outside project
// directories: recent projects, per-workspace editor/debugger state, layout,
// preferences, workspace trust and crash-recovery copies of unsaved files.
package appstate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// SchemaVersion is the current on-disk format of state.json.
const SchemaVersion = 1

// MaxRecentProjects bounds the recent-project list.
const MaxRecentProjects = 20

const (
	maxRecoveryBytes     = 2 << 20
	maxWorkspaceEntries  = 200
	maxListEntries       = 500
	maxPreferenceEntries = 200
)

type RecentProject struct {
	Path     string    `json:"path"`
	Name     string    `json:"name"`
	OpenedAt time.Time `json:"openedAt"`
}

type Breakpoint struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Enabled bool   `json:"enabled"`
}

// Workspace is the state remembered for one project root.
type Workspace struct {
	Tabs            []string     `json:"tabs"`
	ActiveEditor    string       `json:"activeEditor"`
	EmulatorProfile string       `json:"emulatorProfile"`
	Breakpoints     []Breakpoint `json:"breakpoints"`
	Watches         []string     `json:"watches"`
	// KeyboardJoystick remembers whether the keyboard drives joystick 0.
	KeyboardJoystick bool `json:"keyboardJoystick,omitempty"`
}

// Layout stores panel visibility and sizes in CSS pixels.
type Layout struct {
	PrimaryOpen  bool   `json:"primaryOpen"`
	PanelOpen    bool   `json:"panelOpen"`
	Activity     string `json:"activity"`
	PanelTab     string `json:"panelTab"`
	PrimaryWidth int    `json:"primaryWidth"`
	PanelHeight  int    `json:"panelHeight"`
}

type trustRecord struct {
	Fingerprint string    `json:"fingerprint"`
	GrantedAt   time.Time `json:"grantedAt"`
}

// State is the complete versioned document.
type State struct {
	SchemaVersion  int                    `json:"schemaVersion"`
	RecentProjects []RecentProject        `json:"recentProjects"`
	Workspaces     map[string]Workspace   `json:"workspaces"`
	Layout         Layout                 `json:"layout"`
	Preferences    map[string]string      `json:"preferences"`
	Trusted        map[string]trustRecord `json:"trusted"`
}

// Snapshot is the frontend-facing view of the state for one workspace.
type Snapshot struct {
	SchemaVersion  int               `json:"schemaVersion"`
	RecentProjects []RecentProject   `json:"recentProjects"`
	Workspace      Workspace         `json:"workspace"`
	Layout         Layout            `json:"layout"`
	Preferences    map[string]string `json:"preferences"`
	// Notice is set once after a corrupt state file was set aside.
	Notice string `json:"notice,omitempty"`
}

// Recovery describes a crash-recovery copy of an unsaved editor buffer.
type Recovery struct {
	Path    string    `json:"path"`
	Content string    `json:"content"`
	SavedAt time.Time `json:"savedAt"`
}

type recoveryFile struct {
	SchemaVersion int       `json:"schemaVersion"`
	Root          string    `json:"root"`
	Path          string    `json:"path"`
	Content       string    `json:"content"`
	SavedAt       time.Time `json:"savedAt"`
}

// Store serializes all access to the state directory.
type Store struct {
	dir    string
	mu     sync.Mutex
	state  State
	loaded bool
	notice string
}

// DefaultDir is the per-user configuration directory used by Studio.
func DefaultDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "exbasic-studio")
}

func NewStore(dir string) *Store { return &Store{dir: dir} }

// Dir returns the directory holding state, recovery data and logs.
func (s *Store) Dir() string { return s.dir }

func defaultState() State {
	return State{
		SchemaVersion:  SchemaVersion,
		RecentProjects: []RecentProject{},
		Workspaces:     map[string]Workspace{},
		Layout:         Layout{PrimaryOpen: true, PanelOpen: true, Activity: "explorer", PanelTab: "terminal", PrimaryWidth: 280, PanelHeight: 240},
		Preferences:    map[string]string{},
		Trusted:        map[string]trustRecord{},
	}
}

// rootKey normalizes a workspace root so the same directory always maps to
// the same record regardless of spelling.
func rootKey(root string) string {
	key := filepath.Clean(root)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		key = strings.ToLower(key)
	}
	return key
}

func (s *Store) statePath() string { return filepath.Join(s.dir, "state.json") }

// load reads state.json once. A missing file yields defaults; an unreadable
// or newer-format file is set aside so the application can still start and
// the user's data is not silently overwritten.
func (s *Store) load() {
	if s.loaded {
		return
	}
	s.loaded = true
	s.state = defaultState()
	raw, err := os.ReadFile(s.statePath())
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			s.notice = "Saved application state could not be read; defaults are in use."
		}
		return
	}
	var parsed State
	if err = json.Unmarshal(raw, &parsed); err != nil || parsed.SchemaVersion < 1 || parsed.SchemaVersion > SchemaVersion {
		aside := s.statePath() + ".corrupt-" + time.Now().UTC().Format("20060102T150405")
		reason := "was damaged"
		if err == nil && parsed.SchemaVersion > SchemaVersion {
			reason = "was written by a newer exBASIC Studio"
		}
		if renameErr := os.Rename(s.statePath(), aside); renameErr == nil {
			s.notice = fmt.Sprintf("Saved application state %s and was set aside as %s; defaults are in use.", reason, filepath.Base(aside))
		} else {
			s.notice = fmt.Sprintf("Saved application state %s; defaults are in use.", reason)
		}
		return
	}
	normalize(&parsed)
	s.state = parsed
}

func normalize(state *State) {
	defaults := defaultState()
	state.SchemaVersion = SchemaVersion
	if state.RecentProjects == nil {
		state.RecentProjects = defaults.RecentProjects
	}
	if len(state.RecentProjects) > MaxRecentProjects {
		state.RecentProjects = state.RecentProjects[:MaxRecentProjects]
	}
	if state.Workspaces == nil {
		state.Workspaces = defaults.Workspaces
	}
	if state.Preferences == nil {
		state.Preferences = defaults.Preferences
	}
	if state.Trusted == nil {
		state.Trusted = defaults.Trusted
	}
	if state.Layout.Activity == "" {
		state.Layout = defaults.Layout
	}
	if state.Layout.PrimaryWidth < 160 || state.Layout.PrimaryWidth > 900 {
		state.Layout.PrimaryWidth = defaults.Layout.PrimaryWidth
	}
	if state.Layout.PanelHeight < 80 || state.Layout.PanelHeight > 1200 {
		state.Layout.PanelHeight = defaults.Layout.PanelHeight
	}
	for key, workspace := range state.Workspaces {
		state.Workspaces[key] = normalizeWorkspace(workspace)
	}
}

func normalizeWorkspace(workspace Workspace) Workspace {
	if workspace.Tabs == nil {
		workspace.Tabs = []string{}
	}
	if workspace.Breakpoints == nil {
		workspace.Breakpoints = []Breakpoint{}
	}
	if workspace.Watches == nil {
		workspace.Watches = []string{}
	}
	if len(workspace.Tabs) > maxListEntries {
		workspace.Tabs = workspace.Tabs[:maxListEntries]
	}
	if len(workspace.Breakpoints) > maxListEntries {
		workspace.Breakpoints = workspace.Breakpoints[:maxListEntries]
	}
	if len(workspace.Watches) > maxListEntries {
		workspace.Watches = workspace.Watches[:maxListEntries]
	}
	return workspace
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	return writePrivate(s.statePath(), append(data, '\n'))
}

// writePrivate replaces a file atomically with user-only permissions.
func writePrivate(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".state-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if err = temp.Chmod(0o600); err != nil && runtime.GOOS != "windows" {
		temp.Close()
		return err
	}
	if _, err = temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}

// Snapshot returns the state relevant to one workspace root ("" for none).
// The corruption notice, if any, is delivered exactly once.
func (s *Store) Snapshot(root string) Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	snapshot := Snapshot{
		SchemaVersion:  SchemaVersion,
		RecentProjects: append([]RecentProject{}, s.state.RecentProjects...),
		Workspace:      normalizeWorkspace(Workspace{}),
		Layout:         s.state.Layout,
		Preferences:    map[string]string{},
		Notice:         s.notice,
	}
	s.notice = ""
	for key, value := range s.state.Preferences {
		snapshot.Preferences[key] = value
	}
	if root != "" {
		if workspace, ok := s.state.Workspaces[rootKey(root)]; ok {
			snapshot.Workspace = normalizeWorkspace(workspace)
		}
	}
	return snapshot
}

// AddRecent moves a project to the front of the recent list.
func (s *Store) AddRecent(root, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	key := rootKey(root)
	recent := []RecentProject{{Path: root, Name: name, OpenedAt: time.Now().UTC()}}
	for _, item := range s.state.RecentProjects {
		if rootKey(item.Path) != key && len(recent) < MaxRecentProjects {
			recent = append(recent, item)
		}
	}
	s.state.RecentProjects = recent
	return s.save()
}

// RemoveRecent forgets one project, or all of them when root is empty.
func (s *Store) RemoveRecent(root string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	kept := []RecentProject{}
	if root != "" {
		key := rootKey(root)
		for _, item := range s.state.RecentProjects {
			if rootKey(item.Path) != key {
				kept = append(kept, item)
			}
		}
	}
	s.state.RecentProjects = kept
	return s.save()
}

// SaveWorkspace stores editor and debugger state for a project root.
func (s *Store) SaveWorkspace(root string, workspace Workspace) error {
	if strings.TrimSpace(root) == "" {
		return errors.New("no workspace is open")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	key := rootKey(root)
	if _, exists := s.state.Workspaces[key]; !exists && len(s.state.Workspaces) >= maxWorkspaceEntries {
		s.evictWorkspaceLocked()
	}
	s.state.Workspaces[key] = normalizeWorkspace(workspace)
	return s.save()
}

// evictWorkspaceLocked drops a workspace record that is no longer in the
// recent list so the state file cannot grow without bound.
func (s *Store) evictWorkspaceLocked() {
	recent := map[string]bool{}
	for _, item := range s.state.RecentProjects {
		recent[rootKey(item.Path)] = true
	}
	keys := make([]string, 0, len(s.state.Workspaces))
	for key := range s.state.Workspaces {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !recent[key] {
			delete(s.state.Workspaces, key)
			return
		}
	}
	if len(keys) > 0 {
		delete(s.state.Workspaces, keys[0])
	}
}

func (s *Store) SaveLayout(layout Layout) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	s.state.Layout = layout
	normalize(&s.state)
	return s.save()
}

// SetPreference stores one preference; an empty value removes it.
func (s *Store) SetPreference(key, value string) error {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 64 || len(value) > 4096 {
		return errors.New("invalid preference")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	if value == "" {
		delete(s.state.Preferences, key)
	} else {
		if _, exists := s.state.Preferences[key]; !exists && len(s.state.Preferences) >= maxPreferenceEntries {
			return errors.New("too many preferences")
		}
		s.state.Preferences[key] = value
	}
	return s.save()
}

// ---- Workspace trust -------------------------------------------------------

// ToolFingerprint condenses the external programs a workspace is configured
// to launch. Trust is bound to it, so editing any tool path revokes trust.
func ToolFingerprint(tools ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(tools, "\x00")))
	return hex.EncodeToString(sum[:])
}

// IsTrusted reports whether the user trusted this root with these tools.
func (s *Store) IsTrusted(root, fingerprint string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	record, ok := s.state.Trusted[rootKey(root)]
	return ok && record.Fingerprint == fingerprint
}

// Trust records an explicit user decision to trust a workspace.
func (s *Store) Trust(root, fingerprint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	s.state.Trusted[rootKey(root)] = trustRecord{Fingerprint: fingerprint, GrantedAt: time.Now().UTC()}
	return s.save()
}

// Revoke removes trust for a workspace.
func (s *Store) Revoke(root string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	delete(s.state.Trusted, rootKey(root))
	return s.save()
}

// ---- Crash recovery --------------------------------------------------------

func hashName(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:16])
}

func (s *Store) recoveryDir(root string) string {
	return filepath.Join(s.dir, "recovery", hashName(rootKey(root)))
}

func (s *Store) recoveryPath(root, rel string) string {
	return filepath.Join(s.recoveryDir(root), hashName(filepath.ToSlash(rel))+".json")
}

// SaveRecovery stores an unsaved buffer. The copy is private to the user.
func (s *Store) SaveRecovery(root, rel, content string) error {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(rel) == "" {
		return errors.New("recovery needs a workspace file")
	}
	if len(content) > maxRecoveryBytes {
		return errors.New("file is too large for crash recovery")
	}
	data, err := json.Marshal(recoveryFile{SchemaVersion: SchemaVersion, Root: root, Path: filepath.ToSlash(rel), Content: content, SavedAt: time.Now().UTC()})
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return writePrivate(s.recoveryPath(root, rel), data)
}

// ClearRecovery removes the recovery copy of one file (after a successful
// save or an explicit discard). A missing copy is not an error.
func (s *Store) ClearRecovery(root, rel string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(s.recoveryPath(root, rel))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Recoveries lists the recovery copies stored for a workspace, oldest first.
// Unreadable entries are removed rather than offered to the user.
func (s *Store) Recoveries(root string) ([]Recovery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := s.recoveryDir(root)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []Recovery{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Recovery{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, readErr := os.ReadFile(path)
		var file recoveryFile
		if readErr != nil || json.Unmarshal(raw, &file) != nil || file.Path == "" || rootKey(file.Root) != rootKey(root) {
			_ = os.Remove(path)
			continue
		}
		out = append(out, Recovery{Path: file.Path, Content: file.Content, SavedAt: file.SavedAt})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SavedAt.Before(out[j].SavedAt) })
	return out, nil
}

// ClearAllRecovery deletes every recovery copy for every workspace.
func (s *Store) ClearAllRecovery() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.RemoveAll(filepath.Join(s.dir, "recovery"))
}
