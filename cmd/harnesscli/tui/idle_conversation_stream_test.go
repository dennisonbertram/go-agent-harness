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
	var onceStreamOpened sync.Once
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
		w.Header().Set("X-Harness-Conversation-Replay-Boundary", "snapshot")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "event: conversation.replay.completed\ndata: {\"type\":\"conversation.replay.completed\",\"payload\":{\"messages\":[],\"last_event_id\":\"\"}}\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		onceStreamOpened.Do(func() { close(streamOpened) })
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
		if r.Header.Get("X-Harness-Conversation-Replay-Boundary") == "snapshot" {
			w.Header().Set("X-Harness-Conversation-Replay-Boundary", "snapshot")
			fmt.Fprint(w, "event: conversation.replay.completed\ndata: {\"type\":\"conversation.replay.completed\",\"payload\":{\"messages\":[],\"last_event_id\":\"\"}}\n\n")
		}
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
	const historicEvent = "HISTORIC_REPLAY_SUPPRESSED"

	var mu sync.Mutex
	connections := 0
	var reconnectCursor string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/conversations/" + conversationID + "/events":
			mu.Lock()
			connections++
			connection := connections
			if connection == 2 {
				reconnectCursor = r.Header.Get("Last-Event-ID")
			}
			mu.Unlock()
			w.Header().Set("Content-Type", "text/event-stream")
			if connection == 1 {
				if got, want := r.Header.Get("X-Harness-Conversation-Replay-Boundary"), "snapshot"; got != want {
					http.Error(w, "missing replay boundary opt-in", http.StatusConflict)
					return
				}
				w.Header().Set("X-Harness-Conversation-Replay-Boundary", "snapshot")
				fmt.Fprintf(w, "id: durable:1\nevent: message\ndata: {\"type\":\"assistant.message\",\"payload\":{\"content\":%q}}\n\n", historicEvent)
				fmt.Fprintf(w, "event: conversation.replay.completed\ndata: {\"type\":\"conversation.replay.completed\",\"payload\":{\"messages\":[{\"role\":\"assistant\",\"content\":%q}],\"last_event_id\":\"durable:1\"}}\n\n", historyMarker)
				w.(http.Flusher).Flush()
				return
			}
			fmt.Fprintf(w, "id: future:2\nevent: message\ndata: {\"type\":\"assistant.message\",\"run_id\":\"future\",\"payload\":{\"content\":%q}}\n\n", eventMarker)
			fmt.Fprint(w, "id: future:3\nevent: message\ndata: {\"type\":\"run.completed\",\"run_id\":\"future\",\"payload\":{}}\n\n")
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/conversations/" + conversationID + "/events":
			mu.Lock()
			connections++
			connection := connections
			mu.Unlock()
			if connection == 1 {
				close(streamOpened)
			}
			if connection == 1 && r.Header.Get("X-Harness-Conversation-Replay-Boundary") != "snapshot" {
				http.Error(w, "initial stream missed replay-boundary opt-in", http.StatusConflict)
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("X-Harness-Conversation-Replay-Boundary", "snapshot")
			if connection == 2 {
				w.(http.Flusher).Flush()
				<-r.Context().Done()
				return
			}
			fmt.Fprintf(w, "event: conversation.replay.completed\ndata: {\"type\":\"conversation.replay.completed\",\"payload\":{\"messages\":[{\"role\":\"assistant\",\"content\":%q}],\"last_event_id\":\"prior-run:3\"}}\n\n", marker)
			w.(http.Flusher).Flush()
			close(replayed)
			return // force reconnect from the atomic snapshot cursor.
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
				return strings.Contains(model.View(), marker)
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
	if markerEntries != 1 {
		t.Fatalf("atomic snapshot rendered %d duplicate transcript entries, want 1", markerEntries)
	}
}

