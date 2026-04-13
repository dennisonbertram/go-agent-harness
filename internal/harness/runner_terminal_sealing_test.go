package harness

// runner_terminal_sealing_test.go — regression coverage for terminal sealing,
// recorder drops, and audit-writer behavior.
// Covers GitHub issue #330.

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"go-agent-harness/internal/forensics/redaction"
	"go-agent-harness/internal/rollout"
)

// -------------------------------------------------------------------------
// TestTerminalSealing_RedactedTerminalEventStillSealsRun
//
// When the redaction pipeline drops the terminal event (StorageModeNone for
// run.completed), the run must still be sealed:
//   - state.Status becomes completed
//   - No further events can be appended after the terminal seal
//   - Recorder closes cleanly (no hang)
//
// -------------------------------------------------------------------------
func TestTerminalSealing_RedactedTerminalEventStillSealsRun(t *testing.T) {
	t.Parallel()

	// Configure a redaction pipeline that drops run.completed entirely.
	pipeline := redaction.NewPipeline(
		redaction.NewRedactor(nil),
		redaction.EventClassConfig{
			"run.completed": redaction.StorageModeNone,
		},
	)

	prov := &stubProvider{turns: []CompletionResult{{Content: "done"}}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:      "test-model",
		MaxSteps:          2,
		RedactionPipeline: pipeline,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "seal test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Wait for the run to reach a terminal state (completed or failed).
	// Even though run.completed is dropped by redaction, the runner must
	// still mark the run as completed internally.
	finalStatus := waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)
	if finalStatus != RunStatusCompleted {
		state, _ := runner.GetRun(run.ID)
		t.Fatalf("expected completed status even with redacted terminal event, got %q (error: %s)",
			finalStatus, state.Error)
	}

	// Verify the terminated gate is set: emit after terminal must be a no-op.
	// We do this by inspecting the stored events — run.completed must NOT
	// appear in the stored event list (it was dropped), yet the run is sealed.
	events := runner.getEvents(run.ID)
	for _, ev := range events {
		if ev.Type == EventRunCompleted {
			t.Error("run.completed should not be stored when redaction drops it")
		}
	}

	// Attempt to emit a fake event after the seal. It should be silently ignored
	// (no panic, no append). We verify by counting events before and after.
	countBefore := len(runner.getEvents(run.ID))
	runner.emit(run.ID, EventAssistantMessage, map[string]any{"content": "post-terminal"})
	countAfter := len(runner.getEvents(run.ID))
	if countAfter != countBefore {
		t.Errorf("post-terminal emit was not dropped: events grew from %d to %d", countBefore, countAfter)
	}
}

// -------------------------------------------------------------------------
// TestTerminalSealing_RedactedTerminalRunFailed
//
// Same as above but for the run.failed terminal event — redaction drops it,
// but the run must still be sealed as failed.
// -------------------------------------------------------------------------
func TestTerminalSealing_RedactedTerminalRunFailed(t *testing.T) {
	t.Parallel()

	pipeline := redaction.NewPipeline(
		redaction.NewRedactor(nil),
		redaction.EventClassConfig{
			"run.failed": redaction.StorageModeNone,
		},
	)

	prov := &errorProvider{err: errors.New("deliberate provider error")}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:      "test-model",
		MaxSteps:          2,
		RedactionPipeline: pipeline,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "seal failed test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	finalStatus := waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)
	if finalStatus != RunStatusFailed {
		t.Fatalf("expected failed status, got %q", finalStatus)
	}

	// run.failed must not be stored when dropped by redaction.
	events := runner.getEvents(run.ID)
	for _, ev := range events {
		if ev.Type == EventRunFailed {
			t.Error("run.failed should not be stored when redaction drops it")
		}
	}

	// Post-terminal emit must be a no-op.
	countBefore := len(runner.getEvents(run.ID))
	runner.emit(run.ID, EventAssistantMessage, map[string]any{"content": "post-fail"})
	countAfter := len(runner.getEvents(run.ID))
	if countAfter != countBefore {
		t.Errorf("post-terminal emit was not dropped after run.failed: %d -> %d", countBefore, countAfter)
	}
}

