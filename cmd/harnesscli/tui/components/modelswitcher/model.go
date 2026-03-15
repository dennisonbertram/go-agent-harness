package modelswitcher

// ModelEntry holds display information for a single LLM model.
type ModelEntry struct {
	ID          string // e.g. "gpt-4.1-mini"
	DisplayName string // e.g. "GPT-4.1 Mini"
	Provider    string // e.g. "openai"
	IsCurrent   bool
}

// DefaultModels is the hardcoded list of available models shown by New().
var DefaultModels = []ModelEntry{
	{ID: "gpt-4.1", DisplayName: "GPT-4.1", Provider: "openai"},
	{ID: "gpt-4.1-mini", DisplayName: "GPT-4.1 Mini", Provider: "openai"},
	{ID: "gpt-4.1-nano", DisplayName: "GPT-4.1 Nano", Provider: "openai"},
	{ID: "o3", DisplayName: "o3", Provider: "openai"},
	{ID: "o4-mini", DisplayName: "o4-mini", Provider: "openai"},
}

// Model is the model switcher dropdown state.
// All methods return a new Model (value semantics — safe for concurrent use
// when each goroutine holds its own copy).
type Model struct {
	Models   []ModelEntry
	Selected int  // index into Models
	IsOpen   bool
	Width    int
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
