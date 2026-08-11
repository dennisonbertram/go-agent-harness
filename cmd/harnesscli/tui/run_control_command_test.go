package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func testRunControlModel(baseURL string) Model {
	cfg := DefaultTUIConfig()
	cfg.BaseURL = baseURL
	m := New(cfg)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return m2.(Model)
}

func lastCmd(t *testing.T, cmds []tea.Cmd) tea.Cmd {
	t.Helper()
	if len(cmds) == 0 {
		t.Fatal("expected command")
	}
	cmd := cmds[len(cmds)-1]
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}
	return cmd
}

func TestRunControl_RunsCommandFetchesAndDisplaysRuns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/runs" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"runs": []map[string]any{{
				"id":     "run_daily_1",
				"status": "completed",
				"model":  "gpt-4.1",
				"prompt": "fix terminal workflow",
			}},
		})
	}))
	defer srv.Close()

	m := testRunControlModel(srv.URL)
	cmds, quit := executeRunsCommand(&m, Command{Name: "runs"})
	if quit {
		t.Fatal("/runs must not quit")
	}
	msg := lastCmd(t, cmds)()
	m2, _ := m.Update(msg)
	m = m2.(Model)

	view := m.View()
	for _, want := range []string{"Runs", "run_daily_1", "completed", "gpt-4.1", "fix terminal"} {
		if !strings.Contains(view, want) {
			t.Fatalf("/runs view missing %q:\n%s", want, view)
		}
	}
	if !strings.Contains(m.StatusMsg(), "Loaded 1 run") {
		t.Fatalf("StatusMsg() = %q, want loaded run count", m.StatusMsg())
	}
}

func TestRunControl_CancelCommandCallsCancelEndpoint(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost || r.URL.Path != "/v1/runs/run_cancel_1/cancel" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"cancelling"}`))
	}))
	defer srv.Close()

	m := testRunControlModel(srv.URL)
	cmds, quit := executeCancelCommand(&m, Command{Name: "cancel", Args: []string{"run_cancel_1"}})
	if quit {
		t.Fatal("/cancel must not quit")
	}
	msg := lastCmd(t, cmds)()
	m2, _ := m.Update(msg)
	m = m2.(Model)

	if !called {
		t.Fatal("cancel endpoint was not called")
	}
	if !strings.Contains(m.StatusMsg(), "Run run_cancel_1 cancelling") {
		t.Fatalf("StatusMsg() = %q, want cancelling status", m.StatusMsg())
	}
}

func TestRunControl_ReplayCommandCallsReplayEndpoint(t *testing.T) {
	const replayedRunID = "run_replayed_1"
	const marker = "REPLAYED_RUN_STREAMED"
	streamOpened := make(chan struct{})
	streamClosed := make(chan struct{})
	var streamOnce sync.Once
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/runs/run_replay_1/replay":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"run_id":"run_replayed_1","status":"queued","replayed_from":"run_replay_1","conversation_id":"conv-1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/runs/"+replayedRunID+"/events":
			if got, want := r.Header.Get("Accept"), "text/event-stream"; got != want {
				t.Fatalf("SSE Accept = %q, want %q", got, want)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			streamOnce.Do(func() { close(streamOpened) })
			fmt.Fprintf(w, "id: %s:1\nevent: message\ndata: {\"type\":\"assistant.message\",\"payload\":{\"content\":%q}}\n\n", replayedRunID, marker)
			fmt.Fprintf(w, "id: %s:2\nevent: message\ndata: {\"type\":\"run.completed\",\"run_id\":%q,\"payload\":{}}\n\n", replayedRunID, replayedRunID)
			w.(http.Flusher).Flush()
			close(streamClosed) // Returning immediately makes terminal closure observable.
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer func() { srv.CloseClientConnections(); srv.Close() }()

	m := testRunControlModel(srv.URL)
	cmds, quit := executeReplayCommand(&m, Command{Name: "replay", Args: []string{"run_replay_1"}})
	if quit {
		t.Fatal("/replay must not quit")
	}
	msg := lastCmd(t, cmds)()
	m2, next := m.Update(msg)
	m = m2.(Model)

	if m.RunID != replayedRunID || !m.runActive {
		t.Fatalf("/replay did not start returned run: id=%q active=%v", m.RunID, m.runActive)
	}
	first := runControlAwaitSSE(t, next, 2*time.Second)
	m2, next = m.Update(first)
	m = m2.(Model)
	if !strings.Contains(m.View(), marker) {
		t.Fatalf("replayed run stream omitted %q:\n%s", marker, m.View())
	}
	terminal := runControlAwaitSSE(t, next, 2*time.Second)
	if done, ok := terminal.(SSEDoneMsg); !ok || done.EventType != "run.completed" {
		t.Fatalf("terminal stream message = %#v, want run.completed SSEDoneMsg", terminal)
	}
	m2, _ = m.Update(terminal)
	m = m2.(Model)
	if m.RunActive() {
		t.Fatal("terminal replay stream must mark returned run inactive")
	}
	select {
	case <-streamOpened:
	case <-time.After(2 * time.Second):
		t.Fatal("returned replay run SSE request was not observed")
	}
	select {
	case <-streamClosed:
	case <-time.After(2 * time.Second):
		t.Fatal("replayed SSE handler did not close after terminal event")
	}
}

func TestRunControl_ReplayRolloutPathStaysOnSimulationEndpoint(t *testing.T) {
	var payload map[string]any
	var mu sync.Mutex
	eventRequests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/events") {
			mu.Lock()
			eventRequests++
			mu.Unlock()
			http.Error(w, "rollout simulation must not open a run event stream", http.StatusConflict)
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/v1/runs/replay" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"mode":"simulate","events_replayed":3,"matched":true}`))
	}))
	defer srv.Close()

	m := testRunControlModel(srv.URL)
	cmds, quit := executeReplayCommand(&m, Command{Name: "replay", Args: []string{"/tmp/run_replay.jsonl"}})
	if quit {
		t.Fatal("/replay must not quit")
	}
	msg := lastCmd(t, cmds)()
	m2, _ := m.Update(msg)
	m = m2.(Model)

	if payload["rollout_path"] != "/tmp/run_replay.jsonl" || payload["mode"] != "simulate" {
		t.Fatalf("unexpected replay payload: %#v", payload)
	}
	if m.RunActive() || !strings.Contains(m.View(), "Replay result") {
		t.Fatalf("rollout replay should remain a one-shot result:\n%s", m.View())
	}
	mu.Lock()
	gotEventRequests := eventRequests
	mu.Unlock()
	if gotEventRequests != 0 {
		t.Fatalf("rollout simulation opened %d event stream request(s), want 0", gotEventRequests)
	}
}

