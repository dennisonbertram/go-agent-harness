package viewport

// State snapshots the current viewport position for tests.
type State struct {
	TotalLines    int
	Offset        int
	AtBottom      bool
	AutoScroll    bool
	HasNewContent bool
}

// Snapshot returns a state snapshot.
func (m Model) Snapshot() State {
	return State{
		TotalLines:    len(m.lines),
		Offset:        m.offset,
		AtBottom:      m.AtBottom(),
		AutoScroll:    m.autoScroll,
		HasNewContent: m.HasNewContent(),
	}
}
