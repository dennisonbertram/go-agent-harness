package tui_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	tui "go-agent-harness/cmd/harnesscli/tui"
)

// TestIdleConversationStreamRendersExternalContinuation proves that a TUI
// resumed into an otherwise idle selected conversation keeps consuming the
// server's durable conversation event journal. A scheduled run can therefore
// become visible without requiring a new user prompt to open a run-only stream.
func TestIdleConversationStreamRendersExternalContinuation(t *testing.T) {
	const conversationID = "conversation-idle-stream"
	const marker = "CRON_TUI_FIRED"

	streamOpened := make(chan struct{})
	releaseEvent := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/v1/conversations/"+conversationID+"/events"; got != want {
			http.NotFound(w, r)
			return
		}
		if got, want := r.Header.Get("Authorization"), "Bearer idle-stream-key"; got != want {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		close(streamOpened)
		<-releaseEvent
		fmt.Fprintf(w, "id: external-run:7\nevent: message\ndata: {\"type\":\"assistant.message\",\"payload\":{\"content\":%q}}\n\n", marker)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-r.Context().Done()
	}))
	defer func() {
		srv.CloseClientConnections()
		srv.Close()
	}()

	cfg := tui.DefaultTUIConfig()
	cfg.BaseURL = srv.URL
	cfg.APIKey = "idle-stream-key"
	cfg.ResumeConversationID = conversationID
	m := tui.New(cfg)
	m2, initCmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = m2.(tui.Model)

	// The first ready window is the selected-conversation lifecycle boundary.
	// There is no active
	// RunStartedMsg in this test: the next assistant event belongs to a later
	// external scheduled run.
	go func() {
		<-streamOpened
		close(releaseEvent)
	}()
	m = driveModel(t, m, initCmd, 2*time.Second, func(model tui.Model, _ tea.Msg) bool {
		return strings.Contains(model.View(), marker)
	})

	if got := m.ConversationID(); got != conversationID {
		t.Fatalf("ConversationID() = %q, want %q", got, conversationID)
	}
	if got := m.View(); !strings.Contains(got, marker) {
		t.Fatalf("idle selected-conversation transcript omitted %q:\n%s", marker, got)
	}
}

func TestIdleConversationStreamReconnectsWithCursor(t *testing.T) {
	const conversationID = "conversation-reconnect"
	const first = "FIRST_IDLE_CONTINUATION"
	const second = "SECOND_IDLE_CONTINUATION"

	var mu sync.Mutex
	connections := 0
	var reconnectCursor string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/v1/conversations/"+conversationID+"/events"; got != want {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		connections++
		connection := connections
		if connection == 2 {
			reconnectCursor = r.Header.Get("Last-Event-ID")
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		if connection == 1 {
			fmt.Fprintf(w, "id: external-run:1\nevent: message\ndata: {\"type\":\"assistant.message\",\"payload\":{\"content\":%q}}\n\n", first)
			fmt.Fprint(w, "id: external-run:terminal\nevent: message\ndata: {\"type\":\"run.completed\",\"payload\":{}}\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return // unexpected EOF forces the conversation bridge to reconnect.
		}
		fmt.Fprintf(w, "id: external-run:2\nevent: message\ndata: {\"type\":\"assistant.message\",\"payload\":{\"content\":%q}}\n\n", second)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-r.Context().Done()
	}))
	defer func() {
		srv.CloseClientConnections()
		srv.Close()
	}()

	cfg := tui.DefaultTUIConfig()
	cfg.BaseURL, cfg.ResumeConversationID = srv.URL, conversationID
	m := tui.New(cfg)
	m2, initCmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = m2.(tui.Model)
	m = driveModel(t, m, initCmd, 4*time.Second, func(model tui.Model, _ tea.Msg) bool {
		return strings.Contains(model.View(), second)
	})
	if got := m.View(); !strings.Contains(got, first) || !strings.Contains(got, second) {
		t.Fatalf("reconnected transcript = %q, want both continuation markers", got)
	}
	mu.Lock()
	gotCursor := reconnectCursor
	mu.Unlock()
	if got, want := gotCursor, "external-run:terminal"; got != want {
		t.Fatalf("Last-Event-ID = %q, want %q", got, want)
	}
}

