package diffview

// Model renders a unified diff view for file changes.
type Model struct {
	// FilePath is the file being diffed.
	FilePath string
	// Diff is the unified diff content.
	Diff string
	// Width is the available rendering width.
	Width int
}

// New creates a new diff view model.
func New(filePath, diff string) Model {
	return Model{FilePath: filePath, Diff: diff}
}

// View renders the diff view. Stub for now.
func (m Model) View() string {
	return ""
}
