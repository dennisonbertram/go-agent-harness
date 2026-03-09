package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// --- Mock ---

type mockRunStarter struct {
	mu      sync.Mutex
	calls   []startRunCall
	err     error
	startFn func(prompt, convID string) error
}

type startRunCall struct {
	Prompt         string
	ConversationID string
}

func (m *mockRunStarter) StartRun(prompt, conversationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, startRunCall{Prompt: prompt, ConversationID: conversationID})
	if m.startFn != nil {
		return m.startFn(prompt, conversationID)
	}
	return m.err
}

func (m *mockRunStarter) getCalls() []startRunCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]startRunCall, len(m.calls))
	copy(result, m.calls)
	return result
}

// --- CallbackManager Tests ---

func TestCallbackManagerSet(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		info, err := mgr.Set("conv-1", 10*time.Second, "check status")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.ID == "" {
			t.Fatal("expected non-empty ID")
		}
		if info.ConversationID != "conv-1" {
			t.Errorf("expected conv-1, got %s", info.ConversationID)
		}
		if info.State != CallbackStatePending {
			t.Errorf("expected pending, got %s", info.State)
		}
		if info.Prompt != "check status" {
			t.Errorf("expected 'check status', got %s", info.Prompt)
		}
		if info.Delay != "10s" {
			t.Errorf("expected '10s', got %s", info.Delay)
		}
	})

	t.Run("delay too short", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		_, err := mgr.Set("conv-1", 1*time.Second, "check")
		if err == nil {
			t.Fatal("expected error for short delay")
		}
	})

	t.Run("delay too long", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		_, err := mgr.Set("conv-1", 2*time.Hour, "check")
		if err == nil {
			t.Fatal("expected error for long delay")
		}
	})

	t.Run("empty prompt", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		_, err := mgr.Set("conv-1", 10*time.Second, "")
		if err == nil {
			t.Fatal("expected error for empty prompt")
		}
	})

	t.Run("max callbacks per conversation", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		for i := 0; i < MaxCallbacksPerConv; i++ {
			_, err := mgr.Set("conv-1", 30*time.Second, fmt.Sprintf("check %d", i))
			if err != nil {
				t.Fatalf("unexpected error on callback %d: %v", i, err)
			}
		}

		_, err := mgr.Set("conv-1", 30*time.Second, "one too many")
		if err == nil {
			t.Fatal("expected error exceeding max callbacks")
		}
	})

	t.Run("max callbacks per conversation does not affect other conversations", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		for i := 0; i < MaxCallbacksPerConv; i++ {
			_, err := mgr.Set("conv-1", 30*time.Second, fmt.Sprintf("check %d", i))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}

		// Different conversation should still work
		_, err := mgr.Set("conv-2", 30*time.Second, "check")
		if err != nil {
			t.Fatalf("unexpected error for different conversation: %v", err)
		}
	})

	t.Run("set after shutdown", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		mgr.Shutdown()

		_, err := mgr.Set("conv-1", 10*time.Second, "check")
		if err == nil {
			t.Fatal("expected error after shutdown")
		}
	})
}

func TestCallbackManagerCancel(t *testing.T) {
	t.Run("cancel pending", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		info, _ := mgr.Set("conv-1", 30*time.Second, "check")
		canceled, err := mgr.Cancel(info.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if canceled.State != CallbackStateCanceled {
			t.Errorf("expected canceled, got %s", canceled.State)
		}
	})

	t.Run("cancel nonexistent", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		_, err := mgr.Cancel("nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent callback")
		}
	})

	t.Run("cancel already canceled", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		info, _ := mgr.Set("conv-1", 30*time.Second, "check")
		mgr.Cancel(info.ID)

		_, err := mgr.Cancel(info.ID)
		if err == nil {
			t.Fatal("expected error for already canceled callback")
		}
	})
}

func TestCallbackManagerList(t *testing.T) {
	t.Run("list empty", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		callbacks := mgr.List("conv-1")
		if len(callbacks) != 0 {
			t.Errorf("expected empty list, got %d", len(callbacks))
		}
	})

	t.Run("list multiple", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		mgr.Set("conv-1", 10*time.Second, "check 1")
		mgr.Set("conv-1", 20*time.Second, "check 2")
		mgr.Set("conv-2", 10*time.Second, "check 3") // different conv

		callbacks := mgr.List("conv-1")
		if len(callbacks) != 2 {
			t.Errorf("expected 2 callbacks, got %d", len(callbacks))
		}

		callbacks2 := mgr.List("conv-2")
		if len(callbacks2) != 1 {
			t.Errorf("expected 1 callback, got %d", len(callbacks2))
		}
	})
}

