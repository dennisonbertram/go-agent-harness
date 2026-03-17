package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	harnessconfig "go-agent-harness/cmd/harnesscli/config"
	"go-agent-harness/cmd/harnesscli/tui/components/contextgrid"
	"go-agent-harness/cmd/harnesscli/tui/components/helpdialog"
	"go-agent-harness/cmd/harnesscli/tui/components/inputarea"
	"go-agent-harness/cmd/harnesscli/tui/components/layout"
	"go-agent-harness/cmd/harnesscli/tui/components/modelswitcher"
	"go-agent-harness/cmd/harnesscli/tui/components/slashcomplete"
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

	// responseStarted tracks whether the first assistant delta for the current
	// run has been written to the viewport. On the first delta we call
	// AppendLine("") to start a fresh line; subsequent deltas use AppendChunk.
	responseStarted bool

	// transcript accumulates entries for the current session (used by /export).
	transcript []transcriptexport.TranscriptEntry

	// usageDataPoints accumulates per-day DataPoints for the stats panel.
	// Updated whenever a usage.delta SSE event is received.
	usageDataPoints []statspanel.DataPoint

	// cumulativeCostUSD is the running total cost for the current session.
	cumulativeCostUSD float64

	// totalTokens is the cumulative token count for the context grid.
	totalTokens int

	// overlayActive is true when an overlay (help, context, stats, etc.) is open.
	overlayActive bool

	// activeOverlay identifies which overlay is currently displayed.
	// Valid values: "", "help", "stats", "context".
	activeOverlay string

	// statusMsg is a transient overlay message shown on the status bar.
	statusMsg string
	// statusMsgExpiry is when statusMsg should be cleared.
	statusMsgExpiry time.Time

	// commandRegistry holds the dispatch table for slash commands.
	commandRegistry *CommandRegistry

	// autocompleteProvider is stored here so it can be re-wired whenever the
	// input component is re-created (e.g. on WindowSizeMsg).
	autocompleteProvider inputarea.AutocompleteProvider

	// slashComplete is the autocomplete dropdown shown when the user types "/".
	slashComplete slashcomplete.Model

	// modelSwitcher is the 2-level model + reasoning overlay.
	modelSwitcher modelswitcher.Model

	// selectedModel is the currently active model ID.
	selectedModel string

	// selectedProvider is the currently active provider name.
	selectedProvider string

	// selectedReasoningEffort is the currently active reasoning effort.
	selectedReasoningEffort string

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
		config:        cfg,
		keys:          DefaultKeyMap(),
		theme:         DefaultTheme(),
		contextGrid:   contextgrid.New(),
		statsPanel:    statspanel.New(nil),
		selectedModel: cfg.Model,
	}
	m.modelSwitcher = modelswitcher.New(cfg.Model)
	// Load starred models from persistent config.
	if persistCfg, err := harnessconfig.Load(); err == nil {
		m.modelSwitcher = m.modelSwitcher.WithStarred(persistCfg.StarredModels)
	}
	m.commandRegistry = m.buildCommandRegistry()
	// Wire help dialog with real command list and keybindings derived from the
	// registered commands and the default key map.
	m.helpDialog = buildHelpDialog(m.commandRegistry, m.keys)
	// Wire tab completion: derive the provider from the registered commands so
	// it stays in sync with whatever commands are registered at startup.
	m = m.WithAutocompleteProvider(buildSlashCommandProvider(m.commandRegistry))
	// Wire slash-complete dropdown.
	m.slashComplete = buildSlashComplete(m.commandRegistry)
	return m
}

