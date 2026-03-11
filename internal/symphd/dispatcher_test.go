package symphd

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-agent-harness/internal/workspace"
)

// ---------------------------------------------------------------------------
// Mock workspace
// ---------------------------------------------------------------------------

type mockWorkspace struct {
	mu          sync.Mutex
	provisioned bool
	destroyed   bool
	provideErr  error
	path        string
	harnessURL  string
}

func (m *mockWorkspace) Provision(_ context.Context, _ workspace.Options) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.provideErr != nil {
		return m.provideErr
	}
	m.provisioned = true
	return nil
}

func (m *mockWorkspace) HarnessURL() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.harnessURL
}

func (m *mockWorkspace) WorkspacePath() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.path
}

func (m *mockWorkspace) Destroy(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.destroyed = true
	return nil
}

// ---------------------------------------------------------------------------
// Mock harness client
// ---------------------------------------------------------------------------

type mockHarnessClient struct {
	startFunc  func(ctx context.Context, prompt, path string) (string, error)
	statusFunc func(ctx context.Context, runID string) (string, error)
}

func (m *mockHarnessClient) StartRun(ctx context.Context, prompt, path string) (string, error) {
	return m.startFunc(ctx, prompt, path)
}

func (m *mockHarnessClient) RunStatus(ctx context.Context, runID string) (string, error) {
	return m.statusFunc(ctx, runID)
}

// ---------------------------------------------------------------------------
// Mock tracker
// ---------------------------------------------------------------------------

type mockTracker struct {
	mu       sync.Mutex
	issues   map[int]*TrackedIssue
	started  []int
	complete []int
	failed   []int
}

func newMockTracker(issues ...*TrackedIssue) *mockTracker {
	m := &mockTracker{issues: make(map[int]*TrackedIssue)}
	for _, issue := range issues {
		m.issues[issue.Number] = issue
	}
	return m
}

func (m *mockTracker) Poll(_ context.Context) error { return nil }

func (m *mockTracker) Candidates() []*TrackedIssue {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*TrackedIssue
	for _, i := range m.issues {
		if i.ClaimState == ClaimStateClaimed {
			cp := *i
			out = append(out, &cp)
		}
	}
	return out
}

func (m *mockTracker) Claim(number int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	i, ok := m.issues[number]
	if !ok {
		return fmt.Errorf("not found: %d", number)
	}
	i.ClaimState = ClaimStateClaimed
	return nil
}

func (m *mockTracker) Start(number int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	i, ok := m.issues[number]
	if !ok {
		return fmt.Errorf("not found: %d", number)
	}
	if i.ClaimState != ClaimStateClaimed {
		return fmt.Errorf("issue #%d is %s, cannot start", number, i.ClaimState)
	}
	i.ClaimState = ClaimStateRunning
	m.started = append(m.started, number)
	return nil
}

func (m *mockTracker) Complete(number int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	i, ok := m.issues[number]
	if !ok {
		return fmt.Errorf("not found: %d", number)
	}
	if i.ClaimState != ClaimStateRunning {
		return fmt.Errorf("issue #%d is %s, cannot complete", number, i.ClaimState)
	}
	i.ClaimState = ClaimStateDone
	m.complete = append(m.complete, number)
	return nil
}

func (m *mockTracker) Fail(number int, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	i, ok := m.issues[number]
	if !ok {
		return fmt.Errorf("not found: %d", number)
	}
	if i.ClaimState != ClaimStateRunning {
		return fmt.Errorf("issue #%d is %s, cannot fail: %s", number, i.ClaimState, reason)
	}
	i.ClaimState = ClaimStateFailed
	m.failed = append(m.failed, number)
	return nil
}

func (m *mockTracker) Issues() []*TrackedIssue {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*TrackedIssue, 0, len(m.issues))
	for _, i := range m.issues {
		cp := *i
		out = append(out, &cp)
	}
	return out
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func claimedIssue(n int) *TrackedIssue {
	return &TrackedIssue{
		Number:     n,
		Title:      fmt.Sprintf("Issue %d", n),
		Body:       fmt.Sprintf("Body for issue %d", n),
		ClaimState: ClaimStateClaimed,
	}
}

