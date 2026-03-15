package streamrenderer

import (
	"strings"
	"testing"
)

// TestEmphasisNone verifies that EmphasisNone is a no-op and returns the same string.
func TestEmphasisNone(t *testing.T) {
	input := "hello world"
	out := ApplyEmphasis(input, EmphasisNone)
	// No-op: the visible text must be unchanged (ANSI stripped)
	visible := stripANSIForTest(out)
	if visible != input {
		t.Errorf("EmphasisNone: expected %q, got %q (raw: %q)", input, visible, out)
	}
}

// TestEmphasisBold verifies that EmphasisBold produces non-empty styled output
// and the visible text matches the input.
func TestEmphasisBold(t *testing.T) {
	input := "bold text"
	out := ApplyEmphasis(input, EmphasisBold)

	if out == "" {
		t.Fatal("ApplyEmphasis returned empty string")
	}

	// Visible text must match
	visible := stripANSIForTest(out)
	if visible != input {
		t.Errorf("EmphasisBold: expected visible %q, got %q", input, visible)
	}
}

// TestEmphasisItalic verifies that EmphasisItalic produces output.
func TestEmphasisItalic(t *testing.T) {
	input := "italic text"
	out := ApplyEmphasis(input, EmphasisItalic)

	if out == "" {
		t.Fatal("ApplyEmphasis returned empty string")
	}

	visible := stripANSIForTest(out)
	if visible != input {
		t.Errorf("EmphasisItalic: expected visible %q, got %q", input, visible)
	}
}

// TestEmphasisUnderline verifies that EmphasisUnderline produces output.
func TestEmphasisUnderline(t *testing.T) {
	input := "underlined text"
	out := ApplyEmphasis(input, EmphasisUnderline)

	if out == "" {
		t.Fatal("ApplyEmphasis returned empty string")
	}

	visible := stripANSIForTest(out)
	if visible != input {
		t.Errorf("EmphasisUnderline: expected visible %q, got %q", input, visible)
	}
}

// TestEmphasisBoldItalic verifies that EmphasisBoldItalic produces output.
func TestEmphasisBoldItalic(t *testing.T) {
	input := "bold italic text"
	out := ApplyEmphasis(input, EmphasisBoldItalic)

	if out == "" {
		t.Fatal("ApplyEmphasis returned empty string")
	}

	visible := stripANSIForTest(out)
	if visible != input {
		t.Errorf("EmphasisBoldItalic: expected visible %q, got %q", input, visible)
	}
}

// TestEmphasisCode verifies that EmphasisCode applies code style and
// the visible text matches the input.
func TestEmphasisCode(t *testing.T) {
	input := "some_code_here"
	out := ApplyEmphasis(input, EmphasisCode)

	if out == "" {
		t.Fatal("ApplyEmphasis returned empty string")
	}

	visible := stripANSIForTest(out)
	if visible != input {
		t.Errorf("EmphasisCode: expected visible %q, got %q", input, visible)
	}
}

// TestApplyEmphasisEmptyInput verifies empty string is handled safely.
func TestApplyEmphasisEmptyInput(t *testing.T) {
	emphases := []Emphasis{EmphasisNone, EmphasisBold, EmphasisItalic, EmphasisUnderline, EmphasisBoldItalic, EmphasisCode}
	for _, e := range emphases {
		out := ApplyEmphasis("", e)
		// Must not panic; result may be empty or styled empty string
		_ = out
	}
}

// stripANSIForTest removes ANSI escape sequences for visible-text comparison.
func stripANSIForTest(s string) string {
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
