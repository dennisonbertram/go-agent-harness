package symphd

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewOrchestrator(t *testing.T) {
	cfg := DefaultConfig()
	o := NewOrchestrator(cfg)
	if o == nil {
		t.Fatal("NewOrchestrator returned nil")
	}
	if o.config != cfg {
		t.Error("config not set")
	}
	if o.startedAt.IsZero() {
		t.Error("startedAt not set")
	}
}

func TestOrchestrator_State(t *testing.T) {
	cfg := DefaultConfig()
	o := NewOrchestrator(cfg)
	state := o.State()
	if state["version"] != "0.1.0" {
		t.Errorf("version = %v", state["version"])
	}
	if _, ok := state["running_since"]; !ok {
		t.Error("running_since missing")
	}
	if state["agent_count"] != 0 {
		t.Errorf("agent_count = %v", state["agent_count"])
	}
}

func TestOrchestrator_State_RunningTimeIncreases(t *testing.T) {
	cfg := DefaultConfig()
	o := NewOrchestrator(cfg)
	s1 := o.State()
	time.Sleep(10 * time.Millisecond)
	s2 := o.State()
	// running_since should be the same (fixed at start)
	if s1["running_since"] != s2["running_since"] {
		t.Error("running_since changed between calls")
	}
}

func TestOrchestrator_Start(t *testing.T) {
	cfg := DefaultConfig()
	o := NewOrchestrator(cfg)
	if err := o.Start(context.Background()); err != nil {
		t.Errorf("Start returned error: %v", err)
	}
}

func TestOrchestrator_Shutdown(t *testing.T) {
	cfg := DefaultConfig()
	o := NewOrchestrator(cfg)
	if err := o.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown returned error: %v", err)
	}
}

