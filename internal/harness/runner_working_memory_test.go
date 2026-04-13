package harness

import (
	"context"
	"strings"
	"testing"

	om "go-agent-harness/internal/observationalmemory"
	"go-agent-harness/internal/workingmemory"
)

func TestRunnerInjectsWorkingMemoryBeforeObservationalMemory(t *testing.T) {
	t.Parallel()

	provider := &capturingProvider{
		turns: []CompletionResult{{Content: "done"}},
	}
	registry := NewRegistry()
	memStore := workingmemory.NewMemoryStore()
	scope := om.ScopeKey{TenantID: "default", ConversationID: "conv-working-memory", AgentID: "default"}
	if err := memStore.Set(context.Background(), scope, "plan", map[string]any{"step": "collect"}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	runner := NewRunner(provider, registry, RunnerConfig{
		DefaultModel:       "test-model",
		MaxSteps:           1,
		WorkingMemoryStore: memStore,
		MemoryManager: &memoryStub{
			status:  om.Status{Mode: om.ModeLocalCoordinator, Scope: scope},
			snippet: "<observational-memory>\nremember this\n</observational-memory>",
		},
	})

	run, err := runner.StartRun(RunRequest{
		Prompt:         "hello",
		ConversationID: "conv-working-memory",
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	if len(provider.calls) == 0 {
		t.Fatal("expected provider call")
	}
	messages := provider.calls[0].Messages
	if len(messages) < 3 {
		t.Fatalf("message count = %d, want at least 3", len(messages))
	}
	if !strings.Contains(messages[0].Content, "<working-memory>") {
		t.Fatalf("first message = %q, want working-memory snippet", messages[0].Content)
	}
	if !strings.Contains(messages[1].Content, "<observational-memory>") {
		t.Fatalf("second message = %q, want observational-memory snippet", messages[1].Content)
	}
}
