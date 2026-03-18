package tui_test

import (
	"strings"
	"testing"

	tui "go-agent-harness/cmd/harnesscli/tui"
)

func TestTUI006_SeparatorFor(t *testing.T) {
	t.Parallel()

	th := tui.DefaultTheme()
	if got := th.SeparatorFor(0); got != "" {
		t.Fatalf("expected empty separator for width 0, got %q", got)
	}

	got := th.SeparatorFor(4)
	if !strings.Contains(got, "────") {
		t.Fatalf("expected rendered separator line, got %q", got)
	}
}
