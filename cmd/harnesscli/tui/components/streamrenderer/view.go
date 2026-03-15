package streamrenderer

import "strings"

// Lines returns lines ready for viewport.AppendLine calls.
// Call this when new deltas arrive to push updated content into the viewport.
func (m Model) Lines() []string {
	content := strings.Join(m.content, "")
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}
