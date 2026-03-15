package tooluse

// Model renders a tool call with its status, arguments, and result.
type Model struct {
	// CallID uniquely identifies this tool call.
	CallID string
	// ToolName is the name of the tool being called.
	ToolName string
	// Status tracks the call lifecycle (pending, running, completed, failed).
	Status string
	// Expanded controls whether the full output is shown.
	Expanded bool
	// Width is the available rendering width.
	Width int
}

// New creates a new tool use display model.
func New(callID, toolName string) Model {
	return Model{CallID: callID, ToolName: toolName}
}

// View renders the tool use display. Stub for now.
func (m Model) View() string {
	return ""
}
