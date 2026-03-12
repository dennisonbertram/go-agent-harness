package harness

// Regression tests for GitHub issue #221:
// Conversation history loaded without tenant/agent scoping (cross-tenant disclosure).
//
// An attacker who knows or guesses another tenant's conversation_id must not
// be able to load that conversation's history into their own run context.

import (
	"context"
	"errors"
	"testing"
)

// TestStartRunRejectsCrossTenantsConversationInMemory verifies that StartRun
// rejects a caller-supplied ConversationID that belongs to a different tenant
// (in-memory path: the conversation was created and completed in the same
// Runner instance before the attacker's request).
func TestStartRunRejectsCrossTenantsConversationInMemory(t *testing.T) {
	t.Parallel()

	prov := &continuationProvider{
		turns: []CompletionResult{
			{Content: "victim response"},
			{Content: "attacker response (should never happen)"},
		},
	}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})

	// Tenant A creates and completes a run.
	victimRun, err := runner.StartRun(RunRequest{
		Prompt:         "victim prompt",
		TenantID:       "tenant-a",
		AgentID:        "agent-a",
		ConversationID: "conv-victim-123",
	})
	if err != nil {
		t.Fatalf("StartRun (victim): %v", err)
	}
	waitForStatusCont(t, runner, victimRun.ID, RunStatusCompleted, RunStatusFailed)

	// Tenant B tries to inject the victim's conversation_id into their own run.
	_, err = runner.StartRun(RunRequest{
		Prompt:         "attacker prompt",
		TenantID:       "tenant-b",
		AgentID:        "agent-b",
		ConversationID: "conv-victim-123", // stolen conversation ID
	})
	if err == nil {
		t.Fatal("expected error when cross-tenant ConversationID is supplied, got nil")
	}
	if !errors.Is(err, ErrConversationAccessDenied) {
		t.Fatalf("expected ErrConversationAccessDenied, got: %v", err)
	}
}

// TestStartRunRejectsCrossAgentConversationInMemory verifies that StartRun
// rejects a ConversationID belonging to a different agent within the same
// tenant (in-memory path).
func TestStartRunRejectsCrossAgentConversationInMemory(t *testing.T) {
	t.Parallel()

	prov := &continuationProvider{
		turns: []CompletionResult{
			{Content: "agent1 response"},
			{Content: "agent2 hijacked response (should never happen)"},
		},
	}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})

	// Same tenant, agent-1 creates a conversation.
	run1, err := runner.StartRun(RunRequest{
		Prompt:         "agent1 prompt",
		TenantID:       "tenant-shared",
		AgentID:        "agent-1",
		ConversationID: "conv-agent1-xyz",
	})
	if err != nil {
		t.Fatalf("StartRun (agent1): %v", err)
	}
	waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

	// Same tenant, agent-2 tries to use agent-1's conversation.
	_, err = runner.StartRun(RunRequest{
		Prompt:         "agent2 prompt",
		TenantID:       "tenant-shared",
		AgentID:        "agent-2",
		ConversationID: "conv-agent1-xyz",
	})
	if err == nil {
		t.Fatal("expected error when cross-agent ConversationID is supplied, got nil")
	}
	if !errors.Is(err, ErrConversationAccessDenied) {
		t.Fatalf("expected ErrConversationAccessDenied, got: %v", err)
	}
}

// TestStartRunAllowsSameTenantSameAgentConversation verifies that a valid
// continuation — same tenant + same agent reusing their own conversation_id —
// is still allowed.
func TestStartRunAllowsSameTenantSameAgentConversation(t *testing.T) {
	t.Parallel()

	prov := &continuationProvider{
		turns: []CompletionResult{
			{Content: "first response"},
			{Content: "second response"},
		},
	}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})

	// First run establishes the conversation.
	run1, err := runner.StartRun(RunRequest{
		Prompt:         "first prompt",
		TenantID:       "tenant-c",
		AgentID:        "agent-c",
		ConversationID: "conv-legit-abc",
	})
	if err != nil {
		t.Fatalf("StartRun (run1): %v", err)
	}
	waitForStatusCont(t, runner, run1.ID, RunStatusCompleted, RunStatusFailed)

	// Same tenant + agent reuses the conversation — must succeed.
	run2, err := runner.StartRun(RunRequest{
		Prompt:         "second prompt",
		TenantID:       "tenant-c",
		AgentID:        "agent-c",
		ConversationID: "conv-legit-abc",
	})
	if err != nil {
		t.Fatalf("StartRun (run2): unexpected error for legitimate reuse: %v", err)
	}
	waitForStatusCont(t, runner, run2.ID, RunStatusCompleted, RunStatusFailed)

	// Verify the second run shares the same conversation.
	if run2.ConversationID != "conv-legit-abc" {
		t.Errorf("expected ConversationID %q, got %q", "conv-legit-abc", run2.ConversationID)
	}
}

// TestStartRunAllowsNewConversationWithoutPreexistingOwner verifies that a
// caller-supplied ConversationID that doesn't exist yet is accepted (it's a
// new conversation, no ownership conflict).
func TestStartRunAllowsNewConversationWithoutPreexistingOwner(t *testing.T) {
	t.Parallel()

	prov := &continuationProvider{
		turns: []CompletionResult{{Content: "response"}},
	}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})

	// A completely new conversation_id — no prior owner, must be allowed.
	run, err := runner.StartRun(RunRequest{
		Prompt:         "brand new conversation",
		TenantID:       "tenant-x",
		AgentID:        "agent-x",
		ConversationID: "conv-brand-new-never-seen",
	})
	if err != nil {
		t.Fatalf("StartRun: unexpected error for new conversation: %v", err)
	}
	waitForStatusCont(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)
}

