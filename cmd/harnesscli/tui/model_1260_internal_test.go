package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestIssue1260_AssistantRunLifecycleIsBoundedAndEvictedTerminalCannotSettleCurrent(t *testing.T) {
	m := New(DefaultTUIConfig())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)
	for i := 0; i <= maxSeenSSEEventIDs; i++ {
		runID := fmt.Sprintf("lifecycle-%d", i)
		next, _ = m.Update(RunStartedMsg{RunID: runID})
		m = next.(Model)
		next, _ = m.Update(SSEEventMsg{RunID: runID, ID: runID + ":1", EventType: "assistant.message", Raw: []byte(`{"content":"final"}`)})
		m = next.(Model)
		next, _ = m.Update(SSEDoneMsg{EventType: "run.completed", RunID: runID})
		m = next.(Model)
	}
	if got := len(m.assistantRunLifecycle); got > maxSeenSSEEventIDs {
		t.Fatalf("assistantRunLifecycle size = %d, want <= %d", got, maxSeenSSEEventIDs)
	}
	if got := len(m.assistantRunText); got > maxSeenSSEEventIDs {
		t.Fatalf("assistantRunText size = %d, want <= %d", got, maxSeenSSEEventIDs)
	}
	if got := len(m.terminalRunIDs); got > maxSeenSSEEventIDs {
		t.Fatalf("terminalRunIDs size = %d, want <= %d", got, maxSeenSSEEventIDs)
	}
	if got := len(m.assistantRunOrder); got > maxSeenSSEEventIDs {
		t.Fatalf("assistantRunOrder size = %d, want <= %d", got, maxSeenSSEEventIDs)
	}

	next, _ = m.Update(RunStartedMsg{RunID: "retained-current"})
	m = next.(Model)
	// lifecycle-0 is evicted. Its terminal must not settle retained-current.
	next, _ = m.Update(SSEDoneMsg{EventType: "run.completed", RunID: "lifecycle-0"})
	m = next.(Model)
	if !m.runActive || m.RunID != "retained-current" {
		t.Fatalf("evicted stale terminal changed current run: active=%v run=%q", m.runActive, m.RunID)
	}
	next, _ = m.Update(SSEEventMsg{RunID: "retained-current", ID: "retained-current:1", EventType: "assistant.message", Raw: []byte(`{"content":"RETAINED_FINAL"}`)})
	m = next.(Model)
	next, _ = m.Update(SSEDoneMsg{EventType: "run.completed", RunID: "retained-current"})
	m = next.(Model)
	if got := m.Transcript(); len(got) == 0 || got[len(got)-1].Content != "RETAINED_FINAL" {
		t.Fatalf("retained current transcript lost final: %#v", got)
	}
}
