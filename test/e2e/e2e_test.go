package e2e

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/harness"
)

// TestMain forces a fast SSE keepalive interval for the whole package so
// tests that wait on a slow provider reliably observe at least one ": ping"
// comment line, proving the live stream (not just buffered history) is
// exercised and that keepalive framing is parsed correctly by clients.
func TestMain(m *testing.M) {
	os.Setenv("HARNESS_SSE_KEEPALIVE_SECONDS", "1")
	os.Exit(m.Run())
}

// TestE2E_HappyPathRunCompletes drives a full run over real HTTP + SSE: POST
// /v1/runs, then GET /v1/runs/{id}/events as a live stream. The provider
// deliberately takes longer than the keepalive interval so the stream must
// survive at least one ": ping" comment line before the terminal event, and
// the test asserts both the terminal event sequence and the SSE keepalive
// framing are handled correctly end to end.
func TestE2E_HappyPathRunCompletes(t *testing.T) {
	t.Parallel()

	provider := &slowProvider{
		delay:  1500 * time.Millisecond,
		result: harness.CompletionResult{Content: "hello from the fake model"},
	}
	ts := newTestServer(t, provider, nil, nil)

	runID := startRun(t, ts, `{"prompt":"say hi"}`)

	reader, closeStream := openEventStream(t, ts, runID)
	defer closeStream()

	types, terminal := drainUntilTerminal(t, reader, 10*time.Second)

	if len(types) == 0 || types[0] != harness.EventRunStarted {
		t.Fatalf("expected first event to be %q, got sequence %v", harness.EventRunStarted, types)
	}
	if terminal.Type != harness.EventRunCompleted {
		t.Fatalf("expected terminal event %q, got %q (sequence %v)", harness.EventRunCompleted, terminal.Type, types)
	}
	if reader.pings < 1 {
		t.Fatalf("expected at least one SSE keepalive ping comment line while the run was in flight, saw %d", reader.pings)
	}

	// The stream must end (server closes after the terminal event) rather
	// than hang waiting for more events.
	if _, err := reader.next(); err == nil {
		t.Fatal("expected SSE stream to close after the terminal event")
	}
}

// TestE2E_CancelledRunReachesCancelledEvent drives a run whose provider
// blocks indefinitely, cancels it via POST /v1/runs/{id}/cancel exactly as
// the CLI does, and asserts the live SSE stream delivers a terminal
// run.cancelled event.
func TestE2E_CancelledRunReachesCancelledEvent(t *testing.T) {
	t.Parallel()

	provider := newBlockingProvider()
	ts := newTestServer(t, provider, nil, nil)

	runID := startRun(t, ts, `{"prompt":"do something slow"}`)

	reader, closeStream := openEventStream(t, ts, runID)
	defer closeStream()

	select {
	case <-provider.started:
	case <-time.After(5 * time.Second):
		t.Fatal("provider never started blocking")
	}

	res, err := ts.Client().Post(ts.URL+"/v1/runs/"+runID+"/cancel", "application/json", nil)
	if err != nil {
		t.Fatalf("POST cancel: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("POST cancel: expected 200, got %d", res.StatusCode)
	}

	types, terminal := drainUntilTerminal(t, reader, 10*time.Second)

	if terminal.Type != harness.EventRunCancelled {
		t.Fatalf("expected terminal event %q, got %q (sequence %v)", harness.EventRunCancelled, terminal.Type, types)
	}
	if terminal.RunID != runID {
		t.Fatalf("terminal event run_id = %q, want %q", terminal.RunID, runID)
	}
}

// TestE2E_ToolCallApprovalRoundTrip drives a run whose fake provider requests
// a tool call under an "approval: all" permission policy, watches the live
// SSE stream for tool.call.started and tool.approval_required, approves the
// pending call via POST /v1/runs/{id}/approve exactly as a client would, and
// asserts the tool executes and the run reaches a completed terminal event
// with the post-approval assistant content.
func TestE2E_ToolCallApprovalRoundTrip(t *testing.T) {
	t.Parallel()

	broker := harness.NewInMemoryApprovalBroker()

	provider := &scriptedProvider{
		turns: []harness.CompletionResult{
			{
				ToolCalls: []harness.ToolCall{{
					ID:        "call_1",
					Name:      "echo_tool",
					Arguments: `{"value":"ping"}`,
				}},
			},
			{Content: "done after approval"},
		},
	}

	registry := harness.NewRegistry()
	if err := registry.Register(harness.ToolDefinition{
		Name:        "echo_tool",
		Description: "echoes its input",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"value": map[string]any{"type": "string"}},
		},
		ParallelSafe: true,
	}, func(_ context.Context, args json.RawMessage) (string, error) {
		return string(args), nil
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	ts := newTestServer(t, provider, registry, broker)

	runID := startRun(t, ts, `{"prompt":"use the tool","permissions":{"sandbox":"unrestricted","approval":"all"}}`)

	reader, closeStream := openEventStream(t, ts, runID)
	defer closeStream()

	var (
		types           []harness.EventType
		sawToolStarted  bool
		sawApproval     bool
		sawToolComplete bool
		terminal        harness.Event
	)

	deadline := time.Now().Add(10 * time.Second)
	approved := false
	for {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for run to complete (sequence so far: %v)", types)
		}
		ev, err := reader.next()
		if err != nil {
			t.Fatalf("reading SSE stream: %v (sequence so far: %v)", err, types)
		}
		hev := ev.harnessEvent(t)
		types = append(types, hev.Type)

		switch hev.Type {
		case harness.EventToolCallStarted:
			sawToolStarted = true
		case harness.EventToolApprovalRequired:
			sawApproval = true
			if !approved {
				approved = true
				res, err := ts.Client().Post(ts.URL+"/v1/runs/"+runID+"/approve", "application/json", nil)
				if err != nil {
					t.Fatalf("POST approve: %v", err)
				}
				res.Body.Close()
				if res.StatusCode != 200 {
					t.Fatalf("POST approve: expected 200, got %d", res.StatusCode)
				}
			}
		case harness.EventToolCallCompleted:
			sawToolComplete = true
		}

		if harness.IsTerminalEvent(hev.Type) {
			terminal = hev
			break
		}
	}

	if !sawToolStarted {
		t.Errorf("expected to observe %q in the SSE stream, sequence: %v", harness.EventToolCallStarted, types)
	}
	if !sawApproval {
		t.Errorf("expected to observe %q in the SSE stream, sequence: %v", harness.EventToolApprovalRequired, types)
	}
	if !sawToolComplete {
		t.Errorf("expected to observe %q in the SSE stream, sequence: %v", harness.EventToolCallCompleted, types)
	}
	if terminal.Type != harness.EventRunCompleted {
		t.Fatalf("expected terminal event %q, got %q (sequence %v)", harness.EventRunCompleted, terminal.Type, types)
	}
}

