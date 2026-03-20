package tui_test

import (
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tui "go-agent-harness/cmd/harnesscli/tui"
)

const sampleUnifiedDiff = `--- a/main.go
+++ b/main.go
@@ -1,3 +1,3 @@
-old line
+new line
 context line`

func TestDiffViewRouting_SSECompletedToolResultUsesDiffComponent(t *testing.T) {
	m := initModel(t, 120, 40)
	m = m.WithCancelRun(func() {})

	startedRun, _ := m.Update(tui.RunStartedMsg{RunID: "run-diff-1"})
	m = startedRun.(tui.Model)

	startedTool, _ := m.Update(tui.SSEEventMsg{
		EventType: "tool.call.started",
		Raw:       []byte(`{"tool":"git_diff","call_id":"call-diff","arguments":"HEAD"}`),
	})
	m = startedTool.(tui.Model)

	expanded, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = expanded.(tui.Model)

	payload, err := json.Marshal(map[string]any{
		"tool":        "git_diff",
		"call_id":     "call-diff",
		"output":      sampleUnifiedDiff,
		"duration_ms": 12,
	})
	if err != nil {
		t.Fatalf("marshal diff payload: %v", err)
	}

	completed, _ := m.Update(tui.SSEEventMsg{
		EventType: "tool.call.completed",
		Raw:       payload,
	})
	m = completed.(tui.Model)

	view := m.View()
	if !strings.Contains(view, "main.go") {
		t.Fatalf("expected diff header filename in root view, got %q", view)
	}
	if !strings.Contains(view, "╌") {
		t.Fatalf("expected diff viewer border in root view, got %q", view)
	}
	if strings.Contains(view, "⎿  --- a/main.go") {
		t.Fatalf("root tool rendering should not fall back to generic expanded tool lines for unified diffs, got %q", view)
	}
}
