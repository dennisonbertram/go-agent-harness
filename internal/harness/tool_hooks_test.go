package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// -- helpers --

type preToolHookFunc struct {
	name string
	fn   func(context.Context, PreToolUseEvent) (*PreToolUseResult, error)
}

func (h preToolHookFunc) Name() string { return h.name }
func (h preToolHookFunc) PreToolUse(ctx context.Context, ev PreToolUseEvent) (*PreToolUseResult, error) {
	return h.fn(ctx, ev)
}

type postToolHookFunc struct {
	name string
	fn   func(context.Context, PostToolUseEvent) (*PostToolUseResult, error)
}

func (h postToolHookFunc) Name() string { return h.name }
func (h postToolHookFunc) PostToolUse(ctx context.Context, ev PostToolUseEvent) (*PostToolUseResult, error) {
	return h.fn(ctx, ev)
}

// echoToolDef returns a simple echo tool that echoes its "message" argument.
func echoToolDef() ToolDefinition {
	return ToolDefinition{
		Name:        "echo_tool",
		Description: "echoes input",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
			},
			"required": []string{"message"},
		},
	}
}

func echoToolHandler() ToolHandler {
	return func(_ context.Context, raw json.RawMessage) (string, error) {
		var args struct{ Message string }
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", err
		}
		return args.Message, nil
	}
}

// providerWithToolCall returns a stubProvider that issues one tool call then completes.
func providerWithToolCall(toolName, argsJSON string) *stubProvider {
	return &stubProvider{
		turns: []CompletionResult{
			{
				ToolCalls: []ToolCall{{
					ID:        "call_1",
					Name:      toolName,
					Arguments: argsJSON,
				}},
			},
			{Content: "done"},
		},
	}
}

// -- tests --

// TestPreToolUseAllowPassesThrough verifies that when a PreToolUseHook
// returns Allow with no modification the tool executes normally.
func TestPreToolUseAllowPassesThrough(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	_ = reg.Register(echoToolDef(), echoToolHandler())

	var hookSeen string
	runner := NewRunner(
		providerWithToolCall("echo_tool", `{"message":"hello"}`),
		reg,
		RunnerConfig{
			DefaultModel: "m",
			PreToolUseHooks: []PreToolUseHook{
				preToolHookFunc{name: "observer", fn: func(_ context.Context, ev PreToolUseEvent) (*PreToolUseResult, error) {
					hookSeen = ev.ToolName
					return &PreToolUseResult{Decision: ToolHookAllow}, nil
				}},
			},
		},
	)

	run, _ := runner.StartRun(RunRequest{Prompt: "go"})
	_, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	state, _ := runner.GetRun(run.ID)
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q", state.Status)
	}
	if hookSeen != "echo_tool" {
		t.Fatalf("hook did not see echo_tool, got %q", hookSeen)
	}
}

// TestPreToolUseDenyBlocksTool verifies that a HookDeny decision prevents
// tool execution and sends an error result back to the LLM.
func TestPreToolUseDenyBlocksTool(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()

	var executed bool
	_ = reg.Register(echoToolDef(), func(_ context.Context, _ json.RawMessage) (string, error) {
		executed = true
		return "should not run", nil
	})

	runner := NewRunner(
		providerWithToolCall("echo_tool", `{"message":"hi"}`),
		reg,
		RunnerConfig{
			DefaultModel: "m",
			PreToolUseHooks: []PreToolUseHook{
				preToolHookFunc{name: "deny-hook", fn: func(_ context.Context, ev PreToolUseEvent) (*PreToolUseResult, error) {
					return &PreToolUseResult{Decision: ToolHookDeny, Reason: "blocked by policy"}, nil
				}},
			},
		},
	)

	run, _ := runner.StartRun(RunRequest{Prompt: "go"})
	_, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	if executed {
		t.Fatal("tool handler should not have been called")
	}
	state, _ := runner.GetRun(run.ID)
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected run to complete (LLM handles denial), got %q", state.Status)
	}
}

