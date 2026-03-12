package main

import (
	"strings"
	"testing"
)

// TestInputModel_NewInputModel verifies a new InputModel starts empty.
func TestInputModel_NewInputModel(t *testing.T) {
	m := NewInputModel()
	if !m.IsEmpty() {
		t.Error("new InputModel should be empty")
	}
	if m.LineCount() != 1 {
		t.Errorf("expected 1 line, got %d", m.LineCount())
	}
}

// TestInputModel_InsertChar verifies basic character insertion.
func TestInputModel_InsertChar(t *testing.T) {
	m := NewInputModel()
	m.InsertChar('h')
	m.InsertChar('i')

	if m.IsEmpty() {
		t.Error("model should not be empty after inserting chars")
	}
	if m.LineCount() != 1 {
		t.Errorf("expected 1 line, got %d", m.LineCount())
	}
	result := m.Submit()
	if result != "hi" {
		t.Errorf("expected 'hi', got %q", result)
	}
}

// TestInputModel_InsertNewline verifies Shift+Enter/Ctrl+J inserts a newline.
func TestInputModel_InsertNewline(t *testing.T) {
	m := NewInputModel()
	m.InsertChar('l')
	m.InsertChar('i')
	m.InsertChar('n')
	m.InsertChar('e')
	m.InsertChar('1')
	m.InsertNewline()
	m.InsertChar('l')
	m.InsertChar('i')
	m.InsertChar('n')
	m.InsertChar('e')
	m.InsertChar('2')

	if m.LineCount() != 2 {
		t.Errorf("expected 2 lines, got %d", m.LineCount())
	}
}

// TestInputModel_Submit_MultiLine verifies Submit joins lines with \n.
func TestInputModel_Submit_MultiLine(t *testing.T) {
	m := NewInputModel()
	m.InsertChar('f')
	m.InsertChar('o')
	m.InsertChar('o')
	m.InsertNewline()
	m.InsertChar('b')
	m.InsertChar('a')
	m.InsertChar('r')

	result := m.Submit()
	if result != "foo\nbar" {
		t.Errorf("expected 'foo\\nbar', got %q", result)
	}
}

// TestInputModel_Submit_ClearsState verifies Submit resets the model.
func TestInputModel_Submit_ClearsState(t *testing.T) {
	m := NewInputModel()
	m.InsertChar('x')
	m.Submit()

	if !m.IsEmpty() {
		t.Error("model should be empty after Submit")
	}
	if m.LineCount() != 1 {
		t.Errorf("expected 1 line after Submit, got %d", m.LineCount())
	}
}

// TestInputModel_Submit_EmptyReturnsEmpty verifies Submit on empty model returns "".
func TestInputModel_Submit_EmptyReturnsEmpty(t *testing.T) {
	m := NewInputModel()
	result := m.Submit()
	if result != "" {
		t.Errorf("expected empty string from Submit on empty model, got %q", result)
	}
}

// TestInputModel_Clear verifies Clear resets the model.
func TestInputModel_Clear(t *testing.T) {
	m := NewInputModel()
	m.InsertChar('a')
	m.InsertChar('b')
	m.InsertNewline()
	m.InsertChar('c')
	m.Clear()

	if !m.IsEmpty() {
		t.Error("model should be empty after Clear")
	}
	if m.LineCount() != 1 {
		t.Errorf("expected 1 line after Clear, got %d", m.LineCount())
	}
}

// TestInputModel_IsEmpty verifies IsEmpty correctly reflects state.
func TestInputModel_IsEmpty(t *testing.T) {
	m := NewInputModel()
	if !m.IsEmpty() {
		t.Error("new model should be empty")
	}
	m.InsertChar('x')
	if m.IsEmpty() {
		t.Error("model with char should not be empty")
	}
	m.Clear()
	if !m.IsEmpty() {
		t.Error("cleared model should be empty")
	}
}

