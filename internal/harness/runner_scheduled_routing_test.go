package harness

import (
	"context"
	"go-agent-harness/internal/store"
	"slices"
	"testing"
)

func TestRunMetadataPreservesImmutableScheduledRoutingPolicy(t *testing.T) {
	runner := &Runner{runs: map[string]*runState{
		"run-origin": {
			run: Run{
				ID: "run-origin", TenantID: "tenant", ConversationID: "conversation", AgentID: "agent",
				Model: "fixture-model", ProviderName: "effective-provider",
			},
			allowFallback:     true,
			fallbackProviders: []string{"secondary", "tertiary"},
		},
	}}

	metadata := runner.runMetadata("run-origin")
	if metadata.Model != "fixture-model" || metadata.ProviderName != "effective-provider" ||
		!metadata.AllowFallback || !slices.Equal(metadata.FallbackProviders, []string{"secondary", "tertiary"}) {
		t.Fatalf("run metadata routing = %#v", metadata)
	}
	metadata.FallbackProviders[0] = "mutated"
	if got := runner.runs["run-origin"].fallbackProviders[0]; got != "secondary" {
		t.Fatalf("metadata fallback providers alias run state: %q", got)
	}
}

func TestStartRunPersistsRequestedProviderBeforeScheduledDispatch(t *testing.T) {
	provider := newHeldProvider()
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "fixture-model",
		Store:        store.NewMemoryStore(),
	})
	t.Cleanup(func() {
		provider.unblockAll()
		_ = runner.Shutdown(context.Background())
	})

	run, err := runner.StartRunWithID(RunRequest{
		Prompt:       "scheduled prompt",
		ProviderName: "scheduled-primary",
	}, "run_scheduled-provider")
	if err != nil {
		t.Fatalf("StartRunWithID: %v", err)
	}
	if run.ProviderName != "scheduled-primary" {
		t.Fatalf("queued provider = %q, want scheduled-primary", run.ProviderName)
	}
}
