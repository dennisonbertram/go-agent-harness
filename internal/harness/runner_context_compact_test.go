package harness

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	htools "go-agent-harness/internal/harness/tools"
)

// TestGetRunContextStatus_NotFound verifies ErrRunNotFound for an unknown run.
func TestGetRunContextStatus_NotFound(t *testing.T) {
	t.Parallel()

	runner := NewRunner(&staticRunnerProvider{result: CompletionResult{Content: "done"}},
		NewRegistry(), RunnerConfig{DefaultModel: "test", MaxSteps: 2})

	_, err := runner.GetRunContextStatus("nonexistent-run-id")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != ErrRunNotFound {
		t.Fatalf("expected ErrRunNotFound, got: %v", err)
	}
}

// TestGetRunContextStatus_ReturnsData verifies context status is returned for a
// known run and the pressure field is one of the expected values.
func TestGetRunContextStatus_ReturnsData(t *testing.T) {
	t.Parallel()

	blockCh := make(chan struct{})
	releaseCh := make(chan struct{})

	provider := &contextCompactGatingProvider{
		results: []CompletionResult{{Content: "done"}},
		beforeCall: func(idx int) {
			if idx == 0 {
				close(blockCh)
				<-releaseCh
			}
		},
	}

	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "test",
		MaxSteps:     2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	<-blockCh

	status, err := runner.GetRunContextStatus(run.ID)
	if err != nil {
		t.Fatalf("GetRunContextStatus: %v", err)
	}

	valid := map[string]bool{"low": true, "medium": true, "high": true}
	if !valid[status.ContextPressure] {
		t.Errorf("unexpected context_pressure %q", status.ContextPressure)
	}
	if status.MessageCount < 0 {
		t.Errorf("expected non-negative message_count, got %d", status.MessageCount)
	}
	if status.EstimatedTokens < 0 {
		t.Errorf("expected non-negative estimated_tokens, got %d", status.EstimatedTokens)
	}

	close(releaseCh)
}

// TestContextPressureLevel verifies the thresholds used to compute pressure level.
func TestContextPressureLevel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		tokens int
		want   string
	}{
		{0, "low"},
		{1000, "low"},
		{30000, "low"},
		{30001, "medium"},
		{60000, "medium"},
		{60001, "high"},
		{200000, "high"},
	}
	for _, tc := range cases {
		got := contextPressureLevel(tc.tokens)
		if got != tc.want {
			t.Errorf("contextPressureLevel(%d) = %q, want %q", tc.tokens, got, tc.want)
		}
	}
}

// TestCompactRun_NotFound verifies ErrRunNotFound for unknown run.
func TestCompactRun_NotFound(t *testing.T) {
	t.Parallel()

	runner := NewRunner(&staticRunnerProvider{result: CompletionResult{Content: "done"}},
		NewRegistry(), RunnerConfig{DefaultModel: "test", MaxSteps: 2})

	_, err := runner.CompactRun(context.Background(), "nonexistent-run-id", CompactRunRequest{Mode: "strip"})
	if err != ErrRunNotFound {
		t.Fatalf("expected ErrRunNotFound, got: %v", err)
	}
}

// TestCompactRun_NotActive verifies ErrRunNotActive for a completed run.
func TestCompactRun_NotActive(t *testing.T) {
	t.Parallel()

	provider := &staticRunnerProvider{result: CompletionResult{Content: "done"}}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "test",
		MaxSteps:     2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	// Wait for completion.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		state, ok := runner.GetRun(run.ID)
		if !ok {
			t.Fatal("run disappeared")
		}
		if state.Status == RunStatusCompleted || state.Status == RunStatusFailed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	_, err = runner.CompactRun(context.Background(), run.ID, CompactRunRequest{Mode: "strip"})
	if err != ErrRunNotActive {
		t.Fatalf("expected ErrRunNotActive, got: %v", err)
	}
}