// TestInputModel_IsEmpty_NewlineOnly verifies a model with only newlines is not considered empty.
func TestInputModel_IsEmpty_NewlineOnly(t *testing.T) {
	m := NewInputModel()
	m.InsertNewline()
	// A model with a newline has 2 lines but both are empty - this should still be considered empty
	if !m.IsEmpty() {
		t.Error("model with only newlines should be empty")
	}
}

// TestInputModel_Backspace verifies Backspace deletes the previous character.
func TestInputModel_Backspace(t *testing.T) {
	m := NewInputModel()
	m.InsertChar('a')
	m.InsertChar('b')
	m.InsertChar('c')
	m.Backspace()

	result := m.Submit()
	if result != "ab" {
		t.Errorf("expected 'ab' after backspace, got %q", result)
	}
}

// TestInputModel_Backspace_AtLineStart verifies Backspace at start of a line merges with previous line.
func TestInputModel_Backspace_AtLineStart(t *testing.T) {
	m := NewInputModel()
	m.InsertChar('a')
	m.InsertNewline()
	m.InsertChar('b')
	// Move cursor back to start of line 2 by clearing it
	// Backspace from position 0 of line 2 should merge with line 1
	m.Backspace() // deletes 'b'
	m.Backspace() // should merge lines (remove the newline)

	if m.LineCount() != 1 {
		t.Errorf("expected 1 line after merging, got %d", m.LineCount())
	}
	result := m.Submit()
	if result != "a" {
		t.Errorf("expected 'a' after merge, got %q", result)
	}
}

// TestInputModel_Backspace_EmptyModel verifies Backspace on empty model is a no-op.
func TestInputModel_Backspace_EmptyModel(t *testing.T) {
	m := NewInputModel()
	m.Backspace() // should not panic
	if !m.IsEmpty() {
		t.Error("model should still be empty after backspace on empty model")
	}
}

// TestInputModel_LineCount_Multiple verifies LineCount returns correct counts.
func TestInputModel_LineCount_Multiple(t *testing.T) {
	m := NewInputModel()
	if m.LineCount() != 1 {
		t.Errorf("expected 1, got %d", m.LineCount())
	}
	m.InsertNewline()
	if m.LineCount() != 2 {
		t.Errorf("expected 2, got %d", m.LineCount())
	}
	m.InsertNewline()
	if m.LineCount() != 3 {
		t.Errorf("expected 3, got %d", m.LineCount())
	}
}

// TestInputModel_MaxLines_AtBoundary verifies inserting newlines up to and at maxLines.
func TestInputModel_MaxLines_AtBoundary(t *testing.T) {
	m := NewInputModel()
	m.maxLines = 6

	// Insert 5 newlines to get 6 lines (maxLines)
	for i := 0; i < 5; i++ {
		m.InsertChar('x')
		m.InsertNewline()
	}
	m.InsertChar('x')

	if m.LineCount() != 6 {
		t.Errorf("expected 6 lines at max, got %d", m.LineCount())
	}

	// Trying to insert a 7th newline should not increase linecount beyond 6
	// (the model scrolls/rejects rather than growing past max)
	m.InsertNewline()
	if m.LineCount() > 6 {
		t.Errorf("expected lineCount <= 6 after exceeding max, got %d", m.LineCount())
	}
}

// TestInputModel_Render_SingleLine verifies Render for a single-line input.
func TestInputModel_Render_SingleLine(t *testing.T) {
	m := NewInputModel()
	m.InsertChar('h')
	m.InsertChar('e')
	m.InsertChar('l')
	m.InsertChar('l')
	m.InsertChar('o')

	rendered := m.Render()
	if !strings.Contains(rendered, "hello") {
		t.Errorf("expected 'hello' in render output, got %q", rendered)
	}
	// Single line should not have line count indicator
	if strings.Contains(rendered, "lines]") {
		t.Errorf("single-line render should not have line count indicator, got %q", rendered)
	}
}