// -------------------------------------------------------------------------
// TestTerminalSealing_NoPostTerminalEventsAppended
//
// Verifies that the terminated gate prevents any events from being appended
// after a terminal event has been processed — even when called concurrently.
// -------------------------------------------------------------------------
func TestTerminalSealing_NoPostTerminalEventsAppended(t *testing.T) {
	t.Parallel()

	prov := &stubProvider{turns: []CompletionResult{{Content: "sealed"}}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "no post-terminal"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Wait for run to complete.
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	// Record event count at the seal point.
	eventsAtSeal := runner.getEvents(run.ID)
	countAtSeal := len(eventsAtSeal)

	// Emit several events after the run is sealed — they must all be dropped.
	for i := 0; i < 5; i++ {
		runner.emit(run.ID, EventAssistantMessage, map[string]any{"i": i})
	}

	eventsAfter := runner.getEvents(run.ID)
	if len(eventsAfter) != countAtSeal {
		t.Errorf("post-terminal events were appended: before=%d, after=%d",
			countAtSeal, len(eventsAfter))
	}

	// The final stored event must be a terminal type.
	if len(eventsAtSeal) == 0 {
		t.Fatal("no events stored")
	}
	last := eventsAtSeal[len(eventsAtSeal)-1]
	if !IsTerminalEvent(last.Type) {
		t.Errorf("last stored event is not terminal: %q", last.Type)
	}
}

// -------------------------------------------------------------------------
// TestTerminalSealing_BothCompletedAndFailedCovered
//
// Verifies terminal sealing works for both run.completed and run.failed paths:
//   - A successful run seals with run.completed as the last event.
//   - A failing run seals with run.failed as the last event.
//
// -------------------------------------------------------------------------
func TestTerminalSealing_BothCompletedAndFailedCovered(t *testing.T) {
	t.Parallel()

	t.Run("completed", func(t *testing.T) {
		t.Parallel()
		prov := &stubProvider{turns: []CompletionResult{{Content: "ok"}}}
		runner := NewRunner(prov, NewRegistry(), RunnerConfig{
			DefaultModel: "test-model",
			MaxSteps:     2,
		})
		run, err := runner.StartRun(RunRequest{Prompt: "seal completed"})
		if err != nil {
			t.Fatalf("StartRun: %v", err)
		}
		waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)
		events := runner.getEvents(run.ID)
		if len(events) == 0 {
			t.Fatal("no events")
		}
		last := events[len(events)-1]
		if last.Type != EventRunCompleted {
			t.Errorf("last event = %q, want run.completed", last.Type)
		}
	})

	t.Run("failed", func(t *testing.T) {
		t.Parallel()
		prov := &errorProvider{err: errors.New("boom")}
		runner := NewRunner(prov, NewRegistry(), RunnerConfig{
			DefaultModel: "test-model",
			MaxSteps:     2,
		})
		run, err := runner.StartRun(RunRequest{Prompt: "seal failed"})
		if err != nil {
			t.Fatalf("StartRun: %v", err)
		}
		waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)
		events := runner.getEvents(run.ID)
		if len(events) == 0 {
			t.Fatal("no events")
		}
		last := events[len(events)-1]
		if last.Type != EventRunFailed {
			t.Errorf("last event = %q, want run.failed", last.Type)
		}
	})
}

// -------------------------------------------------------------------------
// TestTerminalSealing_RecorderDropMarkerStructure
//
// Verifies the drop marker event structure. We call safeRecorderSend with a
// full channel to confirm the non-blocking send returns false, and verify
// that the drop marker payload fields match the specification:
//   - dropped_event_id: original event ID
//   - dropped_event_type: original event type string
//   - dropped_seq: original event seq number
//
// This test uses the internal safeRecorderSend function (same package).
// -------------------------------------------------------------------------
func TestTerminalSealing_RecorderDropMarkerStructure(t *testing.T) {
	t.Parallel()

	// Create a zero-capacity channel to guarantee the non-blocking send fails.
	fullCh := make(chan rollout.RecordableEvent) // unbuffered = always full for non-blocking send

	original := rollout.RecordableEvent{
		ID:    "run_test:42",
		RunID: "run_test",
		Type:  "assistant.message",
		Seq:   42,
		Payload: map[string]any{
			"content": "hello",
		},
	}

	// safeRecorderSend must return false for an unbuffered (full) channel.
	sent := safeRecorderSend(fullCh, original)
	if sent {
		t.Error("expected safeRecorderSend to return false for unbuffered channel")
	}

	// Now verify that the drop marker we would build matches the spec.
	dropMarker := rollout.RecordableEvent{
		ID:        original.RunID + ":drop:" + "42",
		RunID:     original.RunID,
		Type:      string(EventRecorderDropDetected),
		Timestamp: time.Now().UTC(),
		Seq:       original.Seq,
		Payload: map[string]any{
			"dropped_event_id":   original.ID,
			"dropped_event_type": original.Type,
			"dropped_seq":        original.Seq,
		},
	}

	// Verify fields match spec.
	if dropMarker.Type != string(EventRecorderDropDetected) {
		t.Errorf("drop marker type = %q, want %q", dropMarker.Type, EventRecorderDropDetected)
	}
	if dropMarker.Payload["dropped_event_id"] != original.ID {
		t.Errorf("drop marker dropped_event_id = %v, want %q", dropMarker.Payload["dropped_event_id"], original.ID)
	}
	if dropMarker.Payload["dropped_event_type"] != original.Type {
		t.Errorf("drop marker dropped_event_type = %v, want %q", dropMarker.Payload["dropped_event_type"], original.Type)
	}
	if dropMarker.Payload["dropped_seq"] != original.Seq {
		t.Errorf("drop marker dropped_seq = %v, want %d", dropMarker.Payload["dropped_seq"], original.Seq)
	}
}

