package harness

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	htools "go-agent-harness/internal/harness/tools"
	om "go-agent-harness/internal/observationalmemory"
)

// resetContextProvider drives the step loop through a reset_context call
// and then a final completion.
type resetContextProvider struct {
	mu      sync.Mutex
	calls   int
	results []CompletionResult
}

func (p *resetContextProvider) Complete(_ context.Context, req CompletionRequest) (CompletionResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	idx := p.calls
	p.calls++
	if idx < len(p.results) {
		return p.results[idx], nil
	}
	return CompletionResult{Content: "done"}, nil
}

// makeResetContextRegistry creates and registers the reset_context tool.
// Before implementation the call to htools.ResetContextTool() will not compile;
// after implementation it returns a Registry with the real tool registered.
func makeResetContextRegistry() *Registry {
	reg := NewRegistry()
	tl := htools.ResetContextTool()
	// htools.Handler and harness.ToolHandler have the same underlying function
	// signature but are distinct named types; cast explicitly.
	handler := ToolHandler(tl.Handler)
	_ = reg.Register(ToolDefinition{
		Name:        tl.Definition.Name,
		Description: tl.Definition.Description,
		Parameters:  tl.Definition.Parameters,
	}, handler)
	return reg
}

// captureContextResetStore records RecordContextReset calls for test assertions.
type captureContextResetStore struct {
	mu     sync.Mutex
	resets []contextResetRecord
}

type contextResetRecord struct {
	runID      string
	resetIndex int
	atStep     int
	persist    json.RawMessage
}

func (s *captureContextResetStore) RecordContextReset(_ context.Context, runID string, resetIndex, atStep int, persist json.RawMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resets = append(s.resets, contextResetRecord{
		runID:      runID,
		resetIndex: resetIndex,
		atStep:     atStep,
		persist:    persist,
	})
	return nil
}

func (s *captureContextResetStore) GetContextResets(_ context.Context, runID string) ([]ContextReset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []ContextReset
	for _, r := range s.resets {
		if r.runID == runID {
			out = append(out, ContextReset{
				RunID:      r.runID,
				ResetIndex: r.resetIndex,
				AtStep:     r.atStep,
				Persist:    r.persist,
			})
		}
	}
	return out, nil
}

// buildResetProvider creates a provider that:
//
//	step 1: returns a reset_context tool call with the given persist payload
//	step 2+: returns a terminal "done" completion
func buildResetProvider(persistJSON string) *resetContextProvider {
	return &resetContextProvider{
		results: []CompletionResult{
			{
				ToolCalls: []ToolCall{{
					ID:        "call-reset-1",
					Name:      "reset_context",
					Arguments: `{"persist":` + persistJSON + `}`,
				}},
			},
			{Content: "all done after reset"},
		},
	}
}

// buildDoubleResetProvider creates a provider with two resets then done.
func buildDoubleResetProvider() *resetContextProvider {
	return &resetContextProvider{
		results: []CompletionResult{
			{
				ToolCalls: []ToolCall{{
					ID:        "call-reset-1",
					Name:      "reset_context",
					Arguments: `{"persist":{"task":"first reset"}}`,
				}},
			},
			{
				ToolCalls: []ToolCall{{
					ID:        "call-reset-2",
					Name:      "reset_context",
					Arguments: `{"persist":{"task":"second reset"}}`,
				}},
			},
			{Content: "done after two resets"},
		},
	}
}

