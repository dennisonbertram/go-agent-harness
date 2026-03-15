package planoverlay

// PlanState represents the current state of the plan overlay.
type PlanState int

const (
	PlanStateHidden   PlanState = iota // overlay not shown
	PlanStatePending                   // waiting for user approval
	PlanStateApproved                  // user approved the plan
	PlanStateRejected                  // user rejected the plan
)

// Model is the plan mode overlay state.
// All methods return a new Model (immutable value semantics — safe for
// concurrent use when each goroutine holds its own copy).
type Model struct {
	State        PlanState
	PlanText     string // the plan content (plain text / markdown)
	Width        int
	Height       int
	ScrollOffset int
}

// New creates a new plan overlay Model in the hidden state.
func New() Model {
	return Model{
		State: PlanStateHidden,
	}
}

// Show transitions the overlay to PlanStatePending and stores the plan text.
// The scroll offset is reset to zero.
func (m Model) Show(planText string) Model {
	m.State = PlanStatePending
	m.PlanText = planText
	m.ScrollOffset = 0
	return m
}

// Approve transitions from PlanStatePending to PlanStateApproved.
// If the current state is not PlanStatePending the call is a no-op.
func (m Model) Approve() Model {
	if m.State == PlanStatePending {
		m.State = PlanStateApproved
	}
	return m
}

// Reject transitions from PlanStatePending to PlanStateRejected.
// If the current state is not PlanStatePending the call is a no-op.
func (m Model) Reject() Model {
	if m.State == PlanStatePending {
		m.State = PlanStateRejected
	}
	return m
}

// Hide transitions the overlay to PlanStateHidden regardless of the current state.
func (m Model) Hide() Model {
	m.State = PlanStateHidden
	return m
}

// IsVisible reports whether the overlay should be rendered.
func (m Model) IsVisible() bool {
	return m.State != PlanStateHidden
}

// ScrollUp decrements the scroll offset by one, clamped at zero.
func (m Model) ScrollUp() Model {
	m.ScrollOffset--
	if m.ScrollOffset < 0 {
		m.ScrollOffset = 0
	}
	return m
}

// ScrollDown increments the scroll offset by one.
// The offset is clamped so that it never exceeds maxLines-Height (floor 0).
func (m Model) ScrollDown(maxLines int) Model {
	m.ScrollOffset++
	limit := maxLines - m.Height
	if limit < 0 {
		limit = 0
	}
	if m.ScrollOffset > limit {
		m.ScrollOffset = limit
	}
	return m
}

// CurrentState returns the current PlanState.
func (m Model) CurrentState() PlanState {
	return m.State
}
