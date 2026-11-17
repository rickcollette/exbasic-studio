package compiler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	processutil "exbasicstudio/internal/process"
)

// MetadataSchemaVersion is the compiler metadata document version that this
// release understands.
const MetadataSchemaVersion = 1

// Feature names reported by `exbasic --capabilities=json`.
const (
	FeatureMetadata    = "metadata-json"
	FeatureDiagnostics = "structured-diagnostics"
	FeatureDebug       = "debug-metadata"
)

// Capabilities describes what a compiler executable supports.
type Capabilities struct {
	SchemaVersion int      `json:"schemaVersion"`
	Version       string   `json:"version"`
	Features      []string `json:"features"`
}

// Has reports whether a feature is available.
func (c Capabilities) Has(feature string) bool {
	for _, item := range c.Features {
		if item == feature {
			return true
		}
	}
	return false
}

type capabilityEntry struct {
	modified time.Time
	size     int64
	value    Capabilities
}

var (
	capabilityMu    sync.Mutex
	capabilityCache = map[string]capabilityEntry{}
)

// DetectCapabilities asks a compiler what it supports. A compiler that does
// not understand the question (an older or third-party build) simply has no
// optional features; that is not an error.
func DetectCapabilities(ctx context.Context, compilerPath string) Capabilities {
	info, statErr := os.Stat(compilerPath)
	capabilityMu.Lock()
	if statErr == nil {
		if cached, ok := capabilityCache[compilerPath]; ok && cached.modified.Equal(info.ModTime()) && cached.size == info.Size() {
			capabilityMu.Unlock()
			return cached.value
		}
	}
	capabilityMu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := processutil.Command(ctx, compilerPath, "--capabilities=json")
	out, err := cmd.Output()
	var caps Capabilities
	if err != nil || json.Unmarshal(out, &caps) != nil || caps.SchemaVersion != 1 {
		caps = Capabilities{}
	}
	if caps.Features == nil {
		caps.Features = []string{}
	}
	if statErr == nil && ctx.Err() == nil {
		capabilityMu.Lock()
		capabilityCache[compilerPath] = capabilityEntry{modified: info.ModTime(), size: info.Size(), value: caps}
		capabilityMu.Unlock()
	}
	return caps
}

// ---- Metadata document -----------------------------------------------------

// Position is 1-based; Column counts bytes within the line.
type Position struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// MetaDiagnostic is a structured compiler message.
type MetaDiagnostic struct {
	Path     string `json:"path"`
	Range    Range  `json:"range"`
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

// SourceFile identifies one input and the exact bytes that were compiled.
type SourceFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type MetaArtifact struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

type Target struct {
	Name         string `json:"name"`
	BinaryFormat string `json:"binaryFormat"`
}

// AddressRange is half-open: [Start, End). Kind is "native" for 6502 code
// reached directly by the program counter and "bytecode" for code executed
// by the interpreter, where the interpreter's instruction pointer holds the
// address.
type AddressRange struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	Kind  string `json:"kind"`
}

// LineMapping ties one executable source line to its code.
type LineMapping struct {
	Source int            `json:"source"`
	Line   int            `json:"line"`
	Ranges []AddressRange `json:"ranges"`
}

// Symbol is a typed program symbol. Size is the storage size in bytes.
type Symbol struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Address int    `json:"address"`
	Size    int    `json:"size"`
	Source  int    `json:"source"`
	Line    int    `json:"line"`
}

// Interpreter locates the bytecode dispatch loop: Dispatch is the address
// executed once per bytecode instruction and Pointer the zero-page location
// of the 16-bit bytecode instruction pointer.
type Interpreter struct {
	Dispatch int `json:"dispatch"`
	Pointer  int `json:"pointer"`
}

type DebugInfo struct {
	Interpreter Interpreter   `json:"interpreter"`
	Lines       []LineMapping `json:"lines"`
	Symbols     []Symbol      `json:"symbols"`
}

