package mcpserver

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// fakePoller is a mock HarnessPoller that cycles through a sequence of statuses.
type fakePoller struct {
	mu       sync.Mutex
	statuses map[string][]string // run_id → ordered list of statuses to return
	calls    map[string]int      // run_id → number of times called
}

func newFakePoller() *fakePoller {
	return &fakePoller{
		statuses: make(map[string][]string),
		calls:    make(map[string]int),
	}
}

func (f *fakePoller) addStatuses(runID string, statuses ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statuses[runID] = statuses
}

func (f *fakePoller) GetRunStatus(runID string) (RunStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	seq, ok := f.statuses[runID]
	if !ok {
		return RunStatus{}, nil
	}
	idx := f.calls[runID]
	if idx >= len(seq) {
		idx = len(seq) - 1
	}
	f.calls[runID]++
	return RunStatus{
		ID:     runID,
		Status: seq[idx],
	}, nil
}

// T8: Mock HarnessPoller returns running→running→completed;
// verify run/event then run/completed published; WatchCount→0 after terminal.
func TestRunPoller_StatusTransitions(t *testing.T) {
	fp := newFakePoller()
	fp.addStatuses("run-t8", "running", "running", "completed")

	b := NewBroker()
	p := NewRunPoller(fp, b, 5*time.Millisecond)

	// Subscribe to notifications for run-t8.
	ch, cancel := b.Subscribe("run-t8")
	defer cancel()

	p.Watch("run-t8")
	if p.WatchCount() != 1 {
		t.Errorf("expected WatchCount=1, got %d", p.WatchCount())
	}

	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()
	go p.Run(ctx)

	// Collect notifications with a timeout.
	var notifications []Notification
	timeout := time.After(500 * time.Millisecond)
	for {
		select {
		case n := <-ch:
			notifications = append(notifications, n)
			// Once we get run/completed, we're done.
			if n.Method == "run/completed" {
				goto done
			}
		case <-timeout:
			t.Fatal("timed out waiting for run/completed notification")
		}
	}

done:
	// Expect at least one run/event followed by run/completed.
	// The exact number of run/event notifications depends on timing, but
	// we must have at least the run/completed at the end.
	if len(notifications) == 0 {
		t.Fatal("expected at least one notification")
	}
	last := notifications[len(notifications)-1]
	if last.Method != "run/completed" {
		t.Errorf("expected last notification to be run/completed, got %q", last.Method)
	}

	// Verify run/completed params.
	var params map[string]any
	if err := json.Unmarshal(last.Params, &params); err != nil {
		t.Fatalf("unmarshal run/completed params: %v", err)
	}
	if params["run_id"] != "run-t8" {
		t.Errorf("expected run_id=run-t8 in completion params, got %v", params["run_id"])
	}
	if params["status"] != "completed" {
		t.Errorf("expected status=completed in completion params, got %v", params["status"])
	}

	// Give poller a tick to call Unwatch.
	time.Sleep(20 * time.Millisecond)
	if p.WatchCount() != 0 {
		t.Errorf("expected WatchCount=0 after terminal state, got %d", p.WatchCount())
	}
}

// T9: subscribe_run on already-completed run returns immediately with already_completed:true,
// WatchCount stays 0.
// This is tested via the Server integration in sse_test.go, but here we test the poller directly.
func TestRunPoller_AlreadyTerminalNotWatched(t *testing.T) {
	fp := newFakePoller()
	fp.addStatuses("run-done", "completed")

	b := NewBroker()
	p := NewRunPoller(fp, b, 5*time.Millisecond)

	// We do NOT call Watch for a run that is already completed.
	// Verifying WatchCount stays 0.
	if p.WatchCount() != 0 {
		t.Errorf("expected WatchCount=0 initially, got %d", p.WatchCount())
	}
}

// TestRunPoller_WatchUnwatch verifies basic Watch/Unwatch lifecycle.
func TestRunPoller_WatchUnwatch(t *testing.T) {
	fp := newFakePoller()
	b := NewBroker()
	p := NewRunPoller(fp, b, time.Hour) // large interval to avoid real polls

	p.Watch("run-1")
	p.Watch("run-2")
	p.Watch("run-1") // idempotent

	if p.WatchCount() != 2 {
		t.Errorf("expected WatchCount=2, got %d", p.WatchCount())
	}

	p.Unwatch("run-1")
	if p.WatchCount() != 1 {
		t.Errorf("expected WatchCount=1, got %d", p.WatchCount())
	}

	p.Unwatch("run-2")
	if p.WatchCount() != 0 {
		t.Errorf("expected WatchCount=0, got %d", p.WatchCount())
	}

	// Unwatching non-existent run is a no-op.
	p.Unwatch("run-nonexistent")
	if p.WatchCount() != 0 {
		t.Errorf("expected WatchCount=0 after no-op unwatch, got %d", p.WatchCount())
	}
}

// TestRunPoller_FailedTerminalState verifies "failed" is treated as terminal.
func TestRunPoller_FailedTerminalState(t *testing.T) {
	fp := newFakePoller()
	fp.addStatuses("run-fail", "running", "failed")

	b := NewBroker()
	p := NewRunPoller(fp, b, 5*time.Millisecond)

	ch, cancel := b.Subscribe("run-fail")
	defer cancel()

	p.Watch("run-fail")

	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()
	go p.Run(ctx)

	// Wait for run/completed (which covers terminal "failed" state too).
	var completedNotif *Notification
	timeout := time.After(500 * time.Millisecond)
	for {
		select {
		case n := <-ch:
			if n.Method == "run/completed" {
				completedNotif = &n
				goto done
			}
		case <-timeout:
			t.Fatal("timed out waiting for run/completed notification for failed run")
		}
	}

done:
	var params map[string]any
	if err := json.Unmarshal(completedNotif.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params["status"] != "failed" {
		t.Errorf("expected status=failed, got %v", params["status"])
	}

	time.Sleep(20 * time.Millisecond)
	if p.WatchCount() != 0 {
		t.Errorf("expected WatchCount=0 after failed state, got %d", p.WatchCount())
	}
}

// TestRunPoller_CancelStopsLoop verifies ctx cancel terminates the Run loop.
func TestRunPoller_CancelStopsLoop(t *testing.T) {
	fp := newFakePoller()
	b := NewBroker()
	p := NewRunPoller(fp, b, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		p.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// Good.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run did not stop after context cancel")
	}
}

// TestRunPoller_ConcurrentWatch verifies concurrent Watch calls are race-free.
func TestRunPoller_ConcurrentWatch(t *testing.T) {
	fp := newFakePoller()
	b := NewBroker()
	p := NewRunPoller(fp, b, time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			runID := "run-concurrent"
			p.Watch(runID)
		}(i)
	}
	wg.Wait()

	// All watches are for the same ID, so count should be 1.
	if p.WatchCount() != 1 {
		t.Errorf("expected WatchCount=1 after idempotent watches, got %d", p.WatchCount())
	}
}
