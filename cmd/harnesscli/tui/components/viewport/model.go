package viewport

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Model is the scrollable viewport for conversation content.
type Model struct {
	width      int
	height     int
	lines      []string
	offset     int  // lines from the bottom (0 = at bottom)
	autoScroll bool
	lastLen    int // tracks when new content arrives while scrolled up
}

// New creates a viewport with given dimensions.
func New(width, height int) Model {
	return Model{width: width, height: height, autoScroll: true}
}

// AppendLine adds a line to the viewport.
// If auto-scroll is enabled, the viewport stays at the bottom.
func (m *Model) AppendLine(line string) {
	m.lines = append(m.lines, line)
	if m.autoScroll {
		m.offset = 0
	}
}

// AppendLines adds multiple lines.
func (m *Model) AppendLines(lines []string) {
	for _, l := range lines {
		m.AppendLine(l)
	}
}

// SetContent replaces all lines (e.g., for re-render of last message).
// If the new content is shorter than the current offset, the offset is
// clamped so it cannot exceed the maximum scrollable range.
func (m *Model) SetContent(content string) {
	m.lines = strings.Split(content, "\n")
	if m.autoScroll {
		m.offset = 0
	} else {
		// Clamp offset so it stays within valid range of new content.
		maxOff := len(m.lines) - m.height
		if maxOff < 0 {
			maxOff = 0
		}
		if m.offset > maxOff {
			m.offset = maxOff
		}
	}
}

// ScrollUp scrolls up by n lines and disables auto-scroll.
func (m *Model) ScrollUp(n int) {
	m.autoScroll = false
	maxOff := len(m.lines) - m.height
	if maxOff < 0 {
		maxOff = 0
	}
	m.offset += n
	if m.offset > maxOff {
		m.offset = maxOff
	}
	m.lastLen = len(m.lines)
}

// ScrollDown scrolls down by n lines. Re-enables auto-scroll if reaching the bottom.
func (m *Model) ScrollDown(n int) {
	m.offset -= n
	if m.offset < 0 {
		m.offset = 0
		m.autoScroll = true
	}
}

// ScrollToBottom jumps to the bottom and re-enables auto-scroll.
func (m *Model) ScrollToBottom() {
	m.offset = 0
	m.autoScroll = true
	m.lastLen = len(m.lines)
}

// AtBottom reports whether viewport is at the bottom.
func (m Model) AtBottom() bool { return m.offset == 0 }

// AutoScrollEnabled reports whether auto-scroll is active.
func (m Model) AutoScrollEnabled() bool { return m.autoScroll }

// ScrollOffset returns the current scroll offset (lines from bottom).
func (m Model) ScrollOffset() int { return m.offset }

// HasNewContent reports if new lines arrived while scrolled up.
func (m Model) HasNewContent() bool {
	return !m.autoScroll && len(m.lines) > m.lastLen
}

// SetSize updates viewport dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles key messages for scrolling.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyPgUp:
			m.ScrollUp(m.height / 2)
		case tea.KeyPgDown:
			m.ScrollDown(m.height / 2)
		case tea.KeyUp:
			m.ScrollUp(1)
		case tea.KeyDown:
			m.ScrollDown(1)
		}
	}
	return m, nil
}

// View renders the visible portion of the conversation.
func (m Model) View() string {
	if m.height <= 0 || m.width <= 0 {
		return ""
	}

	total := len(m.lines)
	if total == 0 {
		return strings.Repeat("\n", m.height-1)
	}

	// Calculate visible window. offset is from the bottom.
	end := total - m.offset
	if end > total {
		end = total
	}
	start := end - m.height
	if start < 0 {
		start = 0
	}

	visible := m.lines[start:end]

	var sb strings.Builder
	for _, line := range visible {
		runes := []rune(line)
		if len(runes) > m.width {
			line = string(runes[:m.width])
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// Pad remaining lines to fill height.
	for i := len(visible); i < m.height; i++ {
		sb.WriteString("\n")
	}

	result := sb.String()
	// Trim trailing newline from padding.
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}
	return result
}
