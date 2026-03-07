package harness

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	htools "go-agent-harness/internal/harness/tools"
	om "go-agent-harness/internal/observationalmemory"
)

type stubProvider struct {
	turns []CompletionResult
	calls int
}

func (s *stubProvider) Complete(_ context.Context, req CompletionRequest) (CompletionResult, error) {
	if s.calls >= len(s.turns) {
		return CompletionResult{}, nil
	}
	turn := s.turns[s.calls]
	s.calls++
	if req.Stream != nil {
		for _, delta := range turn.Deltas {
			req.Stream(delta)
		}
	}
	return turn, nil
}

type errorProvider struct {
	err error
}

func (e *errorProvider) Complete(_ context.Context, _ CompletionRequest) (CompletionResult, error) {
	return CompletionResult{}, e.err
}

type capturingProvider struct {
	turns []CompletionResult
	calls []CompletionRequest
}

func (c *capturingProvider) Complete(_ context.Context, req CompletionRequest) (CompletionResult, error) {
	c.calls = append(c.calls, req)
	if len(c.turns) == 0 {
		return CompletionResult{}, nil
	}
	turn := c.turns[0]
	c.turns = c.turns[1:]
	return turn, nil
}

type memoryStub struct {
	status  om.Status
	snippet string
}

func (m *memoryStub) Close() error                                           { return nil }
func (m *memoryStub) Mode() om.Mode                                          { return om.ModeLocalCoordinator }
func (m *memoryStub) Status(context.Context, om.ScopeKey) (om.Status, error) { return m.status, nil }
func (m *memoryStub) SetEnabled(context.Context, om.ScopeKey, bool, *om.Config, string, string) (om.Status, error) {
	return m.status, nil
}
func (m *memoryStub) Observe(_ context.Context, req om.ObserveRequest) (om.ObserveResult, error) {
	m.status.LastObservedMessageIndex = int64(len(req.Messages) - 1)
	return om.ObserveResult{Status: m.status, Observed: true}, nil
}
func (m *memoryStub) Snippet(context.Context, om.ScopeKey) (string, om.Status, error) {
	return m.snippet, m.status, nil
}
func (m *memoryStub) ReflectNow(context.Context, om.ScopeKey, string, string) (om.Status, error) {
	return m.status, nil
}
func (m *memoryStub) Export(context.Context, om.ScopeKey, string) (om.ExportResult, error) {
	return om.ExportResult{Status: m.status}, nil
}

func TestRunnerExecutesToolCallsAndPublishesEvents(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	err := registry.Register(ToolDefinition{
		Name:        "echo_json",
		Description: "echoes payload",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
			},
			"required": []string{"message"},
		},
	}, func(_ context.Context, raw json.RawMessage) (string, error) {
		var payload struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			return "", err
		}
		return `{"echo":"` + payload.Message + `"}`, nil
	})
	if err != nil {
		t.Fatalf("register tool: %v", err)
	}

	provider := &stubProvider{turns: []CompletionResult{
		{
			ToolCalls: []ToolCall{{
				ID:        "call-1",
				Name:      "echo_json",
				Arguments: `{"message":"hello"}`,
			}},
		},
		{Content: "All done"},
	}}

	runner := NewRunner(provider, registry, RunnerConfig{
		DefaultModel:        "gpt-4.1-mini",
		DefaultSystemPrompt: "You are a coding harness.",
		MaxSteps:            4,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "Say hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}

	if provider.calls != 2 {
		t.Fatalf("expected provider to be called twice, got %d", provider.calls)
	}

	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatalf("expected run state")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed status, got %q", state.Status)
	}
	if state.Output != "All done" {
		t.Fatalf("unexpected run output: %q", state.Output)
	}

	requireEventOrder(t, events,
		"run.started",
		"llm.turn.completed",
		"tool.call.started",
		"tool.call.completed",
		"assistant.message",
		"run.completed",
	)
}