// Metadata is the versioned document written by `--metadata-json`, on both
// success and failure.
type Metadata struct {
	SchemaVersion int `json:"schemaVersion"`
	Compiler      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"compiler"`
	Success     bool             `json:"success"`
	Target      Target           `json:"target"`
	Sources     []SourceFile     `json:"sources"`
	Diagnostics []MetaDiagnostic `json:"diagnostics"`
	Artifacts   []MetaArtifact   `json:"artifacts"`
	Debug       *DebugInfo       `json:"debug,omitempty"`
}

const maxMetadataBytes = 32 << 20

// LoadMetadata reads and validates a metadata document.
func LoadMetadata(path string) (*Metadata, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxMetadataBytes {
		return nil, errors.New("compiler metadata is too large")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseMetadata(raw)
}

// ParseMetadata decodes a document and rejects anything the debugger could
// not use safely: unknown versions, out-of-range addresses and references
// to sources that do not exist.
func ParseMetadata(raw []byte) (*Metadata, error) {
	var meta Metadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, fmt.Errorf("compiler metadata is not valid JSON: %w", err)
	}
	if meta.SchemaVersion != MetadataSchemaVersion {
		return nil, fmt.Errorf("compiler metadata schema version %d is not supported (expected %d)", meta.SchemaVersion, MetadataSchemaVersion)
	}
	for index, source := range meta.Sources {
		if source.Path == "" {
			return nil, fmt.Errorf("metadata source %d has no path", index)
		}
		if decoded, err := hex.DecodeString(source.SHA256); err != nil || len(decoded) != sha256.Size {
			return nil, fmt.Errorf("metadata source %q has an invalid hash", source.Path)
		}
	}
	for index, diagnostic := range meta.Diagnostics {
		switch diagnostic.Severity {
		case "error", "warning", "info":
		default:
			return nil, fmt.Errorf("metadata diagnostic %d has unknown severity %q", index, diagnostic.Severity)
		}
		if diagnostic.Range.Start.Line < 0 || diagnostic.Range.Start.Column < 0 {
			return nil, fmt.Errorf("metadata diagnostic %d has a negative position", index)
		}
	}
	if meta.Debug != nil {
		if err := validateDebug(&meta); err != nil {
			return nil, err
		}
	}
	if meta.Sources == nil {
		meta.Sources = []SourceFile{}
	}
	if meta.Diagnostics == nil {
		meta.Diagnostics = []MetaDiagnostic{}
	}
	if meta.Artifacts == nil {
		meta.Artifacts = []MetaArtifact{}
	}
	return &meta, nil
}

func validAddress(value int) bool { return value >= 0 && value <= 0xFFFF }

func validateDebug(meta *Metadata) error {
	debug := meta.Debug
	if !validAddress(debug.Interpreter.Dispatch) || debug.Interpreter.Pointer < 0 || debug.Interpreter.Pointer > 0xFE {
		return errors.New("metadata interpreter description is out of range")
	}
	for index, line := range debug.Lines {
		if line.Source < 0 || line.Source >= len(meta.Sources) {
			return fmt.Errorf("metadata line mapping %d refers to an unknown source", index)
		}
		if line.Line < 1 {
			return fmt.Errorf("metadata line mapping %d has an invalid line", index)
		}
		for _, r := range line.Ranges {
			if !validAddress(r.Start) || r.End < r.Start || r.End > 0x10000 {
				return fmt.Errorf("metadata line mapping %d has an invalid address range", index)
			}
			if r.Kind != "native" && r.Kind != "bytecode" {
				return fmt.Errorf("metadata line mapping %d has unknown code kind %q", index, r.Kind)
			}
		}
	}
	for index, symbol := range debug.Symbols {
		if symbol.Name == "" || !validAddress(symbol.Address) || symbol.Size < 0 || symbol.Address+symbol.Size > 0x10000 {
			return fmt.Errorf("metadata symbol %d is invalid", index)
		}
		if symbol.Source < -1 || symbol.Source >= len(meta.Sources) {
			return fmt.Errorf("metadata symbol %q refers to an unknown source", symbol.Name)
		}
	}
	if debug.Lines == nil {
		debug.Lines = []LineMapping{}
	}
	if debug.Symbols == nil {
		debug.Symbols = []Symbol{}
	}
	sort.SliceStable(debug.Lines, func(i, j int) bool {
		if debug.Lines[i].Source != debug.Lines[j].Source {
			return debug.Lines[i].Source < debug.Lines[j].Source
		}
		return debug.Lines[i].Line < debug.Lines[j].Line
	})
	return nil
}

// HashFile returns the SHA-256 of a file's bytes in lowercase hex.
func HashFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// ResolveSource maps a metadata source path to a workspace-relative path.
// The compiler writes paths relative to its working directory (the workspace
// root); anything that escapes the root resolves to "".
func ResolveSource(root, path string) string {
	candidate := filepath.FromSlash(path)
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	rel, err := filepath.Rel(root, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return ""
	}
	return filepath.ToSlash(rel)
}

// StaleSources lists workspace-relative sources whose current content no
// longer matches what was compiled. Source debugging must be refused while
// the list is non-empty.
func (m *Metadata) StaleSources(root string) []string {
	stale := []string{}
	for _, source := range m.Sources {
		path := filepath.FromSlash(source.Path)
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		hash, err := HashFile(path)
		if err != nil || !strings.EqualFold(hash, source.SHA256) {
			name := ResolveSource(root, source.Path)
			if name == "" {
				name = source.Path
			}
			stale = append(stale, name)
		}
	}
	return stale
}