func TestCallbackManagerFire(t *testing.T) {
	t.Run("fire calls StartRun", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		mgr.now = func() time.Time { return time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC) }
		defer mgr.Shutdown()

		info, err := mgr.Set("conv-1", 5*time.Second, "check deployment")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Cancel the real timer, we'll fire manually
		mgr.mu.Lock()
		mgr.callbacks[info.ID].timer.Stop()
		mgr.mu.Unlock()

		// Fire manually
		mgr.fire(info.ID)

		calls := starter.getCalls()
		if len(calls) != 1 {
			t.Fatalf("expected 1 StartRun call, got %d", len(calls))
		}
		if calls[0].Prompt != "check deployment" {
			t.Errorf("expected prompt 'check deployment', got %s", calls[0].Prompt)
		}
		if calls[0].ConversationID != "conv-1" {
			t.Errorf("expected conv-1, got %s", calls[0].ConversationID)
		}

		// Verify state is fired
		mgr.mu.Lock()
		cb := mgr.callbacks[info.ID]
		mgr.mu.Unlock()
		if cb.info.State != CallbackStateFired {
			t.Errorf("expected fired, got %s", cb.info.State)
		}
	})

	t.Run("fire with StartRun error still marks as fired", func(t *testing.T) {
		starter := &mockRunStarter{err: fmt.Errorf("runner busy")}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		info, _ := mgr.Set("conv-1", 5*time.Second, "check")

		mgr.mu.Lock()
		mgr.callbacks[info.ID].timer.Stop()
		mgr.mu.Unlock()

		mgr.fire(info.ID)

		mgr.mu.Lock()
		cb := mgr.callbacks[info.ID]
		mgr.mu.Unlock()
		if cb.info.State != CallbackStateFired {
			t.Errorf("expected fired even after error, got %s", cb.info.State)
		}
	})

	t.Run("fire already fired is no-op", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		info, _ := mgr.Set("conv-1", 5*time.Second, "check")

		mgr.mu.Lock()
		mgr.callbacks[info.ID].timer.Stop()
		mgr.mu.Unlock()

		mgr.fire(info.ID)
		mgr.fire(info.ID) // second fire should be no-op

		calls := starter.getCalls()
		if len(calls) != 1 {
			t.Errorf("expected 1 call, got %d", len(calls))
		}
	})

	t.Run("cancel already fired", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		info, _ := mgr.Set("conv-1", 5*time.Second, "check")

		mgr.mu.Lock()
		mgr.callbacks[info.ID].timer.Stop()
		mgr.mu.Unlock()

		mgr.fire(info.ID)

		_, err := mgr.Cancel(info.ID)
		if err == nil {
			t.Fatal("expected error canceling fired callback")
		}
	})

	t.Run("fire via timer integration", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		// Use minimum delay so it fires quickly
		_, err := mgr.Set("conv-1", 5*time.Second, "check")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Wait for it to fire (with timeout)
		deadline := time.After(10 * time.Second)
		for {
			calls := starter.getCalls()
			if len(calls) >= 1 {
				if calls[0].Prompt != "check" {
					t.Errorf("expected 'check', got %s", calls[0].Prompt)
				}
				break
			}
			select {
			case <-deadline:
				t.Fatal("timed out waiting for callback to fire")
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
	})
}

func TestCallbackManagerShutdown(t *testing.T) {
	t.Run("shutdown cancels pending", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)

		mgr.Set("conv-1", 30*time.Second, "check 1")
		mgr.Set("conv-1", 30*time.Second, "check 2")

		mgr.Shutdown()

		callbacks := mgr.List("conv-1")
		for _, cb := range callbacks {
			if cb.State != CallbackStateCanceled {
				t.Errorf("expected all canceled after shutdown, got %s for %s", cb.State, cb.ID)
			}
		}
	})

	t.Run("shutdown is idempotent", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		mgr.Shutdown()
		mgr.Shutdown() // should not panic
	})
}

func TestCallbackManagerConcurrent(t *testing.T) {
	starter := &mockRunStarter{}
	mgr := NewCallbackManager(starter)
	defer mgr.Shutdown()

	var wg sync.WaitGroup
	var setCount int32

	// Concurrent sets
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			convID := fmt.Sprintf("conv-%d", i%3)
			_, err := mgr.Set(convID, 30*time.Second, fmt.Sprintf("check %d", i))
			if err == nil {
				atomic.AddInt32(&setCount, 1)
			}
		}(i)
	}

	// Concurrent lists
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mgr.List(fmt.Sprintf("conv-%d", i%3))
		}(i)
	}

	wg.Wait()

	if setCount == 0 {
		t.Error("expected some successful sets")
	}
}

// --- Tool Handler Tests ---

func testContextWithConversation(convID string) context.Context {
	return context.WithValue(context.Background(), ContextKeyRunMetadata, RunMetadata{
		ConversationID: convID,
	})
}

