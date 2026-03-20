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
