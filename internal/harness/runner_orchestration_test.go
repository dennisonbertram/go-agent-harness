package harness

import (
	"context"
	"errors"
	"testing"

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
