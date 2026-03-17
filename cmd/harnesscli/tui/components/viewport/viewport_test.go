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

// TestView_BottomAnchored_LessContentThanHeight verifies that when content (3 lines)
// is less than viewport height (10), the content is anchored to the bottom with
// blank lines above (like a chat app), not at the top with blank lines below.
func TestView_BottomAnchored_LessContentThanHeight(t *testing.T) {
	vp := viewport.New(80, 10)
	vp.AppendLine("line A")
	vp.AppendLine("line B")
	vp.AppendLine("line C")
	view := vp.View()
	lines := strings.Split(view, "\n")
	// View() trims trailing newline so we get exactly height lines after split.
	// With height=10 and 3 content lines, first 7 lines must be blank, last 3 are content.
	if len(lines) != 10 {
		t.Fatalf("expected 10 rendered lines, got %d: %q", len(lines), view)
	}
	// First 7 lines must be blank (padding above).
	for i := 0; i < 7; i++ {
		if lines[i] != "" {
			t.Errorf("line[%d] should be blank (top padding), got %q", i, lines[i])
		}
	}
	// Last 3 lines must be the content.
	if lines[7] != "line A" {
		t.Errorf("line[7] should be 'line A', got %q", lines[7])
	}
	if lines[8] != "line B" {
		t.Errorf("line[8] should be 'line B', got %q", lines[8])
	}
	if lines[9] != "line C" {
		t.Errorf("line[9] should be 'line C', got %q", lines[9])
	}
}

// TestView_BottomAnchored_ExactHeight verifies that exactly-full content has no padding.
func TestView_BottomAnchored_ExactHeight(t *testing.T) {
	vp := viewport.New(80, 10)
	for i := 0; i < 10; i++ {
		vp.AppendLine(fmt.Sprintf("line %d", i))
	}
	view := vp.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 rendered lines, got %d", len(lines))
	}
	// No blank lines at top — content fills exactly.
	for i, l := range lines {
		if l == "" {
			t.Errorf("line[%d] should not be blank when content fills viewport exactly", i)
		}
	}
}

// TestView_BottomAnchored_MoreThanHeight verifies that when content exceeds height,
// the bottom height lines are shown with no top padding.
func TestView_BottomAnchored_MoreThanHeight(t *testing.T) {
	vp := viewport.New(80, 10)
	for i := 0; i < 15; i++ {
		vp.AppendLine(fmt.Sprintf("line %d", i))
	}
	view := vp.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 rendered lines, got %d", len(lines))
	}
	// No blank top-padding lines.
	for i := 0; i < 10; i++ {
		if lines[i] == "" {
			t.Errorf("line[%d] should not be blank when content > height", i)
		}
	}
	// Last visible line should be line 14 (the last appended).
	if !strings.Contains(lines[9], "line 14") {
		t.Errorf("last rendered line should be 'line 14', got %q", lines[9])
	}
}

// TestRegression_ViewportNoTopAnchor verifies that a fresh viewport with 1 line of
// content and height=5 shows the content on the LAST rendered line, not the first.
func TestRegression_ViewportNoTopAnchor(t *testing.T) {
	vp := viewport.New(80, 5)
	vp.AppendLine("only line")
	view := vp.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 rendered lines, got %d: %q", len(lines), view)
	}
	// Content should appear on the last line (index 4), not the first.
	if lines[0] == "only line" {
		t.Error("content must NOT be on the first (top) line — viewport should bottom-anchor")
	}
	if lines[4] != "only line" {
		t.Errorf("content should be on the last line (index 4), got %q", lines[4])
	}
}

// TestViewport_AppendChunk_SingleChunk verifies that AppendChunk on an empty
// viewport starts a new line with the chunk content.
func TestViewport_AppendChunk_SingleChunk(t *testing.T) {
	vp := viewport.New(80, 5)
	vp.AppendChunk("hello")
	view := vp.View()
	if !strings.Contains(view, "hello") {
		t.Errorf("view should contain 'hello' after AppendChunk, got: %q", view)
	}
	// Must appear on exactly one line, not spread over multiple.
	count := 0
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(l, "hello") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("'hello' should appear on exactly 1 line, found on %d lines", count)
	}
}

// TestViewport_AppendChunk_MultipleChunks verifies that multiple AppendChunk calls
// accumulate on the same line, not as separate lines.
func TestViewport_AppendChunk_MultipleChunks(t *testing.T) {
	vp := viewport.New(80, 5)
	vp.AppendLine("") // start a fresh line (simulating pre-response blank line)
	vp.AppendChunk("Hello")
	vp.AppendChunk(",")
	vp.AppendChunk(" world")
	view := vp.View()
	if !strings.Contains(view, "Hello, world") {
		t.Errorf("chunks should accumulate as 'Hello, world' on one line, got: %q", view)
	}
	// Count lines that contain any of the chunks — should be just one line.
	contentLines := 0
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(l, "Hello") || strings.Contains(l, ",") && strings.Contains(l, "world") {
			contentLines++
		}
	}
	if contentLines != 1 {
		t.Errorf("all chunks should be on exactly 1 line, found on %d lines; view=%q", contentLines, view)
	}
}

// TestViewport_AppendChunk_ThenNewLine verifies that AppendChunk followed by
// AppendLine("") and then another AppendChunk puts the second chunk on a new line.
func TestViewport_AppendChunk_ThenNewLine(t *testing.T) {
	vp := viewport.New(80, 5)
	vp.AppendChunk("hello")
	vp.AppendLine("") // start new line
	vp.AppendChunk("world")
	view := vp.View()
	lines := strings.Split(view, "\n")
	helloLine, worldLine := -1, -1
	for i, l := range lines {
		if strings.Contains(l, "hello") {
			helloLine = i
		}
		if strings.Contains(l, "world") {
			worldLine = i
		}
	}
	if helloLine == -1 {
		t.Errorf("'hello' not found in view: %q", view)
	}
	if worldLine == -1 {
		t.Errorf("'world' not found in view: %q", view)
	}
	if helloLine == worldLine {
		t.Errorf("'hello' and 'world' should be on different lines after AppendLine between them")
	}
}

// TestRegression_StreamingTokensOnOneLine simulates 5 delta chunks and verifies
// they all end up on a single line in the viewport, not 5 separate lines.
func TestRegression_StreamingTokensOnOneLine(t *testing.T) {
	vp := viewport.New(80, 10)
	vp.AppendLine("") // fresh line for assistant response
	tokens := []string{"The ", "answer ", "is ", "42", "."}
	for _, tok := range tokens {
		vp.AppendChunk(tok)
	}
	view := vp.View()
	// The full assembled text must appear.
	if !strings.Contains(view, "The answer is 42.") {
		t.Errorf("assembled text 'The answer is 42.' not found in view: %q", view)
	}
	// Must be on exactly one line, not five.
	tokenLines := 0
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(l, "The") && strings.Contains(l, "42") {
			tokenLines++
		}
	}
	if tokenLines != 1 {
		t.Errorf("streaming tokens should render on 1 line, found on %d lines; view=%q", tokenLines, view)
	}
}
