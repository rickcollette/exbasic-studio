package debugger

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"exbasicstudio/internal/compiler"
)

// fakeMemory is a 64K address space for tests.
type fakeMemory struct {
	data  [0x10000]byte
	reads int
}

func (m *fakeMemory) ReadMemory(address, length int) ([]byte, error) {
	m.reads++
	if address < 0 || length < 1 || address+length > 0x10000 || length > 1024 {
		return nil, errors.New("out of range")
	}
	return append([]byte{}, m.data[address:address+length]...), nil
}

func (m *fakeMemory) poke(address int, bytes ...byte) { copy(m.data[address:], bytes) }

func writeSource(t *testing.T, root, rel, content string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func sampleMap(t *testing.T) (*SourceMap, string) {
	t.Helper()
	root := t.TempDir()
	mainHash := writeSource(t, root, "src/main.bas", "' c\nA=1\n\nPRINT A\nEND\n")
	utilHash := writeSource(t, root, "lib/util.bas", "FUNC F()\nENDFUNC\n")
	meta := &compiler.Metadata{SchemaVersion: 1}
	meta.Sources = []compiler.SourceFile{{Path: "src/main.bas", SHA256: mainHash}, {Path: "lib/util.bas", SHA256: utilHash}}
	meta.Debug = &compiler.DebugInfo{
		Interpreter: compiler.Interpreter{Dispatch: 130, Pointer: 131},
		Lines: []compiler.LineMapping{
			{Source: 0, Line: 2, Ranges: []compiler.AddressRange{{Start: 0x3000, End: 0x3008, Kind: "bytecode"}}},
			{Source: 0, Line: 4, Ranges: []compiler.AddressRange{{Start: 0x3008, End: 0x3020, Kind: "native"}, {Start: 0x3100, End: 0x3104, Kind: "bytecode"}}},
			{Source: 0, Line: 5, Ranges: []compiler.AddressRange{{Start: 0x3020, End: 0x3021, Kind: "bytecode"}}},
			{Source: 1, Line: 1, Ranges: []compiler.AddressRange{{Start: 0x3021, End: 0x3030, Kind: "bytecode"}}},
		},
		Symbols: []compiler.Symbol{
			{Name: "A", Type: "word", Address: 0x2000, Size: 2, Source: -1},
			{Name: "NAME$", Type: "string", Address: 0x2002, Size: 2, Source: -1},
			{Name: "X%", Type: "float", Address: 0x2004, Size: 6, Source: -1},
			{Name: "SCORES", Type: "word-array", Address: 0x200A, Size: 2, Source: -1},
			{Name: "F", Type: "function", Address: 0x3021, Size: 0, Source: -1},
		},
	}
	m, err := NewSourceMap(root, meta)
	if err != nil {
		t.Fatal(err)
	}
	return m, root
}

func TestSourceMapLocatesAddresses(t *testing.T) {
	m, _ := sampleMap(t)
	cases := []struct {
		address int
		path    string
		line    int
	}{
		{0x3000, "src/main.bas", 2}, {0x3007, "src/main.bas", 2}, {0x3008, "src/main.bas", 4},
		{0x301F, "src/main.bas", 4}, {0x3020, "src/main.bas", 5}, {0x3021, "lib/util.bas", 1},
		{0x3102, "src/main.bas", 4},
	}
	for _, tc := range cases {
		path, line, ok := m.Locate(tc.address)
		if !ok || path != tc.path || line != tc.line {
			t.Errorf("Locate(%#x) = %q:%d %v, want %s:%d", tc.address, path, line, ok, tc.path, tc.line)
		}
	}
	for _, outside := range []int{0x2FFF, 0x3030, 0x3050, 0x3104, 0, 0xFFFF} {
		if _, _, ok := m.Locate(outside); ok {
			t.Errorf("Locate(%#x) matched a line", outside)
		}
	}
	if got := m.LineStarts(); fmt.Sprint(got) != fmt.Sprint([]int{0x3000, 0x3008, 0x3020, 0x3021, 0x3100}) {
		t.Fatalf("line starts = %v", got)
	}
	if m.Dispatch != 130 || m.Pointer != 131 {
		t.Fatalf("interpreter = %d/%d", m.Dispatch, m.Pointer)
	}
}

func TestBreakpointsRelocateToExecutableLines(t *testing.T) {
	m, _ := sampleMap(t)
	exact := m.Resolve(Breakpoint{Path: "src/main.bas", Line: 4, Enabled: true})
	if !exact.Verified || exact.ResolvedLine != 4 || fmt.Sprint(exact.Addresses) != fmt.Sprint([]int{0x3008, 0x3100}) {
		t.Fatalf("exact = %+v", exact)
	}
	// Line 1 is a comment and line 3 is blank: both move to the next code.
	if got := m.Resolve(Breakpoint{Path: "src/main.bas", Line: 1}); got.ResolvedLine != 2 || !got.Verified {
		t.Fatalf("comment line = %+v", got)
	}
	if got := m.Resolve(Breakpoint{Path: "src/main.bas", Line: 3}); got.ResolvedLine != 4 {
		t.Fatalf("blank line = %+v", got)
	}
	// Past the end: the last executable line.
	if got := m.Resolve(Breakpoint{Path: "src/main.bas", Line: 99}); got.ResolvedLine != 5 || !got.Verified {
		t.Fatalf("past end = %+v", got)
	}
	if got := m.Resolve(Breakpoint{Path: "src/other.bas", Line: 1}); got.Verified || len(got.Addresses) != 0 {
		t.Fatalf("unknown file = %+v", got)
	}
	resolved, addresses := m.ResolveAll([]Breakpoint{
		{Path: "src/main.bas", Line: 2, Enabled: true}, {Path: "src/main.bas", Line: 1, Enabled: true},
		{Path: "src/main.bas", Line: 5, Enabled: false}, {Path: "nope.bas", Line: 1, Enabled: true},
	})
	if len(resolved) != 4 || fmt.Sprint(addresses) != fmt.Sprint([]int{0x3000}) {
		t.Fatalf("resolved=%+v addresses=%v", resolved, addresses)
	}
	if got := Unresolved([]Breakpoint{{Path: "a.bas", Line: 3, Enabled: true}}); len(got) != 1 || got[0].Verified || got[0].Addresses == nil {
		t.Fatalf("unresolved = %+v", got)
	}
}

func TestStaleAndMissingDebugInformationIsRefused(t *testing.T) {
	m, root := sampleMap(t)
	_ = m
	meta := &compiler.Metadata{SchemaVersion: 1}
	if _, err := NewSourceMap(root, meta); err == nil || !strings.Contains(err.Error(), "no debug information") {
		t.Fatalf("missing debug info accepted: %v", err)
	}
	hash := writeSource(t, root, "src/main.bas", "PRINT 1\n")
	meta.Sources = []compiler.SourceFile{{Path: "src/main.bas", SHA256: hash}}
	meta.Debug = &compiler.DebugInfo{Lines: []compiler.LineMapping{{Source: 0, Line: 1, Ranges: []compiler.AddressRange{{Start: 1, End: 2, Kind: "bytecode"}}}}}
	if _, err := NewSourceMap(root, meta); err != nil {
		t.Fatalf("fresh build refused: %v", err)
	}
	writeSource(t, root, "src/main.bas", "PRINT 2\n")
	if _, err := NewSourceMap(root, meta); err == nil || !strings.Contains(err.Error(), "build again") {
		t.Fatalf("stale mapping accepted: %v", err)
	}
}

func TestSymbolsAreCaseInsensitive(t *testing.T) {
	m, _ := sampleMap(t)
	if symbol, ok := m.Symbol(" name$ "); !ok || symbol.Address != 0x2002 || symbol.Type != "string" {
		t.Fatalf("symbol = %+v %v", symbol, ok)
	}
	if _, ok := m.Symbol("missing"); ok {
		t.Fatal("unknown symbol found")
	}
	list := m.Symbols()
	if len(list) != 5 || list[0].Name != "A" || list[len(list)-1].Name != "X%" {
		t.Fatalf("symbols = %+v", list)
	}
}

func TestDisassemble(t *testing.T) {
	code := []byte{0xA9, 0x10, 0x8D, 0x0A, 0xD4, 0x20, 0x00, 0x30, 0xD0, 0xFE, 0x6C, 0x34, 0x12, 0xB1, 0x82, 0x0A, 0x60, 0x02, 0xA9}
	got := Disassemble(code, 0x2000, 20)
	want := []string{"LDA #$10", "STA $D40A", "JSR $3000", "BNE $2008", "JMP ($1234)", "LDA ($82),Y", "ASL A", "RTS", ".BYTE $02"}
	if len(got) != len(want) {
		t.Fatalf("got %d instructions: %+v", len(got), got)
	}
	for i, text := range want {
		if got[i].Text != text {
			t.Errorf("instruction %d = %q, want %q", i, got[i].Text, text)
		}
	}
	if got[1].Address != 0x2002 || got[1].Bytes != "8D 0A D4" {
		t.Fatalf("second instruction = %+v", got[1])
	}
	// The trailing LDA has no operand in the buffer and is not emitted.
	if last := got[len(got)-1]; last.Address != 0x2011 {
		t.Fatalf("last = %+v", last)
	}
	if limited := Disassemble(code, 0x2000, 2); len(limited) != 2 {
		t.Fatalf("count not honoured: %d", len(limited))
	}
	if empty := Disassemble(nil, 0, 5); len(empty) != 0 {
		t.Fatal("empty input produced instructions")
	}
	// Backward branch and address wrap.
	if back := Disassemble([]byte{0x10, 0xFC}, 0x0004, 1); back[0].Text != "BPL $0002" {
		t.Fatalf("backward branch = %q", back[0].Text)
	}
}

func TestFramesFollowTheReturnStack(t *testing.T) {
	memory := &fakeMemory{}
	m, _ := sampleMap(t)
	// JSR $4000 at $300A (inside line 4) and an outer JSR $300A at $5000.
	memory.poke(0x300A, 0x20, 0x00, 0x40)
	memory.poke(0x5000, 0x20, 0x0A, 0x30)
	// Stack (top first): pushed data byte, then $300C, then $5002.
	sp := 0xF9
	memory.poke(0x100+sp+1, 0x55, 0x0C, 0x30, 0x02, 0x50, 0x77)
	frames, err := Frames(memory, sp, m)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 2 {
		t.Fatalf("frames = %+v", frames)
	}
	if frames[0].Call != 0x300A || frames[0].Return != 0x300D || frames[0].Target != 0x4000 || frames[0].Path != "src/main.bas" || frames[0].Line != 4 {
		t.Fatalf("inner frame = %+v", frames[0])
	}
	if frames[1].Call != 0x5000 || frames[1].Return != 0x5003 || frames[1].Path != "" {
		t.Fatalf("outer frame = %+v", frames[1])
	}
	if empty, _ := Frames(memory, 0xFF, m); len(empty) != 0 {
		t.Fatalf("empty stack produced frames: %+v", empty)
	}
}

func TestParseWatch(t *testing.T) {
	m, _ := sampleMap(t)
	cases := []struct {
		expression string
		address    int
		typ        string
	}{
		{"A", 0x2000, "word"}, {"a", 0x2000, "word"}, {"name$", 0x2002, "string"}, {"X%", 0x2004, "float"},
		{"1536", 1536, "byte"}, {"$D40A", 0xD40A, "byte"}, {"0x0600", 0x600, "byte"},
		{"A + 2", 0x2002, "byte"}, {"SCORES-1", 0x2009, "byte"}, {"A+$10", 0x2010, "byte"}, {"$600+1", 0x601, "byte"},
	}
	for _, tc := range cases {
		address, typ, err := ParseWatch(tc.expression, m)
		if err != nil || address != tc.address || typ != tc.typ {
			t.Errorf("ParseWatch(%q) = %#x %q %v", tc.expression, address, typ, err)
		}
	}
	for _, bad := range []string{"", "NOPE", "A +", "A + B", "$FFFF + 1", "$ZZ", "-5", "A*2", strings.Repeat("A", 81)} {
		if _, _, err := ParseWatch(bad, m); err == nil {
			t.Errorf("ParseWatch(%q) succeeded", bad)
		}
	}
	if _, _, err := ParseWatch("A", nil); err == nil {
		t.Error("a symbol resolved without debug information")
	}
	if address, _, err := ParseWatch("$600", nil); err != nil || address != 0x600 {
		t.Errorf("an address needs no symbols: %v", err)
	}
}

func TestEvaluateTypedValues(t *testing.T) {
	m, _ := sampleMap(t)
	memory := &fakeMemory{}
	memory.poke(0x2000, 0xFE, 0xFF) // A = -2
	memory.poke(0x2002, 0x00, 0x40) // NAME$ -> $4000
	memory.poke(0x4000, 5, 'H', 'E', 'L', 'L', 'O'|0x80)
	memory.poke(0x2004, 0x40, 0x12, 0x34, 0x50, 0x00, 0x00) // 12.345
	memory.poke(0x200A, 0x00, 0x50)                         // SCORES -> $5000
	memory.poke(0x5000, 1, 0, 2, 0)
	memory.poke(0x0600, 0x2A, 0x01)

	if got := Evaluate("A", m, memory); got.Error != "" || got.Value != "-2 ($FFFE)" || got.Type != "word" {
		t.Fatalf("word = %+v", got)
	}
	if got := Evaluate("NAME$", m, memory); got.Error != "" || !strings.HasPrefix(got.Value, `"HELLO" (5 characters`) {
		t.Fatalf("string = %+v", got)
	}
	if got := Evaluate("X%", m, memory); got.Error != "" || got.Value != "12.345" {
		t.Fatalf("float = %+v", got)
	}
	if got := Evaluate("SCORES", m, memory); got.Error != "" || !strings.HasPrefix(got.Value, "$5000: 01 00 02 00") {
		t.Fatalf("array = %+v", got)
	}
	if strings.Count(Evaluate("SCORES", m, memory).Value, " ") > arrayPreviewBytes+2 {
		t.Fatal("array view is not bounded")
	}
	if got := Evaluate("$0600", m, memory); got.Value != "42 ($2A), word 298 ($012A)" {
		t.Fatalf("address = %+v", got)
	}
	if got := Evaluate("F", m, memory); got.Value != "$3021" {
		t.Fatalf("function = %+v", got)
	}
	if got := Evaluate("NOPE", m, memory); got.Error == "" {
		t.Fatal("unknown symbol evaluated")
	}
	// Uninitialized strings and arrays hold a null pointer.
	memory.poke(0x2002, 0, 0)
	memory.poke(0x200A, 0, 0)
	if got := Evaluate("NAME$", m, memory); !strings.Contains(got.Value, "empty") {
		t.Fatalf("null string = %+v", got)
	}
	if got := Evaluate("SCORES", m, memory); got.Value != "not dimensioned" {
		t.Fatalf("null array = %+v", got)
	}
	if got := Evaluate("$FFFF", m, memory); got.Error != "" || got.Value != "0 ($00)" {
		t.Fatalf("last byte = %+v", got)
	}
}

func TestDecodeFloat(t *testing.T) {
	cases := []struct {
		bytes []byte
		want  float64
	}{
		{[]byte{0, 0, 0, 0, 0, 0}, 0},
		{[]byte{0x40, 0x01, 0, 0, 0, 0}, 1},
		{[]byte{0x40, 0x12, 0x34, 0x50, 0, 0}, 12.345},
		{[]byte{0xC0, 0x02, 0x50, 0, 0, 0}, -2.5},
		{[]byte{0x41, 0x01, 0, 0, 0, 0}, 100},
		{[]byte{0x3F, 0x50, 0, 0, 0, 0}, 0.5},
	}
	for _, tc := range cases {
		got, err := DecodeFloat(tc.bytes)
		if err != nil || math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("DecodeFloat(% X) = %v, %v; want %v", tc.bytes, got, err, tc.want)
		}
	}
	if _, err := DecodeFloat([]byte{0x40, 0xAB, 0, 0, 0, 0}); err == nil {
		t.Error("invalid BCD accepted")
	}
	if _, err := DecodeFloat([]byte{1, 2}); err == nil {
		t.Error("short value accepted")
	}
}

func FuzzParseWatch(f *testing.F) {
	for _, seed := range []string{"A", "$600", "A+1", "NAME$-2", "0x10", "+", "A++", "-", "$"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, expression string) {
		address, _, err := ParseWatch(expression, nil)
		if err == nil && (address < 0 || address > 0xFFFF) {
			t.Fatalf("ParseWatch(%q) returned out-of-range address %d", expression, address)
		}
	})
}