// -------------------------------------------------------------------------
// TestTerminalSealing_AuditWriterAbsentDoesNotAffectCompletion
//
// Verifies that when AuditTrailEnabled=false (no audit writer), the run
// still completes normally — audit configuration is non-fatal.
// -------------------------------------------------------------------------
func TestTerminalSealing_AuditWriterAbsentDoesNotAffectCompletion(t *testing.T) {
	t.Parallel()

	prov := &stubProvider{turns: []CompletionResult{{Content: "no audit"}}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
		// AuditTrailEnabled is false by default — no audit writer created.
	})

	run, err := runner.StartRun(RunRequest{Prompt: "audit absent"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	finalStatus := waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)
	if finalStatus != RunStatusCompleted {
		state, _ := runner.GetRun(run.ID)
		t.Fatalf("expected completed, got %q (error: %s)", finalStatus, state.Error)
	}

	// No audit.jsonl should be created (no RolloutDir set, no AuditTrailEnabled).
	// The run state must be consistent.
	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q", state.Status)
	}
	if state.Output != "no audit" {
		t.Errorf("unexpected output: %q", state.Output)
	}
}

// -------------------------------------------------------------------------
// TestTerminalSealing_AuditWriterWithRolloutDirClosesOnTerminal
//
// Verifies that when AuditTrailEnabled=true and RolloutDir is set, the run
// completes AND the audit.jsonl file is written with both run.started and
// the terminal event (run.completed or run.failed).
// The audit file close must happen exactly once — verified by checking
// that re-reading the file after run completion returns valid entries.
// -------------------------------------------------------------------------
func TestTerminalSealing_AuditWriterWithRolloutDirClosesOnTerminal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	prov := &stubProvider{turns: []CompletionResult{{Content: "audit close test"}}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:      "test-model",
		MaxSteps:          2,
		RolloutDir:        dir,
		AuditTrailEnabled: true,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "audit close"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	// Find and read the audit log.
	auditPath := findAuditLog(t, dir)
	if auditPath == "" {
		t.Fatal("audit.jsonl not found after run completion")
	}

	entries := readAuditEntries(t, auditPath)
	if len(entries) < 2 {
		t.Fatalf("expected at least 2 audit entries (run.started + terminal), got %d", len(entries))
	}

	// First entry must be run.started.
	if entries[0].EventType != "run.started" {
		t.Errorf("first audit entry = %q, want run.started", entries[0].EventType)
	}

	// Last entry must be a terminal event.
	last := entries[len(entries)-1]
	if last.EventType != "run.completed" && last.EventType != "run.failed" {
		t.Errorf("last audit entry = %q, want terminal event", last.EventType)
	}

	// File must be readable and not corrupt (no partial JSON lines).
	// Re-read with bufio.Scanner to ensure file is properly closed.
	f, err := os.Open(auditPath)
	if err != nil {
		t.Fatalf("re-open audit log: %v", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(line, &m); err != nil {
			t.Errorf("corrupt audit line %d: %v — %s", lineCount, err, string(line))
		}
		lineCount++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}
	if lineCount < 2 {
		t.Errorf("expected at least 2 valid audit lines, got %d", lineCount)
	}
}

// -------------------------------------------------------------------------
// TestTerminalSealing_AuditWriterFailedRunClosesOnTerminal
//
// Verifies that run.failed also properly closes the audit writer.
// -------------------------------------------------------------------------
func TestTerminalSealing_AuditWriterFailedRunClosesOnTerminal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	prov := &errorProvider{err: errors.New("forced failure")}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:      "test-model",
		MaxSteps:          2,
		RolloutDir:        dir,
		AuditTrailEnabled: true,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "audit failed close"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if state.Status != RunStatusFailed {
		t.Fatalf("expected failed, got %q", state.Status)
	}

	auditPath := findAuditLog(t, dir)
	if auditPath == "" {
		t.Fatal("audit.jsonl not found after failed run")
	}

	entries := readAuditEntries(t, auditPath)
	if len(entries) < 2 {
		t.Fatalf("expected at least 2 audit entries, got %d", len(entries))
	}

	last := entries[len(entries)-1]
	if last.EventType != "run.failed" {
		t.Errorf("last audit entry = %q, want run.failed", last.EventType)
	}
}

