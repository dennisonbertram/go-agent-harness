package harnessmcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// sseServer emits n events with incrementing IDs, then optionally holds the
// connection open like a live run would.
func sseServer(t *testing.T, n int, hold bool, seenLastEventID *string, mu *sync.Mutex) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seenLastEventID != nil {
			mu.Lock()
			*seenLastEventID = r.Header.Get("Last-Event-ID")
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "text/event-stream")
		f, _ := w.(http.Flusher)
		for i := 1; i <= n; i++ {
			fmt.Fprintf(w, "id: %d\nevent: step\ndata: {\"n\":%d}\n\n", i, i)
			if f != nil {
				f.Flush()
			}
		}
		if hold {
			<-r.Context().Done()
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestTailRunEventsReturnsEventsAndCursor — polling must yield events and a
// cursor to continue from (issue #1323).
func TestTailRunEventsReturnsEventsAndCursor(t *testing.T) {
	var mu sync.Mutex
	srv := sseServer(t, 3, false, nil, &mu)

	events, cursor, err := NewHarnessClient(srv.URL).
		TailRunEvents(context.Background(), "run-1", "", 100, time.Second)
	if err != nil {
		t.Fatalf("TailRunEvents: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}
	if cursor != "3" {
		t.Errorf("cursor = %q, want the last event ID 3", cursor)
	}
	if events[0].Type != "step" {
		t.Errorf("event type = %q, want step", events[0].Type)
	}
}

// TestTailRunEventsSendsCursorAsLastEventID is the false-positive control: a tool
// that re-read from the start would also return events, but would replay them.
func TestTailRunEventsSendsCursorAsLastEventID(t *testing.T) {
	var mu sync.Mutex
	var seen string
	srv := sseServer(t, 2, false, &seen, &mu)

	_, _, err := NewHarnessClient(srv.URL).
		TailRunEvents(context.Background(), "run-1", "7", 100, time.Second)
	if err != nil {
		t.Fatalf("TailRunEvents: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if seen != "7" {
		t.Errorf("server saw Last-Event-ID %q, want 7 — polling would replay without it", seen)
	}
}

// TestTailRunEventsReturnsPromptlyWhenQuiet — a live but silent run must not hold
// the tool call open past its window.
func TestTailRunEventsReturnsPromptlyWhenQuiet(t *testing.T) {
	var mu sync.Mutex
	srv := sseServer(t, 0, true, nil, &mu) // connects, sends nothing, holds

	start := time.Now()
	events, _, err := NewHarnessClient(srv.URL).
		TailRunEvents(context.Background(), "run-1", "", 100, 300*time.Millisecond)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("TailRunEvents: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("got %d events from a silent run, want 0", len(events))
	}
	if elapsed > 3*time.Second {
		t.Errorf("took %v; a quiet poll must return near its window", elapsed)
	}
}

// TestTailRunEventsRespectsMaxEvents keeps a busy run from returning unbounded output.
func TestTailRunEventsRespectsMaxEvents(t *testing.T) {
	var mu sync.Mutex
	srv := sseServer(t, 50, true, nil, &mu)

	events, cursor, err := NewHarnessClient(srv.URL).
		TailRunEvents(context.Background(), "run-1", "", 5, 2*time.Second)
	if err != nil {
		t.Fatalf("TailRunEvents: %v", err)
	}
	if len(events) != 5 {
		t.Fatalf("got %d events, want the 5 requested", len(events))
	}
	if cursor != "5" {
		t.Errorf("cursor = %q, want 5 so the next poll resumes correctly", cursor)
	}
}

// TestGetRunInputReturnsPendingQuestion — a delegate that asks must be answerable.
func TestGetRunInputReturnsPendingQuestion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"questions":[{"id":"q1","question":"Which database?"}]}`))
	}))
	defer srv.Close()

	h := newGetRunInputHandler(NewHarnessClient(srv.URL))
	res, err := h(context.Background(), json.RawMessage(`{"run_id":"r1"}`))
	if err != nil {
		t.Fatalf("get_run_input: %v", err)
	}
	if !strings.Contains(res.Content[0].Text, "Which database?") {
		t.Errorf("pending question not surfaced: %s", res.Content[0].Text)
	}
}

// TestGetRunInputWhenNonePending — the 409 case must read clearly, not opaquely.
func TestGetRunInputWhenNonePending(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":{"code":"no_pending_input","message":"run is not waiting for user input"}}`))
	}))
	defer srv.Close()

	h := newGetRunInputHandler(NewHarnessClient(srv.URL))
	res, _ := h(context.Background(), json.RawMessage(`{"run_id":"r1"}`))
	if !strings.Contains(res.Content[0].Text, "not waiting for user input") {
		t.Errorf("expected a clear not-waiting message, got: %s", res.Content[0].Text)
	}
}

// TestSubmitUserInputPostsAnswers — the answers must reach the server.
func TestSubmitUserInputPostsAnswers(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	h := newSubmitUserInputHandler(NewHarnessClient(srv.URL))
	res, err := h(context.Background(), json.RawMessage(`{"run_id":"r1","answers":{"q1":"postgres"}}`))
	if err != nil {
		t.Fatalf("submit_user_input: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content[0].Text)
	}
	answers, ok := body["answers"].(map[string]any)
	if !ok || answers["q1"] != "postgres" {
		t.Errorf("posted body = %v, want answers.q1=postgres", body)
	}
}

// TestRunProgressToolsHitTheirEndpoints covers the remaining progress signals.
func TestRunProgressToolsHitTheirEndpoints(t *testing.T) {
	for _, tc := range []struct {
		tool     string
		wantPath string
		build    func(*HarnessClient) ToolHandler
	}{
		{"get_run_todos", "/v1/runs/r1/todos", newGetRunTodosHandler},
		{"get_run_summary", "/v1/runs/r1/summary", newGetRunSummaryHandler},
		{"get_run_context", "/v1/runs/r1/context", newGetRunContextHandler},
		{"compact_run", "/v1/runs/r1/compact", newCompactRunHandler},
	} {
		t.Run(tc.tool, func(t *testing.T) {
			var gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer srv.Close()

			res, err := tc.build(NewHarnessClient(srv.URL))(context.Background(), json.RawMessage(`{"run_id":"r1"}`))
			if err != nil {
				t.Fatalf("%s: %v", tc.tool, err)
			}
			if res.IsError {
				t.Fatalf("%s errored: %s", tc.tool, res.Content[0].Text)
			}
			if gotPath != tc.wantPath {
				t.Errorf("%s hit %q, want %q", tc.tool, gotPath, tc.wantPath)
			}
		})
	}
}
