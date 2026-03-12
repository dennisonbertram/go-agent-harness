package harness

import (
	"context"
	"errors"
	"testing"
)

// ---------------------------------------------------------------------------
// Integration tests for error chain tracing (issue #210)
// ---------------------------------------------------------------------------

// TestErrorChainDisabled verifies that when ErrorChainEnabled is false,
// no error.context event is emitted before run.failed.
func TestErrorChainDisabled(t *testing.T) {
	t.Parallel()

	p := &errorProvider{err: errors.New("deliberate failure")}
	r := NewRunner(p, NewRegistry(), RunnerConfig{})

	run, err := r.StartRun(RunRequest{Prompt: "trigger error"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	waitForStatus(t, r, run.ID, RunStatusFailed)
	events := collectEvents(t, r, run.ID)

	// error.context must NOT appear when ErrorChainEnabled is false
	for _, ev := range events {
		if ev.Type == EventErrorContext {
			t.Errorf("unexpected error.context event when ErrorChainEnabled=false")
		}
	}

	// run.failed must still appear
	assertEventTypeEC(t, events, EventRunFailed)
}

// TestErrorChainEnabled verifies that when ErrorChainEnabled is true,
// an error.context event is emitted immediately before run.failed.
func TestErrorChainEnabled(t *testing.T) {
	t.Parallel()

	p := &errorProvider{err: errors.New("provider blew up")}
	r := NewRunner(p, NewRegistry(), RunnerConfig{
		ErrorChainEnabled: true,
	})

	run, err := r.StartRun(RunRequest{Prompt: "trigger error"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	waitForStatus(t, r, run.ID, RunStatusFailed)
	events := collectEvents(t, r, run.ID)

	// Locate error.context and run.failed events
	ecIdx := -1
	rfIdx := -1
	for i, ev := range events {
		if ev.Type == EventErrorContext {
			ecIdx = i
		}
		if ev.Type == EventRunFailed {
			rfIdx = i
		}
	}

	if ecIdx < 0 {
		t.Fatal("expected error.context event, none found")
	}
	if rfIdx < 0 {
		t.Fatal("expected run.failed event, none found")
	}
	if ecIdx >= rfIdx {
		t.Errorf("error.context (idx %d) must come before run.failed (idx %d)", ecIdx, rfIdx)
	}
}

// TestErrorChainPayloadFields verifies that the error.context payload contains
// the required fields: class, error, snapshot.
func TestErrorChainPayloadFields(t *testing.T) {
	t.Parallel()

	p := &errorProvider{err: errors.New("bad provider")}
	r := NewRunner(p, NewRegistry(), RunnerConfig{
		ErrorChainEnabled: true,
	})

	run, err := r.StartRun(RunRequest{Prompt: "check fields"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	waitForStatus(t, r, run.ID, RunStatusFailed)
	events := collectEvents(t, r, run.ID)

	var ecPayload map[string]any
	for _, ev := range events {
		if ev.Type == EventErrorContext {
			ecPayload = ev.Payload
			break
		}
	}
	if ecPayload == nil {
		t.Fatal("no error.context event found")
	}

	if _, ok := ecPayload["class"]; !ok {
		t.Error("error.context payload missing 'class' field")
	}
	if _, ok := ecPayload["error"]; !ok {
		t.Error("error.context payload missing 'error' field")
	}
	if _, ok := ecPayload["snapshot"]; !ok {
		t.Error("error.context payload missing 'snapshot' field")
	}

	// snapshot must have tool_calls and messages
	snap, ok := ecPayload["snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("snapshot is not map[string]any, got %T", ecPayload["snapshot"])
	}
	if _, ok := snap["tool_calls"]; !ok {
		t.Error("snapshot missing 'tool_calls'")
	}
	if _, ok := snap["messages"]; !ok {
		t.Error("snapshot missing 'messages'")
	}
}

// TestErrorChainCustomDepth verifies that ErrorContextDepth is honoured.
func TestErrorChainCustomDepth(t *testing.T) {
	t.Parallel()

	p := &errorProvider{err: errors.New("depth test")}
	r := NewRunner(p, NewRegistry(), RunnerConfig{
		ErrorChainEnabled: true,
		ErrorContextDepth: 3,
	})

	run, err := r.StartRun(RunRequest{Prompt: "depth test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	waitForStatus(t, r, run.ID, RunStatusFailed)
	events := collectEvents(t, r, run.ID)

	var ecPayload map[string]any
	for _, ev := range events {
		if ev.Type == EventErrorContext {
			ecPayload = ev.Payload
			break
		}
	}
	if ecPayload == nil {
		t.Fatal("no error.context event found")
	}

	snap, ok := ecPayload["snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("snapshot is not map[string]any")
	}

	depth, ok := snap["depth"]
	if !ok {
		t.Fatal("snapshot missing 'depth'")
	}
	depthI, ok := depth.(int)
	if !ok {
		t.Fatalf("snapshot depth is not int, got %T", depth)
	}
	if depthI != 3 {
		t.Errorf("snapshot depth = %d, want 3", depthI)
	}
}

// TestErrorChainSnapshotCapturesUserMessage verifies that the initial user
// message is recorded in the snapshot's messages list.
func TestErrorChainSnapshotCapturesUserMessage(t *testing.T) {
	t.Parallel()

	p := &errorProvider{err: errors.New("msg test")}
	r := NewRunner(p, NewRegistry(), RunnerConfig{
		ErrorChainEnabled: true,
	})

	const userPrompt = "my specific prompt text"
	run, err := r.StartRun(RunRequest{Prompt: userPrompt})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	waitForStatus(t, r, run.ID, RunStatusFailed)
	events := collectEvents(t, r, run.ID)

	var ecPayload map[string]any
	for _, ev := range events {
		if ev.Type == EventErrorContext {
			ecPayload = ev.Payload
			break
		}
	}
	if ecPayload == nil {
		t.Fatal("no error.context event found")
	}

	snap, ok := ecPayload["snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("snapshot is not map[string]any")
	}

	msgs, ok := snap["messages"].([]map[string]any)
	if !ok {
		t.Fatalf("snapshot messages not []map[string]any, got %T", snap["messages"])
	}

	found := false
	for _, mm := range msgs {
		if mm["role"] == "user" && mm["content"] == userPrompt {
			found = true
		}
	}
	if !found {
		t.Errorf("user prompt %q not found in snapshot messages: %v", userPrompt, msgs)
	}
}

// TestErrorChainSnapshotToolCallRecorded verifies that a tool call that
// occurs before an error is recorded in the snapshot.
func TestErrorChainSnapshotToolCallRecorded(t *testing.T) {
	t.Parallel()

	// Use a stub that calls one tool successfully, then on next step fails.
	p := &oneToolThenErrorProvider{
		toolName: "bash",
		toolArgs: `{"cmd":"echo hi"}`,
		err:      errors.New("after tool error"),
	}
	r := NewRunner(p, NewRegistry(), RunnerConfig{
		ErrorChainEnabled: true,
		MaxSteps:          10,
	})

	run, err := r.StartRun(RunRequest{Prompt: "run tool then fail"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	waitForStatus(t, r, run.ID, RunStatusFailed)
	events := collectEvents(t, r, run.ID)

	var ecPayload map[string]any
	for _, ev := range events {
		if ev.Type == EventErrorContext {
			ecPayload = ev.Payload
			break
		}
	}
	if ecPayload == nil {
		t.Fatal("no error.context event found")
	}

	snap, ok := ecPayload["snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("snapshot is not map[string]any")
	}

	toolCalls, ok := snap["tool_calls"].([]map[string]any)
	if !ok {
		t.Fatalf("snapshot tool_calls not []map[string]any, got %T", snap["tool_calls"])
	}

	found := false
	for _, tcm := range toolCalls {
		if tcm["name"] == "bash" {
			found = true
		}
	}
	if !found {
		t.Errorf("bash tool call not found in snapshot tool_calls: %v", toolCalls)
	}
}

// TestErrorChainRunFailedStillEmitted verifies that run.failed is always
// emitted even when ErrorChainEnabled is true.
func TestErrorChainRunFailedStillEmitted(t *testing.T) {
	t.Parallel()

	p := &errorProvider{err: errors.New("fail")}
	r := NewRunner(p, NewRegistry(), RunnerConfig{
		ErrorChainEnabled: true,
	})

	run, err := r.StartRun(RunRequest{Prompt: "ensure run.failed"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	waitForStatus(t, r, run.ID, RunStatusFailed)
	events := collectEvents(t, r, run.ID)

	assertEventTypeEC(t, events, EventRunFailed)
}

// TestErrorChainDefaultDepth verifies that when ErrorContextDepth is 0,
// the default depth (10) is used.
func TestErrorChainDefaultDepth(t *testing.T) {
	t.Parallel()

	p := &errorProvider{err: errors.New("default depth test")}
	r := NewRunner(p, NewRegistry(), RunnerConfig{
		ErrorChainEnabled: true,
		ErrorContextDepth: 0, // should use default
	})

	run, err := r.StartRun(RunRequest{Prompt: "default depth"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	waitForStatus(t, r, run.ID, RunStatusFailed)
	events := collectEvents(t, r, run.ID)

	var ecPayload map[string]any
	for _, ev := range events {
		if ev.Type == EventErrorContext {
			ecPayload = ev.Payload
			break
		}
	}
	if ecPayload == nil {
		t.Fatal("no error.context event found")
	}

	snap, ok := ecPayload["snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("snapshot is not map[string]any")
	}

	depth, ok := snap["depth"]
	if !ok {
		t.Fatal("snapshot missing 'depth'")
	}
	depthI, ok := depth.(int)
	if !ok {
		t.Fatalf("snapshot depth is not int, got %T", depth)
	}
	if depthI != 10 {
		t.Errorf("snapshot depth = %d, want 10 (default)", depthI)
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// assertEventTypeEC checks that at least one event of the given type exists.
// Named with EC suffix to avoid redeclaration in the harness package.
func assertEventTypeEC(t *testing.T, events []Event, et EventType) {
	t.Helper()
	for _, ev := range events {
		if ev.Type == et {
			return
		}
	}
	t.Errorf("expected event %q, not found in %d events", et, len(events))
}

// oneToolThenErrorProvider is a stub that returns one tool call on the first
// Complete call, then returns an error on the second call.
type oneToolThenErrorProvider struct {
	toolName string
	toolArgs string
	err      error
	calls    int
}

func (p *oneToolThenErrorProvider) Complete(_ context.Context, _ CompletionRequest) (CompletionResult, error) {
	p.calls++
	if p.calls == 1 {
		return CompletionResult{
			ToolCalls: []ToolCall{
				{ID: "tc1", Name: p.toolName, Arguments: p.toolArgs},
			},
		}, nil
	}
	return CompletionResult{}, p.err
}
