package slashcomplete_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"go-agent-harness/cmd/harnesscli/tui/components/slashcomplete"
)

// Issue #1401: the dropdown must read correctly to a first-time user.

func polishSuggestions() []slashcomplete.Suggestion {
	return []slashcomplete.Suggestion{
		{Name: "add-dir", Description: "Attach an extra directory to the session (/add-dir [remove] <path>)"},
		{Name: "clear", Description: "Clear conversation history"},
		{Name: "help", Description: "Show help dialog"},
	}
}

func TestView_NoMatchRow(t *testing.T) {
	m := slashcomplete.New(polishSuggestions()).Open().SetQuery("zzz")
	out := m.View(80)
	if !strings.Contains(out, "No matching commands") {
		t.Fatalf("no-match query must render a hint row, got %q", out)
	}
}

func TestView_EllipsisTruncation(t *testing.T) {
	m := slashcomplete.New(polishSuggestions()).Open()
	out := m.View(40)
	for _, line := range strings.Split(out, "\n") {
		if w := lipgloss.Width(line); w > 40 {
			t.Errorf("line wider than terminal (%d > 40): %q", w, line)
		}
	}
	if !strings.Contains(out, "…") {
		t.Errorf("long description must be truncated with an ellipsis at width 40, got:\n%s", out)
	}
	if !strings.Contains(out, "/add-dir") {
		t.Errorf("the command name must never be cut, got:\n%s", out)
	}
}

func TestView_NoTrailingNewline(t *testing.T) {
	out := slashcomplete.New(polishSuggestions()).Open().View(80)
	if strings.HasSuffix(out, "\n") {
		t.Fatalf("View must not end with a newline (it produces a blank row in the screen stack)")
	}
}

func TestView_FooterHint(t *testing.T) {
	out := slashcomplete.New(polishSuggestions()).Open().View(80)
	for _, want := range []string{"↑↓", "Enter", "Tab", "Esc"} {
		if !strings.Contains(out, want) {
			t.Errorf("footer hint must mention %q, got:\n%s", want, out)
		}
	}
}

func TestHasUserChoice(t *testing.T) {
	m := slashcomplete.New(polishSuggestions()).Open().SetQuery("")
	if m.HasUserChoice() {
		t.Fatal("bare '/' with no navigation is not a choice")
	}
	if !m.Down().HasUserChoice() {
		t.Fatal("navigating with Down is a choice")
	}
	if !m.SetQuery("he").HasUserChoice() {
		t.Fatal("typing a query is a choice")
	}
	if m.Down().SetQuery("").HasUserChoice() {
		t.Fatal("clearing the query resets the choice")
	}
}
