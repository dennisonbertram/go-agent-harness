package harness

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	htools "go-agent-harness/internal/harness/tools"
)

// tenantRecordingStore records the filter passed to ListConversations so the
// tenant-scoping assertion can inspect it.
type tenantRecordingStore struct {
	ConversationStore
	gotFilter ConversationFilter
}

func (s *tenantRecordingStore) ListConversations(_ context.Context, filter ConversationFilter, _, _ int) ([]Conversation, error) {
	s.gotFilter = filter
	return nil, nil
}

func (s *tenantRecordingStore) SearchMessages(_ context.Context, _, _ string, _ int) ([]MessageSearchResult, error) {
	return nil, nil
}

// TestListConversationsToolIsTenantScoped pins the fix for a cross-tenant
// disclosure: the list_conversations tool passed an empty ConversationFilter,
// so it enumerated the IDs, titles, and message counts of conversations owned
// by every other tenant. Its sibling search_conversations was already scoped.
func TestListConversationsToolIsTenantScoped(t *testing.T) {
	store := &tenantRecordingStore{}
	adapter := &conversationStoreAdapter{store: store}

	ctx := context.WithValue(context.Background(), htools.ContextKeyRunMetadata, htools.RunMetadata{TenantID: "tenant-a"})
	if _, err := adapter.ListConversations(ctx, 10, 0); err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if store.gotFilter.TenantID != "tenant-a" {
		t.Errorf("ListConversations filter TenantID = %q, want %q", store.gotFilter.TenantID, "tenant-a")
	}

	// No run metadata (auth-disabled local callers) leaves the filter off, which
	// preserves single-process behaviour rather than hiding everything.
	store.gotFilter = ConversationFilter{TenantID: "stale"}
	if _, err := adapter.ListConversations(context.Background(), 10, 0); err != nil {
		t.Fatalf("list conversations without metadata: %v", err)
	}
	if store.gotFilter.TenantID != "" {
		t.Errorf("TenantID without run metadata = %q, want empty", store.gotFilter.TenantID)
	}
}

