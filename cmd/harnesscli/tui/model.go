package tui

// Model is the root BubbleTea model for the TUI.
// It will be fleshed out in TUI-003.
type Model struct {
	// Width and Height track the terminal dimensions.
	Width  int
	Height int

	// RunID is the current run being displayed.
	RunID string

	// config holds TUI configuration.
	config TUIConfig
}
