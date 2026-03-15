package permissionprompt

import (
	tea "github.com/charmbracelet/bubbletea"
)

// PromptOption represents an approval choice.
type PromptOption int

const (
	OptionYes    PromptOption = iota // allow once
	OptionNo                         // deny
	OptionAllowAll                   // allow for session
)

// optionLabel returns the human-readable label for a PromptOption.
func optionLabel(o PromptOption) string {
	switch o {
	case OptionYes:
		return "Yes (allow once)"
	case OptionNo:
		return "No (deny)"
	case OptionAllowAll:
		return "Allow all (this session)"
	default:
		return "Unknown"
	}
}

// PromptResult is the resolved approval decision.
type PromptResult struct {
	Option  PromptOption
	Amended string // amended resource if Tab-amended
}

// Model is the permission prompt state machine.
//
// Model is a value type — all Update calls return a new Model. This guarantees
// no data races when multiple goroutines each hold their own copy.
type Model struct {
	ToolName string
	Resource string         // file path, command, etc.
	Options  []PromptOption // available choices (nil = empty/fallback)
	selected int            // cursor index into Options
	active   bool
	resolved bool
	result   PromptResult
	amended  string // user-amended resource (accumulates typed chars)
	amending bool   // Tab-amend mode active
}

// New creates a new, active permission prompt with the given tool name,
// resource, and option list.
func New(toolName, resource string, options []PromptOption) Model {
	return Model{
		ToolName: toolName,
		Resource: resource,
		Options:  options,
		active:   true,
	}
}

// IsActive reports whether the prompt is awaiting a decision.
func (m Model) IsActive() bool { return m.active && !m.resolved }

// IsResolved reports whether the user has made a decision.
func (m Model) IsResolved() bool { return m.resolved }

// IsAmending reports whether the user is currently in Tab-amend mode.
func (m Model) IsAmending() bool { return m.amending }

// AmendedResource returns the resource string as amended by the user (may be
// empty if no amendment was made).
func (m Model) AmendedResource() string { return m.amended }

// Result returns the resolved decision. Only meaningful after IsResolved().
func (m Model) Result() PromptResult { return m.result }

// Update processes a tea.Msg and returns the updated Model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if m.resolved {
		return m, nil
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if m.amending {
		return m.updateAmending(key)
	}
	return m.updateSelecting(key)
}

// updateSelecting handles key input in normal (option selection) mode.
func (m Model) updateSelecting(key tea.KeyMsg) (Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyUp:
		if len(m.Options) > 0 && m.selected > 0 {
			m.selected--
		}

	case tea.KeyDown:
		if len(m.Options) > 0 && m.selected < len(m.Options)-1 {
			m.selected++
		}

	case tea.KeyEnter:
		if len(m.Options) == 0 {
			// Fallback: deny when no options provided.
			m.resolved = true
			m.active = false
			m.result = PromptResult{Option: OptionNo}
		} else {
			chosen := m.Options[m.selected]
			m.resolved = true
			m.active = false
			m.result = PromptResult{
				Option:  chosen,
				Amended: m.amended,
			}
		}

	case tea.KeyEsc:
		m.resolved = true
		m.active = false
		m.result = PromptResult{Option: OptionNo}

	case tea.KeyTab:
		// Enter amend mode. Clear the amended buffer so the user types a fresh
		// path (the original resource is still visible in the header).
		m.amending = true
		m.amended = ""
	}

	return m, nil
}

// updateAmending handles key input in Tab-amend mode.
func (m Model) updateAmending(key tea.KeyMsg) (Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyEnter:
		// Confirm amendment; return to selection mode.
		m.amending = false

	case tea.KeyEsc:
		// Cancel amendment; return to selection mode without changing amended.
		m.amending = false
		m.amended = ""

	case tea.KeyBackspace, tea.KeyDelete:
		if len(m.amended) > 0 {
			// Remove last rune (UTF-8 safe).
			runes := []rune(m.amended)
			m.amended = string(runes[:len(runes)-1])
		}

	case tea.KeyRunes:
		m.amended += string(key.Runes)
	}

	return m, nil
}
