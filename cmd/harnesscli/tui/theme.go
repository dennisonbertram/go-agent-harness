package tui

// Theme holds the color palette and style tokens for the TUI.
// Will be populated with lipgloss styles in TUI-006.
type Theme struct {
	// UserColor is the style for user messages.
	UserColor string
	// AssistantColor is the style for assistant messages.
	AssistantColor string
	// ToolColor is the style for tool call output.
	ToolColor string
	// ErrorColor is the style for error indicators.
	ErrorColor string
	// BorderColor is the style for borders and separators.
	BorderColor string
}

// DefaultTheme returns the default theme configuration.
func DefaultTheme() Theme {
	return Theme{}
}
