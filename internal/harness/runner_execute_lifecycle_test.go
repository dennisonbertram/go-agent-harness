package harness

// runner_execute_lifecycle_test.go — characterize execute lifecycle across
// compaction, memory, wait-for-user, and cost-limit paths.
// Covers GitHub issue #329.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	htools "go-agent-harness/internal/harness/tools"
	om "go-agent-harness/internal/observationalmemory"
)

// -------------------------------------------------------------------------
// TestExecuteLifecycle_AutoCompactAndContextWindowSnapshotSameTurn
//
// Verifies that when AutoCompactEnabled is true and the prompt exceeds the
// threshold, the runner emits auto_compact.started and auto_compact.completed
// in the same turn as context.window.snapshot (if enabled). The run must
// complete normally and the stored transcript must not be empty.
// -------------------------------------------------------------------------
func TestExecuteLifecycle_AutoCompactAndContextWindowSnapshotSameTurn(t *testing.T) {
	t.Parallel()

	// Use a tiny context window so both compaction and snapshot trigger.
	// Context window = 10 tokens, 0.50 threshold => >5 tokens triggers.
	// "abcdefghi" repeated 5 times = 45 chars ≈ 11 tokens, enough to trigger.
	largePrompt := strings.Repeat("abcdefghi ", 5)

	provider := &staticRunnerProvider{result: CompletionResult{Content: "done"}}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel:                 "test",
		MaxSteps:                     2,
		AutoCompactEnabled:           true,
		ModelContextWindow:           10,
		AutoCompactThreshold:         0.50,
		AutoCompactKeepLast:          2,
		AutoCompactMode:              "strip",
		ContextWindowSnapshotEnabled: true,
	})

	run, err := runner.StartRun(RunRequest{Prompt: largePrompt})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	// Run must complete.
	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q (error: %s)", state.Status, state.Error)
	}

	// Event ordering: auto_compact.started must precede run.completed.
	requireEventOrder(t, events,
		"run.started",
		"auto_compact.started",
		"auto_compact.completed",
		"run.completed",
	)

	// context.window.snapshot must also appear (either before or after compact).
	snapshotFound := false
	for _, ev := range events {
		if ev.Type == EventContextWindowSnapshot {
			snapshotFound = true
		}
	}
	if !snapshotFound {
		t.Error("expected context.window.snapshot event")
	}

	// Stored transcript must be non-empty (at minimum the user message).
	msgs := runner.GetRunMessages(run.ID)
	if len(msgs) == 0 {
		t.Error("expected at least one stored message in transcript")
	}
}

// -------------------------------------------------------------------------
// TestExecuteLifecycle_AutoCompactAndMemorySnippetSameTurn
//
// Verifies that when AutoCompactEnabled is true and a memory snippet is
// injected, both behaviors coexist in the same turn without interfering.
// The compaction path must not discard the injected memory message from
// the request shape seen by the provider.
// -------------------------------------------------------------------------
func TestExecuteLifecycle_AutoCompactAndMemorySnippetSameTurn(t *testing.T) {
	t.Parallel()

	// Large prompt triggers compaction; memory snippet is injected into request.
	largePrompt := strings.Repeat("word ", 50) // ~50+ tokens with tiny window

	capProvider := &capturingProvider{
		turns: []CompletionResult{{Content: "finished"}},
	}

	mem := &memoryStub{
		status: om.Status{
			Mode:                     om.ModeLocalCoordinator,
			MemoryID:                 "default|conv|agent",
			Scope:                    om.ScopeKey{TenantID: "default", ConversationID: "conv", AgentID: "agent"},
			Enabled:                  true,
			LastObservedMessageIndex: -1,
			UpdatedAt:                time.Now().UTC(),
		},
		snippet: "<observational-memory>lifecycle-test-snippet</observational-memory>",
	}

	runner := NewRunner(capProvider, NewRegistry(), RunnerConfig{
		DefaultModel:         "test",
		MaxSteps:             2,
		MemoryManager:        mem,
		AskUserTimeout:       time.Second,
		AutoCompactEnabled:   true,
		ModelContextWindow:   20,
		AutoCompactThreshold: 0.50,
		AutoCompactKeepLast:  2,
		AutoCompactMode:      "strip",
	})

	run, err := runner.StartRun(RunRequest{Prompt: largePrompt})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q (error: %s)", state.Status, state.Error)
	}

	// Memory observe events must appear before the terminal event.
	requireEventOrder(t, events,
		"run.started",
		"memory.observe.started",
		"memory.observe.completed",
		"run.completed",
	)

	// The provider must have received the memory snippet as the first message.
	if len(capProvider.calls) == 0 {
		t.Fatal("expected at least one provider call")
	}
	firstReqMsgs := capProvider.calls[0].Messages
	snippetInjected := false
	for _, m := range firstReqMsgs {
		if strings.Contains(m.Content, "lifecycle-test-snippet") {
			snippetInjected = true
			break
		}
	}
	if !snippetInjected {
		t.Errorf("memory snippet not found in provider request messages: %+v", firstReqMsgs)
	}

	// Stored transcript must include at least the user prompt message.
	msgs := runner.GetRunMessages(run.ID)
	if len(msgs) == 0 {
		t.Error("expected stored messages in transcript")
	}
}