// TestCompactRun_InvalidMode verifies an error is returned for an unknown mode.
func TestCompactRun_InvalidMode(t *testing.T) {
	t.Parallel()

	blockCh := make(chan struct{})
	releaseCh := make(chan struct{})

	provider := &contextCompactGatingProvider{
		results: []CompletionResult{{Content: "done"}},
		beforeCall: func(idx int) {
			if idx == 0 {
				close(blockCh)
				<-releaseCh
			}
		},
	}

	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "test",
		MaxSteps:     2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	<-blockCh

	_, err = runner.CompactRun(context.Background(), run.ID, CompactRunRequest{Mode: "badmode"})
	if err == nil {
		t.Fatal("expected error for invalid mode, got nil")
	}

	close(releaseCh)
}

// TestCompactRun_StripMode verifies strip compaction runs without error on an
// active run and returns a MessagesRemoved count >= 0.
func TestCompactRun_StripMode(t *testing.T) {
	t.Parallel()

	blockCh := make(chan struct{})
	releaseCh := make(chan struct{})

	provider := &contextCompactGatingProvider{
		results: []CompletionResult{{Content: "done"}},
		beforeCall: func(idx int) {
			if idx == 0 {
				close(blockCh)
				<-releaseCh
			}
		},
	}

	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "test",
		MaxSteps:     2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	<-blockCh

	result, err := runner.CompactRun(context.Background(), run.ID, CompactRunRequest{Mode: "strip"})
	if err != nil {
		t.Fatalf("CompactRun strip: %v", err)
	}
	if result.MessagesRemoved < 0 {
		t.Errorf("expected non-negative MessagesRemoved, got %d", result.MessagesRemoved)
	}

	close(releaseCh)
}

// TestCompactRun_ConcurrentSafe verifies no races when GetRunContextStatus and
// CompactRun are called concurrently while a run is active.
func TestCompactRun_ConcurrentSafe(t *testing.T) {
	t.Parallel()

	blockCh := make(chan struct{})
	releaseCh := make(chan struct{})

	provider := &contextCompactGatingProvider{
		results: []CompletionResult{{Content: "done"}},
		beforeCall: func(idx int) {
			if idx == 0 {
				close(blockCh)
				<-releaseCh
			}
		},
	}

	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "test",
		MaxSteps:     2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	<-blockCh

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = runner.GetRunContextStatus(run.ID)
		}()
	}
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = runner.CompactRun(context.Background(), run.ID, CompactRunRequest{Mode: "strip"})
		}()
	}

	wg.Wait()
	close(releaseCh)
}

// TestMessagesAsTranscriptSnapshot verifies meta messages are excluded.
func TestMessagesAsTranscriptSnapshot(t *testing.T) {
	t.Parallel()

	msgs := []Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi", IsMeta: true}, // should be excluded
		{Role: "assistant", Content: "how can I help?"},
		{Role: "tool", Content: "result", ToolCallID: "tc1"},
	}

	snap := messagesAsTranscriptSnapshot(msgs)

	if len(snap) != 3 {
		t.Errorf("expected 3 transcript messages (meta excluded), got %d", len(snap))
	}
	for _, m := range snap {
		if m.Role == "assistant" && m.Content == "hi" {
			t.Error("meta message should have been excluded")
		}
	}
}

// TestCompactStripHTTP verifies strip compaction removes tool messages.
func TestCompactStripHTTP(t *testing.T) {
	t.Parallel()

	msgs := []htools.TranscriptMessage{
		{Index: 0, Role: "user", Content: "hello"},
		{Index: 1, Role: "assistant", Content: "calling tool"},
		{Index: 2, Role: "tool", Content: "tool result", ToolCallID: "tc1"},
		{Index: 3, Role: "user", Content: "second"},
		{Index: 4, Role: "assistant", Content: "done"},
	}

	result, err := compactMessagesHTTP(context.Background(), msgs, "strip", 2, nil)
	if err != nil {
		t.Fatalf("compactMessagesHTTP strip: %v", err)
	}
	// tool message should be stripped
	for _, m := range result {
		if m.Role == "tool" {
			t.Errorf("expected tool message to be stripped, but found one: %v", m)
		}
	}
}

