package symphd

import (
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// RetryPolicy tests
// ---------------------------------------------------------------------------

func TestDefaultRetryPolicy(t *testing.T) {
	p := DefaultRetryPolicy()
	if p.MaxAttempts != 5 {
		t.Errorf("MaxAttempts: want 5, got %d", p.MaxAttempts)
	}
	if p.BaseDelayMs != 10000 {
		t.Errorf("BaseDelayMs: want 10000, got %d", p.BaseDelayMs)
	}
	if p.MaxDelayMs != 300000 {
		t.Errorf("MaxDelayMs: want 300000, got %d", p.MaxDelayMs)
	}
}

func TestRetryPolicy_ShouldRetry_BelowMax(t *testing.T) {
	p := DefaultRetryPolicy() // MaxAttempts = 5
	for attempts := 0; attempts < 5; attempts++ {
		if !p.ShouldRetry(attempts) {
			t.Errorf("ShouldRetry(%d): want true, got false", attempts)
		}
	}
}

func TestRetryPolicy_ShouldRetry_AtMax(t *testing.T) {
	p := DefaultRetryPolicy()
	if p.ShouldRetry(p.MaxAttempts) {
		t.Errorf("ShouldRetry(%d): want false, got true", p.MaxAttempts)
	}
}

func TestRetryPolicy_ShouldRetry_AboveMax(t *testing.T) {
	p := DefaultRetryPolicy()
	if p.ShouldRetry(p.MaxAttempts + 10) {
		t.Errorf("ShouldRetry(%d): want false, got true", p.MaxAttempts+10)
	}
}

func TestRetryPolicy_BackoffDelay_Attempt1(t *testing.T) {
	p := DefaultRetryPolicy()
	got := p.BackoffDelay(1)
	want := 10 * time.Second
	if got != want {
		t.Errorf("BackoffDelay(1): want %v, got %v", want, got)
	}
}

func TestRetryPolicy_BackoffDelay_Attempt2(t *testing.T) {
	p := DefaultRetryPolicy()
	got := p.BackoffDelay(2)
	want := 20 * time.Second
	if got != want {
		t.Errorf("BackoffDelay(2): want %v, got %v", want, got)
	}
}

func TestRetryPolicy_BackoffDelay_Attempt3(t *testing.T) {
	p := DefaultRetryPolicy()
	got := p.BackoffDelay(3)
	want := 40 * time.Second
	if got != want {
		t.Errorf("BackoffDelay(3): want %v, got %v", want, got)
	}
}

func TestRetryPolicy_BackoffDelay_LargeAttempt(t *testing.T) {
	p := DefaultRetryPolicy()
	got := p.BackoffDelay(100)
	want := time.Duration(p.MaxDelayMs) * time.Millisecond
	if got != want {
		t.Errorf("BackoffDelay(100): want %v (cap), got %v", want, got)
	}
}

func TestRetryPolicy_BackoffDelay_ZeroAttempt(t *testing.T) {
	p := DefaultRetryPolicy()
	got := p.BackoffDelay(0)
	want := 10 * time.Second // treated as attempt 1
	if got != want {
		t.Errorf("BackoffDelay(0): want %v (treated as 1), got %v", want, got)
	}
}

func TestRetryPolicy_BackoffDelay_NegativeAttempt(t *testing.T) {
	p := DefaultRetryPolicy()
	got := p.BackoffDelay(-5)
	want := 10 * time.Second // treated as attempt 1
	if got != want {
		t.Errorf("BackoffDelay(-5): want %v (treated as 1), got %v", want, got)
	}
}

// TestRetryPolicy_BackoffDelay_ExactFormula verifies the formula
// min(10000 * 2^(attempt-1), MaxDelayMs) for attempts 1–5.
func TestRetryPolicy_BackoffDelay_ExactFormula(t *testing.T) {
	p := RetryPolicy{
		MaxAttempts: 5,
		BaseDelayMs: 10000,
		MaxDelayMs:  300000,
	}

	cases := []struct {
		attempt int
		wantMs  int
	}{
		{1, 10000},  // 10000 * 2^0 = 10000
		{2, 20000},  // 10000 * 2^1 = 20000
		{3, 40000},  // 10000 * 2^2 = 40000
		{4, 80000},  // 10000 * 2^3 = 80000
		{5, 160000}, // 10000 * 2^4 = 160000
	}
	for _, tc := range cases {
		got := p.BackoffDelay(tc.attempt)
		want := time.Duration(tc.wantMs) * time.Millisecond
		if got != want {
			t.Errorf("BackoffDelay(%d): want %v, got %v", tc.attempt, want, got)
		}
	}
}

// ---------------------------------------------------------------------------
// DeadLetterQueue tests
// ---------------------------------------------------------------------------

func TestDeadLetterQueue_Add(t *testing.T) {
	q := NewDeadLetterQueue()
	issue := &TrackedIssue{Number: 42, Title: "fix bug", Attempts: 5}
	q.Add(issue, "run timed out")

	if q.Len() != 1 {
		t.Fatalf("Len: want 1, got %d", q.Len())
	}
	items := q.Items()
	if len(items) != 1 {
		t.Fatalf("Items len: want 1, got %d", len(items))
	}
	dl := items[0]
	if dl.IssueNumber != 42 {
		t.Errorf("IssueNumber: want 42, got %d", dl.IssueNumber)
	}
	if dl.Title != "fix bug" {
		t.Errorf("Title: want %q, got %q", "fix bug", dl.Title)
	}
	if dl.Attempts != 5 {
		t.Errorf("Attempts: want 5, got %d", dl.Attempts)
	}
	if dl.LastError != "run timed out" {
		t.Errorf("LastError: want %q, got %q", "run timed out", dl.LastError)
	}
	if dl.ExhaustedAt.IsZero() {
		t.Error("ExhaustedAt should not be zero")
	}
}

func TestDeadLetterQueue_Items_ReturnsAll(t *testing.T) {
	q := NewDeadLetterQueue()
	for i := 1; i <= 3; i++ {
		q.Add(&TrackedIssue{Number: i, Title: "issue", Attempts: 5}, "err")
	}
	items := q.Items()
	if len(items) != 3 {
		t.Fatalf("Items len: want 3, got %d", len(items))
	}
}

func TestDeadLetterQueue_Items_ReturnsCopy(t *testing.T) {
	q := NewDeadLetterQueue()
	q.Add(&TrackedIssue{Number: 1, Title: "original", Attempts: 5}, "err")

	items := q.Items()
	items[0].Title = "mutated"

	// Internal state must not be affected.
	items2 := q.Items()
	if items2[0].Title == "mutated" {
		t.Error("Items() returned a reference to internal state; mutations affected the queue")
	}
}

func TestDeadLetterQueue_Len(t *testing.T) {
	q := NewDeadLetterQueue()
	if q.Len() != 0 {
		t.Errorf("Len (empty): want 0, got %d", q.Len())
	}
	q.Add(&TrackedIssue{Number: 1, Title: "a", Attempts: 5}, "e")
	q.Add(&TrackedIssue{Number: 2, Title: "b", Attempts: 5}, "e")
	if q.Len() != 2 {
		t.Errorf("Len (after 2 adds): want 2, got %d", q.Len())
	}
}

func TestDeadLetterQueue_Concurrent(t *testing.T) {
	q := NewDeadLetterQueue()
	var wg sync.WaitGroup
	const goroutines = 50

	// Concurrent adds.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			q.Add(&TrackedIssue{Number: n, Title: "issue", Attempts: 5}, "err")
		}(i)
	}

	// Concurrent reads.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = q.Items()
			_ = q.Len()
		}()
	}

	wg.Wait()
	if q.Len() != goroutines {
		t.Errorf("Len after concurrent adds: want %d, got %d", goroutines, q.Len())
	}
}

