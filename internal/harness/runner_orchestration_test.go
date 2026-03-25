package harness

import (
	"context"
	"errors"
	"testing"
	"time"

	htools "go-agent-harness/internal/harness/tools"
)

func TestSubmitInput_MapsBrokerValidationFailure(t *testing.T) {
	t.Parallel()

	broker := NewInMemoryAskUserQuestionBroker(nil)
	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{
		DefaultModel:  "gpt-4.1-mini",
		MaxSteps:      1,
		AskUserBroker: broker,
	})

	const runID = "run-submit-invalid"
	runner.mu.Lock()
	runner.runs[runID] = &runState{run: Run{ID: runID}}
	broker.pending[runID] = &pendingUserQuestion{
		pending: htools.AskUserQuestionPending{
			RunID:  runID,
			CallID: "call-1",
			Tool:   htools.AskUserQuestionToolName,
			Questions: []htools.AskUserQuestion{{
				Question: "Where next?",
				Header:   "Route",
				Options: []htools.AskUserQuestionOption{
					{Label: "Docs", Description: "Read docs"},
					{Label: "Code", Description: "Read code"},
				},
			}},
		},
		answerC: make(chan askUserSubmission, 1),
	}
	runner.mu.Unlock()

	err := runner.SubmitInput(runID, map[string]string{"Where next?": "Nope"})
	if !errors.Is(err, ErrInvalidRunInput) {
		t.Fatalf("SubmitInput error = %v, want %v", err, ErrInvalidRunInput)
	}
}

func TestSubmitInput_MapsMissingPendingQuestion(t *testing.T) {
	t.Parallel()

	broker := NewInMemoryAskUserQuestionBroker(nil)
	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{
		DefaultModel:  "gpt-4.1-mini",
		MaxSteps:      1,
		AskUserBroker: broker,
	})

	const runID = "run-submit-missing"
	runner.mu.Lock()
	runner.runs[runID] = &runState{run: Run{ID: runID}}
	runner.mu.Unlock()

	err := runner.SubmitInput(runID, map[string]string{"Where next?": "Docs"})
	if !errors.Is(err, ErrNoPendingInput) {
		t.Fatalf("SubmitInput error = %v, want %v", err, ErrNoPendingInput)
	}
}

func TestWaitForTerminalResult_UsesTerminalHistory(t *testing.T) {
	t.Parallel()

	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     1,
	})

	const runID = "run-history"
	runner.mu.Lock()
	runner.runs[runID] = &runState{run: Run{ID: runID, Output: "history output"}}
	runner.mu.Unlock()

	result, err := runner.waitForTerminalResult(context.Background(), runID, []Event{{
		Type: EventRunCompleted,
	}}, nil)
	if err != nil {
		t.Fatalf("waitForTerminalResult: %v", err)
	}
	if result.Output != "history output" {
		t.Fatalf("result.Output = %q, want %q", result.Output, "history output")
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q, want empty", result.Error)
	}
}

func TestWaitForTerminalResult_ReturnsOnStreamClose(t *testing.T) {
	t.Parallel()

	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     1,
	})

	const runID = "run-stream-close"
	runner.mu.Lock()
	runner.runs[runID] = &runState{run: Run{ID: runID, Output: "closed output"}}
	runner.mu.Unlock()

	stream := make(chan Event)
	close(stream)

	result, err := runner.waitForTerminalResult(context.Background(), runID, nil, stream)
	if err != nil {
		t.Fatalf("waitForTerminalResult: %v", err)
	}
	if result.Output != "closed output" {
		t.Fatalf("result.Output = %q, want %q", result.Output, "closed output")
	}
}