func TestRunnerInjectsMemorySnippetAndEmitsMemoryEvents(t *testing.T) {
	t.Parallel()

	provider := &capturingProvider{turns: []CompletionResult{{Content: "done"}}}
	mem := &memoryStub{
		status: om.Status{
			Mode:                     om.ModeLocalCoordinator,
			MemoryID:                 "default|conv|agent",
			Scope:                    om.ScopeKey{TenantID: "default", ConversationID: "conv", AgentID: "agent"},
			Enabled:                  true,
			LastObservedMessageIndex: -1,
			UpdatedAt:                time.Now().UTC(),
		},
		snippet: "<observational-memory>test</observational-memory>",
	}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel:   "gpt-4.1-mini",
		MaxSteps:       2,
		MemoryManager:  mem,
		AskUserTimeout: time.Second,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}
	if len(provider.calls) == 0 {
		t.Fatalf("expected provider call")
	}
	if len(provider.calls[0].Messages) < 1 || provider.calls[0].Messages[0].Content != "<observational-memory>test</observational-memory>" {
		t.Fatalf("expected injected memory snippet in first request: %+v", provider.calls[0].Messages)
	}
	requireEventOrder(t, events, "memory.observe.started", "memory.observe.completed", "run.completed")
}

func TestRunnerFailsWhenProviderErrors(t *testing.T) {
	t.Parallel()

	runner := NewRunner(&errorProvider{err: errors.New("provider exploded")}, NewRegistry(), RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "Fail now"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}
	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatalf("expected run state")
	}
	if state.Status != RunStatusFailed {
		t.Fatalf("expected failed status, got %q", state.Status)
	}
	if state.Error == "" {
		t.Fatalf("expected run error")
	}

	requireEventOrder(t, events, "run.started", "llm.turn.requested", "run.failed")
}

func TestRunnerEmitsAssistantMessageDeltaEvents(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{turns: []CompletionResult{{
		Content: "Hello",
		Deltas: []CompletionDelta{
			{Content: "Hel"},
			{Content: "lo"},
		},
	}}}

	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     2,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "Say hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}

	requireEventOrder(t, events,
		"run.started",
		"assistant.message.delta",
		"llm.turn.completed",
		"assistant.message",
		"run.completed",
	)

	var got []string
	for _, ev := range events {
		if ev.Type != "assistant.message.delta" {
			continue
		}
		content, _ := ev.Payload["content"].(string)
		got = append(got, content)
	}
	if !slices.Equal(got, []string{"Hel", "lo"}) {
		t.Fatalf("unexpected delta payloads: %+v", got)
	}
}

func TestRunnerEmitsToolCallDeltaEventsBeforeExecution(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name:        "echo_json",
		Description: "echoes payload",
		Parameters:  map[string]any{"type": "object"},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return `{}`, nil
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	provider := &stubProvider{turns: []CompletionResult{
		{
			ToolCalls: []ToolCall{{
				ID:        "call-1",
				Name:      "echo_json",
				Arguments: `{"message":"hello"}`,
			}},
			Deltas: []CompletionDelta{
				{ToolCall: ToolCallDelta{Index: 0, ID: "call-1", Name: "echo_json"}},
				{ToolCall: ToolCallDelta{Index: 0, Arguments: `{"message":"`}},
				{ToolCall: ToolCallDelta{Index: 0, Arguments: `hello"}`}},
			},
		},
		{Content: "done"},
	}}

	runner := NewRunner(provider, registry, RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     4,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}

	requireEventOrder(t, events,
		"tool.call.delta",
		"llm.turn.completed",
		"tool.call.started",
		"tool.call.completed",
	)

	var argsParts []string
	for _, ev := range events {
		if ev.Type != "tool.call.delta" {
			continue
		}
		arguments, _ := ev.Payload["arguments"].(string)
		if arguments != "" {
			argsParts = append(argsParts, arguments)
		}
	}
	if !slices.Equal(argsParts, []string{`{"message":"`, `hello"}`}) {
		t.Fatalf("unexpected tool delta payloads: %+v", argsParts)
	}
}

func TestFailRunWithNilErrorUsesDefaultMessage(t *testing.T) {
	t.Parallel()

	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{})
	now := time.Now().UTC()
	runner.mu.Lock()
	runner.runs["run_manual"] = &runState{
		run: Run{
			ID:        "run_manual",
			Prompt:    "x",
			Model:     "gpt-4.1-mini",
			Status:    RunStatusRunning,
			CreatedAt: now,
			UpdatedAt: now,
		},
		events:      make([]Event, 0, 4),
		subscribers: make(map[chan Event]struct{}),
	}
	runner.mu.Unlock()

	runner.failRun("run_manual", nil)

	state, ok := runner.GetRun("run_manual")
	if !ok {
		t.Fatalf("expected run state")
	}
	if state.Status != RunStatusFailed {
		t.Fatalf("expected failed status, got %q", state.Status)
	}
	if state.Error != "run failed" {
		t.Fatalf("unexpected error: %q", state.Error)
	}
}

