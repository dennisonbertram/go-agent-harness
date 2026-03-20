package tui_test

import (
	"strings"
	"testing"

	tui "go-agent-harness/cmd/harnesscli/tui"
)

func TestThinkingRouting_DirectThinkingDeltaShowsAndClears(t *testing.T) {
	m := initModel(t, 120, 40)

	first, _ := m.Update(tui.ThinkingDeltaMsg{Delta: "planning next step"})
	m = first.(tui.Model)

	second, _ := m.Update(tui.ThinkingDeltaMsg{Delta: " carefully"})
	m = second.(tui.Model)

	view := m.View()
	if !strings.Contains(view, "Thinking: planning next step carefully...") {
		t.Fatalf("expected visible thinking text in the root view, got %q", view)
	}

	assistant, _ := m.Update(tui.AssistantDeltaMsg{Delta: "final answer"})
	m = assistant.(tui.Model)

	view = m.View()
	if strings.Contains(view, "Thinking: planning next step carefully...") {
		t.Fatalf("thinking indicator should clear once assistant output starts, got %q", view)
	}
	if !strings.Contains(view, "final answer") {
		t.Fatalf("assistant output should remain visible after thinking clears, got %q", view)
	}
}

func TestThinkingRouting_SSEThinkingDeltaUsesContentField(t *testing.T) {
	m := initModel(t, 120, 40)
	m = m.WithCancelRun(func() {})

	started, _ := m.Update(tui.RunStartedMsg{RunID: "run-thinking-1"})
	m = started.(tui.Model)

	thinking, _ := m.Update(tui.SSEEventMsg{
		EventType: "assistant.thinking.delta",
		Raw:       []byte(`{"content":"drafting a plan","step":1}`),
	})
	m = thinking.(tui.Model)

	view := m.View()
	if !strings.Contains(view, "Thinking: drafting a plan...") {
		t.Fatalf("expected SSE thinking delta content to be visible, got %q", view)
	}

	answer, _ := m.Update(tui.SSEEventMsg{
		EventType: "assistant.message.delta",
		Raw:       []byte(`{"content":"done","step":1}`),
	})
	m = answer.(tui.Model)

	view = m.View()
	if strings.Contains(view, "Thinking: drafting a plan...") {
		t.Fatalf("thinking indicator should clear after assistant.message.delta, got %q", view)
	}
	if !strings.Contains(view, "done") {
		t.Fatalf("assistant SSE output should render after thinking clears, got %q", view)
	}
}
