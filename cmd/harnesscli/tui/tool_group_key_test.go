package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tui "go-agent-harness/cmd/harnesscli/tui"
)

// emitCall drives one complete tool call through the model.
func emitCall(t *testing.T, m tui.Model, tool, callID string) tui.Model {
	t.Helper()
	m2, _ := m.Update(tui.SSEEventMsg{
		EventType: "tool.call.started",
		Raw:       []byte(`{"tool":"` + tool + `","call_id":"` + callID + `","arguments":{"path":"/x"}}`),
	})
	m = m2.(tui.Model)
	m3, _ := m.Update(tui.SSEEventMsg{
		EventType: "tool.call.completed",
		Raw:       []byte(`{"tool":"` + tool + `","call_id":"` + callID + `","output":"ok","duration_ms":1}`),
	})
	return m3.(tui.Model)
}

// TestToolCallsCollapseAndExpandWithCtrlO is the end-to-end half of issue #1308:
// a tool-heavy turn costs one transcript line until the user asks for more.
func TestToolCallsCollapseAndExpandWithCtrlO(t *testing.T) {
	m := initModel(t, 120, 40)
	m = m.WithCancelRun(func() {})
	m2, _ := m.Update(tui.RunStartedMsg{RunID: "run-1308"})
	m = m2.(tui.Model)

	for _, c := range []struct{ tool, id string }{
		{"ls", "c1"}, {"read", "c2"}, {"git_status", "c3"}, {"read", "c4"},
	} {
		m = emitCall(t, m, c.tool, c.id)
	}

	collapsed := m.View()
	if !strings.Contains(collapsed, "4 tool calls") {
		t.Fatalf("expected a collapsed summary naming 4 calls; view=%q", collapsed)
	}
	// False-positive control: the summary must not be the expanded list.
	if strings.Count(collapsed, "git_status(") > 0 {
		t.Errorf("collapsed view leaked an individual call card; view=%q", collapsed)
	}

	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = m3.(tui.Model)

	expanded := m.View()
	for _, tool := range []string{"ls(", "read(", "git_status("} {
		if !strings.Contains(expanded, tool) {
			t.Errorf("expanded view missing %q; view=%q", tool, expanded)
		}
	}

	m4, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = m4.(tui.Model)

	if recollapsed := m.View(); !strings.Contains(recollapsed, "4 tool calls") {
		t.Errorf("ctrl+o must re-collapse the group; view=%q", recollapsed)
	}
}

// TestFailedToolCallVisibleWhileCollapsed is the safety rule end to end: an error
// must never require a keypress to discover.
func TestFailedToolCallVisibleWhileCollapsed(t *testing.T) {
	m := initModel(t, 120, 40)
	m = m.WithCancelRun(func() {})
	m2, _ := m.Update(tui.RunStartedMsg{RunID: "run-1308-err"})
	m = m2.(tui.Model)

	m = emitCall(t, m, "ls", "ok1")
	m3, _ := m.Update(tui.SSEEventMsg{
		EventType: "tool.call.started",
		Raw:       []byte(`{"tool":"bash","call_id":"bad","arguments":{"command":"ls /app"}}`),
	})
	m = m3.(tui.Model)
	// Failures arrive on tool.call.completed carrying an error, not on a
	// separate event type.
	m4, _ := m.Update(tui.SSEEventMsg{
		EventType: "tool.call.completed",
		Raw:       []byte(`{"tool":"bash","call_id":"bad","error":"sandbox violation: absolute path \"/app\" escapes workspace","duration_ms":2}`),
	})
	m = m4.(tui.Model)
	m = emitCall(t, m, "read", "ok2")

	if view := m.View(); !strings.Contains(view, "sandbox violation") {
		t.Errorf("failure must be visible without expanding; view=%q", view)
	}
}
