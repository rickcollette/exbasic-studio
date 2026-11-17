package compiler

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"exbasicstudio/internal/project"
	"exbasicstudio/internal/tools/toolstest"
)

// fakeCompiler builds a small Go program that stands in for exbasic, so the
// driver can be tested on every platform without the real toolchain. Its
// behaviour is selected by the FAKE_MODE environment variable.
func fakeCompiler(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	source := filepath.Join(dir, "main.go")
	program := `package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	mode := os.Getenv("FAKE_MODE")
	args := os.Args[1:]
	if len(args) == 1 && args[0] == "--capabilities=json" {
		if strings.Contains(mode, "nocaps") {
			fmt.Fprintln(os.Stderr, "exbasic: invalid option '--capabilities=json', try -h for help")
			os.Exit(1)
		}
		fmt.Println("{\"schemaVersion\":1,\"version\":\"9.9.9\",\"features\":[\"metadata-json\",\"structured-diagnostics\",\"debug-metadata\"]}")
		return
	}
	var out, meta, input string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-o" && i+1 < len(args):
			out = args[i+1]
			i++
		case args[i] == "--metadata-json" && i+1 < len(args):
			meta = args[i+1]
			i++
		case !strings.HasPrefix(args[i], "-"):
			if input == "" {
				input = args[i]
			}
		}
	}
	for _, arg := range args {
		fmt.Println("ARG " + arg)
	}
	if strings.Contains(mode, "slow") {
		fmt.Println("started")
		time.Sleep(time.Minute)
	}
	if strings.Contains(mode, "fail") {
		fmt.Fprintln(os.Stderr, input+":3:4: parse error, missing IF")
		fmt.Fprintln(os.Stderr, "  PRINT +")
		if meta != "" {
			os.WriteFile(meta, []byte("{\"schemaVersion\":1,\"success\":false,\"sources\":[],\"diagnostics\":[{\"path\":\"src/main.bas\",\"severity\":\"error\",\"code\":\"E100\",\"message\":\"structured message\",\"range\":{\"start\":{\"line\":3,\"column\":5},\"end\":{\"line\":3,\"column\":9}}}]}"), 0o644)
		}
		os.Exit(1)
	}
	if out != "" {
		os.WriteFile(out, []byte{0xff, 0xff}, 0o644)
	}
	if meta != "" && !strings.Contains(mode, "badmeta") {
		os.WriteFile(meta, []byte(os.Getenv("FAKE_METADATA")), 0o644)
	}
	if meta != "" && strings.Contains(mode, "badmeta") {
		os.WriteFile(meta, []byte("{not json"), 0o644)
	}
}
`
	if err := os.WriteFile(source, []byte(program), 0o644); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "fake-exbasic")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", binary, source)
	cmd.Env = append(os.Environ(), "GOFLAGS=-buildvcs=false", "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake compiler: %v\n%s", err, out)
	}
	return binary
}

var (
	fakeOnce sync.Once
	fakePath string
)

func sharedFake(t *testing.T) string {
	t.Helper()
	fakeOnce.Do(func() {
		dir, err := os.MkdirTemp("", "exbasic-fake-*")
		if err != nil {
			t.Fatal(err)
		}
		built := fakeCompiler(t)
		fakePath = filepath.Join(dir, filepath.Base(built))
		data, _ := os.ReadFile(built)
		if err = os.WriteFile(fakePath, data, 0o755); err != nil {
			t.Fatal(err)
		}
	})
	if fakePath == "" {
		t.Fatal("fake compiler unavailable")
	}
	return fakePath
}

