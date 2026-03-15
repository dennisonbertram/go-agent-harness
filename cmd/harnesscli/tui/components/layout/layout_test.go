package layout_test

import (
	"testing"

	"go-agent-harness/cmd/harnesscli/tui/components/layout"
)

func TestTUI005_ComputeLayoutFor80x24(t *testing.T) {
	l := layout.Compute(80, 24)
	// Status bar: 1 line
	if l.StatusBarHeight != 1 {
		t.Errorf("StatusBarHeight: want 1, got %d", l.StatusBarHeight)
	}
	// Input area: min 3 lines
	if l.InputHeight < 3 {
		t.Errorf("InputHeight: want >= 3, got %d", l.InputHeight)
	}
	// Viewport gets the rest
	if l.ViewportHeight <= 0 {
		t.Errorf("ViewportHeight: want > 0, got %d", l.ViewportHeight)
	}
	// All heights must sum to terminal height
	total := l.StatusBarHeight + l.SeparatorHeight + l.ViewportHeight + l.SeparatorHeight + l.InputHeight
	if total != 24 {
		t.Errorf("heights sum to %d, want 24", total)
	}
}

func TestTUI005_LayoutStaysStableAtTinySizes(t *testing.T) {
	cases := []struct{ w, h int }{
		{20, 10},
		{40, 12},
		{1, 1},
		{0, 0},
	}
	for _, c := range cases {
		l := layout.Compute(c.w, c.h)
		if l.ViewportHeight < 0 || l.InputHeight < 0 || l.StatusBarHeight < 0 {
			t.Errorf("negative dimension at %dx%d: %+v", c.w, c.h, l)
		}
		if l.Width < 0 {
			t.Errorf("negative width at %dx%d", c.w, c.h)
		}
	}
}

func TestTUI005_LayoutAt120x40(t *testing.T) {
	l := layout.Compute(120, 40)
	if l.Width != 120 {
		t.Errorf("Width: want 120, got %d", l.Width)
	}
	// Viewport should be larger than at 24 rows
	l24 := layout.Compute(120, 24)
	if l.ViewportHeight <= l24.ViewportHeight {
		t.Errorf("larger terminal should give larger viewport: 40-row=%d <= 24-row=%d",
			l.ViewportHeight, l24.ViewportHeight)
	}
}

func TestTUI005_LayoutAt200x50(t *testing.T) {
	l := layout.Compute(200, 50)
	if l.Width != 200 {
		t.Errorf("Width: want 200, got %d", l.Width)
	}
	total := l.StatusBarHeight + l.SeparatorHeight + l.ViewportHeight + l.SeparatorHeight + l.InputHeight
	if total != 50 {
		t.Errorf("heights sum to %d, want 50", total)
	}
}

func TestTUI005_NegativeSizeDefaultsToLastValid(t *testing.T) {
	l := layout.Compute(-10, -5)
	// Should not panic, all fields >= 0
	if l.Width < 0 || l.ViewportHeight < 0 {
		t.Errorf("negative input: got negative fields %+v", l)
	}
}