func fastDispatchConfig() DispatchConfig {
	return DispatchConfig{
		MaxConcurrent: 2,
		StallTimeout:  200 * time.Millisecond,
		HarnessURL:    "http://localhost:8080",
		PollInterval:  10 * time.Millisecond,
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestNewDispatcher verifies constructor sets fields correctly.
func TestNewDispatcher(t *testing.T) {
	ws := &mockWorkspace{}
	tr := newMockTracker()
	cl := &mockHarnessClient{}
	cfg := DispatchConfig{MaxConcurrent: 3, StallTimeout: time.Minute, HarnessURL: "http://example.com"}
	d := NewDispatcher(cfg, ws, tr, cl)

	if d == nil {
		t.Fatal("NewDispatcher returned nil")
	}
	if cap(d.sem) != 3 {
		t.Errorf("semaphore capacity = %d, want 3", cap(d.sem))
	}
	if d.config.StallTimeout != time.Minute {
		t.Errorf("StallTimeout = %v, want 1m", d.config.StallTimeout)
	}
	if d.results == nil {
		t.Error("results channel is nil")
	}
	if d.running == nil {
		t.Error("running map is nil")
	}
}

// TestNewDispatcher_Defaults verifies zero-value fields receive sensible defaults.
func TestNewDispatcher_Defaults(t *testing.T) {
	ws := &mockWorkspace{}
	tr := newMockTracker()
	cl := &mockHarnessClient{}
	d := NewDispatcher(DispatchConfig{}, ws, tr, cl)

	if d.config.StallTimeout != 5*time.Minute {
		t.Errorf("default StallTimeout = %v, want 5m", d.config.StallTimeout)
	}
	if d.config.PollInterval != 5*time.Second {
		t.Errorf("default PollInterval = %v, want 5s", d.config.PollInterval)
	}
	if cap(d.sem) != 1 {
		t.Errorf("default semaphore capacity = %d, want 1", cap(d.sem))
	}
}

// TestDispatcher_Dispatch_Success verifies a happy-path dispatch:
// workspace provisioned, run started, polled to "completed", tracker updated.
func TestDispatcher_Dispatch_Success(t *testing.T) {
	issue := claimedIssue(42)
	tr := newMockTracker(issue)
	ws := &mockWorkspace{path: "/tmp/ws-42"}

	cl := &mockHarnessClient{
		startFunc: func(_ context.Context, prompt, path string) (string, error) {
			if path != "/tmp/ws-42" {
				return "", fmt.Errorf("unexpected workspace path: %q", path)
			}
			return "run-42", nil
		},
		statusFunc: func(_ context.Context, runID string) (string, error) {
			return "completed", nil
		},
	}

	cfg := fastDispatchConfig()
	d := NewDispatcher(cfg, ws, tr, cl)

	ctx := context.Background()
	if err := d.Dispatch(ctx, issue); err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}

	result := <-d.Results()

	if !result.Success {
		t.Errorf("expected Success=true, got error: %v", result.Error)
	}
	if result.IssueNumber != 42 {
		t.Errorf("IssueNumber = %d, want 42", result.IssueNumber)
	}
	if result.Duration <= 0 {
		t.Error("Duration should be positive")
	}

	// Verify workspace was provisioned.
	if !ws.provisioned {
		t.Error("workspace was not provisioned")
	}

	// Verify tracker transitions.
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if len(tr.started) != 1 || tr.started[0] != 42 {
		t.Errorf("tracker.started = %v, want [42]", tr.started)
	}
	if len(tr.complete) != 1 || tr.complete[0] != 42 {
		t.Errorf("tracker.complete = %v, want [42]", tr.complete)
	}
	if len(tr.failed) != 0 {
		t.Errorf("tracker.failed = %v, want []", tr.failed)
	}
}

// TestDispatcher_Dispatch_HarnessError verifies that a StartRun failure
// calls tracker.Fail and returns an error result.
func TestDispatcher_Dispatch_HarnessError(t *testing.T) {
	issue := claimedIssue(10)
	tr := newMockTracker(issue)
	ws := &mockWorkspace{path: "/tmp/ws-10"}

	cl := &mockHarnessClient{
		startFunc: func(_ context.Context, _, _ string) (string, error) {
			return "", errors.New("connection refused")
		},
		statusFunc: func(_ context.Context, _ string) (string, error) {
			return "", errors.New("should not be called")
		},
	}

	cfg := fastDispatchConfig()
	d := NewDispatcher(cfg, ws, tr, cl)

	ctx := context.Background()
	if err := d.Dispatch(ctx, issue); err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}

	result := <-d.Results()

	if result.Success {
		t.Error("expected Success=false")
	}
	if result.Error == nil {
		t.Error("expected non-nil error")
	}

	tr.mu.Lock()
	defer tr.mu.Unlock()
	if len(tr.failed) != 1 || tr.failed[0] != 10 {
		t.Errorf("tracker.failed = %v, want [10]", tr.failed)
	}
	if len(tr.complete) != 0 {
		t.Errorf("tracker.complete = %v, want []", tr.complete)
	}
}

