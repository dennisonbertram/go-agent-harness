package harness

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// continuationProvider returns scripted CompletionResults on successive calls.
type continuationProvider struct {
	mu    sync.Mutex
	turns []CompletionResult
	calls int
}

func (p *continuationProvider) Complete(_ context.Context, _ CompletionRequest) (CompletionResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.calls >= len(p.turns) {
		return CompletionResult{Content: "done"}, nil
	}
	out := p.turns[p.calls]
	p.calls++
	return out, nil
}

// blockingProvider blocks until its blocker channel is closed.
type blockingProvider struct {
	blocker <-chan struct{}
}

func (p *blockingProvider) Complete(_ context.Context, _ CompletionRequest) (CompletionResult, error) {
	<-p.blocker
	return CompletionResult{Content: "unblocked"}, nil
}

// capturingContinuationProvider captures all CompletionRequests and returns scripted results.
type capturingContinuationProvider struct {
	mu       sync.Mutex
	requests []CompletionRequest
	turns    []CompletionResult
	calls    int
}

func (p *capturingContinuationProvider) Complete(_ context.Context, req CompletionRequest) (CompletionResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, req)
	if p.calls >= len(p.turns) {
		return CompletionResult{Content: "done"}, nil
	}
	out := p.turns[p.calls]
	p.calls++
	return out, nil
}

func (p *capturingContinuationProvider) captured() []CompletionRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]CompletionRequest(nil), p.requests...)
}

// alwaysErrorProvider always returns an error.
type alwaysErrorProvider struct{}

func (p *alwaysErrorProvider) Complete(_ context.Context, _ CompletionRequest) (CompletionResult, error) {
	return CompletionResult{}, errors.New("provider always fails")
}