// buildHelpDialog constructs a helpdialog.Model populated with the commands from
// the registry and keybindings from the key map.
func buildHelpDialog(reg *CommandRegistry, keys KeyMap) helpdialog.Model {
	entries := reg.All()
	cmds := make([]helpdialog.CommandEntry, len(entries))
	for i, e := range entries {
		cmds[i] = helpdialog.CommandEntry{
			Name:        e.Name,
			Description: e.Description,
		}
	}

	kbs := []helpdialog.KeyEntry{
		{Keys: "enter", Description: keys.Submit.Help().Desc},
		{Keys: "shift+enter / ctrl+j", Description: keys.Newline.Help().Desc},
		{Keys: "up / ctrl+p", Description: keys.ScrollUp.Help().Desc},
		{Keys: "down / ctrl+n", Description: keys.ScrollDown.Help().Desc},
		{Keys: "pgup", Description: keys.PageUp.Help().Desc},
		{Keys: "pgdn", Description: keys.PageDown.Help().Desc},
		{Keys: "esc", Description: keys.Interrupt.Help().Desc},
		{Keys: "ctrl+s", Description: keys.Copy.Help().Desc},
		{Keys: "ctrl+c", Description: keys.Quit.Help().Desc},
	}

	about := []string{
		"go-agent-harness",
		"Type /help to see this dialog",
		"Type /stats for usage statistics",
		"Type /context for context window usage",
	}

	return helpdialog.New(cmds, kbs, about)
}

// WithAutocompleteProvider returns a copy of the Model with the given autocomplete
// provider wired into the input area.  The provider is also stored on the Model
// so it can be re-applied whenever the input component is re-created (e.g. on
// every WindowSizeMsg).
func (m Model) WithAutocompleteProvider(fn inputarea.AutocompleteProvider) Model {
	m.autocompleteProvider = fn
	m.input = m.input.SetAutocompleteProvider(fn)
	return m
}

// buildSlashComplete constructs a slashcomplete.Model populated with the
// commands from the registry.
func buildSlashComplete(reg *CommandRegistry) slashcomplete.Model {
	entries := reg.All()
	suggestions := make([]slashcomplete.Suggestion, len(entries))
	for i, e := range entries {
		suggestions[i] = slashcomplete.Suggestion{
			Name:        e.Name,
			Description: e.Description,
		}
	}
	return slashcomplete.New(suggestions)
}

// syncSlashComplete updates the dropdown state to match the current input value.
// Opens/filters when input starts with "/"; closes otherwise.
func syncSlashComplete(m slashcomplete.Model, input string) slashcomplete.Model {
	if strings.HasPrefix(input, "/") {
		query := strings.TrimPrefix(input, "/")
		// Strip any trailing space (command fully typed)
		if strings.Contains(query, " ") {
			return m.Close()
		}
		return m.Open().SetQuery(query)
	}
	return m.Close()
}

// buildSlashCommandProvider returns an AutocompleteProvider that completes
// slash commands drawn from the given registry.
func buildSlashCommandProvider(reg *CommandRegistry) inputarea.AutocompleteProvider {
	return func(input string) []string {
		if !strings.HasPrefix(input, "/") {
			return nil
		}
		entries := reg.All()
		var matches []string
		for _, e := range entries {
			full := "/" + e.Name
			if strings.HasPrefix(full, input) {
				matches = append(matches, full)
			}
		}
		return matches
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

// ActiveOverlay returns the name of the currently active overlay (for testing).
// Returns "" when no overlay is open. Valid values: "help", "stats", "context".
func (m Model) ActiveOverlay() string {
	return m.activeOverlay
}

// ConversationID returns the current conversation ID (for testing and multi-turn use).
func (m Model) ConversationID() string {
	return m.conversationID
}

// SelectedModel returns the currently active model ID (for testing).
func (m Model) SelectedModel() string {
	return m.selectedModel
}

// SelectedReasoningEffort returns the currently active reasoning effort (for testing).
func (m Model) SelectedReasoningEffort() string {
	return m.selectedReasoningEffort
}

// displayModelName returns the display name for a model ID, or the ID itself
// if not found in DefaultModels.
func displayModelName(id string) string {
	for _, dm := range modelswitcher.DefaultModels {
		if dm.ID == id {
			return dm.DisplayName
		}
	}
	return id
}

// LastAssistantText returns the accumulated assistant text for the current run (for testing).
func (m Model) LastAssistantText() string {
	return m.lastAssistantText
}

// Input returns the current value of the input area (for testing).
func (m Model) Input() string {
	return m.input.Value()
}

// ViewportScrollOffset returns the current viewport scroll offset (lines from bottom).
// This is used by tests to assert scrolling behavior.
func (m Model) ViewportScrollOffset() int {
	return m.vp.ScrollOffset()
}

// ViewportAtBottom reports whether the viewport is at the bottom.
// This is used by tests to assert scroll state.
func (m Model) ViewportAtBottom() bool {
	return m.vp.AtBottom()
}

// Transcript returns a copy of the current transcript entries (for testing).
func (m Model) Transcript() []transcriptexport.TranscriptEntry {
	cp := make([]transcriptexport.TranscriptEntry, len(m.transcript))
	copy(cp, m.transcript)
	return cp
}

// WithCancelRun returns a copy of the Model with the given cancel func set.
// This is used to wire up the SSE bridge cancel func before a run starts.
func (m Model) WithCancelRun(cancel func()) Model {
	m.cancelRun = cancel
	return m
}

// statusTickCmd returns a tea.Cmd that fires statusTickMsg after duration d.
func statusTickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return statusTickMsg{} })
}

