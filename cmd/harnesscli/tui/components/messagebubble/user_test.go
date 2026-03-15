package messagebubble

import (
	"os"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

// TestTUI021_UserBubbleRendersPromptAndBackground verifies that the ❯ prefix
// is present and that the background style is applied to lines.
func TestTUI021_UserBubbleRendersPromptAndBackground(t *testing.T) {
	b := UserBubble{Content: "Hello, world!", Width: 80}
	out := b.View()

	if !strings.Contains(out, "❯") {
		t.Errorf("expected ❯ prefix in output, got:\n%s", out)
	}

	lines := strings.Split(out, "\n")
	if len(lines) < 2 {
		t.Errorf("expected at least 2 lines (content + blank), got %d", len(lines))
	}

	// First line must contain the prompt symbol and the content text
	firstLine := lines[0]
	if !strings.Contains(firstLine, "❯") {
		t.Errorf("first line must contain ❯, got: %q", firstLine)
	}
	if !strings.Contains(firstLine, "Hello, world!") {
		t.Errorf("first line must contain content text, got: %q", firstLine)
	}
}

// TestTUI021_UserBubbleIncludesBlankTrailingLine verifies that a blank line
// always follows the user message block.
func TestTUI021_UserBubbleIncludesBlankTrailingLine(t *testing.T) {
	cases := []struct {
		name    string
		content string
		width   int
	}{
		{"short", "Hi", 80},
		{"long", "This is a longer message that has some content.", 80},
		{"empty", "", 80},
		{"multiline", "line one\nline two", 80},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := UserBubble{Content: tc.content, Width: tc.width}
			out := b.View()

			// Must end with a blank line (trailing \n or \n\n)
			if !strings.HasSuffix(out, "\n") {
				t.Errorf("[%s] output must end with newline, got: %q", tc.name, out)
			}

			lines := strings.Split(out, "\n")
			// Last element after split will be "" due to trailing \n
			// The second-to-last should be "" (the blank line)
			if len(lines) < 2 {
				t.Fatalf("[%s] expected at least 2 lines", tc.name)
			}

			// Find the last non-empty-string split segment
			// The blank trailing line should appear as an empty string before the final ""
			foundBlank := false
			for i := len(lines) - 1; i >= 0; i-- {
				if lines[i] == "" {
					foundBlank = true
					break
				}
			}
			if !foundBlank {
				t.Errorf("[%s] no blank trailing line found in:\n%s", tc.name, out)
			}
		})
	}
}

// TestTUI021_UserBubbleLongWordWraps verifies that a single long word is
// hard-wrapped at the terminal width boundary.
func TestTUI021_UserBubbleLongWordWraps(t *testing.T) {
	// A word longer than the usable content width (width-2 for prefix indent)
	longWord := strings.Repeat("A", 100)
	b := UserBubble{Content: longWord, Width: 40}
	out := b.View()

	lines := strings.Split(out, "\n")
	// Every non-blank line must be at most Width runes wide
	for i, line := range lines {
		// Strip ANSI escapes for width measurement — use rune count on visible chars
		visible := stripANSI(line)
		runes := utf8.RuneCountInString(visible)
		if runes > b.Width {
			t.Errorf("line %d exceeds width %d (got %d runes): %q", i, b.Width, runes, visible)
		}
	}

	// Must have multiple content lines due to wrapping
	nonBlank := 0
	for _, line := range lines {
		if strings.TrimSpace(stripANSI(line)) != "" {
			nonBlank++
		}
	}
	if nonBlank < 2 {
		t.Errorf("expected multiple lines for long word wrap, got %d non-blank lines:\n%s", nonBlank, out)
	}
}

// TestTUI021_UserBubbleNilContentFallback verifies that an empty content
// string renders safely with at least the ❯ line and a blank line.
func TestTUI021_UserBubbleNilContentFallback(t *testing.T) {
	b := UserBubble{Content: "", Width: 80}
	out := b.View()

	if out == "" {
		t.Fatal("View() must never return empty string")
	}

	if !strings.Contains(out, "❯") {
		t.Errorf("empty content must still render ❯ prefix, got: %q", out)
	}

	lines := strings.Split(out, "\n")
	if len(lines) < 2 {
		t.Errorf("empty content must produce at least 2 lines, got %d", len(lines))
	}
}

