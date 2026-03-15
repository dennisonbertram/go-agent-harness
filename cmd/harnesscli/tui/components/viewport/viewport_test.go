package viewport_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"go-agent-harness/cmd/harnesscli/tui/components/viewport"
)

func TestTUI013_ViewportAppendsAndKeepsOrder(t *testing.T) {
	vp := viewport.New(80, 20)
	vp.AppendLine("first line")
	vp.AppendLine("second line")
	vp.AppendLine("third line")
	view := vp.View()
	lines := strings.Split(view, "\n")
	// All lines should appear in order
	firstIdx, secondIdx := -1, -1
	for i, l := range lines {
		if strings.Contains(l, "first line") {
			firstIdx = i
		}
		if strings.Contains(l, "second line") {
			secondIdx = i
		}
	}
	if firstIdx == -1 || secondIdx == -1 {
		t.Errorf("lines missing from view: %q", view)
	}
	if firstIdx >= secondIdx {
		t.Errorf("line order wrong: first=%d second=%d", firstIdx, secondIdx)
	}
}

func TestTUI013_UserManualScrollPausesAutoScroll(t *testing.T) {
	vp := viewport.New(80, 5) // small viewport
	for i := 0; i < 20; i++ {
		vp.AppendLine(fmt.Sprintf("line %d", i))
	}
	// Scroll up manually
	vp.ScrollUp(3)
	atBottom := vp.AtBottom()
	if atBottom {
		t.Error("after scrolling up, AtBottom() should be false")
	}
	// Auto-scroll should be paused
	if vp.AutoScrollEnabled() {
		t.Error("auto-scroll should be disabled after manual scroll")
	}
}

func TestTUI013_ScrollToBottomReEnablesAutoScroll(t *testing.T) {
	vp := viewport.New(80, 5)
	for i := 0; i < 20; i++ {
		vp.AppendLine(fmt.Sprintf("line %d", i))
	}
	vp.ScrollUp(5)
	vp.ScrollToBottom()
	if !vp.AtBottom() {
		t.Error("after ScrollToBottom, should be at bottom")
	}
	if !vp.AutoScrollEnabled() {
		t.Error("auto-scroll should re-enable after jumping to bottom")
	}
}

func TestTUI013_ViewportHeightRespected(t *testing.T) {
	vp := viewport.New(80, 5)
	for i := 0; i < 30; i++ {
		vp.AppendLine(fmt.Sprintf("line %d", i))
	}
	view := vp.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) > 5 {
		t.Errorf("viewport shows %d lines, max is 5", len(lines))
	}
}

func TestTUI013_NewContentIndicatorWhenNotAtBottom(t *testing.T) {
	vp := viewport.New(80, 5)
	for i := 0; i < 10; i++ {
		vp.AppendLine(fmt.Sprintf("line %d", i))
	}
	vp.ScrollUp(3)
	// Append new content while scrolled up
	vp.AppendLine("new content after scroll")
	// Should show new-content indicator
	if !vp.HasNewContent() {
		t.Error("HasNewContent should be true when scrolled up and new lines exist")
	}
	vp.ScrollToBottom()
	if vp.HasNewContent() {
		t.Error("HasNewContent should be false after scrolling to bottom")
	}
}

func TestTUI013_KeyPageUpScrollsUp(t *testing.T) {
	vp := viewport.New(80, 5)
	for i := 0; i < 20; i++ {
		vp.AppendLine(fmt.Sprintf("line %d", i))
	}
	before := vp.ScrollOffset()
	vp2, _ := vp.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	after := vp2.ScrollOffset()
	if after <= before {
		t.Errorf("PageUp should increase scroll offset: before=%d after=%d", before, after)
	}
}

func TestTUI013_KeyPageDownScrollsDown(t *testing.T) {
	vp := viewport.New(80, 5)
	for i := 0; i < 20; i++ {
		vp.AppendLine(fmt.Sprintf("line %d", i))
	}
	vp.ScrollUp(10)
	before := vp.ScrollOffset()
	vp2, _ := vp.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	after := vp2.ScrollOffset()
	if after >= before {
		t.Errorf("PageDown should decrease scroll offset: before=%d after=%d", before, after)
	}
}