// TestInputModel_Render_MultiLine verifies Render for multi-line input shows line count.
func TestInputModel_Render_MultiLine(t *testing.T) {
	m := NewInputModel()
	m.InsertChar('a')
	m.InsertNewline()
	m.InsertChar('b')
	m.InsertNewline()
	m.InsertChar('c')

	rendered := m.Render()
	if !strings.Contains(rendered, "[3 lines]") {
		t.Errorf("multi-line render should contain '[3 lines]', got %q", rendered)
	}
}

// TestInputModel_Render_TwoLines verifies two-line render shows indicator.
func TestInputModel_Render_TwoLines(t *testing.T) {
	m := NewInputModel()
	m.InsertChar('a')
	m.InsertNewline()
	m.InsertChar('b')

	rendered := m.Render()
	if !strings.Contains(rendered, "[2 lines]") {
		t.Errorf("two-line render should contain '[2 lines]', got %q", rendered)
	}
}

// TestInputModel_CtrlC_NonEmpty_Clears verifies that the Ctrl+C behavior signal works for non-empty.
func TestInputModel_CtrlC_NonEmpty(t *testing.T) {
	m := NewInputModel()
	m.InsertChar('t')
	m.InsertChar('e')
	m.InsertChar('s')
	m.InsertChar('t')

	// HandleCtrlC returns true = should exit, false = cleared input
	shouldExit := m.HandleCtrlC()
	if shouldExit {
		t.Error("non-empty: HandleCtrlC should return false (clear input, not exit)")
	}
	if !m.IsEmpty() {
		t.Error("model should be empty after HandleCtrlC on non-empty input")
	}
}

// TestInputModel_CtrlC_Empty_SignalsExit verifies HandleCtrlC on empty model returns true.
func TestInputModel_CtrlC_Empty(t *testing.T) {
	m := NewInputModel()
	shouldExit := m.HandleCtrlC()
	if !shouldExit {
		t.Error("empty: HandleCtrlC should return true (signal exit)")
	}
}

