package tui

// TUIConfig holds configuration for the TUI mode.
type TUIConfig struct {
	// BaseURL is the harnessd server URL.
	BaseURL string
	// Model is the LLM model identifier.
	Model string
	// Workspace is the workspace root path.
	Workspace string
	// MaxSteps limits the number of agent steps.
	MaxSteps int
	// Theme selects the color theme.
	Theme string
}

// DefaultTUIConfig returns a TUIConfig with sensible defaults.
func DefaultTUIConfig() TUIConfig {
	return TUIConfig{
		BaseURL:  "http://localhost:8080",
		MaxSteps: 8,
	}
}
