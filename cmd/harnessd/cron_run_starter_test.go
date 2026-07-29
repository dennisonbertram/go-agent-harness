package main

import (
	"strings"
	"testing"

	"go-agent-harness/internal/harness"
)

func TestCronRunStarterRequiresBoundRunner(t *testing.T) {
	_, err := (&cronRunStarter{}).StartRun("prompt", "conversation")
	if err == nil || !strings.Contains(err.Error(), "not yet initialized") {
		t.Fatalf("StartRun error = %v", err)
	}
}

func TestCronRunStarterStartsHarnessRun(t *testing.T) {
	runner := harness.NewRunner(&noopProvider{}, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     1,
	})
	starter := &cronRunStarter{runner: runner}

	runID, err := starter.StartRun("scheduled work", "conv-cron")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if runID == "" {
		t.Fatal("StartRun returned an empty run id")
	}
	run, ok := runner.GetRun(runID)
	if !ok {
		t.Fatalf("runner does not know started run %q", runID)
	}
	if run.ConversationID != "conv-cron" {
		t.Fatalf("conversation id = %q, want conv-cron", run.ConversationID)
	}
}