func TestMustJSONFallback(t *testing.T) {
	t.Parallel()

	got := mustJSON(map[string]any{"bad": make(chan int)})
	if got != `{"error":"failed to marshal tool error"}` {
		t.Fatalf("unexpected fallback json: %s", got)
	}
}

func TestRunnerAskUserQuestionWaitsAndResumes(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{turns: []CompletionResult{
		{
			ToolCalls: []ToolCall{{
				ID:        "call_ask",
				Name:      htools.AskUserQuestionToolName,
				Arguments: `{"questions":[{"question":"Where next?","header":"Route","options":[{"label":"Docs","description":"Read docs"},{"label":"Code","description":"Read code"}],"multiSelect":false}]}`,
			}},
		},
		{Content: "All done"},
	}}

	broker := NewInMemoryAskUserQuestionBroker(time.Now)
	runner := NewRunner(provider, NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode:   ToolApprovalModeFullAuto,
		AskUserBroker:  broker,
		AskUserTimeout: 2 * time.Second,
	}), RunnerConfig{
		DefaultModel:   "gpt-5-nano",
		MaxSteps:       4,
		AskUserBroker:  broker,
		AskUserTimeout: 2 * time.Second,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "Need clarification"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	deadline := time.Now().Add(1500 * time.Millisecond)
	for {
		pending, err := runner.PendingInput(run.ID)
		if err == nil {
			if pending.CallID != "call_ask" {
				t.Fatalf("unexpected call id: %q", pending.CallID)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for pending input: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatalf("expected run state")
	}
	if state.Status != RunStatusWaitingForUser {
		t.Fatalf("expected waiting_for_user status, got %q", state.Status)
	}

	if err := runner.SubmitInput(run.ID, map[string]string{"Where next?": "Docs"}); err != nil {
		t.Fatalf("submit input: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}

	state, ok = runner.GetRun(run.ID)
	if !ok {
		t.Fatalf("expected run state")
	}
	if state.Status != RunStatusCompleted {
		t.Fatalf("expected completed status, got %q", state.Status)
	}
	if provider.calls != 2 {
		t.Fatalf("expected provider called twice, got %d", provider.calls)
	}

	requireEventOrder(t, events,
		"run.started",
		"tool.call.started",
		"run.waiting_for_user",
		"tool.call.completed",
		"run.resumed",
		"run.completed",
	)
}

func TestRunnerAskUserQuestionTimeoutFailsRun(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{turns: []CompletionResult{
		{
			ToolCalls: []ToolCall{{
				ID:        "call_ask_timeout",
				Name:      htools.AskUserQuestionToolName,
				Arguments: `{"questions":[{"question":"Where next?","header":"Route","options":[{"label":"Docs","description":"Read docs"},{"label":"Code","description":"Read code"}],"multiSelect":false}]}`,
			}},
		},
		{Content: "should not happen"},
	}}

	broker := NewInMemoryAskUserQuestionBroker(time.Now)
	runner := NewRunner(provider, NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode:   ToolApprovalModeFullAuto,
		AskUserBroker:  broker,
		AskUserTimeout: 20 * time.Millisecond,
	}), RunnerConfig{
		DefaultModel:   "gpt-5-nano",
		MaxSteps:       4,
		AskUserBroker:  broker,
		AskUserTimeout: 20 * time.Millisecond,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "Need clarification"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}

	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatalf("expected run state")
	}
	if state.Status != RunStatusFailed {
		t.Fatalf("expected failed status, got %q", state.Status)
	}
	if provider.calls != 1 {
		t.Fatalf("expected provider called once, got %d", provider.calls)
	}
	if !strings.Contains(state.Error, "timed out") {
		t.Fatalf("expected timeout error, got %q", state.Error)
	}

	requireEventOrder(t, events, "run.waiting_for_user", "run.failed")
}

func TestRunnerEmitsUsageDeltaAndPersistsTotals(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name:        "echo_json",
		Description: "echoes payload",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return `{}`, nil
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	provider := &stubProvider{turns: []CompletionResult{
		{
			ToolCalls: []ToolCall{{ID: "call-1", Name: "echo_json", Arguments: `{}`}},
			Usage: &CompletionUsage{
				PromptTokens:     10,
				CompletionTokens: 4,
				TotalTokens:      14,
			},
			CostUSD:     floatPtr(0.001),
			UsageStatus: UsageStatusProviderReported,
			CostStatus:  CostStatusAvailable,
			Cost: &CompletionCost{
				TotalUSD: 0.001,
			},
		},
		{
			Content: "done",
			Usage: &CompletionUsage{
				PromptTokens:     8,
				CompletionTokens: 3,
				TotalTokens:      11,
			},
			CostUSD:     floatPtr(0.002),
			UsageStatus: UsageStatusProviderReported,
			CostStatus:  CostStatusAvailable,
			Cost: &CompletionCost{
				TotalUSD: 0.002,
			},
		},
	}}

	runner := NewRunner(provider, registry, RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     4,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}

	usageDeltaCount := 0
	var completed Event
	for _, ev := range events {
		if ev.Type == "usage.delta" {
			usageDeltaCount++
		}
		if ev.Type == "run.completed" {
			completed = ev
		}
	}
	if usageDeltaCount != 2 {
		t.Fatalf("expected two usage.delta events, got %d", usageDeltaCount)
	}

	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatalf("expected run state")
	}
	if state.UsageTotals == nil || state.CostTotals == nil {
		t.Fatalf("expected usage and cost totals on run state")
	}
	if state.UsageTotals.PromptTokensTotal != 18 || state.UsageTotals.CompletionTokensTotal != 7 || state.UsageTotals.TotalTokens != 25 {
		t.Fatalf("unexpected run usage totals: %+v", state.UsageTotals)
	}
	if math.Abs(state.CostTotals.CostUSDTotal-0.003) > 1e-12 {
		t.Fatalf("unexpected run cost totals: %+v", state.CostTotals)
	}
	if state.CostTotals.CostStatus != CostStatusAvailable {
		t.Fatalf("unexpected run cost status: %q", state.CostTotals.CostStatus)
	}

	usageTotals, ok := completed.Payload["usage_totals"].(RunUsageTotals)
	if !ok {
		t.Fatalf("expected usage_totals in run.completed payload: %+v", completed.Payload)
	}
	if usageTotals.TotalTokens != 25 {
		t.Fatalf("unexpected completed usage totals: %+v", usageTotals)
	}
	costTotals, ok := completed.Payload["cost_totals"].(RunCostTotals)
	if !ok {
		t.Fatalf("expected cost_totals in run.completed payload: %+v", completed.Payload)
	}
	if math.Abs(costTotals.CostUSDTotal-0.003) > 1e-12 {
		t.Fatalf("unexpected completed cost totals: %+v", costTotals)
	}
}

func TestRunnerFailedRunIncludesPartialUsageTotals(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name:        "echo_json",
		Description: "echoes payload",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return `{}`, nil
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	provider := &flakyProvider{
		turns: []CompletionResult{
			{
				ToolCalls: []ToolCall{{ID: "call-1", Name: "echo_json", Arguments: `{}`}},
				Usage: &CompletionUsage{
					PromptTokens:     5,
					CompletionTokens: 2,
					TotalTokens:      7,
				},
				CostUSD:     floatPtr(0.0007),
				UsageStatus: UsageStatusProviderReported,
				CostStatus:  CostStatusAvailable,
				Cost: &CompletionCost{
					TotalUSD: 0.0007,
				},
			},
		},
		errAt: 1,
		err:   errors.New("provider exploded"),
	}

	runner := NewRunner(provider, registry, RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     4,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}

	var failed Event
	usageDeltaCount := 0
	for _, ev := range events {
		if ev.Type == "usage.delta" {
			usageDeltaCount++
		}
		if ev.Type == "run.failed" {
			failed = ev
		}
	}
	if usageDeltaCount != 1 {
		t.Fatalf("expected one usage.delta event, got %d", usageDeltaCount)
	}

	state, ok := runner.GetRun(run.ID)
	if !ok {
		t.Fatalf("expected run state")
	}
	if state.Status != RunStatusFailed {
		t.Fatalf("expected failed status, got %q", state.Status)
	}
	if state.UsageTotals == nil || state.UsageTotals.TotalTokens != 7 {
		t.Fatalf("unexpected run usage totals: %+v", state.UsageTotals)
	}
	if state.CostTotals == nil || math.Abs(state.CostTotals.CostUSDTotal-0.0007) > 1e-12 {
		t.Fatalf("unexpected run cost totals: %+v", state.CostTotals)
	}

	usageTotals, ok := failed.Payload["usage_totals"].(RunUsageTotals)
	if !ok {
		t.Fatalf("expected usage_totals in run.failed payload: %+v", failed.Payload)
	}
	if usageTotals.TotalTokens != 7 {
		t.Fatalf("unexpected failed usage totals payload: %+v", usageTotals)
	}
	costTotals, ok := failed.Payload["cost_totals"].(RunCostTotals)
	if !ok {
		t.Fatalf("expected cost_totals in run.failed payload: %+v", failed.Payload)
	}
	if math.Abs(costTotals.CostUSDTotal-0.0007) > 1e-12 {
		t.Fatalf("unexpected failed cost totals payload: %+v", costTotals)
	}
}

func collectRunEvents(t *testing.T, runner *Runner, runID string) ([]Event, error) {
	t.Helper()

	history, stream, cancel, err := runner.Subscribe(runID)
	if err != nil {
		return nil, err
	}
	defer cancel()

	events := append([]Event(nil), history...)
	if hasTerminalEvent(events) {
		return events, nil
	}

	timeout := time.After(4 * time.Second)
	for {
		select {
		case ev, ok := <-stream:
			if !ok {
				return events, nil
			}
			events = append(events, ev)
			if isTerminalEvent(ev.Type) {
				return events, nil
			}
		case <-timeout:
			return nil, context.DeadlineExceeded
		}
	}
}

func hasTerminalEvent(events []Event) bool {
	for _, ev := range events {
		if isTerminalEvent(ev.Type) {
			return true
		}
	}
	return false
}

func isTerminalEvent(eventType string) bool {
	return eventType == "run.completed" || eventType == "run.failed"
}

func requireEventOrder(t *testing.T, events []Event, expected ...string) {
	t.Helper()

	positions := make(map[string]int, len(expected))
	for i, ev := range events {
		if _, exists := positions[ev.Type]; !exists {
			positions[ev.Type] = i
		}
	}

	prev := -1
	for _, eventType := range expected {
		idx, ok := positions[eventType]
		if !ok {
			t.Fatalf("missing event %q in %+v", eventType, eventTypes(events))
		}
		if idx <= prev {
			t.Fatalf("event %q out of order in %+v", eventType, eventTypes(events))
		}
		prev = idx
	}
}

func eventTypes(events []Event) []string {
	result := make([]string, 0, len(events))
	for _, ev := range events {
		result = append(result, ev.Type)
	}
	return result
}

type flakyProvider struct {
	turns []CompletionResult
	errAt int
	err   error
	calls int
}

func (f *flakyProvider) Complete(_ context.Context, _ CompletionRequest) (CompletionResult, error) {
	if f.calls == f.errAt {
		f.calls++
		if f.err == nil {
			return CompletionResult{}, errors.New("provider error")
		}
		return CompletionResult{}, f.err
	}
	if f.calls >= len(f.turns) {
		f.calls++
		return CompletionResult{}, nil
	}
	out := f.turns[f.calls]
	f.calls++
	return out, nil
}

func floatPtr(v float64) *float64 {
	n := v
	return &n
}

func TestRunnerStoresConversationOnCompletion(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{turns: []CompletionResult{{Content: "done"}}}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     2,
	})

	run, err := runner.StartRun(RunRequest{
		Prompt:         "hello",
		ConversationID: "conv-1",
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	_, err = collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}

	msgs, ok := runner.ConversationMessages("conv-1")
	if !ok {
		t.Fatalf("expected conversation messages to be stored")
	}
	if len(msgs) < 2 {
		t.Fatalf("expected at least user + assistant messages, got %d", len(msgs))
	}

	hasUser := false
	hasAssistant := false
	for _, m := range msgs {
		if m.Role == "user" {
			hasUser = true
		}
		if m.Role == "assistant" {
			hasAssistant = true
		}
	}
	if !hasUser {
		t.Fatalf("expected user message in conversation history")
	}
	if !hasAssistant {
		t.Fatalf("expected assistant message in conversation history")
	}
}

