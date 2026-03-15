package tooluse

// ToggleState manages the expanded/collapsed toggle for a single tool call.
// It is an immutable value type — all mutations return a new ToggleState.
type ToggleState struct {
	expanded bool
}

// Toggle returns a new ToggleState with the expanded flag flipped.
func (t ToggleState) Toggle() ToggleState {
	return ToggleState{expanded: !t.expanded}
}

// IsExpanded reports whether the tool call is in the expanded state.
func (t ToggleState) IsExpanded() bool {
	return t.expanded
}

// View returns either the expanded or collapsed rendering depending on the
// current toggle state.
func (t ToggleState) View(c CollapsedView, e ExpandedView) string {
	if t.expanded {
		return e.View()
	}
	return c.View()
}