// A resumed conversation must finish rendering its durable message history
// before it subscribes, then use the durable event cursor on reconnect. This
// is the hand-off which keeps the idle subscription from racing history and
// replaying the event that it has already rendered.
func TestResumedConversationHistoryThenReplayCursor(t *testing.T) {
	const conversationID = "conversation-history-cursor"
	const historyMarker = "HISTORY_BEFORE_STREAM"
	const eventMarker = "EVENT_AFTER_HISTORY"

	var mu sync.Mutex
	historyServed := false
	connections := 0
	var reconnectCursor string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/conversations/" + conversationID + "/messages":
			mu.Lock()
			historyServed = true
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"messages":[{"role":"assistant","content":%q}]}`, historyMarker)
		case "/v1/conversations/" + conversationID + "/events":
			mu.Lock()
			if !historyServed {
				mu.Unlock()
				http.Error(w, "events opened before history", http.StatusConflict)
				return
			}
			connections++
			connection := connections
			if connection == 2 {
				reconnectCursor = r.Header.Get("Last-Event-ID")
			}
			mu.Unlock()
			w.Header().Set("Content-Type", "text/event-stream")
			if connection == 1 {
				fmt.Fprintf(w, "id: durable:1\nevent: message\ndata: {\"type\":\"assistant.message\",\"payload\":{\"content\":%q}}\n\n", eventMarker)
				w.(http.Flusher).Flush()
				return
			}
			// A replay of the last durable event must not render twice.
			fmt.Fprintf(w, "id: durable:1\nevent: message\ndata: {\"type\":\"assistant.message\",\"payload\":{\"content\":%q}}\n\n", eventMarker)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	defer func() { srv.CloseClientConnections(); srv.Close() }()

	cfg := tui.DefaultTUIConfig()
	cfg.BaseURL, cfg.ResumeConversationID = srv.URL, conversationID
	m := tui.New(cfg)
	m2, initCmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = m2.(tui.Model)
	m = driveModel(t, m, initCmd, 8*time.Second, func(model tui.Model, _ tea.Msg) bool {
		mu.Lock()
		connected := connections >= 2
		mu.Unlock()
		return connected && strings.Contains(model.View(), eventMarker)
	})
	if got := strings.Count(m.View(), eventMarker); got != 1 {
		t.Fatalf("replayed selected-conversation event rendered %d times, want 1:\\n%s", got, m.View())
	}
	mu.Lock()
	cursor := reconnectCursor
	mu.Unlock()
	if got, want := cursor, "durable:1"; got != want {
		t.Fatalf("reconnect Last-Event-ID = %q, want %q", got, want)
	}
}

// The messages endpoint has no event-ID cursor. On a resumed conversation the
// first SSE page can therefore contain the same durable assistant.message
// already rendered from history; it must be reconciled rather than producing
// a second bubble.
func TestResumedConversationSuppressesPreexistingAssistantReplay(t *testing.T) {
	const conversationID = "conversation-history-replay"
	const marker = "PREEXISTING_ASSISTANT_ONCE"
	streamOpened := make(chan struct{})
	replayed := make(chan struct{})
	var mu sync.Mutex
	connections := 0
	var cursor string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/conversations/" + conversationID + "/messages":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"messages":[{"role":"assistant","content":%q}],"last_event_id":"prior-run:3"}`, marker)
		case "/v1/conversations/" + conversationID + "/events":
			mu.Lock()
			connections++
			connection := connections
			if connection == 2 {
				cursor = r.Header.Get("Last-Event-ID")
			}
			mu.Unlock()
			if connection == 1 {
				close(streamOpened)
			}
			if connection == 1 && r.Header.Get("Last-Event-ID") != "prior-run:3" {
				http.Error(w, "initial stream missed snapshot watermark", http.StatusConflict)
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			if connection == 2 {
				w.(http.Flusher).Flush()
				<-r.Context().Done()
				return
			}
			fmt.Fprintf(w, "id: later-run:1\nevent: message\ndata: {\"type\":\"assistant.message\",\"payload\":{\"content\":%q}}\n\n", marker)
			fmt.Fprint(w, "id: later-run:2\nevent: message\ndata: {\"type\":\"run.completed\",\"run_id\":\"later-run\",\"payload\":{}}\n\n")
			w.(http.Flusher).Flush()
			close(replayed)
			return // force EOF/reconnect from the exact final ID.
		default:
			http.NotFound(w, r)
		}
	}))
	defer func() { srv.CloseClientConnections(); srv.Close() }()

	cfg := tui.DefaultTUIConfig()
	cfg.BaseURL, cfg.ResumeConversationID = srv.URL, conversationID
	m := tui.New(cfg)
	m2, initCmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = m2.(tui.Model)
	m = driveModel(t, m, initCmd, 6*time.Second, func(model tui.Model, _ tea.Msg) bool {
		select {
		case <-streamOpened:
			select {
			case <-replayed:
				mu.Lock()
				connected := connections >= 2
				mu.Unlock()
				return connected
			default:
				return false
			}
		default:
			return false
		}
	})
	markerEntries := 0
	for _, entry := range m.Transcript() {
		if entry.Content == marker {
			markerEntries++
		}
	}
	if markerEntries != 2 {
		t.Fatalf("history plus distinct same-content event created %d transcript entries, want 2", markerEntries)
	}
	mu.Lock()
	gotCursor := cursor
	mu.Unlock()
	if got, want := gotCursor, "later-run:2"; got != want {
		t.Fatalf("reconnect cursor = %q, want %q", got, want)
	}
}