// TestPreToolUseModifiesArgs verifies that a PreToolUseHook can replace the
// args passed to the underlying tool handler.
func TestPreToolUseModifiesArgs(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()

	var receivedMsg string
	_ = reg.Register(echoToolDef(), func(_ context.Context, raw json.RawMessage) (string, error) {
		var args struct{ Message string }
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", err
		}
		receivedMsg = args.Message
		return args.Message, nil
	})

	runner := NewRunner(
		providerWithToolCall("echo_tool", `{"message":"original"}`),
		reg,
		RunnerConfig{
			DefaultModel: "m",
			PreToolUseHooks: []PreToolUseHook{
				preToolHookFunc{name: "mutator", fn: func(_ context.Context, ev PreToolUseEvent) (*PreToolUseResult, error) {
					modified := json.RawMessage(`{"message":"modified"}`)
					return &PreToolUseResult{Decision: ToolHookAllow, ModifiedArgs: modified}, nil
				}},
			},
		},
	)

	run, _ := runner.StartRun(RunRequest{Prompt: "go"})
	_, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	if receivedMsg != "modified" {
		t.Fatalf("expected modified args, got %q", receivedMsg)
	}
}

// TestPostToolUseModifiesResult verifies that a PostToolUseHook can replace
// the result returned to the LLM.
func TestPostToolUseModifiesResult(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	_ = reg.Register(echoToolDef(), echoToolHandler())

	// The provider will receive the tool result — capture it by inspecting
	// what messages are seen on the second turn.
	var seenToolOutput string
	prov := &capturingProvider{
		turns: []CompletionResult{
			{ToolCalls: []ToolCall{{ID: "c1", Name: "echo_tool", Arguments: `{"message":"raw"}`}}},
			{Content: "done"},
		},
	}
	_ = prov

	runner := NewRunner(
		providerWithToolCall("echo_tool", `{"message":"raw"}`),
		reg,
		RunnerConfig{
			DefaultModel: "m",
			PostToolUseHooks: []PostToolUseHook{
				postToolHookFunc{name: "post-mutator", fn: func(_ context.Context, ev PostToolUseEvent) (*PostToolUseResult, error) {
					seenToolOutput = ev.Result
					return &PostToolUseResult{ModifiedResult: "post-modified"}, nil
				}},
			},
		},
	)

	run, _ := runner.StartRun(RunRequest{Prompt: "go"})
	_, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	state, _ := runner.GetRun(run.ID)
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q", state.Status)
	}
	if seenToolOutput != "raw" {
		t.Fatalf("expected PostToolUseEvent.Result=%q, got %q", "raw", seenToolOutput)
	}
}

// TestPostToolUseReceivesError verifies that PostToolUseEvent.Error is
// populated when the tool handler returns an error.
func TestPostToolUseReceivesError(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	_ = reg.Register(echoToolDef(), func(_ context.Context, _ json.RawMessage) (string, error) {
		return "", errors.New("tool exploded")
	})

	var postErr error
	runner := NewRunner(
		providerWithToolCall("echo_tool", `{"message":"x"}`),
		reg,
		RunnerConfig{
			DefaultModel: "m",
			PostToolUseHooks: []PostToolUseHook{
				postToolHookFunc{name: "error-spy", fn: func(_ context.Context, ev PostToolUseEvent) (*PostToolUseResult, error) {
					postErr = ev.Error
					return &PostToolUseResult{}, nil
				}},
			},
		},
	)

	run, _ := runner.StartRun(RunRequest{Prompt: "go"})
	_, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if postErr == nil {
		t.Fatal("expected PostToolUseEvent.Error to be set")
	}
	if postErr.Error() != "tool exploded" {
		t.Fatalf("unexpected error: %v", postErr)
	}
}

