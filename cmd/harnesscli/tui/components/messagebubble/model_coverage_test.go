package messagebubble

import (
	"strings"
	"testing"
)

func TestModelViewRendersUserAndAssistantRoles(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		role     Role
		content  string
		wantText string
	}{
		{name: "user", role: RoleUser, content: "hello from user", wantText: "hello from user"},
		{name: "assistant", role: RoleAssistant, content: "hello from assistant", wantText: "hello from assistant"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			m := New(tc.role, tc.content)
			m.Width = 80

			got := m.View()
			if got == "" {
				t.Fatal("View() must return non-empty output")
			}
			if !strings.Contains(stripANSI(got), tc.wantText) {
				t.Fatalf("View() = %q, want content %q", got, tc.wantText)
			}
		})
	}
}

func TestModelViewAssistantMarkdownUsesComponentRenderer(t *testing.T) {
	orig := MarkdownEnabled
	defer func() { MarkdownEnabled = orig }()

	markdown := New(RoleAssistant, "# Heading\n\nThis is **bold**")
	markdown.Width = 80

	MarkdownEnabled = true
	withMarkdown := stripANSI(markdown.View())
	if withMarkdown == "" {
		t.Fatal("View() must return non-empty output for assistant markdown")
	}
	if !strings.Contains(withMarkdown, "Heading") {
		t.Fatalf("rendered assistant markdown missing heading text: %q", withMarkdown)
	}

	MarkdownEnabled = false
	withoutMarkdown := stripANSI(markdown.View())
	if withoutMarkdown == "" {
		t.Fatal("View() must return non-empty output when markdown is disabled")
	}
	if withMarkdown == withoutMarkdown {
		t.Fatalf("assistant messagebubble path must invoke markdown rendering when enabled; output unchanged:\n%s", withMarkdown)
	}
}

func TestModelViewUserKeepsPromptPrefixStylingContract(t *testing.T) {
	m := New(RoleUser, "hello user bubble")
	m.Width = 60

	got := stripANSI(m.View())
	if got == "" {
		t.Fatal("View() must return non-empty output for user bubble")
	}
	if !strings.Contains(got, "❯ hello user bubble") {
		t.Fatalf("user bubble must keep the prompt prefix contract, got %q", got)
	}
}

func TestModelAndWrapperCoverage(t *testing.T) {
	t.Parallel()

	tool := New(RoleTool, "alpha beta")
	tool.Width = 24
	if got := tool.View(); !strings.Contains(got, "⎿") {
		t.Fatalf("expected tool bubble to include tree prefix, got %q", got)
	}

	if wrapped := WrapUserMessage("hello world", 8); len(wrapped) == 0 {
		t.Fatal("expected wrapped user message lines")
	}
	if wrapped := WrapAssistantMessage("hello world", 8); len(wrapped) == 0 {
		t.Fatal("expected wrapped assistant message lines")
	}
	if wrapped := WrapToolResult("alpha beta", 12); len(wrapped) == 0 {
		t.Fatal("expected wrapped tool result lines")
	}
}