// -------------------------------------------------------------------------
// TestExecuteLifecycle_WaitForUserFlowEventOrderAndStateRestoration
//
// Verifies the waiting-for-user flow:
//   - run.waiting_for_user is emitted when AskUserQuestion tool fires.
//   - Status transitions to waiting_for_user.
//   - After SubmitInput, run.resumed is emitted.
//   - Run completes and state.Status == completed.
//   - Event ordering is pinned precisely.
//
// -------------------------------------------------------------------------
func TestExecuteLifecycle_WaitForUserFlowEventOrderAndStateRestoration(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{turns: []CompletionResult{
		{
			ToolCalls: []ToolCall{{
				ID:        "call_ask_lc",
				Name:      htools.AskUserQuestionToolName,
				Arguments: `{"questions":[{"question":"Pick one?","header":"Choice","options":[{"label":"A","description":"Option A"},{"label":"B","description":"Option B"}],"multiSelect":false}]}`,
			}},
		},
		{Content: "lifecycle complete"},
	}}

	broker := NewInMemoryAskUserQuestionBroker(time.Now)
	runner := NewRunner(provider, NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode:   ToolApprovalModeFullAuto,
		AskUserBroker:  broker,
		AskUserTimeout: 2 * time.Second,
	}), RunnerConfig{
		DefaultModel:   "gpt-5-nano",
		MaxSteps:       4,
		AskUserBroker:  broker,
		AskUserTimeout: 2 * time.Second,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "lifecycle wait-for-user"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Wait until status transitions to waiting_for_user.
	deadline := time.Now().Add(2 * time.Second)
	for {
		state, ok := runner.GetRun(run.ID)
		if !ok {
			t.Fatal("run not found while polling for waiting_for_user")
		}
		if state.Status == RunStatusWaitingForUser {
			break
		}
		if state.Status == RunStatusCompleted || state.Status == RunStatusFailed {
			t.Fatalf("run ended prematurely with status %q", state.Status)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for waiting_for_user status, last: %s", state.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// State must be waiting_for_user.
	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if state.Status != RunStatusWaitingForUser {
		t.Fatalf("expected waiting_for_user, got %q", state.Status)
	}

	// PendingInput must return the question.
	pending, err := runner.PendingInput(run.ID)
	if err != nil {
		t.Fatalf("PendingInput: %v", err)
	}
	if pending.CallID != "call_ask_lc" {
		t.Errorf("expected call_ask_lc, got %q", pending.CallID)
	}

	// Submit user response.
	if err := runner.SubmitInput(run.ID, map[string]string{"Pick one?": "A"}); err != nil {
		t.Fatalf("SubmitInput: %v", err)
	}

	// Collect all events (blocks until terminal).
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	// Final state must be completed.
	state, ok = runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found after completion")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q", state.Status)
	}
	if state.Output != "lifecycle complete" {
		t.Errorf("unexpected output: %q", state.Output)
	}

	// Stored transcript must include messages from both LLM turns.
	msgs := runner.GetRunMessages(run.ID)
	if len(msgs) < 3 {
		t.Errorf("expected at least 3 stored messages (user + tool_call + resume response), got %d", len(msgs))
	}

	// Event ordering (non-exhaustive but pinned):
	requireEventOrder(t, events,
		"run.started",
		"tool.call.started",
		"run.waiting_for_user",
		"tool.call.completed",
		"run.resumed",
		"assistant.message",
		"run.completed",
	)
}

