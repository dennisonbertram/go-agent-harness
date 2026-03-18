package sessionpicker

import "testing"

func TestTruncateStrCoverage(t *testing.T) {
	t.Parallel()

	if got := truncateStr("hello", 0); got != "" {
		t.Fatalf("expected empty string for width 0, got %q", got)
	}
	if got := truncateStr("hello", 1); got != "…" {
		t.Fatalf("expected ellipsis for width 1, got %q", got)
	}
	if got := truncateStr("hello", 4); got != "hel…" {
		t.Fatalf("expected truncated string, got %q", got)
	}
	if got := truncateStr("hi", 4); got != "hi" {
		t.Fatalf("expected untouched short string, got %q", got)
	}
}