func newProject(t *testing.T, mode string) *project.Project {
	t.Helper()
	p, err := project.Create(t.TempDir(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	// Each mode gets its own copy so the capability cache cannot mix them up.
	original := sharedFake(t)
	copyPath := filepath.Join(t.TempDir(), filepath.Base(original))
	data, _ := os.ReadFile(original)
	if err = os.WriteFile(copyPath, data, 0o755); err != nil {
		t.Fatal(err)
	}
	p.Config.Compiler = copyPath
	t.Setenv("FAKE_MODE", mode)
	return p
}

func goodMetadata(t *testing.T, p *project.Project) string {
	t.Helper()
	hash, err := HashFile(filepath.Join(p.Root, "src", "main.bas"))
	if err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{
		"schemaVersion": 1,
		"compiler":      map[string]any{"name": "exbasic", "version": "9.9.9"},
		"success":       true,
		"target":        map[string]any{"name": "atari", "binaryFormat": "xex"},
		"sources":       []any{map[string]any{"path": "src/main.bas", "sha256": hash}},
		"diagnostics":   []any{},
		"artifacts":     []any{},
		"debug": map[string]any{
			"interpreter": map[string]any{"dispatch": 0x2100, "pointer": 0x82},
			"lines": []any{
				map[string]any{"source": 0, "line": 3, "ranges": []any{map[string]any{"start": 0x3010, "end": 0x3020, "kind": "bytecode"}}},
				map[string]any{"source": 0, "line": 2, "ranges": []any{map[string]any{"start": 0x3000, "end": 0x3010, "kind": "bytecode"}}},
			},
			"symbols": []any{map[string]any{"name": "SCORE", "type": "word", "address": 0x2400, "size": 2, "source": 0, "line": 2}},
		},
	}
	raw, _ := json.Marshal(doc)
	return string(raw)
}

func TestBuildPassesArgumentVectorAndStreamsOutput(t *testing.T) {
	p := newProject(t, "")
	t.Setenv("FAKE_METADATA", goodMetadata(t, p))
	var streamed strings.Builder
	var mu sync.Mutex
	result, err := Build(context.Background(), p, func(_, text string) {
		mu.Lock()
		streamed.WriteString(text)
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success || result.Cancelled {
		t.Fatalf("build failed: %+v", result)
	}
	if len(result.Steps) != 1 || result.Steps[0].Argv[0] != p.Config.Compiler {
		t.Fatalf("steps = %+v", result.Steps)
	}
	argv := result.Steps[0].Argv
	joined := "\n" + strings.Join(argv, "\n") + "\n"
	for _, expected := range []string{"\n-t:atari\n", "\n-g\n", "\n-keep\n", "\n--metadata-json\n", "\n-o\n"} {
		if !strings.Contains(joined, expected) {
			t.Errorf("argv lacks %q: %v", strings.TrimSpace(expected), argv)
		}
	}
	if argv[len(argv)-1] != filepath.Join(p.Root, "src", "main.bas") {
		t.Fatalf("entry is not the final argument: %v", argv)
	}
	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(streamed.String(), "ARG -t:atari") || result.Output != streamed.String() {
		t.Fatalf("streamed output differs from the captured output:\n%q\n%q", streamed.String(), result.Output)
	}
	if result.Artifact("executable") != filepath.Join(p.Root, "build", "demo.xex") {
		t.Fatalf("artifacts = %+v", result.Artifacts)
	}
	if result.MetadataPath == "" || result.Artifact("metadata") != result.MetadataPath || !result.DebugAvailable {
		t.Fatalf("metadata not reported: %+v", result)
	}
	meta, err := LoadMetadata(result.MetadataPath)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Debug.Lines[0].Line != 2 {
		t.Fatalf("line mappings are not sorted: %+v", meta.Debug.Lines)
	}
	if stale := meta.StaleSources(p.Root); len(stale) != 0 {
		t.Fatalf("fresh build reported stale sources: %v", stale)
	}
	if err = os.WriteFile(filepath.Join(p.Root, "src", "main.bas"), []byte("PRINT 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if stale := meta.StaleSources(p.Root); len(stale) != 1 || stale[0] != "src/main.bas" {
		t.Fatalf("edited source not reported stale: %v", stale)
	}
}

func TestArgumentsAreNeverShellInterpreted(t *testing.T) {
	p := newProject(t, "")
	t.Setenv("FAKE_METADATA", goodMetadata(t, p))
	hostile := "inc; echo pwned > pwned.txt & $(id) `id`"
	if err := os.MkdirAll(filepath.Join(p.Root, hostile), 0o755); err != nil {
		t.Skipf("filesystem refuses the test directory name: %v", err)
	}
	p.Config.IncludePaths = []string{hostile}
	result, err := Build(context.Background(), p, nil)
	if err != nil || !result.Success {
		t.Fatalf("build: %+v %v", result, err)
	}
	if !strings.Contains(result.Output, "ARG -I:"+filepath.Join(p.Root, hostile)) {
		t.Fatalf("include path was not passed as one literal argument:\n%s", result.Output)
	}
	if _, statErr := os.Stat(filepath.Join(p.Root, "pwned.txt")); statErr == nil {
		t.Fatal("an argument was interpreted by a shell")
	}
}

func TestFailedBuildReportsStructuredDiagnostics(t *testing.T) {
	p := newProject(t, "fail")
	result, err := Build(context.Background(), p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success || len(result.Diagnostics) != 1 {
		t.Fatalf("result = %+v", result)
	}
	d := result.Diagnostics[0]
	if d.Path != "src/main.bas" || d.Line != 3 || d.Column != 5 || d.EndColumn != 9 || d.Code != "E100" || d.Message != "structured message" {
		t.Fatalf("diagnostic = %+v", d)
	}
	if len(result.Artifacts) != 0 || result.DebugAvailable {
		t.Fatalf("failed build reported artifacts: %+v", result)
	}
}

func TestCompilerWithoutMetadataFallsBackWithExplanation(t *testing.T) {
	p := newProject(t, "nocaps,fail")
	result, err := Build(context.Background(), p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success || len(result.Diagnostics) != 1 {
		t.Fatalf("result = %+v", result)
	}
	d := result.Diagnostics[0]
	// The plain-text position is a 0-based offset; diagnostics are 1-based.
	if d.Path != "src/main.bas" || d.Line != 3 || d.Column != 5 || d.Message != "parse error, missing IF" {
		t.Fatalf("fallback diagnostic = %+v", d)
	}
	for _, arg := range result.Steps[0].Argv {
		if arg == "--metadata-json" {
			t.Fatal("metadata flag passed to a compiler that does not support it")
		}
	}

	p = newProject(t, "nocaps")
	result, err = Build(context.Background(), p, nil)
	if err != nil || !result.Success {
		t.Fatalf("fallback build: %+v %v", result, err)
	}
	if result.DebugAvailable || result.MetadataPath != "" || !strings.Contains(result.DebugNote, "source-level debugging") {
		t.Fatalf("missing explanation for unavailable debugging: %+v", result)
	}
}

func TestUnusableMetadataDisablesDebuggingButNotTheBuild(t *testing.T) {
	p := newProject(t, "badmeta")
	result, err := Build(context.Background(), p, nil)
	if err != nil || !result.Success {
		t.Fatalf("build: %+v %v", result, err)
	}
	if result.DebugAvailable || result.DebugNote == "" {
		t.Fatalf("corrupt metadata was trusted: %+v", result)
	}
}

func TestBuildCancellationStopsTheCompiler(t *testing.T) {
	p := newProject(t, "slow")
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{}, 1)
	done := make(chan Result, 1)
	go func() {
		result, _ := Build(ctx, p, func(_, text string) {
			if strings.Contains(text, "started") {
				select {
				case started <- struct{}{}:
				default:
				}
			}
		})
		done <- result
	}()
	select {
	case <-started:
	case <-time.After(30 * time.Second):
		t.Fatal("compiler never started")
	}
	begin := time.Now()
	cancel()
	select {
	case result := <-done:
		if !result.Cancelled || result.Success {
			t.Fatalf("cancelled build = %+v", result)
		}
		if time.Since(begin) > 15*time.Second {
			t.Fatalf("cancellation took %v", time.Since(begin))
		}
	case <-time.After(30 * time.Second):
		t.Fatal("cancellation did not stop the build")
	}
}

func TestLintMapsTemporaryFileBackToTheEditedPath(t *testing.T) {
	p := newProject(t, "nocaps,fail")
	diagnostics, err := Lint(context.Background(), p, "src/main.bas", "PRINT +\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 || diagnostics[0].Path != "src/main.bas" || diagnostics[0].Line != 3 {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
	entries, _ := os.ReadDir(filepath.Join(p.Root, "src"))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".exbasic-lint-") {
			t.Fatalf("lint left %s behind", entry.Name())
		}
	}
}

func TestParseFallbackDiagnostics(t *testing.T) {
	output := "BAS compile 'a.bas' to 'a.asm'\r\nsrc/a.bas:12:0: parse error\r\n  PRINT\nsrc/b.bas:7: unclosed FOR\nexbasic: can't link file\nC:\\x\\y.bas:3:2: with drive\n"
	got := ParseFallbackDiagnostics(output, nil)
	if len(got) != 3 {
		t.Fatalf("got %d diagnostics: %+v", len(got), got)
	}
	if got[0].Path != "src/a.bas" || got[0].Line != 12 || got[0].Column != 1 || got[0].Message != "parse error" {
		t.Fatalf("first = %+v", got[0])
	}
	if got[1].Path != `C:\x\y.bas` || got[1].Line != 3 || got[1].Column != 3 {
		t.Fatalf("drive path = %+v", got[1])
	}
	if got[2].Path != "src/b.bas" || got[2].Line != 7 || got[2].Column != 1 || got[2].Message != "unclosed FOR" {
		t.Fatalf("line-only = %+v", got[2])
	}
}

func TestParseMetadataRejectsUnsafeDocuments(t *testing.T) {
	hash := strings.Repeat("ab", 32)
	valid := `{"schemaVersion":1,"success":true,"sources":[{"path":"a.bas","sha256":"` + hash + `"}],"debug":{"interpreter":{"dispatch":8192,"pointer":130},"lines":[{"source":0,"line":1,"ranges":[{"start":100,"end":110,"kind":"native"}]}],"symbols":[{"name":"A","type":"word","address":65534,"size":2,"source":0,"line":1}]}}`
	if _, err := ParseMetadata([]byte(valid)); err != nil {
		t.Fatalf("valid metadata rejected: %v", err)
	}
	bad := map[string]string{
		"version":        strings.Replace(valid, `"schemaVersion":1`, `"schemaVersion":2`, 1),
		"hash":           strings.Replace(valid, hash, "zz", 1),
		"source index":   strings.Replace(valid, `"source":0,"line":1,"ranges"`, `"source":4,"line":1,"ranges"`, 1),
		"address":        strings.Replace(valid, `"start":100`, `"start":70000`, 1),
		"reversed range": strings.Replace(valid, `"end":110`, `"end":5`, 1),
		"kind":           strings.Replace(valid, `"kind":"native"`, `"kind":"weird"`, 1),
		"symbol overrun": strings.Replace(valid, `"address":65534,"size":2`, `"address":65535,"size":2`, 1),
		"dispatch":       strings.Replace(valid, `"dispatch":8192`, `"dispatch":-1`, 1),
		"pointer":        strings.Replace(valid, `"pointer":130`, `"pointer":255`, 1),
		"line":           strings.Replace(valid, `"source":0,"line":1,"ranges"`, `"source":0,"line":0,"ranges"`, 1),
		"not json":       "{",
	}
	for name, document := range bad {
		if _, err := ParseMetadata([]byte(document)); err == nil {
			t.Errorf("%s: unsafe metadata accepted", name)
		}
	}
}

func FuzzParseMetadata(f *testing.F) {
	f.Add([]byte(`{"schemaVersion":1,"sources":[],"debug":{"interpreter":{"dispatch":1,"pointer":2},"lines":[],"symbols":[]}}`))
	f.Add([]byte(`{"schemaVersion":1,"debug":{"lines":[{"source":-1}]}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		meta, err := ParseMetadata(data)
		if err != nil || meta.Debug == nil {
			return
		}
		for _, line := range meta.Debug.Lines {
			if line.Source < 0 || line.Source >= len(meta.Sources) {
				t.Fatalf("accepted an out-of-range source index %d", line.Source)
			}
			for _, r := range line.Ranges {
				if r.Start < 0 || r.Start > 0xFFFF || r.End < r.Start || r.End > 0x10000 {
					t.Fatalf("accepted an invalid range %+v", r)
				}
			}
		}
	})
}

func TestResolveSourceStaysInsideTheWorkspace(t *testing.T) {
	root := t.TempDir()
	if got := ResolveSource(root, "src/main.bas"); got != "src/main.bas" {
		t.Fatalf("relative = %q", got)
	}
	if got := ResolveSource(root, filepath.Join(root, "lib", "a.bas")); got != "lib/a.bas" {
		t.Fatalf("absolute inside = %q", got)
	}
	for _, outside := range []string{"../x.bas", filepath.Join(filepath.Dir(root), "x.bas")} {
		if got := ResolveSource(root, outside); got != "" {
			t.Fatalf("ResolveSource(%q) = %q, want empty", outside, got)
		}
	}
}

// ---- Real toolchain --------------------------------------------------------

func TestBundledCompilerProducesLintDiagnostic(t *testing.T) {
	toolstest.Tool(t, "exbasic")
	p, err := project.Create(t.TempDir(), "lintdemo")
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Lint(context.Background(), p, p.Config.Entry, "PRINT +\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 || diagnostics[0].Line != 1 || diagnostics[0].Column < 1 || diagnostics[0].Path != "src/main.bas" {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}
	clean, err := Lint(context.Background(), p, p.Config.Entry, "PRINT 1\n", nil)
	if err != nil || len(clean) != 0 {
		t.Fatalf("clean source produced diagnostics: %#v %v", clean, err)
	}
}

func TestBundledCompilerBuildsSample(t *testing.T) {
	toolstest.Tool(t, "exbasic")
	p, err := project.Create(t.TempDir(), "sample")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Build(context.Background(), p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("sample build failed:\n%s", result.Output)
	}
	info, err := os.Stat(result.Artifact("executable"))
	if err != nil || info.Size() == 0 {
		t.Fatalf("no executable: %v", err)
	}
}

// An INCLUDE file with unsaved edits is checked as it appears in the editor,
// and unchanged files beside it are still read from the project.
func TestLintUsesUnsavedTextOfIncludedFiles(t *testing.T) {
	toolstest.Tool(t, "exbasic")
	root := t.TempDir()
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	program := "INCLUDE \"basfile.inc\"\nINCLUDE \"other.inc\"\nPRINT oneplusone() + two()\n"
	write("new.bas", program)
	write("basfile.inc", "FUNC oneplusone\n  RETURN 2\nENDFUNC\n")
	write("other.inc", "FUNC two()\n  RETURN 2\nENDFUNC\n")
	p, err := project.Open(root)
	if err != nil {
		t.Fatal(err)
	}

	onDisk, err := Lint(context.Background(), p, "new.bas", program, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(onDisk) != 1 || onDisk[0].Path != "basfile.inc" || onDisk[0].Line != 1 {
		t.Fatalf("saved include: unexpected diagnostics %#v", onDisk)
	}

	fixed := map[string]string{"basfile.inc": "FUNC oneplusone()\n  RETURN 2\nENDFUNC\n"}
	unsaved, err := Lint(context.Background(), p, "new.bas", program, fixed)
	if err != nil {
		t.Fatal(err)
	}
	if len(unsaved) != 0 {
		t.Fatalf("unsaved fix in the include was ignored: %#v", unsaved)
	}

	broken := map[string]string{"other.inc": "FUNC two\n  RETURN 2\nENDFUNC\n", "basfile.inc": fixed["basfile.inc"]}
	unsaved, err = Lint(context.Background(), p, "new.bas", program, broken)
	if err != nil {
		t.Fatal(err)
	}
	if len(unsaved) != 1 || unsaved[0].Path != "other.inc" {
		t.Fatalf("unsaved error in an include: unexpected diagnostics %#v", unsaved)
	}

	entries, _ := os.ReadDir(root)
	if len(entries) != 3 {
		t.Fatalf("lint left files in the project: %v", entries)
	}
}

func TestProgressFilterDropsRepeatedProgressLines(t *testing.T) {
	var out strings.Builder
	filter := newProgressFilter(func(_ string, chunk string) { out.WriteString(chunk) })
	// Chunks need not end on line boundaries.
	for _, chunk := range []string{"BAS compile 'a' to 'b'\nASM asse", "mble 'b' to 'c'\nLINK x.xex\nBAS compile 'a' to 'b'\n", "ASM assemble 'b' to 'c'\nLINK x.xex\nnote: kept the smallest\nnote: kept the smallest\nlast"} {
		filter.write("stdout", chunk)
	}
	filter.flush()
	want := "BAS compile 'a' to 'b'\nASM assemble 'b' to 'c'\nLINK x.xex\nnote: kept the smallest\nnote: kept the smallest\nlast"
	if out.String() != want {
		t.Fatalf("filtered output:\n%q\nwant:\n%q", out.String(), want)
	}
}
