package viewport

// Model is the scrollable viewport for conversation content.
type Model struct {
	// Width and Height define the viewport dimensions.
	Width  int
	Height int
	// Content is the rendered text content.
	Content string
	// YOffset tracks vertical scroll position.
	YOffset int
}

// New creates a new viewport model.
func New(width, height int) Model {
	return Model{Width: width, Height: height}
}

// View renders the viewport. Stub for now.
func (m Model) View() string {
	return ""
}