func TestConversationTerminalCannotFinalizeActiveRun(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	m2, _ := m.Update(tui.RunStartedMsg{RunID: "active-run"})
	m = m2.(tui.Model)
	m3, _ := m.Update(tui.SSEEventMsg{EventType: "assistant.message", ID: "active-run:1", Raw: []byte(`{"content":"ACTIVE_RUN_FINAL"}`)})
	m = m3.(tui.Model)
	// The selected conversation may observe the same terminal frame before the
	// run stream. It must not clear the active response or finalize it twice.
	m4, _ := m.Update(tui.SSEEventMsg{EventType: "run.completed", ID: "active-run:2", RunID: "active-run", Conversation: true, ConversationID: m.ConversationID(), Raw: []byte(`{}`)})
	m = m4.(tui.Model)
	if got := m.LastAssistantText(); got != "ACTIVE_RUN_FINAL" {
		t.Fatalf("conversation terminal cleared active assistant text: %q", got)
	}
	m5, _ := m.Update(tui.SSEDoneMsg{EventType: "run.completed"})
	m = m5.(tui.Model)
	if got := len(m.Transcript()); got != 1 {
		t.Fatalf("active run transcript entries = %d, want 1", got)
	}
	if got := m.Transcript()[0].Content; got != "ACTIVE_RUN_FINAL" {
		t.Fatalf("active run transcript = %q, want final response", got)
	}
	// The reverse arrival order is just as important: the conversation copy
	// must not reopen or reset the response that the run stream just finalized.
	m6, _ := m.Update(tui.SSEEventMsg{EventType: "run.completed", ID: "active-run:3", RunID: "active-run", Conversation: true, ConversationID: m.ConversationID(), Raw: []byte(`{}`)})
	m = m6.(tui.Model)
	if got := len(m.Transcript()); got != 1 {
		t.Fatalf("late conversation terminal changed active transcript to %d entries", got)
	}
}

func TestLocalTerminalThenExternalAssistantMessageRendersBothTurns(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = m2.(tui.Model)
	m2, _ = m.Update(tui.RunStartedMsg{RunID: "local-run"})
	m = m2.(tui.Model)
	m2, _ = m.Update(tui.SSEEventMsg{ID: "local-run:1", RunID: "local-run", EventType: "assistant.message", Raw: []byte(`{"content":"LOCAL_FINAL"}`)})
	m = m2.(tui.Model)
	m2, _ = m.Update(tui.SSEDoneMsg{EventType: "run.completed"})
	m = m2.(tui.Model)
	m2, _ = m.Update(tui.SSEEventMsg{Conversation: true, ConversationID: m.ConversationID(), ID: "external-run:1", RunID: "external-run", EventType: "assistant.message", Raw: []byte(`{"content":"EXTERNAL_FINAL"}`)})
	m = m2.(tui.Model)
	m2, _ = m.Update(tui.SSEEventMsg{Conversation: true, ConversationID: m.ConversationID(), ID: "external-run:2", RunID: "external-run", EventType: "run.completed", Raw: []byte(`{}`)})
	m = m2.(tui.Model)
	for _, want := range []string{"LOCAL_FINAL", "EXTERNAL_FINAL"} {
		if got := strings.Count(m.View(), want); got != 1 {
			t.Fatalf("%s rendered %d times: %s", want, got, m.View())
		}
	}
	if got := len(m.Transcript()); got != 2 {
		t.Fatalf("transcript entries = %d, want 2", got)
	}
}

func TestConversationSSEEventIDDedupeIsBounded(t *testing.T) {
	m := tui.New(tui.DefaultTUIConfig())
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = m2.(tui.Model)
	for i := 0; i < 4097; i++ {
		m2, _ = m.Update(tui.SSEEventMsg{Conversation: true, ID: fmt.Sprintf("run:%d", i), EventType: "assistant.message", Raw: []byte(`{"content":""}`)})
		m = m2.(tui.Model)
	}
	// The newest ID remains in the cap and is suppressed even when it later
	// carries content; the oldest was evicted and is accepted again.
	m2, _ = m.Update(tui.SSEEventMsg{Conversation: true, ID: "run:4096", EventType: "assistant.message", Raw: []byte(`{"content":"NEWEST"}`)})
	m = m2.(tui.Model)
	m2, _ = m.Update(tui.SSEEventMsg{Conversation: true, ID: "run:0", EventType: "assistant.message", Raw: []byte(`{"content":"EVICTED"}`)})
	m = m2.(tui.Model)
	if strings.Contains(m.View(), "NEWEST") {
		t.Fatal("newest duplicate rendered after bounded dedupe")
	}
	if got := strings.Count(m.View(), "EVICTED"); got != 1 {
		t.Fatalf("evicted oldest rendered %d times, want 1", got)
	}
}

