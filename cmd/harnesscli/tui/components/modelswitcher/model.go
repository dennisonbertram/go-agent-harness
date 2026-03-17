package modelswitcher

// ModelEntry holds display information for a single LLM model.
type ModelEntry struct {
	ID            string // e.g. "gpt-4.1-mini"
	DisplayName   string // e.g. "GPT-4.1 Mini"
	Provider      string // provider key for API (e.g. "openai", "anthropic")
	ProviderLabel string // human-readable provider name for display (e.g. "OpenAI")
	ReasoningMode bool   // true for reasoning models (deepseek-reasoner, qwen-qwq-32b, etc.)
	IsCurrent     bool
}

// DefaultModels is the list of available models shown by New(), grouped by provider.
var DefaultModels = []ModelEntry{
	// OpenAI
	{ID: "gpt-4.1", DisplayName: "GPT-4.1", Provider: "openai", ProviderLabel: "OpenAI"},
	{ID: "gpt-4.1-mini", DisplayName: "GPT-4.1 Mini", Provider: "openai", ProviderLabel: "OpenAI"},
	// Anthropic
	{ID: "claude-sonnet-4-6", DisplayName: "Claude Sonnet 4.6", Provider: "anthropic", ProviderLabel: "Anthropic"},
	{ID: "claude-opus-4-6", DisplayName: "Claude Opus 4.6", Provider: "anthropic", ProviderLabel: "Anthropic"},
	{ID: "claude-haiku-4-5-20251001", DisplayName: "Claude Haiku 4.5", Provider: "anthropic", ProviderLabel: "Anthropic"},
	// Google
	{ID: "gemini-2.5-flash", DisplayName: "Gemini 2.5 Flash", Provider: "gemini", ProviderLabel: "Google"},
	{ID: "gemini-2.0-flash", DisplayName: "Gemini 2.0 Flash", Provider: "gemini", ProviderLabel: "Google"},
	// DeepSeek
	{ID: "deepseek-chat", DisplayName: "DeepSeek Chat", Provider: "deepseek", ProviderLabel: "DeepSeek"},
	{ID: "deepseek-reasoner", DisplayName: "DeepSeek Reasoner", Provider: "deepseek", ProviderLabel: "DeepSeek", ReasoningMode: true},
	// xAI
	{ID: "grok-3-mini", DisplayName: "Grok 3 Mini", Provider: "xai", ProviderLabel: "xAI"},
	{ID: "grok-4-1-fast-reasoning", DisplayName: "Grok 4.1 Fast", Provider: "xai", ProviderLabel: "xAI", ReasoningMode: true},
	// Groq
	{ID: "llama-3.3-70b-versatile", DisplayName: "Llama 3.3 70B", Provider: "groq", ProviderLabel: "Groq"},
	{ID: "qwen-qwq-32b", DisplayName: "QwQ 32B", Provider: "groq", ProviderLabel: "Groq", ReasoningMode: true},
	// Qwen
	{ID: "qwen-plus", DisplayName: "Qwen Plus", Provider: "qwen", ProviderLabel: "Qwen"},
	{ID: "qwen-turbo", DisplayName: "Qwen Turbo", Provider: "qwen", ProviderLabel: "Qwen"},
	// Kimi
	{ID: "kimi-k2.5", DisplayName: "Kimi K2.5", Provider: "kimi", ProviderLabel: "Kimi"},
}

// ReasoningEntry holds display information for a single reasoning effort level.
type ReasoningEntry struct {
	ID          string // "", "low", "medium", "high"
	DisplayName string // "Default", "Low", "Medium", "High"
}

// ReasoningLevels is the ordered list of reasoning effort levels.
var ReasoningLevels = []ReasoningEntry{
	{ID: "", DisplayName: "Default"},
	{ID: "low", DisplayName: "Low"},
	{ID: "medium", DisplayName: "Medium"},
	{ID: "high", DisplayName: "High"},
}

// Model is the model switcher dropdown state.
// All methods return a new Model (value semantics — safe for concurrent use
// when each goroutine holds its own copy).
type Model struct {
	Models   []ModelEntry
	Selected int  // index into Models
	IsOpen   bool
	Width    int

	reasoningMode     bool   // true = Level-1 (reasoning effort) active
	reasoningSelected int    // cursor in ReasoningLevels
	currentReasoning  string // "", "low", "medium", "high"
}