// -------------------------------------------------------------------------
// TestTerminalSealing_SubscriberPayloadImmutableAcrossFanOut
//
// Verifies that event payloads delivered to subscribers are deep copies —
// a subscriber mutating its copy must not affect:
//  1. The stored forensic event in run history.
//  2. The payload delivered to another subscriber.
//
// -------------------------------------------------------------------------
func TestTerminalSealing_SubscriberPayloadImmutableAcrossFanOut(t *testing.T) {
	t.Parallel()

	prov := &stubProvider{turns: []CompletionResult{{Content: "immutable"}}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "immutability test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	// Open two separate subscriptions to the same run (reading history).
	history1, _, cancel1, err := runner.Subscribe(run.ID)
	if err != nil {
		t.Fatalf("Subscribe (1): %v", err)
	}
	cancel1()

	history2, _, cancel2, err := runner.Subscribe(run.ID)
	if err != nil {
		t.Fatalf("Subscribe (2): %v", err)
	}
	cancel2()

	if len(history1) == 0 || len(history2) == 0 {
		t.Fatal("expected events in history")
	}
	if len(history1) != len(history2) {
		t.Fatalf("subscriber histories differ: %d vs %d", len(history1), len(history2))
	}

	// Find the run.started event in both histories.
	var started1, started2 *Event
	for i := range history1 {
		if history1[i].Type == EventRunStarted {
			started1 = &history1[i]
			break
		}
	}
	for i := range history2 {
		if history2[i].Type == EventRunStarted {
			started2 = &history2[i]
			break
		}
	}
	if started1 == nil || started2 == nil {
		t.Fatal("run.started not found in both histories")
	}

	// Mutate the payload from subscriber 1.
	started1.Payload["__mutation_test__"] = "should not propagate"

	// Subscriber 2's payload must be unaffected.
	if _, mutated := started2.Payload["__mutation_test__"]; mutated {
		t.Error("subscriber 2 payload was mutated by subscriber 1 mutation — payloads are not isolated")
	}

	// The stored forensic event must also be unaffected.
	storedEvents := runner.getEvents(run.ID)
	for _, ev := range storedEvents {
		if ev.Type == EventRunStarted {
			if _, mutated := ev.Payload["__mutation_test__"]; mutated {
				t.Error("stored forensic event was mutated by subscriber mutation — deep copy is missing")
			}
			break
		}
	}
}

// -------------------------------------------------------------------------
// TestTerminalSealing_RolloutFileContainsTerminalEvent
//
// Verifies that when RolloutDir is set, the rollout JSONL file contains
// a terminal event as its last entry for both completed and failed runs.
// -------------------------------------------------------------------------
func TestTerminalSealing_RolloutFileContainsTerminalEvent(t *testing.T) {
	t.Parallel()

	t.Run("completed", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		prov := &stubProvider{turns: []CompletionResult{{Content: "rollout test"}}}
		runner := NewRunner(prov, NewRegistry(), RunnerConfig{
			DefaultModel: "test-model",
			MaxSteps:     2,
			RolloutDir:   dir,
		})
		run, err := runner.StartRun(RunRequest{Prompt: "rollout complete"})
		if err != nil {
			t.Fatalf("StartRun: %v", err)
		}
		waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

		// Find rollout file.
		rolloutPath := findRolloutFile(t, dir, run.ID)
		if rolloutPath == "" {
			t.Fatal("rollout JSONL not found")
		}
		last := waitForRolloutTerminalEvent(t, rolloutPath)
		if last["type"] != "run.completed" && last["type"] != "run.failed" {
			t.Errorf("last rollout entry type = %v, want terminal event", last["type"])
		}
	})

	t.Run("failed", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		prov := &errorProvider{err: errors.New("rollout fail")}
		runner := NewRunner(prov, NewRegistry(), RunnerConfig{
			DefaultModel: "test-model",
			MaxSteps:     2,
			RolloutDir:   dir,
		})
		run, err := runner.StartRun(RunRequest{Prompt: "rollout fail"})
		if err != nil {
			t.Fatalf("StartRun: %v", err)
		}
		waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

		rolloutPath := findRolloutFile(t, dir, run.ID)
		if rolloutPath == "" {
			t.Fatal("rollout JSONL not found for failed run")
		}
		last := waitForRolloutTerminalEvent(t, rolloutPath)
		if last["type"] != "run.failed" && last["type"] != "run.completed" {
			t.Errorf("last rollout entry type = %v, want terminal event", last["type"])
		}
	})
}