// ---------------------------------------------------------------------------
// Tracker.Reset tests
// ---------------------------------------------------------------------------

func TestTracker_Reset_FromFailed(t *testing.T) {
	tr := newTestTracker()
	tr.addIssue(1, "fix-me", ClaimStateRunning)

	if err := tr.Fail(1, "boom"); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if err := tr.Reset(1); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	issues := tr.Issues()
	if len(issues) != 1 {
		t.Fatalf("Issues len: want 1, got %d", len(issues))
	}
	got := issues[0]
	if got.ClaimState != ClaimStateUnclaimed {
		t.Errorf("ClaimState: want unclaimed, got %s", got.ClaimState)
	}
	if got.Attempts != 2 {
		// Started at 1 (set in addIssue), Reset increments to 2.
		t.Errorf("Attempts: want 2, got %d", got.Attempts)
	}
}

func TestTracker_Reset_NotFailed_Error(t *testing.T) {
	tr := newTestTracker()
	tr.addIssue(2, "other", ClaimStateRunning)

	err := tr.Reset(2) // still running — must fail
	if err == nil {
		t.Error("Reset on Running issue: want error, got nil")
	}
}

func TestTracker_Reset_Unknown_Error(t *testing.T) {
	tr := newTestTracker()
	err := tr.Reset(999)
	if err == nil {
		t.Error("Reset on unknown issue: want error, got nil")
	}
}

// ---------------------------------------------------------------------------
// helpers for tracker tests
// ---------------------------------------------------------------------------

// testTracker is a thin wrapper around GitHubTracker that exposes addIssue for
// test setup.
type testTracker struct {
	*GitHubTracker
}

func newTestTracker() *testTracker {
	return &testTracker{
		GitHubTracker: &GitHubTracker{
			issues: make(map[int]*TrackedIssue),
		},
	}
}

func (tt *testTracker) addIssue(number int, title string, state ClaimState) {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	tt.issues[number] = &TrackedIssue{
		Number:     number,
		Title:      title,
		ClaimState: state,
		Attempts:   1,
	}
}
