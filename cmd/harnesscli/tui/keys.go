package tui

// keyMap defines all key bindings for the TUI.
// Will be populated with BubbleTea key.Binding values in TUI-008.
type keyMap struct {
	Quit       keyBinding
	Help       keyBinding
	Submit     keyBinding
	Cancel     keyBinding
	ScrollUp   keyBinding
	ScrollDown keyBinding
	Tab        keyBinding
	SlashCmd   keyBinding
}

// keyBinding is a placeholder for key binding configuration.
type keyBinding struct {
	Keys []string
	Help string
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() keyMap {
	return keyMap{}
}
