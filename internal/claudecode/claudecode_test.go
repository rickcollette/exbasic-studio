package claudecode

import (
	"strings"
	"testing"
)

func TestUserPromptIncludesSelectedCode(t *testing.T) {
	got := userPrompt("Explain this", "src/main.bas", "PRINT 42")
	for _, want := range []string{"Explain this", "src/main.bas", "PRINT 42"} {
		if !strings.Contains(got, want) {
			t.Fatalf("userPrompt() missing %q: %q", want, got)
		}
	}
}

func TestSystemPromptUsesPlanMode(t *testing.T) {
	got := systemPrompt()
	if !strings.Contains(got, "plan mode") || !strings.Contains(got, "do not modify") {
		t.Fatalf("systemPrompt() does not describe the safety boundary: %q", got)
	}
}

func TestSystemPromptAllowsExplicitEditMode(t *testing.T) {
	text := systemPrompt("edit")
	if !strings.Contains(text, "explicitly enabled workspace changes") {
		t.Fatalf("edit prompt did not permit changes: %s", text)
	}
}