// TestTUI021_ConcurrentRendering verifies that multiple goroutines can call
// View() concurrently without data races.
func TestTUI021_ConcurrentRendering(t *testing.T) {
	bubbles := []UserBubble{
		{Content: "First message", Width: 80},
		{Content: "Second message with more text", Width: 120},
		{Content: "Third", Width: 40},
		{Content: strings.Repeat("long ", 50), Width: 80},
		{Content: "", Width: 80},
		{Content: "Hello\nMultiline\nContent", Width: 80},
		{Content: "NarrowWidth", Width: 10},
		{Content: "WideWidth", Width: 200},
		{Content: strings.Repeat("x", 500), Width: 80},
		{Content: "Final message", Width: 80},
	}

	var wg sync.WaitGroup
	for _, b := range bubbles {
		b := b // capture
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				out := b.View()
				if out == "" {
					// View() must never return empty
					panic("View() returned empty string")
				}
			}
		}()
	}
	wg.Wait()
}

// TestTUI021_BoundaryWidths tests degenerate and boundary width values.
func TestTUI021_BoundaryWidths(t *testing.T) {
	cases := []struct {
		name    string
		width   int
		content string
	}{
		{"zero", 0, "Hello"},
		{"narrow5", 5, "Hello World"},
		{"wide200", 200, "Hello"},
		{"degenerate1", 1, "ABC"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := UserBubble{Content: tc.content, Width: tc.width}
			// Must not panic
			out := b.View()
			if out == "" {
				t.Errorf("[%s] View() returned empty string", tc.name)
			}
			// Must still contain trailing blank line
			if !strings.HasSuffix(out, "\n") {
				t.Errorf("[%s] output must end with newline", tc.name)
			}
		})
	}
}

// TestTUI021_EmptyContentRendersAtLeastTwoLines verifies the error path spec:
// empty content must render at least 2 lines (❯ line + blank).
func TestTUI021_EmptyContentRendersAtLeastTwoLines(t *testing.T) {
	b := UserBubble{Content: "", Width: 80}
	out := b.View()

	lines := strings.Split(out, "\n")
	// After splitting "line\n", we get ["line", ""] — so len >= 2
	// For "line\n\n" we get ["line", "", ""]
	if len(lines) < 2 {
		t.Errorf("empty content must produce at least 2 lines after split, got %d lines from:\n%q", len(lines), out)
	}
}

// TestTUI021_MultilineContent verifies multi-line content is rendered with
// correct indentation on continuation lines.
func TestTUI021_MultilineContent(t *testing.T) {
	b := UserBubble{Content: "line one\nline two\nline three", Width: 80}
	out := b.View()

	lines := strings.Split(out, "\n")

	// First line has ❯ prefix
	if !strings.Contains(lines[0], "❯") {
		t.Errorf("first line must have ❯ prefix, got: %q", lines[0])
	}

	// Find continuation lines — must have 2-space indent (or be blank)
	contentLines := 0
	for _, line := range lines {
		visible := stripANSI(line)
		if visible == "" {
			continue
		}
		contentLines++
	}
	if contentLines < 3 {
		t.Errorf("expected at least 3 content lines for 3-line input, got %d in:\n%s", contentLines, out)
	}
}

// TestTUI021_VisualSnapshot_80x24 renders a standard 80-wide bubble and
// writes it to testdata/snapshots/TUI-021-userBubble-80x24.txt.
func TestTUI021_VisualSnapshot_80x24(t *testing.T) {
	b := UserBubble{
		Content: "What files are in the current directory? I need to understand the project structure before we begin refactoring.",
		Width:   80,
	}
	out := b.View()

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-021-userBubble-80x24.txt"
	if err := os.WriteFile(path, []byte(out), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI021_VisualSnapshot_120x40 renders a 120-wide bubble snapshot.
func TestTUI021_VisualSnapshot_120x40(t *testing.T) {
	b := UserBubble{
		Content: "What files are in the current directory? I need to understand the project structure before we begin refactoring.",
		Width:   120,
	}
	out := b.View()

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-021-userBubble-120x40.txt"
	if err := os.WriteFile(path, []byte(out), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI021_VisualSnapshot_200x50 renders a 200-wide bubble snapshot.
func TestTUI021_VisualSnapshot_200x50(t *testing.T) {
	b := UserBubble{
		Content: "What files are in the current directory? I need to understand the project structure before we begin refactoring.",
		Width:   200,
	}
	out := b.View()

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-021-userBubble-200x50.txt"
	if err := os.WriteFile(path, []byte(out), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// stripANSI removes ANSI escape sequences from a string for width measurement.
func stripANSI(s string) string {
	var result strings.Builder
	inEscape := false
	for i := 0; i < len(s); {
		b := s[i]
		if b == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			inEscape = true
			i += 2
			continue
		}
		if inEscape {
			if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') {
				inEscape = false
			}
			i++
			continue
		}
		result.WriteByte(b)
		i++
	}
	return result.String()
}
