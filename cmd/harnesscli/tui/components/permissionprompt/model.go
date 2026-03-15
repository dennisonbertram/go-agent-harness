package permissionprompt

// Model renders a permission prompt for tool execution approval.
type Model struct {
	// ToolName is the tool requesting permission.
	ToolName string
	// Description explains what the tool will do.
	Description string
	// Approved tracks the user's decision.
	Approved *bool
	// Width is the available rendering width.
	Width int
}

// New creates a new permission prompt model.
func New(toolName, description string) Model {
	return Model{ToolName: toolName, Description: description}
}

// View renders the permission prompt. Stub for now.
func (m Model) View() string {
	return ""
}