func TestWaitForTerminalResult_CancelledContextReturnsTerminalResultWhenRunAlreadyFinished(t *testing.T) {
	t.Parallel()

	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     1,
	})

	const runID = "run-finished-before-parent-cancel"
	runner.mu.Lock()
	runner.runs[runID] = &runState{run: Run{
		ID:     runID,
		Status: RunStatusCompleted,
		Output: "finished output",
	}}
	runner.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	stream := make(chan Event)
	defer close(stream)

	result, err := runner.waitForTerminalResult(ctx, runID, nil, stream)
	if err != nil {
		t.Fatalf("waitForTerminalResult error = %v, want nil", err)
	}
	if result.Output != "finished output" {
		t.Fatalf("result.Output = %q, want %q", result.Output, "finished output")
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q, want empty", result.Error)
	}
}

func TestRunForkedSkill_ReturnsFailedForkResult(t *testing.T) {
	t.Parallel()

	runner := NewRunner(&errorProvider{err: errors.New("provider exploded")}, NewRegistry(), RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     2,
	})

	result, err := runner.RunForkedSkill(context.Background(), htools.ForkConfig{
		Prompt:    "forked task",
		SkillName: "test-skill",
	})
	if err != nil {
		t.Fatalf("RunForkedSkill error = %v, want nil", err)
	}
	if result.Error == "" {
		t.Fatal("expected failed fork result to include terminal error text")
	}
	if result.Output != "" {
		t.Fatalf("result.Output = %q, want empty", result.Output)
	}
}

func TestRunPrompt_CancelsChildRunOnContextCancellation(t *testing.T) {
	t.Parallel()

	provider := newBlockingCancelProvider()
	t.Cleanup(func() {
		close(provider.releaseCh)
	})
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     2,
	})

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := runner.RunPrompt(ctx, "do some work")
		errCh <- err
	}()

	select {
	case <-provider.blockCh:
	case <-time.After(3 * time.Second):
		t.Fatal("provider never started blocking")
	}

	runID := singleRunID(t, runner)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RunPrompt error = %v, want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("RunPrompt did not return after parent cancellation")
	}

	waitForRunStatusWithin(t, runner, runID, RunStatusCancelled, 3*time.Second)
}

func TestRunForkedSkill_CancelsChildRunOnContextCancellation(t *testing.T) {
	t.Parallel()

	provider := newBlockingCancelProvider()
	t.Cleanup(func() {
		close(provider.releaseCh)
	})
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     2,
	})

	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan htools.ForkResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := runner.RunForkedSkill(ctx, htools.ForkConfig{
			Prompt:    "forked task",
			SkillName: "test-skill",
		})
		resultCh <- result
		errCh <- err
	}()

	select {
	case <-provider.blockCh:
	case <-time.After(3 * time.Second):
		t.Fatal("provider never started blocking")
	}

	runID := singleRunID(t, runner)
	cancel()

	var result htools.ForkResult
	select {
	case result = <-resultCh:
	case <-time.After(3 * time.Second):
		t.Fatal("RunForkedSkill did not return after parent cancellation")
	}

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RunForkedSkill error = %v, want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("RunForkedSkill error not returned")
	}

	if result.Error == "" {
		t.Fatal("expected fork result error text when parent context is cancelled")
	}

	waitForRunStatusWithin(t, runner, runID, RunStatusCancelled, 3*time.Second)
}

func singleRunID(t *testing.T, runner *Runner) string {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		runner.mu.RLock()
		runCount := len(runner.runs)
		var runID string
		for id := range runner.runs {
			runID = id
		}
		runner.mu.RUnlock()
		if runCount == 1 {
			return runID
		}
		time.Sleep(10 * time.Millisecond)
	}

	runner.mu.RLock()
	defer runner.mu.RUnlock()
	t.Fatalf("expected exactly one run, found %d", len(runner.runs))
	return ""
}

func waitForRunStatusWithin(t *testing.T, runner *Runner, runID string, want RunStatus, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		run, ok := runner.GetRun(runID)
		if !ok {
			t.Fatalf("run %q not found", runID)
		}
		if run.Status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	run, _ := runner.GetRun(runID)
	t.Fatalf("timed out waiting for run %q status %q, last status %q", runID, want, run.Status)
}