// TestE2E_ToolApprovalEventIsImmediatelyResolvable pins the readiness
// contract for live SSE consumers: observing tool.approval_required makes an
// immediate approve or deny actionable, and the conversation reaches its
// terminal outcome. The gate holds the legacy Ask-only lifecycle before it can
// create its pending entry, so the old publish-then-Ask ordering
// deterministically returns 404 rather than relying on scheduler timing.
func TestE2E_ToolApprovalEventIsImmediatelyResolvable(t *testing.T) {
	for _, tc := range []struct {
		name string
		path string
		want harness.EventType
	}{
		{name: "approve", path: "approve", want: harness.EventToolApprovalGranted},
		{name: "deny", path: "deny", want: harness.EventToolApprovalDenied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			broker := newRegistrationGatedApprovalBroker()
			defer broker.release()

			provider := &scriptedProvider{turns: []harness.CompletionResult{
				{ToolCalls: []harness.ToolCall{{ID: "call_ready", Name: "echo_tool", Arguments: `{"value":"ready"}`}}},
				{Content: "done after immediate resolution"},
			}}
			registry := harness.NewRegistry()
			if err := registry.Register(harness.ToolDefinition{
				Name:         "echo_tool",
				Description:  "echoes its input",
				Parameters:   map[string]any{"type": "object"},
				ParallelSafe: true,
			}, func(_ context.Context, args json.RawMessage) (string, error) {
				return string(args), nil
			}); err != nil {
				t.Fatalf("register tool: %v", err)
			}

			ts := newTestServer(t, provider, registry, broker)
			runID := startRun(t, ts, `{"prompt":"use the tool","permissions":{"sandbox":"unrestricted","approval":"all"}}`)
			reader, closeStream := openEventStream(t, ts, runID)
			defer closeStream()

			deadline := time.Now().Add(10 * time.Second)
			resolved := false
			sawResolution := false
			for {
				if time.Now().After(deadline) {
					t.Fatalf("timed out waiting for terminal event after immediate %s", tc.path)
				}
				ev, err := reader.next()
				if err != nil {
					t.Fatalf("reading SSE stream: %v", err)
				}
				hev := ev.harnessEvent(t)
				switch hev.Type {
				case harness.EventToolApprovalRequired:
					if resolved {
						continue
					}
					pending, ok := broker.Pending(runID)
					if !ok {
						t.Fatal("tool.approval_required was observable before its pending approval")
					}
					gotDeadline, err := time.Parse(time.RFC3339Nano, hev.Payload["deadline_at"].(string))
					if err != nil {
						t.Fatalf("parse event deadline_at: %v", err)
					}
					if !gotDeadline.Equal(pending.DeadlineAt) {
						t.Fatalf("event deadline_at=%s, want exact registered deadline %s", gotDeadline, pending.DeadlineAt)
					}
					resolved = true
					res, err := ts.Client().Post(ts.URL+"/v1/runs/"+runID+"/"+tc.path, "application/json", nil)
					if err != nil {
						t.Fatalf("POST %s immediately after event: %v", tc.path, err)
					}
					res.Body.Close()
					if res.StatusCode != 200 {
						t.Fatalf("POST %s immediately after event: expected 200, got %d", tc.path, res.StatusCode)
					}
				case tc.want:
					sawResolution = true
				}
				if harness.IsTerminalEvent(hev.Type) {
					if hev.Type != harness.EventRunCompleted {
						t.Fatalf("terminal event = %q, want %q", hev.Type, harness.EventRunCompleted)
					}
					if !resolved || !sawResolution {
						t.Fatalf("terminal event arrived without immediate %s resolution (resolved=%v saw=%v)", tc.path, resolved, sawResolution)
					}
					return
				}
			}
		})
	}
}