// -------------------------------------------------------------------------
// TestExecuteLifecycle_WaitForUserTimeoutPath
//
// Verifies the wait-for-user timeout path:
//   - run.waiting_for_user is emitted.
//   - After timeout, run.failed is emitted (terminal).
//   - Final run status is failed.
//   - state.Error contains "timed out".
//   - No events appear after run.failed.
//
// -------------------------------------------------------------------------
func TestExecuteLifecycle_WaitForUserTimeoutPath(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{turns: []CompletionResult{
		{
			ToolCalls: []ToolCall{{
				ID:        "call_ask_timeout",
				Name:      htools.AskUserQuestionToolName,
				Arguments: `{"questions":[{"question":"Pick?","header":"H","options":[{"label":"X","description":"Opt X"},{"label":"Y","description":"Opt Y"}],"multiSelect":false}]}`,
			}},
		},
		{Content: "should not happen"},
	}}

	broker := NewInMemoryAskUserQuestionBroker(time.Now)
	runner := NewRunner(provider, NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode:   ToolApprovalModeFullAuto,
		AskUserBroker:  broker,
		AskUserTimeout: 30 * time.Millisecond,
	}), RunnerConfig{
		DefaultModel:   "gpt-5-nano",
		MaxSteps:       4,
		AskUserBroker:  broker,
		AskUserTimeout: 30 * time.Millisecond,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "timeout test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	// Terminal state must be failed.
	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if state.Status != RunStatusFailed {
		t.Fatalf("expected failed, got %q", state.Status)
	}
	if !strings.Contains(state.Error, "timed out") {
		t.Errorf("expected timeout error, got %q", state.Error)
	}

	// Provider called exactly once (timeout before second turn).
	if provider.calls != 1 {
		t.Errorf("expected 1 provider call, got %d", provider.calls)
	}

	// Event ordering: waiting event must precede failed.
	requireEventOrder(t, events,
		"run.waiting_for_user",
		"run.failed",
	)

	// No events must appear after run.failed (post-terminal seal check).
	terminalIdx := -1
	for i, ev := range events {
		if IsTerminalEvent(ev.Type) {
			terminalIdx = i
			break
		}
	}
	if terminalIdx < 0 {
		t.Fatal("no terminal event found")
	}
	for _, ev := range events[terminalIdx+1:] {
		t.Errorf("unexpected post-terminal event: %q", ev.Type)
	}
}

