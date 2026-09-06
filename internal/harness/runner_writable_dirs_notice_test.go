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

// TestRunnerFirstTurnPermissionsNoticeOmitsWritableCacheDirsForUnrestricted
// is a regression test for issue #1399, guarding a different angle than the
// positive test above: "local" and "unrestricted" sandbox scope already
// permit unrestricted filesystem writes, so calling out temp/cache dirs
// specifically there would be noise (and would misleadingly suggest those
// scopes are MORE restricted than they actually are). If a future change
// stopped gating the new sentence on Sandbox == SandboxScopeWorkspace, this
// test would fail by finding the sentence present for unrestricted scope.
func TestRunnerFirstTurnPermissionsNoticeOmitsWritableCacheDirsForUnrestricted(t *testing.T) {
	t.Parallel()

	provider := &capturingProvider{turns: []CompletionResult{{Content: "done"}}}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel:       "gpt-5-nano",
		MaxSteps:           2,
		DefaultAgentIntent: "general",
	})

	run, err := runner.StartRun(RunRequest{
		Prompt:      "hello",
		Permissions: &PermissionConfig{Sandbox: SandboxScopeUnrestricted, Approval: ApprovalPolicyNone},
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
	if anyMessageContains(provider.calls[0].Messages, "temp and per-user cache directories are writable") {
		t.Fatalf("expected unrestricted-scope notice to omit the writable-cache-dirs sentence, got %+v", provider.calls[0].Messages)
	}
}
