package slashcomplete

import "strings"

// Suggestion is a completion candidate.
type Suggestion struct {
	Name        string
	Description string
}

// Model is the autocomplete dropdown state machine.
// All methods return a new Model (value semantics — safe for concurrent use
// when each goroutine holds its own copy).
type Model struct {
	suggestions []Suggestion
	filtered    []Suggestion // current filtered+ranked results
	query       string       // current filter query (without leading /)
	selected    int          // cursor index in filtered
	active      bool
	maxVisible  int // max rows to show (default 8)
}

// New creates a new Model seeded with the given suggestions.
// The model starts inactive (overlay hidden).
func New(suggestions []Suggestion) Model {
	cp := make([]Suggestion, len(suggestions))
	copy(cp, suggestions)
	m := Model{
		suggestions: cp,
		maxVisible:  8,
	}
	m.filtered = applyFilter(m.suggestions, "")
	return m
}

// Open activates the dropdown overlay.
func (m Model) Open() Model {
	m.active = true
	return m
}

// Close deactivates the dropdown overlay.
func (m Model) Close() Model {
	m.active = false
	return m
}

// IsActive reports whether the dropdown is currently visible.
func (m Model) IsActive() bool {
	return m.active
}

// SetQuery updates the filter query and resets the cursor to position 0.
// query should NOT include the leading '/'.
func (m Model) SetQuery(query string) Model {
	m.query = query
	m.filtered = applyFilter(m.suggestions, query)
	m.selected = 0
	return m
}

// Down moves the cursor down by one, wrapping to the top when past the last item.
func (m Model) Down() Model {
	if len(m.filtered) == 0 {
		return m
	}
	m.selected = (m.selected + 1) % len(m.filtered)
	return m
}

// Up moves the cursor up by one, wrapping to the bottom when past the first item.
func (m Model) Up() Model {
	if len(m.filtered) == 0 {
		return m
	}
	m.selected = (m.selected - 1 + len(m.filtered)) % len(m.filtered)
	return m
}

// Selected returns the currently highlighted suggestion.
// Returns (zero, false) when the filtered list is empty.
func (m Model) Selected() (Suggestion, bool) {
	if len(m.filtered) == 0 {
		return Suggestion{}, false
	}
	return m.filtered[m.selected], true
}

// Filtered returns a copy of the current filtered results.
func (m Model) Filtered() []Suggestion {
	cp := make([]Suggestion, len(m.filtered))
	copy(cp, m.filtered)
	return cp
}

// Accept selects the current suggestion, closes the dropdown, and returns
// the completed text (e.g. "/clear ").
// If there are no filtered results it returns ("", closed model).
func (m Model) Accept() (Model, string) {
	s, ok := m.Selected()
	m.active = false
	if !ok {
		return m, ""
	}
	return m, "/" + s.Name + " "
}

// applyFilter returns the subset of suggestions whose Name has query as a
// case-insensitive prefix.  An empty query returns all suggestions.
func applyFilter(suggestions []Suggestion, query string) []Suggestion {
	if query == "" {
		cp := make([]Suggestion, len(suggestions))
		copy(cp, suggestions)
		return cp
	}
	lower := strings.ToLower(query)
	var out []Suggestion
	for _, s := range suggestions {
		if strings.HasPrefix(strings.ToLower(s.Name), lower) {
			out = append(out, s)
		}
	}
	return out
}
