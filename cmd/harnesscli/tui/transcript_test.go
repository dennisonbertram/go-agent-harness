package tui_test

import (
	"encoding/json"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tui "go-agent-harness/cmd/harnesscli/tui"
	"go-agent-harness/cmd/harnesscli/tui/components/inputarea"
)

// newReadyModel returns a model initialised at 80x24 for use in transcript tests.
func newReadyModel() tui.Model {
	m := tui.New(tui.DefaultTUIConfig())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return m2.(tui.Model)
}

// sseContentMsg builds an SSEEventMsg carrying an assistant.message.delta payload.
func sseContentMsg(content string) tui.SSEEventMsg {
	raw, _ := json.Marshal(map[string]string{"content": content})
	return tui.SSEEventMsg{
		EventType: "assistant.message.delta",
		Raw:       raw,
	}
}

// TestTranscript_AssistantResponseRecorded verifies that when a run completes the
// accumulated assistant deltas are appended to the transcript as a single entry.
func TestTranscript_AssistantResponseRecorded(t *testing.T) {
	m := newReadyModel()

	// User submits a message.
	m1, _ := m.Update(inputarea.CommandSubmittedMsg{Value: "hello"})
	// Run starts.
	m2, _ := m1.(tui.Model).Update(tui.RunStartedMsg{RunID: "r1"})
	// Assistant streams two deltas.
	m3, _ := m2.(tui.Model).Update(sseContentMsg("hello"))
	m4, _ := m3.(tui.Model).Update(sseContentMsg(" world"))
	// Run completes.
	m5, _ := m4.(tui.Model).Update(tui.SSEDoneMsg{EventType: "run.completed"})

	model := m5.(tui.Model)
	entries := model.Transcript()

	if len(entries) != 2 {
		t.Fatalf("expected 2 transcript entries (user + assistant), got %d", len(entries))
	}

	if entries[0].Role != "user" {
		t.Errorf("entries[0].Role: want %q, got %q", "user", entries[0].Role)
	}
	if entries[0].Content != "hello" {
		t.Errorf("entries[0].Content: want %q, got %q", "hello", entries[0].Content)
	}

	if entries[1].Role != "assistant" {
		t.Errorf("entries[1].Role: want %q, got %q", "assistant", entries[1].Role)
	}
	if entries[1].Content != "hello world" {
		t.Errorf("entries[1].Content: want %q, got %q", "hello world", entries[1].Content)
	}
}

// TestTranscript_NoAssistantEntryOnEmptyResponse verifies that when the run ends
// with no assistant text, no empty assistant entry is appended.
func TestTranscript_NoAssistantEntryOnEmptyResponse(t *testing.T) {
	m := newReadyModel()

	m1, _ := m.Update(tui.RunStartedMsg{RunID: "r1"})
	m2, _ := m1.(tui.Model).Update(tui.SSEDoneMsg{EventType: "run.completed"})

	model := m2.(tui.Model)
	entries := model.Transcript()

	for _, e := range entries {
		if e.Role == "assistant" && e.Content == "" {
			t.Errorf("transcript must not contain empty assistant entry, got: %+v", e)
		}
	}
}

// TestRegression_ExportIncludesAssistantResponses verifies a two-turn conversation
// produces four transcript entries in the correct order: user/assistant/user/assistant.
func TestRegression_ExportIncludesAssistantResponses(t *testing.T) {
	m := newReadyModel()

	// Turn 1: user sends a message and receives an assistant response.
	m1, _ := m.Update(inputarea.CommandSubmittedMsg{Value: "turn one user"})
	m2, _ := m1.(tui.Model).Update(tui.RunStartedMsg{RunID: "r1"})
	m3, _ := m2.(tui.Model).Update(sseContentMsg("turn one assistant"))
	m4, _ := m3.(tui.Model).Update(tui.SSEDoneMsg{EventType: "run.completed"})

	// Turn 2: user sends another message and receives another assistant response.
	m5, _ := m4.(tui.Model).Update(inputarea.CommandSubmittedMsg{Value: "turn two user"})
	m6, _ := m5.(tui.Model).Update(tui.RunStartedMsg{RunID: "r2"})
	m7, _ := m6.(tui.Model).Update(sseContentMsg("turn two assistant"))
	m8, _ := m7.(tui.Model).Update(tui.SSEDoneMsg{EventType: "run.completed"})

	model := m8.(tui.Model)
	entries := model.Transcript()

	if len(entries) != 4 {
		t.Fatalf("expected 4 transcript entries, got %d: %+v", len(entries), entries)
	}

	want := []struct {
		role    string
		content string
	}{
		{"user", "turn one user"},
		{"assistant", "turn one assistant"},
		{"user", "turn two user"},
		{"assistant", "turn two assistant"},
	}

	for i, w := range want {
		if entries[i].Role != w.role {
			t.Errorf("entries[%d].Role: want %q, got %q", i, w.role, entries[i].Role)
		}
		if entries[i].Content != w.content {
			t.Errorf("entries[%d].Content: want %q, got %q", i, w.content, entries[i].Content)
		}
	}
}
