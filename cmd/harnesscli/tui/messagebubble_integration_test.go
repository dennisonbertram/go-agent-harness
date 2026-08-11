package tui_test

import (
	"fmt"
	"strings"
	"testing"

	tui "go-agent-harness/cmd/harnesscli/tui"
	"go-agent-harness/cmd/harnesscli/tui/components/inputarea"
)

func TestMessageBubbleRouting_StreamedAssistantResponseUsesBubbleRenderer(t *testing.T) {
	m := initModel(t, 80, 24)

	m1, _ := m.Update(inputarea.CommandSubmittedMsg{Value: "What is 2+2?"})
	m2, _ := m1.(tui.Model).Update(tui.AssistantDeltaMsg{Delta: "The answer is 4."})
	m3, _ := m2.(tui.Model).Update(tui.AssistantDeltaMsg{Delta: " Simple math."})

	view := m3.(tui.Model).View()
	if !strings.Contains(view, "What is 2+2?") {
		t.Fatalf("user message missing from view: %q", view)
	}
	if !strings.Contains(view, "⏺") {
		t.Fatalf("assistant bubble prefix missing from view: %q", view)
	}
	if !strings.Contains(view, "The answer is 4. Simple math.") {
		t.Fatalf("assistant content missing from bubble-rendered view: %q", view)
	}
}

func TestRegression_MessageBubbleStreamingPreservesTranscriptEntries(t *testing.T) {
	m := initModel(t, 80, 24)

	m1, _ := m.Update(inputarea.CommandSubmittedMsg{Value: "hello"})
	m2, _ := m1.(tui.Model).Update(tui.AssistantDeltaMsg{Delta: "hello"})
	m3, _ := m2.(tui.Model).Update(tui.AssistantDeltaMsg{Delta: " world"})
	m4, _ := m3.(tui.Model).Update(tui.SSEDoneMsg{EventType: "run.completed"})

	entries := m4.(tui.Model).Transcript()
	if len(entries) != 2 {
		t.Fatalf("expected 2 transcript entries after bubble rendering, got %d", len(entries))
	}
	if entries[0].Role != "user" || entries[0].Content != "hello" {
		t.Fatalf("unexpected user transcript entry: %+v", entries[0])
	}
	if entries[1].Role != "assistant" || entries[1].Content != "hello world" {
		t.Fatalf("unexpected assistant transcript entry: %+v", entries[1])
	}
}

// TestRegression_FinalOnlyAssistantMessagesPreserveTwoTurnConversation proves
// that terminal-only providers render and export a complete two-turn exchange
// without duplicating either final assistant response.
func TestRegression_FinalOnlyAssistantMessagesPreserveTwoTurnConversation(t *testing.T) {
	m := initModel(t, 100, 30).WithCancelRun(func() {})

	turns := []struct {
		user, runID, assistant string
	}{
		{"FIRST_USER_SENTINEL", "run-final-only-one", "FIRST_ASSISTANT_FINAL_SENTINEL"},
		{"SECOND_USER_SENTINEL", "run-final-only-two", "SECOND_ASSISTANT_FINAL_SENTINEL"},
	}
	for _, turn := range turns {
		next, _ := m.Update(inputarea.CommandSubmittedMsg{Value: turn.user})
		m = next.(tui.Model)
		next, _ = m.Update(tui.RunStartedMsg{RunID: turn.runID})
		m = next.(tui.Model)
		next, _ = m.Update(tui.SSEEventMsg{
			EventType: "assistant.message",
			Raw:       []byte(fmt.Sprintf(`{"content":%q}`, turn.assistant)),
		})
		m = next.(tui.Model)
		next, _ = m.Update(tui.SSEDoneMsg{EventType: "run.completed"})
		m = next.(tui.Model)
	}

	wantTranscript := []struct{ role, content string }{
		{"user", "FIRST_USER_SENTINEL"},
		{"assistant", "FIRST_ASSISTANT_FINAL_SENTINEL"},
		{"user", "SECOND_USER_SENTINEL"},
		{"assistant", "SECOND_ASSISTANT_FINAL_SENTINEL"},
	}
	transcript := m.Transcript()
	if len(transcript) != len(wantTranscript) {
		t.Fatalf("transcript = %+v, want exactly user/assistant/user/assistant", transcript)
	}
	for i, want := range wantTranscript {
		if transcript[i].Role != want.role || transcript[i].Content != want.content {
			t.Fatalf("transcript[%d] = %+v, want role=%q content=%q", i, transcript[i], want.role, want.content)
		}
	}

	view := m.View()
	previous := -1
	for _, want := range wantTranscript {
		if got := strings.Count(view, want.content); got != 1 {
			t.Fatalf("viewport renders %q %d times, want once; view=%q", want.content, got, view)
		}
		position := strings.Index(view, want.content)
		if position <= previous {
			t.Fatalf("viewport order is not user/assistant/user/assistant; view=%q", view)
		}
		previous = position
	}
}

func TestRegression_MessageBubbleStreamingKeepsViewportAtBottom(t *testing.T) {
	m := initModel(t, 80, 24)

	m1, _ := m.Update(inputarea.CommandSubmittedMsg{Value: "stream to the bottom"})
	m = m1.(tui.Model)
	for i := 0; i < 40; i++ {
		next, _ := m.Update(tui.AssistantDeltaMsg{Delta: fmt.Sprintf("line %d\n", i)})
		m = next.(tui.Model)
	}

	if !m.ViewportAtBottom() {
		t.Fatalf("viewport lost autoscroll while assistant deltas rendered through messagebubble; offset=%d", m.ViewportScrollOffset())
	}
}