// A selected conversation may have no safe durable snapshot cursor after a
// restart or overlapping history. In that case empty-cursor SSE replay is
// correct, but rendering history before subscribing duplicates the historic
// assistant response. The opt-in replay boundary registers SSE first, lets
// the client discard the known historical replay, and then has it fetch the
// durable snapshot. A later distinct event (the shape produced by a cron or
// callback continuation) must remain visible exactly once.
func TestResumedConversationEmptyCursorReplayBoundaryRendersHistoricAndFutureOnce(t *testing.T) {
	const conversationID = "conversation-empty-cursor-boundary"
	const historic = "HISTORIC_EMPTY_CURSOR_ONCE"
	const future = "SCHEDULED_FUTURE_CONTINUATION_ONCE"

	markerSeen := make(chan struct{})
	futureSent := make(chan struct{})
	var onceMarker sync.Once
	var mu sync.Mutex
	var initialHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/conversations/" + conversationID + "/events":
			mu.Lock()
			initialHeader = r.Header.Get("X-Harness-Conversation-Replay-Boundary")
			mu.Unlock()
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("X-Harness-Conversation-Replay-Boundary", "snapshot")
			fmt.Fprintf(w, "id: historic-run:1\nevent: message\ndata: {\"type\":\"assistant.message\",\"payload\":{\"content\":%q}}\n\n", historic)
			fmt.Fprintf(w, "event: conversation.replay.completed\ndata: {\"type\":\"conversation.replay.completed\",\"payload\":{\"messages\":[{\"role\":\"assistant\",\"content\":%q}],\"last_event_id\":\""+"historic-run:1"+"\"}}\n\n", historic)
			w.(http.Flusher).Flush()
			onceMarker.Do(func() { close(markerSeen) })
			fmt.Fprintf(w, "id: scheduled-run:1\nevent: message\ndata: {\"type\":\"assistant.message\",\"run_id\":\"scheduled-run\",\"payload\":{\"content\":%q}}\n\n", future)
			w.(http.Flusher).Flush()
			close(futureSent)
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
	m = driveModel(t, m, initCmd, 6*time.Second, func(model tui.Model, _ tea.Msg) bool {
		select {
		case <-markerSeen:
			select {
			case <-futureSent:
				return strings.Contains(model.View(), historic) && strings.Contains(model.View(), future)
			default:
				return false
			}
		default:
			return false
		}
	})

	mu.Lock()
	header := initialHeader
	mu.Unlock()
	if got, want := header, "snapshot"; got != want {
		t.Fatalf("initial replay boundary header = %q, want %q", got, want)
	}
	entries := map[string]int{}
	for _, entry := range m.Transcript() {
		entries[entry.Content]++
	}
	if got, want := entries[historic], 1; got != want {
		t.Fatalf("historic transcript entries = %d, want %d; transcript=%+v", got, want, m.Transcript())
	}
	if got := m.View(); !strings.Contains(got, future) {
		t.Fatalf("post-boundary scheduled continuation was not rendered: %s", got)
	}
}