func TestConversationAndRunStreamsDeduplicateSharedEvent(t *testing.T) {
	const conversationID = "conversation-overlap"
	const marker = "ONE_RENDER_FOR_OVERLAP"
	var mu sync.Mutex
	opened := 0
	gate := make(chan struct{})
	var once sync.Once
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tail string
		switch r.URL.Path {
		case "/v1/conversations/" + conversationID + "/events":
			tail = "CONVERSATION_STREAM_DRAINED"
		case "/v1/runs/" + conversationID + "/events":
			tail = "RUN_STREAM_DRAINED"
		default:
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		opened++
		if opened == 2 {
			once.Do(func() { close(gate) })
		}
		mu.Unlock()
		<-gate
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "id: shared-run:9\nevent: message\ndata: {\"type\":\"assistant.message\",\"payload\":{\"content\":%q}}\n\n", marker)
		fmt.Fprintf(w, "id: %s\nevent: message\ndata: {\"type\":\"steering.received\",\"payload\":{\"message\":%q}}\n\n", tail, tail)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-r.Context().Done()
	}))
	defer func() {
		srv.CloseClientConnections()
		srv.Close()
	}()

	cfg := tui.DefaultTUIConfig()
	cfg.BaseURL = srv.URL
	cfg.ResumeConversationID = conversationID
	m := tui.New(cfg)
	m2, initCmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = m2.(tui.Model)
	history := initCmd()
	m3, startCmd := m.Update(history)
	m = m3.(tui.Model)
	conversationStart := startCmd()
	if batch, ok := conversationStart.(tea.BatchMsg); ok {
		conversationStart = batch[len(batch)-1]() // status first, stream start last
	}
	m4, conversationPoll := m.Update(conversationStart)
	m = m4.(tui.Model)
	m5, cmd := m.Update(tui.RunStartedMsg{RunID: conversationID})
	m = m5.(tui.Model)
	m = driveModel(t, m, tea.Batch(conversationPoll, cmd), 3*time.Second, func(model tui.Model, _ tea.Msg) bool {
		return strings.Contains(model.View(), "CONVERSATION_STREAM_DRAINED") && strings.Contains(model.View(), "RUN_STREAM_DRAINED")
	})
	if got := strings.Count(m.View(), marker); got != 1 {
		t.Fatalf("overlapping run and conversation feeds rendered marker %d times, want 1:\n%s", got, m.View())
	}
}

func TestConversationStreamSwitchCancelsOldSelection(t *testing.T) {
	const oldID = "conversation-old"
	const newID = "conversation-new"
	oldOpened := make(chan struct{})
	oldCancelled := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		switch r.URL.Path {
		case "/v1/conversations/" + oldID + "/events":
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			close(oldOpened)
			<-r.Context().Done()
			close(oldCancelled)
		case "/v1/conversations/" + newID + "/events":
			fmt.Fprint(w, "id: new-run:1\nevent: message\ndata: {\"type\":\"assistant.message\",\"payload\":{\"content\":\"NEW_SELECTION_ONLY\"}}\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	defer func() {
		srv.CloseClientConnections()
		srv.Close()
	}()

	cfg := tui.DefaultTUIConfig()
	cfg.BaseURL, cfg.ResumeConversationID = srv.URL, oldID
	m := tui.New(cfg)
	m2, initCmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = m2.(tui.Model)
	// The initial start command is enough to establish the old subscription;
	// no event is needed before we exercise the switch boundary.
	history := initCmd()
	m3, startCmd := m.Update(history)
	m = m3.(tui.Model)
	start := startCmd()
	if batch, ok := start.(tea.BatchMsg); ok {
		start = batch[len(batch)-1]() // status first, stream start last
	}
	m4, _ := m.Update(start)
	m = m4.(tui.Model)
	select {
	case <-oldOpened:
	case <-time.After(time.Second):
		t.Fatal("old selected-conversation SSE request did not open")
	}
	m5, cmd := m.Update(tui.SessionPickerSelectedMsg{SessionID: newID})
	m = m5.(tui.Model)
	m = driveModel(t, m, cmd, 3*time.Second, func(model tui.Model, _ tea.Msg) bool {
		return strings.Contains(model.View(), "NEW_SELECTION_ONLY")
	})
	select {
	case <-oldCancelled:
	case <-time.After(time.Second):
		t.Fatal("old selected-conversation SSE request was not canceled on switch")
	}
	if got := m.View(); strings.Contains(got, "conversation-old") {
		t.Fatalf("switched transcript leaked old selection: %s", got)
	}
}