func TestRunnerSecondRunGetsPriorMessages(t *testing.T) {
	t.Parallel()

	provider := &capturingProvider{turns: []CompletionResult{
		{Content: "first answer"},
		{Content: "second answer"},
	}}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     2,
	})

	// First run
	run1, err := runner.StartRun(RunRequest{
		Prompt:         "first question",
		ConversationID: "conv-multi",
	})
	if err != nil {
		t.Fatalf("start run 1: %v", err)
	}
	_, err = collectRunEvents(t, runner, run1.ID)
	if err != nil {
		t.Fatalf("collect events run 1: %v", err)
	}

	// Second run with same conversation ID
	run2, err := runner.StartRun(RunRequest{
		Prompt:         "follow up",
		ConversationID: "conv-multi",
	})
	if err != nil {
		t.Fatalf("start run 2: %v", err)
	}
	events2, err := collectRunEvents(t, runner, run2.ID)
	if err != nil {
		t.Fatalf("collect events run 2: %v", err)
	}

	// The second provider call should have prior messages
	if len(provider.calls) < 2 {
		t.Fatalf("expected at least 2 provider calls, got %d", len(provider.calls))
	}

	secondCallMsgs := provider.calls[1].Messages
	// Filter out system messages
	var nonSystem []Message
	for _, m := range secondCallMsgs {
		if m.Role != "system" {
			nonSystem = append(nonSystem, m)
		}
	}

	// Should have: prior user ("first question"), prior assistant ("first answer"), new user ("follow up")
	if len(nonSystem) < 3 {
		t.Fatalf("expected at least 3 non-system messages in second call, got %d: %+v", len(nonSystem), nonSystem)
	}

	if nonSystem[0].Role != "user" || nonSystem[0].Content != "first question" {
		t.Fatalf("expected first prior message to be user 'first question', got %+v", nonSystem[0])
	}
	if nonSystem[1].Role != "assistant" || nonSystem[1].Content != "first answer" {
		t.Fatalf("expected second prior message to be assistant 'first answer', got %+v", nonSystem[1])
	}
	if nonSystem[len(nonSystem)-1].Role != "user" || nonSystem[len(nonSystem)-1].Content != "follow up" {
		t.Fatalf("expected last message to be user 'follow up', got %+v", nonSystem[len(nonSystem)-1])
	}

	// Check for conversation.continued event
	foundContinued := false
	for _, ev := range events2 {
		if ev.Type == "conversation.continued" {
			foundContinued = true
			convID, _ := ev.Payload["conversation_id"].(string)
			if convID != "conv-multi" {
				t.Fatalf("expected conversation_id 'conv-multi', got %q", convID)
			}
			break
		}
	}
	if !foundContinued {
		t.Fatalf("expected conversation.continued event in second run, got events: %+v", eventTypes(events2))
	}
}

func TestRunnerConversationNotFound(t *testing.T) {
	t.Parallel()

	runner := NewRunner(&stubProvider{}, NewRegistry(), RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     2,
	})

	msgs, ok := runner.ConversationMessages("nonexistent")
	if ok {
		t.Fatalf("expected ok=false for nonexistent conversation")
	}
	if msgs != nil {
		t.Fatalf("expected nil messages for nonexistent conversation, got %+v", msgs)
	}
}

func TestRunnerFailedRunDoesNotStore(t *testing.T) {
	t.Parallel()

	runner := NewRunner(&errorProvider{err: errors.New("boom")}, NewRegistry(), RunnerConfig{
		DefaultModel: "gpt-4.1-mini",
		MaxSteps:     2,
	})

	run, err := runner.StartRun(RunRequest{
		Prompt:         "fail please",
		ConversationID: "conv-fail",
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	_, err = collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}

	msgs, ok := runner.ConversationMessages("conv-fail")
	if ok {
		t.Fatalf("expected ok=false for failed run conversation")
	}
	if msgs != nil {
		t.Fatalf("expected nil messages for failed run, got %+v", msgs)
	}
}
