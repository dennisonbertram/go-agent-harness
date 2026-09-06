package harness

import "testing"

// TestRunnerFirstTurnPermissionsNoticeIncludesWritableCacheDirsForWorkspace
// verifies that under SandboxScopeWorkspace the permissions notice tells the
// model that temp and per-user cache directories are writable (issue
// #1399), so it does not misdiagnose a `go build`/`npm install` cache
// failure as a permissions problem it must additionally route around.
func TestRunnerFirstTurnPermissionsNoticeIncludesWritableCacheDirsForWorkspace(t *testing.T) {
	t.Parallel()

	provider := &capturingProvider{turns: []CompletionResult{{Content: "done"}}}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel:       "gpt-5-nano",
		MaxSteps:           2,
		DefaultAgentIntent: "general",
	})

	run, err := runner.StartRun(RunRequest{
		Prompt:      "hello",
		Permissions: &PermissionConfig{Sandbox: SandboxScopeWorkspace, Approval: ApprovalPolicyNone, Network: NetworkPolicyAllow},
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
	if !anyMessageContains(provider.calls[0].Messages, "temp and per-user cache directories are writable") {
		t.Fatalf("expected first-turn messages to mention writable temp/cache dirs for workspace scope, got %+v", provider.calls[0].Messages)
	}
}
