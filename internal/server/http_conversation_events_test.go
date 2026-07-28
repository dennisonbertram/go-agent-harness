package server

// http_conversation_events_test.go — proves GET /v1/conversations/{id}/events
// (issue #950) streams events for ANY run on a conversation, not just a run
// the connected client itself started. This is the fix for delayed
// (set_delayed_callback) and cron-started runs whose output was previously
// invisible: those runs execute on a conversation with no run-scoped SSE
// listener attached (internal/harness/callback_bridge.go), so nothing was
// ever streamed to the client. A conversation-scoped subscriber does not have
// this gap: it is registered once and observes every run started afterwards.
//
// Design notes:
//   - package server (in-package) to mirror http_callback_sse_test.go: needed
//     so a future extension of these tests could reach unexported helpers.
//   - Uses fakeprovider (content-only turns, no tool calls) so each run
//     completes in exactly one Complete() call -- deterministic without Hang.
//   - openConversationEventsStream's synchronization guarantee: the handler
//     (handleConversationEvents) calls Runner.SubscribeConversation, which
//     registers the subscriber channel BEFORE the response headers are
//     written. Since http.Client.Do only returns once headers are received,
//     any run started after Do() returns is guaranteed to be observed live,
//     not just replayed from history.

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-agent-harness/internal/fakeprovider"
	"go-agent-harness/internal/harness"
)

// conversationSSEEvent is a minimally-typed view of an SSE-delivered
// harness.Event frame for the assertions in this file.
type conversationSSEEvent struct {
	Type    string         `json:"type"`
	RunID   string         `json:"run_id"`
	Payload map[string]any `json:"payload"`
}

// newConversationEventsTestServer builds a Runner backed by a scripted
// fakeprovider (one content-only turn per run) and an httptest server for it.
func newConversationEventsTestServer(t *testing.T, turns []fakeprovider.Turn) (*harness.Runner, *httptest.Server) {
	t.Helper()

	prov := fakeprovider.New(turns)
	runner := harness.NewRunner(prov, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     3,
	})
	t.Cleanup(func() { runner.Shutdown(context.Background()) })

	ts := httptest.NewServer(New(runner))
	t.Cleanup(ts.Close)

	return runner, ts
}

// pollUntilRunTerminal polls GetRun until runID reaches a terminal status.
func pollUntilRunTerminal(t *testing.T, runner *harness.Runner, runID string) harness.Run {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		run, ok := runner.GetRun(runID)
		if !ok {
			t.Fatalf("run %q not found", runID)
		}
		switch run.Status {
		case harness.RunStatusCompleted, harness.RunStatusFailed, harness.RunStatusCancelled:
			return run
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run %q did not reach a terminal status in time", runID)
	return harness.Run{}
}

// openConversationEventsStream issues GET /v1/conversations/{id}/events and
// returns a channel of decoded SSE frames plus a cleanup func. By the time
// this function returns, the runner has already registered the subscription
// (see the package doc comment above) -- callers can rely on that ordering.
func openConversationEventsStream(t *testing.T, baseURL, convID string) (<-chan conversationSSEEvent, func()) {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, baseURL+"/v1/conversations/"+convID+"/events", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open conversation events stream: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("conversation events stream: expected 200, got %d: %s", resp.StatusCode, body)
	}

	out := make(chan conversationSSEEvent, 64)
	go func() {
		defer close(out)
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
		for scanner.Scan() {
			data, ok := strings.CutPrefix(scanner.Text(), "data: ")
			if !ok {
				continue
			}
			var ev conversationSSEEvent
			if err := json.Unmarshal([]byte(data), &ev); err != nil {
				continue
			}
			out <- ev
		}
	}()

	return out, func() { resp.Body.Close() }
}

// waitForRunStartedEvent reads events until it sees run.started for runID.
func waitForRunStartedEvent(t *testing.T, events <-chan conversationSSEEvent, runID string) conversationSSEEvent {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatal("conversation events stream closed before run.started was observed")
			}
			if ev.Type == "run.started" && ev.RunID == runID {
				return ev
			}
		case <-deadline:
			t.Fatalf("timed out waiting for run.started for run %q on the conversation stream", runID)
		}
	}
}

