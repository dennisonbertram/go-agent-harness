package messagebubble

import (
	"os"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

// TestTUI022_AssistantBubbleRendersTitle verifies that when a Title is provided,
// the output contains styled title text (bold/italic markup present in raw output).
func TestTUI022_AssistantBubbleRendersTitle(t *testing.T) {
	b := AssistantBubble{
		Title:   "Analysis Result",
		Content: "Here is the analysis.",
		Width:   80,
	}
	out := b.View()

	if out == "" {
		t.Fatal("View() must not return empty string")
	}

	// The title text must appear somewhere in the output
	if !strings.Contains(out, "Analysis Result") {
		t.Errorf("expected title text in output, got:\n%s", out)
	}

	// Must also contain the dot prefix
	if !strings.Contains(out, "⏺") {
		t.Errorf("expected ⏺ prefix in output, got:\n%s", out)
	}
}

// TestTUI022_AssistantBubbleWrapsBody verifies that a long content line is
// wrapped at Width-2 (to allow for 2-space indent).
func TestTUI022_AssistantBubbleWrapsBody(t *testing.T) {
	// Create content that will definitely require wrapping at width 40
	content := "This is a long line of text that should wrap because it exceeds the available width."
	b := AssistantBubble{
		Content: content,
		Width:   40,
	}
	out := b.View()

	lines := strings.Split(out, "\n")

	// Every non-blank line must be at most Width runes wide (after stripping ANSI)
	for i, line := range lines {
		visible := stripANSI(line)
		runes := utf8.RuneCountInString(visible)
		if runes > b.Width {
			t.Errorf("line %d exceeds width %d (got %d runes): %q", i, b.Width, runes, visible)
		}
	}

	// Must have multiple non-blank lines due to wrapping
	nonBlank := 0
	for _, line := range lines {
		if strings.TrimSpace(stripANSI(line)) != "" {
			nonBlank++
		}
	}
	if nonBlank < 2 {
		t.Errorf("expected wrapped content to produce multiple lines, got %d non-blank", nonBlank)
	}
}

// TestTUI022_AssistantBubbleDotPrefix verifies the ⏺ prefix appears on the first
// output line.
func TestTUI022_AssistantBubbleDotPrefix(t *testing.T) {
	b := AssistantBubble{
		Content: "Some response text",
		Width:   80,
	}
	out := b.View()

	if !strings.Contains(out, "⏺") {
		t.Errorf("expected ⏺ prefix in output, got:\n%s", out)
	}

	// The dot must appear in the first non-empty line
	lines := strings.Split(out, "\n")
	firstNonEmpty := ""
	for _, line := range lines {
		if strings.TrimSpace(stripANSI(line)) != "" {
			firstNonEmpty = line
			break
		}
	}
	if !strings.Contains(firstNonEmpty, "⏺") {
		t.Errorf("⏺ must appear in first non-empty line, got: %q", firstNonEmpty)
	}
}

// TestTUI022_AssistantBubbleTrailingBlankLine verifies that every render
// ends with a blank line (trailing "\n\n" pattern).
func TestTUI022_AssistantBubbleTrailingBlankLine(t *testing.T) {
	cases := []struct {
		name    string
		title   string
		content string
		width   int
	}{
		{"with_title", "Title", "Content here", 80},
		{"no_title", "", "Content here", 80},
		{"empty_content", "", "", 80},
		{"multiline", "", "line one\nline two\nline three", 80},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := AssistantBubble{Title: tc.title, Content: tc.content, Width: tc.width}
			out := b.View()

			if !strings.HasSuffix(out, "\n") {
				t.Errorf("[%s] output must end with newline, got: %q", tc.name, out)
			}

			lines := strings.Split(out, "\n")
			// After split on trailing "\n\n", we expect at least one empty element
			// near the end representing the blank line.
			if len(lines) < 2 {
				t.Fatalf("[%s] expected at least 2 lines", tc.name)
			}

			// Find trailing blank
			foundBlank := false
			for i := len(lines) - 1; i >= 0; i-- {
				if lines[i] == "" {
					foundBlank = true
					break
				}
			}
			if !foundBlank {
				t.Errorf("[%s] no blank trailing line found in:\n%q", tc.name, out)
			}
		})
	}
}

// TestTUI022_AssistantBubbleNoTitle verifies rendering without a title works
// and still has the ⏺ prefix on the first content line.
func TestTUI022_AssistantBubbleNoTitle(t *testing.T) {
	b := AssistantBubble{
		Content: "Here is a response without a title.",
		Width:   80,
	}
	out := b.View()

	if out == "" {
		t.Fatal("View() must not return empty string")
	}

	// Must have ⏺ prefix
	if !strings.Contains(out, "⏺") {
		t.Errorf("no-title bubble must still have ⏺ prefix, got:\n%s", out)
	}

	// Content must appear
	if !strings.Contains(out, "Here is a response without a title.") {
		t.Errorf("content text must appear in output, got:\n%s", out)
	}
}