func TestSetDelayedCallbackTool(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		tool := setDelayedCallbackTool(mgr)
		if tool.Definition.Name != "set_delayed_callback" {
			t.Errorf("expected name set_delayed_callback, got %s", tool.Definition.Name)
		}

		ctx := testContextWithConversation("conv-1")
		args, _ := json.Marshal(map[string]string{"delay": "30s", "prompt": "check deploy"})
		result, err := tool.Handler(ctx, args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var info CallbackInfo
		if err := json.Unmarshal([]byte(result), &info); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}
		if info.State != CallbackStatePending {
			t.Errorf("expected pending, got %s", info.State)
		}
		if info.ConversationID != "conv-1" {
			t.Errorf("expected conv-1, got %s", info.ConversationID)
		}
	})

	t.Run("invalid delay format", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		tool := setDelayedCallbackTool(mgr)
		ctx := testContextWithConversation("conv-1")
		args, _ := json.Marshal(map[string]string{"delay": "not-a-duration", "prompt": "check"})
		_, err := tool.Handler(ctx, args)
		if err == nil {
			t.Fatal("expected error for invalid delay")
		}
	})

	t.Run("no run metadata", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		tool := setDelayedCallbackTool(mgr)
		args, _ := json.Marshal(map[string]string{"delay": "30s", "prompt": "check"})
		_, err := tool.Handler(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for missing run metadata")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		tool := setDelayedCallbackTool(mgr)
		ctx := testContextWithConversation("conv-1")
		_, err := tool.Handler(ctx, json.RawMessage(`{invalid`))
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestCancelDelayedCallbackTool(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		info, _ := mgr.Set("conv-1", 30*time.Second, "check")

		tool := cancelDelayedCallbackTool(mgr)
		ctx := testContextWithConversation("conv-1")
		args, _ := json.Marshal(map[string]string{"callback_id": info.ID})
		result, err := tool.Handler(ctx, args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var canceled CallbackInfo
		json.Unmarshal([]byte(result), &canceled)
		if canceled.State != CallbackStateCanceled {
			t.Errorf("expected canceled, got %s", canceled.State)
		}
	})

	t.Run("cancel nonexistent", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		tool := cancelDelayedCallbackTool(mgr)
		ctx := testContextWithConversation("conv-1")
		args, _ := json.Marshal(map[string]string{"callback_id": "nonexistent"})
		_, err := tool.Handler(ctx, args)
		if err == nil {
			t.Fatal("expected error for nonexistent callback")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		tool := cancelDelayedCallbackTool(mgr)
		ctx := testContextWithConversation("conv-1")
		_, err := tool.Handler(ctx, json.RawMessage(`{bad`))
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestListDelayedCallbacksTool(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		mgr.Set("conv-1", 10*time.Second, "check 1")
		mgr.Set("conv-1", 20*time.Second, "check 2")

		tool := listDelayedCallbacksTool(mgr)
		ctx := testContextWithConversation("conv-1")
		result, err := tool.Handler(ctx, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var callbacks []CallbackInfo
		json.Unmarshal([]byte(result), &callbacks)
		if len(callbacks) != 2 {
			t.Errorf("expected 2 callbacks, got %d", len(callbacks))
		}
	})

	t.Run("no run metadata", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		tool := listDelayedCallbacksTool(mgr)
		_, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("expected error for missing run metadata")
		}
	})
}

// --- Catalog Integration Tests ---

func TestDelayedCallbackCatalogIntegration(t *testing.T) {
	t.Run("callback tools included when enabled", func(t *testing.T) {
		starter := &mockRunStarter{}
		mgr := NewCallbackManager(starter)
		defer mgr.Shutdown()

		tools, err := BuildCatalog(BuildOptions{
			WorkspaceRoot:   t.TempDir(),
			EnableCallbacks: true,
			CallbackManager: mgr,
		})
		if err != nil {
			t.Fatalf("BuildCatalog error: %v", err)
		}

		names := make(map[string]bool)
		for _, tool := range tools {
			names[tool.Definition.Name] = true
		}

		expected := []string{"set_delayed_callback", "cancel_delayed_callback", "list_delayed_callbacks"}
		for _, name := range expected {
			if !names[name] {
				t.Errorf("expected tool %s in catalog", name)
			}
		}
	})

	t.Run("callback tools excluded when disabled", func(t *testing.T) {
		tools, err := BuildCatalog(BuildOptions{
			WorkspaceRoot:   t.TempDir(),
			EnableCallbacks: false,
		})
		if err != nil {
			t.Fatalf("BuildCatalog error: %v", err)
		}

		for _, tool := range tools {
			if tool.Definition.Name == "set_delayed_callback" ||
				tool.Definition.Name == "cancel_delayed_callback" ||
				tool.Definition.Name == "list_delayed_callbacks" {
				t.Errorf("unexpected callback tool %s in catalog when disabled", tool.Definition.Name)
			}
		}
	})

	t.Run("callback tools excluded when manager nil", func(t *testing.T) {
		tools, err := BuildCatalog(BuildOptions{
			WorkspaceRoot:   t.TempDir(),
			EnableCallbacks: true,
			CallbackManager: nil,
		})
		if err != nil {
			t.Fatalf("BuildCatalog error: %v", err)
		}

		for _, tool := range tools {
			if tool.Definition.Name == "set_delayed_callback" {
				t.Error("unexpected set_delayed_callback tool when manager is nil")
			}
		}
	})
}