// TestDispatcher_Dispatch_WorkspaceProvisionError verifies that a workspace
// provision failure calls tracker.Fail.
func TestDispatcher_Dispatch_WorkspaceProvisionError(t *testing.T) {
	issue := claimedIssue(7)
	tr := newMockTracker(issue)
	ws := &mockWorkspace{provideErr: errors.New("disk full")}

	cl := &mockHarnessClient{
		startFunc: func(_ context.Context, _, _ string) (string, error) {
			return "", errors.New("should not be called")
		},
		statusFunc: func(_ context.Context, _ string) (string, error) {
			return "", errors.New("should not be called")
		},
	}

	cfg := fastDispatchConfig()
	d := NewDispatcher(cfg, ws, tr, cl)

	ctx := context.Background()
	if err := d.Dispatch(ctx, issue); err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}

	result := <-d.Results()

	if result.Success {
		t.Error("expected Success=false")
	}
	if result.Error == nil {
		t.Error("expected non-nil error")
	}

	tr.mu.Lock()
	defer tr.mu.Unlock()
	if len(tr.failed) != 1 || tr.failed[0] != 7 {
		t.Errorf("tracker.failed = %v, want [7]", tr.failed)
	}
}

// TestDispatcher_Dispatch_Concurrency dispatches 3 issues with MaxConcurrent=2
// and verifies that at most 2 run simultaneously.
func TestDispatcher_Dispatch_Concurrency(t *testing.T) {
	const numIssues = 3
	issues := make([]*TrackedIssue, numIssues)
	for i := range issues {
		issues[i] = claimedIssue(100 + i)
	}

	tr := newMockTracker(issues...)

	ws := &mockWorkspace{path: "/tmp/ws"}

	var (
		mu         sync.Mutex
		concurrent int
		maxSeen    int
	)

	gate := make(chan struct{})
	var started atomic.Int32

	cl := &mockHarnessClient{
		startFunc: func(_ context.Context, _, _ string) (string, error) {
			return "run-x", nil
		},
		statusFunc: func(_ context.Context, runID string) (string, error) {
			// Increment concurrent counter and track maximum.
			mu.Lock()
			concurrent++
			if concurrent > maxSeen {
				maxSeen = concurrent
			}
			mu.Unlock()

			// Signal that this goroutine is active.
			started.Add(1)

			// Block until gate is released.
			<-gate

			mu.Lock()
			concurrent--
			mu.Unlock()
			return "completed", nil
		},
	}

	cfg := DispatchConfig{
		MaxConcurrent: 2,
		StallTimeout:  5 * time.Second,
		HarnessURL:    "http://localhost:8080",
		PollInterval:  5 * time.Millisecond,
	}
	d := NewDispatcher(cfg, ws, tr, cl)

	ctx := context.Background()

	// Dispatch all issues. Since MaxConcurrent=2 and the status func blocks,
	// the third Dispatch call should block until a slot is free.
	errCh := make(chan error, numIssues)
	var wg sync.WaitGroup
	for _, issue := range issues {
		wg.Add(1)
		go func(i *TrackedIssue) {
			defer wg.Done()
			errCh <- d.Dispatch(ctx, i)
		}(issue)
	}

	// Wait until at least 2 goroutines have entered status polling.
	deadline := time.Now().Add(2 * time.Second)
	for started.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	// Release all gates.
	close(gate)

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Errorf("Dispatch returned error: %v", err)
		}
	}

	// Drain results.
	for i := 0; i < numIssues; i++ {
		<-d.Results()
	}

	if maxSeen > 2 {
		t.Errorf("maximum concurrent runs = %d, want <= 2", maxSeen)
	}
}

// TestDispatcher_Dispatch_ContextCancel verifies that cancelling the context
// during a run causes the run to be cleaned up and marked failed.
func TestDispatcher_Dispatch_ContextCancel(t *testing.T) {
	issue := claimedIssue(99)
	tr := newMockTracker(issue)
	ws := &mockWorkspace{path: "/tmp/ws-99"}

	started := make(chan struct{})
	cl := &mockHarnessClient{
		startFunc: func(_ context.Context, _, _ string) (string, error) {
			return "run-99", nil
		},
		statusFunc: func(ctx context.Context, _ string) (string, error) {
			// Signal that polling has started.
			select {
			case started <- struct{}{}:
			default:
			}
			// Block until context is cancelled.
			<-ctx.Done()
			return "", ctx.Err()
		},
	}

	cfg := fastDispatchConfig()
	ctx, cancel := context.WithCancel(context.Background())
	d := NewDispatcher(cfg, ws, tr, cl)

	if err := d.Dispatch(ctx, issue); err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}

	// Wait until status polling has begun, then cancel.
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("status polling never started")
	}
	cancel()

	result := <-d.Results()
	if result.Success {
		t.Error("expected Success=false after context cancel")
	}

	tr.mu.Lock()
	defer tr.mu.Unlock()
	if len(tr.failed) != 1 || tr.failed[0] != 99 {
		t.Errorf("tracker.failed = %v, want [99]", tr.failed)
	}
}