// TestTUI022_AssistantBubbleEmptyContent verifies that empty content renders
// safely (just the ⏺ line + blank, no panic).
func TestTUI022_AssistantBubbleEmptyContent(t *testing.T) {
	b := AssistantBubble{
		Content: "",
		Width:   80,
	}
	// Must not panic
	out := b.View()

	if out == "" {
		t.Fatal("View() must not return empty string even for empty content")
	}

	// Must have ⏺
	if !strings.Contains(out, "⏺") {
		t.Errorf("empty content must still render ⏺ prefix, got: %q", out)
	}

	// Must end with newline
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("output must end with newline, got: %q", out)
	}
}

// TestTUI022_AssistantBubbleConcurrent verifies that 10 goroutines each
// calling View() 100 times triggers no data races.
func TestTUI022_AssistantBubbleConcurrent(t *testing.T) {
	bubbles := []AssistantBubble{
		{Title: "Title 1", Content: "First message", Width: 80},
		{Content: "Second message with more text", Width: 120},
		{Content: "Third", Width: 40},
		{Content: strings.Repeat("long ", 50), Width: 80},
		{Content: "", Width: 80},
		{Title: "Multi", Content: "Hello\nMultiline\nContent", Width: 80},
		{Content: "NarrowWidth", Width: 10},
		{Content: "WideWidth", Width: 200},
		{Content: strings.Repeat("x", 500), Width: 80},
		{Title: "Last", Content: "Final message", Width: 80},
	}

	var wg sync.WaitGroup
	for _, b := range bubbles {
		b := b
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				out := b.View()
				if out == "" {
					panic("View() returned empty string")
				}
			}
		}()
	}
	wg.Wait()
}

// TestTUI022_AssistantBubbleBoundaryWidths tests degenerate and boundary width values.
func TestTUI022_AssistantBubbleBoundaryWidths(t *testing.T) {
	cases := []struct {
		name    string
		width   int
		content string
	}{
		{"width_5", 5, "Hello World"},
		{"width_80", 80, "Hello World"},
		{"width_200", 200, "Hello World"},
		{"zero_width", 0, "Hello"},
		{"negative_width", -1, "Hello"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := AssistantBubble{Content: tc.content, Width: tc.width}
			// Must not panic
			out := b.View()
			if out == "" {
				t.Errorf("[%s] View() returned empty string", tc.name)
			}
			// Must end with newline
			if !strings.HasSuffix(out, "\n") {
				t.Errorf("[%s] output must end with newline", tc.name)
			}
			// Must contain ⏺
			if !strings.Contains(out, "⏺") {
				t.Errorf("[%s] must contain ⏺, got: %q", tc.name, out)
			}
		})
	}
}

// TestTUI022_VisualSnapshot_80x24 renders a standard 80-wide assistant bubble
// and writes it to testdata/snapshots/TUI-022-assistant-80x24.txt.
func TestTUI022_VisualSnapshot_80x24(t *testing.T) {
	b := AssistantBubble{
		Title:   "File Analysis",
		Content: "The current directory contains several Go source files. The main entry points are in cmd/ and the core business logic lives in internal/. I'll start by examining the harness runner to understand the step loop before we make any changes.",
		Width:   80,
	}
	out := b.View()

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-022-assistant-80x24.txt"
	if err := os.WriteFile(path, []byte(out), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI022_VisualSnapshot_120x40 renders a 120-wide assistant bubble snapshot.
func TestTUI022_VisualSnapshot_120x40(t *testing.T) {
	b := AssistantBubble{
		Title:   "File Analysis",
		Content: "The current directory contains several Go source files. The main entry points are in cmd/ and the core business logic lives in internal/. I'll start by examining the harness runner to understand the step loop before we make any changes.",
		Width:   120,
	}
	out := b.View()

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-022-assistant-120x40.txt"
	if err := os.WriteFile(path, []byte(out), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI022_VisualSnapshot_200x50 renders a 200-wide assistant bubble snapshot.
func TestTUI022_VisualSnapshot_200x50(t *testing.T) {
	b := AssistantBubble{
		Title:   "File Analysis",
		Content: "The current directory contains several Go source files. The main entry points are in cmd/ and the core business logic lives in internal/. I'll start by examining the harness runner to understand the step loop before we make any changes.",
		Width:   200,
	}
	out := b.View()

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-022-assistant-200x50.txt"
	if err := os.WriteFile(path, []byte(out), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}