// TestCompactSummarizeHTTP verifies summarize compaction calls the summarizer.
func TestCompactSummarizeHTTP(t *testing.T) {
	t.Parallel()

	msgs := []htools.TranscriptMessage{
		{Index: 0, Role: "user", Content: "hello"},
		{Index: 1, Role: "assistant", Content: "hi"},
		{Index: 2, Role: "user", Content: "world"},
		{Index: 3, Role: "assistant", Content: "done"},
	}

	summarizer := &fixedSummarizer{summary: "a summary"}
	result, err := compactMessagesHTTP(context.Background(), msgs, "summarize", 2, summarizer)
	if err != nil {
		t.Fatalf("compactMessagesHTTP summarize: %v", err)
	}
	// Should contain a compact_summary system message.
	found := false
	for _, m := range result {
		if m.Role == "system" && m.Name == "compact_summary" {
			found = true
		}
	}
	if !found {
		t.Error("expected compact_summary system message in result")
	}
}

// TestCompactHybridHTTP verifies hybrid compaction runs without error.
func TestCompactHybridHTTP(t *testing.T) {
	t.Parallel()

	// Create a large tool result to trigger hybrid removal.
	largeContent := string(make([]byte, 3000))
	for i := range largeContent {
		largeContent = largeContent[:i] + "x" + largeContent[i+1:]
	}

	msgs := []htools.TranscriptMessage{
		{Index: 0, Role: "user", Content: "hello"},
		{Index: 1, Role: "assistant", Content: "calling tool"},
		{Index: 2, Role: "tool", Content: largeContent, ToolCallID: "tc1"},
		{Index: 3, Role: "user", Content: "second"},
		{Index: 4, Role: "assistant", Content: "done"},
	}

	summarizer := &fixedSummarizer{summary: "hybrid summary"}
	result, err := compactMessagesHTTP(context.Background(), msgs, "hybrid", 2, summarizer)
	if err != nil {
		t.Fatalf("compactMessagesHTTP hybrid: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected non-empty result from hybrid compaction")
	}
}

// TestCompactMessagesHTTP_NoOp verifies that when there is nothing to compact,
// the original slice is returned unchanged.
func TestCompactMessagesHTTP_NoOp(t *testing.T) {
	t.Parallel()

	msgs := []htools.TranscriptMessage{
		{Index: 0, Role: "user", Content: "hello"},
		{Index: 1, Role: "assistant", Content: "done"},
	}

	result, err := compactMessagesHTTP(context.Background(), msgs, "strip", 4, nil)
	if err != nil {
		t.Fatalf("compactMessagesHTTP: %v", err)
	}
	if len(result) != len(msgs) {
		t.Errorf("expected %d messages (no-op), got %d", len(msgs), len(result))
	}
}

// TestCompactRun_SummarizeMode verifies summarize mode compaction runs via CompactRun.
func TestCompactRun_SummarizeMode(t *testing.T) {
	t.Parallel()

	blockCh := make(chan struct{})
	releaseCh := make(chan struct{})

	provider := &contextCompactGatingProvider{
		results: []CompletionResult{{Content: "a summary"}, {Content: "done"}},
		beforeCall: func(idx int) {
			if idx == 0 {
				close(blockCh)
				<-releaseCh
			}
		},
	}

	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "test",
		MaxSteps:     2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	<-blockCh

	_, err = runner.CompactRun(context.Background(), run.ID, CompactRunRequest{Mode: "summarize"})
	// summarize may succeed or fail (no messages to compact yet), just confirm no panic.
	_ = err

	close(releaseCh)
}

// TestCompactRun_HybridMode verifies hybrid mode compaction runs via CompactRun.
func TestCompactRun_HybridMode(t *testing.T) {
	t.Parallel()

	blockCh := make(chan struct{})
	releaseCh := make(chan struct{})

	provider := &contextCompactGatingProvider{
		results: []CompletionResult{{Content: "hybrid summary"}, {Content: "done"}},
		beforeCall: func(idx int) {
			if idx == 0 {
				close(blockCh)
				<-releaseCh
			}
		},
	}

	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "test",
		MaxSteps:     2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	<-blockCh

	_, err = runner.CompactRun(context.Background(), run.ID, CompactRunRequest{Mode: "hybrid"})
	// hybrid may succeed or fail (no messages to compact yet), just confirm no panic.
	_ = err

	close(releaseCh)
}