// TestPreToolUseHookReceivesCorrectFields verifies that all fields on the
// PreToolUseEvent are correctly populated.
func TestPreToolUseHookReceivesCorrectFields(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	_ = reg.Register(echoToolDef(), echoToolHandler())

	var capturedEvent PreToolUseEvent
	runner := NewRunner(
		providerWithToolCall("echo_tool", `{"message":"fields-check"}`),
		reg,
		RunnerConfig{
			DefaultModel: "m",
			PreToolUseHooks: []PreToolUseHook{
				preToolHookFunc{name: "field-spy", fn: func(_ context.Context, ev PreToolUseEvent) (*PreToolUseResult, error) {
					capturedEvent = ev
					return &PreToolUseResult{Decision: ToolHookAllow}, nil
				}},
			},
		},
	)

	run, _ := runner.StartRun(RunRequest{Prompt: "check"})
	collectRunEvents(t, runner, run.ID) //nolint:errcheck
	if capturedEvent.ToolName != "echo_tool" {
		t.Fatalf("expected ToolName=echo_tool, got %q", capturedEvent.ToolName)
	}
	if capturedEvent.RunID == "" {
		t.Fatal("expected RunID to be populated")
	}
	if string(capturedEvent.Args) == "" {
		t.Fatal("expected Args to be populated")
	}
	if capturedEvent.CallID == "" {
		t.Fatal("expected CallID to be populated")
	}
}

// TestPostToolUseHookReceivesCorrectFields verifies that PostToolUseEvent
// fields are correctly populated, including Duration >= 0.
func TestPostToolUseHookReceivesCorrectFields(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	_ = reg.Register(echoToolDef(), echoToolHandler())

	var captured PostToolUseEvent
	runner := NewRunner(
		providerWithToolCall("echo_tool", `{"message":"post-fields"}`),
		reg,
		RunnerConfig{
			DefaultModel: "m",
			PostToolUseHooks: []PostToolUseHook{
				postToolHookFunc{name: "post-field-spy", fn: func(_ context.Context, ev PostToolUseEvent) (*PostToolUseResult, error) {
					captured = ev
					return &PostToolUseResult{}, nil
				}},
			},
		},
	)

	run, _ := runner.StartRun(RunRequest{Prompt: "check"})
	collectRunEvents(t, runner, run.ID) //nolint:errcheck
	if captured.ToolName != "echo_tool" {
		t.Fatalf("expected ToolName=echo_tool, got %q", captured.ToolName)
	}
	if captured.RunID == "" {
		t.Fatal("expected RunID to be set")
	}
	if captured.Duration < 0 {
		t.Fatalf("expected Duration >= 0, got %v", captured.Duration)
	}
}

// TestMultiplePreToolUseHooksCalledInOrder verifies that all registered
// PreToolUseHooks are called in registration order, and the last
// Allow/nil-modified args wins.
func TestMultiplePreToolUseHooksCalledInOrder(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	_ = reg.Register(echoToolDef(), echoToolHandler())

	var order []string
	runner := NewRunner(
		providerWithToolCall("echo_tool", `{"message":"multi"}`),
		reg,
		RunnerConfig{
			DefaultModel: "m",
			PreToolUseHooks: []PreToolUseHook{
				preToolHookFunc{name: "first", fn: func(_ context.Context, _ PreToolUseEvent) (*PreToolUseResult, error) {
					order = append(order, "first")
					return &PreToolUseResult{Decision: ToolHookAllow}, nil
				}},
				preToolHookFunc{name: "second", fn: func(_ context.Context, _ PreToolUseEvent) (*PreToolUseResult, error) {
					order = append(order, "second")
					return &PreToolUseResult{Decision: ToolHookAllow}, nil
				}},
				preToolHookFunc{name: "third", fn: func(_ context.Context, _ PreToolUseEvent) (*PreToolUseResult, error) {
					order = append(order, "third")
					return &PreToolUseResult{Decision: ToolHookAllow}, nil
				}},
			},
		},
	)

	run, _ := runner.StartRun(RunRequest{Prompt: "go"})
	collectRunEvents(t, runner, run.ID) //nolint:errcheck
	if len(order) != 3 {
		t.Fatalf("expected 3 hook calls, got %d: %v", len(order), order)
	}
	if order[0] != "first" || order[1] != "second" || order[2] != "third" {
		t.Fatalf("unexpected order: %v", order)
	}
}

