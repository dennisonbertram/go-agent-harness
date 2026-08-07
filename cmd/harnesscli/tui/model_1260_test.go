package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"go-agent-harness/cmd/harnesscli/tui"
)

func update1260(t *testing.T, m tui.Model, msg any) tui.Model {
	t.Helper()
	next, _ := m.Update(msg)
	return next.(tui.Model)
}

func resumed1260Model(t *testing.T) tui.Model {
	t.Helper()
	cfg := tui.DefaultTUIConfig()
	cfg.ResumeConversationID = "resumed-conversation"
	m := tui.New(cfg).WithCancelRun(func() {})
	m = update1260(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	return update1260(t, m, tui.SSEEventMsg{
		Conversation: true, ConversationID: m.ConversationID(),
		EventType: "conversation.replay.completed",
		Raw:       []byte(`{"messages":[{"role":"assistant","content":"CALLBACK_HISTORY"}],"last_event_id":"callback-run:7"}`),
	})
}

// TestIssue1260_ResumedConversationSameRunCopiesKeepFreshReply exercises the
// two subscriptions owned by a resumed TUI. The conversation feed can see a
// new run's authoritative final before the run-local feed, and the latter must
// then be harmlessly deduplicated rather than losing the reply.
func TestIssue1260_ResumedConversationSameRunCopiesKeepFreshReply(t *testing.T) {
	m := resumed1260Model(t)
	m = update1260(t, m, tui.RunStartedMsg{RunID: "local-run"})
	m = update1260(t, m, tui.SSEEventMsg{
		Conversation: true, ConversationID: m.ConversationID(), RunID: "local-run", ID: "local-run:1",
		EventType: "assistant.message", Raw: []byte(`{"content":"FRESH_LOCAL_REPLY"}`),
	})
	m = update1260(t, m, tui.SSEEventMsg{
		RunID: "local-run", ID: "local-run:1",
		EventType: "assistant.message", Raw: []byte(`{"content":"FRESH_LOCAL_REPLY"}`),
	})
	m = update1260(t, m, tui.SSEDoneMsg{EventType: "run.completed"})

	if got := strings.Count(m.View(), "FRESH_LOCAL_REPLY"); got != 1 {
		t.Fatalf("fresh reply rendered %d times, want once: %s", got, m.View())
	}
	if got := len(m.Transcript()); got != 2 || m.Transcript()[0].Content != "CALLBACK_HISTORY" || m.Transcript()[1].Content != "FRESH_LOCAL_REPLY" {
		t.Fatalf("transcript = %#v, want retained callback history plus the fresh reply", m.Transcript())
	}
}

// TestIssue1260_ConversationCopyBeforeRunStartedRetainsFreshReply exercises
// the inverse race. The conversation stream is already attached during resume
// and can deliver the new run's final before startRunCmd returns RunStartedMsg.
// The local-stream copy has the same event ID, so RunStarted must retain that
// run's accumulator for the terminal transcript rather than resetting it.
func TestIssue1260_ConversationCopyBeforeRunStartedRetainsFreshReply(t *testing.T) {
	m := resumed1260Model(t)
	m = update1260(t, m, tui.SSEEventMsg{
		Conversation: true, ConversationID: m.ConversationID(), RunID: "local-run", ID: "local-run:1",
		EventType: "assistant.message", Raw: []byte(`{"content":"FRESH_BEFORE_START"}`),
	})
	m = update1260(t, m, tui.RunStartedMsg{RunID: "local-run"})
	m = update1260(t, m, tui.SSEEventMsg{
		RunID: "local-run", ID: "local-run:1",
		EventType: "assistant.message", Raw: []byte(`{"content":"FRESH_BEFORE_START"}`),
	})
	m = update1260(t, m, tui.SSEDoneMsg{EventType: "run.completed", RunID: "local-run"})

	if got := strings.Count(m.View(), "FRESH_BEFORE_START"); got != 1 {
		t.Fatalf("fresh reply rendered %d times, want once: %s", got, m.View())
	}
	entries := m.Transcript()
	if got := len(entries); got != 2 || entries[1].Content != "FRESH_BEFORE_START" {
		t.Fatalf("transcript = %#v, want retained history plus fresh reply", entries)
	}
}

// TestIssue1260_RunTerminalBeforeConversationCopyKeepsFreshReply models the
// cross-stream scheduling order from the live regression: a terminal from the
// run feed reaches Bubble Tea before its conversation-feed copy of the final.
func TestIssue1260_RunTerminalBeforeConversationCopyKeepsFreshReply(t *testing.T) {
	m := resumed1260Model(t)
	m = update1260(t, m, tui.RunStartedMsg{RunID: "local-run"})
	m = update1260(t, m, tui.SSEDoneMsg{EventType: "run.completed", RunID: "local-run"})
	m = update1260(t, m, tui.SSEEventMsg{
		Conversation: true, ConversationID: m.ConversationID(), RunID: "local-run", ID: "local-run:1",
		EventType: "assistant.message", Raw: []byte(`{"content":"FRESH_AFTER_TERMINAL"}`),
	})
	m = update1260(t, m, tui.SSEEventMsg{
		RunID: "local-run", ID: "local-run:1",
		EventType: "assistant.message", Raw: []byte(`{"content":"FRESH_AFTER_TERMINAL"}`),
	})

	if got := strings.Count(m.View(), "FRESH_AFTER_TERMINAL"); got != 1 {
		t.Fatalf("fresh reply rendered %d times, want once: %s", got, m.View())
	}
	entries := m.Transcript()
	if got := len(entries); got != 2 || entries[1].Content != "FRESH_AFTER_TERMINAL" {
		t.Fatalf("transcript = %#v, want retained callback history plus late authoritative final", entries)
	}
}

// TestIssue1260_LatePriorTerminalCannotFinalizeFreshRun captures the failing
// production shape: the selected-conversation feed can still deliver an old
// callback while the next local run is starting. Before this regression, the
// late terminal was indistinguishable from the current run terminal; it
// finalized the shared assistant accumulator, so the new same-run reply was
// rejected and its local duplicate was suppressed by event ID.
func TestIssue1260_LatePriorTerminalCannotFinalizeFreshRun(t *testing.T) {
	m := resumed1260Model(t)
	m = update1260(t, m, tui.RunStartedMsg{RunID: "local-run"})

	// A delayed callback assistant event reaches the resumed conversation feed.
	m = update1260(t, m, tui.SSEEventMsg{
		Conversation: true, ConversationID: m.ConversationID(), RunID: "callback-run", ID: "callback-run:8",
		EventType: "assistant.message", Raw: []byte(`{"content":"CALLBACK_HISTORY"}`),
	})
	// Before the repair SSEDoneMsg carried no run owner, so this terminal
	// incorrectly finalized local-run's accumulator.
	m = update1260(t, m, tui.SSEDoneMsg{EventType: "run.completed", RunID: "callback-run"})

	m = update1260(t, m, tui.SSEEventMsg{
		Conversation: true, ConversationID: m.ConversationID(), RunID: "local-run", ID: "local-run:1",
		EventType: "assistant.message", Raw: []byte(`{"content":"FRESH_AFTER_LATE_CALLBACK"}`),
	})
	m = update1260(t, m, tui.SSEEventMsg{
		RunID: "local-run", ID: "local-run:1",
		EventType: "assistant.message", Raw: []byte(`{"content":"FRESH_AFTER_LATE_CALLBACK"}`),
	})

	if got := strings.Count(m.View(), "FRESH_AFTER_LATE_CALLBACK"); got != 1 {
		t.Fatalf("fresh reply rendered %d times, want once: %s", got, m.View())
	}
}