func TestTUI013_ScrollUpClamps(t *testing.T) {
	vp := viewport.New(80, 5)
	for i := 0; i < 10; i++ {
		vp.AppendLine(fmt.Sprintf("line %d", i))
	}
	// Try to scroll way past the top
	vp.ScrollUp(1000)
	maxOff := 10 - 5 // total lines - height
	if vp.ScrollOffset() > maxOff {
		t.Errorf("scroll offset %d exceeds max %d", vp.ScrollOffset(), maxOff)
	}
}

func TestTUI013_ScrollDownClamps(t *testing.T) {
	vp := viewport.New(80, 5)
	for i := 0; i < 10; i++ {
		vp.AppendLine(fmt.Sprintf("line %d", i))
	}
	vp.ScrollUp(3)
	vp.ScrollDown(100)
	if vp.ScrollOffset() != 0 {
		t.Errorf("scroll offset should be 0 after scrolling down past bottom, got %d", vp.ScrollOffset())
	}
	if !vp.AutoScrollEnabled() {
		t.Error("auto-scroll should re-enable when scrolled back to bottom")
	}
}

func TestTUI013_SetContentReplacesLines(t *testing.T) {
	vp := viewport.New(80, 5)
	vp.AppendLine("old line 1")
	vp.AppendLine("old line 2")
	vp.SetContent("new line A\nnew line B\nnew line C")
	view := vp.View()
	if strings.Contains(view, "old line") {
		t.Error("SetContent should replace all old content")
	}
	if !strings.Contains(view, "new line A") {
		t.Error("SetContent should include new content")
	}
}

func TestTUI013_EmptyViewport(t *testing.T) {
	vp := viewport.New(80, 5)
	view := vp.View()
	// Should not panic; should return something (blank lines)
	if len(view) == 0 {
		t.Error("empty viewport should still produce output (blank lines)")
	}
}

func TestTUI013_ZeroDimensions(t *testing.T) {
	vp := viewport.New(0, 0)
	vp.AppendLine("test")
	view := vp.View()
	if view != "" {
		t.Errorf("zero-dimension viewport should return empty string, got %q", view)
	}
}

func TestTUI013_WidthTruncation(t *testing.T) {
	vp := viewport.New(10, 5)
	vp.AppendLine("this line is much longer than 10 characters")
	view := vp.View()
	lines := strings.Split(view, "\n")
	for _, l := range lines {
		if len([]rune(l)) > 10 {
			t.Errorf("line exceeds viewport width: %q (len=%d)", l, len([]rune(l)))
		}
	}
}

func TestTUI013_SetSizeUpdates(t *testing.T) {
	vp := viewport.New(80, 5)
	for i := 0; i < 20; i++ {
		vp.AppendLine(fmt.Sprintf("line %d", i))
	}
	vp.SetSize(40, 10)
	view := vp.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) > 10 {
		t.Errorf("after SetSize to height=10, got %d lines", len(lines))
	}
}

func TestTUI013_Snapshot(t *testing.T) {
	vp := viewport.New(80, 5)
	for i := 0; i < 10; i++ {
		vp.AppendLine(fmt.Sprintf("line %d", i))
	}
	snap := vp.Snapshot()
	if snap.TotalLines != 10 {
		t.Errorf("snapshot TotalLines: got %d, want 10", snap.TotalLines)
	}
	if !snap.AtBottom {
		t.Error("snapshot should be at bottom initially")
	}
	if !snap.AutoScroll {
		t.Error("snapshot should have auto-scroll enabled initially")
	}
}

func TestTUI013_AppendLinesMultiple(t *testing.T) {
	vp := viewport.New(80, 10)
	vp.AppendLines([]string{"alpha", "beta", "gamma"})
	view := vp.View()
	if !strings.Contains(view, "alpha") || !strings.Contains(view, "gamma") {
		t.Errorf("AppendLines should add all lines, got: %q", view)
	}
}

func TestTUI013_AutoScrollOnAppend(t *testing.T) {
	vp := viewport.New(80, 5)
	for i := 0; i < 20; i++ {
		vp.AppendLine(fmt.Sprintf("line %d", i))
	}
	// With auto-scroll enabled, last line should be visible
	view := vp.View()
	if !strings.Contains(view, "line 19") {
		t.Errorf("auto-scroll should show last line, got: %q", view)
	}
}