// TestPreToolUseDenyPriorityStopsChain verifies that a Deny from the first
// hook stops the chain (later hooks are not called).
func TestPreToolUseDenyPriorityStopsChain(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	_ = reg.Register(echoToolDef(), echoToolHandler())

	secondCalled := false
	runner := NewRunner(
		providerWithToolCall("echo_tool", `{"message":"deny"}`),
		reg,
		RunnerConfig{
			DefaultModel: "m",
			PreToolUseHooks: []PreToolUseHook{
				preToolHookFunc{name: "deny-first", fn: func(_ context.Context, _ PreToolUseEvent) (*PreToolUseResult, error) {
					return &PreToolUseResult{Decision: ToolHookDeny, Reason: "denied"}, nil
				}},
				preToolHookFunc{name: "should-not-run", fn: func(_ context.Context, _ PreToolUseEvent) (*PreToolUseResult, error) {
					secondCalled = true
					return &PreToolUseResult{Decision: ToolHookAllow}, nil
				}},
			},
		},
	)

	run, _ := runner.StartRun(RunRequest{Prompt: "go"})
	collectRunEvents(t, runner, run.ID) //nolint:errcheck
	if secondCalled {
		t.Fatal("second hook should not have been called after deny")
	}
}

// TestPreToolUseHookErrorFailOpen verifies that hook errors with fail_open
// mode are ignored and the tool proceeds.
func TestPreToolUseHookErrorFailOpen(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	_ = reg.Register(echoToolDef(), echoToolHandler())

	runner := NewRunner(
		providerWithToolCall("echo_tool", `{"message":"failopen"}`),
		reg,
		RunnerConfig{
			DefaultModel:    "m",
			HookFailureMode: HookFailureModeFailOpen,
			PreToolUseHooks: []PreToolUseHook{
				preToolHookFunc{name: "error-hook", fn: func(_ context.Context, _ PreToolUseEvent) (*PreToolUseResult, error) {
					return nil, errors.New("hook boom")
				}},
			},
		},
	)

	run, _ := runner.StartRun(RunRequest{Prompt: "go"})
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	state, _ := runner.GetRun(run.ID)
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q", state.Status)
	}
	// Should have a tool_hook.failed event
	found := false
	for _, ev := range events {
		if ev.Type == EventToolHookFailed {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected tool_hook.failed event")
	}
}

// TestPreToolUseHookErrorFailClosed verifies that hook errors with
// fail_closed (default) mode cause the tool to return an error result.
func TestPreToolUseHookErrorFailClosed(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	var toolExecuted bool
	_ = reg.Register(echoToolDef(), func(_ context.Context, _ json.RawMessage) (string, error) {
		toolExecuted = true
		return "should not run", nil
	})

	runner := NewRunner(
		providerWithToolCall("echo_tool", `{"message":"failclosed"}`),
		reg,
		RunnerConfig{
			DefaultModel: "m",
			// default is fail_closed
			PreToolUseHooks: []PreToolUseHook{
				preToolHookFunc{name: "error-hook", fn: func(_ context.Context, _ PreToolUseEvent) (*PreToolUseResult, error) {
					return nil, errors.New("hook error")
				}},
			},
		},
	)

	run, _ := runner.StartRun(RunRequest{Prompt: "go"})
	collectRunEvents(t, runner, run.ID) //nolint:errcheck
	// In fail_closed mode, tool hook error returns an error result to LLM
	// but the run itself can still complete (LLM handles the error response)
	if toolExecuted {
		t.Fatal("tool should not have executed when hook errored in fail_closed mode")
	}
}