// TestStartRunRejectsCrossTenantConversationFromStore verifies that StartRun
// rejects a caller-supplied ConversationID that belongs to a different tenant
// in the persistent store (SQLite path).
func TestStartRunRejectsCrossTenantConversationFromStore(t *testing.T) {
	t.Parallel()

	store := newTestConversationStore(t)
	ctx := context.Background()

	// Directly insert a conversation owned by tenant-a into the store.
	victimConvID := "conv-store-victim-789"
	msgs := []Message{
		{Role: "user", Content: "victim message"},
		{Role: "assistant", Content: "victim reply"},
	}
	if err := store.SaveConversationWithCost(ctx, victimConvID, msgs, ConversationTokenCost{}); err != nil {
		t.Fatalf("SaveConversationWithCost: %v", err)
	}
	// Set tenant ownership on the conversation row.
	if err := store.UpdateConversationMeta(ctx, victimConvID, "", "tenant-a"); err != nil {
		t.Fatalf("UpdateConversationMeta: %v", err)
	}

	prov := &continuationProvider{
		turns: []CompletionResult{{Content: "attacker (should not happen)"}},
	}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:      "test-model",
		MaxSteps:          2,
		ConversationStore: store,
	})

	// tenant-b tries to start a run using tenant-a's conversation ID from the store.
	_, err := runner.StartRun(RunRequest{
		Prompt:         "attacker prompt",
		TenantID:       "tenant-b",
		AgentID:        "agent-b",
		ConversationID: victimConvID,
	})
	if err == nil {
		t.Fatal("expected error for cross-tenant conversation from store, got nil")
	}
	if !errors.Is(err, ErrConversationAccessDenied) {
		t.Fatalf("expected ErrConversationAccessDenied, got: %v", err)
	}
}

// TestStartRunAllowsSameTenantConversationFromStore verifies that the same
// tenant can resume their own conversation that lives only in the store.
func TestStartRunAllowsSameTenantConversationFromStore(t *testing.T) {
	t.Parallel()

	store := newTestConversationStore(t)
	ctx := context.Background()

	ownConvID := "conv-store-own-456"
	msgs := []Message{
		{Role: "user", Content: "prior message"},
		{Role: "assistant", Content: "prior reply"},
	}
	if err := store.SaveConversationWithCost(ctx, ownConvID, msgs, ConversationTokenCost{}); err != nil {
		t.Fatalf("SaveConversationWithCost: %v", err)
	}
	if err := store.UpdateConversationMeta(ctx, ownConvID, "", "tenant-legit"); err != nil {
		t.Fatalf("UpdateConversationMeta: %v", err)
	}

	prov := &continuationProvider{
		turns: []CompletionResult{{Content: "continued response"}},
	}
	runner := NewRunner(prov, NewRegistry(), RunnerConfig{
		DefaultModel:      "test-model",
		MaxSteps:          2,
		ConversationStore: store,
	})

	// Same tenant reusing their own conversation from the store — must succeed.
	run, err := runner.StartRun(RunRequest{
		Prompt:         "continue my conversation",
		TenantID:       "tenant-legit",
		AgentID:        "agent-legit",
		ConversationID: ownConvID,
	})
	if err != nil {
		t.Fatalf("StartRun: unexpected error for same-tenant store resume: %v", err)
	}
	waitForStatusCont(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)
}

// TestConversationAccessDeniedIsDistinctError verifies ErrConversationAccessDenied
// is a distinct error type and is not wrapped inside other errors.
func TestConversationAccessDeniedIsDistinctError(t *testing.T) {
	t.Parallel()

	if ErrConversationAccessDenied == nil {
		t.Fatal("ErrConversationAccessDenied must not be nil")
	}
	if errors.Is(ErrConversationAccessDenied, ErrRunNotFound) {
		t.Error("ErrConversationAccessDenied must not match ErrRunNotFound")
	}
}

// TestGetConversationOwnerSQLite verifies the GetConversationOwner method
// returns the correct tenant_id from SQLite and returns nil for unknown IDs.
func TestGetConversationOwnerSQLite(t *testing.T) {
	t.Parallel()

	store := newTestConversationStore(t)
	ctx := context.Background()

	// Unknown conversation — must return nil, nil.
	conv, err := store.GetConversationOwner(ctx, "nonexistent-conv")
	if err != nil {
		t.Fatalf("GetConversationOwner (nonexistent): %v", err)
	}
	if conv != nil {
		t.Fatalf("expected nil for nonexistent conversation, got %+v", conv)
	}

	// Known conversation with tenant_id.
	convID := "conv-owner-test-111"
	msgs := []Message{{Role: "user", Content: "hello"}}
	if err := store.SaveConversationWithCost(ctx, convID, msgs, ConversationTokenCost{}); err != nil {
		t.Fatalf("SaveConversationWithCost: %v", err)
	}
	if err := store.UpdateConversationMeta(ctx, convID, "ws-1", "tenant-owner"); err != nil {
		t.Fatalf("UpdateConversationMeta: %v", err)
	}

	conv, err = store.GetConversationOwner(ctx, convID)
	if err != nil {
		t.Fatalf("GetConversationOwner (existing): %v", err)
	}
	if conv == nil {
		t.Fatalf("expected non-nil conversation for existing convID")
	}
	if conv.TenantID != "tenant-owner" {
		t.Errorf("TenantID = %q, want %q", conv.TenantID, "tenant-owner")
	}
}