// runControlAwaitSSE executes the batch returned by RunStartedMsg until the
// replay run's polling command produces the next stream message. The spinner
// tick is intentionally ignored: production dispatches it independently, and
// it does not participate in returned-run SSE ownership.
func runControlAwaitSSE(t *testing.T, cmd tea.Cmd, timeout time.Duration) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected replay RunStartedMsg to start the SSE bridge")
	}
	result := cmd()
	results := make(chan tea.Msg, 4)
	switch batch := result.(type) {
	case tea.BatchMsg:
		for _, sub := range batch {
			if sub == nil {
				continue
			}
			go func(sub tea.Cmd) { results <- sub() }(sub)
		}
	default:
		// Once the first stream event is rendered, Model.Update returns its
		// direct pollSSECmd rather than another batch.
		go func() { results <- result }()
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		select {
		case msg := <-results:
			switch msg.(type) {
			case SSEEventMsg, SSEDoneMsg:
				return msg
			}
		case <-deadline.C:
			t.Fatalf("timed out after %s waiting for replay SSE message", timeout)
		}
	}
}

func TestRunControl_ResumeCommandStartsContinuationRun(t *testing.T) {
	var prompt string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/runs/run_prev/continue" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body struct {
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		prompt = body.Prompt
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"run_id":"run_next","status":"queued","conversation_id":"run_prev"}`))
	}))
	defer srv.Close()

	m := testRunControlModel(srv.URL)
	m = m.WithCancelRun(func() {})
	cmds, quit := executeResumeCommand(&m, Command{Name: "resume", Args: []string{"run_prev", "keep", "going"}})
	if quit {
		t.Fatal("/resume must not quit")
	}
	msg := lastCmd(t, cmds)()
	m2, _ := m.Update(msg)
	m = m2.(Model)

	if prompt != "keep going" {
		t.Fatalf("continuation prompt = %q, want %q", prompt, "keep going")
	}
	if m.RunID != "run_next" {
		t.Fatalf("RunID = %q, want run_next", m.RunID)
	}
	if !m.RunActive() {
		t.Fatal("continuation run should be active")
	}
}

// TestIssue1261_ResumeContinuationUsesReturnedConversationIdentity proves a
// blank TUI does not convert a continuation child run ID into a conversation
// ID. The terminal bridge must subscribe to the parent conversation that owns
// the child run's durable messages.
func TestIssue1261_ResumeContinuationUsesReturnedConversationIdentity(t *testing.T) {
	const sourceConversation = "run_source_parent"
	const childRun = "run_child_continue"
	conversationPath := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/runs/"+sourceConversation+"/continue":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"run_id":"` + childRun + `","status":"queued","conversation_id":"` + sourceConversation + `"}`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/conversations/"):
			conversationPath <- r.URL.Path
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: message\ndata: {\"type\":\"run.completed\",\"payload\":{}}\n\n"))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	m := testRunControlModel(srv.URL).WithCancelRun(func() {})
	cmds, quit := executeResumeCommand(&m, Command{Name: "resume", Args: []string{sourceConversation, "continue"}})
	if quit {
		t.Fatal("/resume must not quit")
	}
	msg := lastCmd(t, cmds)()
	m2, _ := m.Update(msg)
	m = m2.(Model)
	if got := m.ConversationID(); got != sourceConversation {
		t.Fatalf("conversation identity after continuation = %q, want %q", got, sourceConversation)
	}

	// The injected run cancel avoids an unrelated run bridge. Its terminal
	// message must still launch the idle conversation bridge against P, never C.
	m2, cmd := m.Update(SSEDoneMsg{EventType: "run.completed"})
	m = m2.(Model)
	if cmd == nil {
		t.Fatal("terminal continuation must start the conversation SSE bridge")
	}
	// The terminal command is a Bubble Tea batch (spinner plus stream startup).
	// Invoke the identical selected-conversation owner directly so this test
	// cannot block on its independent spinner tick while proving the endpoint.
	started := m.startConversationSSE()()
	if _, ok := started.(conversationSSEStartedMsg); !ok {
		t.Fatalf("conversation command = %T, want conversationSSEStartedMsg", started)
	}
	select {
	case got := <-conversationPath:
		if want := "/v1/conversations/" + sourceConversation + "/events"; got != want {
			t.Fatalf("conversation SSE path = %q, want %q", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("continuation did not open its parent conversation SSE endpoint")
	}
	// Complete the selected-conversation lifecycle. A successful child reply
	// must not leave a false 404 / "run not found" status after its stream
	// reaches terminal idle state.
	m2, poll := m.Update(started)
	m = m2.(Model)
	if poll == nil {
		t.Fatal("conversation stream did not install its polling command")
	}
	m2, poll = m.Update(poll())
	m = m2.(Model)
	if poll != nil {
		m2, _ = m.Update(poll())
		m = m2.(Model)
	}
	if got := m.StatusMsg(); strings.Contains(strings.ToLower(got), "not found") || strings.Contains(strings.ToLower(got), "sse bridge") {
		t.Fatalf("successful continuation left false SSE warning: %q", got)
	}
}

// TestIssue1261_ResumeContinuationLegacyResponseResolvesChildIdentity proves
// a mixed-version server cannot make the TUI silently subscribe with the child
// run ID when its accepted continuation response predates conversation_id.
func TestIssue1261_ResumeContinuationLegacyResponseResolvesChildIdentity(t *testing.T) {
	const childRun = "run_child_legacy"
	const parentConversation = "run_parent_legacy"
	var gotChildLookup bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/runs/run_parent_legacy/continue":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"run_id":"` + childRun + `","status":"queued"}`))
		case "/v1/runs/" + childRun:
			gotChildLookup = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + childRun + `","conversation_id":"` + parentConversation + `"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	msg := continueRunCmd(srv.URL, parentConversation, "continue", "")()
	started, ok := msg.(RunStartedMsg)
	if !ok {
		t.Fatalf("legacy continuation = %T, want RunStartedMsg: %#v", msg, msg)
	}
	if !gotChildLookup {
		t.Fatal("legacy continuation did not resolve child run identity")
	}
	if got := started.ConversationID; got != parentConversation {
		t.Fatalf("legacy conversation identity = %q, want %q", got, parentConversation)
	}
}

func TestIssue1261_ResumeContinuationLegacyResponseFailsClosedWithoutConversation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/runs/run_parent_missing/continue":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"run_id":"run_child_missing","status":"queued"}`))
		case "/v1/runs/run_child_missing":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"run_child_missing"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	msg := continueRunCmd(srv.URL, "run_parent_missing", "continue", "")()
	failed, ok := msg.(RunFailedMsg)
	if !ok {
		t.Fatalf("missing legacy conversation identity = %T, want RunFailedMsg: %#v", msg, msg)
	}
	if !strings.Contains(failed.Error, "resolve conversation identity") {
		t.Fatalf("legacy error = %q, want identity-resolution failure", failed.Error)
	}
}

