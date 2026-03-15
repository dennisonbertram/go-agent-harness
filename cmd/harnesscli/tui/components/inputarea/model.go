package inputarea

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CommandSubmittedMsg is sent when the user presses Enter with content.
type CommandSubmittedMsg struct{ Value string }

// Model is the multiline input area component.
type Model struct {
	width   int
	value   string
	cursor  int
	history []string
	histIdx int // -1 = current, 0..len-1 = history
	focused bool
}

var (
	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}).
			Bold(true)
	inputStyle = lipgloss.NewStyle()
)

const promptSymbol = "❯"

// New creates a new input area for the given width.
func New(width int) Model {
	return Model{width: width, histIdx: -1, focused: true}
}

// Value returns the current input text.
func (m Model) Value() string { return m.value }

// SetWidth updates the display width.
func (m *Model) SetWidth(w int) { m.width = w }

// Focus sets keyboard focus.
func (m *Model) Focus() { m.focused = true }

// Blur removes keyboard focus.
func (m *Model) Blur() { m.focused = false }

// Update handles key messages for the input area.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused {
		return m, nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch key.Type {
	case tea.KeyEnter:
		if m.value == "" {
			return m, nil
		}
		submitted := m.value
		// Add to history (avoid duplicates at end)
		if len(m.history) == 0 || m.history[len(m.history)-1] != submitted {
			m.history = append(m.history, submitted)
		}
		m.value = ""
		m.cursor = 0
		m.histIdx = -1
		return m, func() tea.Msg { return CommandSubmittedMsg{Value: submitted} }

	case tea.KeyCtrlJ: // alternative newline (ctrl+j / shift+enter)
		m.value = m.value[:m.cursor] + "\n" + m.value[m.cursor:]
		m.cursor++

	case tea.KeyBackspace, tea.KeyDelete:
		if m.cursor > 0 && len(m.value) > 0 {
			runes := []rune(m.value)
			if m.cursor <= len(runes) {
				runes = append(runes[:m.cursor-1], runes[m.cursor:]...)
				m.value = string(runes)
				m.cursor--
			}
		}

	case tea.KeyLeft:
		if m.cursor > 0 {
			m.cursor--
		}

	case tea.KeyRight:
		if m.cursor < len([]rune(m.value)) {
			m.cursor++
		}

	case tea.KeyUp:
		// History navigation
		if len(m.history) > 0 {
			if m.histIdx == -1 {
				m.histIdx = len(m.history) - 1
			} else if m.histIdx > 0 {
				m.histIdx--
			}
			m.value = m.history[m.histIdx]
			m.cursor = len([]rune(m.value))
		}

	case tea.KeyDown:
		if m.histIdx != -1 {
			m.histIdx++
			if m.histIdx >= len(m.history) {
				m.histIdx = -1
				m.value = ""
				m.cursor = 0
			} else {
				m.value = m.history[m.histIdx]
				m.cursor = len([]rune(m.value))
			}
		}

	case tea.KeyRunes:
		runes := []rune(m.value)
		insert := key.Runes
		newRunes := make([]rune, 0, len(runes)+len(insert))
		newRunes = append(newRunes, runes[:m.cursor]...)
		newRunes = append(newRunes, insert...)
		newRunes = append(newRunes, runes[m.cursor:]...)
		m.value = string(newRunes)
		m.cursor += len(insert)
	}

	return m, nil
}

// View renders the input area with prompt symbol and cursor.
func (m Model) View() string {
	prompt := promptStyle.Render(promptSymbol)

	runes := []rune(m.value)
	// Split at cursor
	before := string(runes[:m.cursor])
	var cursor, after string
	if m.cursor < len(runes) {
		// Cursor on existing char — show as reverse video
		cursor = lipgloss.NewStyle().Reverse(true).Render(string(runes[m.cursor]))
		after = string(runes[m.cursor+1:])
	} else {
		// Cursor at end — show block
		cursor = lipgloss.NewStyle().Reverse(true).Render(" ")
		after = ""
	}

	// Show first line of multi-line input (full multiline layout in TUI-018)
	displayLines := strings.Split(before+cursor+after, "\n")
	inputLine := inputStyle.Render(displayLines[0])

	return prompt + " " + inputLine
}

// Init satisfies tea.Model for standalone use.
func (m Model) Init() tea.Cmd { return nil }
