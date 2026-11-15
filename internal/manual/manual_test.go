package manual

import "testing"

func TestBundledManual(t *testing.T) {
	if n := len(Files()); n < 24 {
		t.Fatalf("expected supplied manual, found %d files", n)
	}
	contents, err := Chapter("19-compiler-reference.md")
	if err != nil || len(contents) < 5000 {
		t.Fatal("compiler reference missing")
	}
	matches := Search("INCLUDE", 10)
	if len(matches) == 0 {
		t.Fatal("manual search returned no INCLUDE matches")
	}
	if _, err := Chapter("../secrets.md"); err == nil {
		t.Fatal("path traversal accepted")
	}
}
