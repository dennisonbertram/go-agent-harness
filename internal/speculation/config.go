// Package speculation provides the foundation for speculative pre-execution:
// predicting the user's next input and pre-executing it in an isolated overlay.
// This is the primitive layer — prediction engine and overlay management.
// Integration with the runner is handled by future work.
package speculation

// SpeculationConfig holds the configuration for speculative pre-execution.
type SpeculationConfig struct {
	// Enabled is the master toggle. Disabled by default — feature is experimental.
	Enabled bool `toml:"enabled"`

	// MaxTurns is the maximum number of assistant turns per speculation session.
	MaxTurns int `toml:"max_turns"`

	// MaxMessages is the maximum number of messages (user + assistant) per speculation session.
	MaxMessages int `toml:"max_messages"`

	// OverlayDir is the base directory for overlay subdirectories.
	// If empty, $TMPDIR/speculation/ is used automatically.
	OverlayDir string `toml:"overlay_dir"`

	// StopOnWrite stops speculation when a write operation is attempted.
	StopOnWrite bool `toml:"stop_on_write"`

	// AllowedTools is the list of tool names permitted during speculation.
	// Write-capable tools should be excluded.
	AllowedTools []string `toml:"allowed_tools"`
}

// DefaultSpeculationConfig returns the recommended default configuration for speculation.
// Speculation is disabled by default because it is experimental.
func DefaultSpeculationConfig() SpeculationConfig {
	return SpeculationConfig{
		Enabled:      false,
		MaxTurns:     20,
		MaxMessages:  100,
		OverlayDir:   "",
		StopOnWrite:  true,
		AllowedTools: []string{"read", "grep", "glob", "bash_readonly"},
	}
}