// TestE2E_PlanExitApprovalEventIsImmediatelyResolvable drives both immediate
// plan denial and the subsequent immediate approval through real HTTP/SSE.
// The completed conversation proves that plan-exit registration also happens
// before its observable approval-required event.
func TestE2E_PlanExitApprovalEventIsImmediatelyResolvable(t *testing.T) {
	broker := newRegistrationGatedApprovalBroker()
	defer broker.release()
	ts := newTestServer(t, &scriptedProvider{turns: []harness.CompletionResult{
		{Content: "# initial plan"},
		{Content: "# revised plan"},
	}}, nil, broker)
	runID := startRun(t, ts, `{"prompt":"plan","plan_mode":true}`)
	reader, closeStream := openEventStream(t, ts, runID)
	defer closeStream()

	deadline := time.Now().Add(10 * time.Second)
	approvalCount := 0
	sawDenied := false
	sawGranted := false
	for {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for terminal plan event (approvals=%d)", approvalCount)
		}
		ev, err := reader.next()
		if err != nil {
			t.Fatalf("reading SSE stream: %v", err)
		}
		hev := ev.harnessEvent(t)
		switch hev.Type {
		case harness.EventPlanApprovalRequired:
			if _, ok := broker.Pending(runID); !ok {
				t.Fatal("plan.approval_required was observable before its pending approval")
			}
			path := "deny"
			if approvalCount > 0 {
				path = "approve"
			}
			approvalCount++
			res, err := ts.Client().Post(ts.URL+"/v1/runs/"+runID+"/"+path, "application/json", nil)
			if err != nil {
				t.Fatalf("POST %s immediately after plan event: %v", path, err)
			}
			res.Body.Close()
			if res.StatusCode != 200 {
				t.Fatalf("POST %s immediately after plan event: expected 200, got %d", path, res.StatusCode)
			}
		case harness.EventPlanApprovalDenied:
			sawDenied = true
		case harness.EventPlanApprovalGranted:
			sawGranted = true
		}
		if harness.IsTerminalEvent(hev.Type) {
			if hev.Type != harness.EventRunCompleted || approvalCount != 2 || !sawDenied || !sawGranted {
				t.Fatalf("unexpected plan terminal state: type=%q approvals=%d denied=%v granted=%v", hev.Type, approvalCount, sawDenied, sawGranted)
			}
			return
		}
	}
}

// registrationGatedApprovalBroker keeps the old Ask lifecycle at a
// deterministic pre-registration barrier. A readiness-aware runner registers
// before publication and therefore does not need this legacy path.
type registrationGatedApprovalBroker struct {
	inner      *harness.InMemoryApprovalBroker
	releaseAsk chan struct{}
	once       sync.Once
}

func newRegistrationGatedApprovalBroker() *registrationGatedApprovalBroker {
	return &registrationGatedApprovalBroker{
		inner:      harness.NewInMemoryApprovalBroker(),
		releaseAsk: make(chan struct{}),
	}
}

func (b *registrationGatedApprovalBroker) Ask(ctx context.Context, req harness.ApprovalRequest) (bool, string, error) {
	select {
	case <-b.releaseAsk:
		return b.inner.Ask(ctx, req)
	case <-ctx.Done():
		return false, "", ctx.Err()
	}
}

func (b *registrationGatedApprovalBroker) Register(ctx context.Context, req harness.ApprovalRequest) (harness.ApprovalWaiter, error) {
	return b.inner.Register(ctx, req)
}

func (b *registrationGatedApprovalBroker) Pending(runID string) (harness.PendingApproval, bool) {
	return b.inner.Pending(runID)
}

func (b *registrationGatedApprovalBroker) Approve(runID string) error {
	return b.inner.Approve(runID)
}

func (b *registrationGatedApprovalBroker) ApproveWithOption(runID, option string) error {
	return b.inner.ApproveWithOption(runID, option)
}

func (b *registrationGatedApprovalBroker) Deny(runID string) error {
	return b.inner.Deny(runID)
}

func (b *registrationGatedApprovalBroker) release() {
	b.once.Do(func() { close(b.releaseAsk) })
}
