package harness

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestSchemaVersionOnAllEvents verifies that every event emitted by the runner
// carries schema_version == EventSchemaVersion in its payload.
func TestSchemaVersionOnAllEvents(t *testing.T) {
	t.Parallel()

	prov := &stubProvider{turns: []CompletionResult{
		{Content: "done"},
	}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	events := collectEvents(t, runner, run.ID)
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}

	for _, evt := range events {
		sv, ok := evt.Payload["schema_version"]
		if !ok {
			t.Errorf("event %s (type=%s) missing schema_version in payload", evt.ID, evt.Type)
			continue
		}
		if sv != EventSchemaVersion {
			t.Errorf("event %s (type=%s) schema_version=%v, want %q", evt.ID, evt.Type, sv, EventSchemaVersion)
		}
	}
}

// TestConversationIDOnAllEvents verifies that every event carries a
// conversation_id field matching the run's ConversationID.
func TestConversationIDOnAllEvents(t *testing.T) {
	t.Parallel()

	prov := &stubProvider{turns: []CompletionResult{
		{Content: "done"},
	}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	events := collectEvents(t, runner, run.ID)
	runFinal, _ := runner.GetRun(run.ID)

	for _, evt := range events {
		cid, ok := evt.Payload["conversation_id"]
		if !ok {
			t.Errorf("event %s (type=%s) missing conversation_id", evt.ID, evt.Type)
			continue
		}
		if cid != runFinal.ConversationID {
			t.Errorf("event %s conversation_id=%v, want %q", evt.ID, cid, runFinal.ConversationID)
		}
	}
}

// TestConversationIDStableAcrossContinue verifies that when a run is continued,
// the conversation_id in events from both runs is identical.
func TestConversationIDStableAcrossContinue(t *testing.T) {
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

	run1, err := runner.StartRun(RunRequest{Prompt: "initial"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

	run2, err := runner.ContinueRun(run1.ID, "follow up")
	if err != nil {
		t.Fatalf("ContinueRun: %v", err)
	}
	waitForStatusCont(t, runner, run2.ID, RunStatusCompleted, RunStatusFailed)

	events1 := collectEvents(t, runner, run1.ID)
	events2 := collectEvents(t, runner, run2.ID)

	// Extract conversation_id from first run's events.
	var convID1 string
	for _, evt := range events1 {
		if cid, ok := evt.Payload["conversation_id"].(string); ok && cid != "" {
			convID1 = cid
			break
		}
	}
	if convID1 == "" {
		t.Fatal("run1 events have no conversation_id")
	}

	// Verify all run2 events have the same conversation_id.
	for _, evt := range events2 {
		cid, ok := evt.Payload["conversation_id"].(string)
		if !ok {
			t.Errorf("run2 event %s (type=%s) missing conversation_id", evt.ID, evt.Type)
			continue
		}
		if cid != convID1 {
			t.Errorf("run2 event %s conversation_id=%q, want %q (same as run1)", evt.ID, cid, convID1)
		}
	}

	// Verify run2's run.started event includes previous_run_id.
	for _, evt := range events2 {
		if evt.Type == EventRunStarted {
			prevID, ok := evt.Payload["previous_run_id"].(string)
			if !ok || prevID == "" {
				t.Errorf("run2 run.started event missing previous_run_id")
			} else if prevID != run1.ID {
				t.Errorf("previous_run_id=%q, want %q", prevID, run1.ID)
			}
			break
		}
	}
}

// TestStepInAllEventsAfterRunStarted verifies that a "step" field is present
// on all events emitted after run.started.
func TestStepInAllEventsAfterRunStarted(t *testing.T) {
	t.Parallel()

	prov := &stubProvider{turns: []CompletionResult{
		{Content: "done"},
	}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	events := collectEvents(t, runner, run.ID)

	sawRunStarted := false
	for _, evt := range events {
		if evt.Type == EventRunStarted {
			sawRunStarted = true
			// run.started itself gets step=0 (before the loop)
			continue
		}
		if !sawRunStarted {
			continue
		}
		if _, ok := evt.Payload["step"]; !ok {
			t.Errorf("event %s (type=%s) after run.started missing step field", evt.ID, evt.Type)
		}
	}
}

// TestCorrelationFieldsInRollout verifies that schema_version and
// conversation_id appear in the JSONL rollout file.
func TestCorrelationFieldsInRollout(t *testing.T) {
	t.Parallel()

	rolloutDir := t.TempDir()
	prov := &stubProvider{turns: []CompletionResult{
		{Content: "done"},
	}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            2,
		RolloutDir:          rolloutDir,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	// Find the rollout file.
	dateDir := filepath.Join(rolloutDir, time.Now().UTC().Format("2006-01-02"))
	jsonlPath := filepath.Join(dateDir, run.ID+".jsonl")
	f, err := os.Open(jsonlPath)
	if err != nil {
		t.Fatalf("open rollout file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		var entry struct {
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("line %d: parse JSONL: %v", lineNum, err)
		}
		if entry.Data == nil {
			continue
		}
		if _, ok := entry.Data["schema_version"]; !ok {
			t.Errorf("line %d: missing schema_version in rollout data", lineNum)
		}
		if _, ok := entry.Data["conversation_id"]; !ok {
			t.Errorf("line %d: missing conversation_id in rollout data", lineNum)
		}
	}
	if lineNum == 0 {
		t.Fatal("rollout file is empty")
	}
}

// waitForStatus polls GetRun until one of the target statuses is reached.
func waitForStatus(t *testing.T, r *Runner, runID string, targets ...RunStatus) RunStatus {
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

// collectEvents returns all events for a run via Subscribe.
func collectEvents(t *testing.T, r *Runner, runID string) []Event {
	t.Helper()
	history, _, cancel, err := r.Subscribe(runID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	cancel()
	return history
}
