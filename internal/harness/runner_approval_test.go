package harness

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// TestRunnerPermissionsApprovalRequired verifies that when a run is started with
// ApprovalPolicyAll and a tool is called, a tool.approval_required event is
// emitted and the run pauses (waiting_for_approval status).
func TestRunnerPermissionsApprovalRequired(t *testing.T) {
	t.Parallel()

	approvalBroker := NewInMemoryApprovalBroker()

	provider := &stubProvider{
		turns: []CompletionResult{
			{
				ToolCalls: []ToolCall{{
					ID:        "call_1",
					Name:      "echo_json",
					Arguments: `{"value":"hello"}`,
				}},
			},
			// After approval, tool executes and we get a final message.
			{Content: "done"},
		},
	}

	registry := NewRegistry()
	_ = registry.Register(ToolDefinition{
		Name:        "echo_json",
		Description: "echoes payload",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"value": map[string]any{"type": "string"},
			},
		},
		ParallelSafe: true,
	}, func(_ context.Context, args json.RawMessage) (string, error) {
		return string(args), nil
	})

	runner := NewRunner(provider, registry, RunnerConfig{
		ApprovalBroker: approvalBroker,
	})

	run, err := runner.StartRun(RunRequest{
		Prompt: "test approval",
		Permissions: &PermissionConfig{
			Sandbox:  SandboxScopeUnrestricted,
			Approval: ApprovalPolicyAll,
		},
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Wait for pending approval to appear in the broker.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, ok := approvalBroker.Pending(run.ID); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for pending approval in broker")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// The run should be in waiting_for_approval status.
	r, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatalf("GetRun: not found")
	}
	if r.Status != RunStatusWaitingForApproval {
		t.Errorf("run status = %q, want %q", r.Status, RunStatusWaitingForApproval)
	}

	// Check pending fields.
	pending, _ := approvalBroker.Pending(run.ID)
	if pending.Tool != "echo_json" {
		t.Errorf("pending tool = %q, want echo_json", pending.Tool)
	}
	if pending.CallID != "call_1" {
		t.Errorf("pending CallID = %q, want call_1", pending.CallID)
	}

	// Subscribe to get events from this run.
	runHistory, runStream, runCancel, subErr := runner.Subscribe(run.ID)
	if subErr != nil {
		t.Fatalf("Subscribe: %v", subErr)
	}
	defer runCancel()

	// Approve the tool call — run should resume and complete.
	if err := approvalBroker.Approve(run.ID); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	// Collect events until terminal.
	allEvents := append([]Event(nil), runHistory...)
	collectionTimeout := time.After(5 * time.Second)
	done := false
	for !done {
		select {
		case ev, ok := <-runStream:
			if !ok {
				done = true
				break
			}
			allEvents = append(allEvents, ev)
			if IsTerminalEvent(ev.Type) {
				done = true
			}
		case <-collectionTimeout:
			t.Fatal("timed out collecting events after approval")
		}
	}

	// Run should be completed.
	r, ok = runner.GetRun(run.ID)
	if !ok {
		t.Fatalf("GetRun after approval: not found")
	}
	if r.Status != RunStatusCompleted {
		t.Errorf("run status = %q after approval, want completed", r.Status)
	}

	// Verify approval_required and approval_granted events were emitted.
	var requiredSeen, grantedSeen bool
	for _, evt := range allEvents {
		switch evt.Type {
		case EventToolApprovalRequired:
			requiredSeen = true
			if evt.Payload["tool"] != "echo_json" {
				t.Errorf("approval_required tool = %v, want echo_json", evt.Payload["tool"])
			}
			if evt.Payload["call_id"] != "call_1" {
				t.Errorf("approval_required call_id = %v, want call_1", evt.Payload["call_id"])
			}
		case EventToolApprovalGranted:
			grantedSeen = true
			if evt.Payload["call_id"] != "call_1" {
				t.Errorf("approval_granted call_id = %v, want call_1", evt.Payload["call_id"])
			}
		}
	}
	if !requiredSeen {
		t.Error("expected tool.approval_required event, not found")
	}
	if !grantedSeen {
		t.Error("expected tool.approval_granted event, not found")
	}
}

