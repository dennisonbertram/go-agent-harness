package main

import (
	"fmt"
	"strings"
)

// InputModel manages multi-line input state for the demo-cli REPL.
// It is intentionally pure (no terminal I/O) for testability.
// The terminal rendering layer calls Render() and performs the actual writes.
//
// Key bindings (handled by the caller, not this struct):
//   - Enter (0x0d / '\r'): call Submit() to retrieve the input
//   - Ctrl+J (0x0a / '\n') or Shift+Enter (kitty: \x1b[13;2u): call InsertNewline()
//   - Ctrl+C (0x03): call HandleCtrlC() — clears if non-empty, signals exit if empty
//   - Backspace (0x7f / 0x08): call Backspace()
//   - Printable rune: call InsertChar(ch)
type InputModel struct {
	lines    []string // current input lines (always at least one element)
	curLine  int      // cursor line index (0-based)
	curCol   int      // cursor column index within the current line (0-based, points to insert position)
	maxLines int      // maximum visible lines before scrolling (default: 6)
}

// NewInputModel returns an initialised InputModel with sane defaults.
func NewInputModel() *InputModel {
	return &InputModel{
		lines:    []string{""},
		curLine:  0,
		curCol:   0,
		maxLines: 6,
	}
}

// InsertChar inserts a printable character at the current cursor position.
func (m *InputModel) InsertChar(ch rune) {
	line := m.lines[m.curLine]
	// Insert ch at curCol
	before := line[:m.curCol]
	after := line[m.curCol:]
	m.lines[m.curLine] = before + string(ch) + after
	m.curCol++
}

// InsertNewline inserts a newline at the cursor position (Shift+Enter / Ctrl+J).
// If the model is already at maxLines, the topmost line is scrolled off the
// visible buffer (the submit will still include all lines).
func (m *InputModel) InsertNewline() {
	currentLine := m.lines[m.curLine]
	before := currentLine[:m.curCol]
	after := currentLine[m.curCol:]

	// Replace current line with the part before cursor
	m.lines[m.curLine] = before

	// Insert a new line after the current one with the part after cursor
	newLines := make([]string, 0, len(m.lines)+1)
	newLines = append(newLines, m.lines[:m.curLine+1]...)
	newLines = append(newLines, after)
	newLines = append(newLines, m.lines[m.curLine+1:]...)
	m.lines = newLines

	m.curLine++
	m.curCol = 0

	// If we've exceeded maxLines, scroll: remove the oldest line but keep
	// curLine pointed to the same logical content by adjusting index.
	// The full content is preserved for Submit(); only the visible window shrinks.
	if len(m.lines) > m.maxLines {
		// Remove the first line (scroll up)
		m.lines = m.lines[1:]
		m.curLine--
	}
}

// Submit returns the current input as a single string with lines joined by '\n',
// then resets the model to the initial empty state.
// Returns "" if the model is empty.
func (m *InputModel) Submit() string {
	result := strings.Join(m.lines, "\n")
	// Trim trailing newlines that result from trailing empty lines
	result = strings.TrimRight(result, "\n")
	m.Clear()
	return result
}

// Clear resets the model to the initial empty state.
func (m *InputModel) Clear() {
	m.lines = []string{""}
	m.curLine = 0
	m.curCol = 0
}

// IsEmpty returns true if the model contains no non-whitespace content.
func (m *InputModel) IsEmpty() bool {
	for _, line := range m.lines {
		if strings.TrimSpace(line) != "" {
			return false
		}
	}
	return true
}

// LineCount returns the number of lines currently in the buffer.
func (m *InputModel) LineCount() int {
	return len(m.lines)
}

// HandleCtrlC implements the Ctrl+C behaviour:
//   - If input is non-empty, clears it and returns false (continue running).
//   - If input is empty, returns true (caller should exit).
func (m *InputModel) HandleCtrlC() bool {
	if m.IsEmpty() {
		return true // signal exit
	}
	m.Clear()
	return false
}

// Backspace deletes the character immediately before the cursor.
// If the cursor is at the start of a non-first line, it merges with the previous line.
func (m *InputModel) Backspace() {
	if m.curCol > 0 {
		// Delete the character before the cursor on the same line.
		line := m.lines[m.curLine]
		m.lines[m.curLine] = line[:m.curCol-1] + line[m.curCol:]
		m.curCol--
		return
	}
	// curCol == 0: merge with the previous line.
	if m.curLine == 0 {
		return // nothing to do
	}
	prevLine := m.lines[m.curLine-1]
	currLine := m.lines[m.curLine]
	m.lines[m.curLine-1] = prevLine + currLine
	// Remove current line
	m.lines = append(m.lines[:m.curLine], m.lines[m.curLine+1:]...)
	m.curLine--
	m.curCol = len(prevLine)
}

// Render returns a string representation of the current input for display in a terminal.
// Format:
//
//	Single line:  "> <content>"
//	Multi-line:
//	  "> Line 1\n  Line 2\n  Line 3 [3 lines]"
//
// The Render output uses '\n' as a line separator. The terminal layer is responsible
// for translating to \r\n in raw mode.
//
// InsertNewline rolls the buffer so len(m.lines) <= maxLines always; this function
// simply renders all lines in the buffer with the first line getting the ">" prefix.
func (m *InputModel) Render() string {
	lineCount := len(m.lines)

	var sb strings.Builder
	for i, line := range m.lines {
		// First line gets the ">" prompt prefix; subsequent lines get indent.
		prefix := "  "
		if i == 0 {
			prefix = "> "
		}

		// Add line count indicator to the last line when multi-line.
		if i == lineCount-1 && lineCount > 1 {
			sb.WriteString(fmt.Sprintf("%s%s [%d lines]", prefix, line, lineCount))
		} else {
			sb.WriteString(prefix + line)
		}

		if i < lineCount-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