// TestAllowedToolsEnforcedAtCallGate proves the allowlist is enforced when a
// tool is CALLED, not merely when the tool list is offered. A model is free to
// emit a tool name it was never offered; before this gate existed, the registry
// executed it.
func TestAllowedToolsEnforcedAtCallGate(t *testing.T) {
	registry := NewRegistry()
	var bashCalls int
	if err := registry.Register(ToolDefinition{
		Name: "bash", Description: "runs bash", Parameters: map[string]any{"type": "object"},
	}, func(context.Context, json.RawMessage) (string, error) {
		bashCalls++
		return `{"output":"ran"}`, nil
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := registry.Register(ToolDefinition{
		Name: "read_file", Description: "reads a file", Parameters: map[string]any{"type": "object"},
	}, func(context.Context, json.RawMessage) (string, error) {
		return `{"content":"data"}`, nil
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	step := 0
	provider := &funcProvider{
		fn: func(_ context.Context, _ CompletionRequest) (CompletionResult, error) {
			step++
			if step == 1 {
				// bash was never offered — the model calls it anyway.
				return CompletionResult{ToolCalls: []ToolCall{{ID: "c1", Name: "bash", Arguments: `{"command":"id"}`}}}, nil
			}
			return CompletionResult{Content: "done"}, nil
		},
	}

	runner := NewRunner(provider, registry, RunnerConfig{DefaultModel: "gpt-4.1-mini", MaxSteps: 3})
	run, err := runner.StartRun(RunRequest{Prompt: "do task", AllowedTools: []string{"read_file"}})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}

	if bashCalls != 0 {
		t.Errorf("bash handler ran %d time(s); a tool outside allowed_tools must never execute", bashCalls)
	}
	var blocked bool
	for _, ev := range events {
		if ev.Type == EventToolCallBlocked && ev.Payload["reason"] == "tool_not_in_allowed_tools" {
			blocked = true
		}
	}
	if !blocked {
		t.Error("expected a tool.call.blocked event with reason tool_not_in_allowed_tools")
	}
}

// TestDeniedAskUserQuestionDoesNotStrandRunStatus pins the fix for a run left
// parked in waiting_for_user. The wait was announced before the permission
// gates ran, so a denied AskUserQuestion call skipped every status-restoring
// path while the run kept executing — clients saw it blocked on input that
// would never be asked for.
func TestDeniedAskUserQuestionDoesNotStrandRunStatus(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name: htools.AskUserQuestionToolName, Description: "asks the user",
		Parameters: map[string]any{"type": "object"},
	}, func(context.Context, json.RawMessage) (string, error) {
		return `{"answers":{}}`, nil
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	// The status must be observed MID-RUN: run completion overwrites it with a
	// terminal status, so the final value proves nothing. Step 2 samples the
	// status the run is carrying while it continues working after the denial.
	var runner *Runner
	var runID string
	var statusAfterDenial RunStatus
	step := 0
	provider := &funcProvider{
		fn: func(_ context.Context, _ CompletionRequest) (CompletionResult, error) {
			step++
			if step == 1 {
				return CompletionResult{ToolCalls: []ToolCall{{
					ID:        "c1",
					Name:      htools.AskUserQuestionToolName,
					Arguments: `{"questions":[{"question":"pick one?","header":"Pick","options":[{"label":"a","description":"a"},{"label":"b","description":"b"}],"multiSelect":false}]}`,
				}}}, nil
			}
			if step == 2 {
				if snapshot, ok := runner.GetRun(runID); ok {
					statusAfterDenial = snapshot.Status
				}
			}
			return CompletionResult{Content: "done"}, nil
		},
	}

	runner = NewRunner(provider, registry, RunnerConfig{DefaultModel: "gpt-4.1-mini", MaxSteps: 3})
	run, err := runner.StartRun(RunRequest{
		Prompt: "do task",
		Permissions: &PermissionConfig{
			Rules: NewPermissionRuleSet([]PermissionRule{
				{Pattern: htools.AskUserQuestionToolName, Effect: PermissionEffectDeny},
			}),
		},
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	runID = run.ID
	if _, err := collectRunEvents(t, runner, run.ID); err != nil {
		t.Fatalf("collect events: %v", err)
	}

	if step < 2 {
		t.Fatalf("run stopped after %d step(s); the denial should not have ended the run", step)
	}
	if statusAfterDenial == RunStatusWaitingForUser {
		t.Errorf("run still reported %q while it kept executing after the AskUserQuestion call was denied", statusAfterDenial)
	}
	if statusAfterDenial != RunStatusRunning {
		t.Errorf("status after denial = %q, want %q", statusAfterDenial, RunStatusRunning)
	}

	final, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if !isTerminalRunStatus(final.Status) {
		t.Errorf("final run status = %q, want a terminal status", final.Status)
	}
	if strings.Contains(final.Error, "panic") {
		t.Errorf("unexpected panic in run: %s", final.Error)
	}
}

// erroringApprovalBroker fails every Ask, standing in for an approval that
// times out or whose transport dies.
type erroringApprovalBroker struct{ err error }

func (b *erroringApprovalBroker) Ask(context.Context, ApprovalRequest) (bool, string, error) {
	return false, "", b.err
}
func (b *erroringApprovalBroker) Pending(string) (PendingApproval, bool) {
	return PendingApproval{}, false
}
func (b *erroringApprovalBroker) Approve(string) error                   { return nil }
func (b *erroringApprovalBroker) ApproveWithOption(string, string) error { return nil }
func (b *erroringApprovalBroker) Deny(string) error                      { return nil }

// runWithAskRuleForBash starts a run whose permission rules require approval
// for bash, using the supplied broker, and returns the collected events plus a
// count of how many times the bash handler actually executed.
func runWithAskRuleForBash(t *testing.T, broker ApprovalBroker) ([]Event, *int) {
	t.Helper()

	executed := 0
	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name: "bash", Description: "runs bash", Parameters: map[string]any{"type": "object"}, Mutating: true,
	}, func(context.Context, json.RawMessage) (string, error) {
		executed++
		return `{"output":"ran"}`, nil
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	step := 0
	provider := &funcProvider{fn: func(context.Context, CompletionRequest) (CompletionResult, error) {
		step++
		if step == 1 {
			return CompletionResult{ToolCalls: []ToolCall{{ID: "c1", Name: "bash", Arguments: `{"command":"id"}`}}}, nil
		}
		return CompletionResult{Content: "done"}, nil
	}}

	runner := NewRunner(provider, registry, RunnerConfig{
		DefaultModel:   "gpt-4.1-mini",
		MaxSteps:       3,
		ApprovalBroker: broker,
		AskUserTimeout: time.Second,
	})
	run, err := runner.StartRun(RunRequest{
		Prompt: "do task",
		Permissions: &PermissionConfig{
			Rules: NewPermissionRuleSet([]PermissionRule{{Pattern: "bash", Effect: PermissionEffectAsk}}),
		},
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}
	return events, &executed
}

// TestApprovalBrokerErrorDeniesTheCallAndContinues covers the approval-failure
// branch: when the broker cannot obtain a decision, the call must be denied and
// reported as a timeout, and the run must carry on rather than hanging or
// executing the tool anyway.
func TestApprovalBrokerErrorDeniesTheCallAndContinues(t *testing.T) {
	events, executed := runWithAskRuleForBash(t, &erroringApprovalBroker{err: errors.New("broker unavailable")})

	if *executed != 0 {
		t.Errorf("bash ran %d time(s); a call whose approval failed must not execute", *executed)
	}
	var denied, completedWithTimeout bool
	for _, ev := range events {
		if ev.Type == EventToolApprovalDenied {
			denied = true
		}
		if ev.Type == EventToolCallCompleted {
			if out, _ := ev.Payload["output"].(string); strings.Contains(out, "approval_timeout") {
				completedWithTimeout = true
			}
		}
	}
	if !denied {
		t.Error("expected a tool.approval.denied event when the broker errors")
	}
	if !completedWithTimeout {
		t.Error("expected the tool result to carry the approval_timeout error code")
	}
}

// TestAskRuleWithoutBrokerBlocksTheCall covers the other half: a permission rule
// asking for approval when no broker is configured must fail closed with an
// explicit block, not fall through to execution.
func TestAskRuleWithoutBrokerBlocksTheCall(t *testing.T) {
	events, executed := runWithAskRuleForBash(t, nil)

	if *executed != 0 {
		t.Errorf("bash ran %d time(s); an unapprovable call must not execute", *executed)
	}
	var blocked bool
	for _, ev := range events {
		if ev.Type == EventToolCallBlocked && ev.Payload["reason"] == "permission_rule_approval_unavailable" {
			blocked = true
		}
	}
	if !blocked {
		t.Error("expected a tool.call.blocked event with reason permission_rule_approval_unavailable")
	}
}
