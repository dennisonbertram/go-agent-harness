// Package permissionspanel implements the /permissions panel component for the
// harnesscli TUI. The panel shows the session's current allowed/denied tool
// permission rules and lets the user navigate, toggle, and delete them.
//
// All Model methods use immutable value semantics — every mutating operation
// returns a new Model copy so that concurrent goroutines each holding their
// own snapshot are race-free.
package permissionspanel

// PermissionRule describes a single tool permission entry in the session.
type PermissionRule struct {
	ToolName  string // e.g. "bash", "read", "write"
	Allowed   bool   // true = allowed, false = denied
	Permanent bool   // true = permanent for session, false = once
}

// Model is the permissions panel state.
//
// Width and Height may be set directly by the caller before calling View().
// All other state changes must go through the provided methods to preserve
// immutable value semantics.
type Model struct {
	Rules    []PermissionRule
	Selected int
	IsOpen   bool
	Width    int
	Height   int
}

// New creates a new, closed Model with no rules.
func New() Model {
	return Model{}
}

// Open returns a copy of the model with IsOpen set to true and Rules replaced
// by the provided slice. Selection is reset to 0.
func (m Model) Open(rules []PermissionRule) Model {
	m.IsOpen = true
	m.Selected = 0
	if rules != nil {
		cp := make([]PermissionRule, len(rules))
		copy(cp, rules)
		m.Rules = cp
	} else {
		m.Rules = nil
	}
	return m
}

// Close returns a copy of the model with IsOpen set to false. Rules are
// preserved so that they can be re-displayed when the panel is re-opened.
func (m Model) Close() Model {
	m.IsOpen = false
	return m
}

// IsVisible reports whether the panel is currently open.
func (m Model) IsVisible() bool { return m.IsOpen }

// SetRules returns a copy of the model with Rules replaced by the provided
// slice. The selection index is clamped to the new length.
func (m Model) SetRules(rules []PermissionRule) Model {
	if rules != nil {
		cp := make([]PermissionRule, len(rules))
		copy(cp, rules)
		m.Rules = cp
	} else {
		m.Rules = nil
	}
	m.Selected = clamp(m.Selected, m.Rules)
	return m
}

// SelectUp returns a copy of the model with the selection moved up by one
// row, wrapping around to the last entry when already at the top.
func (m Model) SelectUp() Model {
	n := len(m.Rules)
	if n == 0 {
		return m
	}
	m.Selected = (m.Selected - 1 + n) % n
	return m
}

// SelectDown returns a copy of the model with the selection moved down by one
// row, wrapping around to the first entry when already at the bottom.
func (m Model) SelectDown() Model {
	n := len(m.Rules)
	if n == 0 {
		return m
	}
	m.Selected = (m.Selected + 1) % n
	return m
}

// ToggleSelected returns a copy of the model where the Allowed field of the
// currently selected rule has been flipped. If the rule list is empty the
// model is returned unchanged.
func (m Model) ToggleSelected() Model {
	if len(m.Rules) == 0 {
		return m
	}
	// Deep-copy the rules slice so the original is not mutated.
	cp := make([]PermissionRule, len(m.Rules))
	copy(cp, m.Rules)
	cp[m.Selected].Allowed = !cp[m.Selected].Allowed
	m.Rules = cp
	return m
}

// RemoveSelected returns a copy of the model with the currently selected rule
// removed. The selection index is adjusted so it remains in bounds. If the
// rule list is empty the model is returned unchanged.
func (m Model) RemoveSelected() Model {
	if len(m.Rules) == 0 {
		return m
	}
	idx := m.Selected
	newRules := make([]PermissionRule, 0, len(m.Rules)-1)
	newRules = append(newRules, m.Rules[:idx]...)
	newRules = append(newRules, m.Rules[idx+1:]...)
	m.Rules = newRules
	m.Selected = clamp(idx, m.Rules)
	return m
}

// SelectedRule returns the currently selected PermissionRule and true. If the
// rule list is empty or the index is out of range, it returns a zero Rule and
// false.
func (m Model) SelectedRule() (PermissionRule, bool) {
	if len(m.Rules) == 0 || m.Selected < 0 || m.Selected >= len(m.Rules) {
		return PermissionRule{}, false
	}
	return m.Rules[m.Selected], true
}

// clamp returns idx clamped to [0, len(rules)-1]. When rules is empty it
// returns 0.
func clamp(idx int, rules []PermissionRule) int {
	n := len(rules)
	if n == 0 {
		return 0
	}
	if idx >= n {
		return n - 1
	}
	if idx < 0 {
		return 0
	}
	return idx
}
