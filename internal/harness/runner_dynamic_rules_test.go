package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// TestDynamicRuleTypes verifies the struct definitions are correct.
func TestDynamicRuleTypes(t *testing.T) {
	rule := DynamicRule{
		ID: "rule-1",
		Trigger: RuleTrigger{
			ToolNames: []string{"bash", "grep"},
		},
		Content:  "Remember to be careful with bash commands.",
		FireOnce: true,
	}
	if rule.ID != "rule-1" {
		t.Errorf("ID = %q, want %q", rule.ID, "rule-1")
	}
	if len(rule.Trigger.ToolNames) != 2 {
		t.Errorf("ToolNames len = %d, want 2", len(rule.Trigger.ToolNames))
	}
	if !rule.FireOnce {
		t.Error("FireOnce should be true")
	}
}

// TestDynamicRuleJSONSerialization verifies that DynamicRule serialises/
// deserialises correctly via JSON (important for RunRequest wire format).
func TestDynamicRuleJSONSerialization(t *testing.T) {
	orig := DynamicRule{
		ID: "json-rule",
		Trigger: RuleTrigger{
			ToolNames: []string{"write_file"},
		},
		Content:  "Always check file paths.",
		FireOnce: false,
	}
	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var got DynamicRule
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got.ID != orig.ID {
		t.Errorf("ID = %q, want %q", got.ID, orig.ID)
	}
	if len(got.Trigger.ToolNames) != 1 || got.Trigger.ToolNames[0] != "write_file" {
		t.Errorf("ToolNames = %v, want [write_file]", got.Trigger.ToolNames)
	}
	if got.Content != orig.Content {
		t.Errorf("Content = %q, want %q", got.Content, orig.Content)
	}
}

// TestRunRequestDynamicRulesField verifies DynamicRules is present on RunRequest.
func TestRunRequestDynamicRulesField(t *testing.T) {
	req := RunRequest{
		Prompt: "test",
		DynamicRules: []DynamicRule{
			{
				ID:      "r1",
				Trigger: RuleTrigger{ToolNames: []string{"bash"}},
				Content: "Be careful",
			},
		},
	}
	if len(req.DynamicRules) != 1 {
		t.Errorf("DynamicRules len = %d, want 1", len(req.DynamicRules))
	}
}