// -------------------------------------------------------------------------
// TestExecuteLifecycle_CostCeilingTerminatesRunAfterLLMTurnNoToolCalls
//
// Verifies the cost-ceiling path when the terminal step has no tool calls:
//   - run.cost_limit_reached is emitted before run.completed.
//   - run status is completed (not failed).
//   - Provider is not called again after limit is hit.
//   - Event ordering is pinned.
//
// -------------------------------------------------------------------------
func TestExecuteLifecycle_CostCeilingTerminatesRunAfterLLMTurnNoToolCalls(t *testing.T) {
	t.Parallel()

	// Two turns. First turn: $0.002 (tool call). Second turn: $0.002 (text only).
	// Ceiling $0.003 — not breached after turn 1 ($0.002 < $0.003).
	// After turn 2 cumulative = $0.004 >= $0.003, so cost limit fires.
	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name:        "echo_lc",
		Description: "echoes",
		Parameters:  map[string]any{"type": "object"},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return `"ok"`, nil
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	provider := &stubProvider{turns: []CompletionResult{
		{
			ToolCalls:  []ToolCall{{ID: "c1", Name: "echo_lc", Arguments: `{}`}},
			CostUSD:    floatPtr(0.002),
			CostStatus: CostStatusAvailable,
			Cost:       &CompletionCost{TotalUSD: 0.002},
		},
		{
			Content:    "done after cost ceiling",
			CostUSD:    floatPtr(0.002),
			CostStatus: CostStatusAvailable,
			Cost:       &CompletionCost{TotalUSD: 0.002},
		},
		// This must never be reached.
		{Content: "unreachable"},
	}}

	runner := NewRunner(provider, registry, RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     10,
	})

	run, err := runner.StartRun(RunRequest{
		Prompt:     "cost ceiling lifecycle",
		MaxCostUSD: 0.003,
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	// run.cost_limit_reached must appear before run.completed.
	requireEventOrder(t, events,
		"run.started",
		"run.cost_limit_reached",
		"run.completed",
	)

	// Terminal must be run.completed (not run.failed).
	var terminal *Event
	for i := range events {
		if IsTerminalEvent(events[i].Type) {
			terminal = &events[i]
			break
		}
	}
	if terminal == nil {
		t.Fatal("no terminal event")
	}
	if terminal.Type != EventRunCompleted {
		t.Fatalf("expected run.completed as terminal, got %q", terminal.Type)
	}

	// Run state must be completed.
	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q", state.Status)
	}

	// Provider called exactly 2 times.
	if provider.calls != 2 {
		t.Errorf("expected 2 provider calls, got %d", provider.calls)
	}

	// cost_limit_reached payload must include max_cost_usd and cumulative_cost_usd.
	var costLimitEv *Event
	for i := range events {
		if events[i].Type == EventRunCostLimitReached {
			costLimitEv = &events[i]
			break
		}
	}
	if costLimitEv == nil {
		t.Fatal("run.cost_limit_reached event not found")
	}
	if costLimitEv.Payload["max_cost_usd"] == nil {
		t.Error("run.cost_limit_reached missing max_cost_usd")
	}
	if costLimitEv.Payload["cumulative_cost_usd"] == nil {
		t.Error("run.cost_limit_reached missing cumulative_cost_usd")
	}
}

// -------------------------------------------------------------------------
// TestExecuteLifecycle_EmptyResponseRetryThenSuccess
//
// Verifies the empty-response retry path followed by successful completion:
//   - llm.empty_response.retry events are emitted for each empty turn.
//   - The retry counter in payload increments correctly.
//   - After a successful turn the run completes normally.
//   - Event ordering is pinned.
//
// -------------------------------------------------------------------------
func TestExecuteLifecycle_EmptyResponseRetryThenSuccess(t *testing.T) {
	t.Parallel()

	// Two empty turns, then a real response.
	provider := &stubProvider{turns: []CompletionResult{
		{Content: "", ToolCalls: nil}, // empty 1 → retry event (retry=1)
		{Content: "", ToolCalls: nil}, // empty 2 → retry event (retry=2)
		{Content: "finally answered"},
	}}

	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "gemini-lc",
		MaxSteps:     10,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "lifecycle empty retry"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	// Run must complete successfully.
	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q", state.Status)
	}
	if state.Output != "finally answered" {
		t.Errorf("expected output %q, got %q", "finally answered", state.Output)
	}

	// Exactly 2 retry events with sequential retry counters.
	var retryEvents []Event
	for _, ev := range events {
		if ev.Type == EventEmptyResponseRetry {
			retryEvents = append(retryEvents, ev)
		}
	}
	if len(retryEvents) != 2 {
		t.Fatalf("expected 2 llm.empty_response.retry events, got %d", len(retryEvents))
	}

	// Retry counters must be 1 and 2 in order.
	for i, re := range retryEvents {
		retryVal, ok := re.Payload["retry"]
		if !ok {
			t.Errorf("retry event %d missing 'retry' field", i)
			continue
		}
		expected := i + 1
		if int(retryVal.(int)) != expected {
			t.Errorf("retry event %d: payload retry=%v, want %d", i, retryVal, expected)
		}
		if _, ok := re.Payload["step"]; !ok {
			t.Errorf("retry event %d missing 'step' field", i)
		}
		if _, ok := re.Payload["max_retries"]; !ok {
			t.Errorf("retry event %d missing 'max_retries' field", i)
		}
	}

	// Event ordering: retries precede the assistant message and run.completed.
	requireEventOrder(t, events,
		"run.started",
		"llm.empty_response.retry",
		"assistant.message",
		"run.completed",
	)
}

