package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func replayBoundaryModel(t *testing.T) Model {
	t.Helper()
	cfg := DefaultTUIConfig()
	cfg.ResumeConversationID = "conv-replay-boundary-state"
	m := New(cfg)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return next.(Model)
}

func selectedSessionReplayBoundaryModel(t *testing.T) Model {
	t.Helper()
	m := New(DefaultTUIConfig())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)
	next, _ = m.Update(SessionPickerSelectedMsg{SessionID: "conv-selected-boundary"})
	m = next.(Model)
	if !m.conversationReplayAwaitingMarker {
		t.Fatal("selected session did not enable the atomic replay boundary")
	}
	return m
}

func replayBoundaryMarker(messages []ConversationMessage, lastEventID string) SSEEventMsg {
	return SSEEventMsg{
		Conversation:   true,
		ConversationID: "conv-replay-boundary-state",
		EventType:      "conversation.replay.completed",
		Raw:            []byte(`{"messages":[{"role":"assistant","content":"` + messages[0].Content + `"}],"last_event_id":"` + lastEventID + `"}`),
	}
}

// A stale history completion must not win the events-first handshake. The
// marker snapshot is the only rendering authority; a queued post-marker event
// then flows through the ordinary assistant-message reducer.
func TestHistoryMsgFirstThenQueuedCursor(t *testing.T) {
	m := replayBoundaryModel(t)
	first, _ := m.Update(ConversationHistoryMsg{
		ConversationID: m.conversationID,
		Messages:       []ConversationMessage{{Role: "assistant", Content: "STALE_HISTORY_MUST_NOT_RENDER"}},
		LastEventID:    "stale:1",
	})
	m = first.(Model)
	if strings.Contains(m.View(), "STALE_HISTORY_MUST_NOT_RENDER") {
		t.Fatal("history result rendered while atomic marker was still pending")
	}
	pre, _ := m.Update(SSEEventMsg{Conversation: true, ConversationID: m.conversationID, ID: "pre:1", EventType: "assistant.message", Raw: []byte(`{"content":"PREMARKER_MUST_NOT_RENDER"}`)})
	m = pre.(Model)
	marked, _ := m.Update(replayBoundaryMarker([]ConversationMessage{{Role: "assistant", Content: "ATOMIC_SNAPSHOT_ONCE"}}, "pre:1"))
	m = marked.(Model)
	queued, _ := m.Update(SSEEventMsg{Conversation: true, ConversationID: m.conversationID, ID: "queued:2", RunID: "queued", EventType: "assistant.message", Raw: []byte(`{"content":"QUEUED_NORMAL_REDUCER"}`)})
	m = queued.(Model)
	if got := m.View(); strings.Contains(got, "PREMARKER_MUST_NOT_RENDER") || !strings.Contains(got, "ATOMIC_SNAPSHOT_ONCE") || !strings.Contains(got, "QUEUED_NORMAL_REDUCER") {
		t.Fatalf("boundary ordering rendered wrong state: %s", got)
	}
}

// A durable snapshot cursor need not occur in the bounded replay page. That
// absence never grants a pre-marker event permission to render.
func TestNonemptyCursorAbsentKeepsDiscarding(t *testing.T) {
	m := replayBoundaryModel(t)
	for _, id := range []string{"replay:1", "replay:2"} {
		next, _ := m.Update(SSEEventMsg{Conversation: true, ConversationID: m.conversationID, ID: id, EventType: "assistant.message", Raw: []byte(`{"content":"PREMARKER_ABSENT_CURSOR"}`)})
		m = next.(Model)
	}
	next, _ := m.Update(replayBoundaryMarker([]ConversationMessage{{Role: "assistant", Content: "SNAPSHOT_WITH_ABSENT_CURSOR"}}, "durable:99"))
	m = next.(Model)
	if got := m.View(); strings.Contains(got, "PREMARKER_ABSENT_CURSOR") || !strings.Contains(got, "SNAPSHOT_WITH_ABSENT_CURSOR") {
		t.Fatalf("nonempty absent cursor did not keep replay suppression: %s", got)
	}
}

// A post-marker event is intentionally not special-cased as an assistant
// append: it enters the existing normal reducer, preserving every event type's
// established behavior.
func TestPostBoundaryWaitingEventUsesNormalReducer(t *testing.T) {
	m := replayBoundaryModel(t)
	next, _ := m.Update(replayBoundaryMarker([]ConversationMessage{{Role: "assistant", Content: "SNAPSHOT"}}, "snapshot:1"))
	m = next.(Model)
	next, _ = m.Update(SSEEventMsg{Conversation: true, ConversationID: m.conversationID, ID: "waiting:2", RunID: "external", EventType: "run.waiting_for_user", Raw: []byte(`{"run_id":"external","call_id":"ask-1"}`)})
	m = next.(Model)
	if !m.askUser.active || m.askUser.runID != "external" {
		t.Fatalf("post-marker waiting event bypassed normal reducer: %+v", m.askUser)
	}
}

