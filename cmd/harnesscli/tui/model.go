package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Model is the root BubbleTea model for the TUI.
type Model struct {
	width  int
	height int
	config TUIConfig
	keys   KeyMap
	ready  bool

	// RunID is the current run being displayed.
	RunID string
}

// New creates a new root Model.
func New(cfg TUIConfig) Model {
	return Model{config: cfg, keys: DefaultKeyMap()}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	if !m.ready {
		return "Initializing...\n"
	}
	return "go-agent-harness\n\n[TUI initializing -- Phase 0 placeholder]\n"
}
