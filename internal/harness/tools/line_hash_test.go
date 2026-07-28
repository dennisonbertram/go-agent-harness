package tools

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// LineHash unit tests
//
// The read/edit hash_lines and start_line_hash/end_line_hash behavior tests
// that used to live here were ported to internal/harness/tools/core, since
// the read/edit tools now live there (see core/line_hash_test.go).
// ---------------------------------------------------------------------------

func TestLineHashReturns12HexChars(t *testing.T) {
	t.Parallel()
	h := LineHash("hello world")
	if len(h) != 12 {
		t.Errorf("expected 12-char hash, got %q (len=%d)", h, len(h))
	}
	for _, c := range h {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("non-hex char %q in hash %q", c, h)
		}
	}
}

func TestLineHashIsDeterministic(t *testing.T) {
	t.Parallel()
	h1 := LineHash("foo bar")
	h2 := LineHash("foo bar")
	if h1 != h2 {
		t.Errorf("hash not deterministic: %q != %q", h1, h2)
	}
}

func TestLineHashTrimsTrailingWhitespace(t *testing.T) {
	t.Parallel()
	h1 := LineHash("hello")
	h2 := LineHash("hello   ")
	h3 := LineHash("hello\t")
	h4 := LineHash("hello\r")
	if h1 != h2 {
		t.Errorf("trailing spaces should not affect hash: %q != %q", h1, h2)
	}
	if h1 != h3 {
		t.Errorf("trailing tab should not affect hash: %q != %q", h1, h3)
	}
	if h1 != h4 {
		t.Errorf("trailing CR should not affect hash: %q != %q", h1, h4)
	}
}

func TestLineHashDifferentForDifferentContent(t *testing.T) {
	t.Parallel()
	h1 := LineHash("line one")
	h2 := LineHash("line two")
	if h1 == h2 {
		t.Errorf("expected different hashes for different content, got same: %q", h1)
	}
}

func TestLineHashEmptyString(t *testing.T) {
	t.Parallel()
	h := LineHash("")
	if len(h) != 12 {
		t.Errorf("expected 12-char hash for empty string, got %q (len=%d)", h, len(h))
	}
}
