package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"go-agent-harness/cmd/harnesscli/tui/components/inputarea"
	"go-agent-harness/cmd/harnesscli/tui/components/layout"
	"go-agent-harness/cmd/harnesscli/tui/components/statusbar"
	"go-agent-harness/cmd/harnesscli/tui/components/viewport"
)

// statusMsgDuration is how long a transient status message is shown.
const statusMsgDuration = 3 * time.Second

// Model is the root BubbleTea model for the TUI.
type Model struct {
	width  int
	height int
	layout layout.Layout
	theme  Theme
	config TUIConfig
	keys   KeyMap
	ready  bool

	// RunID is the current run being displayed.
	RunID string

	// runActive is true while a run is in flight.
	runActive bool

	// cancelRun holds the cancel func from the SSE bridge; nil when no run is active.
	cancelRun func()

	// toolExpanded tracks which tool calls are in the expanded view, keyed by
	// tool call ID. True = expanded, absent/false = collapsed.
	toolExpanded map[string]bool

	// activeToolCallID is the ID of the currently active/selected tool call,
	// used when toggling expansion via Ctrl+O.
	activeToolCallID string

	// lastAssistantText accumulates all assistant deltas for the current run.
	lastAssistantText string

	// overlayActive is true when an overlay (help, context, stats, etc.) is open.
	overlayActive bool

	// statusMsg is a transient overlay message shown on the status bar.
	statusMsg string
	// statusMsgExpiry is when statusMsg should be cleared.
	statusMsgExpiry time.Time

	// Components
	statusBar statusbar.Model
	vp        viewport.Model
	input     inputarea.Model
}

// New creates a new root Model.
func New(cfg TUIConfig) Model {
	return Model{
		config: cfg,
		keys:   DefaultKeyMap(),
		theme:  DefaultTheme(),
	}
}

// RunActive returns true if a run is currently in flight.
func (m Model) RunActive() bool {
	return m.runActive
}

// StatusMsg returns the current transient status message (for testing).
func (m Model) StatusMsg() string {
	return m.statusMsg
}

// OverlayActive returns true when an overlay is currently open (for testing).
func (m Model) OverlayActive() bool {
	return m.overlayActive
}

// WithCancelRun returns a copy of the Model with the given cancel func set.
// This is used to wire up the SSE bridge cancel func before a run starts.
func (m Model) WithCancelRun(cancel func()) Model {
	m.cancelRun = cancel
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Clear expired status message.
	if m.statusMsg != "" && !m.statusMsgExpiry.IsZero() && time.Now().After(m.statusMsgExpiry) {
		m.statusMsg = ""
		m.statusMsgExpiry = time.Time{}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout = layout.Compute(msg.Width, msg.Height)
		m.ready = true

		// Initialize/resize components
		m.statusBar = statusbar.New(msg.Width)
		m.vp = viewport.New(msg.Width, m.layout.ViewportHeight)
		m.input = inputarea.New(msg.Width)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			// If a run is active, Ctrl+C cancels the run instead of quitting.
			if m.runActive && m.cancelRun != nil {
				m.cancelRun()
				m.runActive = false
				m.cancelRun = nil
				m.statusMsg = "Interrupted"
				m.statusMsgExpiry = time.Now().Add(statusMsgDuration)
				// Do NOT quit — return without tea.Quit
				return m, tea.Batch(cmds...)
			}
			// No active run: fall through to default quit behavior.
			return m, tea.Quit
		case key.Matches(msg, m.keys.Copy):
			ok := CopyToClipboard(m.lastAssistantText)
			if ok {
				m.statusMsg = "Copied!"
			} else {
				m.statusMsg = "Copy unavailable"
			}
			m.statusMsgExpiry = time.Now().Add(statusMsgDuration)
		case key.Matches(msg, m.keys.Interrupt):
			// Multi-priority Escape semantics (highest to lowest):
			// 1. overlayActive  → close overlay
			// 2. runActive      → cancel run
			// 3. input has text → clear input
			// 4. otherwise      → no-op
			if m.overlayActive {
				m.overlayActive = false
				cmds = append(cmds, func() tea.Msg { return EscapeMsg{} })
				return m, tea.Batch(cmds...)
			}
			if m.runActive && m.cancelRun != nil {
				m.cancelRun()
				m.runActive = false
				m.cancelRun = nil
				m.statusMsg = "Interrupted"
				m.statusMsgExpiry = time.Now().Add(statusMsgDuration)
				return m, tea.Batch(cmds...)
			}
			if m.input.Value() != "" {
				// Clear input by sending Ctrl+C to the input area component.
				m.input, _ = m.input.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
				m.statusMsg = "Input cleared"
				m.statusMsgExpiry = time.Now().Add(statusMsgDuration)
				return m, tea.Batch(cmds...)
			}
			// No-op.
			return m, tea.Batch(cmds...)
		case key.Matches(msg, m.keys.ExpandTool):
			// Toggle expanded/collapsed state for the active tool call.
			if m.activeToolCallID != "" {
				if m.toolExpanded == nil {
					m.toolExpanded = make(map[string]bool)
				}
				m.toolExpanded[m.activeToolCallID] = !m.toolExpanded[m.activeToolCallID]
			}
		default:
			// Route to input area
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case inputarea.CommandSubmittedMsg:
		// Reset assistant text accumulator for the new user turn.
		m.lastAssistantText = ""
		// Add user message to viewport
		m.vp.AppendLine("\u276f " + msg.Value)
		m.vp.AppendLine("") // blank line after user message

	case AssistantDeltaMsg:
		m.lastAssistantText += msg.Delta
		m.vp.AppendLine(msg.Delta)

	case ThinkingDeltaMsg:
		// TODO Phase 1: route to thinking indicator

	case ToolStartMsg:
		// TODO Phase 2: route to tool use component

	case RunStartedMsg:
		m.RunID = msg.RunID
		m.runActive = true

	case RunCompletedMsg:
		m.runActive = false
		m.cancelRun = nil

	case RunFailedMsg:
		m.runActive = false
		m.cancelRun = nil

	case OverlayOpenMsg:
		m.overlayActive = true

	case OverlayCloseMsg:
		m.overlayActive = false

	case ClearMsg:
		// TODO Phase 1: clear viewport

	case SSEEventMsg, SSEErrorMsg, SSEDoneMsg, SSEDropMsg:
		// TODO: route SSE events
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model -- composes all components.
func (m Model) View() string {
	if !m.ready {
		return "Initializing...\n"
	}

	sep := m.renderSeparator()

	// Render the status bar, optionally with a transient status message overlay.
	statusBarView := m.statusBar.View()
	if m.statusMsg != "" && !time.Now().After(m.statusMsgExpiry) {
		statusBarView = m.statusMsg
	}

	// Stack: viewport / separator / input / separator / status bar
	sections := []string{
		m.vp.View(),
		sep,
		m.input.View(),
		sep,
		statusBarView,
	}

	return strings.Join(sections, "\n")
}

func (m Model) renderSeparator() string {
	if m.width <= 0 {
		return ""
	}
	return layout.NewSeparator(m.width, false).Render()
}