// TestResetContext_StepCounterContinues verifies that the step counter does not
// reset after a reset_context call — it continues incrementing from where it was.
func TestResetContext_StepCounterContinues(t *testing.T) {
	t.Parallel()

	provider := buildResetProvider(`{"task":"test step counter"}`)
	reg := makeResetContextRegistry()

	runner := NewRunner(provider, reg, RunnerConfig{
		DefaultModel:        "test",
		DefaultSystemPrompt: "you are a test agent",
		MaxSteps:            10,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "start"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	// We expect at least 2 step.started events: one before reset, one after.
	stepStartedCount := 0
	maxStep := 0
	for _, ev := range events {
		if ev.Type == EventRunStepStarted {
			stepStartedCount++
			// Payloads are map[string]any stored without JSON round-trip, so
			// numeric values are int not float64.
			switch s := ev.Payload["step"].(type) {
			case int:
				if s > maxStep {
					maxStep = s
				}
			case float64:
				if int(s) > maxStep {
					maxStep = int(s)
				}
			}
		}
	}
	if stepStartedCount < 2 {
		t.Errorf("expected >= 2 step.started events, got %d", stepStartedCount)
	}
	// After reset, step counter continues from the step where reset happened.
	// So if reset happened at step 1, the next step is 2, not 1 again.
	if maxStep < 2 {
		t.Errorf("expected step counter to continue past 1 after reset (maxStep=%d)", maxStep)
	}
}

// TestResetContext_CostContinues verifies that cost accumulation is not reset
// and that cost events continue to accumulate post-reset.
func TestResetContext_CostContinues(t *testing.T) {
	t.Parallel()

	provider := buildResetProvider(`{"task":"test cost"}`)
	reg := makeResetContextRegistry()

	runner := NewRunner(provider, reg, RunnerConfig{
		DefaultModel:        "test",
		DefaultSystemPrompt: "system prompt",
		MaxSteps:            10,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "start"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	// We expect usage.delta events from both steps.
	usageDeltaCount := 0
	for _, ev := range events {
		if ev.Type == EventUsageDelta {
			usageDeltaCount++
		}
	}
	// Should have at least 2 usage.delta events (one per LLM turn).
	if usageDeltaCount < 2 {
		t.Errorf("expected >= 2 usage.delta events, got %d (cost should accumulate across reset)", usageDeltaCount)
	}
}

// TestResetContext_RunIDUnchanged verifies that the run ID stays the same before
// and after a context reset.
func TestResetContext_RunIDUnchanged(t *testing.T) {
	t.Parallel()

	provider := buildResetProvider(`{"task":"check run id"}`)
	reg := makeResetContextRegistry()

	runner := NewRunner(provider, reg, RunnerConfig{
		DefaultModel:        "test",
		DefaultSystemPrompt: "system",
		MaxSteps:            10,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "start"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	originalRunID := run.ID

	_, err = collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	// The run should still be found under the same ID.
	finalRun, ok := runner.GetRun(originalRunID)
	if !ok {
		t.Fatalf("run %q not found after reset", originalRunID)
	}
	if finalRun.ID != originalRunID {
		t.Errorf("run ID changed: got %q, want %q", finalRun.ID, originalRunID)
	}
}

// TestResetContext_MessagesCleared verifies that after the reset, the messages
// array is fresh (only system prompt + opening message injected by runner).
func TestResetContext_MessagesCleared(t *testing.T) {
	t.Parallel()

	// Use a blocking provider so we can inspect messages mid-run.
	blockAfterReset := make(chan struct{})
	releaseCh := make(chan struct{})

	mu := sync.Mutex{}
	callCount := 0

	provider := &blockingResetProvider{
		results: []CompletionResult{
			{
				ToolCalls: []ToolCall{{
					ID:        "call-reset-1",
					Name:      "reset_context",
					Arguments: `{"persist":{"task":"check messages cleared"}}`,
				}},
			},
			{Content: "done"},
		},
		onCallN: 1, // block on the second call (after reset)
		blockFn: func() {
			close(blockAfterReset)
			<-releaseCh
		},
		mu:        &mu,
		callCount: &callCount,
	}

	reg := makeResetContextRegistry()
	runner := NewRunner(provider, reg, RunnerConfig{
		DefaultModel:        "test",
		DefaultSystemPrompt: "system prompt text",
		MaxSteps:            10,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "start"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Wait until after the reset (the second provider call is about to be made).
	select {
	case <-blockAfterReset:
	case <-time.After(4 * time.Second):
		t.Fatal("timeout waiting for reset to complete")
	}

	// At this point, the runner has cleared and re-injected messages.
	msgs := runner.GetRunMessages(run.ID)

	// After reset, the runner should have re-injected the opening message.
	// There should be NO tool messages from before the reset.
	foundOpeningMsg := false
	for _, m := range msgs {
		if m.Role == "user" && strings.Contains(m.Content, "Context Reset") {
			foundOpeningMsg = true
		}
		// There should be no tool messages from before the reset.
		if m.Role == "tool" {
			t.Errorf("unexpected tool message after reset: %v", m)
		}
	}
	if !foundOpeningMsg {
		t.Errorf("expected opening user message with 'Context Reset' after reset, got messages: %+v", msgs)
	}

	close(releaseCh)
	_, err = collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}
}

// blockingResetProvider is a provider that blocks on the Nth call.
type blockingResetProvider struct {
	results   []CompletionResult
	onCallN   int
	blockFn   func()
	mu        *sync.Mutex
	callCount *int
}

func (p *blockingResetProvider) Complete(_ context.Context, _ CompletionRequest) (CompletionResult, error) {
	p.mu.Lock()
	n := *p.callCount
	*p.callCount++
	p.mu.Unlock()

	if n == p.onCallN {
		p.blockFn()
	}
	if n < len(p.results) {
		return p.results[n], nil
	}
	return CompletionResult{Content: "done"}, nil
}

// TestResetContext_ChainedResets verifies that multiple resets in a single run
// work correctly: each reset clears context and increments the segment index.
func TestResetContext_ChainedResets(t *testing.T) {
	t.Parallel()

	provider := buildDoubleResetProvider()
	reg := makeResetContextRegistry()

	runner := NewRunner(provider, reg, RunnerConfig{
		DefaultModel:        "test",
		DefaultSystemPrompt: "system",
		MaxSteps:            20,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "start"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	// Expect 2 context.reset events.
	resetEventCount := 0
	maxResetIndex := -1
	for _, ev := range events {
		if ev.Type == EventContextReset {
			resetEventCount++
			switch idx := ev.Payload["reset_index"].(type) {
			case int:
				if idx > maxResetIndex {
					maxResetIndex = idx
				}
			case float64:
				if int(idx) > maxResetIndex {
					maxResetIndex = int(idx)
				}
			}
		}
	}
	if resetEventCount != 2 {
		t.Errorf("expected 2 context.reset events, got %d", resetEventCount)
	}
	// First reset has index 0, second has index 1.
	if maxResetIndex != 1 {
		t.Errorf("expected max reset_index=1, got %d", maxResetIndex)
	}

	// Run should complete successfully.
	finalRun, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if finalRun.Status != RunStatusCompleted {
		t.Errorf("expected completed status, got %q", finalRun.Status)
	}
}

// TestResetContext_DBRecorded verifies that context resets are recorded in the
// ContextResetStore if one is configured.
func TestResetContext_DBRecorded(t *testing.T) {
	t.Parallel()

	provider := buildResetProvider(`{"task":"db record test","step":"write jwt.go"}`)
	reg := makeResetContextRegistry()

	store := &captureContextResetStore{}

	runner := NewRunner(provider, reg, RunnerConfig{
		DefaultModel:        "test",
		DefaultSystemPrompt: "system",
		MaxSteps:            10,
		ContextResetStore:   store,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "start"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	_, err = collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	store.mu.Lock()
	resets := append([]contextResetRecord(nil), store.resets...)
	store.mu.Unlock()

	if len(resets) != 1 {
		t.Fatalf("expected 1 recorded reset, got %d", len(resets))
	}
	if resets[0].runID != run.ID {
		t.Errorf("reset runID = %q, want %q", resets[0].runID, run.ID)
	}
	if resets[0].resetIndex != 0 {
		t.Errorf("reset_index = %d, want 0", resets[0].resetIndex)
	}
	if resets[0].atStep < 1 {
		t.Errorf("at_step = %d, want >= 1", resets[0].atStep)
	}
	// Persist should contain "task" field.
	var payload map[string]any
	if err := json.Unmarshal(resets[0].persist, &payload); err != nil {
		t.Fatalf("persist JSON invalid: %v", err)
	}
	if payload["task"] != "db record test" {
		t.Errorf("persist.task = %v, want 'db record test'", payload["task"])
	}
}

// TestResetContext_ObservationalMemoryWritten verifies that the persist payload
// is written to observational memory when a memory manager is configured.
func TestResetContext_ObservationalMemoryWritten(t *testing.T) {
	t.Parallel()

	provider := buildResetProvider(`{"task":"memory test","key_facts":["fact1"]}`)
	reg := makeResetContextRegistry()

	mem := &recordingMemoryManager{}

	runner := NewRunner(provider, reg, RunnerConfig{
		DefaultModel:        "test",
		DefaultSystemPrompt: "system",
		MaxSteps:            10,
		MemoryManager:       mem,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "start"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	_, err = collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	_ = run // suppress unused

	// Check that the memory manager received an observe call that contains the
	// context_reset marker. The runner should write the persist payload to memory.
	mem.mu.Lock()
	observations := append([]string(nil), mem.contents...)
	mem.mu.Unlock()

	foundContextReset := false
	for _, obs := range observations {
		if strings.Contains(obs, "context_reset") || strings.Contains(obs, "memory test") {
			foundContextReset = true
			break
		}
	}
	if !foundContextReset {
		t.Errorf("expected context_reset observation in memory, got: %v", observations)
	}
}

// recordingMemoryManager records content passed to Observe.
// It embeds memoryStub for the methods we don't care about.
type recordingMemoryManager struct {
	mu       sync.Mutex
	contents []string
	memoryStub
}

func (m *recordingMemoryManager) Observe(_ context.Context, req om.ObserveRequest) (om.ObserveResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, msg := range req.Messages {
		m.contents = append(m.contents, msg.Content)
	}
	return om.ObserveResult{}, nil
}

// TestResetContext_SegmentIndexIncrements verifies that the resetIndex on
// runState increments each time a reset_context call is made.
func TestResetContext_SegmentIndexIncrements(t *testing.T) {
	t.Parallel()

	provider := buildDoubleResetProvider()
	reg := makeResetContextRegistry()

	store := &captureContextResetStore{}

	runner := NewRunner(provider, reg, RunnerConfig{
		DefaultModel:        "test",
		DefaultSystemPrompt: "system",
		MaxSteps:            20,
		ContextResetStore:   store,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "start"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collectRunEvents: %v", err)
	}

	_ = events // already checked in TestResetContext_ChainedResets

	store.mu.Lock()
	resets := append([]contextResetRecord(nil), store.resets...)
	store.mu.Unlock()

	if len(resets) != 2 {
		t.Fatalf("expected 2 recorded resets, got %d", len(resets))
	}

	// resetIndex should be 0 for first reset, 1 for second.
	if resets[0].resetIndex != 0 {
		t.Errorf("first reset_index = %d, want 0", resets[0].resetIndex)
	}
	if resets[1].resetIndex != 1 {
		t.Errorf("second reset_index = %d, want 1", resets[1].resetIndex)
	}

	// The run ID should match for both.
	if resets[0].runID != run.ID {
		t.Errorf("first reset runID = %q, want %q", resets[0].runID, run.ID)
	}
	if resets[1].runID != run.ID {
		t.Errorf("second reset runID = %q, want %q", resets[1].runID, run.ID)
	}
}
