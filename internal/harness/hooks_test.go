package harness

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type preHookFunc struct {
	name string
	fn   func(context.Context, PreMessageHookInput) (PreMessageHookResult, error)
}

func (h preHookFunc) Name() string { return h.name }

func (h preHookFunc) BeforeMessage(ctx context.Context, in PreMessageHookInput) (PreMessageHookResult, error) {
	return h.fn(ctx, in)
}

type postHookFunc struct {
	name string
	fn   func(context.Context, PostMessageHookInput) (PostMessageHookResult, error)
}

func (h postHookFunc) Name() string { return h.name }

func (h postHookFunc) AfterMessage(ctx context.Context, in PostMessageHookInput) (PostMessageHookResult, error) {
	return h.fn(ctx, in)
}

func TestRunnerHooksMutateResponseAndEmitEvents(t *testing.T) {
	t.Parallel()

	runner := NewRunner(&stubProvider{turns: []CompletionResult{{Content: "raw response"}}}, NewRegistry(), RunnerConfig{
		DefaultModel: "gpt-5-nano",
		PreMessageHooks: []PreMessageHook{
			preHookFunc{name: "pre-mutator", fn: func(_ context.Context, in PreMessageHookInput) (PreMessageHookResult, error) {
				if in.Step != 1 {
					t.Fatalf("unexpected step %d", in.Step)
				}
				mutated := in.Request
				mutated.Model = "hook-model"
				return PreMessageHookResult{Action: HookActionContinue, MutatedRequest: &mutated}, nil
			}},
		},
		PostMessageHooks: []PostMessageHook{
			postHookFunc{name: "post-mutator", fn: func(_ context.Context, in PostMessageHookInput) (PostMessageHookResult, error) {
				mutated := in.Response
				mutated.Content = "mutated by post hook"
				return PostMessageHookResult{Action: HookActionContinue, MutatedResponse: &mutated}, nil
			}},
		},
	})

	run, err := runner.StartRun(RunRequest{Prompt: "Hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}
	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatalf("expected run state")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q", state.Status)
	}
	if state.Output != "mutated by post hook" {
		t.Fatalf("unexpected output: %q", state.Output)
	}

	requireEventOrder(t, events,
		"run.started",
		"llm.turn.requested",
		"hook.started",
		"hook.completed",
		"llm.turn.completed",
		"assistant.message",
		"run.completed",
	)
}

func TestRunnerPreHookCanBlockRun(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{turns: []CompletionResult{{Content: "should not happen"}}}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "gpt-5-nano",
		PreMessageHooks: []PreMessageHook{
			preHookFunc{name: "blocker", fn: func(_ context.Context, _ PreMessageHookInput) (PreMessageHookResult, error) {
				return PreMessageHookResult{Action: HookActionBlock, Reason: "blocked by policy"}, nil
			}},
		},
	})

	run, err := runner.StartRun(RunRequest{Prompt: "Hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}
	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatalf("expected run state")
	}
	if state.Status != RunStatusFailed {
		t.Fatalf("expected failed status, got %q", state.Status)
	}
	if !strings.Contains(state.Error, "blocked by pre-message hook blocker") {
		t.Fatalf("unexpected error: %q", state.Error)
	}
	if provider.calls != 0 {
		t.Fatalf("expected provider not called, got %d", provider.calls)
	}

	requireEventOrder(t, events, "run.started", "hook.completed", "run.failed")
}

func TestRunnerHookErrorFailOpen(t *testing.T) {
	t.Parallel()

	runner := NewRunner(&stubProvider{turns: []CompletionResult{{Content: "ok"}}}, NewRegistry(), RunnerConfig{
		DefaultModel:    "gpt-5-nano",
		HookFailureMode: HookFailureModeFailOpen,
		PreMessageHooks: []PreMessageHook{
			preHookFunc{name: "error-hook", fn: func(_ context.Context, _ PreMessageHookInput) (PreMessageHookResult, error) {
				return PreMessageHookResult{}, errors.New("hook boom")
			}},
		},
	})

	run, err := runner.StartRun(RunRequest{Prompt: "Hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}
	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatalf("expected run state")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed status, got %q", state.Status)
	}

	seenHookFailed := false
	for _, ev := range events {
		if ev.Type == "hook.failed" {
			seenHookFailed = true
			break
		}
	}
	if !seenHookFailed {
		t.Fatalf("expected hook.failed event")
	}
}

func TestRunnerHookErrorFailClosed(t *testing.T) {
	t.Parallel()

	runner := NewRunner(&stubProvider{turns: []CompletionResult{{Content: "ok"}}}, NewRegistry(), RunnerConfig{
		DefaultModel: "gpt-5-nano",
		PreMessageHooks: []PreMessageHook{
			preHookFunc{name: "error-hook", fn: func(_ context.Context, _ PreMessageHookInput) (PreMessageHookResult, error) {
				return PreMessageHookResult{}, errors.New("hook boom")
			}},
		},
	})

	run, err := runner.StartRun(RunRequest{Prompt: "Hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	_, err = collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}
	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatalf("expected run state")
	}
	if state.Status != RunStatusFailed {
		t.Fatalf("expected failed status, got %q", state.Status)
	}
	if !strings.Contains(state.Error, "pre-message hook error-hook failed") {
		t.Fatalf("unexpected error: %q", state.Error)
	}
}

func TestRunnerPostHookCanBlockRun(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{turns: []CompletionResult{{Content: "provider output"}}}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "gpt-5-nano",
		PostMessageHooks: []PostMessageHook{
			postHookFunc{name: "post-blocker", fn: func(_ context.Context, _ PostMessageHookInput) (PostMessageHookResult, error) {
				return PostMessageHookResult{Action: HookActionBlock, Reason: "unsafe response"}, nil
			}},
		},
	})

	run, err := runner.StartRun(RunRequest{Prompt: "Hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}
	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatalf("expected run state")
	}
	if state.Status != RunStatusFailed {
		t.Fatalf("expected failed status, got %q", state.Status)
	}
	if !strings.Contains(state.Error, "blocked by post-message hook post-blocker") {
		t.Fatalf("unexpected error: %q", state.Error)
	}
	if provider.calls != 1 {
		t.Fatalf("expected provider called once, got %d", provider.calls)
	}

	requireEventOrder(t, events, "run.started", "hook.completed", "run.failed")
}

func TestRunnerPostHookErrorFailOpen(t *testing.T) {
	t.Parallel()

	runner := NewRunner(&stubProvider{turns: []CompletionResult{{Content: "ok"}}}, NewRegistry(), RunnerConfig{
		DefaultModel:    "gpt-5-nano",
		HookFailureMode: HookFailureModeFailOpen,
		PostMessageHooks: []PostMessageHook{postHookFunc{name: "post-error", fn: func(_ context.Context, _ PostMessageHookInput) (PostMessageHookResult, error) {
			return PostMessageHookResult{}, errors.New("post boom")
		}}},
	})

	run, err := runner.StartRun(RunRequest{Prompt: "Hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}
	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatalf("expected run state")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed status, got %q", state.Status)
	}
	seenHookFailed := false
	for _, ev := range events {
		if ev.Type == "hook.failed" && ev.Payload["stage"] == "post_message" {
			seenHookFailed = true
			break
		}
	}
	if !seenHookFailed {
		t.Fatalf("expected post-message hook.failed event")
	}
}
