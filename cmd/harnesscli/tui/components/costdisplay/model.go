package costdisplay

// CostSnapshot holds a point-in-time record of token usage and cost for a session.
type CostSnapshot struct {
	InputTokens  int
	OutputTokens int
	TotalCostUSD float64
	Model        string
}

// Model is the cost display component that shows running token usage and cost.
// All methods return new copies (immutable value semantics).
type Model struct {
	// Snapshot holds the latest cost/token data.
	Snapshot CostSnapshot
	// Visible controls whether the component renders.
	Visible bool
	// Width is the available rendering width for right-alignment.
	Width int
}

// New creates a new cost display model, hidden by default.
func New() Model {
	return Model{}
}

// Show returns a copy with Visible set to true.
func (m Model) Show() Model {
	m.Visible = true
	return m
}

// Hide returns a copy with Visible set to false.
func (m Model) Hide() Model {
	m.Visible = false
	return m
}

// Toggle returns a copy with Visible flipped.
func (m Model) Toggle() Model {
	m.Visible = !m.Visible
	return m
}

// IsVisible reports whether the component is currently visible.
func (m Model) IsVisible() bool {
	return m.Visible
}

// Update returns a copy with the snapshot replaced by snap.
func (m Model) Update(snap CostSnapshot) Model {
	m.Snapshot = snap
	return m
}