// A later turn may intentionally repeat the same assistant text. The atomic
// marker protocol must not rely on content equality to suppress it.
func TestPostBoundarySameContentSemanticTurnIsNotSuppressed(t *testing.T) {
	m := replayBoundaryModel(t)
	next, _ := m.Update(replayBoundaryMarker([]ConversationMessage{{Role: "assistant", Content: "SAME_TEXT"}}, "snapshot:1"))
	m = next.(Model)
	next, _ = m.Update(SSEEventMsg{Conversation: true, ConversationID: m.conversationID, ID: "later:2", RunID: "later", EventType: "assistant.message", Raw: []byte(`{"content":"SAME_TEXT"}`)})
	m = next.(Model)
	if got := strings.Count(m.View(), "SAME_TEXT"); got != 2 {
		t.Fatalf("same-content later turn rendered %d times, want 2", got)
	}
}

// The legacy GET-first path must clear its temporary state on failure, then
// permit the restarted stream to deliver future conversation events.
func TestHistoryFailureReleasesLegacyLiveDelivery(t *testing.T) {
	m := replayBoundaryModel(t)
	next, _ := m.Update(SSEConversationReplayBoundaryMsg{Conversation: true, ConversationID: m.conversationID, Supported: false, StatusCode: 200})
	m = next.(Model)
	next, _ = m.Update(ConversationHistoryErrorMsg{ConversationID: m.conversationID, Err: "legacy GET unavailable"})
	m = next.(Model)
	if m.conversationReplayLegacyHistoryLoading {
		t.Fatal("history failure left legacy loading state stuck")
	}
	next, _ = m.Update(SSEEventMsg{Conversation: true, ConversationID: m.conversationID, ID: "future:1", RunID: "future", EventType: "assistant.message", Raw: []byte(`{"content":"LIVE_AFTER_HISTORY_FAILURE"}`)})
	m = next.(Model)
	if got := m.View(); !strings.Contains(got, "LIVE_AFTER_HISTORY_FAILURE") {
		t.Fatalf("live delivery remained blocked after history failure: %s", got)
	}
}

// A selected session uses the same atomic supported-server path as an initial
// resume: the snapshot renders once, while a future semantic turn with equal
// text still renders as a second distinct event. No GET-first history fallback
// is involved in this supported path.
func TestIssue1246_SelectedSessionSupportedBoundaryRendersSnapshotOnceThenFutureSameText(t *testing.T) {
	m := selectedSessionReplayBoundaryModel(t)
	next, _ := m.Update(SSEEventMsg{
		Conversation:   true,
		ConversationID: "conv-selected-boundary",
		EventType:      "conversation.replay.completed",
		Raw:            []byte(`{"messages":[{"role":"assistant","content":"SAME_SELECTED_TEXT"}],"last_event_id":"snapshot:1"}`),
	})
	m = next.(Model)
	next, _ = m.Update(SSEEventMsg{
		Conversation:   true,
		ConversationID: "conv-selected-boundary",
		ID:             "future:2",
		RunID:          "future-run",
		EventType:      "assistant.message",
		Raw:            []byte(`{"content":"SAME_SELECTED_TEXT"}`),
	})
	m = next.(Model)
	if got := strings.Count(m.View(), "SAME_SELECTED_TEXT"); got != 2 {
		t.Fatalf("selected supported boundary rendered %d copies, want snapshot plus future = 2", got)
	}
}

// An unsupported selected-session boundary cancels into the existing legacy
// GET path. Without a durable cursor it must remain snapshot-only: history is
// rendered once and no empty-cursor stream restart can replay it.
func TestIssue1246_SelectedSessionLegacyEmptyCursorStaysSnapshotOnly(t *testing.T) {
	m := selectedSessionReplayBoundaryModel(t)
	next, fallbackCmd := m.Update(SSEConversationReplayBoundaryMsg{
		Conversation:   true,
		ConversationID: "conv-selected-boundary",
		Supported:      false,
		StatusCode:     200,
	})
	m = next.(Model)
	if fallbackCmd == nil || !m.conversationReplayLegacyHistoryLoading {
		t.Fatal("unsupported selected-session boundary did not enter legacy history fallback")
	}
	next, restartCmd := m.Update(ConversationHistoryMsg{
		ConversationID: "conv-selected-boundary",
		Messages:       []ConversationMessage{{Role: "assistant", Content: "LEGACY_SNAPSHOT_ONCE"}},
		LastEventID:    "",
	})
	m = next.(Model)
	if restartCmd == nil {
		t.Fatal("legacy snapshot-only result must report its status")
	}
	if m.conversationReplayLegacyHistoryLoading {
		t.Fatal("legacy empty cursor left loading state set")
	}
	if got := strings.Count(m.View(), "LEGACY_SNAPSHOT_ONCE"); got != 1 {
		t.Fatalf("legacy empty cursor rendered %d historical copies, want 1", got)
	}
	if !strings.Contains(m.StatusMsg(), "snapshot-only mode") {
		t.Fatalf("legacy empty cursor did not retain snapshot-only status: %q", m.StatusMsg())
	}
}