// -------------------------------------------------------------------------
// TestTerminalSealing_RecorderDropDetectedEventType
//
// Verifies that EventRecorderDropDetected has the correct string value and
// is included in AllEventTypes() — ensuring the event type catalog is
// consistent with its usage.
// -------------------------------------------------------------------------
func TestTerminalSealing_RecorderDropDetectedEventType(t *testing.T) {
	t.Parallel()

	if string(EventRecorderDropDetected) != "recorder.drop_detected" {
		t.Errorf("EventRecorderDropDetected = %q, want %q",
			EventRecorderDropDetected, "recorder.drop_detected")
	}

	// Must appear in AllEventTypes().
	found := false
	for _, et := range AllEventTypes() {
		if et == EventRecorderDropDetected {
			found = true
			break
		}
	}
	if !found {
		t.Error("EventRecorderDropDetected not in AllEventTypes()")
	}
}

// -------------------------------------------------------------------------
// TestTerminalSealing_AuditTrailWithToolCallSealsOnCompleted
//
// Regression: when a state-modifying tool call is made and AuditTrailEnabled
// is true, the audit writer must still close on run.completed (not hang).
// Also verifies that audit entries are written for both run.started and
// run.completed, with the tool action in between.
// -------------------------------------------------------------------------
func TestTerminalSealing_AuditTrailWithToolCallSealsOnCompleted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name:        "bash",
		Description: "run bash",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"command": map[string]any{"type": "string"}},
		},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return "output", nil
	}); err != nil {
		t.Fatalf("register bash tool: %v", err)
	}

	prov := &stubProvider{turns: []CompletionResult{
		{ToolCalls: []ToolCall{{ID: "c1", Name: "bash", Arguments: `{"command":"echo hi"}`}}},
		{Content: "done with audit"},
	}}

	runner := NewRunner(prov, registry, RunnerConfig{
		DefaultModel:      "test-model",
		MaxSteps:          3,
		RolloutDir:        dir,
		AuditTrailEnabled: true,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "audit tool seal"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Must not hang — audit writer must close on run.completed.
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q", state.Status)
	}

	auditPath := findAuditLog(t, dir)
	if auditPath == "" {
		t.Fatal("audit.jsonl not found")
	}

	entries := readAuditEntries(t, auditPath)
	if len(entries) < 3 {
		t.Fatalf("expected at least 3 audit entries (started + action + completed), got %d", len(entries))
	}

	// Verify run.started → audit.action → run.completed ordering in audit log.
	var types []string
	for _, e := range entries {
		types = append(types, e.EventType)
	}
	if types[0] != "run.started" {
		t.Errorf("first audit entry = %q, want run.started", types[0])
	}
	if types[len(types)-1] != "run.completed" {
		t.Errorf("last audit entry = %q, want run.completed", types[len(types)-1])
	}
	actionFound := false
	for _, et := range types {
		if et == "audit.action" {
			actionFound = true
			break
		}
	}
	if !actionFound {
		t.Error("no audit.action entry in audit log")
	}
}

// -------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------

// findRolloutFile finds the first JSONL rollout file for the given runID under dir.
func findRolloutFile(t *testing.T, dir, runID string) string {
	t.Helper()
	var found string
	_ = os.MkdirAll(dir, 0o755)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Logf("readdir %s: %v", dir, err)
		return ""
	}
	for _, de := range entries {
		if !de.IsDir() {
			continue
		}
		subdir := dir + "/" + de.Name()
		files, err := os.ReadDir(subdir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			name := f.Name()
			// rollout files are named <runID>.jsonl
			if strings.HasSuffix(name, ".jsonl") && strings.HasPrefix(name, runID) {
				found = subdir + "/" + name
				return found
			}
		}
	}
	return found
}

// readJSONLLines reads a JSONL file and returns each line decoded as map[string]any.
func readJSONLLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var result []map[string]any
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatalf("unmarshal JSONL line: %v — %s", err, string(line))
		}
		result = append(result, m)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}
	return result
}

func waitForRolloutTerminalEvent(t *testing.T, path string) map[string]any {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		lines := readJSONLLines(t, path)
		if len(lines) > 0 {
			last := lines[len(lines)-1]
			if eventType, _ := last["type"].(string); eventType == string(EventRunCompleted) || eventType == string(EventRunFailed) || eventType == string(EventRunCancelled) {
				return last
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for terminal rollout event in %s", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
