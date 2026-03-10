package harness

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// noopToolHandler is a tool handler that always returns "ok".
func noopToolHandler(_ context.Context, _ json.RawMessage) (string, error) {
	return `"ok"`, nil
}

// registerNoopTool registers a noop tool with the given name in a registry.
func registerNoopTool(t *testing.T, reg *Registry, name string) {
	t.Helper()
	def := ToolDefinition{
		Name:        name,
		Description: "does nothing",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
	}
	if err := reg.Register(def, noopToolHandler); err != nil {
		t.Fatalf("register tool %s: %v", name, err)
	}
}

// TestSteerRun_BasicInjection verifies that a steering message sent while a run
// is active gets injected into the transcript before the next LLM call.
func TestSteerRun_BasicInjection(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var capturedMessages [][]Message

	blockDuringFirst := make(chan struct{})
	releaseDuringFirst := make(chan struct{})

	capturer := &steerGatingProvider{
		turns: []CompletionResult{
			// First call: return a tool call so the loop continues
			{ToolCalls: []ToolCall{{ID: "tc1", Name: "noop_steer_basic", Arguments: `{}`}}},
			// Second call: finish
			{Content: "done"},
		},
		// Block DURING first call so the test can inject steering before step 2
		beforeCall: func(idx int) {
			if idx == 0 {
				close(blockDuringFirst)
				<-releaseDuringFirst
			}
		},
		afterCall: func(idx int, req CompletionRequest) {
			mu.Lock()
			capturedMessages = append(capturedMessages, append([]Message(nil), req.Messages...))
			mu.Unlock()
		},
	}

	registry := NewRegistry()
	registerNoopTool(t, registry, "noop_steer_basic")

	runner := NewRunner(capturer, registry, RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     4,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Wait until first LLM call is in progress (blocked inside provider)
	<-blockDuringFirst

	// Inject steering message while run is blocked in LLM call
	if err := runner.SteerRun(run.ID, "please focus on the main issue"); err != nil {
		t.Fatalf("SteerRun: %v", err)
	}

	// Release the first LLM call
	close(releaseDuringFirst)

	// Wait for run to complete
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		state, ok := runner.GetRun(run.ID)
		if !ok {
			t.Fatalf("run not found")
		}
		if state.Status == RunStatusCompleted || state.Status == RunStatusFailed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// The second LLM call should see the steering message injected
	mu.Lock()
	defer mu.Unlock()

	if len(capturedMessages) < 2 {
		t.Fatalf("expected at least 2 LLM calls, got %d", len(capturedMessages))
	}

	secondCallMsgs := capturedMessages[1]
	found := false
	for _, msg := range secondCallMsgs {
		if msg.Role == "user" && msg.Content == "please focus on the main issue" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("steering message not found in second LLM call messages: %v", secondCallMsgs)
	}
}

// TestSteerRun_RunNotFound verifies ErrRunNotFound is returned for unknown runs.
func TestSteerRun_RunNotFound(t *testing.T) {
	t.Parallel()

	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{MaxSteps: 2})
	err := runner.SteerRun("nonexistent-run-id", "steer me")
	if err == nil {
		t.Fatal("expected error for non-existent run")
	}
	if err != ErrRunNotFound {
		t.Errorf("expected ErrRunNotFound, got: %v", err)
	}
}

// TestSteerRun_EmptyMessage verifies that empty steering messages are rejected.
func TestSteerRun_EmptyMessage(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{
		turns: []CompletionResult{{Content: "done"}},
	}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	// Give run time to start
	time.Sleep(10 * time.Millisecond)

	err = runner.SteerRun(run.ID, "")
	if err == nil {
		t.Fatal("expected error for empty steering message")
	}
}

