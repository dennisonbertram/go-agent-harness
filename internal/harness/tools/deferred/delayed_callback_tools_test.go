package deferred_test

// Tool-surface tests for the delayed-callback tools.
//
// These moved here from the deleted duplicate tool package: the tools they
// exercise now live in tools/deferred, and this file was their only coverage.
// The CallbackManager tests themselves stayed behind in package tools, where
// the manager still lives.

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/deferred"
)

type listFailingCallbackStore struct{ tools.CallbackStore }

func (*listFailingCallbackStore) ListAll(context.Context) ([]tools.CallbackInfo, error) {
	return nil, errors.New("durable callback list unavailable")
}

func TestSetDelayedCallbackTool(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		starter := &movedMockRunStarter{}
		mgr := tools.NewCallbackManager(starter)
		defer mgr.Shutdown()

		tool := deferred.SetDelayedCallbackTool(mgr)
		if tool.Definition.Name != "set_delayed_callback" {
			t.Errorf("expected name set_delayed_callback, got %s", tool.Definition.Name)
		}

		ctx := movedTestContextWithConversation("conv-1")
		args, _ := json.Marshal(map[string]string{"delay": "30s", "prompt": "check deploy"})
		result, err := tool.Handler(ctx, args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var info tools.CallbackInfo
		if err := json.Unmarshal([]byte(result), &info); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}
		if info.State != tools.CallbackStatePending {
			t.Errorf("expected pending, got %s", info.State)
		}
		if info.ConversationID != "conv-1" {
			t.Errorf("expected conv-1, got %s", info.ConversationID)
		}
	})

	t.Run("invalid delay format", func(t *testing.T) {
		starter := &movedMockRunStarter{}
		mgr := tools.NewCallbackManager(starter)
		defer mgr.Shutdown()

		tool := deferred.SetDelayedCallbackTool(mgr)
		ctx := movedTestContextWithConversation("conv-1")
		args, _ := json.Marshal(map[string]string{"delay": "not-a-duration", "prompt": "check"})
		_, err := tool.Handler(ctx, args)
		if err == nil {
			t.Fatal("expected error for invalid delay")
		}
	})

	t.Run("no run metadata", func(t *testing.T) {
		starter := &movedMockRunStarter{}
		mgr := tools.NewCallbackManager(starter)
		defer mgr.Shutdown()

		tool := deferred.SetDelayedCallbackTool(mgr)
		args, _ := json.Marshal(map[string]string{"delay": "30s", "prompt": "check"})
		_, err := tool.Handler(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for missing run metadata")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		starter := &movedMockRunStarter{}
		mgr := tools.NewCallbackManager(starter)
		defer mgr.Shutdown()

		tool := deferred.SetDelayedCallbackTool(mgr)
		ctx := movedTestContextWithConversation("conv-1")
		_, err := tool.Handler(ctx, json.RawMessage(`{invalid`))
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestCancelDelayedCallbackTool(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		starter := &movedMockRunStarter{}
		mgr := tools.NewCallbackManager(starter)
		defer mgr.Shutdown()

		info, _ := mgr.Set(movedSetReq("conv-1", 30*time.Second, "check"))

		tool := deferred.CancelDelayedCallbackTool(mgr)
		ctx := movedTestContextWithConversation("conv-1")
		args, _ := json.Marshal(map[string]string{"callback_id": info.ID})
		result, err := tool.Handler(ctx, args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var canceled tools.CallbackInfo
		json.Unmarshal([]byte(result), &canceled)
		if canceled.State != tools.CallbackStateCanceled {
			t.Errorf("expected canceled, got %s", canceled.State)
		}
	})

	t.Run("cancel nonexistent", func(t *testing.T) {
		starter := &movedMockRunStarter{}
		mgr := tools.NewCallbackManager(starter)
		defer mgr.Shutdown()

		tool := deferred.CancelDelayedCallbackTool(mgr)
		ctx := movedTestContextWithConversation("conv-1")
		args, _ := json.Marshal(map[string]string{"callback_id": "nonexistent"})
		_, err := tool.Handler(ctx, args)
		if err == nil {
			t.Fatal("expected error for nonexistent callback")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		starter := &movedMockRunStarter{}
		mgr := tools.NewCallbackManager(starter)
		defer mgr.Shutdown()

		tool := deferred.CancelDelayedCallbackTool(mgr)
		ctx := movedTestContextWithConversation("conv-1")
		_, err := tool.Handler(ctx, json.RawMessage(`{bad`))
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestListDelayedCallbacksTool(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		starter := &movedMockRunStarter{}
		mgr := tools.NewCallbackManager(starter)
		defer mgr.Shutdown()

		mgr.Set(movedSetReq("conv-1", 10*time.Second, "check 1"))
		mgr.Set(movedSetReq("conv-1", 20*time.Second, "check 2"))

		tool := deferred.ListDelayedCallbacksTool(mgr)
		ctx := movedTestContextWithConversation("conv-1")
		result, err := tool.Handler(ctx, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var callbacks []tools.CallbackInfo
		json.Unmarshal([]byte(result), &callbacks)
		if len(callbacks) != 2 {
			t.Errorf("expected 2 callbacks, got %d", len(callbacks))
		}
	})

	t.Run("no run metadata", func(t *testing.T) {
		starter := &movedMockRunStarter{}
		mgr := tools.NewCallbackManager(starter)
		defer mgr.Shutdown()

		tool := deferred.ListDelayedCallbacksTool(mgr)
		_, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("expected error for missing run metadata")
		}
	})

	t.Run("durable list failure is not an empty success", func(t *testing.T) {
		mgr := tools.NewCallbackManager(nil, tools.WithCallbackStore(&listFailingCallbackStore{}))
		defer mgr.Shutdown()

		tool := deferred.ListDelayedCallbacksTool(mgr)
		ctx := movedTestContextWithConversation("conv-1")
		if result, err := tool.Handler(ctx, json.RawMessage(`{}`)); err == nil {
			t.Fatalf("durable list failure returned success %q", result)
		}
	})
}

// --- Regression Tests ---

// ============================================================
// Category 1: Concurrency Safety
// ============================================================

func TestRegression_ToolHandlerConversationIsolation(t *testing.T) {
	starter := &movedMockRunStarter{}
	mgr := tools.NewCallbackManager(starter)
	defer mgr.Shutdown()

	// Set callback via tool handler on conv-1
	setTool := deferred.SetDelayedCallbackTool(mgr)
	ctx1 := movedTestContextWithConversation("conv-1")
	args, _ := json.Marshal(map[string]string{"delay": "30s", "prompt": "check deploy"})
	_, err := setTool.Handler(ctx1, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// List via tool handler on conv-2 — should see nothing
	listTool := deferred.ListDelayedCallbacksTool(mgr)
	ctx2 := movedTestContextWithConversation("conv-2")
	result, err := listTool.Handler(ctx2, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var callbacks []tools.CallbackInfo
	if err := json.Unmarshal([]byte(result), &callbacks); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(callbacks) != 0 {
		t.Errorf("expected 0 callbacks for conv-2, got %d", len(callbacks))
	}

	// List via tool handler on conv-1 — should see 1
	result1, err := listTool.Handler(ctx1, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var callbacks1 []tools.CallbackInfo
	if err := json.Unmarshal([]byte(result1), &callbacks1); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(callbacks1) != 1 {
		t.Errorf("expected 1 callback for conv-1, got %d", len(callbacks1))
	}
}

// ============================================================
// Category 5: Constraint Enforcement
// ============================================================

// ---- helpers moved alongside these tests ----

type movedStartRunCall struct {
	Prompt         string
	ConversationID string
	TenantID       string
	AgentID        string
}

type movedMockRunStarter struct {
	mu      sync.Mutex
	calls   []movedStartRunCall
	err     error
	startFn func(prompt, convID, tenantID, agentID string) error
}

func (m *movedMockRunStarter) StartRun(prompt, conversationID, tenantID, agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, movedStartRunCall{
		Prompt:         prompt,
		ConversationID: conversationID,
		TenantID:       tenantID,
		AgentID:        agentID,
	})
	if m.startFn != nil {
		return m.startFn(prompt, conversationID, tenantID, agentID)
	}
	return m.err
}

func (m *movedMockRunStarter) getCalls() []movedStartRunCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]movedStartRunCall, len(m.calls))
	copy(result, m.calls)
	return result
}

func movedTestContextWithConversation(convID string) context.Context {
	return context.WithValue(context.Background(), tools.ContextKeyRunMetadata, tools.RunMetadata{
		ConversationID: convID,
	})
}

// movedSetReq is a small helper for tests that schedule callbacks via the manager
// directly (not through the tool handler). It builds a tools.SetRequest with the
// given conversation/delay/prompt and an empty (default/unscoped) tenant+agent.
func movedSetReq(convID string, delay time.Duration, prompt string) tools.SetRequest {
	return tools.SetRequest{ConversationID: convID, Delay: delay, Prompt: prompt}
}
