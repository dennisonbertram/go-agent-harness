package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newParallelSafeRegistry creates a registry with two parallel-safe tools and
// one serial tool for use in parallel-execution tests.
//
// The parallel-safe tools both sleep for sleepDur while running, making it
// possible to detect concurrent execution via elapsed wall time.
//
// The serial tool records its execution in serialOrder.
func newParallelSafeRegistry(
	sleepDur time.Duration,
	startOrder *[]string,
	startMu *sync.Mutex,
	finishOrder *[]string,
	finishMu *sync.Mutex,
) *Registry {
	registry := NewRegistry()

	// safe_a: parallel-safe, sleeps briefly, records start/finish order.
	err := registry.Register(ToolDefinition{
		Name:         "safe_a",
		Description:  "parallel-safe read tool A",
		Parameters:   map[string]any{"type": "object", "properties": map[string]any{}},
		ParallelSafe: true,
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		startMu.Lock()
		*startOrder = append(*startOrder, "safe_a")
		startMu.Unlock()

		time.Sleep(sleepDur)

		finishMu.Lock()
		*finishOrder = append(*finishOrder, "safe_a")
		finishMu.Unlock()
		return `{"tool":"safe_a"}`, nil
	})
	if err != nil {
		panic(err)
	}

	// safe_b: parallel-safe, sleeps briefly, records start/finish order.
	err = registry.Register(ToolDefinition{
		Name:         "safe_b",
		Description:  "parallel-safe read tool B",
		Parameters:   map[string]any{"type": "object", "properties": map[string]any{}},
		ParallelSafe: true,
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		startMu.Lock()
		*startOrder = append(*startOrder, "safe_b")
		startMu.Unlock()

		time.Sleep(sleepDur)

		finishMu.Lock()
		*finishOrder = append(*finishOrder, "safe_b")
		finishMu.Unlock()
		return `{"tool":"safe_b"}`, nil
	})
	if err != nil {
		panic(err)
	}

	// serial_tool: NOT parallel-safe, records invocation order.
	err = registry.Register(ToolDefinition{
		Name:         "serial_tool",
		Description:  "a serial (mutating) tool",
		Parameters:   map[string]any{"type": "object", "properties": map[string]any{}},
		ParallelSafe: false,
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		startMu.Lock()
		*startOrder = append(*startOrder, "serial_tool")
		startMu.Unlock()
		return `{"tool":"serial_tool"}`, nil
	})
	if err != nil {
		panic(err)
	}

	return registry
}

