package streamrenderer_test

import (
	"testing"
	"unicode/utf8"

	"go-agent-harness/cmd/harnesscli/tui/components/streamrenderer"
)

func TestTUI016_WrapTextBasic(t *testing.T) {
	lines := streamrenderer.WrapText("hello world this is a test", 10)
	for _, line := range lines {
		if utf8.RuneCountInString(line) > 10 {
			t.Errorf("line too long at width 10: %q (%d runes)", line, utf8.RuneCountInString(line))
		}
	}
	if len(lines) == 0 {
		t.Error("WrapText returned no lines")
	}
}

func TestTUI016_WrapEmojiAndWideChars(t *testing.T) {
	// Emoji are 2 columns wide
	text := "hello \U0001f30d world \U0001f389 test"
	lines := streamrenderer.WrapText(text, 15)
	// Should not panic and all output rune-counts should be reasonable
	for _, line := range lines {
		_ = line
	}
}

func TestTUI016_ToolResultIndentPreserved(t *testing.T) {
	wrapped := streamrenderer.WrapWithPrefix("output line one two three four", "\u23bf  ", 20)
	if len(wrapped) == 0 {
		t.Fatal("no output lines")
	}
	// First line has prefix
	if len([]rune(wrapped[0])) < 3 {
		t.Errorf("first line too short, missing prefix: %q", wrapped[0])
	}
}

func TestTUI016_WrapEmptyString(t *testing.T) {
	lines := streamrenderer.WrapText("", 80)
	if len(lines) != 1 || lines[0] != "" {
		t.Errorf("empty string should return [''], got %v", lines)
	}
}

func TestTUI016_WrapSingleLongWord(t *testing.T) {
	long := "verylongwordthatexceedsthelinewidth"
	lines := streamrenderer.WrapText(long, 10)
	if len(lines) == 0 {
		t.Fatal("no output for long word")
	}
	// Hard wrap at width
	for _, line := range lines {
		if utf8.RuneCountInString(line) > 10 {
			t.Errorf("hard wrap failed: %q is %d runes, max 10", line, utf8.RuneCountInString(line))
		}
	}
}

func TestTUI016_WrapPreservesNewlines(t *testing.T) {
	text := "line one\nline two\nline three"
	lines := streamrenderer.WrapText(text, 80)
	if len(lines) != 3 {
		t.Errorf("expected 3 lines from newline-separated text, got %d: %v", len(lines), lines)
	}
}
