package messagebubble

import (
	"strings"
	"testing"
)

func TestModelAndWrapperCoverage(t *testing.T) {
	t.Parallel()

	m := New(RoleAssistant, "hello")
	if m.Role != RoleAssistant || m.Content != "hello" {
		t.Fatalf("unexpected model: %+v", m)
	}
	if got := m.View(); got != "" {
		t.Fatalf("expected stub view to return empty string, got %q", got)
	}

	if wrapped := WrapUserMessage("hello world", 8); len(wrapped) == 0 {
		t.Fatal("expected wrapped user message lines")
	}
	if wrapped := WrapAssistantMessage("hello world", 8); len(wrapped) == 0 {
		t.Fatal("expected wrapped assistant message lines")
	}

	toolLines := WrapToolResult("alpha beta", 12)
	if !strings.Contains(strings.Join(toolLines, "\n"), "⎿") {
		t.Fatalf("expected tool result wrapper to include tree prefix, got %v", toolLines)
	}
}