// Regression for #1249's causal handoff: the replay boundary itself, not a
// later /messages GET, owns the selected-conversation snapshot. The server
// has queued two historical events (including the most recent one) before it
// writes the boundary snapshot. Both must be suppressed as pre-marker replay,
// then the atomic marker snapshot rendered once, and a later live event
// rendered normally. A client that fetches history after the marker has a
// snapshot/live race and fails this test by touching /messages at all.
func TestResumedConversationReplayBoundarySnapshotIncludesQueuedFuture(t *testing.T) {
	const conversationID = "conversation-boundary-atomic-snapshot"
	const historic = "BOUNDARY_HISTORIC_ONCE"
	const queuedFuture = "BOUNDARY_QUEUED_FUTURE_ONCE"
	const liveFuture = "BOUNDARY_LIVE_FUTURE_ONCE"

	// The server must not publish the post-boundary live event until the model
	// has reduced the atomic replay snapshot. Keeping this hand-off explicit
	// prevents the fixture from racing the client-side replay marker reducer.
	releaseLive := make(chan struct{})
	var onceRelease sync.Once
	var mu sync.Mutex
	messagesRequests := 0
	stage := "server not opened"
	snapshotHistoric := 0
	snapshotQueued := 0
	decodedLive := 0
	reducedLive := 0
	setStage := func(next string) {
		mu.Lock()
		stage = next
		mu.Unlock()
	}
	defer func() {
		if t.Failed() {
			mu.Lock()
			gotStage := stage
			mu.Unlock()
			t.Logf("replay-boundary fixture stage at failure: %s; snapshot historic=%d queued=%d; decoded live:3=%d reduced live:3=%d", gotStage, snapshotHistoric, snapshotQueued, decodedLive, reducedLive)
		}
	}()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/conversations/" + conversationID + "/messages":
			mu.Lock()
			messagesRequests++
			mu.Unlock()
			http.Error(w, "opt-in boundary must not fetch history", http.StatusConflict)
		case "/v1/conversations/" + conversationID + "/events":
			setStage("replay stream opened")
			if got, want := r.Header.Get("X-Harness-Conversation-Replay-Boundary"), "snapshot"; got != want {
				http.Error(w, "missing replay boundary opt-in", http.StatusConflict)
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("X-Harness-Conversation-Replay-Boundary", "snapshot")
			fmt.Fprintf(w, "id: historic:1\nevent: message\ndata: {\"type\":\"assistant.message\",\"payload\":{\"content\":%q}}\n\n", historic)
			fmt.Fprintf(w, "id: queued:2\nevent: message\ndata: {\"type\":\"assistant.message\",\"payload\":{\"content\":%q}}\n\n", queuedFuture)
			fmt.Fprintf(w, "event: conversation.replay.completed\ndata: {\"type\":\"conversation.replay.completed\",\"payload\":{\"messages\":[{\"role\":\"assistant\",\"content\":%q},{\"role\":\"assistant\",\"content\":%q}],\"last_event_id\":\"queued:2\"}}\n\n", historic, queuedFuture)
			w.(http.Flusher).Flush()
			setStage("replay marker flushed; waiting for model snapshot reduction")
			// A failing model assertion must still be able to cancel this local
			// handler. Otherwise httptest cleanup would hang behind the deliberate
			// fixture gate and conceal the original assertion failure.
			select {
			case <-releaseLive:
			case <-r.Context().Done():
				return
			}
			setStage("model reduced replay snapshot; publishing live event")
			fmt.Fprintf(w, "id: live:3\nevent: message\ndata: {\"type\":\"assistant.message\",\"run_id\":\"live\",\"payload\":{\"content\":%q}}\n\n", liveFuture)
			fmt.Fprint(w, "id: live:4\nevent: message\ndata: {\"type\":\"run.completed\",\"run_id\":\"live\",\"payload\":{}}\n\n")
			w.(http.Flusher).Flush()
			setStage("live event flushed")
			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	defer func() { srv.CloseClientConnections(); srv.Close() }()

	cfg := tui.DefaultTUIConfig()
	cfg.BaseURL, cfg.ResumeConversationID = srv.URL, conversationID
	m := tui.New(cfg)
	// Keep the viewport deliberately shorter than the atomic replay snapshot.
	// A completion predicate must therefore use transcript/reducer state rather
	// than require every historical entry to remain visible after auto-scroll.
	m2, initCmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 8})
	m = m2.(tui.Model)
	m = driveModel(t, m, initCmd, 6*time.Second, func(model tui.Model, msg tea.Msg) bool {
		entries := map[string]int{}
		for _, entry := range model.Transcript() {
			entries[entry.Content]++
		}
		snapshotHistoric = entries[historic]
		snapshotQueued = entries[queuedFuture]
		// Transcript is the durable model state. The short auto-scrolled viewport
		// deliberately cannot be used to prove both historical snapshot entries.
		if snapshotHistoric == 1 && snapshotQueued == 1 {
			onceRelease.Do(func() { close(releaseLive) })
		}
		event, ok := msg.(tui.SSEEventMsg)
		if !ok || event.ID != "live:3" || event.EventType != "assistant.message" || !strings.Contains(string(event.Raw), liveFuture) {
			return false
		}
		decodedLive++
		if strings.Contains(model.View(), liveFuture) {
			reducedLive++
			return true
		}
		return false
	})

	entries := map[string]int{}
	for _, entry := range m.Transcript() {
		entries[entry.Content]++
	}
	for _, want := range []string{historic, queuedFuture} {
		if got := entries[want]; got != 1 {
			t.Fatalf("transcript entries for %q = %d, want 1; transcript=%+v", want, got, m.Transcript())
		}
	}
	if got := m.View(); !strings.Contains(got, liveFuture) {
		t.Fatalf("post-boundary live event was not routed through the normal reducer: %s", got)
	}
	mu.Lock()
	gotMessagesRequests := messagesRequests
	mu.Unlock()
	if gotMessagesRequests != 0 {
		t.Fatalf("opt-in replay boundary fetched /messages %d times, want 0", gotMessagesRequests)
	}
}

