package tui_test

import (
	"strings"
	"testing"

	tui "go-agent-harness/cmd/harnesscli/tui"
)

// TestSSEEventMsg_AssistantDeltaUsesContentField verifies that the model
// correctly parses the "content" field from assistant.message.delta events
// and renders it in the viewport.
func TestSSEEventMsg_AssistantDeltaUsesContentField(t *testing.T) {
	m := initModel(t, 80, 24)

	m2, _ := m.Update(tui.SSEEventMsg{
		EventType: "assistant.message.delta",
		Raw:       []byte(`{"content":"hello world","step":1}`),
	})
	result := m2.(tui.Model)

	// lastAssistantText must accumulate the content.
	if result.LastAssistantText() != "hello world" {
		t.Errorf("LastAssistantText() = %q, want %q", result.LastAssistantText(), "hello world")
	}

	// Viewport must contain the content.
	view := result.View()
	if !strings.Contains(view, "hello world") {
		t.Errorf("viewport does not contain 'hello world'; view=%q", view)
	}
}

// TestSSEEventMsg_AssistantDeltaTextFieldIgnored is a regression test that
// confirms the old (wrong) "text" field is NOT parsed — if a payload uses
// "text" instead of "content", nothing should be appended.
func TestSSEEventMsg_AssistantDeltaTextFieldIgnored(t *testing.T) {
	m := initModel(t, 80, 24)

	m2, _ := m.Update(tui.SSEEventMsg{
		EventType: "assistant.message.delta",
		Raw:       []byte(`{"text":"should be ignored","step":1}`),
	})
	result := m2.(tui.Model)

	// lastAssistantText must stay empty.
	if result.LastAssistantText() != "" {
		t.Errorf("LastAssistantText() = %q, want empty (wrong field name should be ignored)", result.LastAssistantText())
	}

	// Viewport must NOT contain the ignored text.
	view := result.View()
	if strings.Contains(view, "should be ignored") {
		t.Errorf("viewport contains 'should be ignored' but it should be silently dropped; view=%q", view)
	}
}

// TestRegression_AssistantResponseRendered simulates a complete message flow:
// RunStartedMsg -> SSEEventMsg(assistant.message.delta) -> SSEDoneMsg and
// asserts the response is visible in the viewport.
func TestRegression_AssistantResponseRendered(t *testing.T) {
	m := initModel(t, 80, 24)

	// Wire a no-op cancel so RunStartedMsg does not launch a real SSE bridge.
	m = m.WithCancelRun(func() {})

	// RunStartedMsg
	m2, _ := m.Update(tui.RunStartedMsg{RunID: "run-test-1"})
	model := m2.(tui.Model)

	// assistant.message.delta with content = "2+2=4"
	m3, _ := model.Update(tui.SSEEventMsg{
		EventType: "assistant.message.delta",
		Raw:       []byte(`{"content":"2+2=4","step":1}`),
	})
	model = m3.(tui.Model)

	// SSEDoneMsg (run completed)
	m4, _ := model.Update(tui.SSEDoneMsg{EventType: "run.completed"})
	model = m4.(tui.Model)

	view := model.View()
	if !strings.Contains(view, "2+2=4") {
		t.Errorf("viewport should contain '2+2=4' after complete message flow; view=%q", view)
	}

	if model.LastAssistantText() != "2+2=4" {
		t.Errorf("LastAssistantText() = %q, want %q", model.LastAssistantText(), "2+2=4")
	}
}

// TestSSEDelta_AccumulatesOnOneLine sends 3 delta SSEEventMsgs and asserts
// that the viewport does NOT render each token as a separate line.
func TestSSEDelta_AccumulatesOnOneLine(t *testing.T) {
	m := initModel(t, 80, 24)
	m = m.WithCancelRun(func() {})

	m2, _ := m.Update(tui.RunStartedMsg{RunID: "run-delta-1"})
	model := m2.(tui.Model)

	deltas := []string{"Hello", ",", " world"}
	for _, d := range deltas {
		raw := []byte(`{"content":"` + d + `"}`)
		m3, _ := model.Update(tui.SSEEventMsg{
			EventType: "assistant.message.delta",
			Raw:       raw,
		})
		model = m3.(tui.Model)
	}

	view := model.View()

	// The assembled text must appear in the view.
	if !strings.Contains(view, "Hello, world") {
		t.Errorf("assembled text 'Hello, world' not found in view: %q", view)
	}

	// Count lines that contain each token separately — there must NOT be 3
	// lines each containing only a single token from the set.
	tokenOnlyLines := 0
	for _, l := range strings.Split(view, "\n") {
		trimmed := strings.TrimSpace(l)
		if trimmed == "Hello" || trimmed == "," || trimmed == " world" || trimmed == "world" {
			tokenOnlyLines++
		}
	}
	if tokenOnlyLines > 0 {
		t.Errorf("found %d lines with isolated tokens — tokens should be on one line; view=%q", tokenOnlyLines, view)
	}
}

// TestRegression_StreamingNotOnePerLine asserts that after 3 delta events the
// viewport line count attributable to those deltas equals 1, not 3.
func TestRegression_StreamingNotOnePerLine(t *testing.T) {
	m := initModel(t, 80, 24)
	m = m.WithCancelRun(func() {})

	m2, _ := m.Update(tui.RunStartedMsg{RunID: "run-stream-1"})
	model := m2.(tui.Model)

	// Send 3 distinct delta tokens.
	tokens := []string{"token1", "token2", "token3"}
	for _, tok := range tokens {
		raw := []byte(`{"content":"` + tok + `"}`)
		m3, _ := model.Update(tui.SSEEventMsg{
			EventType: "assistant.message.delta",
			Raw:       raw,
		})
		model = m3.(tui.Model)
	}

	view := model.View()

	// Count how many lines contain any of the individual tokens
	// but NOT the fully assembled form — these would be "wrong" isolated lines.
	assembled := "token1token2token3"
	tokenLines := 0
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(l, "token1") || strings.Contains(l, "token2") || strings.Contains(l, "token3") {
			tokenLines++
		}
	}
	// There should be exactly 1 line containing the assembled token content.
	if tokenLines != 1 {
		t.Errorf("expected exactly 1 line containing streaming tokens, got %d; assembled=%q view=%q",
			tokenLines, assembled, view)
	}
}