// TestRunnerPermissionsApprovalDenied verifies that when an operator denies
// a tool call, the tool returns an error result to the LLM and the run continues.
func TestRunnerPermissionsApprovalDenied(t *testing.T) {
	t.Parallel()

	approvalBroker := NewInMemoryApprovalBroker()

	provider := &stubProvider{
		turns: []CompletionResult{
			{
				ToolCalls: []ToolCall{{
					ID:        "call_deny",
					Name:      "echo_json",
					Arguments: `{"value":"hello"}`,
				}},
			},
			// After the deny, the LLM sees the error and decides to stop.
			{Content: "understood, tool was denied"},
		},
	}

	registry := NewRegistry()
	_ = registry.Register(ToolDefinition{
		Name:        "echo_json",
		Description: "echoes payload",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"value": map[string]any{"type": "string"},
			},
		},
		ParallelSafe: true,
	}, func(_ context.Context, args json.RawMessage) (string, error) {
		return string(args), nil
	})

	runner := NewRunner(provider, registry, RunnerConfig{
		ApprovalBroker: approvalBroker,
	})

	run, err := runner.StartRun(RunRequest{
		Prompt: "test denial",
		Permissions: &PermissionConfig{
			Sandbox:  SandboxScopeUnrestricted,
			Approval: ApprovalPolicyAll,
		},
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Wait for pending approval to appear.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, ok := approvalBroker.Pending(run.ID); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for pending approval")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Subscribe before denial so we capture all events.
	runHistory, runStream, runCancel, subErr := runner.Subscribe(run.ID)
	if subErr != nil {
		t.Fatalf("Subscribe: %v", subErr)
	}
	defer runCancel()

	// Deny the tool call.
	if err := approvalBroker.Deny(run.ID); err != nil {
		t.Fatalf("Deny: %v", err)
	}

	// Collect events until terminal.
	allEvents := append([]Event(nil), runHistory...)
	collectionTimeout := time.After(5 * time.Second)
	done := false
	for !done {
		select {
		case ev, ok := <-runStream:
			if !ok {
				done = true
				break
			}
			allEvents = append(allEvents, ev)
			if IsTerminalEvent(ev.Type) {
				done = true
			}
		case <-collectionTimeout:
			t.Fatal("timed out collecting events after denial")
		}
	}

	// Run should be completed (the LLM gets the denial error and responds).
	r, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatalf("GetRun after denial: not found")
	}
	if r.Status != RunStatusCompleted {
		t.Errorf("run status = %q after denial, want completed", r.Status)
	}

	// Verify denial event emitted.
	var deniedSeen, completedSeen bool
	for _, evt := range allEvents {
		switch evt.Type {
		case EventToolApprovalDenied:
			deniedSeen = true
			if evt.Payload["call_id"] != "call_deny" {
				t.Errorf("approval_denied call_id = %v, want call_deny", evt.Payload["call_id"])
			}
		case EventToolCallCompleted:
			if callID, _ := evt.Payload["call_id"].(string); callID == "call_deny" {
				completedSeen = true
			}
		}
	}
	if !deniedSeen {
		t.Error("expected tool.approval_denied event, not found")
	}
	if !completedSeen {
		t.Error("expected tool.call.completed for denied call, not found")
	}
}

// TestRunnerPermissionsNoneSkipsApproval verifies that when ApprovalPolicyNone
// is set, tool calls execute immediately without consulting the approval broker.
func TestRunnerPermissionsNoneSkipsApproval(t *testing.T) {
	t.Parallel()

	approvalBroker := NewInMemoryApprovalBroker()

	provider := &stubProvider{
		turns: []CompletionResult{
			{
				ToolCalls: []ToolCall{{
					ID:        "call_no_approval",
					Name:      "echo_json",
					Arguments: `{"value":"hello"}`,
				}},
			},
			{Content: "done"},
		},
	}

	registry := NewRegistry()
	_ = registry.Register(ToolDefinition{
		Name:        "echo_json",
		Description: "echoes payload",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"value": map[string]any{"type": "string"}},
		},
		ParallelSafe: true,
	}, func(_ context.Context, args json.RawMessage) (string, error) {
		return string(args), nil
	})

	runner := NewRunner(provider, registry, RunnerConfig{
		ApprovalBroker: approvalBroker,
	})

	run, err := runner.StartRun(RunRequest{
		Prompt: "no approval needed",
		Permissions: &PermissionConfig{
			Sandbox:  SandboxScopeUnrestricted,
			Approval: ApprovalPolicyNone,
		},
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	r, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatalf("GetRun: not found")
	}
	if r.Status != RunStatusCompleted {
		t.Errorf("run status = %q, want completed", r.Status)
	}

	// No pending approvals should exist.
	if _, ok := approvalBroker.Pending(run.ID); ok {
		t.Error("expected no pending approval for ApprovalPolicyNone run")
	}

	// No approval events should have been emitted.
	for _, evt := range events {
		if evt.Type == EventToolApprovalRequired {
			t.Error("unexpected tool.approval_required event for ApprovalPolicyNone run")
		}
	}
}

