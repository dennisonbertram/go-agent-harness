package harness

import (
	"strings"
	"testing"
)

// TestRunnerFirstTurnPermissionsNoticeIncludesNetworkAllow verifies that the
// permissions statement injected into the very first turn's messages
// includes the network axis (issue #1397). Previously this notice only
// appeared on a continuation whose permissions changed, so a model never
// learned its network policy on turn one.
func TestRunnerFirstTurnPermissionsNoticeIncludesNetworkAllow(t *testing.T) {
	t.Parallel()

	provider := &capturingProvider{turns: []CompletionResult{{Content: "done"}}}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel:       "gpt-5-nano",
		MaxSteps:           2,
		DefaultAgentIntent: "general",
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	if _, err := collectRunEvents(t, runner, run.ID); err != nil {
		t.Fatalf("collect events: %v", err)
	}

	if len(provider.calls) != 1 {
		t.Fatalf("expected one provider call, got %d", len(provider.calls))
	}
	if !anyMessageContains(provider.calls[0].Messages, "network=allow") {
		t.Fatalf("expected first-turn messages to include a network=allow permissions notice, got %+v", provider.calls[0].Messages)
	}
}

// TestRunnerFirstTurnPermissionsNoticeIncludesNetworkDenyWarning verifies
// that when a run's PermissionConfig sets Network: NetworkPolicyDeny, the
// first-turn permissions notice both reports network=deny and warns the
// model that dependency installs will fail rather than letting it silently
// substitute a different design (issue #1397).
func TestRunnerFirstTurnPermissionsNoticeIncludesNetworkDenyWarning(t *testing.T) {
	t.Parallel()

	provider := &capturingProvider{turns: []CompletionResult{{Content: "done"}}}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel:       "gpt-5-nano",
		MaxSteps:           2,
		DefaultAgentIntent: "general",
	})

	run, err := runner.StartRun(RunRequest{
		Prompt:      "hello",
		Permissions: &PermissionConfig{Sandbox: SandboxScopeWorkspace, Approval: ApprovalPolicyNone, Network: NetworkPolicyDeny},
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	if _, err := collectRunEvents(t, runner, run.ID); err != nil {
		t.Fatalf("collect events: %v", err)
	}

	if len(provider.calls) != 1 {
		t.Fatalf("expected one provider call, got %d", len(provider.calls))
	}
	if !anyMessageContains(provider.calls[0].Messages, "network=deny") {
		t.Fatalf("expected first-turn messages to include a network=deny permissions notice, got %+v", provider.calls[0].Messages)
	}
	wantWarning := "Outbound network is blocked for this run: dependency installs will fail; report the blocker instead of substituting a different design."
	if !anyMessageContains(provider.calls[0].Messages, wantWarning) {
		t.Fatalf("expected first-turn messages to include the network-deny warning sentence, got %+v", provider.calls[0].Messages)
	}
}

func anyMessageContains(messages []Message, substr string) bool {
	for _, msg := range messages {
		if strings.Contains(msg.Content, substr) {
			return true
		}
	}
	return false
}