// StatusTickMsgForTesting returns a statusTickMsg as a tea.Msg for use in
// external test packages that need to drive the auto-dismiss path.
func StatusTickMsgForTesting() tea.Msg { return statusTickMsg{} }

// setStatusMsg sets the transient status message and schedules its auto-dismiss tick.
// The returned tea.Cmd must be appended to the caller's cmds slice.
func (m *Model) setStatusMsg(msg string) tea.Cmd {
	m.statusMsg = msg
	m.statusMsgExpiry = time.Now().Add(statusMsgDuration)
	return statusTickCmd(statusMsgDuration)
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
		// Re-wire autocomplete provider each time the input is re-created.
		if m.autocompleteProvider != nil {
			m.input = m.input.SetAutocompleteProvider(m.autocompleteProvider)
		}

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			// If a run is active, Ctrl+C cancels the run instead of quitting.
			if m.runActive && m.cancelRun != nil {
				m.cancelRun()
				m.runActive = false
				m.cancelRun = nil
				cmds = append(cmds, m.setStatusMsg("Interrupted"))
				// Do NOT quit — return without tea.Quit
				return m, tea.Batch(cmds...)
			}
			// No active run: fall through to default quit behavior.
			return m, tea.Quit
		case key.Matches(msg, m.keys.Copy):
			ok := CopyToClipboard(m.lastAssistantText)
			if ok {
				cmds = append(cmds, m.setStatusMsg("Copied!"))
			} else {
				cmds = append(cmds, m.setStatusMsg("Copy unavailable"))
			}
		case key.Matches(msg, m.keys.Interrupt):
			// Always close the slash-complete dropdown on Escape.
			m.slashComplete = m.slashComplete.Close()
			// Multi-priority Escape semantics (highest to lowest):
			// 1. model overlay  → back/close (2-level)
			// 2. overlayActive  → close overlay
			// 3. runActive      → cancel run
			// 4. input has text → clear input
			// 5. otherwise      → no-op
			if m.activeOverlay == "model" {
				if m.modelSwitcher.IsReasoningMode() {
					// Escape at Level-1: go back to Level-0.
					m.modelSwitcher = m.modelSwitcher.ExitReasoningMode()
					return m, tea.Batch(cmds...)
				}
				// Escape at Level-0 with active search: clear search first.
				if m.modelSwitcher.SearchQuery() != "" {
					m.modelSwitcher = m.modelSwitcher.SetSearch("")
					return m, tea.Batch(cmds...)
				}
				// Escape at Level-0 with no search: close overlay entirely.
				m.modelSwitcher = m.modelSwitcher.Close()
				m.overlayActive = false
				m.activeOverlay = ""
				return m, tea.Batch(cmds...)
			}
			if m.overlayActive {
				m.overlayActive = false
				m.activeOverlay = ""
				m.helpDialog = m.helpDialog.Close()
				cmds = append(cmds, func() tea.Msg { return EscapeMsg{} })
				return m, tea.Batch(cmds...)
			}
			if m.runActive && m.cancelRun != nil {
				m.cancelRun()
				m.runActive = false
				m.cancelRun = nil
				cmds = append(cmds, m.setStatusMsg("Interrupted"))
				return m, tea.Batch(cmds...)
			}
			if m.input.Value() != "" {
				// Clear input directly via Clear() — no fragile key simulation.
				m.input = m.input.Clear()
				cmds = append(cmds, m.setStatusMsg("Input cleared"))
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
		case key.Matches(msg, m.keys.Submit):
			// When the model overlay is active, Enter navigates or confirms.
			if m.overlayActive && m.activeOverlay == "model" {
				if m.modelSwitcher.IsReasoningMode() {
					// Level-1: accept the reasoning level.
					re, _ := m.modelSwitcher.AcceptReasoning()
					// Get the model entry from the switcher (the highlighted model in Level-0).
					modelEntry, _ := m.modelSwitcher.Accept()
					m.modelSwitcher = m.modelSwitcher.WithCurrentReasoning(re.ID).ExitReasoningMode().Close()
					m.overlayActive = false
					m.activeOverlay = ""
					modelID := modelEntry.ID
					modelProvider := modelEntry.Provider
					reasoningID := re.ID
					cmds = append(cmds, func() tea.Msg {
						return ModelSelectedMsg{ModelID: modelID, Provider: modelProvider, ReasoningEffort: reasoningID}
					})
					return m, tea.Batch(cmds...)
				}
				// Level-0: check if the selected model requires reasoning effort.
				entry, _ := m.modelSwitcher.Accept()
				if entry.ReasoningMode {
					// Enter reasoning level selection.
					m.modelSwitcher = m.modelSwitcher.EnterReasoningMode()
					return m, tea.Batch(cmds...)
				}
				// Non-reasoning model: accept immediately.
				m.modelSwitcher = m.modelSwitcher.Close()
				m.overlayActive = false
				m.activeOverlay = ""
				entryID := entry.ID
				entryProvider := entry.Provider
				cmds = append(cmds, func() tea.Msg {
					return ModelSelectedMsg{ModelID: entryID, Provider: entryProvider, ReasoningEffort: ""}
				})
				return m, tea.Batch(cmds...)
			}
			// When the dropdown is active, Enter accepts the selected suggestion
			// instead of submitting the input as a message.
			if m.slashComplete.IsActive() {
				newModel, accepted := m.slashComplete.Accept()
				m.slashComplete = newModel
				if accepted != "" {
					m.input = m.input.SetValue(accepted)
				}
				return m, tea.Batch(cmds...)
			}
			// No active dropdown — pass Enter to the input area normally.
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		case key.Matches(msg, m.keys.ScrollUp):
			// When the model overlay is active, Up navigates the model list or reasoning levels.
			if m.overlayActive && m.activeOverlay == "model" {
				if m.modelSwitcher.IsReasoningMode() {
					m.modelSwitcher = m.modelSwitcher.ReasoningUp()
				} else {
					m.modelSwitcher = m.modelSwitcher.SelectUp()
				}
				return m, tea.Batch(cmds...)
			}
			// When the dropdown is active, Up navigates the dropdown.
			if m.slashComplete.IsActive() {
				m.slashComplete = m.slashComplete.Up()
				return m, tea.Batch(cmds...)
			}
			m.vp.ScrollUp(1)
		case key.Matches(msg, m.keys.ScrollDown):
			// When the model overlay is active, Down navigates the model list or reasoning levels.
			if m.overlayActive && m.activeOverlay == "model" {
				if m.modelSwitcher.IsReasoningMode() {
					m.modelSwitcher = m.modelSwitcher.ReasoningDown()
				} else {
					m.modelSwitcher = m.modelSwitcher.SelectDown()
				}
				return m, tea.Batch(cmds...)
			}
			// When the dropdown is active, Down navigates the dropdown.
			if m.slashComplete.IsActive() {
				m.slashComplete = m.slashComplete.Down()
				return m, tea.Batch(cmds...)
			}
			m.vp.ScrollDown(1)
		case key.Matches(msg, m.keys.PageUp):
			m.vp.ScrollUp(m.vp.Height() / 2)
		case key.Matches(msg, m.keys.PageDown):
			m.vp.ScrollDown(m.vp.Height() / 2)
		default:
			// When model overlay is open at Level-0, intercept keys for search and star.
			if m.overlayActive && m.activeOverlay == "model" && !m.modelSwitcher.IsReasoningMode() {
				switch msg.Type {
				case tea.KeyBackspace, tea.KeyDelete:
					q := m.modelSwitcher.SearchQuery()
					if len(q) > 0 {
						runes := []rune(q)
						m.modelSwitcher = m.modelSwitcher.SetSearch(string(runes[:len(runes)-1]))
					}
					return m, tea.Batch(cmds...)
				case tea.KeyRunes:
					// 's' toggles star BEFORE the generic rune-as-search handler.
					if msg.String() == "s" {
						m.modelSwitcher = m.modelSwitcher.ToggleStar()
						// Persist to config.
						if persistCfg, err := harnessconfig.Load(); err == nil {
							persistCfg.StarredModels = m.modelSwitcher.StarredIDs()
							_ = harnessconfig.Save(persistCfg)
						}
						return m, tea.Batch(cmds...)
					}
					// All other printable characters accumulate into search query.
					m.modelSwitcher = m.modelSwitcher.SetSearch(m.modelSwitcher.SearchQuery() + msg.String())
					return m, tea.Batch(cmds...)
				}
				return m, tea.Batch(cmds...)
			}
			// Route to input area
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			// Sync autocomplete dropdown with current input value.
			m.slashComplete = syncSlashComplete(m.slashComplete, m.input.Value())
		}

	case inputarea.CommandSubmittedMsg:
		// Close the dropdown whenever a command is submitted.
		m.slashComplete = m.slashComplete.Close()
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
					m.slashComplete = m.slashComplete.Close()
				case "help":
					m.helpDialog = m.helpDialog.Open()
					m.overlayActive = true
					m.activeOverlay = "help"
				case "context":
					m.overlayActive = true
					m.activeOverlay = "context"
					cmds = append(cmds, func() tea.Msg { return OverlayOpenMsg{Kind: "context"} })
				case "stats":
					m.overlayActive = true
					m.activeOverlay = "stats"
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
				case "model":
					// Preserve currently starred models across re-open.
					currentStarred := m.modelSwitcher.StarredIDs()
					m.modelSwitcher = modelswitcher.New(m.selectedModel).Open()
					m.modelSwitcher = m.modelSwitcher.WithCurrentReasoning(m.selectedReasoningEffort)
					m.modelSwitcher = m.modelSwitcher.WithStarred(currentStarred)
					m.modelSwitcher = m.modelSwitcher.SetLoading(true)
					m.overlayActive = true
					m.activeOverlay = "model"
					cmds = append(cmds, fetchModelsCmd(m.config.BaseURL))
				case "subagents":
					cmds = append(cmds, m.setStatusMsg("Loading subagents..."))
					cmds = append(cmds, loadSubagentsCmd(m.config.BaseURL))
				default:
					if result.Output != "" {
						m.vp.AppendLine(result.Output)
						m.vp.AppendLine("")
					}
				}
			case CmdError:
				cmds = append(cmds, m.setStatusMsg(result.Output))
			case CmdUnknown:
				cmds = append(cmds, m.setStatusMsg(result.Hint))
			}
			return m, tea.Batch(cmds...)
		}
		// Normal user message: reset assistant text accumulator for the new user turn.
		m.lastAssistantText = ""
		m.responseStarted = false
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
		cmds = append(cmds, startRunCmd(m.config.BaseURL, msg.Value, m.conversationID, m.selectedModel, m.selectedProvider, m.selectedReasoningEffort))

	case AssistantDeltaMsg:
		m.lastAssistantText += msg.Delta
		if !m.responseStarted {
			m.vp.AppendLine("")
			m.responseStarted = true
		}
		m.vp.AppendChunk(msg.Delta)

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
		if msg.Kind != "" {
			m.activeOverlay = msg.Kind
		}

	case OverlayCloseMsg:
		m.overlayActive = false
		m.activeOverlay = ""
		m.helpDialog = m.helpDialog.Close()

	case ClearMsg:
		m.vp = viewport.New(m.width, m.layout.ViewportHeight)
		m.transcript = nil

	case ExportTranscriptMsg:
		if msg.FilePath != "" {
			cmds = append(cmds, m.setStatusMsg("Transcript saved to "+msg.FilePath))
		} else {
			cmds = append(cmds, m.setStatusMsg("Export failed"))
		}

	case SubagentsLoadedMsg:
		for _, line := range formatSubagentsLines(msg.Subagents) {
			m.vp.AppendLine(line)
		}
		m.vp.AppendLine("")
		cmds = append(cmds, m.setStatusMsg(fmt.Sprintf("Loaded %d subagent(s)", len(msg.Subagents))))

	case SubagentsLoadFailedMsg:
		cmds = append(cmds, m.setStatusMsg("Load subagents failed: "+msg.Err))

	case SSEEventMsg:
		// Route event to viewport based on type.
		switch msg.EventType {
		case "assistant.message.delta":
			var p struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(msg.Raw, &p); err == nil && p.Content != "" {
				m.lastAssistantText += p.Content
				if !m.responseStarted {
					// Start a fresh line for the assistant response so that any
					// preceding tool-call lines are not contaminated by the chunk.
					m.vp.AppendLine("")
					m.responseStarted = true
				}
				m.vp.AppendChunk(p.Content) // accumulate on same line
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
		case "usage.delta":
			var p struct {
				CumulativeUsage struct {
					TotalTokens int `json:"total_tokens"`
				} `json:"cumulative_usage"`
				CumulativeCostUSD float64 `json:"cumulative_cost_usd"`
			}
			if err := json.Unmarshal(msg.Raw, &p); err == nil {
				m.cumulativeCostUSD = p.CumulativeCostUSD
				m.totalTokens = p.CumulativeUsage.TotalTokens
				// Update today's data point for the stats panel.
				m.usageDataPoints = upsertTodayDataPoint(m.usageDataPoints, 1, p.CumulativeCostUSD)
				m.statsPanel = statspanel.New(m.usageDataPoints)
				// Update context grid token count.
				m.contextGrid.UsedTokens = m.totalTokens
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
		m.responseStarted = false
		if m.cancelRun != nil {
			m.cancelRun()
			m.cancelRun = nil
		}
		// Record completed assistant response in transcript.
		if m.lastAssistantText != "" {
			m.transcript = append(m.transcript, transcriptexport.TranscriptEntry{
				Role:      "assistant",
				Content:   m.lastAssistantText,
				Timestamp: time.Now(),
			})
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

	case ModelsFetchedMsg:
		currentStarred := m.modelSwitcher.StarredIDs()
		m.modelSwitcher = m.modelSwitcher.WithModels(msg.Models).SetLoading(false)
		m.modelSwitcher = m.modelSwitcher.WithStarred(currentStarred)

	case ModelsFetchErrorMsg:
		m.modelSwitcher = m.modelSwitcher.SetLoadError("Error loading models: " + msg.Err)

	case ModelSelectedMsg:
		// Preserve starred models when model is selected.
		currentStarred := m.modelSwitcher.StarredIDs()
		m.selectedModel = msg.ModelID
		m.selectedProvider = msg.Provider
		m.selectedReasoningEffort = msg.ReasoningEffort
		m.modelSwitcher = modelswitcher.New(msg.ModelID)
		m.modelSwitcher = m.modelSwitcher.WithCurrentReasoning(msg.ReasoningEffort)
		m.modelSwitcher = m.modelSwitcher.WithStarred(currentStarred)
		m.statusBar.SetModel(displayModelName(msg.ModelID))
		label := displayModelName(msg.ModelID)
		if msg.ReasoningEffort != "" {
			label += " (" + msg.ReasoningEffort + ")"
		}
		cmds = append(cmds, m.setStatusMsg("Model: "+label))

	case statusTickMsg:
		// Only clear if the message hasn't been replaced with a newer one.
		if m.statusMsg != "" && time.Now().After(m.statusMsgExpiry) {
			m.statusMsg = ""
			m.statusMsgExpiry = time.Time{}
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

	// Render viewport OR active overlay.
	var mainContent string
	if m.overlayActive {
		switch m.activeOverlay {
		case "help":
			mainContent = m.helpDialog.View(m.width, m.layout.ViewportHeight)
		case "stats":
			mainContent = m.statsPanel.SetWidth(m.width).View()
		case "context":
			cg := m.contextGrid
			cg.Width = m.width
			mainContent = cg.View()
			if mainContent == "" {
				mainContent = "Context grid not available"
			}
		case "model":
			mainContent = m.modelSwitcher.View(m.width)
		default:
			// Unknown overlay kind — fall back to viewport.
			mainContent = m.vp.View()
		}
	} else {
		mainContent = m.vp.View()
	}

	// Stack: main content / separator / autocomplete dropdown / input / separator / status bar
	inputView := m.input.View()
	dropdownView := m.slashComplete.View(m.width)

	sections := []string{
		mainContent,
		sep,
	}
	if dropdownView != "" {
		sections = append(sections, dropdownView)
	}
	sections = append(sections, inputView, sep, statusBarView)

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

	r.Register(CommandEntry{
		Name:        "subagents",
		Description: "List managed subagents and their isolation state",
		Handler: func(cmd Command) CommandResult {
			return CommandResult{Status: CmdOK}
		},
	})

	r.Register(CommandEntry{
		Name:        "model",
		Description: "Switch model and reasoning effort",
		Handler: func(cmd Command) CommandResult {
			return CommandResult{Status: CmdOK}
		},
	})

	return r
}

// upsertTodayDataPoint updates (or inserts) a DataPoint for today in the given
// slice.  count is added to the existing count and cost replaces the cost
// (since usage.delta carries cumulative values).
func upsertTodayDataPoint(pts []statspanel.DataPoint, count int, cost float64) []statspanel.DataPoint {
	today := time.Now()
	todayKey := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	for i := range pts {
		dp := pts[i]
		k := time.Date(dp.Date.Year(), dp.Date.Month(), dp.Date.Day(), 0, 0, 0, 0, time.UTC)
		if k.Equal(todayKey) {
			pts[i].Count += count
			pts[i].Cost = cost
			return pts
		}
	}
	return append(pts, statspanel.DataPoint{
		Date:  todayKey,
		Count: count,
		Cost:  cost,
	})
}

func formatSubagentsLines(items []RemoteSubagent) []string {
	if len(items) == 0 {
		return []string{"No managed subagents."}
	}

	lines := make([]string, 0, len(items)*2)
	for _, item := range items {
		summary := fmt.Sprintf("%s [%s] %s (%s)", item.ID, item.Status, item.Isolation, item.CleanupPolicy)
		if item.WorkspaceCleaned {
			summary += " cleaned"
		}
		lines = append(lines, summary)

		details := make([]string, 0, 3)
		if item.BranchName != "" {
			details = append(details, "branch="+item.BranchName)
		}
		if item.BaseRef != "" {
			details = append(details, "base="+item.BaseRef)
		}
		if item.WorkspacePath != "" {
			details = append(details, "path="+item.WorkspacePath)
		}
		if len(details) > 0 {
			lines = append(lines, "  "+strings.Join(details, " "))
		}
	}

	return lines
}