// TestSteerRun_CompletedRun verifies that steering a completed run returns ErrRunNotActive.
func TestSteerRun_CompletedRun(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{
		turns: []CompletionResult{{Content: "done"}},
	}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Wait for run to complete
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		state, ok := runner.GetRun(run.ID)
		if !ok {
			t.Fatalf("run not found")
		}
		if state.Status == RunStatusCompleted || state.Status == RunStatusFailed {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	err = runner.SteerRun(run.ID, "too late")
	if err == nil {
		t.Fatal("expected error steering completed run")
	}
	if err != ErrRunNotActive {
		t.Errorf("expected ErrRunNotActive, got: %v", err)
	}
}

// TestSteerRun_BufferFull verifies that sending too many steers returns ErrSteeringBufferFull.
func TestSteerRun_BufferFull(t *testing.T) {
	t.Parallel()

	blocker := make(chan struct{})
	provider := &blockingProvider{blocker: blocker}

	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	// Give run time to start and block
	time.Sleep(20 * time.Millisecond)

	// Fill the steering buffer (capacity is 10 per issue spec)
	var lastErr error
	for i := 0; i < 15; i++ {
		lastErr = runner.SteerRun(run.ID, "steer message")
		if lastErr != nil {
			break
		}
	}

	// Release the blocking provider
	close(blocker)

	if lastErr == nil {
		t.Error("expected ErrSteeringBufferFull when sending too many steers")
	} else if lastErr != ErrSteeringBufferFull {
		t.Errorf("expected ErrSteeringBufferFull, got: %v", lastErr)
	}
}

// TestSteerRun_MultipleMessages verifies that multiple steering messages are
// all injected in order before the next LLM call.
func TestSteerRun_MultipleMessages(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var capturedMessages [][]Message

	blockAfterFirst := make(chan struct{})
	releaseFirst := make(chan struct{})

	capturer := &steerGatingProvider{
		turns: []CompletionResult{
			{ToolCalls: []ToolCall{{ID: "tc1", Name: "noop_steer_multi", Arguments: `{}`}}},
			{Content: "done"},
		},
		beforeCall: func(idx int) {
			if idx == 0 {
				close(blockAfterFirst)
				<-releaseFirst
			}
		},
		afterCall: func(idx int, req CompletionRequest) {
			mu.Lock()
			capturedMessages = append(capturedMessages, append([]Message(nil), req.Messages...))
			mu.Unlock()
		},
	}

	registry := NewRegistry()
	registerNoopTool(t, registry, "noop_steer_multi")

	runner := NewRunner(capturer, registry, RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     4,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Wait until first LLM call is about to happen
	<-blockAfterFirst

	// Inject two steering messages
	if err := runner.SteerRun(run.ID, "steer one"); err != nil {
		t.Fatalf("SteerRun 1: %v", err)
	}
	if err := runner.SteerRun(run.ID, "steer two"); err != nil {
		t.Fatalf("SteerRun 2: %v", err)
	}

	// Release the first LLM call
	close(releaseFirst)

	// Wait for run to complete
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		state, ok := runner.GetRun(run.ID)
		if !ok {
			t.Fatalf("run not found")
		}
		if state.Status == RunStatusCompleted || state.Status == RunStatusFailed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(capturedMessages) < 2 {
		t.Fatalf("expected at least 2 LLM calls, got %d", len(capturedMessages))
	}

	secondCallMsgs := capturedMessages[1]
	var userMsgs []string
	for _, msg := range secondCallMsgs {
		if msg.Role == "user" {
			userMsgs = append(userMsgs, msg.Content)
		}
	}

	// Both steering messages should appear
	foundOne, foundTwo := false, false
	for _, m := range userMsgs {
		if m == "steer one" {
			foundOne = true
		}
		if m == "steer two" {
			foundTwo = true
		}
	}
	if !foundOne {
		t.Errorf("first steering message not found in second LLM call; user messages: %v", userMsgs)
	}
	if !foundTwo {
		t.Errorf("second steering message not found in second LLM call; user messages: %v", userMsgs)
	}
}

// TestSteerRun_SSEEvent verifies that a steering.received SSE event is emitted.
func TestSteerRun_SSEEvent(t *testing.T) {
	t.Parallel()

	blockCh := make(chan struct{})
	releaseCh := make(chan struct{})

	capturer := &steerGatingProvider{
		turns: []CompletionResult{
			{ToolCalls: []ToolCall{{ID: "tc1", Name: "noop_steer_sse", Arguments: `{}`}}},
			{Content: "done"},
		},
		beforeCall: func(idx int) {
			if idx == 0 {
				close(blockCh)
				<-releaseCh
			}
		},
		afterCall: func(int, CompletionRequest) {},
	}

	registry := NewRegistry()
	registerNoopTool(t, registry, "noop_steer_sse")

	runner := NewRunner(capturer, registry, RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     4,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Subscribe to events
	history, eventCh, cancel, err := runner.Subscribe(run.ID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	_ = history

	// Wait until first LLM call is happening
	<-blockCh

	// Steer
	if err := runner.SteerRun(run.ID, "redirect now"); err != nil {
		t.Fatalf("SteerRun: %v", err)
	}

	// Release
	close(releaseCh)

	// Look for steering.received in events; drain until terminal event
	// so we only cancel AFTER the run is done (avoids send-on-closed-channel race).
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	found := false
	for {
		select {
		case evt, ok := <-eventCh:
			if !ok {
				// channel closed by cancel, stop
				goto done
			}
			if evt.Type == EventSteeringReceived {
				msg, _ := evt.Payload["message"].(string)
				if msg == "redirect now" {
					found = true
				}
			}
			if IsTerminalEvent(evt.Type) {
				// Run complete: safe to cancel now
				cancel()
				goto done
			}
		case <-timer.C:
			cancel()
			goto done
		}
	}
done:

	if !found {
		t.Error("steering.received event not emitted")
	}
}

// TestSteerRun_ConcurrentSafety tests concurrent SteerRun calls don't race.
func TestSteerRun_ConcurrentSafety(t *testing.T) {
	t.Parallel()

	blocker := make(chan struct{})
	provider := &blockingProvider{blocker: blocker}

	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	// Give run time to be active
	time.Sleep(10 * time.Millisecond)

	// Fire concurrent steers
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = runner.SteerRun(run.ID, "concurrent steer")
		}()
	}
	wg.Wait()

	// Release the provider
	close(blocker)
}

// --- test helper providers ---

// steerGatingProvider is a scripted provider with beforeCall/afterCall hooks.
type steerGatingProvider struct {
	mu         sync.Mutex
	turns      []CompletionResult
	calls      int
	beforeCall func(idx int)
	afterCall  func(idx int, req CompletionRequest)
}

func (p *steerGatingProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error) {
	p.mu.Lock()
	idx := p.calls
	p.calls++
	var result CompletionResult
	if idx < len(p.turns) {
		result = p.turns[idx]
	}
	p.mu.Unlock()

	if p.beforeCall != nil {
		p.beforeCall(idx)
	}
	if p.afterCall != nil {
		p.afterCall(idx, req)
	}
	return result, nil
}