func TestOrchestrator_Concurrent_State(t *testing.T) {
	cfg := DefaultConfig()
	o := NewOrchestrator(cfg)
	// Concurrent reads should not race
	done := make(chan struct{})
	for i := 0; i < 20; i++ {
		go func() {
			_ = o.State()
			done <- struct{}{}
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}

// ---------------------------------------------------------------------------
// handleFailedResult / result drain tests
// ---------------------------------------------------------------------------

// TestOrchestrator_ResultDrain_TriggersRetry verifies that when handleFailedResult
// is called for an issue whose attempt count is below the retry ceiling, the issue
// is Reset() back to Unclaimed so it can be picked up on the next poll tick.
func TestOrchestrator_ResultDrain_TriggersRetry(t *testing.T) {
	cfg := DefaultConfig()
	// MaxAttempts = 5 (default); issue has Attempts=1, so ShouldRetry returns true.
	o := NewOrchestrator(cfg)

	// Set up mock tracker with issue #1 in ClaimStateFailed state.
	tr := newMockTracker(&TrackedIssue{
		Number:     1,
		Title:      "Retry me",
		ClaimState: ClaimStateFailed,
		Attempts:   1,
	})
	o.SetTracker(tr)

	result := RunResult{
		IssueNumber: 1,
		Success:     false,
		Error:       errors.New("harness run failed"),
	}

	o.handleFailedResult(result)

	// Verify Reset was called, which transitions the issue back to Unclaimed.
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if len(tr.reset) != 1 || tr.reset[0] != 1 {
		t.Errorf("tracker.reset = %v, want [1]", tr.reset)
	}

	issue, ok := tr.issues[1]
	if !ok {
		t.Fatal("issue #1 not found in tracker")
	}
	if issue.ClaimState != ClaimStateUnclaimed {
		t.Errorf("issue ClaimState = %s, want %s", issue.ClaimState, ClaimStateUnclaimed)
	}

	// Nothing should have ended up in the dead-letter queue.
	if dl := o.DeadLetters(); len(dl) != 0 {
		t.Errorf("dead_letters = %d, want 0", len(dl))
	}
}

// TestOrchestrator_ResultDrain_DeadLettersAfterMaxAttempts verifies that when an
// issue has reached (or exceeded) the retry limit, handleFailedResult moves it to
// the dead-letter queue rather than resetting it.
func TestOrchestrator_ResultDrain_DeadLettersAfterMaxAttempts(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RetryMaxAttempts = 3
	o := NewOrchestrator(cfg)

	// Issue already has Attempts=3 (== MaxAttempts), so ShouldRetry returns false.
	tr := newMockTracker(&TrackedIssue{
		Number:     42,
		Title:      "Exhausted",
		ClaimState: ClaimStateFailed,
		Attempts:   3,
	})
	o.SetTracker(tr)

	result := RunResult{
		IssueNumber: 42,
		Success:     false,
		Error:       errors.New("stall timeout exceeded"),
	}

	o.handleFailedResult(result)

	// Verify no reset was called.
	tr.mu.Lock()
	if len(tr.reset) != 0 {
		t.Errorf("tracker.reset = %v, want []", tr.reset)
	}
	tr.mu.Unlock()

	// Verify the issue appears in the dead-letter queue.
	dl := o.DeadLetters()
	if len(dl) != 1 {
		t.Fatalf("dead_letters = %d, want 1", len(dl))
	}
	if dl[0].IssueNumber != 42 {
		t.Errorf("dead_letter.IssueNumber = %d, want 42", dl[0].IssueNumber)
	}
	if dl[0].LastError != "stall timeout exceeded" {
		t.Errorf("dead_letter.LastError = %q, want %q", dl[0].LastError, "stall timeout exceeded")
	}
	if dl[0].Attempts != 3 {
		t.Errorf("dead_letter.Attempts = %d, want 3", dl[0].Attempts)
	}
}

// TestOrchestrator_Start_ProcessesResults is an integration test that wires a
// real *Dispatcher to the Orchestrator, injects a failed run result through the
// dispatcher's results channel (by dispatching an issue with a harness client
// that always returns "failed"), and verifies that the drain goroutine in Start()
// forwards the failure to handleFailedResult — ultimately resulting in the issue
// appearing in the dead-letter queue (RetryMaxAttempts=1 so first failure = DLQ).
func TestOrchestrator_Start_ProcessesResults(t *testing.T) {
	srv := newHealthzServer()
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.RetryMaxAttempts = 1 // first failure immediately dead-letters
	cfg.PollIntervalMs = 50  // fast poll for test
	o := NewOrchestrator(cfg)

	// Issue starts in ClaimStateFailed with Attempts=1 (>= MaxAttempts=1).
	// The dispatcher itself will call tracker.Fail() after getting "failed" status,
	// but we need the issue to already be at max attempts for the DLQ path.
	// Strategy: use Attempts=1 with MaxAttempts=1 → ShouldRetry(1) = false → DLQ.
	issue := &TrackedIssue{
		Number:     99,
		Title:      "Integration test issue",
		Body:       "test body",
		ClaimState: ClaimStateClaimed, // Dispatcher expects Claimed → can call Start()
		Attempts:   1,                 // already at max
	}
	tr := newMockTracker(issue)
	o.SetTracker(tr)

	cl := &mockHarnessClient{
		startFunc: func(_ context.Context, _, _ string) (string, error) {
			return "run-99", nil
		},
		statusFunc: func(_ context.Context, _ string) (string, error) {
			return "failed", nil
		},
	}

	ws := &mockWorkspace{path: "/tmp/ws-99", harnessURL: srv.URL}
	dcfg := DispatchConfig{
		MaxConcurrent: 1,
		StallTimeout:  500 * time.Millisecond,
		PollInterval:  10 * time.Millisecond,
		HarnessURL:    srv.URL,
	}
	d := NewDispatcher(dcfg, wsFactory(ws), tr, clFactory(cl))
	o.SetDispatcher(d)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Run Start() in a goroutine; it blocks until ctx is cancelled.
	startDone := make(chan error, 1)
	go func() {
		startDone <- o.Start(ctx)
	}()

	// Dispatch the issue manually (simulates what the poll tick would do).
	// We do it after Start() has launched the drain goroutine.
	// Give Start a moment to set up its drain goroutine.
	time.Sleep(20 * time.Millisecond)
	if err := d.Dispatch(ctx, issue); err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}

	// Poll until the issue appears in the dead-letter queue, or timeout.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if dl := o.DeadLetters(); len(dl) > 0 {
			if dl[0].IssueNumber == 99 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel() // stop Start()
	<-startDone

	dl := o.DeadLetters()
	if len(dl) != 1 {
		t.Fatalf("dead_letters = %d, want 1", len(dl))
	}
	if dl[0].IssueNumber != 99 {
		t.Errorf("dead_letter.IssueNumber = %d, want 99", dl[0].IssueNumber)
	}
}
