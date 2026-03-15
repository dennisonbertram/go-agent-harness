package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"go-agent-harness/cmd/harnesscli/tui/components/layout"
)

// Model is the root BubbleTea model for the TUI.
type Model struct {
	width  int
	height int
	layout layout.Layout
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
		m.layout = layout.Compute(msg.Width, msg.Height)
		m.ready = true
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		}

	case AssistantDeltaMsg:
		// TODO Phase 1: route to conversation viewport

	case ThinkingDeltaMsg:
		// TODO Phase 1: route to thinking indicator

	case ToolStartMsg:
		// TODO Phase 2: route to tool use component

	case RunCompletedMsg:
		// TODO: update run state

	case RunFailedMsg:
		// TODO: update run state

	case OverlayOpenMsg:
		// TODO Phase 4: open overlay

	case OverlayCloseMsg:
		// TODO Phase 4: close overlay

	case ClearMsg:
		// TODO Phase 1: clear viewport
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