// TestRunnerPermissionsDestructiveApprovalOnMutating verifies that
// ApprovalPolicyDestructive requires approval for mutating tools but not
// for read-only tools.
//
// Strategy: the broker is checked directly (broker.Pending) to confirm which
// tools triggered an approval pause. We also check the run's event history for
// approval_required payloads. Since we subscribe after StartRun, early events
// may already be in the history slice returned by Subscribe.
func TestRunnerPermissionsDestructiveApprovalOnMutating(t *testing.T) {
	t.Parallel()

	approvalBroker := NewInMemoryApprovalBroker()

	provider := &stubProvider{
		turns: []CompletionResult{
			{
				// First, call a read (non-mutating) tool — should not require approval.
				ToolCalls: []ToolCall{{
					ID:        "call_read",
					Name:      "echo_readonly",
					Arguments: `{"value":"hello"}`,
				}},
			},
			{
				// Then call a mutating tool — should require approval.
				ToolCalls: []ToolCall{{
					ID:        "call_mutating",
					Name:      "echo_mutating",
					Arguments: `{"value":"world"}`,
				}},
			},
			{Content: "done"},
		},
	}

	registry := NewRegistry()
	// Read-only tool — Mutating=false (default).
	_ = registry.Register(ToolDefinition{
		Name:        "echo_readonly",
		Description: "read only",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"value": map[string]any{"type": "string"}},
		},
		ParallelSafe: true,
	}, func(_ context.Context, args json.RawMessage) (string, error) {
		return string(args), nil
	})
	// Mutating tool.
	_ = registry.RegisterWithOptions(ToolDefinition{
		Name:        "echo_mutating",
		Description: "mutating",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"value": map[string]any{"type": "string"}},
		},
		ParallelSafe: false,
		Mutating:     true,
	}, func(_ context.Context, args json.RawMessage) (string, error) {
		return string(args), nil
	}, RegisterOptions{})

	runner := NewRunner(provider, registry, RunnerConfig{
		ApprovalBroker: approvalBroker,
	})

	run, err := runner.StartRun(RunRequest{
		Prompt: "test destructive policy",
		Permissions: &PermissionConfig{
			Sandbox:  SandboxScopeUnrestricted,
			Approval: ApprovalPolicyDestructive,
		},
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Wait for the mutating tool to be pending for approval.
	// The read-only tool should have already executed without pausing.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if p, ok := approvalBroker.Pending(run.ID); ok && p.Tool == "echo_mutating" {
			break
		}
		if time.Now().After(deadline) {
			r, _ := runner.GetRun(run.ID)
			t.Fatalf("timed out waiting for mutating tool approval, run status=%s", r.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Subscribe to get event history (which now includes the approval_required event).
	runHistory, runStream, runCancel, subErr := runner.Subscribe(run.ID)
	if subErr != nil {
		t.Fatalf("Subscribe: %v", subErr)
	}
	defer runCancel()

	// Confirm via broker that echo_readonly did NOT require approval —
	// if it had, the broker would have held the run before echo_mutating.
	// Since we're now waiting on echo_mutating, echo_readonly must have run freely.

	// Approve the mutating tool.
	if err := approvalBroker.Approve(run.ID); err != nil {
		t.Fatalf("Approve mutating: %v", err)
	}

	// Drain remaining events until the run completes.
	allEvents := append([]Event(nil), runHistory...)
	collectTimeout := time.After(5 * time.Second)
	done := false
	for !done {
		select {
		case ev, ok := <-runStream:
			if !ok {
				done = true
				break
			}
			allEvents = append(allEvents, ev)
			if IsTerminalEvent(ev.Type) {
				done = true
			}
		case <-collectTimeout:
			t.Fatal("timed out collecting events after mutating tool approval")
		}
	}

	// Run should be completed.
	r, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatalf("GetRun: not found")
	}
	if r.Status != RunStatusCompleted {
		t.Errorf("run status = %q, want completed", r.Status)
	}

	// Check all events (history + stream) for approval events.
	var readonlyApprovalSeen bool
	var mutatingApprovalSeen bool
	for _, evt := range allEvents {
		if evt.Type == EventToolApprovalRequired {
			if tool, _ := evt.Payload["tool"].(string); tool == "echo_readonly" {
				readonlyApprovalSeen = true
			}
			if tool, _ := evt.Payload["tool"].(string); tool == "echo_mutating" {
				mutatingApprovalSeen = true
			}
		}
	}
	if readonlyApprovalSeen {
		t.Error("unexpected tool.approval_required for read-only tool")
	}
	// The mutating approval_required event is in the history since we subscribed
	// while the run was paused at that point.
	if !mutatingApprovalSeen {
		t.Error("expected tool.approval_required for mutating tool, not found in events")
	}
}
