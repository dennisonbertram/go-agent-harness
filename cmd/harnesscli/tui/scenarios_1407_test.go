package tui_test

import (
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"go-agent-harness/cmd/harnesscli/tui"
)

func sse(m tui.Model, typ, runID, payload string) tui.Model {
	m2, _ := m.Update(tui.SSEEventMsg{EventType: typ, RunID: runID, ID: runID + ":" + typ, Raw: json.RawMessage(payload)})
	return m2.(tui.Model)
}

// After a run is interrupted mid tool call, ctrl+o must not duplicate the
// interrupted tool's card in the transcript.
func TestInterruptedTool_CtrlOKeepsOneCard(t *testing.T) {
	m := initModel(t, 120, 40)
	// Prior history: a finished turn with a two-call tool group and an answer,
	// then a question turn, as in the live session where the bug showed.
	m = typeIntoModel(m, "Create hello.go and run it")
	m = sendKey(m, tea.KeyEnter)
	m0, _ := m.Update(tui.RunStartedMsg{RunID: "run-0"})
	m = m0.(tui.Model)
	m = sse(m, "run.started", "run-0", `{"prompt":"Create hello.go and run it","step":0}`)
	m = sse(m, "tool.call.started", "run-0", `{"call_id":"c1","tool":"write","arguments":"{\"path\":\"hello.go\"}","step":1}`)
	m = sse(m, "tool.call.completed", "run-0", `{"call_id":"c1","tool":"write","output":"ok","duration_ms":1,"step":1}`)
	m = sse(m, "tool.call.started", "run-0", `{"call_id":"c2","tool":"bash","arguments":"{\"command\":\"go run hello.go\"}","step":2}`)
	m = sse(m, "tool.call.completed", "run-0", `{"call_id":"c2","tool":"bash","output":"hi","duration_ms":1,"step":2}`)
	m = sse(m, "assistant.message", "run-0", `{"content":"Created hello.go and ran it.","step":3}`)
	m = sse(m, "run.completed", "run-0", `{"output":"Created hello.go and ran it.","step":3}`)
	m = typeIntoModel(m, "Run a long command")
	m = sendKey(m, tea.KeyEnter)
	m2, _ := m.Update(tui.RunStartedMsg{RunID: "run-1"})
	m = m2.(tui.Model)
	m = sse(m, "run.started", "run-1", `{"prompt":"Run a long command","step":0}`)
	m = sse(m, "tool.call.started", "run-1", `{"call_id":"s1","tool":"bash","arguments":"{\"command\":\"sleep 60\"}","step":1}`)
	m = sendKey(m, tea.KeyEsc) // interrupt
	m = sse(m, "run.cancelled", "run-1", `{"step":1}`)
	before := strings.Count(m.View(), "bash")
	m = sendKey(m, tea.KeyCtrlO)
	after := strings.Count(m.View(), "bash")
	if after > before {
		t.Fatalf("ctrl+o duplicated the interrupted tool card (bash mentions %d -> %d):\n%s", before, after, m.View())
	}
}
