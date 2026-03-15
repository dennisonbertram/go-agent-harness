package streamrenderer_test

import (
	"strings"
	"testing"

	"go-agent-harness/cmd/harnesscli/tui/components/streamrenderer"
)

// TestTUI025_StreamRendererSpinnerSummary verifies that SpinnerSummary()
// returns a "✻ Worked for..." formatted string when in Complete state.
func TestTUI025_StreamRendererSpinnerSummary(t *testing.T) {
	sr := streamrenderer.New()
	sr.AppendDelta("Some response text.")
	sr.Complete(500, 3.7)

	summary := sr.SpinnerSummary()
	if summary == "" {
		t.Fatal("SpinnerSummary() should return non-empty string in Complete state")
	}
	if !strings.Contains(summary, "Worked for") {
		t.Errorf("SpinnerSummary() should contain 'Worked for', got: %q", summary)
	}
	// Should contain the glyph ✻ (the canonical completion glyph).
	if !strings.Contains(summary, "✻") {
		t.Errorf("SpinnerSummary() should contain '✻', got: %q", summary)
	}
}

// TestTUI025_StreamRendererSpinnerSummaryIdleReturnsEmpty verifies that
// SpinnerSummary() returns "" when the renderer is in Idle state.
func TestTUI025_StreamRendererSpinnerSummaryIdleReturnsEmpty(t *testing.T) {
	sr := streamrenderer.New()

	summary := sr.SpinnerSummary()
	if summary != "" {
		t.Errorf("SpinnerSummary() should return empty string in Idle state, got: %q", summary)
	}
}