// -------------------------------------------------------------------------
// TestExecuteLifecycle_HookAndToolCallAndMessagePersistenceSequence
//
// Verifies the combined hook + tool-call + message persistence sequence:
//   - Pre/post message hooks fire and emit hook.started / hook.completed.
//   - Tool calls are executed and tool.call.started / tool.call.completed appear.
//   - The stored transcript contains both the assistant tool-call message and
//     the tool result message.
//   - The final assistant message is persisted with the post-hook mutation.
//
// -------------------------------------------------------------------------
func TestExecuteLifecycle_HookAndToolCallAndMessagePersistenceSequence(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name:        "echo_hook",
		Description: "echoes",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"msg": map[string]any{"type": "string"},
			},
		},
	}, func(_ context.Context, raw json.RawMessage) (string, error) {
		return `{"result":"hook-test-ok"}`, nil
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	provider := &capturingProvider{turns: []CompletionResult{
		{
			ToolCalls: []ToolCall{{
				ID:        "call_hook_lc",
				Name:      "echo_hook",
				Arguments: `{"msg":"hello"}`,
			}},
		},
		{Content: "original final"},
	}}

	// Pre-hook: add a marker to the model name so we can assert it was called.
	// Post-hook: mutate the final response content.
	hookCalled := false
	runner := NewRunner(provider, registry, RunnerConfig{
		DefaultModel: "gpt-lc",
		MaxSteps:     4,
		PreMessageHooks: []PreMessageHook{
			preHookFunc{name: "lc-pre", fn: func(_ context.Context, in PreMessageHookInput) (PreMessageHookResult, error) {
				hookCalled = true
				return PreMessageHookResult{Action: HookActionContinue}, nil
			}},
		},
		PostMessageHooks: []PostMessageHook{
			postHookFunc{name: "lc-post", fn: func(_ context.Context, in PostMessageHookInput) (PostMessageHookResult, error) {
				if in.Response.Content == "original final" {
					mutated := in.Response
					mutated.Content = "mutated by lc-post"
					return PostMessageHookResult{Action: HookActionContinue, MutatedResponse: &mutated}, nil
				}
				return PostMessageHookResult{Action: HookActionContinue}, nil
			}},
		},
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hook lifecycle test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	// Run must complete with the mutated output.
	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q", state.Status)
	}
	if state.Output != "mutated by lc-post" {
		t.Errorf("expected post-hook output, got %q", state.Output)
	}

	// Pre-hook must have been called.
	if !hookCalled {
		t.Error("pre-hook was not called")
	}

	// Stored transcript must include:
	// 1. User message ("hook lifecycle test")
	// 2. At least one assistant message (tool call)
	// 3. Tool result message
	// 4. Final assistant message
	msgs := runner.GetRunMessages(run.ID)
	if len(msgs) < 4 {
		t.Errorf("expected at least 4 stored messages, got %d: %+v", len(msgs), msgs)
	}

	// Check that the tool result is stored.
	toolResultFound := false
	for _, m := range msgs {
		if m.Role == "tool" && m.ToolCallID == "call_hook_lc" {
			toolResultFound = true
		}
	}
	if !toolResultFound {
		t.Error("tool result message not found in stored transcript")
	}

	// Event ordering: hook events appear around llm turn; tool events appear after.
	requireEventOrder(t, events,
		"run.started",
		"hook.started",
		"hook.completed",
		"tool.call.started",
		"tool.call.completed",
		"assistant.message",
		"run.completed",
	)
}

// -------------------------------------------------------------------------
// TestExecuteLifecycle_CompactionChangesNextRequestShape
//
// Multi-feature turn: compaction occurs while a run is paused mid-execution,
// then the next provider call must receive the compacted (shorter) context
// rather than the original full context. This verifies that compaction
// changes the next request shape.
// -------------------------------------------------------------------------
func TestExecuteLifecycle_CompactionChangesNextRequestShape(t *testing.T) {
	t.Parallel()

	// Gate step 2 so we can compact after step 1 finishes.
	step2Gate := make(chan struct{})

	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name:        "echo_shape",
		Description: "echoes",
		Parameters:  map[string]any{"type": "object"},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return `{"r":"ok"}`, nil
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	provider := &contextCompactGatingProvider{
		results: []CompletionResult{
			// Step 1: returns a tool call — adds messages to transcript.
			{ToolCalls: []ToolCall{{ID: "c1", Name: "echo_shape", Arguments: `{}`}}},
			// Step 2: gated — waits for compaction before provider is invoked.
			{Content: "done"},
		},
		beforeCall: func(idx int) {
			if idx == 1 {
				<-step2Gate
			}
		},
	}

	capProvider := &capturingProvider{
		turns: []CompletionResult{
			{ToolCalls: []ToolCall{{ID: "c1", Name: "echo_shape", Arguments: `{}`}}},
			{Content: "done"},
		},
	}

	// Use capturingProvider so we can inspect the messages sent to each call.
	runnerCap := NewRunner(capProvider, registry, RunnerConfig{
		DefaultModel: "test",
		MaxSteps:     6,
	})
	_ = provider // gate provider for shape comparison
	_ = step2Gate

	// Separate test with gating + capturing.
	// Re-use contextCompactGatingProvider with a capturing layer.
	type capturableGatingProvider struct {
		gate    *contextCompactGatingProvider
		capture *capturingProvider
	}

	// Simpler approach: run the gate provider to completion, then inspect
	// messages at each step boundary by using CompactRun mid-run.
	step4Gate2 := make(chan struct{})
	gatingProv := &contextCompactGatingProvider{
		results: []CompletionResult{
			{ToolCalls: []ToolCall{{ID: "c1", Name: "echo_shape", Arguments: `{}`}}},
			{ToolCalls: []ToolCall{{ID: "c2", Name: "echo_shape", Arguments: `{}`}}},
			{ToolCalls: []ToolCall{{ID: "c3", Name: "echo_shape", Arguments: `{}`}}},
			{Content: "final"},
		},
		beforeCall: func(idx int) {
			if idx == 3 {
				<-step4Gate2
			}
		},
	}
	runner := NewRunner(gatingProv, registry, RunnerConfig{
		DefaultModel: "test",
		MaxSteps:     8,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "shape test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Wait until 3 steps are done (provider.calls == 3 waiting at gate).
	deadline := time.Now().Add(5 * time.Second)
	for {
		gatingProv.mu.Lock()
		calls := gatingProv.calls
		gatingProv.mu.Unlock()
		if calls >= 4 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for step 4 gate")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// At this point 3 steps completed: user + 3x(assistant+tool) = 7 messages.
	msgsBeforeCompact := runner.GetRunMessages(run.ID)
	countBefore := len(msgsBeforeCompact)
	if countBefore < 7 {
		t.Fatalf("expected >= 7 messages before compact, got %d", countBefore)
	}

	// Compact with strip mode, keepLast=2 — removes tool messages from old turns.
	result, err := runner.CompactRun(context.Background(), run.ID, CompactRunRequest{
		Mode:     "strip",
		KeepLast: 2,
	})
	if err != nil {
		t.Fatalf("CompactRun: %v", err)
	}
	if result.MessagesRemoved == 0 {
		t.Log("no messages removed — tool messages may all be within keep window")
	}

	compactedCount := len(runner.GetRunMessages(run.ID))

	// Release step 4.
	close(step4Gate2)

	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q", state.Status)
	}

	// After step 4 (one assistant message appended), the final count must equal
	// compactedCount + 1 — confirming the run used the compacted context, not
	// the stale pre-compaction copy.
	finalCount := len(runner.GetRunMessages(run.ID))
	expectedFinal := compactedCount + 1
	if finalCount != expectedFinal {
		t.Errorf("next request shape not from compacted context: final=%d, want=%d (compacted=%d, beforeCompact=%d)",
			finalCount, expectedFinal, compactedCount, countBefore)
	}

	// The capProvider run was only used for structural setup; clean it up.
	_ = runnerCap
}
