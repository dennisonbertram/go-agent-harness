package harness

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
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

// TestSubscribeCancelConcurrentEmit verifies that rapidly subscribing and
// cancelling while events are being emitted does not panic (send on closed channel).
func TestSubscribeCancelConcurrentEmit(t *testing.T) {
	t.Parallel()

	// Use a blocking provider so the run stays active while we hammer subscribe/cancel.
	blocker := make(chan struct{})
	prov := &blockingProvider{blocker: blocker}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "concurrent test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Wait until the run is actually running.
	deadline := time.Now().Add(2 * time.Second)
	for {
		r, _ := runner.GetRun(run.ID)
		if r.Status == RunStatusRunning {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for running status")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Spin up goroutines that subscribe, cancel, and emit concurrently.
	var wg sync.WaitGroup
	const n = 20
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, _, cancel, err := runner.Subscribe(run.ID)
			if err != nil {
				return
			}
			// Emit an event while the subscription is live, then immediately cancel.
			runner.emit(run.ID, EventType("test.concurrent"), map[string]any{"i": 1})
			cancel()
		}()
	}
	wg.Wait()

	// Unblock the provider so the run completes cleanly.
	close(blocker)
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)
}

// TestContinueRunPreservesSourceStatus verifies that the source run remains
// in Completed status after ContinueRun (not mutated to Running).
func TestContinueRunPreservesSourceStatus(t *testing.T) {
	t.Parallel()

	prov := &continuationProvider{
		turns: []CompletionResult{
			{Content: "first"},
			{Content: "second"},
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
	waitForStatus(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

	_, err = runner.ContinueRun(run1.ID, "follow up")
	if err != nil {
		t.Fatalf("ContinueRun: %v", err)
	}

	// The source run must still be Completed.
	run1Final, _ := runner.GetRun(run1.ID)
	if run1Final.Status != RunStatusCompleted {
		t.Errorf("source run status = %s, want %s", run1Final.Status, RunStatusCompleted)
	}

	// A second ContinueRun should fail.
	_, err = runner.ContinueRun(run1.ID, "second follow up")
	if err == nil {
		t.Error("expected error on second ContinueRun, got nil")
	}
}

// TestEmitDoesNotMutateCallerPayload verifies that emit() clones the payload
// map and does not modify the caller's copy.
func TestEmitDoesNotMutateCallerPayload(t *testing.T) {
	t.Parallel()

	prov := &stubProvider{turns: []CompletionResult{{Content: "done"}}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "payload test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	// Emit with a known payload and verify it is not mutated.
	original := map[string]any{"my_key": "my_value"}
	runner.emit(run.ID, EventType("test.payload"), original)

	if _, ok := original["schema_version"]; ok {
		t.Error("emit() mutated the caller's payload: found injected schema_version")
	}
	if _, ok := original["conversation_id"]; ok {
		t.Error("emit() mutated the caller's payload: found injected conversation_id")
	}
	if _, ok := original["step"]; ok {
		t.Error("emit() mutated the caller's payload: found injected step")
	}
}

// TestSubscribePayloadIsolation verifies that mutating an event payload received
// via Subscribe (either from history or from the live channel) does not corrupt
// the runner's stored forensic event history.
func TestSubscribePayloadIsolation(t *testing.T) {
	t.Parallel()

	prov := &stubProvider{turns: []CompletionResult{{Content: "done"}}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "isolation test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	// First subscription: mutate every event's payload from history.
	history1, _, cancel1, err := runner.Subscribe(run.ID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	cancel1()

	for i := range history1 {
		history1[i].Payload["__tamper__"] = true
	}

	// Second subscription: verify the stored events were NOT affected.
	history2, _, cancel2, err := runner.Subscribe(run.ID)
	if err != nil {
		t.Fatalf("Subscribe (2): %v", err)
	}
	cancel2()

	for i, ev := range history2 {
		if _, ok := ev.Payload["__tamper__"]; ok {
			t.Errorf("event[%d] (type=%s): stored payload was tampered by first subscriber", i, ev.Type)
		}
	}
}

// TestDeepPayloadCloneIsolation verifies that nested map/slice values in event
// payloads are deep-copied, so mutating a nested structure obtained via
// Subscribe does not corrupt stored forensic history or other subscriber copies.
func TestDeepPayloadCloneIsolation(t *testing.T) {
	t.Parallel()

	// Use a blocking provider so the run stays active while we emit our
	// nested-structure test event.
	blocker := make(chan struct{})
	prov := &blockingProvider{blocker: blocker}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "deep clone test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Wait until the run is actually running.
	deadline := time.Now().Add(2 * time.Second)
	for {
		r, _ := runner.GetRun(run.ID)
		if r.Status == RunStatusRunning {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for running status")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Emit an event with nested structures while the run is still active.
	runner.emit(run.ID, EventType("test.nested"), map[string]any{
		"tags":   []any{"alpha", "beta"},
		"nested": map[string]any{"inner": "original"},
	})

	// Subscriber 1: mutate nested values.
	history1, _, cancel1, err := runner.Subscribe(run.ID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	cancel1()

	// Find our test event.
	var testEvent1 *Event
	for i := range history1 {
		if history1[i].Type == "test.nested" {
			testEvent1 = &history1[i]
			break
		}
	}
	if testEvent1 == nil {
		// Unblock before fatal so goroutines can clean up.
		close(blocker)
		t.Fatal("test.nested event not found in history1")
	}

	// Mutate the nested map and slice from subscriber 1's copy.
	if nested, ok := testEvent1.Payload["nested"].(map[string]any); ok {
		nested["inner"] = "TAMPERED"
		nested["extra"] = "INJECTED"
	}
	if tags, ok := testEvent1.Payload["tags"].([]any); ok && len(tags) > 0 {
		tags[0] = "TAMPERED_TAG"
	}

	// Subscriber 2: verify the stored events are unaffected.
	history2, _, cancel2, err := runner.Subscribe(run.ID)
	if err != nil {
		close(blocker)
		t.Fatalf("Subscribe (2): %v", err)
	}
	cancel2()

	var testEvent2 *Event
	for i := range history2 {
		if history2[i].Type == "test.nested" {
			testEvent2 = &history2[i]
			break
		}
	}
	if testEvent2 == nil {
		close(blocker)
		t.Fatal("test.nested event not found in history2")
	}

	// Check nested map was not corrupted.
	if nested, ok := testEvent2.Payload["nested"].(map[string]any); ok {
		if nested["inner"] != "original" {
			t.Errorf("nested.inner was corrupted: got %v, want %q", nested["inner"], "original")
		}
		if _, exists := nested["extra"]; exists {
			t.Error("nested map has injected 'extra' key from subscriber 1")
		}
	} else {
		t.Error("test event missing nested map")
	}

	// Check slice was not corrupted.
	if tags, ok := testEvent2.Payload["tags"].([]any); ok {
		if len(tags) == 0 || tags[0] != "alpha" {
			t.Errorf("tags[0] was corrupted: got %v, want %q", tags[0], "alpha")
		}
	} else {
		t.Error("test event missing tags slice")
	}

	// Let the run finish cleanly.
	close(blocker)
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)
}

// TestDeepClonePayloadUnit tests the deepClonePayload helper directly for
// correctness with nested structures and nil/empty inputs.
func TestDeepClonePayloadUnit(t *testing.T) {
	t.Parallel()

	t.Run("nil input", func(t *testing.T) {
		if got := deepClonePayload(nil); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("nested structures", func(t *testing.T) {
		orig := map[string]any{
			"str": "hello",
			"num": float64(42),
			"nested": map[string]any{
				"a": "b",
				"deep": map[string]any{
					"level": float64(3),
				},
			},
			"list": []any{"x", float64(1), map[string]any{"in_list": true}},
		}

		cloned := deepClonePayload(orig)

		// Mutate original nested structures.
		orig["nested"].(map[string]any)["a"] = "CHANGED"
		orig["nested"].(map[string]any)["deep"].(map[string]any)["level"] = float64(999)
		orig["list"].([]any)[0] = "CHANGED"
		orig["list"].([]any)[2].(map[string]any)["in_list"] = false

		// Cloned must be unaffected.
		if cloned["nested"].(map[string]any)["a"] != "b" {
			t.Error("nested.a was aliased")
		}
		if cloned["nested"].(map[string]any)["deep"].(map[string]any)["level"] != float64(3) {
			t.Error("nested.deep.level was aliased")
		}
		if cloned["list"].([]any)[0] != "x" {
			t.Error("list[0] was aliased")
		}
		if cloned["list"].([]any)[2].(map[string]any)["in_list"] != true {
			t.Error("list[2].in_list was aliased")
		}
	})
}

// TestPostTerminalEventsDropped verifies that events emitted after the terminal
// event (run.completed / run.failed) are silently dropped and do not appear in
// the forensic event history.
func TestPostTerminalEventsDropped(t *testing.T) {
	t.Parallel()

	prov := &stubProvider{turns: []CompletionResult{{Content: "done"}}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "terminal test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	// Collect the event count before injecting post-terminal events.
	eventsBefore := collectEvents(t, runner, run.ID)
	countBefore := len(eventsBefore)
	if countBefore == 0 {
		t.Fatal("expected at least one event before post-terminal emission")
	}

	// Emit several events after the run is already terminal.
	for i := 0; i < 5; i++ {
		runner.emit(run.ID, EventType("test.post_terminal"), map[string]any{"seq": i})
	}

	// The stored event count must not have increased.
	eventsAfter := collectEvents(t, runner, run.ID)
	if len(eventsAfter) != countBefore {
		t.Errorf("post-terminal events leaked into history: before=%d after=%d",
			countBefore, len(eventsAfter))
	}

	// Verify no post-terminal event type appears.
	for _, ev := range eventsAfter {
		if ev.Type == "test.post_terminal" {
			t.Errorf("post-terminal event found in history: %+v", ev)
		}
	}
}

// TestRecorderNotCalledAfterTerminal verifies that the rollout recorder is not
// invoked on events that arrive after the terminal event (ensures no
// record-after-close panic).  This exercises the atomic detach path in emit().
func TestRecorderNotCalledAfterTerminal(t *testing.T) {
	t.Parallel()

	rolloutDir := t.TempDir()
	prov := &stubProvider{turns: []CompletionResult{{Content: "done"}}}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            2,
		RolloutDir:          rolloutDir,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "recorder test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	// Hammering emit() post-terminal must not panic (write on closed file).
	// The race detector will also flag any data race if the recorder is
	// accessed unsafely.
	for i := 0; i < 20; i++ {
		runner.emit(run.ID, EventType("test.post_terminal"), map[string]any{"i": i})
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
