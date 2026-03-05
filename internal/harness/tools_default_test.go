package harness

import (
	"context"
	"errors"
	"testing"

	htools "go-agent-harness/internal/harness/tools"
)

type staticPolicy struct {
	decision ToolPolicyDecision
	err      error
}

func (s staticPolicy) Allow(_ context.Context, _ ToolPolicyInput) (ToolPolicyDecision, error) {
	return s.decision, s.err
}

func TestToolPolicyAdapterAndDangerousWrapper(t *testing.T) {
	t.Parallel()

	adapter := toolPolicyAdapter{policy: staticPolicy{decision: ToolPolicyDecision{Allow: true, Reason: "ok"}}}
	decision, err := adapter.Allow(context.Background(), htools.PolicyInput{ToolName: "bash", Action: htools.ActionExecute})
	if err != nil {
		t.Fatalf("allow adapter returned error: %v", err)
	}
	if !decision.Allow || decision.Reason != "ok" {
		t.Fatalf("unexpected decision: %+v", decision)
	}

	errAdapter := toolPolicyAdapter{policy: staticPolicy{err: errors.New("boom")}}
	if _, err := errAdapter.Allow(context.Background(), htools.PolicyInput{}); err == nil {
		t.Fatalf("expected adapter error")
	}

	nilAdapter := toolPolicyAdapter{}
	decision, err = nilAdapter.Allow(context.Background(), htools.PolicyInput{})
	if err != nil {
		t.Fatalf("nil adapter should not error: %v", err)
	}
	if decision.Allow {
		t.Fatalf("expected zero decision for nil policy")
	}

	if !isDangerousCommand("rm -rf /") {
		t.Fatalf("expected dangerous wrapper detection")
	}
}

func TestNewDefaultRegistryWithPolicyIncludesAskUserQuestion(t *testing.T) {
	t.Parallel()

	registry := NewDefaultRegistryWithPolicy(t.TempDir(), ToolApprovalModeFullAuto, nil)
	defs := registry.Definitions()
	foundAskUser := false
	foundObsMemory := false
	for _, def := range defs {
		if def.Name == "AskUserQuestion" {
			foundAskUser = true
		}
		if def.Name == "observational_memory" {
			foundObsMemory = true
		}
	}
	if !foundAskUser {
		t.Fatalf("expected AskUserQuestion in default registry")
	}
	if !foundObsMemory {
		t.Fatalf("expected observational_memory in default registry")
	}
}
