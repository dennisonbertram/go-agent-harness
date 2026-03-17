package tui

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"go-agent-harness/cmd/harnesscli/tui/components/contextgrid"
	"go-agent-harness/cmd/harnesscli/tui/components/helpdialog"
	"go-agent-harness/cmd/harnesscli/tui/components/inputarea"
	"go-agent-harness/cmd/harnesscli/tui/components/layout"
	"go-agent-harness/cmd/harnesscli/tui/components/statspanel"
	"go-agent-harness/cmd/harnesscli/tui/components/statusbar"
	"go-agent-harness/cmd/harnesscli/tui/components/transcriptexport"
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

	// conversationID is the stable identifier for the current conversation.
	// It is set to the first run's ID when no conversation_id is supplied,
	// and passed on all subsequent runs so the harness links them together.
	conversationID string

	// runActive is true while a run is in flight.
	runActive bool

	// cancelRun holds the cancel func from the SSE bridge; nil when no run is active.
	cancelRun func()

	// sseCh is the channel delivering SSE messages from the active run's bridge.
	// nil when no run is active.
	sseCh <-chan tea.Msg

	// toolExpanded tracks which tool calls are in the expanded view, keyed by
	// tool call ID. True = expanded, absent/false = collapsed.
	toolExpanded map[string]bool

	// activeToolCallID is the ID of the currently active/selected tool call,
	// used when toggling expansion via Ctrl+O.
	activeToolCallID string

	// lastAssistantText accumulates all assistant deltas for the current run.
	lastAssistantText string

	// transcript accumulates entries for the current session (used by /export).
	transcript []transcriptexport.TranscriptEntry

	// overlayActive is true when an overlay (help, context, stats, etc.) is open.
	overlayActive bool

	// statusMsg is a transient overlay message shown on the status bar.
	statusMsg string
	// statusMsgExpiry is when statusMsg should be cleared.
	statusMsgExpiry time.Time

	// commandRegistry holds the dispatch table for slash commands.
	commandRegistry *CommandRegistry

	// Components
	statusBar   statusbar.Model
	vp          viewport.Model
	input       inputarea.Model
	helpDialog  helpdialog.Model
	contextGrid contextgrid.Model
	statsPanel  statspanel.Model
}