func TestIssue1261_RunStartedPreservesExistingMismatchedConversation(t *testing.T) {
	m := testRunControlModel("http://127.0.0.1:1")
	m2, _ := m.Update(RunStartedMsg{RunID: "run_existing", ConversationID: "conversation_existing"})
	m = m2.(Model)
	m = m.WithCancelRun(func() {})
	m2, _ = m.Update(RunStartedMsg{RunID: "run_child", ConversationID: "conversation_other"})
	m = m2.(Model)
	if got := m.ConversationID(); got != "conversation_existing" {
		t.Fatalf("mismatched continuation changed selected conversation to %q", got)
	}
}

func TestRegression_ResumeWithoutAssistantContentDoesNotDuplicatePriorReply(t *testing.T) {
	for _, terminalEvent := range []string{"run.completed", "run.failed"} {
		t.Run(terminalEvent, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/v1/runs/run_prev/continue" {
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte(`{"run_id":"run_next","status":"queued","conversation_id":"run_prev"}`))
			}))
			defer srv.Close()

			m := testRunControlModel(srv.URL).WithCancelRun(func() {})
			started, _ := m.Update(RunStartedMsg{RunID: "run_prev"})
			m = started.(Model)
			assistant, _ := m.Update(SSEEventMsg{
				EventType: "assistant.message",
				Raw:       []byte(`{"content":"PRIOR_ASSISTANT_REPLY"}`),
			})
			m = assistant.(Model)
			completed, _ := m.Update(SSEDoneMsg{EventType: "run.completed"})
			m = completed.(Model)
			// SSEDoneMsg consumes and clears the active run's cancel function.
			// Reinstall the test seam so the continuation's RunStartedMsg does
			// not open a real SSE bridge against this command-only HTTP server.
			m = m.WithCancelRun(func() {})

			cmds, quit := executeResumeCommand(&m, Command{Name: "resume", Args: []string{"run_prev", "continue", "without", "a", "reply"}})
			if quit {
				t.Fatal("/resume must not quit")
			}
			continuationStarted := lastCmd(t, cmds)()
			next, _ := m.Update(continuationStarted)
			m = next.(Model)
			terminal, _ := m.Update(SSEDoneMsg{EventType: terminalEvent, Error: "continuation failed"})
			m = terminal.(Model)

			transcript := m.Transcript()
			if len(transcript) != 2 {
				t.Fatalf("transcript = %+v, want prior assistant reply and continuation user prompt only", transcript)
			}
			if transcript[0].Role != "assistant" || transcript[0].Content != "PRIOR_ASSISTANT_REPLY" {
				t.Fatalf("prior assistant transcript = %+v", transcript[0])
			}
			if transcript[1].Role != "user" || transcript[1].Content != "continue without a reply" {
				t.Fatalf("continuation user transcript = %+v", transcript[1])
			}
		})
	}
}

