package tui_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tui "go-agent-harness/cmd/harnesscli/tui"
)

func TestToolUseRouting_BashOutputFlowsThroughComponentPath(t *testing.T) {
	m := initModel(t, 80, 24)

	started, _ := m.Update(tui.ToolStartMsg{
		CallID: "call-bash",
		Name:   "bash",
		Input:  json.RawMessage(`"echo hello"`),
	})
	m = started.(tui.Model)

	expanded, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = expanded.(tui.Model)

	completed, _ := m.Update(tui.ToolResultMsg{
		CallID: "call-bash",
		Output: strings.Repeat("line\n", 12),
	})
	m = completed.(tui.Model)

	view := m.View()
	if !strings.Contains(view, "bash(") {
		t.Fatalf("tool header missing from view: %q", view)
	}
	if !strings.Contains(view, "echo hello") {
		t.Fatalf("bash command label missing from view: %q", view)
	}
	if !strings.Contains(view, "ctrl+o to expand") {
		t.Fatalf("bash output truncation hint missing from view: %q", view)
	}
}

func TestToolUseRouting_ToolErrorUsesComponentPath(t *testing.T) {
	m := initModel(t, 80, 24)

	started, _ := m.Update(tui.ToolStartMsg{CallID: "call-write", Name: "write_file"})
	m = started.(tui.Model)

	failed, _ := m.Update(tui.ToolErrorMsg{CallID: "call-write", Err: errors.New("permission denied")})
	m = failed.(tui.Model)

	view := m.View()
	if !strings.Contains(view, "permission denied") {
		t.Fatalf("tool error text missing from view: %q", view)
	}
	if !strings.Contains(view, "✗") {
		t.Fatalf("tool error indicator missing from view: %q", view)
	}
}