// New constructs a Model pre-loaded with DefaultModels, marking the entry
// whose ID matches currentModelID as IsCurrent. If no match is found, no
// entry is marked (CurrentModel falls back to first).
func New(currentModelID string) Model {
	models := make([]ModelEntry, len(DefaultModels))
	copy(models, DefaultModels)

	// Mark the current model and set initial selection to it.
	selected := 0
	for i := range models {
		if models[i].ID == currentModelID {
			models[i].IsCurrent = true
			selected = i
		}
	}

	return Model{
		Models:   models,
		Selected: selected,
	}
}

// Open opens the dropdown overlay.
func (m Model) Open() Model {
	m.IsOpen = true
	return m
}

// Close closes the dropdown overlay.
func (m Model) Close() Model {
	m.IsOpen = false
	m.reasoningMode = false
	return m
}

// IsVisible reports whether the dropdown is currently shown.
func (m Model) IsVisible() bool {
	return m.IsOpen
}

// SelectUp moves the cursor up by one, wrapping around to the last entry.
func (m Model) SelectUp() Model {
	n := len(m.Models)
	if n == 0 {
		return m
	}
	m.Selected = (m.Selected - 1 + n) % n
	return m
}

// SelectDown moves the cursor down by one, wrapping around to the first entry.
func (m Model) SelectDown() Model {
	n := len(m.Models)
	if n == 0 {
		return m
	}
	m.Selected = (m.Selected + 1) % n
	return m
}

// Accept returns the currently selected ModelEntry and whether it differs from
// the IsCurrent entry. The bool is true when the selection has changed from the
// current model. The model itself is not mutated by Accept — callers should
// call Close() on the returned model when appropriate.
func (m Model) Accept() (ModelEntry, bool) {
	if len(m.Models) == 0 {
		return ModelEntry{}, false
	}
	entry := m.Models[m.Selected]
	changed := !entry.IsCurrent
	return entry, changed
}

// CurrentModel returns the entry with IsCurrent==true.
// If no entry is marked current, the first entry is returned.
func (m Model) CurrentModel() ModelEntry {
	for _, e := range m.Models {
		if e.IsCurrent {
			return e
		}
	}
	if len(m.Models) > 0 {
		return m.Models[0]
	}
	return ModelEntry{}
}

// IsReasoningMode reports whether the Level-1 (reasoning effort) panel is active.
func (m Model) IsReasoningMode() bool {
	return m.reasoningMode
}

// EnterReasoningMode switches to the Level-1 reasoning effort panel.
// The cursor is initialised to the index of the current reasoning level
// (falls back to 0 / "Default" when not found).
func (m Model) EnterReasoningMode() Model {
	m.reasoningMode = true
	// Find current reasoning in ReasoningLevels.
	m.reasoningSelected = 0
	for i, rl := range ReasoningLevels {
		if rl.ID == m.currentReasoning {
			m.reasoningSelected = i
			break
		}
	}
	return m
}

// ExitReasoningMode returns to the Level-0 model list without changing any selection.
func (m Model) ExitReasoningMode() Model {
	m.reasoningMode = false
	return m
}

// ReasoningUp moves the reasoning-level cursor up by one, wrapping around.
func (m Model) ReasoningUp() Model {
	n := len(ReasoningLevels)
	if n == 0 {
		return m
	}
	m.reasoningSelected = (m.reasoningSelected - 1 + n) % n
	return m
}

// ReasoningDown moves the reasoning-level cursor down by one, wrapping around.
func (m Model) ReasoningDown() Model {
	n := len(ReasoningLevels)
	if n == 0 {
		return m
	}
	m.reasoningSelected = (m.reasoningSelected + 1) % n
	return m
}

// AcceptReasoning returns the currently selected ReasoningEntry and whether it
// differs from the current reasoning level.
func (m Model) AcceptReasoning() (ReasoningEntry, bool) {
	if len(ReasoningLevels) == 0 {
		return ReasoningEntry{}, false
	}
	entry := ReasoningLevels[m.reasoningSelected]
	changed := entry.ID != m.currentReasoning
	return entry, changed
}

// WithCurrentReasoning returns a copy with currentReasoning set to effort.
func (m Model) WithCurrentReasoning(effort string) Model {
	m.currentReasoning = effort
	return m
}