func TestRunControl_RunsSnapshot80x24(t *testing.T) {
	writeRunsSnapshot(t, 80, 24, "TUI-058-runs-80x24.txt")
}

func TestRunControl_RunsSnapshot120x40(t *testing.T) {
	writeRunsSnapshot(t, 120, 40, "TUI-058-runs-120x40.txt")
}

func TestRunControl_RunsSnapshot200x50(t *testing.T) {
	writeRunsSnapshot(t, 200, 50, "TUI-058-runs-200x50.txt")
}

func writeRunsSnapshot(t *testing.T, width, height int, name string) {
	t.Helper()
	m := testRunControlModel("http://localhost:8080")
	m2, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	m = m2.(Model)
	m3, _ := m.Update(RunsFetchedMsg{Runs: []tuiRunRecord{
		{ID: "run_daily_1", Status: "completed", Model: "gpt-4.1", Prompt: "fix terminal workflow and replay search"},
		{ID: "run_daily_2", Status: "running", Model: "claude-sonnet-4", Prompt: "continue trusted harness loop"},
	}})
	m = m3.(Model)

	output := m.View()
	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/" + name
	if err := os.WriteFile(path, []byte(output), 0o644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	if !strings.Contains(output, "run_daily_1") || !strings.Contains(output, "run_daily_2") {
		t.Fatalf("snapshot must contain both run IDs, got:\n%s", output)
	}
}