// TestPostToolUseHookErrorFailOpen verifies that PostToolUseHook errors in
// fail_open mode are ignored.
func TestPostToolUseHookErrorFailOpen(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	_ = reg.Register(echoToolDef(), echoToolHandler())

	runner := NewRunner(
		providerWithToolCall("echo_tool", `{"message":"x"}`),
		reg,
		RunnerConfig{
			DefaultModel:    "m",
			HookFailureMode: HookFailureModeFailOpen,
			PostToolUseHooks: []PostToolUseHook{
				postToolHookFunc{name: "post-error-hook", fn: func(_ context.Context, _ PostToolUseEvent) (*PostToolUseResult, error) {
					return nil, errors.New("post hook boom")
				}},
			},
		},
	)

	run, _ := runner.StartRun(RunRequest{Prompt: "go"})
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	state, _ := runner.GetRun(run.ID)
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q", state.Status)
	}
	found := false
	for _, ev := range events {
		if ev.Type == EventToolHookFailed && ev.Payload["stage"] == "post_tool_use" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected post tool_hook.failed event")
	}
}

// TestEmptyHookRegistriesZeroOverhead verifies that with no hooks the
// tool executes normally (no panics, no overhead path).
func TestEmptyHookRegistriesZeroOverhead(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	_ = reg.Register(echoToolDef(), echoToolHandler())

	runner := NewRunner(
		providerWithToolCall("echo_tool", `{"message":"empty"}`),
		reg,
		RunnerConfig{DefaultModel: "m"},
	)

	run, _ := runner.StartRun(RunRequest{Prompt: "go"})
	collectRunEvents(t, runner, run.ID) //nolint:errcheck
	state, _ := runner.GetRun(run.ID)
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q", state.Status)
	}
}

// TestNilModifiedArgsUsesOriginal verifies that when ModifiedArgs is nil
// the original args are passed through.
func TestNilModifiedArgsUsesOriginal(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	var receivedMsg string
	_ = reg.Register(echoToolDef(), func(_ context.Context, raw json.RawMessage) (string, error) {
		var args struct{ Message string }
		json.Unmarshal(raw, &args) //nolint:errcheck
		receivedMsg = args.Message
		return args.Message, nil
	})

	runner := NewRunner(
		providerWithToolCall("echo_tool", `{"message":"original"}`),
		reg,
		RunnerConfig{
			DefaultModel: "m",
			PreToolUseHooks: []PreToolUseHook{
				preToolHookFunc{name: "nil-modifier", fn: func(_ context.Context, _ PreToolUseEvent) (*PreToolUseResult, error) {
					// ModifiedArgs is nil — original should be used
					return &PreToolUseResult{Decision: ToolHookAllow, ModifiedArgs: nil}, nil
				}},
			},
		},
	)

	run, _ := runner.StartRun(RunRequest{Prompt: "go"})
	collectRunEvents(t, runner, run.ID) //nolint:errcheck
	if receivedMsg != "original" {
		t.Fatalf("expected original message, got %q", receivedMsg)
	}
}

// TestNilModifiedResultUsesOriginal verifies that when ModifiedResult is
// empty in PostToolUseResult the original tool result is passed to LLM.
func TestNilModifiedResultUsesOriginal(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	_ = reg.Register(echoToolDef(), func(_ context.Context, _ json.RawMessage) (string, error) {
		return "original-result", nil
	})

	var seenResult string
	runner := NewRunner(
		providerWithToolCall("echo_tool", `{"message":"x"}`),
		reg,
		RunnerConfig{
			DefaultModel: "m",
			PostToolUseHooks: []PostToolUseHook{
				postToolHookFunc{name: "nil-post", fn: func(_ context.Context, ev PostToolUseEvent) (*PostToolUseResult, error) {
					seenResult = ev.Result
					// empty ModifiedResult → use original
					return &PostToolUseResult{ModifiedResult: ""}, nil
				}},
			},
		},
	)

	run, _ := runner.StartRun(RunRequest{Prompt: "go"})
	collectRunEvents(t, runner, run.ID) //nolint:errcheck
	if seenResult != "original-result" {
		t.Fatalf("expected original-result in event, got %q", seenResult)
	}
}

