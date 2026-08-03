package harness

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	htools "go-agent-harness/internal/harness/tools"
)

// registerStubNotifyParentTool registers a minimal stand-in for the real
// deferred.NotifyParentTool (name + Tier only matter for these tests — the
// real tool lives in internal/harness/tools/deferred and can't be imported
// here without an import cycle).
func registerStubNotifyParentTool(t *testing.T, registry *Registry) {
	t.Helper()
	err := registry.RegisterWithOptions(
		ToolDefinition{Name: "notify_parent", Description: "test stand-in"},
		func(ctx context.Context, args json.RawMessage) (string, error) { return "{}", nil },
		RegisterOptions{Tier: htools.TierDeferred},
	)
	if err != nil {
		t.Fatalf("register stub notify_parent: %v", err)
	}
}

// TestStartRun_AutoActivatesNotifyParentForSubagentRuns verifies that a run
// started with a ParentContextHandoff (i.e. it is itself a subagent) has
// notify_parent visible immediately — without the model needing to call
// find_tool first. notify_parent is a key subagent-to-parent communication
// primitive; requiring lazy discovery for it meant models frequently never
// found it and hallucinated alternatives instead (observed in live testing).
func TestStartRun_AutoActivatesNotifyParentForSubagentRuns(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registerStubNotifyParentTool(t, registry)

	provider := &notifyParentRequestGateProvider{
		requests: make(chan CompletionRequest, 1),
		release:  make(chan struct{}),
	}
	runner := NewRunner(provider, registry, RunnerConfig{DefaultModel: "gpt-4.1-mini", MaxSteps: 1})

	run, err := runner.StartRun(RunRequest{
		Prompt: "child task",
		ParentContextHandoff: &htools.ParentContextHandoff{
			ParentRunID: "run_parent_123",
		},
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	// The first actual model request is the user-visible activation boundary.
	// Keep execution blocked after it is captured: a post-StartRun registry read
	// is invalid because fast terminal cleanup may legitimately remove transient
	// activations before the caller can inspect them.
	var first CompletionRequest
	select {
	case first = <-provider.requests:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first provider request")
	}
	if !hasToolNamed(first.Tools, "notify_parent") {
		t.Fatal("expected first provider request to include notify_parent for a recorded parent")
	}
	if !runner.activations.IsActive(run.ID, "notify_parent") {
		t.Fatal("expected notify_parent activation while the first provider request is in flight")
	}

	close(provider.release)
	if got := waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed); got != RunStatusCompleted {
		t.Fatalf("terminal status = %s, want completed", got)
	}
	if runner.activations.IsActive(run.ID, "notify_parent") {
		t.Fatal("expected notify_parent activation cleanup after terminal completion")
	}
}

// notifyParentRequestGateProvider records the request whose tool catalog the
// test must verify, then blocks until the test explicitly allows terminal
// cleanup. It avoids timing assumptions while exercising the real Runner path.
type notifyParentRequestGateProvider struct {
	requests chan CompletionRequest
	release  chan struct{}
}

func (p *notifyParentRequestGateProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error) {
	select {
	case p.requests <- req:
	case <-ctx.Done():
		return CompletionResult{}, ctx.Err()
	}
	select {
	case <-p.release:
		return CompletionResult{Content: "child done"}, nil
	case <-ctx.Done():
		return CompletionResult{}, ctx.Err()
	}
}

// TestStartRun_DoesNotAutoActivateNotifyParentForTopLevelRuns verifies a
// top-level run (no ParentContextHandoff) does NOT get notify_parent
// pre-activated — it has no parent to notify, so it should still require
// find_tool like any other deferred tool (and will typically never need it).
func TestStartRun_DoesNotAutoActivateNotifyParentForTopLevelRuns(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registerStubNotifyParentTool(t, registry)

	provider := &stubProvider{turns: []CompletionResult{{Content: "done"}}}
	runner := NewRunner(provider, registry, RunnerConfig{DefaultModel: "gpt-4.1-mini", MaxSteps: 1})

	run, err := runner.StartRun(RunRequest{Prompt: "top level task"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	defs := runner.filteredToolsForRun(run.ID)
	if hasToolNamed(defs, "notify_parent") {
		t.Fatal("did not expect notify_parent to be auto-activated for a top-level run with no parent")
	}
}

// TestStartRun_DoesNotAutoActivateNotifyParentWhenParentRunIDEmpty verifies a
// ParentContextHandoff with every field blank (e.g. transcript-only handoff,
// no real parent run) does not trigger activation either.
func TestStartRun_DoesNotAutoActivateNotifyParentWhenParentRunIDEmpty(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registerStubNotifyParentTool(t, registry)

	provider := &stubProvider{turns: []CompletionResult{{Content: "done"}}}
	runner := NewRunner(provider, registry, RunnerConfig{DefaultModel: "gpt-4.1-mini", MaxSteps: 1})

	run, err := runner.StartRun(RunRequest{
		Prompt:               "task",
		ParentContextHandoff: &htools.ParentContextHandoff{}, // no ParentRunID
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	defs := runner.filteredToolsForRun(run.ID)
	if hasToolNamed(defs, "notify_parent") {
		t.Fatal("did not expect notify_parent to be auto-activated when ParentRunID is empty")
	}
}

func hasToolNamed(defs []ToolDefinition, name string) bool {
	for _, d := range defs {
		if d.Name == name {
			return true
		}
	}
	return false
}