// BT-001: subscribing to a conversation's events receives events from a run
// the subscriber did not start.
func TestConversationEvents_ReceivesRunSubscriberDidNotStart(t *testing.T) {
	t.Parallel()

	runner, ts := newConversationEventsTestServer(t, []fakeprovider.Turn{
		{Content: "run one output"},
		{Content: "run two output"},
	})

	const convID = "conv-bt001"

	run1, err := runner.StartRun(harness.RunRequest{Prompt: "first", ConversationID: convID})
	if err != nil {
		t.Fatalf("StartRun run1: %v", err)
	}
	pollUntilRunTerminal(t, runner, run1.ID)

	events, closeStream := openConversationEventsStream(t, ts.URL, convID)
	defer closeStream()

	// run2 is started directly against the runner -- not through the HTTP
	// client that opened the conversation stream. This stands in for a
	// callback- or cron-started run: something other than the subscriber
	// caused it to start.
	run2, err := runner.StartRun(harness.RunRequest{Prompt: "second", ConversationID: convID})
	if err != nil {
		t.Fatalf("StartRun run2: %v", err)
	}

	got := waitForRunStartedEvent(t, events, run2.ID)
	if conv, _ := got.Payload["conversation_id"].(string); conv != convID {
		t.Errorf("run.started conversation_id = %q, want %q", conv, convID)
	}
}

// BT-002: a second run on the same conversation also reaches the same
// subscriber (proves the conversation stream is not a one-shot delivery).
func TestConversationEvents_SecondRunAlsoReachesSameSubscriber(t *testing.T) {
	t.Parallel()

	runner, ts := newConversationEventsTestServer(t, []fakeprovider.Turn{
		{Content: "run one output"},
		{Content: "run two output"},
		{Content: "run three output"},
	})

	const convID = "conv-bt002"

	run1, err := runner.StartRun(harness.RunRequest{Prompt: "first", ConversationID: convID})
	if err != nil {
		t.Fatalf("StartRun run1: %v", err)
	}
	pollUntilRunTerminal(t, runner, run1.ID)

	events, closeStream := openConversationEventsStream(t, ts.URL, convID)
	defer closeStream()

	run2, err := runner.StartRun(harness.RunRequest{Prompt: "second", ConversationID: convID})
	if err != nil {
		t.Fatalf("StartRun run2: %v", err)
	}
	waitForRunStartedEvent(t, events, run2.ID)
	pollUntilRunTerminal(t, runner, run2.ID)

	run3, err := runner.StartRun(harness.RunRequest{Prompt: "third", ConversationID: convID})
	if err != nil {
		t.Fatalf("StartRun run3: %v", err)
	}
	waitForRunStartedEvent(t, events, run3.ID)
}

// BT-003: the route 404s for an unknown conversation.
func TestConversationEvents_UnknownConversation404(t *testing.T) {
	t.Parallel()

	_, ts := newConversationEventsTestServer(t, nil)

	resp, err := http.Get(ts.URL + "/v1/conversations/does-not-exist/events")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Code != "not_found" {
		t.Errorf("error.code = %q, want %q", body.Error.Code, "not_found")
	}
}

// Regression (issue #950): a delayed-callback-style run that starts and
// completes entirely AFTER the originating run has already ended must still
// have its full output (not just a started marker) observed by a subscriber
// that opened the conversation stream in between -- proving the fix covers
// the exact reported scenario, not just a "run started" ping.
func TestConversationEvents_RegressionCallbackStyleRunAfterOriginatingRunEnded(t *testing.T) {
	t.Parallel()

	runner, ts := newConversationEventsTestServer(t, []fakeprovider.Turn{
		{Content: "original run output"},
		{Content: "delayed callback run output"},
	})

	const convID = "conv-regression-950"

	run1, err := runner.StartRun(harness.RunRequest{Prompt: "start", ConversationID: convID})
	if err != nil {
		t.Fatalf("StartRun run1: %v", err)
	}
	// The originating run has fully ended -- mirroring callback.fired firing
	// well after the run that scheduled it (internal/harness/callback_bridge.go),
	// long after that run's own /v1/runs/{id}/events stream has closed.
	pollUntilRunTerminal(t, runner, run1.ID)

	events, closeStream := openConversationEventsStream(t, ts.URL, convID)
	defer closeStream()

	callbackRun, err := runner.StartRun(harness.RunRequest{Prompt: "callback fired", ConversationID: convID})
	if err != nil {
		t.Fatalf("StartRun callback-style run: %v", err)
	}

	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatal("conversation stream closed before the callback-style run.completed was observed")
			}
			if ev.RunID == callbackRun.ID && ev.Type == string(harness.EventRunCompleted) {
				output, _ := ev.Payload["output"].(string)
				if output != "delayed callback run output" {
					t.Errorf("callback-style run output = %q, want %q", output, "delayed callback run output")
				}
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for the callback-style run.completed on the conversation stream")
		}
	}
}