// waitForStatusCont polls GetRun until one of the target statuses is reached.
func waitForStatusCont(t *testing.T, r *Runner, runID string, targets ...RunStatus) RunStatus {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for {
		run, ok := r.GetRun(runID)
		if !ok {
			t.Fatalf("run %q not found", runID)
		}
		for _, target := range targets {
			if run.Status == target {
				return run.Status
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for status %v, last status: %s", targets, run.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestContinueRunBasic verifies that a completed run can be continued with a
// new message, producing a second run that shares the same conversation_id.
func TestContinueRunBasic(t *testing.T) {
	t.Parallel()

	prov := &continuationProvider{
		turns: []CompletionResult{
			{Content: "first response"},
			{Content: "second response"},
		},
	}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            4,
	})

	run1, err := runner.StartRun(RunRequest{Prompt: "initial prompt"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)
	run1Final, _ := runner.GetRun(run1.ID)
	if run1Final.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %s", run1Final.Status)
	}
	if run1Final.Output != "first response" {
		t.Fatalf("expected 'first response', got %q", run1Final.Output)
	}

	run2, err := runner.ContinueRun(run1.ID, "follow-up message")
	if err != nil {
		t.Fatalf("ContinueRun: %v", err)
	}

	// Continuation must produce a new run ID.
	if run2.ID == run1.ID {
		t.Fatalf("expected a new run ID, got same as original: %s", run1.ID)
	}
	// Continuation must share the conversation ID.
	if run2.ConversationID != run1Final.ConversationID {
		t.Fatalf("expected same conversation_id %q, got %q", run1Final.ConversationID, run2.ConversationID)
	}

	waitForStatusCont(t, runner, run2.ID, RunStatusCompleted, RunStatusFailed)
	run2Final, _ := runner.GetRun(run2.ID)
	if run2Final.Status != RunStatusCompleted {
		t.Fatalf("expected run2 completed, got %s", run2Final.Status)
	}
	if run2Final.Output != "second response" {
		t.Fatalf("expected 'second response', got %q", run2Final.Output)
	}
}

// TestContinueRunNotFound verifies ErrRunNotFound when the run does not exist.
func TestContinueRunNotFound(t *testing.T) {
	t.Parallel()

	runner := NewRunner(&continuationProvider{}, NewRegistry(), RunnerConfig{MaxSteps: 2})
	_, err := runner.ContinueRun("nonexistent-run-id", "hello")
	if err == nil {
		t.Fatal("expected error for non-existent run, got nil")
	}
	if !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("expected ErrRunNotFound, got %v", err)
	}
}

// TestContinueRunWhileRunning verifies ErrRunNotCompleted when the run is still
// in progress.
func TestContinueRunWhileRunning(t *testing.T) {
	t.Parallel()

	blocker := make(chan struct{})
	prov := &blockingProvider{blocker: blocker}

	runner := NewRunner(prov, NewRegistry(), RunnerConfig{MaxSteps: 2})
	run, err := runner.StartRun(RunRequest{Prompt: "blocking prompt"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Wait until the run is running.
	deadline := time.Now().Add(2 * time.Second)
	for {
		r, _ := runner.GetRun(run.ID)
		if r.Status == RunStatusRunning {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for running status")
		}
		time.Sleep(5 * time.Millisecond)
	}

	_, err = runner.ContinueRun(run.ID, "concurrent continue")
	if err == nil {
		t.Fatal("expected error when continuing a running run")
	}
	if !errors.Is(err, ErrRunNotCompleted) {
		t.Fatalf("expected ErrRunNotCompleted, got %v", err)
	}

	// Unblock so the goroutine can finish.
	close(blocker)
	waitForStatusCont(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)
}

// TestContinueRunEmptyMessage verifies that an empty follow-up message is rejected.
func TestContinueRunEmptyMessage(t *testing.T) {
	t.Parallel()

	prov := &continuationProvider{turns: []CompletionResult{{Content: "done"}}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{MaxSteps: 2})

	run, err := runner.StartRun(RunRequest{Prompt: "initial"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatusCont(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	_, err = runner.ContinueRun(run.ID, "")
	if err == nil {
		t.Fatal("expected error for empty message, got nil")
	}
}

// TestContinueRunCarriesConversationHistory verifies that the continuation
// request to the provider includes prior user and assistant messages.
func TestContinueRunCarriesConversationHistory(t *testing.T) {
	t.Parallel()

	prov := &capturingContinuationProvider{
		turns: []CompletionResult{
			{Content: "first answer"},
			{Content: "second answer"},
		},
	}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "system",
		MaxSteps:            4,
	})

	run1, err := runner.StartRun(RunRequest{
		Prompt:         "what is 1+1?",
		ConversationID: "conv-history-test",
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

	run2, err := runner.ContinueRun(run1.ID, "and what is 2+2?")
	if err != nil {
		t.Fatalf("ContinueRun: %v", err)
	}
	waitForStatusCont(t, runner, run2.ID, RunStatusCompleted, RunStatusFailed)

	reqs := prov.captured()
	if len(reqs) < 2 {
		t.Fatalf("expected at least 2 provider calls, got %d", len(reqs))
	}

	// The second request should include the prior user + assistant messages.
	secondReq := reqs[1]
	found1 := false
	foundAssistant := false
	for _, msg := range secondReq.Messages {
		if msg.Role == "user" && msg.Content == "what is 1+1?" {
			found1 = true
		}
		if msg.Role == "assistant" && msg.Content == "first answer" {
			foundAssistant = true
		}
	}
	if !found1 {
		t.Fatalf("expected prior user message in second request, messages: %+v", secondReq.Messages)
	}
	if !foundAssistant {
		t.Fatalf("expected prior assistant message in second request, messages: %+v", secondReq.Messages)
	}
}

// TestContinueRunConcurrencyRace verifies no data race and that exactly one of
// N concurrent ContinueRun calls on the same completed run succeeds.
func TestContinueRunConcurrencyRace(t *testing.T) {
	t.Parallel()

	prov := &continuationProvider{
		turns: []CompletionResult{
			{Content: "r1"},
			{Content: "r2"},
			{Content: "r3"},
		},
	}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{MaxSteps: 2})

	run, err := runner.StartRun(RunRequest{Prompt: "start"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatusCont(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	const n = 5
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			_, e := runner.ContinueRun(run.ID, "concurrent message")
			errs[i] = e
		}()
	}
	wg.Wait()

	successes := 0
	for _, e := range errs {
		if e == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly 1 successful ContinueRun, got %d", successes)
	}
}

// TestContinueRunFailedRun verifies that continuing a failed run returns
// ErrRunNotCompleted.
func TestContinueRunFailedRun(t *testing.T) {
	t.Parallel()

	runner := NewRunner(&alwaysErrorProvider{}, NewRegistry(), RunnerConfig{MaxSteps: 2})

	run, err := runner.StartRun(RunRequest{Prompt: "will fail"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatusCont(t, runner, run.ID, RunStatusFailed)

	_, err = runner.ContinueRun(run.ID, "try to continue")
	if err == nil {
		t.Fatal("expected error when continuing a failed run")
	}
	if !errors.Is(err, ErrRunNotCompleted) {
		t.Fatalf("expected ErrRunNotCompleted, got %v", err)
	}
}
