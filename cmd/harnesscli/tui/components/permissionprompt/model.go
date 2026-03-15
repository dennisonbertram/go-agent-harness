package permissionprompt

// ApprovalState represents the user's decision on a permission prompt.
type ApprovalState int

const (
	// ApprovalPending means the user has not yet decided.
	ApprovalPending ApprovalState = iota
	// ApprovalGranted means the user approved the tool execution.
	ApprovalGranted
	// ApprovalDenied means the user denied the tool execution.
	ApprovalDenied
)

// Model renders a permission prompt for tool execution approval.
type Model struct {
	// ToolName is the tool requesting permission.
	ToolName string
	// Description explains what the tool will do.
	Description string
	// Approval tracks the user's decision using a typed enum (no nil pointer).
	Approval ApprovalState
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