// New creates a new root Model.
func New(cfg TUIConfig) Model {
	m := Model{
		config:      cfg,
		keys:        DefaultKeyMap(),
		theme:       DefaultTheme(),
		helpDialog:  helpdialog.New(nil, nil, nil),
		contextGrid: contextgrid.New(),
		statsPanel:  statspanel.New(nil),
	}
	m.commandRegistry = m.buildCommandRegistry()
	return m
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

// ConversationID returns the current conversation ID (for testing and multi-turn use).
func (m Model) ConversationID() string {
	return m.conversationID
}

// LastAssistantText returns the accumulated assistant text for the current run (for testing).
func (m Model) LastAssistantText() string {
	return m.lastAssistantText
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
				// Clear input directly via Clear() — no fragile key simulation.
				m.input = m.input.Clear()
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
		// Check if it's a slash command; dispatch if so.
		if cmd, ok := ParseCommand(msg.Value); ok {
			result := m.commandRegistry.Dispatch(cmd)
			switch result.Status {
			case CmdOK:
				// Apply side effects for each built-in command.
				switch cmd.Name {
				case "clear":
					m.vp = viewport.New(m.width, m.layout.ViewportHeight)
					m.transcript = nil
				case "help":
					m.helpDialog = m.helpDialog.Open()
					m.overlayActive = true
				case "context":
					m.overlayActive = true
					cmds = append(cmds, func() tea.Msg { return OverlayOpenMsg{Kind: "context"} })
				case "stats":
					m.overlayActive = true
					cmds = append(cmds, func() tea.Msg { return OverlayOpenMsg{Kind: "stats"} })
				case "quit":
					return m, tea.Quit
				case "export":
					snapshot := make([]transcriptexport.TranscriptEntry, len(m.transcript))
					copy(snapshot, m.transcript)
					exporter := transcriptexport.NewExporter(".")
					cmds = append(cmds, func() tea.Msg {
						path, err := exporter.Export(snapshot)
						if err != nil {
							return ExportTranscriptMsg{FilePath: ""}
						}
						return ExportTranscriptMsg{FilePath: path}
					})
				default:
					if result.Output != "" {
						m.vp.AppendLine(result.Output)
						m.vp.AppendLine("")
					}
				}
			case CmdError:
				m.statusMsg = result.Output
				m.statusMsgExpiry = time.Now().Add(statusMsgDuration)
			case CmdUnknown:
				m.statusMsg = result.Hint
				m.statusMsgExpiry = time.Now().Add(statusMsgDuration)
			}
			return m, tea.Batch(cmds...)
		}
		// Normal user message: reset assistant text accumulator for the new user turn.
		m.lastAssistantText = ""
		// Record in transcript.
		m.transcript = append(m.transcript, transcriptexport.TranscriptEntry{
			Role:      "user",
			Content:   msg.Value,
			Timestamp: time.Now(),
		})
		// Add user message to viewport
		m.vp.AppendLine("\u276f " + msg.Value)
		m.vp.AppendLine("") // blank line after user message
		// Fire off the run against the harness API, carrying the current
		// conversationID so the harness links this turn to the conversation.
		cmds = append(cmds, startRunCmd(m.config.BaseURL, msg.Value, m.conversationID))

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
		// The harness auto-assigns conversation_id = run_id when none is
		// supplied. Record this as the conversationID for subsequent turns so
		// that follow-up messages are linked to the same conversation.
		if m.conversationID == "" {
			m.conversationID = msg.RunID
		}
		// Start the SSE bridge for this run only if no cancel func is already
		// set (e.g. injected by tests via WithCancelRun). This avoids overwriting
		// a test-supplied cancel with a real HTTP bridge.
		if m.cancelRun == nil {
			ch, cancel := startSSEForRun(m.config.BaseURL, msg.RunID)
			m.sseCh = ch
			m.cancelRun = cancel
			cmds = append(cmds, pollSSECmd(m.sseCh))
		}

	case RunCompletedMsg:
		m.runActive = false
		m.cancelRun = nil

	case RunFailedMsg:
		m.runActive = false
		m.cancelRun = nil
		m.sseCh = nil
		errMsg := "run failed"
		if msg.Error != "" {
			errMsg = msg.Error
		}
		m.vp.AppendLine("✗ " + errMsg)
		m.vp.AppendLine("")

	case OverlayOpenMsg:
		m.overlayActive = true

	case OverlayCloseMsg:
		m.overlayActive = false

	case ClearMsg:
		m.vp = viewport.New(m.width, m.layout.ViewportHeight)
		m.transcript = nil

	case ExportTranscriptMsg:
		if msg.FilePath != "" {
			m.statusMsg = "Transcript saved to " + msg.FilePath
		} else {
			m.statusMsg = "Export failed"
		}
		m.statusMsgExpiry = time.Now().Add(statusMsgDuration)

	case SSEEventMsg:
		// Route event to viewport based on type.
		switch msg.EventType {
		case "assistant.message.delta":
			var p struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(msg.Raw, &p); err == nil && p.Content != "" {
				m.lastAssistantText += p.Content
				m.vp.AppendLine(p.Content)
			}
		case "assistant.thinking.delta":
			// Thinking deltas are shown faintly — skip for now to keep output clean.
		case "tool.call.started":
			var p struct {
				Tool   string `json:"tool"`
				CallID string `json:"call_id"`
			}
			if err := json.Unmarshal(msg.Raw, &p); err == nil {
				m.vp.AppendLine("⏺ " + p.Tool + "(" + p.CallID + ")")
			}
		case "tool.call.completed":
			var p struct {
				Tool   string `json:"tool"`
				CallID string `json:"call_id"`
			}
			if err := json.Unmarshal(msg.Raw, &p); err == nil {
				m.vp.AppendLine("  ✓ " + p.Tool + " done")
			}
		}
		// Continue polling the SSE channel.
		if m.sseCh != nil {
			cmds = append(cmds, pollSSECmd(m.sseCh))
		}

	case SSEErrorMsg:
		m.vp.AppendLine("⚠ stream error: " + msg.Err.Error())
		if m.sseCh != nil {
			cmds = append(cmds, pollSSECmd(m.sseCh))
		}

	case SSEDoneMsg:
		m.runActive = false
		m.sseCh = nil
		if m.cancelRun != nil {
			m.cancelRun()
			m.cancelRun = nil
		}
		if msg.EventType == "run.failed" {
			for _, line := range formatRunError(msg.Error) {
				m.vp.AppendLine(line)
			}
		}
		m.vp.AppendLine("")

	case SSEDropMsg:
		// Dropped message — continue polling.
		if m.sseCh != nil {
			cmds = append(cmds, pollSSECmd(m.sseCh))
		}
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

// buildCommandRegistry wires built-in slash commands. Each handler returns a
// CommandResult that signals the outcome; the caller in Update handles any
// required tea.Cmd side-effects based on the command name.
func (m *Model) buildCommandRegistry() *CommandRegistry {
	r := newEmptyCommandRegistry()

	r.Register(CommandEntry{
		Name:        "clear",
		Description: "Clear conversation history",
		Handler: func(cmd Command) CommandResult {
			return CommandResult{Status: CmdOK}
		},
	})

	r.Register(CommandEntry{
		Name:        "help",
		Description: "Show help dialog",
		Handler: func(cmd Command) CommandResult {
			return CommandResult{Status: CmdOK}
		},
	})

	r.Register(CommandEntry{
		Name:        "context",
		Description: "Show context usage grid",
		Handler: func(cmd Command) CommandResult {
			return CommandResult{Status: CmdOK}
		},
	})

	r.Register(CommandEntry{
		Name:        "stats",
		Description: "Show usage statistics",
		Handler: func(cmd Command) CommandResult {
			return CommandResult{Status: CmdOK}
		},
	})

	r.Register(CommandEntry{
		Name:        "quit",
		Description: "Quit the TUI",
		Handler: func(cmd Command) CommandResult {
			return CommandResult{Status: CmdOK}
		},
	})

	r.Register(CommandEntry{
		Name:        "export",
		Description: "Export conversation transcript to a markdown file",
		Handler: func(cmd Command) CommandResult {
			return CommandResult{Status: CmdOK}
		},
	})

	return r
}
