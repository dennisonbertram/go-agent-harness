package viewport_test

import (
	"fmt"
	"strings"
	"testing"

	"go-agent-harness/cmd/harnesscli/tui/components/viewport"
)

func TestTUI017_AutoScrollPinsToBottomOnAppend(t *testing.T) {
	vp := viewport.New(80, 5)
	for i := 0; i < 10; i++ {
		vp.AppendLine(fmt.Sprintf("line %d", i))
	}
	// With autoScroll=true, should always show last 5 lines
	view := vp.View()
	if !strings.Contains(view, "line 9") {
		t.Errorf("auto-scroll should pin to latest line, view: %q", view)
	}
	if strings.Contains(view, "line 0") {
		t.Errorf("auto-scroll should not show old lines: %q", view)
	}
}

func TestTUI017_ManualScrollStopsAutoScroll(t *testing.T) {
	vp := viewport.New(80, 5)
	for i := 0; i < 10; i++ {
		vp.AppendLine(fmt.Sprintf("line %d", i))
	}
	vp.ScrollUp(3)
	// Auto-scroll off — new append should not jump to bottom
	vp.AppendLine("line 10 NEW")
	view := vp.View()
	if strings.Contains(view, "line 10 NEW") {
		t.Errorf("manual scroll should prevent auto-jump to new content: %q", view)
	}
}

func TestTUI017_NewContentIndicatorAppearsWhenScrolledUp(t *testing.T) {
	vp := viewport.New(80, 5)
	for i := 0; i < 10; i++ {
		vp.AppendLine(fmt.Sprintf("line %d", i))
	}
	vp.ScrollUp(3)
	vp.AppendLine("new message while scrolled up")
	// NewContentIndicator should show number of new lines
	indicator := vp.NewContentIndicator()
	if indicator == "" {
		t.Error("NewContentIndicator should return non-empty string when scrolled up with new content")
	}
	if !strings.Contains(indicator, "▼") {
		t.Errorf("indicator should contain ▼, got: %q", indicator)
	}
}

func TestTUI017_IndicatorClearsOnScrollToBottom(t *testing.T) {
	vp := viewport.New(80, 5)
	for i := 0; i < 10; i++ {
		vp.AppendLine(fmt.Sprintf("line %d", i))
	}
	vp.ScrollUp(3)
	vp.AppendLine("new content")
	vp.ScrollToBottom()
	indicator := vp.NewContentIndicator()
	if indicator != "" {
		t.Errorf("indicator should clear after scroll to bottom, got: %q", indicator)
	}
}

func TestTUI017_RapidAppendWhileScrolledUp(t *testing.T) {
	vp := viewport.New(80, 5)
	for i := 0; i < 5; i++ {
		vp.AppendLine(fmt.Sprintf("line %d", i))
	}
	vp.ScrollUp(2)
	// Rapid appends
	for i := 5; i < 50; i++ {
		vp.AppendLine(fmt.Sprintf("line %d", i))
	}
	// Should not panic, offset should be clamped
	_ = vp.View()
	_ = vp.NewContentIndicator()
}
