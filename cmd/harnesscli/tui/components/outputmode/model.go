package outputmode

import "github.com/charmbracelet/lipgloss"

// OutputMode controls whether tool use details are shown in compact or verbose
// mode.
type OutputMode int

const (
	// OutputModeCompact collapses tool calls and hides tool inputs.
	OutputModeCompact OutputMode = iota
	// OutputModeVerbose expands tool calls and shows inputs and outputs.
	OutputModeVerbose
)

// dimColor is the adaptive color used for the status indicator.
var dimColor = lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#5C5C5C"}

// dimStyle applies dim/faint styling for the status indicator.
var dimStyle = lipgloss.NewStyle().Foreground(dimColor)

// Model holds the current output mode. All methods return new copies —
// immutable value semantics guarantee no data races.
type Model struct {
	Mode OutputMode
}

// New creates a new Model defaulting to OutputModeCompact.
func New() Model {
	return Model{Mode: OutputModeCompact}
}

// Toggle flips between OutputModeCompact and OutputModeVerbose and returns
// the updated Model.
func (m Model) Toggle() Model {
	if m.Mode == OutputModeCompact {
		m.Mode = OutputModeVerbose
	} else {
		m.Mode = OutputModeCompact
	}
	return m
}

// SetMode returns a new Model with Mode set to the given value.
func (m Model) SetMode(mode OutputMode) Model {
	m.Mode = mode
	return m
}

// IsVerbose reports whether the current mode is OutputModeVerbose.
func (m Model) IsVerbose() bool { return m.Mode == OutputModeVerbose }

// IsCompact reports whether the current mode is OutputModeCompact.
func (m Model) IsCompact() bool { return m.Mode == OutputModeCompact }

// Label returns the lowercase mode name: "compact" or "verbose".
func (m Model) Label() string {
	if m.Mode == OutputModeVerbose {
		return "verbose"
	}
	return "compact"
}

// StatusIndicator returns a lipgloss-styled indicator string, e.g.
// "[compact]" or "[verbose]", using the dim adaptive color.
func (m Model) StatusIndicator() string {
	return dimStyle.Render("[" + m.Label() + "]")
}

// StatusText returns a formatted string suitable for embedding in a status bar,
// e.g. "  [compact]  " or "  [verbose]  ".
func (m Model) StatusText() string {
	return "  " + m.StatusIndicator() + "  "
}

// HelpLine returns a one-line help hint for display in help dialogs.
func (m Model) HelpLine() string {
	return "ctrl+v  toggle compact/verbose output"
}