// TestDispatcher_Stall verifies that when RunStatus keeps returning "running"
// past StallTimeout, the issue is marked failed.
func TestDispatcher_Stall(t *testing.T) {
	issue := claimedIssue(55)
	tr := newMockTracker(issue)
	ws := &mockWorkspace{path: "/tmp/ws-55"}

	cl := &mockHarnessClient{
		startFunc: func(_ context.Context, _, _ string) (string, error) {
			return "run-55", nil
		},
		statusFunc: func(_ context.Context, _ string) (string, error) {
			// Always return "running" — simulates a stalled run.
			return "running", nil
		},
	}

	cfg := DispatchConfig{
		MaxConcurrent: 1,
		StallTimeout:  50 * time.Millisecond, // very short for tests
		HarnessURL:    "http://localhost:8080",
		PollInterval:  5 * time.Millisecond,
	}
	d := NewDispatcher(cfg, ws, tr, cl)

	ctx := context.Background()
	if err := d.Dispatch(ctx, issue); err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}

	result := <-d.Results()
	if result.Success {
		t.Error("expected Success=false for stalled run")
	}
	if result.Error == nil {
		t.Error("expected non-nil error for stalled run")
	}

	tr.mu.Lock()
	defer tr.mu.Unlock()
	if len(tr.failed) != 1 || tr.failed[0] != 55 {
		t.Errorf("tracker.failed = %v, want [55]", tr.failed)
	}
}

// TestDispatcher_Shutdown cancels all in-flight dispatches.
func TestDispatcher_Shutdown(t *testing.T) {
	const numIssues = 3
	issues := make([]*TrackedIssue, numIssues)
	for i := range issues {
		issues[i] = claimedIssue(200 + i)
	}
	tr := newMockTracker(issues...)
	ws := &mockWorkspace{path: "/tmp/ws"}

	block := make(chan struct{})
	cl := &mockHarnessClient{
		startFunc: func(_ context.Context, _, _ string) (string, error) {
			return "run-x", nil
		},
		statusFunc: func(ctx context.Context, _ string) (string, error) {
			select {
			case <-block:
				return "completed", nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		},
	}

	cfg := DispatchConfig{
		MaxConcurrent: numIssues,
		StallTimeout:  5 * time.Second,
		HarnessURL:    "http://localhost:8080",
		PollInterval:  5 * time.Millisecond,
	}
	d := NewDispatcher(cfg, ws, tr, cl)

	ctx := context.Background()
	for _, issue := range issues {
		if err := d.Dispatch(ctx, issue); err != nil {
			t.Fatalf("Dispatch returned error: %v", err)
		}
	}

	// Give goroutines a moment to start.
	time.Sleep(30 * time.Millisecond)

	// Shutdown should cancel all in-flight runs.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()
	d.Shutdown(shutdownCtx)

	// After shutdown, all running entries should be cleared.
	d.mu.Lock()
	runningCount := len(d.running)
	d.mu.Unlock()
	if runningCount != 0 {
		t.Errorf("running map has %d entries after Shutdown, want 0", runningCount)
	}
}

// TestDispatcher_Results_Channel verifies that Results() always returns the same channel.
func TestDispatcher_Results_Channel(t *testing.T) {
	d := NewDispatcher(fastDispatchConfig(), &mockWorkspace{}, newMockTracker(), &mockHarnessClient{})
	c1 := d.Results()
	c2 := d.Results()
	if c1 != c2 {
		t.Error("Results() should return the same channel on every call")
	}
}

// TestDispatcher_Dispatch_FailedRunStatus verifies that a "failed" status from
// harnessd causes the issue to be marked failed.
func TestDispatcher_Dispatch_FailedRunStatus(t *testing.T) {
	issue := claimedIssue(77)
	tr := newMockTracker(issue)
	ws := &mockWorkspace{path: "/tmp/ws-77"}

	cl := &mockHarnessClient{
		startFunc: func(_ context.Context, _, _ string) (string, error) {
			return "run-77", nil
		},
		statusFunc: func(_ context.Context, _ string) (string, error) {
			return "failed", nil
		},
	}

	cfg := fastDispatchConfig()
	d := NewDispatcher(cfg, ws, tr, cl)

	if err := d.Dispatch(context.Background(), issue); err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}

	result := <-d.Results()
	if result.Success {
		t.Error("expected Success=false for failed run status")
	}
	if result.Error == nil {
		t.Error("expected non-nil error")
	}

	tr.mu.Lock()
	defer tr.mu.Unlock()
	if len(tr.failed) != 1 || tr.failed[0] != 77 {
		t.Errorf("tracker.failed = %v, want [77]", tr.failed)
	}
	if len(tr.complete) != 0 {
		t.Errorf("tracker.complete = %v, want []", tr.complete)
	}
}
