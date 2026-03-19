package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

type gatewayOption struct {
	ID    string
	Label string
	Desc  string
}

type apiKeyProvider struct {
	Name       string
	Configured bool
	APIKeyEnv  string
}

var gatewayOptions = []gatewayOption{
	{ID: "", Label: "Direct", Desc: "Use each model's native provider"},
	{ID: "openrouter", Label: "OpenRouter", Desc: "Route all models via openrouter.ai"},
}

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

	// selectedGateway is the active routing gateway ("" = direct, "openrouter" = OpenRouter).
	selectedGateway string
	// gatewaySelected is the cursor index in the gatewayOptions overlay.
	gatewaySelected int

	// apiKeyProviders holds the provider list from the server for the /keys overlay.
	apiKeyProviders []apiKeyProvider
	// apiKeyCursor is the list cursor in the /keys overlay.
	apiKeyCursor int
	// apiKeyInput is the text being typed in the /keys overlay input mode.
	apiKeyInput string
	// apiKeyInputMode is true when the user is typing a key value.
	apiKeyInputMode bool
	// pendingAPIKeys holds keys loaded from config, replayed on Init().
	pendingAPIKeys map[string]string

	// modelConfigMode is true when the Level-1 config panel is showing.
	modelConfigMode bool
	// modelConfigEntry is the model being configured.
	modelConfigEntry modelswitcher.ModelEntry
	// modelConfigSection is the focused section index (0=gateway, 1=apikey, 2=reasoning).
	modelConfigSection int
	// modelConfigGatewayCursor is the gateway option cursor in the config panel.
	modelConfigGatewayCursor int
	// modelConfigReasoningCursor is the reasoning effort cursor in the config panel.
	modelConfigReasoningCursor int
	// modelConfigKeyInputMode is true when typing a key value in the config panel.
	modelConfigKeyInputMode bool
	// modelConfigKeyInput is the text being typed.
	modelConfigKeyInput string

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
	// Load starred models, gateway, and API keys from persistent config.
	if persistCfg, err := harnessconfig.Load(); err == nil {
		m.modelSwitcher = m.modelSwitcher.WithStarred(persistCfg.StarredModels)
		m.selectedGateway = persistCfg.Gateway
		m.pendingAPIKeys = persistCfg.APIKeys
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

// gatewayIndex returns the index of the gateway option with the given ID,
// or 0 if not found.
func gatewayIndex(id string) int {
	for i, g := range gatewayOptions {
		if g.ID == id {
			return i
		}
	}
	return 0
}

// reasoningLevelIndex returns the index of the reasoning level with the given
// effort ID, or 0 if not found.
func reasoningLevelIndex(effort string) int {
	for i, r := range modelswitcher.ReasoningLevels {
		if r.ID == effort {
			return i
		}
	}
	return 0
}

// providerKeyConfigured returns true if the given provider key has a configured
// API key in the loaded provider list or in pendingAPIKeys (for OpenRouter and
// other keys set via /keys before the server sync completes).
func (m Model) providerKeyConfigured(providerKey string) bool {
	for _, p := range m.apiKeyProviders {
		if p.Name == providerKey && p.Configured {
			return true
		}
	}
	// Fallback: check locally cached keys (set via /keys or loaded from config).
	if key, ok := m.pendingAPIKeys[providerKey]; ok && key != "" {
		return true
	}
	return false
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
	var cmds []tea.Cmd
	for provider, apiKey := range m.pendingAPIKeys {
		cmds = append(cmds, setProviderKeyCmd(m.config.BaseURL, provider, apiKey))
	}
	if len(cmds) > 0 {
		return tea.Batch(cmds...)
	}
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
		m.statusBar.SetModel(m.statusBarModelLabel())
		m.statusBar.SetCost(m.cumulativeCostUSD)
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
			// 0. apikeys overlay → back from input or close
			// 1. model overlay  → back/close (2-level)
			// 2. overlayActive  → close overlay
			// 3. runActive      → cancel run
			// 4. input has text → clear input
			// 5. otherwise      → no-op
			if m.overlayActive && m.activeOverlay == "apikeys" {
				if m.apiKeyInputMode {
					m.apiKeyInputMode = false
					m.apiKeyInput = ""
				} else {
					m.overlayActive = false
					m.activeOverlay = ""
				}
				return m, tea.Batch(cmds...)
			}
			if m.activeOverlay == "provider" {
				m.overlayActive = false
				m.activeOverlay = ""
				return m, tea.Batch(cmds...)
			}
			if m.activeOverlay == "model" {
				// Config panel key input mode → exit key input (keep config panel open).
				if m.modelConfigMode && m.modelConfigKeyInputMode {
					m.modelConfigKeyInputMode = false
					m.modelConfigKeyInput = ""
					return m, tea.Batch(cmds...)
				}
				// Config panel → back to Level-0 model list.
				if m.modelConfigMode {
					m.modelConfigMode = false
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
			// When the apikeys overlay is active, Enter enters input mode or confirms.
			if m.overlayActive && m.activeOverlay == "apikeys" {
				if m.apiKeyInputMode && m.apiKeyInput != "" {
					provider := m.apiKeyProviders[m.apiKeyCursor].Name
					apiKey := m.apiKeyInput
					m.apiKeyInputMode = false
					m.apiKeyInput = ""
					cmds = append(cmds, setProviderKeyCmd(m.config.BaseURL, provider, apiKey))
				} else if !m.apiKeyInputMode && len(m.apiKeyProviders) > 0 {
					m.apiKeyInputMode = true
				}
				return m, tea.Batch(cmds...)
			}
			// When the provider overlay is active, Enter confirms the selection.
			if m.overlayActive && m.activeOverlay == "provider" {
				chosen := gatewayOptions[m.gatewaySelected]
				m.overlayActive = false
				m.activeOverlay = ""
				gateway := chosen.ID
				cmds = append(cmds, func() tea.Msg {
					return GatewaySelectedMsg{Gateway: gateway}
				})
				return m, tea.Batch(cmds...)
			}
			// When the model overlay is active, Enter navigates or confirms.
			if m.overlayActive && m.activeOverlay == "model" {
				// Config panel is active.
				if m.modelConfigMode {
					if m.modelConfigKeyInputMode {
						// Confirm key entry.
						if m.modelConfigKeyInput != "" {
							provider := m.modelConfigEntry.Provider
							key := m.modelConfigKeyInput
							m.modelConfigKeyInputMode = false
							m.modelConfigKeyInput = ""
							cmds = append(cmds, setProviderKeyCmd(m.config.BaseURL, provider, key))
						}
						return m, tea.Batch(cmds...)
					}
					// Enter in config panel (not in key input) → confirm and close.
					gateway := gatewayOptions[m.modelConfigGatewayCursor].ID
					reasoningEffort := ""
					if m.modelConfigEntry.ReasoningMode {
						reasoningEffort = modelswitcher.ReasoningLevels[m.modelConfigReasoningCursor].ID
					}
					m.modelSwitcher = m.modelSwitcher.Close()
					m.overlayActive = false
					m.activeOverlay = ""
					m.modelConfigMode = false
					modelID := m.modelConfigEntry.ID
					modelProvider := m.modelConfigEntry.Provider
					cmds = append(cmds, func() tea.Msg {
						return ModelSelectedMsg{ModelID: modelID, Provider: modelProvider, ReasoningEffort: reasoningEffort}
					})
					cmds = append(cmds, func() tea.Msg {
						return GatewaySelectedMsg{Gateway: gateway}
					})
					return m, tea.Batch(cmds...)
				}
				// Level-0: check availability before entering the config panel.
				entry, _ := m.modelSwitcher.Accept()
				// Only redirect to /keys when availability info is loaded AND the
				// provider is confirmed unconfigured (#315 Gap 1).
				if m.modelSwitcher.AvailabilityKnown() && !entry.Available {
					// Model's provider is not configured — open /keys overlay
					// pre-positioned on the relevant provider.
					m.modelSwitcher = m.modelSwitcher.Close()
					m.modelConfigMode = false
					m.activeOverlay = "apikeys"
					// overlayActive stays true (already set by the outer "model" case).
					m.apiKeyInput = ""
					m.apiKeyInputMode = false
					// Pre-position cursor on the provider for this model.
					if idx := m.providerIndexInAPIKeyList(entry.Provider); idx >= 0 {
						m.apiKeyCursor = idx
					} else {
						m.apiKeyCursor = 0
					}
					return m, tea.Batch(cmds...)
				}
				// Provider is configured (or availability not yet known) — enter the config panel normally.
				m.modelConfigEntry = entry
				m.modelConfigMode = true
				m.modelConfigSection = 0
				m.modelConfigGatewayCursor = gatewayIndex(m.selectedGateway)
				m.modelConfigReasoningCursor = reasoningLevelIndex(m.selectedReasoningEffort)
				return m, tea.Batch(cmds...)
			}
			// When the dropdown is active, Enter accepts the selected suggestion
			// instead of submitting the input as a message.
			if m.slashComplete.IsActive() {
				newModel, accepted := m.slashComplete.Accept()
				m.slashComplete = newModel
				if accepted != "" {
					m.input = m.input.SetValue(accepted)
					// If the accepted value is a complete slash command (no additional
					// arguments needed), execute it immediately so the user doesn't
					// have to press Enter a second time (BUG-1).
					trimmed := strings.TrimSpace(accepted)
					if strings.HasPrefix(trimmed, "/") {
						cmdName := strings.TrimPrefix(trimmed, "/")
						if m.commandRegistry.IsRegistered(cmdName) {
							m.input = m.input.SetValue("")
							m.slashComplete = m.slashComplete.Close()
							cmds = append(cmds, func() tea.Msg {
								return inputarea.CommandSubmittedMsg{Value: trimmed}
							})
							return m, tea.Batch(cmds...)
						}
					}
				}
				return m, tea.Batch(cmds...)
			}
			// No active dropdown — pass Enter to the input area normally.
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		case m.overlayActive && m.activeOverlay == "help":
			// BUG-4/BUG-3: Route keyboard input to the help dialog when it is open.
			switch {
			case msg.Type == tea.KeyTab || msg.Type == tea.KeyRight || msg.String() == "l":
				m.helpDialog = m.helpDialog.NextTab()
			case msg.Type == tea.KeyShiftTab || msg.Type == tea.KeyLeft || msg.String() == "h":
				m.helpDialog = m.helpDialog.PrevTab()
			}
			return m, tea.Batch(cmds...)
		case m.overlayActive && m.activeOverlay == "stats":
			// BUG-5: Route keyboard input to the stats panel when it is open.
			switch msg.String() {
			case "r":
				m.statsPanel = m.statsPanel.TogglePeriod()
			}
			return m, tea.Batch(cmds...)
		case m.overlayActive && m.activeOverlay == "model" && m.modelConfigMode && m.modelConfigKeyInputMode:
			// Character input in config panel key input mode.
			switch {
			case msg.Type == tea.KeyCtrlU:
				m.modelConfigKeyInput = ""
			case msg.Type == tea.KeyBackspace || msg.Type == tea.KeyDelete:
				if len(m.modelConfigKeyInput) > 0 {
					m.modelConfigKeyInput = m.modelConfigKeyInput[:len(m.modelConfigKeyInput)-1]
				}
			case msg.Type == tea.KeyRunes:
				m.modelConfigKeyInput += string(msg.Runes)
			}
			return m, tea.Batch(cmds...)
		case m.overlayActive && m.activeOverlay == "model" && m.modelConfigMode && !m.modelConfigKeyInputMode:
			// Navigation in config panel (not in key input mode).
			// Determine maximum section: 0=gateway, 1=apikey, 2=reasoning (only if reasoning model).
			maxSection := 1
			if m.modelConfigEntry.ReasoningMode {
				maxSection = 2
			}
			isDown := msg.String() == "j" || msg.Type == tea.KeyDown
			isUp := msg.String() == "k" || msg.Type == tea.KeyUp
			isLeft := msg.String() == "h" || msg.Type == tea.KeyLeft
			isRight := msg.String() == "l" || msg.Type == tea.KeyRight
			switch {
			case m.modelConfigSection == 2 && m.modelConfigEntry.ReasoningMode && isDown:
				// Down in reasoning section: navigate reasoning cursor.
				n := len(modelswitcher.ReasoningLevels)
				if n > 0 {
					m.modelConfigReasoningCursor = (m.modelConfigReasoningCursor + 1) % n
				}
			case m.modelConfigSection == 2 && m.modelConfigEntry.ReasoningMode && isUp:
				// Up in reasoning section: navigate reasoning cursor.
				n := len(modelswitcher.ReasoningLevels)
				if n > 0 {
					m.modelConfigReasoningCursor = (m.modelConfigReasoningCursor - 1 + n) % n
				}
			case isDown:
				// Move to next section.
				m.modelConfigSection = (m.modelConfigSection + 1) % (maxSection + 1)
			case isUp:
				// Move to previous section.
				m.modelConfigSection = (m.modelConfigSection - 1 + maxSection + 1) % (maxSection + 1)
			case isLeft && m.modelConfigSection == 0:
				// Left in gateway section: move cursor left.
				if m.modelConfigGatewayCursor > 0 {
					m.modelConfigGatewayCursor--
				}
			case isRight && m.modelConfigSection == 0:
				// Right in gateway section: move cursor right.
				if m.modelConfigGatewayCursor < len(gatewayOptions)-1 {
					m.modelConfigGatewayCursor++
				}
			case msg.String() == "K" || (m.modelConfigSection == 1 && msg.Type == tea.KeyEnter):
				// Enter key input mode for apikey section.
				if m.modelConfigSection == 1 {
					m.modelConfigKeyInputMode = true
				}
			}
			return m, tea.Batch(cmds...)
		case m.overlayActive && m.activeOverlay == "apikeys" && m.apiKeyInputMode:
			// Character input in apikeys input mode.
			switch {
			case msg.Type == tea.KeyCtrlU:
				m.apiKeyInput = ""
			case msg.Type == tea.KeyBackspace || msg.Type == tea.KeyDelete:
				if len(m.apiKeyInput) > 0 {
					m.apiKeyInput = m.apiKeyInput[:len(m.apiKeyInput)-1]
				}
			case msg.Type == tea.KeyRunes:
				m.apiKeyInput += string(msg.Runes)
			}
			return m, tea.Batch(cmds...)
		case m.overlayActive && m.activeOverlay == "apikeys" && !m.apiKeyInputMode && (msg.String() == "j" || msg.String() == "k" || msg.String() == "up" || msg.String() == "down" || msg.Type == tea.KeyUp || msg.Type == tea.KeyDown):
			// Navigation in apikeys list mode.
			switch {
			case msg.String() == "up" || msg.String() == "k" || msg.Type == tea.KeyUp:
				if len(m.apiKeyProviders) > 0 {
					m.apiKeyCursor = (m.apiKeyCursor - 1 + len(m.apiKeyProviders)) % len(m.apiKeyProviders)
				}
			case msg.String() == "down" || msg.String() == "j" || msg.Type == tea.KeyDown:
				if len(m.apiKeyProviders) > 0 {
					m.apiKeyCursor = (m.apiKeyCursor + 1) % len(m.apiKeyProviders)
				}
			}
			return m, tea.Batch(cmds...)
		case m.overlayActive && m.activeOverlay == "provider" && (msg.String() == "j" || msg.String() == "k"):
			// vim-style j/k navigation in the provider overlay.
			if msg.String() == "k" {
				m.gatewaySelected = (m.gatewaySelected - 1 + len(gatewayOptions)) % len(gatewayOptions)
			} else {
				m.gatewaySelected = (m.gatewaySelected + 1) % len(gatewayOptions)
			}
			return m, tea.Batch(cmds...)
		case key.Matches(msg, m.keys.ScrollUp):
			// When the provider overlay is active, Up/Down navigates the gateway list.
			if m.overlayActive && m.activeOverlay == "provider" {
				m.gatewaySelected = (m.gatewaySelected - 1 + len(gatewayOptions)) % len(gatewayOptions)
				return m, tea.Batch(cmds...)
			}
			// When the model overlay is active (Level-0 only), Up navigates the model list.
			if m.overlayActive && m.activeOverlay == "model" && !m.modelConfigMode {
				m.modelSwitcher = m.modelSwitcher.SelectUp()
				return m, tea.Batch(cmds...)
			}
			// When the dropdown is active, Up navigates the dropdown.
			if m.slashComplete.IsActive() {
				m.slashComplete = m.slashComplete.Up()
				return m, tea.Batch(cmds...)
			}
			m.vp.ScrollUp(1)
		case key.Matches(msg, m.keys.ScrollDown):
			// When the provider overlay is active, Down navigates the gateway list.
			if m.overlayActive && m.activeOverlay == "provider" {
				m.gatewaySelected = (m.gatewaySelected + 1) % len(gatewayOptions)
				return m, tea.Batch(cmds...)
			}
			// When the model overlay is active (Level-0 only), Down navigates the model list.
			if m.overlayActive && m.activeOverlay == "model" && !m.modelConfigMode {
				m.modelSwitcher = m.modelSwitcher.SelectDown()
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
			// When model overlay is open at Level-0 (not config panel), intercept keys for search and star.
			if m.overlayActive && m.activeOverlay == "model" && !m.modelConfigMode {
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
					cmds = append(cmds, m.setStatusMsg("Conversation cleared"))
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
					m.modelConfigMode = false
					m.overlayActive = true
					m.activeOverlay = "model"
					// Fetch from the appropriate source based on gateway selection.
					if m.selectedGateway == "openrouter" {
						orKey := m.pendingAPIKeys["openrouter"]
						cmds = append(cmds, fetchOpenRouterModelsCmd(orKey))
					} else {
						cmds = append(cmds, fetchModelsCmd(m.config.BaseURL))
					}
					cmds = append(cmds, fetchProvidersCmd(m.config.BaseURL))
				case "provider":
					m.gatewaySelected = 0
					for i, opt := range gatewayOptions {
						if opt.ID == m.selectedGateway {
							m.gatewaySelected = i
							break
						}
					}
					m.overlayActive = true
					m.activeOverlay = "provider"
				case "keys":
					m.overlayActive = true
					m.activeOverlay = "apikeys"
					m.apiKeyCursor = 0
					m.apiKeyInput = ""
					m.apiKeyInputMode = false
					cmds = append(cmds, fetchProvidersCmd(m.config.BaseURL))
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
		effModel, effProvider := m.effectiveModelAndProvider()
		cmds = append(cmds, startRunCmd(m.config.BaseURL, msg.Value, m.conversationID, effModel, effProvider, m.selectedReasoningEffort))

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
				m.statusBar.SetCost(m.cumulativeCostUSD)
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
		// For OpenRouter models, availability depends solely on the OpenRouter API key.
		if msg.Source == "openrouter" {
			orKeySet := m.providerKeyConfigured("openrouter")
			m.modelSwitcher = m.modelSwitcher.WithKeyStatus(func(_ string) bool {
				return orKeySet
			})
			m.modelSwitcher = m.modelSwitcher.WithAvailability(func(_ string) bool {
				return orKeySet
			})
		} else {
			m.modelSwitcher = m.modelSwitcher.WithKeyStatus(m.providerKeyConfigured)
			m.modelSwitcher = m.modelSwitcher.WithAvailability(m.providerKeyConfigured)
		}

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
		m.statusBar.SetModel(m.statusBarModelLabel())
		label := displayModelName(msg.ModelID)
		if msg.ReasoningEffort != "" {
			label += " (" + msg.ReasoningEffort + ")"
		}
		// Gap 3 (#315): codex models use the OpenAI API key; surface a clear
		// instruction when the model is selected but OpenAI is not configured.
		if isCodexModel(msg.ModelID) && !m.providerKeyConfigured(msg.Provider) {
			cmds = append(cmds, m.setStatusMsg(
				"Codex uses your OpenAI API key. Set OPENAI_API_KEY or enter it via /keys.",
			))
		} else {
			cmds = append(cmds, m.setStatusMsg("Model: "+label))
		}

	case GatewaySelectedMsg:
		m.selectedGateway = msg.Gateway
		hcfg, _ := harnessconfig.Load()
		hcfg.Gateway = msg.Gateway
		_ = harnessconfig.Save(hcfg)
		m.statusBar.SetModel(m.statusBarModelLabel())
		label := "Gateway: Direct"
		if msg.Gateway == "openrouter" {
			label = "Gateway: OpenRouter"
		}
		cmds = append(cmds, m.setStatusMsg(label))

	case ProvidersLoadedMsg:
		providers := make([]apiKeyProvider, len(msg.Providers))
		for i, p := range msg.Providers {
			providers[i] = apiKeyProvider{
				Name:       p.Name,
				Configured: p.Configured,
				APIKeyEnv:  p.APIKeyEnv,
			}
		}
		m.apiKeyProviders = providers
		// Wire key status to the model switcher for the Level-0 indicator dots.
		m.modelSwitcher = m.modelSwitcher.WithKeyStatus(m.providerKeyConfigured)
		// Wire availability so the model switcher renders unavailable models as dimmed/greyed.
		m.modelSwitcher = m.modelSwitcher.WithAvailability(m.providerKeyConfigured)
		// Gap 2 (#315): when ALL providers are unconfigured, show an empty-state hint.
		if len(providers) > 0 {
			allUnconfigured := true
			for _, p := range providers {
				if p.Configured {
					allUnconfigured = false
					break
				}
			}
			if allUnconfigured {
				cmds = append(cmds, m.setStatusMsg("No providers configured — press / then keys to add API keys"))
			}
		}

	case APIKeySetMsg:
		// Save to persistent config.
		hcfg, _ := harnessconfig.Load()
		if hcfg.APIKeys == nil {
			hcfg.APIKeys = make(map[string]string)
		}
		hcfg.APIKeys[msg.Provider] = msg.Key
		_ = harnessconfig.Save(hcfg)
		// Refresh provider list.
		cmds = append(cmds, fetchProvidersCmd(m.config.BaseURL))
		cmds = append(cmds, m.setStatusMsg("Key saved for "+msg.Provider))

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
			if m.modelConfigMode {
				mainContent = m.viewModelConfigPanel()
			} else {
				mainContent = m.modelSwitcher.View(m.width)
			}
		case "provider":
			mainContent = m.viewProviderOverlay()
		case "apikeys":
			mainContent = m.viewAPIKeysOverlay()
		default:
			// Unknown overlay kind — fall back to viewport.
			mainContent = m.vp.View()
		}
	} else {
		if m.selectedModel == "" && m.vp.IsEmpty() {
			// Welcome hint for first-time users who have no model configured.
			hintStyle := lipgloss.NewStyle().Faint(true)
			mainContent = lipgloss.Place(
				m.width, m.layout.ViewportHeight,
				lipgloss.Center, lipgloss.Center,
				hintStyle.Render("Type /model to select a model  •  Type /help for all commands"),
			)
		} else {
			mainContent = m.vp.View()
		}
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
		Description: "Switch model, gateway, and API keys",
		Handler: func(cmd Command) CommandResult {
			return CommandResult{Status: CmdOK}
		},
	})

	r.Register(CommandEntry{
		Name:        "provider",
		Description: "Switch routing gateway (use /model for per-model config)",
		Handler: func(cmd Command) CommandResult {
			return CommandResult{Status: CmdOK}
		},
	})

	r.Register(CommandEntry{
		Name:        "keys",
		Description: "Manage provider API keys",
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

// effectiveModelAndProvider returns the model ID and provider to use for run requests,
// accounting for the selected gateway.
func (m Model) effectiveModelAndProvider() (model, provider string) {
	if m.selectedGateway == "openrouter" {
		return modelswitcher.OpenRouterSlug(m.selectedModel), "openrouter"
	}
	return m.selectedModel, m.selectedProvider
}

// statusBarModelLabel returns the status bar label for the currently selected model,
// including reasoning effort suffix and gateway indicator if applicable.
func (m Model) statusBarModelLabel() string {
	label := displayModelName(m.selectedModel)
	if m.selectedReasoningEffort != "" {
		label += " (" + m.selectedReasoningEffort + ")"
	}
	if m.selectedGateway == "openrouter" {
		label += " " + string('↗') + "OR"
	}
	return label
}

// viewProviderOverlay renders the gateway selection overlay.
func (m Model) viewProviderOverlay() string {
	width := 60
	title := "Routing Gateway"

	var rows []string
	for i, opt := range gatewayOptions {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.gatewaySelected {
			cursor = string('▶') + " "
			style = style.Foreground(lipgloss.Color("220")).Bold(true)
		}
		label := style.Render(fmt.Sprintf("%s%-12s %s", cursor, opt.Label, opt.Desc))
		rows = append(rows, label)
	}

	footer := lipgloss.NewStyle().Faint(true).Render(string('↑') + "/" + string('↓') + " navigate  enter confirm  esc close")

	content := strings.Join(rows, "\n") + "\n\n" + footer

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(width).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render(title),
			"",
			content,
		))

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// SelectedGateway returns the currently active routing gateway (for testing).
func (m Model) SelectedGateway() string { return m.selectedGateway }

// viewAPIKeysOverlay renders the API key management overlay.
func (m Model) viewAPIKeysOverlay() string {
	width := 54

	if m.apiKeyInputMode && len(m.apiKeyProviders) > 0 {
		p := m.apiKeyProviders[m.apiKeyCursor]
		title := "API Keys > " + p.Name

		envLine := lipgloss.NewStyle().Faint(true).Render(p.APIKeyEnv)
		inputLine := "> " + m.apiKeyInput + "\u258c" // block cursor
		footer := lipgloss.NewStyle().Faint(true).Render("enter confirm  ctrl+u clear  esc back")

		content := envLine + "\n\n" + inputLine + "\n\n" + footer

		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2).
			Width(width).
			Render(lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.NewStyle().Bold(true).Render(title),
				"",
				content,
			))

		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}

	// List mode.
	title := "API Keys"

	configuredStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	unsetStyle := lipgloss.NewStyle().Faint(true)

	var rows []string
	for i, p := range m.apiKeyProviders {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.apiKeyCursor {
			cursor = string('\u25b6') + " "
			style = style.Foreground(lipgloss.Color("220")).Bold(true)
		}
		status := unsetStyle.Render("\u25cb unset")
		if p.Configured {
			status = configuredStyle.Render("\u25cf set")
		}
		label := style.Render(fmt.Sprintf("%s%-14s %-24s", cursor, p.Name, p.APIKeyEnv))
		rows = append(rows, label+" "+status)
	}

	if len(rows) == 0 {
		rows = append(rows, "  No providers available")
	}

	footer := lipgloss.NewStyle().Faint(true).Render(string('\u2191') + "/" + string('\u2193') + " navigate  enter edit  esc close")

	content := strings.Join(rows, "\n") + "\n\n" + footer

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(width).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render(title),
			"",
			content,
		))

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// viewModelConfigPanel renders the Level-1 model configuration panel.
// It shows model name, provider, gateway selection, API key status, and
// optionally reasoning effort selection (for reasoning models).
func (m Model) viewModelConfigPanel() string {
	width := 54
	const borderAndPad = 8 // border 2 + padding 2*2 each side

	focusedSectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
	dimStyle := lipgloss.NewStyle().Faint(true)
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
	configuredStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	unconfiguredStyle := lipgloss.NewStyle().Faint(true)

	entry := m.modelConfigEntry

	// Model name and provider header.
	title := lipgloss.NewStyle().Bold(true).Render(entry.DisplayName)
	providerLine := dimStyle.Render(entry.ProviderLabel)

	var sections []string

	// --- Gateway section ---
	isFocusedGateway := m.modelConfigSection == 0
	var gwLabel string
	if isFocusedGateway {
		gwLabel = focusedSectionStyle.Render("Gateway")
	} else {
		gwLabel = "Gateway"
	}

	var gwRows []string
	for i, opt := range gatewayOptions {
		isSelected := i == m.modelConfigGatewayCursor
		var rowStyle lipgloss.Style
		var cursor string
		if isSelected {
			cursor = cursorStyle.Render("▶") + " "
			rowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
		} else {
			cursor = "  "
			rowStyle = dimStyle
		}
		row := cursor + rowStyle.Render(fmt.Sprintf("%-12s %s", opt.Label, opt.Desc))
		gwRows = append(gwRows, row)
	}
	gatewaySection := gwLabel + "\n" + strings.Join(gwRows, "\n")
	sections = append(sections, gatewaySection)

	// --- API Key section ---
	isFocusedKey := m.modelConfigSection == 1
	var keyLabel string
	if isFocusedKey {
		keyLabel = focusedSectionStyle.Render("API Key")
	} else {
		keyLabel = "API Key"
	}

	keyConfigured := m.providerKeyConfigured(entry.Provider)
	var keyStatusStr string
	if keyConfigured {
		keyStatusStr = configuredStyle.Render("● configured")
	} else {
		keyStatusStr = unconfiguredStyle.Render("○ not set")
	}

	var keyContent string
	if m.modelConfigKeyInputMode {
		keyContent = keyLabel + "    " + keyStatusStr + "\n" +
			"> " + m.modelConfigKeyInput + "\u258c" + "\n" +
			dimStyle.Render("enter confirm  ctrl+u clear  esc back")
	} else {
		var keyHint string
		if isFocusedKey {
			keyHint = dimStyle.Render("  (K to update)")
		}
		keyContent = keyLabel + "    " + keyStatusStr + keyHint
	}
	sections = append(sections, keyContent)

	// --- Reasoning section (reasoning models only) ---
	if entry.ReasoningMode {
		isFocusedReasoning := m.modelConfigSection == 2
		var reasoningLabel string
		if isFocusedReasoning {
			reasoningLabel = focusedSectionStyle.Render("Reasoning Effort")
		} else {
			reasoningLabel = "Reasoning Effort"
		}

		var reasoningRows []string
		for i, rl := range modelswitcher.ReasoningLevels {
			isSelected := i == m.modelConfigReasoningCursor
			var row string
			if isSelected {
				row = cursorStyle.Render("▶") + " " + lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render(rl.DisplayName)
			} else {
				row = "  " + dimStyle.Render(rl.DisplayName)
			}
			reasoningRows = append(reasoningRows, row)
		}
		reasoningSection := reasoningLabel + "\n" + strings.Join(reasoningRows, "\n")
		sections = append(sections, reasoningSection)
	}

	// --- Footer ---
	var footer string
	if !m.modelConfigKeyInputMode {
		footer = dimStyle.Render("↑/↓ sections  ←/→ gateway  enter confirm  esc back")
	}

	var innerContent string
	if footer != "" {
		innerContent = title + "\n" + providerLine + "\n\n" +
			strings.Join(sections, "\n\n") + "\n\n" + footer
	} else {
		innerContent = title + "\n" + providerLine + "\n\n" +
			strings.Join(sections, "\n\n")
	}

	_ = borderAndPad

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(width).
		Render(innerContent)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// ModelConfigMode returns true when the Level-1 model config panel is active (for testing).
func (m Model) ModelConfigMode() bool { return m.modelConfigMode }

// ModelConfigEntry returns the model entry being configured (for testing).
func (m Model) ModelConfigEntry() modelswitcher.ModelEntry { return m.modelConfigEntry }

// ModelConfigSection returns the focused section index in the config panel (for testing).
func (m Model) ModelConfigSection() int { return m.modelConfigSection }

// ModelConfigGatewayCursor returns the gateway cursor in the config panel (for testing).
func (m Model) ModelConfigGatewayCursor() int { return m.modelConfigGatewayCursor }

// ModelConfigReasoningCursor returns the reasoning cursor in the config panel (for testing).
func (m Model) ModelConfigReasoningCursor() int { return m.modelConfigReasoningCursor }

// ModelConfigKeyInputMode returns true when the config panel key input is active (for testing).
func (m Model) ModelConfigKeyInputMode() bool { return m.modelConfigKeyInputMode }

// ModelConfigKeyInput returns the current key input text in the config panel (for testing).
func (m Model) ModelConfigKeyInput() string { return m.modelConfigKeyInput }

// APIKeyInputMode returns true when the /keys overlay is in input mode (for testing).
func (m Model) APIKeyInputMode() bool { return m.apiKeyInputMode }

// APIKeyInput returns the current input text in the /keys overlay (for testing).
func (m Model) APIKeyInput() string { return m.apiKeyInput }

// APIKeyProviders returns the provider list in the /keys overlay (for testing).
func (m Model) APIKeyProviders() []apiKeyProvider { return m.apiKeyProviders }

// APIKeyCursor returns the current cursor position in the /keys overlay provider list (for testing).
func (m Model) APIKeyCursor() int { return m.apiKeyCursor }

// ModelSwitcher returns the current modelswitcher Model (for testing).
func (m Model) ModelSwitcher() modelswitcher.Model { return m.modelSwitcher }

// StatusBarView returns the raw status bar view, bypassing any transient status
// message overlay. This is used by tests to verify that the status bar correctly
// stores and renders model name and cost independent of status messages.
func (m Model) StatusBarView() string { return m.statusBar.View() }

// providerIndexInAPIKeyList returns the index of the given provider name in the
// apiKeyProviders list, or -1 if not found.
func (m Model) providerIndexInAPIKeyList(providerName string) int {
	for i, p := range m.apiKeyProviders {
		if p.Name == providerName {
			return i
		}
	}
	return -1
}

// isCodexModel returns true when the modelID refers to a Codex-family model
// (i.e. its ID contains "codex").
func isCodexModel(modelID string) bool {
	return strings.Contains(strings.ToLower(modelID), "codex")
}

// HelpDialogActiveTab returns the currently active tab index in the help dialog (for testing).
func (m Model) HelpDialogActiveTab() int { return int(m.helpDialog.ActiveTab()) }

// StatsPanelActivePeriod returns the currently active period in the stats panel (for testing).
func (m Model) StatsPanelActivePeriod() int { return int(m.statsPanel.ActivePeriod()) }