// TestMergeDynamicRules verifies runner-level + per-run rules merge correctly.
func TestMergeDynamicRules(t *testing.T) {
	t.Run("both empty", func(t *testing.T) {
		result := mergeDynamicRules(nil, nil)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("runner only", func(t *testing.T) {
		runner := []DynamicRule{{ID: "r1", Content: "runner"}}
		result := mergeDynamicRules(runner, nil)
		if len(result) != 1 || result[0].ID != "r1" {
			t.Errorf("unexpected result: %v", result)
		}
	})

	t.Run("req only", func(t *testing.T) {
		req := []DynamicRule{{ID: "r2", Content: "req"}}
		result := mergeDynamicRules(nil, req)
		if len(result) != 1 || result[0].ID != "r2" {
			t.Errorf("unexpected result: %v", result)
		}
	})

	t.Run("both set — runner comes first", func(t *testing.T) {
		runner := []DynamicRule{{ID: "r1", Content: "runner"}}
		req := []DynamicRule{{ID: "r2", Content: "req"}}
		result := mergeDynamicRules(runner, req)
		if len(result) != 2 {
			t.Fatalf("want 2, got %d", len(result))
		}
		if result[0].ID != "r1" || result[1].ID != "r2" {
			t.Errorf("wrong order: %v", result)
		}
	})

	t.Run("original slices not mutated", func(t *testing.T) {
		runner := []DynamicRule{{ID: "r1", Content: "runner"}}
		req := []DynamicRule{{ID: "r2", Content: "req"}}
		result := mergeDynamicRules(runner, req)
		result[0].ID = "mutated"
		// Original slices must be unchanged.
		if runner[0].ID != "r1" {
			t.Error("runner slice was mutated")
		}
		if req[0].ID != "r2" {
			t.Error("req slice was mutated")
		}
	})
}

// TestDynamicRuleZeroRules verifies that runs with no dynamic rules behave
// identically to before (backward compatibility).
func TestDynamicRuleZeroRules(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{
		turns: []CompletionResult{
			{Content: "done"},
		},
	}
	runner := NewRunner(provider, nil, RunnerConfig{})

	run, err := runner.StartRun(RunRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForRunCompletion(t, runner, run.ID)

	events := getRunEvents(t, runner, run.ID)
	for _, ev := range events {
		if ev.Type == EventRuleInjected {
			t.Error("unexpected rule.injected event when no rules configured")
		}
	}
}

// TestDynamicRuleFiresOnToolName verifies that a rule fires when the trigger
// tool name matches a tool called in the previous step.
func TestDynamicRuleFiresOnToolName(t *testing.T) {
	t.Parallel()

	// Step 1: LLM calls "special_tool" → rule should fire on step 2
	// Step 2: LLM returns plain text
	provider := &capturingProvider{
		turns: []CompletionResult{
			{
				Content: "",
				ToolCalls: []ToolCall{
					{ID: "c1", Name: "special_tool", Arguments: `{}`},
				},
			},
			{Content: "all done"},
		},
	}

	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name:        "special_tool",
		Description: "a test tool",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return "tool result", nil
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	runner := NewRunner(provider, registry, RunnerConfig{})

	run, err := runner.StartRun(RunRequest{
		Prompt: "use the tool",
		DynamicRules: []DynamicRule{
			{
				ID:      "rule-special",
				Trigger: RuleTrigger{ToolNames: []string{"special_tool"}},
				Content: "INJECTED: you called special_tool",
			},
		},
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForRunCompletion(t, runner, run.ID)

	// Verify rule.injected event was emitted.
	events := getRunEvents(t, runner, run.ID)
	injectedEvents := filterEvents(events, EventRuleInjected)
	if len(injectedEvents) != 1 {
		t.Errorf("want 1 rule.injected event, got %d", len(injectedEvents))
	}
	if len(injectedEvents) > 0 {
		ev := injectedEvents[0]
		if ev.Payload["rule_id"] != "rule-special" {
			t.Errorf("rule_id = %v, want %q", ev.Payload["rule_id"], "rule-special")
		}
		if ev.Payload["trigger_tool"] != "special_tool" {
			t.Errorf("trigger_tool = %v, want %q", ev.Payload["trigger_tool"], "special_tool")
		}
		stepVal, _ := ev.Payload["step"].(int)
		if stepVal != 2 {
			// step may be float64 from JSON round-trip
			stepFloat, _ := ev.Payload["step"].(float64)
			if int(stepFloat) != 2 && stepVal != 2 {
				t.Errorf("step = %v, want 2", ev.Payload["step"])
			}
		}
	}

	// Verify the rule content was injected into the LLM request for step 2.
	if len(provider.calls) < 2 {
		t.Fatalf("expected at least 2 provider calls, got %d", len(provider.calls))
	}
	step2Req := provider.calls[1]
	found := false
	for _, m := range step2Req.Messages {
		if m.Role == "system" && strings.Contains(m.Content, "INJECTED: you called special_tool") {
			found = true
			break
		}
	}
	if !found {
		t.Error("rule content was not injected into step 2 system messages")
	}
}

// TestDynamicRuleNoFireOnStep1 verifies that rules do not fire on step 1
// (there are no previous tool calls yet).
func TestDynamicRuleNoFireOnStep1(t *testing.T) {
	t.Parallel()

	provider := &capturingProvider{
		turns: []CompletionResult{
			{Content: "done"},
		},
	}

	runner := NewRunner(provider, nil, RunnerConfig{})
	run, err := runner.StartRun(RunRequest{
		Prompt: "test",
		DynamicRules: []DynamicRule{
			{
				ID:      "r1",
				Trigger: RuleTrigger{ToolNames: []string{"any_tool"}},
				Content: "Should not appear on step 1",
			},
		},
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForRunCompletion(t, runner, run.ID)

	events := getRunEvents(t, runner, run.ID)
	for _, ev := range events {
		if ev.Type == EventRuleInjected {
			t.Error("rule.injected should not fire on step 1 (no prior tool calls)")
		}
	}

	// The step 1 system messages must not contain the rule content.
	if len(provider.calls) > 0 {
		for _, m := range provider.calls[0].Messages {
			if m.Role == "system" && strings.Contains(m.Content, "Should not appear") {
				t.Error("rule content appeared in step 1 system messages")
			}
		}
	}
}

// TestDynamicRuleFireOnce verifies that a FireOnce rule fires at most once
// across multiple matching steps.
func TestDynamicRuleFireOnce(t *testing.T) {
	t.Parallel()

	// 3 steps: step1 calls tool, step2 calls tool again, step3 returns text.
	// FireOnce rule should only inject on step 2 (after step 1 tool call).
	provider := &capturingProvider{
		turns: []CompletionResult{
			{
				ToolCalls: []ToolCall{{ID: "c1", Name: "mytool", Arguments: `{}`}},
			},
			{
				ToolCalls: []ToolCall{{ID: "c2", Name: "mytool", Arguments: `{}`}},
			},
			{Content: "done"},
		},
	}

	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name:        "mytool",
		Description: "test",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return "ok", nil
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	runner := NewRunner(provider, registry, RunnerConfig{})
	run, err := runner.StartRun(RunRequest{
		Prompt: "use mytool",
		DynamicRules: []DynamicRule{
			{
				ID:       "once-rule",
				Trigger:  RuleTrigger{ToolNames: []string{"mytool"}},
				Content:  "FireOnce content",
				FireOnce: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForRunCompletion(t, runner, run.ID)

	events := getRunEvents(t, runner, run.ID)
	injected := filterEvents(events, EventRuleInjected)
	if len(injected) != 1 {
		t.Errorf("FireOnce rule should emit exactly 1 rule.injected event, got %d", len(injected))
	}
}

// TestDynamicRuleFireEveryTime verifies that a rule without FireOnce fires
// on every matching step.
func TestDynamicRuleFireEveryTime(t *testing.T) {
	t.Parallel()

	// 3 steps: step1 calls tool, step2 calls tool again, step3 returns text.
	// Non-FireOnce rule should inject on both step 2 and step 3.
	provider := &capturingProvider{
		turns: []CompletionResult{
			{
				ToolCalls: []ToolCall{{ID: "c1", Name: "mytool", Arguments: `{}`}},
			},
			{
				ToolCalls: []ToolCall{{ID: "c2", Name: "mytool", Arguments: `{}`}},
			},
			{Content: "done"},
		},
	}

	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name:        "mytool",
		Description: "test",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return "ok", nil
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	runner := NewRunner(provider, registry, RunnerConfig{})
	run, err := runner.StartRun(RunRequest{
		Prompt: "use mytool",
		DynamicRules: []DynamicRule{
			{
				ID:       "repeat-rule",
				Trigger:  RuleTrigger{ToolNames: []string{"mytool"}},
				Content:  "Repeated injection",
				FireOnce: false,
			},
		},
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForRunCompletion(t, runner, run.ID)

	events := getRunEvents(t, runner, run.ID)
	injected := filterEvents(events, EventRuleInjected)
	if len(injected) != 2 {
		t.Errorf("non-FireOnce rule should emit 2 rule.injected events, got %d", len(injected))
	}
}

// TestDynamicRuleNoMatchNoFire verifies rules do not fire when the trigger
// tool name does not match any tool called in the previous step.
func TestDynamicRuleNoMatchNoFire(t *testing.T) {
	t.Parallel()

	provider := &capturingProvider{
		turns: []CompletionResult{
			{
				ToolCalls: []ToolCall{{ID: "c1", Name: "other_tool", Arguments: `{}`}},
			},
			{Content: "done"},
		},
	}

	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name:        "other_tool",
		Description: "test",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return "ok", nil
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	runner := NewRunner(provider, registry, RunnerConfig{})
	run, err := runner.StartRun(RunRequest{
		Prompt: "test",
		DynamicRules: []DynamicRule{
			{
				ID:      "non-matching-rule",
				Trigger: RuleTrigger{ToolNames: []string{"different_tool"}},
				Content: "Should not fire",
			},
		},
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForRunCompletion(t, runner, run.ID)

	events := getRunEvents(t, runner, run.ID)
	for _, ev := range events {
		if ev.Type == EventRuleInjected {
			t.Errorf("unexpected rule.injected event: %v", ev.Payload)
		}
	}
}

// TestDynamicRuleRunnerConfigRules verifies rules set at runner config level
// are applied to all runs.
func TestDynamicRuleRunnerConfigRules(t *testing.T) {
	t.Parallel()

	provider := &capturingProvider{
		turns: []CompletionResult{
			{
				ToolCalls: []ToolCall{{ID: "c1", Name: "bash", Arguments: `{}`}},
			},
			{Content: "done"},
		},
	}

	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name:        "bash",
		Description: "test",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return "ok", nil
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	runner := NewRunner(provider, registry, RunnerConfig{
		DynamicRules: []DynamicRule{
			{
				ID:      "runner-level-rule",
				Trigger: RuleTrigger{ToolNames: []string{"bash"}},
				Content: "Runner-level bash reminder",
			},
		},
	})

	run, err := runner.StartRun(RunRequest{Prompt: "use bash"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForRunCompletion(t, runner, run.ID)

	events := getRunEvents(t, runner, run.ID)
	injected := filterEvents(events, EventRuleInjected)
	if len(injected) != 1 {
		t.Errorf("want 1 rule.injected from runner config, got %d", len(injected))
	}
	if len(injected) > 0 && injected[0].Payload["rule_id"] != "runner-level-rule" {
		t.Errorf("rule_id = %v, want %q", injected[0].Payload["rule_id"], "runner-level-rule")
	}
}

// TestDynamicRuleRunnerAndRequestRulesMerge verifies that runner-config rules
// and per-request rules are both active in the same run.
func TestDynamicRuleRunnerAndRequestRulesMerge(t *testing.T) {
	t.Parallel()

	provider := &capturingProvider{
		turns: []CompletionResult{
			{
				ToolCalls: []ToolCall{
					{ID: "c1", Name: "toolA", Arguments: `{}`},
					{ID: "c2", Name: "toolB", Arguments: `{}`},
				},
			},
			{Content: "done"},
		},
	}

	registry := NewRegistry()
	for _, name := range []string{"toolA", "toolB"} {
		n := name
		if err := registry.Register(ToolDefinition{
			Name:        n,
			Description: "test",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		}, func(_ context.Context, _ json.RawMessage) (string, error) {
			return "ok", nil
		}); err != nil {
			t.Fatalf("Register %s: %v", n, err)
		}
	}

	runner := NewRunner(provider, registry, RunnerConfig{
		DynamicRules: []DynamicRule{
			{
				ID:      "runner-rule",
				Trigger: RuleTrigger{ToolNames: []string{"toolA"}},
				Content: "Runner rule content",
			},
		},
	})

	run, err := runner.StartRun(RunRequest{
		Prompt: "test merge",
		DynamicRules: []DynamicRule{
			{
				ID:      "req-rule",
				Trigger: RuleTrigger{ToolNames: []string{"toolB"}},
				Content: "Request rule content",
			},
		},
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForRunCompletion(t, runner, run.ID)

	events := getRunEvents(t, runner, run.ID)
	injected := filterEvents(events, EventRuleInjected)
	if len(injected) != 2 {
		t.Errorf("want 2 rule.injected events (one from runner, one from req), got %d", len(injected))
	}

	ruleIDs := make(map[string]bool)
	for _, ev := range injected {
		if id, ok := ev.Payload["rule_id"].(string); ok {
			ruleIDs[id] = true
		}
	}
	if !ruleIDs["runner-rule"] {
		t.Error("runner-rule did not fire")
	}
	if !ruleIDs["req-rule"] {
		t.Error("req-rule did not fire")
	}
}

// TestDynamicRuleEventRuleInjectedInAllEventTypes verifies EventRuleInjected
// is registered in AllEventTypes().
func TestDynamicRuleEventRuleInjectedInAllEventTypes(t *testing.T) {
	found := false
	for _, et := range AllEventTypes() {
		if et == EventRuleInjected {
			found = true
			break
		}
	}
	if !found {
		t.Error("EventRuleInjected not found in AllEventTypes()")
	}
}

// TestDynamicRuleEventType verifies the event type string value.
func TestDynamicRuleEventType(t *testing.T) {
	if string(EventRuleInjected) != "rule.injected" {
		t.Errorf("EventRuleInjected = %q, want %q", EventRuleInjected, "rule.injected")
	}
}

// TestDynamicRuleConcurrencyRace verifies that concurrent StartRun calls with
// dynamic rules do not race on shared state. Run with -race.
func TestDynamicRuleConcurrencyRace(t *testing.T) {
	t.Parallel()

	var wg sync.WaitGroup
	const goroutines = 8

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			provider := &capturingProvider{
				turns: []CompletionResult{
					{Content: "done"},
				},
			}
			runner := NewRunner(provider, nil, RunnerConfig{
				DynamicRules: []DynamicRule{
					{
						ID:      "concurrent-rule",
						Trigger: RuleTrigger{ToolNames: []string{"any_tool"}},
						Content: "concurrent content",
					},
				},
			})

			run, err := runner.StartRun(RunRequest{Prompt: "concurrent test"})
			if err != nil {
				return
			}
			waitForRunCompletion(t, runner, run.ID)
		}(i)
	}
	wg.Wait()
}

// TestDynamicRuleSkipRulesWithEmptyIDOrContent verifies that rules with empty
// ID or empty Content are silently skipped without panic.
func TestDynamicRuleSkipRulesWithEmptyIDOrContent(t *testing.T) {
	t.Parallel()

	provider := &capturingProvider{
		turns: []CompletionResult{
			{
				ToolCalls: []ToolCall{{ID: "c1", Name: "tool1", Arguments: `{}`}},
			},
			{Content: "done"},
		},
	}

	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name:        "tool1",
		Description: "test",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return "ok", nil
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	runner := NewRunner(provider, registry, RunnerConfig{})
	run, err := runner.StartRun(RunRequest{
		Prompt: "test",
		DynamicRules: []DynamicRule{
			{
				// No ID — should be skipped.
				Trigger: RuleTrigger{ToolNames: []string{"tool1"}},
				Content: "content without ID",
			},
			{
				ID:      "no-content",
				Trigger: RuleTrigger{ToolNames: []string{"tool1"}},
				Content: "", // No content — should be skipped.
			},
			{
				ID:      "valid-rule",
				Trigger: RuleTrigger{ToolNames: []string{"tool1"}},
				Content: "valid content",
			},
		},
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForRunCompletion(t, runner, run.ID)

	events := getRunEvents(t, runner, run.ID)
	injected := filterEvents(events, EventRuleInjected)
	// Only the valid rule should fire.
	if len(injected) != 1 {
		t.Errorf("want 1 rule.injected event, got %d", len(injected))
	}
	if len(injected) > 0 && injected[0].Payload["rule_id"] != "valid-rule" {
		t.Errorf("rule_id = %v, want %q", injected[0].Payload["rule_id"], "valid-rule")
	}
}

// TestDynamicRuleMultipleToolNameTriggers verifies that a rule with multiple
// tool names in the trigger fires when any one of them is called.
func TestDynamicRuleMultipleToolNameTriggers(t *testing.T) {
	t.Parallel()

	provider := &capturingProvider{
		turns: []CompletionResult{
			{
				ToolCalls: []ToolCall{{ID: "c1", Name: "toolB", Arguments: `{}`}},
			},
			{Content: "done"},
		},
	}

	registry := NewRegistry()
	if err := registry.Register(ToolDefinition{
		Name:        "toolB",
		Description: "test",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
	}, func(_ context.Context, _ json.RawMessage) (string, error) {
		return "ok", nil
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	runner := NewRunner(provider, registry, RunnerConfig{})
	run, err := runner.StartRun(RunRequest{
		Prompt: "test",
		DynamicRules: []DynamicRule{
			{
				ID:      "multi-trigger",
				Trigger: RuleTrigger{ToolNames: []string{"toolA", "toolB", "toolC"}},
				Content: "Multi trigger fired",
			},
		},
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForRunCompletion(t, runner, run.ID)

	events := getRunEvents(t, runner, run.ID)
	injected := filterEvents(events, EventRuleInjected)
	if len(injected) != 1 {
		t.Errorf("want 1 rule.injected event, got %d", len(injected))
	}
	if len(injected) > 0 {
		if injected[0].Payload["rule_id"] != "multi-trigger" {
			t.Errorf("rule_id = %v, want %q", injected[0].Payload["rule_id"], "multi-trigger")
		}
		if injected[0].Payload["trigger_tool"] != "toolB" {
			t.Errorf("trigger_tool = %v, want %q", injected[0].Payload["trigger_tool"], "toolB")
		}
	}
}

// TestDynamicRuleBoundsValidationTooManyRules verifies that StartRun rejects
// requests with more than the allowed number of dynamic rules.
func TestDynamicRuleBoundsValidationTooManyRules(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{
		turns: []CompletionResult{{Content: "done"}},
	}
	runner := NewRunner(provider, nil, RunnerConfig{})

	// Build 51 rules (1 over the limit of 50).
	rules := make([]DynamicRule, 51)
	for i := range rules {
		rules[i] = DynamicRule{
			ID:      fmt.Sprintf("rule-%d", i),
			Trigger: RuleTrigger{ToolNames: []string{"bash"}},
			Content: "some content",
		}
	}

	_, err := runner.StartRun(RunRequest{
		Prompt:       "test",
		DynamicRules: rules,
	})
	if err == nil {
		t.Fatal("StartRun should have returned an error for too many dynamic rules")
	}
	if !strings.Contains(err.Error(), "too many dynamic rules") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestDynamicRuleBoundsValidationContentTooLarge verifies that StartRun rejects
// a dynamic rule whose Content exceeds the per-rule size limit.
func TestDynamicRuleBoundsValidationContentTooLarge(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{
		turns: []CompletionResult{{Content: "done"}},
	}
	runner := NewRunner(provider, nil, RunnerConfig{})

	// Build content that is 64KB + 1 byte (just over the limit).
	oversizedContent := strings.Repeat("x", 64*1024+1)

	_, err := runner.StartRun(RunRequest{
		Prompt: "test",
		DynamicRules: []DynamicRule{
			{
				ID:      "big-rule",
				Trigger: RuleTrigger{ToolNames: []string{"bash"}},
				Content: oversizedContent,
			},
		},
	})
	if err == nil {
		t.Fatal("StartRun should have returned an error for oversized rule content")
	}
	if !strings.Contains(err.Error(), "content too large") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestDynamicRuleBoundsValidationAtLimitsAccepted verifies that exactly 50
// rules each of exactly 64KB are accepted (boundary: at limit, not over).
func TestDynamicRuleBoundsValidationAtLimitsAccepted(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{
		turns: []CompletionResult{{Content: "done"}},
	}
	runner := NewRunner(provider, nil, RunnerConfig{})

	// Exactly 50 rules, each with exactly 64KB content.
	exactContent := strings.Repeat("y", 64*1024)
	rules := make([]DynamicRule, 50)
	for i := range rules {
		rules[i] = DynamicRule{
			ID:      fmt.Sprintf("rule-%d", i),
			Trigger: RuleTrigger{ToolNames: []string{"bash"}},
			Content: exactContent,
		}
	}

	_, err := runner.StartRun(RunRequest{
		Prompt:       "test",
		DynamicRules: rules,
	})
	if err != nil {
		t.Fatalf("StartRun should accept exactly 50 rules each with 64KB content, got: %v", err)
	}
}

// ---- Helpers ----------------------------------------------------------------

// waitForRunCompletion blocks until the run reaches a terminal state.
func waitForRunCompletion(t *testing.T, runner *Runner, runID string) {
	t.Helper()
	history, stream, cancel, err := runner.Subscribe(runID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer cancel()
	for _, ev := range history {
		if IsTerminalEvent(ev.Type) {
			return
		}
	}
	for ev := range stream {
		if IsTerminalEvent(ev.Type) {
			return
		}
	}
}

// getRunEvents returns all events for a run (read under lock).
func getRunEvents(t *testing.T, runner *Runner, runID string) []Event {
	t.Helper()
	runner.mu.RLock()
	defer runner.mu.RUnlock()
	state, ok := runner.runs[runID]
	if !ok {
		t.Fatalf("run %q not found", runID)
	}
	return append([]Event(nil), state.events...)
}

// filterEvents returns events matching the given type.
func filterEvents(events []Event, et EventType) []Event {
	var out []Event
	for _, ev := range events {
		if ev.Type == et {
			out = append(out, ev)
		}
	}
	return out
}
