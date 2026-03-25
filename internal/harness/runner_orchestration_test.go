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
			t.Fatalf("RunPrompt error = %v, want %v", err, context.Canceled)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("RunPrompt did not return after context cancellation")
	}

	waitForObservedRunStatus(t, runner, runID, RunStatusCancelled)
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
		result, err := runner.RunForkedSkill(ctx, htools.ForkConfig{Prompt: "forked task"})
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

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RunForkedSkill error = %v, want %v", err, context.Canceled)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("RunForkedSkill did not return after context cancellation")
	}

	result := <-resultCh
	if result.Error == "" {
		t.Fatal("expected fork result to contain cancellation error text")
	}

	waitForObservedRunStatus(t, runner, runID, RunStatusCancelled)
}

func singleRunID(t *testing.T, runner *Runner) string {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		runner.mu.RLock()
		if len(runner.runs) == 1 {
			for runID := range runner.runs {
				runner.mu.RUnlock()
				return runID
			}
		}
		runner.mu.RUnlock()
		time.Sleep(10 * time.Millisecond)
	}

	runner.mu.RLock()
	defer runner.mu.RUnlock()
	t.Fatalf("expected exactly one run, found %d", len(runner.runs))
	return ""
}

func waitForObservedRunStatus(t *testing.T, runner *Runner, runID string, want RunStatus) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
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

	run, ok := runner.GetRun(runID)
	if !ok {
		t.Fatalf("run %q not found", runID)
	}
	t.Fatalf("run %q status = %q, want %q", runID, run.Status, want)
}