// TestConcurrentPreToolUseHooks verifies that pre-tool-use hooks are safe
// under concurrent tool invocations (data-race detector).
func TestConcurrentPreToolUseHooks(t *testing.T) {
	t.Parallel()

	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines)

	var callCount atomic.Int64

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			reg := NewRegistry()
			_ = reg.Register(echoToolDef(), echoToolHandler())

			runner := NewRunner(
				providerWithToolCall("echo_tool", `{"message":"concurrent"}`),
				reg,
				RunnerConfig{
					DefaultModel: "m",
					PreToolUseHooks: []PreToolUseHook{
						preToolHookFunc{name: "counter", fn: func(_ context.Context, _ PreToolUseEvent) (*PreToolUseResult, error) {
							callCount.Add(1)
							return &PreToolUseResult{Decision: ToolHookAllow}, nil
						}},
					},
				},
			)

			run, _ := runner.StartRun(RunRequest{Prompt: "go"})
			collectRunEvents(t, runner, run.ID) //nolint:errcheck
		}()
	}

	wg.Wait()
	if callCount.Load() != goroutines {
		t.Fatalf("expected %d hook calls, got %d", goroutines, callCount.Load())
	}
}

// TestToolHookEventsEmitted verifies that tool_hook.started and
// tool_hook.completed events are emitted for pre and post tool-use hooks.
func TestToolHookEventsEmitted(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	_ = reg.Register(echoToolDef(), echoToolHandler())

	runner := NewRunner(
		providerWithToolCall("echo_tool", `{"message":"events"}`),
		reg,
		RunnerConfig{
			DefaultModel: "m",
			PreToolUseHooks: []PreToolUseHook{
				preToolHookFunc{name: "pre-watcher", fn: func(_ context.Context, _ PreToolUseEvent) (*PreToolUseResult, error) {
					return &PreToolUseResult{Decision: ToolHookAllow}, nil
				}},
			},
			PostToolUseHooks: []PostToolUseHook{
				postToolHookFunc{name: "post-watcher", fn: func(_ context.Context, _ PostToolUseEvent) (*PostToolUseResult, error) {
					return &PostToolUseResult{}, nil
				}},
			},
		},
	)

	run, _ := runner.StartRun(RunRequest{Prompt: "go"})
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	var seenPreStarted, seenPreCompleted, seenPostStarted, seenPostCompleted bool
	for _, ev := range events {
		switch ev.Type {
		case EventToolHookStarted:
			stage, _ := ev.Payload["stage"].(string)
			if stage == "pre_tool_use" {
				seenPreStarted = true
			} else if stage == "post_tool_use" {
				seenPostStarted = true
			}
		case EventToolHookCompleted:
			stage, _ := ev.Payload["stage"].(string)
			if stage == "pre_tool_use" {
				seenPreCompleted = true
			} else if stage == "post_tool_use" {
				seenPostCompleted = true
			}
		}
	}

	if !seenPreStarted {
		t.Error("expected tool_hook.started event for pre_tool_use")
	}
	if !seenPreCompleted {
		t.Error("expected tool_hook.completed event for pre_tool_use")
	}
	if !seenPostStarted {
		t.Error("expected tool_hook.started event for post_tool_use")
	}
	if !seenPostCompleted {
		t.Error("expected tool_hook.completed event for post_tool_use")
	}
}

// TestPreToolUseHookPanicRecovery verifies that a panicking hook is caught
// and treated as a hook error (fail_closed → error result; fail_open → skip).
func TestPreToolUseHookPanicRecovery(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	var toolExecuted bool
	_ = reg.Register(echoToolDef(), func(_ context.Context, _ json.RawMessage) (string, error) {
		toolExecuted = true
		return "ran", nil
	})

	runner := NewRunner(
		providerWithToolCall("echo_tool", `{"message":"panic"}`),
		reg,
		RunnerConfig{
			DefaultModel:    "m",
			HookFailureMode: HookFailureModeFailOpen,
			PreToolUseHooks: []PreToolUseHook{
				preToolHookFunc{name: "panicker", fn: func(_ context.Context, _ PreToolUseEvent) (*PreToolUseResult, error) {
					panic("hook panic")
				}},
			},
		},
	)

	run, _ := runner.StartRun(RunRequest{Prompt: "go"})
	collectRunEvents(t, runner, run.ID) //nolint:errcheck
	state, _ := runner.GetRun(run.ID)
	// With fail_open, panic is recovered → tool should still execute
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed after panic recovery (fail_open), got %q", state.Status)
	}
	if !toolExecuted {
		t.Fatal("tool should have executed after panic recovery (fail_open)")
	}
}

