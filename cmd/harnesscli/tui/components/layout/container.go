package layout

// Container applies a Layout to position child component regions.
// Returns lipgloss-compatible width/height pairs.
type Container struct {
	L Layout
}

// NewContainer wraps a Layout for positional helpers.
func NewContainer(l Layout) Container {
	return Container{L: l}
}

// ViewportWidth returns the available width for the viewport.
func (c Container) ViewportWidth() int { return c.L.Width }

// InputWidth returns the available width for the input area.
func (c Container) InputWidth() int { return c.L.Width }