// TestInputModel_Submit_PreservesNewlines verifies Submit output has correct newlines.
func TestInputModel_Submit_PreservesNewlines(t *testing.T) {
	m := NewInputModel()
	lines := []string{"line one", "line two", "line three"}
	for i, line := range lines {
		for _, ch := range line {
			m.InsertChar(ch)
		}
		if i < len(lines)-1 {
			m.InsertNewline()
		}
	}

	result := m.Submit()
	expected := "line one\nline two\nline three"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// TestInputModel_CursorPosition verifies cursor position is tracked.
func TestInputModel_CursorPosition(t *testing.T) {
	m := NewInputModel()
	m.InsertChar('a')
	m.InsertChar('b')
	m.InsertChar('c')

	if m.curCol != 3 {
		t.Errorf("expected curCol=3 after 3 chars, got %d", m.curCol)
	}
	if m.curLine != 0 {
		t.Errorf("expected curLine=0, got %d", m.curLine)
	}
}

// TestInputModel_CursorPosition_AfterNewline verifies cursor moves to new line.
func TestInputModel_CursorPosition_AfterNewline(t *testing.T) {
	m := NewInputModel()
	m.InsertChar('a')
	m.InsertNewline()
	m.InsertChar('b')

	if m.curLine != 1 {
		t.Errorf("expected curLine=1 after newline, got %d", m.curLine)
	}
	if m.curCol != 1 {
		t.Errorf("expected curCol=1 after inserting 'b', got %d", m.curCol)
	}
}

// TestInputModel_Render_MultiLine_FirstLinePrefix verifies the display format.
func TestInputModel_Render_MultiLine_Format(t *testing.T) {
	m := NewInputModel()
	for _, ch := range "Line 1" {
		m.InsertChar(ch)
	}
	m.InsertNewline()
	for _, ch := range "Line 2" {
		m.InsertChar(ch)
	}
	m.InsertNewline()
	for _, ch := range "Line 3" {
		m.InsertChar(ch)
	}

	rendered := m.Render()
	// First line should have ">" prefix
	if !strings.Contains(rendered, "> Line 1") {
		t.Errorf("expected '> Line 1' in render output, got %q", rendered)
	}
	// Subsequent lines should have "  " prefix
	if !strings.Contains(rendered, "  Line 2") {
		t.Errorf("expected '  Line 2' in render output, got %q", rendered)
	}
	// Line count indicator at end of last line
	if !strings.Contains(rendered, "Line 3") || !strings.Contains(rendered, "[3 lines]") {
		t.Errorf("expected last line with [3 lines], got %q", rendered)
	}
}

// TestInputModel_Render_Empty verifies rendering an empty model.
func TestInputModel_Render_Empty(t *testing.T) {
	m := NewInputModel()
	rendered := m.Render()
	// Should show the prompt prefix with empty content
	if !strings.Contains(rendered, "> ") {
		t.Errorf("empty render should contain '> ' prefix, got %q", rendered)
	}
	// Should not have line count indicator
	if strings.Contains(rendered, "lines]") {
		t.Errorf("empty render should not have line count indicator, got %q", rendered)
	}
}

// TestInputModel_Render_ScrolledWindow verifies that scrolled rendering (rolling buffer)
// keeps the most recent lines and shows a line count indicator.
func TestInputModel_Render_ScrolledWindow(t *testing.T) {
	m := NewInputModel()
	m.maxLines = 3

	// Insert 5 lines. With maxLines=3, the buffer rolls: only last 3 lines remain.
	// a, b, c -> then d: rolls to b, c, d -> then e: rolls to c, d, e
	for i := 0; i < 4; i++ {
		m.InsertChar(rune('a' + i))
		m.InsertNewline()
	}
	m.InsertChar('e')

	// After scroll, buffer has 3 lines: ["c", "d", "e"]
	if m.LineCount() != 3 {
		t.Errorf("expected 3 lines (maxLines), got %d", m.LineCount())
	}

	rendered := m.Render()

	// The first visible line should have ">" prefix
	lines := strings.Split(rendered, "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "> ") {
		t.Errorf("first scrolled line should have '> ' prefix: %q", rendered)
	}

	// Should have line count indicator since there are 3 lines (still > 1)
	if !strings.Contains(rendered, "[3 lines]") {
		t.Errorf("expected '[3 lines]' in scrolled render, got %q", rendered)
	}

	// The content should contain the newest lines (c, d, e)
	if !strings.Contains(rendered, "c") || !strings.Contains(rendered, "d") || !strings.Contains(rendered, "e") {
		t.Errorf("render should contain most recent lines (c,d,e), got %q", rendered)
	}
	// The oldest lines (a, b) should have been scrolled off
	if strings.Contains(rendered, "> a") || strings.Contains(rendered, "  a") {
		t.Errorf("oldest line 'a' should be scrolled off, got %q", rendered)
	}
}

// TestInputModel_MaxLines_ScrollBehavior verifies that at maxLines, the view scrolls.
func TestInputModel_MaxLines_ScrollBehavior(t *testing.T) {
	m := NewInputModel()
	m.maxLines = 6

	// Create 8 lines of content
	for i := 0; i < 7; i++ {
		m.InsertChar(rune('a' + i))
		m.InsertNewline()
	}
	m.InsertChar('z')

	// Total lines in model may be > 6
	totalLines := m.LineCount()
	if totalLines < 6 {
		t.Errorf("expected at least 6 lines, got %d", totalLines)
	}

	// Render should show at most maxLines visible lines
	rendered := m.Render()
	renderedLines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
	// Filter out empty lines at end
	actualRendered := 0
	for _, l := range renderedLines {
		if strings.TrimSpace(l) != "" {
			actualRendered++
		}
	}
	if actualRendered > m.maxLines {
		t.Errorf("rendered more than maxLines (%d) visible lines: %d", m.maxLines, actualRendered)
	}
}