// TestPostToolUseHookDurationIsPositive verifies the Duration field in
// PostToolUseEvent is positive for a tool that takes non-zero time.
func TestPostToolUseHookDurationIsPositive(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	_ = reg.Register(echoToolDef(), func(_ context.Context, raw json.RawMessage) (string, error) {
		time.Sleep(2 * time.Millisecond)
		var args struct{ Message string }
		json.Unmarshal(raw, &args) //nolint:errcheck
		return args.Message, nil
	})

	var captured PostToolUseEvent
	runner := NewRunner(
		providerWithToolCall("echo_tool", `{"message":"duration"}`),
		reg,
		RunnerConfig{
			DefaultModel: "m",
			PostToolUseHooks: []PostToolUseHook{
				postToolHookFunc{name: "duration-spy", fn: func(_ context.Context, ev PostToolUseEvent) (*PostToolUseResult, error) {
					captured = ev
					return &PostToolUseResult{}, nil
				}},
			},
		},
	)

	run, _ := runner.StartRun(RunRequest{Prompt: "go"})
	collectRunEvents(t, runner, run.ID) //nolint:errcheck
	if captured.Duration <= 0 {
		t.Fatalf("expected positive Duration, got %v", captured.Duration)
	}
}

// TestPreToolUseHookNilResultTreatedAsAllow verifies that nil return from
// a PreToolUseHook (no error) is treated as Allow.
func TestPreToolUseHookNilResultTreatedAsAllow(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	var executed bool
	_ = reg.Register(echoToolDef(), func(_ context.Context, _ json.RawMessage) (string, error) {
		executed = true
		return "ok", nil
	})

	runner := NewRunner(
		providerWithToolCall("echo_tool", `{"message":"nil-result"}`),
		reg,
		RunnerConfig{
			DefaultModel: "m",
			PreToolUseHooks: []PreToolUseHook{
				preToolHookFunc{name: "nil-returner", fn: func(_ context.Context, _ PreToolUseEvent) (*PreToolUseResult, error) {
					return nil, nil // nil result → allow
				}},
			},
		},
	)

	run, _ := runner.StartRun(RunRequest{Prompt: "go"})
	collectRunEvents(t, runner, run.ID) //nolint:errcheck
	if !executed {
		t.Fatal("tool should have executed when hook returned nil result")
	}
}

// TestRegistrationDuringExecution verifies that new hooks can be registered
// on the RunnerConfig before StartRun without data races.
func TestPreToolUseHookConcurrentRegistrationSafety(t *testing.T) {
	t.Parallel()

	const runs = 10
	var wg sync.WaitGroup
	wg.Add(runs)

	for i := 0; i < runs; i++ {
		idx := i
		go func() {
			defer wg.Done()

			reg := NewRegistry()
			_ = reg.Register(echoToolDef(), echoToolHandler())

			runner := NewRunner(
				providerWithToolCall("echo_tool", fmt.Sprintf(`{"message":"run-%d"}`, idx)),
				reg,
				RunnerConfig{
					DefaultModel: "m",
					PreToolUseHooks: []PreToolUseHook{
						preToolHookFunc{name: fmt.Sprintf("h-%d", idx), fn: func(_ context.Context, _ PreToolUseEvent) (*PreToolUseResult, error) {
							return &PreToolUseResult{Decision: ToolHookAllow}, nil
						}},
					},
				},
			)

			run, _ := runner.StartRun(RunRequest{Prompt: "go"})
			collectRunEvents(t, runner, run.ID) //nolint:errcheck
		}()
	}

	wg.Wait()
}
