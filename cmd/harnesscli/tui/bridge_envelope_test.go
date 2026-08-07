package tui

import "testing"

// Mirrors the JSON envelope emitted by internal/server.writeSSE. The run ID
// lives at the top level, not inside payload; losing it makes a conversation
// terminal indistinguishable from an unrelated terminal event.
func TestDecodeSSEPreservesTopLevelRunID(t *testing.T) {
	msg := decodeSSE("run.completed", `{"id":"active-run:2","run_id":"active-run","type":"run.completed","payload":{}}`, "active-run:2", true)
	event, ok := msg.(SSEEventMsg)
	if !ok {
		t.Fatalf("decodeSSE returned %T, want SSEEventMsg", msg)
	}
	if got, want := event.RunID, "active-run"; got != want {
		t.Fatalf("RunID = %q, want %q", got, want)
	}
}

func TestDecodeSSETerminalPreservesTopLevelRunID(t *testing.T) {
	msg := decodeSSE("run.completed", `{"id":"active-run:3","run_id":"active-run","type":"run.completed","payload":{}}`, "active-run:3", false)
	done, ok := msg.(SSEDoneMsg)
	if !ok {
		t.Fatalf("decodeSSE returned %T, want SSEDoneMsg", msg)
	}
	if got, want := done.RunID, "active-run"; got != want {
		t.Fatalf("terminal RunID = %q, want %q", got, want)
	}
}
