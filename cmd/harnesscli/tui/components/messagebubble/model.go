package messagebubble

// Role identifies the sender of a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Model renders a single conversation message bubble.
type Model struct {
	// Role is the message sender role.
	Role Role
	// Content is the message text.
	Content string
	// Width is the available rendering width.
	Width int
}

// New creates a new message bubble.
func New(role Role, content string) Model {
	return Model{Role: role, Content: content}
}

// View renders the message bubble. Stub for now.
func (m Model) View() string {
	return ""
}