// A legacy server may successfully return HTTP 200 yet omit the opt-in
// acknowledgement. The bridge-status message is then ahead of any decoded
// event in the same channel. The client must cancel that stream before making
// its GET-first fallback request; otherwise its queued assistant replay can
// leak into the transcript ahead of the history snapshot.
func TestResumedConversationLegacyHTTP200CancelsStreamBeforeHistoryGET(t *testing.T) {
	const conversationID = "conversation-legacy-http200"
	const historic = "LEGACY_HISTORY_ONCE"
	const leaked = "LEGACY_QUEUED_REPLAY_MUST_NOT_RENDER"
	const scheduled = "LEGACY_UNSAFE_SCHEDULED_MUST_NOT_FALSE_SUCCESS"

	streamCancelled := make(chan struct{})
	historyRequested := make(chan struct{})
	var onceCancelled sync.Once
	var onceHistory sync.Once
	var mu sync.Mutex
	connections := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/conversations/" + conversationID + "/messages":
			onceHistory.Do(func() { close(historyRequested) })
			select {
			case <-streamCancelled:
			case <-time.After(2 * time.Second):
				http.Error(w, "history requested before legacy stream cancellation", http.StatusConflict)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			// A genuine pre-marker server has no durable snapshot watermark.
			// The client must not treat this omitted field as an empty cursor it
			// can safely replay after rendering the GET snapshot.
			fmt.Fprintf(w, `{"messages":[{"role":"assistant","content":%q}]}`, historic)
		case "/v1/conversations/" + conversationID + "/events":
			mu.Lock()
			connections++
			connection := connections
			mu.Unlock()
			w.Header().Set("Content-Type", "text/event-stream")
			if connection == 1 {
				// Deliberately omit X-Harness-Conversation-Replay-Boundary even
				// though this is a successful SSE response.
				fmt.Fprintf(w, "id: legacy:1\nevent: message\ndata: {\"type\":\"assistant.message\",\"payload\":{\"content\":%q}}\n\n", leaked)
				w.(http.Flusher).Flush()
				<-r.Context().Done()
				onceCancelled.Do(func() { close(streamCancelled) })
				return
			}
			if connection == 2 {
				fmt.Fprintf(w, "id: legacy:1\nevent: message\ndata: {\"type\":\"assistant.message\",\"run_id\":\"legacy\",\"payload\":{\"content\":%q}}\n\n", historic)
				fmt.Fprintf(w, "id: scheduled:1\nevent: message\ndata: {\"type\":\"assistant.message\",\"run_id\":\"scheduled\",\"payload\":{\"content\":%q}}\n\n", scheduled)
			}
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
	m = driveModel(t, m, initCmd, 6*time.Second, func(model tui.Model, _ tea.Msg) bool {
		select {
		case <-historyRequested:
			return strings.Contains(model.View(), historic) && strings.Contains(strings.ToLower(model.StatusMsg()), "upgrade")
		default:
			return false
		}
	})
	if got := m.View(); strings.Contains(got, leaked) {
		t.Fatalf("legacy HTTP 200 stream leaked queued replay into transcript: %s", got)
	}
	if got := strings.Count(m.View(), historic); got != 1 {
		t.Fatalf("legacy missing-cursor replay rendered history %d times, want 1: %s", got, m.View())
	}
	if got := m.View(); strings.Contains(got, scheduled) {
		t.Fatalf("unsafe legacy continuation was rendered as a false success: %s", got)
	}
	mu.Lock()
	gotConnections := connections
	mu.Unlock()
	if gotConnections != 1 {
		t.Fatalf("missing-cursor legacy fallback opened %d event streams, want snapshot-only", gotConnections)
	}
}

func TestResumedConversationLegacyNonemptyCursorRestartsAndRendersContinuation(t *testing.T) {
	const conversationID = "conversation-legacy-cursor"
	const historic = "LEGACY_CURSOR_HISTORY"
	const future = "LEGACY_CURSOR_SCHEDULED_CONTINUATION"
	const cursor = "legacy:1"

	cancelled := make(chan struct{})
	var onceCancelled sync.Once
	var mu sync.Mutex
	connections := 0
	var resumedCursor string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/conversations/" + conversationID + "/messages":
			select {
			case <-cancelled:
			case <-time.After(2 * time.Second):
				http.Error(w, "GET before legacy cancellation", http.StatusConflict)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"messages":[{"role":"assistant","content":%q}],"last_event_id":%q}`, historic, cursor)
		case "/v1/conversations/" + conversationID + "/events":
			mu.Lock()
			connections++
			connection := connections
			if connection == 2 {
				resumedCursor = r.Header.Get("Last-Event-ID")
			}
			mu.Unlock()
			w.Header().Set("Content-Type", "text/event-stream")
			if connection == 1 {
				// HTTP 200 but no acknowledgement is an old server.
				w.(http.Flusher).Flush()
				<-r.Context().Done()
				onceCancelled.Do(func() { close(cancelled) })
				return
			}
			fmt.Fprintf(w, "id: scheduled:2\nevent: message\ndata: {\"type\":\"assistant.message\",\"run_id\":\"scheduled\",\"payload\":{\"content\":%q}}\n\n", future)
			fmt.Fprint(w, "id: scheduled:3\nevent: message\ndata: {\"type\":\"run.completed\",\"run_id\":\"scheduled\",\"payload\":{}}\n\n")
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
	m = driveModel(t, m, initCmd, 6*time.Second, func(model tui.Model, _ tea.Msg) bool {
		mu.Lock()
		gotCursor := resumedCursor
		mu.Unlock()
		return gotCursor == cursor && strings.Contains(model.View(), historic) && strings.Contains(model.View(), future)
	})
	mu.Lock()
	gotCursor := resumedCursor
	mu.Unlock()
	if got, want := gotCursor, cursor; got != want {
		t.Fatalf("legacy resumed Last-Event-ID = %q, want %q", got, want)
	}
	if got := m.View(); !strings.Contains(got, future) {
		t.Fatalf("legacy nonempty-cursor continuation missing: %s", got)
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
	conversationRequests := 0
	runRequests := 0
	gate := make(chan struct{})
	var once sync.Once
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tail string
		switch r.URL.Path {
		case "/v1/conversations/" + conversationID + "/events":
			tail = "CONVERSATION_STREAM_DRAINED"
			w.Header().Set("X-Harness-Conversation-Replay-Boundary", "snapshot")
			mu.Lock()
			conversationRequests++
			if conversationRequests == 1 && runRequests == 1 {
				once.Do(func() { close(gate) })
			}
			mu.Unlock()
		case "/v1/runs/" + conversationID + "/events":
			tail = "RUN_STREAM_DRAINED"
			mu.Lock()
			runRequests++
			if conversationRequests == 1 && runRequests == 1 {
				once.Do(func() { close(gate) })
			}
			mu.Unlock()
		default:
			http.NotFound(w, r)
			return
		}
		<-gate
		w.Header().Set("Content-Type", "text/event-stream")
		if r.URL.Path == "/v1/conversations/"+conversationID+"/events" {
			fmt.Fprint(w, "event: conversation.replay.completed\ndata: {\"type\":\"conversation.replay.completed\",\"payload\":{\"messages\":[],\"last_event_id\":\"\"}}\n\n")
		}
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
	// Install the selected-conversation bridge before starting the overlapping
	// run bridge. The real lifecycle owns one selected-conversation stream; a
	// concurrent test dispatch would manufacture a second start before the
	// first conversationSSEStartedMsg is reduced.
	started := initCmd()
	m3, conversationPollCmd := m.Update(started)
	m = m3.(tui.Model)
	m4, runCmd := m.Update(tui.RunStartedMsg{RunID: conversationID})
	m = m4.(tui.Model)
	// The fixture waits for the one conversation and one run request before
	// writing either stream, preserving the intended live-overlap exercise.
	m = driveModel(t, m, tea.Batch(conversationPollCmd, runCmd), 3*time.Second, func(model tui.Model, _ tea.Msg) bool {
		return strings.Contains(model.View(), "CONVERSATION_STREAM_DRAINED") && strings.Contains(model.View(), "RUN_STREAM_DRAINED")
	})
	if got := strings.Count(m.View(), marker); got != 1 {
		t.Fatalf("overlapping run and conversation feeds rendered marker %d times, want 1:\n%s", got, m.View())
	}
	mu.Lock()
	gotConversationRequests, gotRunRequests := conversationRequests, runRequests
	mu.Unlock()
	if gotConversationRequests != 1 || gotRunRequests != 1 {
		t.Fatalf("stream requests = conversation %d, run %d; want exactly one of each", gotConversationRequests, gotRunRequests)
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