// fixedSummarizer is a MessageSummarizer that always returns a fixed summary.
type fixedSummarizer struct {
	summary string
}

func (s *fixedSummarizer) SummarizeMessages(_ context.Context, _ []map[string]any) (string, error) {
	return s.summary, nil
}

// contextCompactGatingProvider is a scripted provider with a beforeCall hook
// for use in context/compact tests.
type contextCompactGatingProvider struct {
	mu         sync.Mutex
	results    []CompletionResult
	calls      int
	beforeCall func(idx int)
}

func (p *contextCompactGatingProvider) Complete(_ context.Context, _ CompletionRequest) (CompletionResult, error) {
	p.mu.Lock()
	idx := p.calls
	p.calls++
	var result CompletionResult
	if idx < len(p.results) {
		result = p.results[idx]
	}
	p.mu.Unlock()

	if p.beforeCall != nil {
		p.beforeCall(idx)
	}
	return result, nil
}

// staticRunnerProvider is a minimal provider returning a fixed result, for
// context/compact tests that don't need gating.
type staticRunnerProvider struct {
	result CompletionResult
}

func (p *staticRunnerProvider) Complete(_ context.Context, _ CompletionRequest) (CompletionResult, error) {
	return p.result, nil
}

// TestCompactRunSurvivesConcurrentExecute verifies that CompactRun() results
// are not overwritten by execute()'s stale local messages copy.
// Regression test for #232.
func TestCompactRunSurvivesConcurrentExecute(t *testing.T) {
	t.Parallel()

	// step4Gate blocks the step 4 LLM call so we can compact in between.
	step4Gate := make(chan struct{})

	// Provider: steps 1-3 return tool calls (loop continues), step 4 returns text.
	// This generates 4 turns (user + 3x assistant_tool) so strip with keepLast=2
	// actually removes tool messages from the earlier turns.
	provider := &contextCompactGatingProvider{
		results: []CompletionResult{
			{
				ToolCalls: []ToolCall{{
					ID:        "call-1",
					Name:      "echo_json",
					Arguments: `{"message":"step1"}`,
				}},
			},
			{
				ToolCalls: []ToolCall{{
					ID:        "call-2",
					Name:      "echo_json",
					Arguments: `{"message":"step2"}`,
				}},
			},
			{
				ToolCalls: []ToolCall{{
					ID:        "call-3",
					Name:      "echo_json",
					Arguments: `{"message":"step3"}`,
				}},
			},
			{Content: "final answer"},
		},
		beforeCall: func(idx int) {
			if idx == 3 {
				// Step 4 LLM call: wait for the test to compact first.
				<-step4Gate
			}
		},
	}

	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name:        "echo_json",
		Description: "echoes payload",
		Parameters:  map[string]any{"type": "object"},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return `{"echo":"ok"}`, nil
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	runner := NewRunner(provider, registry, RunnerConfig{
		DefaultModel: "test",
		MaxSteps:     6,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	// Wait until step 4 is blocked (provider.calls == 4 means step 4 entered beforeCall).
	deadline := time.Now().Add(5 * time.Second)
	gateReached := false
	for time.Now().Before(deadline) {
		provider.mu.Lock()
		calls := provider.calls
		provider.mu.Unlock()
		if calls >= 4 {
			gateReached = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !gateReached {
		t.Fatalf("timed out waiting for step 4 gate")
	}

	// At this point steps 1-3 are fully done (tool calls executed, messages stored).
	// Step 4 is blocked in beforeCall. execute()'s local `messages` has the full
	// history: user + 3x(assistant+tool) = 7 messages = 4 turns.
	msgsBefore := runner.GetRunMessages(run.ID)
	beforeCount := len(msgsBefore)
	if beforeCount < 7 {
		t.Fatalf("expected at least 7 messages after steps 1-3, got %d", beforeCount)
	}

	// Count tool messages before compaction.
	toolMsgsBefore := 0
	for _, m := range msgsBefore {
		if m.Role == "tool" {
			toolMsgsBefore++
		}
	}

	// Compact: strip mode removes tool messages. Use KeepLast=2 so the early
	// assistant_tool turns (turns 1-2) fall outside the keep window.
	result, err := runner.CompactRun(context.Background(), run.ID, CompactRunRequest{Mode: "strip", KeepLast: 2})
	if err != nil {
		t.Fatalf("CompactRun: %v", err)
	}
	if result.MessagesRemoved == 0 && toolMsgsBefore > 0 {
		t.Fatal("expected strip to remove tool messages, but removed 0")
	}

	compactedCount := len(runner.GetRunMessages(run.ID))

	// Release step 4.
	close(step4Gate)

	// Wait for run to complete.
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		state, ok := runner.GetRun(run.ID)
		if !ok {
			t.Fatal("run disappeared")
		}
		if state.Status == RunStatusCompleted || state.Status == RunStatusFailed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found after completion")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q", state.Status)
	}

	// Step 4 has no tool calls, so execute() appends exactly 1 assistant message.
	// With the fix: final = compactedCount + 1 (re-reads compacted base).
	// With the bug: final = beforeCount + 1 (stale messages overwrite compaction).
	msgsFinal := runner.GetRunMessages(run.ID)
	finalCount := len(msgsFinal)
	expectedWithFix := compactedCount + 1

	if finalCount != expectedWithFix {
		t.Errorf("compaction not preserved: final=%d, want=%d (compacted=%d + 1 assistant), pre-compact=%d",
			finalCount, expectedWithFix, compactedCount, beforeCount)
	}
}

// TestCompactRunAtStepBoundary verifies CompactRun takes effect when called
// after tool-call steps complete but before the final step begins.
// Regression test for #232.
func TestCompactRunAtStepBoundary(t *testing.T) {
	t.Parallel()

	step4Gate := make(chan struct{})

	// Provider: steps 1-3 return tool calls, step 4 returns text.
	// 4 turns total so keepLast=2 leaves 2 turns in the compact window.
	provider := &contextCompactGatingProvider{
		results: []CompletionResult{
			{
				ToolCalls: []ToolCall{{
					ID:        "call-1",
					Name:      "echo_json",
					Arguments: `{"message":"s1"}`,
				}},
			},
			{
				ToolCalls: []ToolCall{{
					ID:        "call-2",
					Name:      "echo_json",
					Arguments: `{"message":"s2"}`,
				}},
			},
			{
				ToolCalls: []ToolCall{{
					ID:        "call-3",
					Name:      "echo_json",
					Arguments: `{"message":"s3"}`,
				}},
			},
			{Content: "done"},
		},
		beforeCall: func(idx int) {
			if idx == 3 {
				<-step4Gate
			}
		},
	}

	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name:        "echo_json",
		Description: "echoes payload",
		Parameters:  map[string]any{"type": "object"},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return `{"echo":"ok"}`, nil
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	runner := NewRunner(provider, registry, RunnerConfig{
		DefaultModel: "test",
		MaxSteps:     6,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	// Wait until step 4 is gated.
	deadline := time.Now().Add(5 * time.Second)
	gateReached := false
	for time.Now().Before(deadline) {
		provider.mu.Lock()
		calls := provider.calls
		provider.mu.Unlock()
		if calls >= 4 {
			gateReached = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !gateReached {
		t.Fatalf("timed out waiting for step 4 gate")
	}

	// Compact while step 4 is gated (steps 1-3 fully processed).
	msgsBefore := runner.GetRunMessages(run.ID)
	_, err = runner.CompactRun(context.Background(), run.ID, CompactRunRequest{Mode: "strip", KeepLast: 2})
	if err != nil {
		t.Fatalf("CompactRun: %v", err)
	}

	msgsAfterCompact := runner.GetRunMessages(run.ID)
	compactedCount := len(msgsAfterCompact)

	// Release step 4 so the run completes.
	close(step4Gate)

	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		state, ok := runner.GetRun(run.ID)
		if !ok {
			t.Fatal("run disappeared")
		}
		if state.Status == RunStatusCompleted || state.Status == RunStatusFailed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %q", state.Status)
	}
	if state.Output != "done" {
		t.Errorf("expected output %q, got %q", "done", state.Output)
	}

	// Step 4 adds exactly 1 assistant message (no tool calls).
	// With the fix: final = compactedCount + 1 (re-reads compacted base).
	msgsFinal := runner.GetRunMessages(run.ID)
	finalCount := len(msgsFinal)
	expectedWithFix := compactedCount + 1

	if finalCount != expectedWithFix {
		t.Errorf("compaction not preserved: final=%d, want=%d (compacted=%d + 1 assistant), pre-compact=%d",
			finalCount, expectedWithFix, compactedCount, len(msgsBefore))
	}
}

// TestCompactRun_HonoursPerRequestSummarizerModel is a regression test for the
// HIGH issue in #25: CompactRun was calling r.NewMessageSummarizer() (no model
// override) instead of r.newMessageSummarizerWithModel(state.resolvedRoleModels.Summarizer).
// This meant a per-request RoleModels.Summarizer was silently ignored during
// manual compaction triggered via the HTTP API.
func TestCompactRun_HonoursPerRequestSummarizerModel(t *testing.T) {
	t.Parallel()

	blockCh := make(chan struct{})
	releaseCh := make(chan struct{})

	// modelCapProvider records which model each Complete call uses,
	// and optionally gates specific calls.
	type modelCapProvider struct {
		mu             sync.Mutex
		calls          int
		capturedModels []string
		results        []CompletionResult
		gate           func(idx int)
	}

	prov := &modelCapProvider{
		results: []CompletionResult{
			// Call 0: main LLM step (gated so we can compact mid-run).
			{Content: "done"},
			// Call 1: summarization call issued by CompactRun in summarize mode.
			{Content: "a compact summary"},
		},
		gate: func(idx int) {
			if idx == 0 {
				close(blockCh)
				<-releaseCh
			}
		},
	}

	// Implement CompletionProvider for modelCapProvider via a wrapper.
	provFn := func(ctx context.Context, req CompletionRequest) (CompletionResult, error) {
		prov.mu.Lock()
		idx := prov.calls
		prov.calls++
		prov.capturedModels = append(prov.capturedModels, req.Model)
		var result CompletionResult
		if idx < len(prov.results) {
			result = prov.results[idx]
		}
		gate := prov.gate
		prov.mu.Unlock()

		if gate != nil {
			gate(idx)
		}
		return result, nil
	}

	runner := NewRunner(compactFuncProvider(provFn), NewRegistry(), RunnerConfig{
		DefaultModel: "default-model",
		// No config-level Summarizer — only the per-request override must be used.
	})

	const perRequestSummarizer = "per-request-summarizer-model"
	run, err := runner.StartRun(RunRequest{
		Prompt: "hello",
		RoleModels: &RoleModels{
			Summarizer: perRequestSummarizer,
		},
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Wait until the run is active (first LLM call is blocked).
	<-blockCh

	// Trigger manual compaction in summarize mode while run is blocked.
	// The run has the initial user message in state, so summarize will attempt
	// to call the provider.
	// With the bug: uses NewMessageSummarizer() → "default-model".
	// With the fix: uses newMessageSummarizerWithModel(perRequestSummarizer).
	_, _ = runner.CompactRun(context.Background(), run.ID, CompactRunRequest{
		Mode:     "summarize",
		KeepLast: 4,
	})

	// Release the blocked LLM step.
	close(releaseCh)

	// Wait for run to complete.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		state, ok := runner.GetRun(run.ID)
		if !ok {
			t.Fatal("run disappeared")
		}
		if state.Status == RunStatusCompleted || state.Status == RunStatusFailed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Check: any provider call after call 0 (the main LLM step) is a
	// summarization call. It must have used the per-request summarizer model.
	prov.mu.Lock()
	captured := append([]string(nil), prov.capturedModels...)
	prov.mu.Unlock()

	for i := 1; i < len(captured); i++ {
		if captured[i] != perRequestSummarizer {
			t.Errorf("summarization call %d used model %q, want %q (per-request summarizer override ignored)",
				i, captured[i], perRequestSummarizer)
		}
	}
}

// compactFuncProvider adapts a plain function to the CompletionProvider interface.
// Named distinctly to avoid collision with funcProvider in runner_tool_filter_test.go.
type compactFuncProvider func(ctx context.Context, req CompletionRequest) (CompletionResult, error)

func (f compactFuncProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error) {
	return f(ctx, req)
}