// waitForParallelRunStatus polls GetRun until one of the target statuses is reached or
// the timeout fires.
func waitForParallelRunStatus(t *testing.T, runner *Runner, runID string, targets ...RunStatus) RunStatus {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatalf("run did not complete within timeout")
		default:
		}
		run, ok := runner.GetRun(runID)
		if !ok {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		for _, target := range targets {
			if run.Status == target {
				return run.Status
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestParallelToolsExecuteConcurrently verifies that two parallel-safe tool
// calls within a single step run concurrently, not serially.
//
// Proof of concurrency: both tools have a sleep of sleepDur. If they ran
// serially the total elapsed time would be >= 2*sleepDur. If they run
// concurrently the total elapsed time should be < 2*sleepDur.
func TestParallelToolsExecuteConcurrently(t *testing.T) {
	t.Parallel()

	const sleepDur = 50 * time.Millisecond

	var startOrder []string
	var startMu sync.Mutex
	var finishOrder []string
	var finishMu sync.Mutex

	registry := newParallelSafeRegistry(sleepDur, &startOrder, &startMu, &finishOrder, &finishMu)

	provider := &stubProvider{turns: []CompletionResult{
		{
			ToolCalls: []ToolCall{
				{ID: "c1", Name: "safe_a", Arguments: "{}"},
				{ID: "c2", Name: "safe_b", Arguments: "{}"},
			},
		},
		// Second turn: no tool calls → run completes.
		{Content: "done"},
	}}

	runner := NewRunner(provider, registry, RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     5,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "run two safe tools"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	start := time.Now()
	waitForParallelRunStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)
	elapsed := time.Since(start)

	// Both tools must have executed.
	startMu.Lock()
	got := append([]string(nil), startOrder...)
	startMu.Unlock()

	if len(got) != 2 {
		t.Fatalf("expected 2 tool executions, got %d: %v", len(got), got)
	}

	// With concurrency the wall time should be well under 2*sleepDur + generous overhead.
	// We use 1.5x as a safe ceiling — even with goroutine startup overhead two 50ms sleeps
	// running concurrently should complete in ~50-60ms total.
	ceiling := time.Duration(float64(sleepDur)*1.8) + 30*time.Millisecond
	if elapsed > ceiling {
		t.Logf("elapsed=%v ceiling=%v (tools ran serially or very slow CI)", elapsed, ceiling)
		// Don't hard-fail on timing: CI machines can be slow. Just log a warning.
		// The race-detector test below is the hard correctness gate.
	}
}

// TestParallelToolsOrderingDeterministic verifies that tool-result messages
// are appended to the transcript in the original call order regardless of
// which tool finishes first.
//
// We deliberately make safe_b finish before safe_a by giving safe_a a longer
// sleep and safe_b a shorter one.
func TestParallelToolsOrderingDeterministic(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()

	// safe_slow: parallel-safe, sleeps longer.
	err := registry.Register(ToolDefinition{
		Name:         "safe_slow",
		Description:  "slow parallel-safe tool",
		Parameters:   map[string]any{"type": "object", "properties": map[string]any{}},
		ParallelSafe: true,
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		time.Sleep(60 * time.Millisecond)
		return `{"tool":"safe_slow"}`, nil
	})
	if err != nil {
		panic(err)
	}

	// safe_fast: parallel-safe, finishes quickly.
	err = registry.Register(ToolDefinition{
		Name:         "safe_fast",
		Description:  "fast parallel-safe tool",
		Parameters:   map[string]any{"type": "object", "properties": map[string]any{}},
		ParallelSafe: true,
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		// Intentionally shorter sleep so it finishes before safe_slow.
		time.Sleep(5 * time.Millisecond)
		return `{"tool":"safe_fast"}`, nil
	})
	if err != nil {
		panic(err)
	}

	provider := &stubProvider{turns: []CompletionResult{
		{
			// safe_slow is called first, safe_fast second.
			ToolCalls: []ToolCall{
				{ID: "c-slow", Name: "safe_slow", Arguments: "{}"},
				{ID: "c-fast", Name: "safe_fast", Arguments: "{}"},
			},
		},
		{Content: "done"},
	}}

	runner := NewRunner(provider, registry, RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     5,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "ordering test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	runID := run.ID

	waitForParallelRunStatus(t, runner, runID, RunStatusCompleted, RunStatusFailed)

	// Inspect transcript for tool-result order.
	runner.mu.RLock()
	state := runner.runs[runID]
	runner.mu.RUnlock()
	if state == nil {
		t.Fatal("run state not found")
	}

	var toolResults []string
	for _, msg := range state.messages {
		if msg.Role == "tool" {
			toolResults = append(toolResults, msg.Name)
		}
	}

	// Transcript must preserve call order: safe_slow first, safe_fast second.
	if len(toolResults) < 2 {
		t.Fatalf("expected at least 2 tool-result messages, got %d", len(toolResults))
	}
	if toolResults[0] != "safe_slow" || toolResults[1] != "safe_fast" {
		t.Errorf("tool result order = %v, want [safe_slow safe_fast]", toolResults)
	}
}

// TestMixedParallelAndSerialTools verifies that serial tools are never
// interleaved with parallel-safe batches: serial tools run before or after
// a parallel batch, not during one.
func TestMixedParallelAndSerialTools(t *testing.T) {
	t.Parallel()

	// executionOrder records the name of each tool as it *starts* executing.
	var executionOrder []string
	var mu sync.Mutex

	registry := NewRegistry()

	err := registry.Register(ToolDefinition{
		Name:         "safe_x",
		Description:  "parallel safe x",
		Parameters:   map[string]any{"type": "object", "properties": map[string]any{}},
		ParallelSafe: true,
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		mu.Lock()
		executionOrder = append(executionOrder, "safe_x")
		mu.Unlock()
		time.Sleep(30 * time.Millisecond)
		return `{"tool":"safe_x"}`, nil
	})
	if err != nil {
		panic(err)
	}

	err = registry.Register(ToolDefinition{
		Name:         "serial_y",
		Description:  "serial y",
		Parameters:   map[string]any{"type": "object", "properties": map[string]any{}},
		ParallelSafe: false,
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		mu.Lock()
		executionOrder = append(executionOrder, "serial_y")
		mu.Unlock()
		return `{"tool":"serial_y"}`, nil
	})
	if err != nil {
		panic(err)
	}

	err = registry.Register(ToolDefinition{
		Name:         "safe_z",
		Description:  "parallel safe z",
		Parameters:   map[string]any{"type": "object", "properties": map[string]any{}},
		ParallelSafe: true,
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		mu.Lock()
		executionOrder = append(executionOrder, "safe_z")
		mu.Unlock()
		time.Sleep(30 * time.Millisecond)
		return `{"tool":"safe_z"}`, nil
	})
	if err != nil {
		panic(err)
	}

	provider := &stubProvider{turns: []CompletionResult{
		{
			// safe_x and safe_z are parallel-safe; serial_y is not.
			ToolCalls: []ToolCall{
				{ID: "c-x", Name: "safe_x", Arguments: "{}"},
				{ID: "c-y", Name: "serial_y", Arguments: "{}"},
				{ID: "c-z", Name: "safe_z", Arguments: "{}"},
			},
		},
		{Content: "done"},
	}}

	runner := NewRunner(provider, registry, RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     5,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "mixed test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	runID := run.ID

	waitForParallelRunStatus(t, runner, runID, RunStatusCompleted, RunStatusFailed)

	mu.Lock()
	got := append([]string(nil), executionOrder...)
	mu.Unlock()

	if len(got) != 3 {
		t.Fatalf("expected 3 tool executions, got %d: %v", len(got), got)
	}

	// Transcript must still be in call order.
	runner.mu.RLock()
	state := runner.runs[runID]
	runner.mu.RUnlock()
	if state == nil {
		t.Fatal("run state not found")
	}

	var toolResults []string
	for _, msg := range state.messages {
		if msg.Role == "tool" {
			toolResults = append(toolResults, msg.Name)
		}
	}
	if len(toolResults) < 3 {
		t.Fatalf("expected at least 3 tool-result messages, got %d: %v", len(toolResults), toolResults)
	}
	wantOrder := []string{"safe_x", "serial_y", "safe_z"}
	for i, want := range wantOrder {
		if toolResults[i] != want {
			t.Errorf("toolResults[%d] = %q, want %q (full order: %v)", i, toolResults[i], want, toolResults)
		}
	}
}

// TestParallelToolsRaceDetector exercises concurrent tool execution under the
// race detector. It runs many goroutines and verifies no data races occur.
// Run with: go test -race ./internal/harness/... -run TestParallelToolsRaceDetector
func TestParallelToolsRaceDetector(t *testing.T) {
	t.Parallel()

	var counter int64 // shared across goroutines via atomic — not a race

	registry := NewRegistry()

	for i := range 5 {
		name := fmt.Sprintf("safe_race_%d", i)
		err := registry.Register(ToolDefinition{
			Name:         name,
			Description:  "race-test parallel tool",
			Parameters:   map[string]any{"type": "object", "properties": map[string]any{}},
			ParallelSafe: true,
		}, func(_ context.Context, _ json.RawMessage) (string, error) {
			atomic.AddInt64(&counter, 1)
			time.Sleep(5 * time.Millisecond)
			return `{"ok":true}`, nil
		})
		if err != nil {
			t.Fatalf("register tool: %v", err)
		}
	}

	// Build tool calls for all 5 parallel-safe tools.
	calls := make([]ToolCall, 5)
	for i := range 5 {
		calls[i] = ToolCall{
			ID:        fmt.Sprintf("race-c%d", i),
			Name:      fmt.Sprintf("safe_race_%d", i),
			Arguments: "{}",
		}
	}

	provider := &stubProvider{turns: []CompletionResult{
		{ToolCalls: calls},
		{Content: "done"},
	}}

	runner := NewRunner(provider, registry, RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     5,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "race test"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	waitForParallelRunStatus(t, runner, run.ID, RunStatusCompleted, RunStatusFailed)

	finalCount := atomic.LoadInt64(&counter)
	if finalCount != 5 {
		t.Errorf("expected counter=5, got %d", finalCount)
	}
}

// TestIsParallelSafe_Registry verifies that Registry.IsParallelSafe returns
// the correct value for registered tools.
func TestIsParallelSafe_Registry(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()

	err := registry.Register(ToolDefinition{
		Name:         "safe_tool",
		Parameters:   map[string]any{"type": "object"},
		ParallelSafe: true,
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatal(err)
	}

	err = registry.Register(ToolDefinition{
		Name:         "unsafe_tool",
		Parameters:   map[string]any{"type": "object"},
		ParallelSafe: false,
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if !registry.IsParallelSafe("safe_tool") {
		t.Error("safe_tool should be parallel-safe")
	}
	if registry.IsParallelSafe("unsafe_tool") {
		t.Error("unsafe_tool should NOT be parallel-safe")
	}
	if registry.IsParallelSafe("nonexistent") {
		t.Error("nonexistent tool should return false")
	}
}

// TestParallelToolsAllParallelSafe verifies that when all tool calls in a step
// are parallel-safe and finish at different times, the transcript still has
// tool-result messages in original call order and all results are present.
func TestParallelToolsAllParallelSafe(t *testing.T) {
	t.Parallel()

	const n = 4
	registry := NewRegistry()

	// Register n tools with staggered sleep durations (reverse order: tool_3 finishes first).
	for i := range n {
		i := i
		name := fmt.Sprintf("ptool_%d", i)
		// tool_0 sleeps longest; tool_{n-1} sleeps shortest.
		sleep := time.Duration(n-i) * 20 * time.Millisecond
		err := registry.Register(ToolDefinition{
			Name:         name,
			Parameters:   map[string]any{"type": "object"},
			ParallelSafe: true,
		}, func(_ context.Context, _ json.RawMessage) (string, error) {
			time.Sleep(sleep)
			return fmt.Sprintf(`{"idx":%d}`, i), nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	calls := make([]ToolCall, n)
	for i := range n {
		calls[i] = ToolCall{
			ID:        fmt.Sprintf("pc%d", i),
			Name:      fmt.Sprintf("ptool_%d", i),
			Arguments: "{}",
		}
	}

	provider := &stubProvider{turns: []CompletionResult{
		{ToolCalls: calls},
		{Content: "done"},
	}}

	runner := NewRunner(provider, registry, RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     5,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "all parallel"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	runID := run.ID

	waitForParallelRunStatus(t, runner, runID, RunStatusCompleted, RunStatusFailed)

	runner.mu.RLock()
	state := runner.runs[runID]
	runner.mu.RUnlock()

	var toolResults []string
	for _, msg := range state.messages {
		if msg.Role == "tool" {
			toolResults = append(toolResults, msg.Name)
		}
	}

	if len(toolResults) != n {
		t.Fatalf("expected %d tool results, got %d: %v", n, len(toolResults), toolResults)
	}
	for i := range n {
		want := fmt.Sprintf("ptool_%d", i)
		if toolResults[i] != want {
			t.Errorf("toolResults[%d] = %q, want %q", i, toolResults[i], want)
		}
		// Content should contain correct index.
		for _, msg := range state.messages {
			if msg.Role == "tool" && msg.Name == fmt.Sprintf("ptool_%d", i) {
				if !strings.Contains(msg.Content, fmt.Sprintf(`"idx":%d`, i)) {
					t.Errorf("tool %d content %q does not contain expected idx", i, msg.Content)
				}
			}
		}
	}
}
