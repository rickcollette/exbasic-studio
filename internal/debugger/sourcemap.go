// Package debugger turns compiler metadata and raw machine state into a
// source-level view: line/address mapping, breakpoint relocation, 6502
// disassembly, return-stack frames and typed watch values.
//
// It reads only the compiler's metadata document; it never parses label or
// listing files.
package debugger

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"exbasicstudio/internal/compiler"
)

// Breakpoint is a user breakpoint on a workspace file line.
type Breakpoint struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Enabled bool   `json:"enabled"`
}

// ResolvedBreakpoint reports where a breakpoint was actually placed.
// Verified is false when the file has no executable code to attach to.
type ResolvedBreakpoint struct {
	Path         string `json:"path"`
	Line         int    `json:"line"`
	Enabled      bool   `json:"enabled"`
	ResolvedLine int    `json:"resolvedLine"`
	Verified     bool   `json:"verified"`
	Addresses    []int  `json:"addresses"`
}

// Symbol is a program symbol from the metadata.
type Symbol struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Address int    `json:"address"`
	Size    int    `json:"size"`
}

type span struct {
	start, end int
	source     int
	line       int
}

type sourceLine struct {
	line   int
	starts []int
}

// SourceMap relates source lines to code addresses for one build.
type SourceMap struct {
	root      string
	sources   []string // workspace-relative, slash-separated; "" if outside
	lines     map[string][]sourceLine
	spans     []span // sorted by start
	Dispatch  int
	Pointer   int
	symbols   []Symbol
	byName    map[string]int
	lineStart []int
}

// NewSourceMap builds the map for a workspace root. It refuses a build
// whose sources have changed since compilation: addresses would no longer
// correspond to the text on screen.
func NewSourceMap(root string, meta *compiler.Metadata) (*SourceMap, error) {
	if meta == nil || meta.Debug == nil || len(meta.Debug.Lines) == 0 {
		return nil, errors.New("this build has no debug information; enable \"Debug symbols\" in the project settings and build again")
	}
	if stale := meta.StaleSources(root); len(stale) > 0 {
		return nil, fmt.Errorf("the sources changed after the last build (%s); build again before debugging", strings.Join(stale, ", "))
	}
	m := &SourceMap{
		root: root, lines: map[string][]sourceLine{}, byName: map[string]int{},
		Dispatch: meta.Debug.Interpreter.Dispatch, Pointer: meta.Debug.Interpreter.Pointer,
	}
	for _, source := range meta.Sources {
		m.sources = append(m.sources, compiler.ResolveSource(root, source.Path))
	}
	starts := map[int]bool{}
	for _, mapping := range meta.Debug.Lines {
		path := m.sources[mapping.Source]
		entry := sourceLine{line: mapping.Line}
		for _, r := range mapping.Ranges {
			if r.End <= r.Start {
				continue
			}
			entry.starts = append(entry.starts, r.Start)
			m.spans = append(m.spans, span{start: r.Start, end: r.End, source: mapping.Source, line: mapping.Line})
			starts[r.Start] = true
		}
		if path != "" && len(entry.starts) > 0 {
			m.lines[path] = append(m.lines[path], entry)
		}
	}
	for path := range m.lines {
		sort.Slice(m.lines[path], func(i, j int) bool { return m.lines[path][i].line < m.lines[path][j].line })
	}
	sort.Slice(m.spans, func(i, j int) bool { return m.spans[i].start < m.spans[j].start })
	for address := range starts {
		m.lineStart = append(m.lineStart, address)
	}
	sort.Ints(m.lineStart)
	for _, symbol := range meta.Debug.Symbols {
		key := strings.ToUpper(symbol.Name)
		if _, exists := m.byName[key]; exists {
			continue
		}
		m.byName[key] = len(m.symbols)
		m.symbols = append(m.symbols, Symbol{Name: symbol.Name, Type: symbol.Type, Address: symbol.Address, Size: symbol.Size})
	}
	return m, nil
}

// LineStarts returns every address at which a source line begins.
func (m *SourceMap) LineStarts() []int { return append([]int{}, m.lineStart...) }

// Symbols lists the program symbols, sorted by name.
func (m *SourceMap) Symbols() []Symbol {
	out := append([]Symbol{}, m.symbols...)
	sort.Slice(out, func(i, j int) bool { return strings.ToUpper(out[i].Name) < strings.ToUpper(out[j].Name) })
	return out
}

// Symbol finds a symbol by name, ignoring case.
func (m *SourceMap) Symbol(name string) (Symbol, bool) {
	index, ok := m.byName[strings.ToUpper(strings.TrimSpace(name))]
	if !ok {
		return Symbol{}, false
	}
	return m.symbols[index], true
}

// Locate maps a code address to the source line whose code contains it.
func (m *SourceMap) Locate(address int) (path string, line int, ok bool) {
	// The last span starting at or before the address is the candidate; a
	// few earlier ones are checked too in case ranges overlap.
	last := sort.Search(len(m.spans), func(i int) bool { return m.spans[i].start > address }) - 1
	for index := last; index >= 0 && index > last-8; index-- {
		s := m.spans[index]
		if address >= s.start && address < s.end {
			if path = m.sources[s.source]; path == "" {
				return "", 0, false
			}
			return path, s.line, true
		}
	}
	return "", 0, false
}

// Resolve places a breakpoint. A line without code moves to the next
// executable line in the same file, or to the last one before it when
// nothing follows.
func (m *SourceMap) Resolve(bp Breakpoint) ResolvedBreakpoint {
	resolved := ResolvedBreakpoint{Path: bp.Path, Line: bp.Line, Enabled: bp.Enabled, Addresses: []int{}}
	lines := m.lines[bp.Path]
	if len(lines) == 0 {
		return resolved
	}
	index := sort.Search(len(lines), func(i int) bool { return lines[i].line >= bp.Line })
	if index == len(lines) {
		index = len(lines) - 1
	}
	resolved.ResolvedLine = lines[index].line
	resolved.Addresses = append(resolved.Addresses, lines[index].starts...)
	resolved.Verified = true
	return resolved
}

// ResolveAll resolves breakpoints and returns the address table for the
// enabled ones.
func (m *SourceMap) ResolveAll(breakpoints []Breakpoint) ([]ResolvedBreakpoint, []int) {
	resolved := make([]ResolvedBreakpoint, 0, len(breakpoints))
	seen := map[int]bool{}
	addresses := []int{}
	for _, bp := range breakpoints {
		item := m.Resolve(bp)
		resolved = append(resolved, item)
		if !item.Enabled || !item.Verified {
			continue
		}
		for _, address := range item.Addresses {
			if !seen[address] {
				seen[address] = true
				addresses = append(addresses, address)
			}
		}
	}
	sort.Ints(addresses)
	return resolved, addresses
}

// Unresolved describes breakpoints when no debug session exists.
func Unresolved(breakpoints []Breakpoint) []ResolvedBreakpoint {
	out := make([]ResolvedBreakpoint, 0, len(breakpoints))
	for _, bp := range breakpoints {
		out = append(out, ResolvedBreakpoint{Path: bp.Path, Line: bp.Line, Enabled: bp.Enabled, Addresses: []int{}})
	}
	return out
}
