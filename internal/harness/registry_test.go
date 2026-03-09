package harness

import (
	"context"
	"encoding/json"
	"testing"

	htools "go-agent-harness/internal/harness/tools"
)

func dummyHandler(_ context.Context, _ json.RawMessage) (string, error) {
	return "ok", nil
}

func TestRegistry_RegisterWithOptions(t *testing.T) {
	r := NewRegistry()
	def := ToolDefinition{Name: "test_tool", Description: "a test tool"}
	opts := RegisterOptions{
		Tier: htools.TierDeferred,
		Tags: []string{"search", "code"},
	}
	if err := r.RegisterWithOptions(def, dummyHandler, opts); err != nil {
		t.Fatalf("RegisterWithOptions failed: %v", err)
	}

	// Verify tool is registered
	defs := r.Definitions()
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(defs))
	}
	if defs[0].Name != "test_tool" {
		t.Errorf("expected name %q, got %q", "test_tool", defs[0].Name)
	}

	// Verify duplicate registration fails
	err := r.RegisterWithOptions(def, dummyHandler, opts)
	if err == nil {
		t.Fatal("expected error for duplicate registration")
	}

	// Verify empty name fails
	err = r.RegisterWithOptions(ToolDefinition{}, dummyHandler, opts)
	if err == nil {
		t.Fatal("expected error for empty tool name")
	}

	// Verify nil handler fails
	err = r.RegisterWithOptions(ToolDefinition{Name: "another"}, nil, opts)
	if err == nil {
		t.Fatal("expected error for nil handler")
	}
}

func TestRegistry_RegisterWithOptions_DefaultTier(t *testing.T) {
	r := NewRegistry()
	def := ToolDefinition{Name: "default_tier_tool", Description: "no tier set"}
	// Register with empty tier -- should default to core
	opts := RegisterOptions{}
	if err := r.RegisterWithOptions(def, dummyHandler, opts); err != nil {
		t.Fatalf("RegisterWithOptions failed: %v", err)
	}

	// Should appear in DefinitionsForRun without activation since it defaults to core
	defs := r.DefinitionsForRun("run-1", nil)
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition (default core), got %d", len(defs))
	}
}

func TestRegistry_DefinitionsForRun_CoreOnly(t *testing.T) {
	r := NewRegistry()

	// Register a core tool via Register
	coreDef := ToolDefinition{Name: "core_tool", Description: "core"}
	if err := r.Register(coreDef, dummyHandler); err != nil {
		t.Fatalf("Register core failed: %v", err)
	}

	// Register a deferred tool via RegisterWithOptions
	deferredDef := ToolDefinition{Name: "deferred_tool", Description: "deferred"}
	if err := r.RegisterWithOptions(deferredDef, dummyHandler, RegisterOptions{
		Tier: htools.TierDeferred,
		Tags: []string{"advanced"},
	}); err != nil {
		t.Fatalf("RegisterWithOptions deferred failed: %v", err)
	}

	// DefinitionsForRun with no activations should return only core
	defs := r.DefinitionsForRun("run-1", nil)
	if len(defs) != 1 {
		t.Fatalf("expected 1 core definition, got %d", len(defs))
	}
	if defs[0].Name != "core_tool" {
		t.Errorf("expected %q, got %q", "core_tool", defs[0].Name)
	}

	// Also test with a tracker that has no activations
	tracker := NewActivationTracker()
	defs = r.DefinitionsForRun("run-1", tracker)
	if len(defs) != 1 {
		t.Fatalf("expected 1 core definition with empty tracker, got %d", len(defs))
	}
	if defs[0].Name != "core_tool" {
		t.Errorf("expected %q, got %q", "core_tool", defs[0].Name)
	}
}

func TestRegistry_DefinitionsForRun_WithActivation(t *testing.T) {
	r := NewRegistry()

	coreDef := ToolDefinition{Name: "core_tool", Description: "core"}
	if err := r.Register(coreDef, dummyHandler); err != nil {
		t.Fatalf("Register core failed: %v", err)
	}

	deferredDef := ToolDefinition{Name: "deferred_tool", Description: "deferred"}
	if err := r.RegisterWithOptions(deferredDef, dummyHandler, RegisterOptions{
		Tier: htools.TierDeferred,
	}); err != nil {
		t.Fatalf("RegisterWithOptions deferred failed: %v", err)
	}

	tracker := NewActivationTracker()
	tracker.Activate("run-1", "deferred_tool")

	defs := r.DefinitionsForRun("run-1", tracker)
	if len(defs) != 2 {
		t.Fatalf("expected 2 definitions (core + activated deferred), got %d", len(defs))
	}

	// Results are sorted by name
	if defs[0].Name != "core_tool" {
		t.Errorf("expected first definition %q, got %q", "core_tool", defs[0].Name)
	}
	if defs[1].Name != "deferred_tool" {
		t.Errorf("expected second definition %q, got %q", "deferred_tool", defs[1].Name)
	}

	// Different run should not see the activation
	defs2 := r.DefinitionsForRun("run-2", tracker)
	if len(defs2) != 1 {
		t.Fatalf("expected 1 definition for unactivated run, got %d", len(defs2))
	}
	if defs2[0].Name != "core_tool" {
		t.Errorf("expected %q, got %q", "core_tool", defs2[0].Name)
	}
}

func TestRegistry_DeferredDefinitions(t *testing.T) {
	r := NewRegistry()

	// Register a core tool
	if err := r.Register(ToolDefinition{Name: "core_tool", Description: "core"}, dummyHandler); err != nil {
		t.Fatalf("Register core failed: %v", err)
	}

	// Register two deferred tools
	if err := r.RegisterWithOptions(ToolDefinition{Name: "deferred_b", Description: "deferred b"}, dummyHandler, RegisterOptions{
		Tier: htools.TierDeferred,
		Tags: []string{"b"},
	}); err != nil {
		t.Fatalf("RegisterWithOptions deferred_b failed: %v", err)
	}
	if err := r.RegisterWithOptions(ToolDefinition{Name: "deferred_a", Description: "deferred a"}, dummyHandler, RegisterOptions{
		Tier: htools.TierDeferred,
		Tags: []string{"a"},
	}); err != nil {
		t.Fatalf("RegisterWithOptions deferred_a failed: %v", err)
	}

	defs := r.DeferredDefinitions()
	if len(defs) != 2 {
		t.Fatalf("expected 2 deferred definitions, got %d", len(defs))
	}
	// Should be sorted by name
	if defs[0].Name != "deferred_a" {
		t.Errorf("expected first %q, got %q", "deferred_a", defs[0].Name)
	}
	if defs[1].Name != "deferred_b" {
		t.Errorf("expected second %q, got %q", "deferred_b", defs[1].Name)
	}
}

func TestRegistry_BackwardCompat_Definitions(t *testing.T) {
	r := NewRegistry()

	// Register core via Register
	if err := r.Register(ToolDefinition{Name: "core_tool", Description: "core"}, dummyHandler); err != nil {
		t.Fatalf("Register core failed: %v", err)
	}

	// Register deferred via RegisterWithOptions
	if err := r.RegisterWithOptions(ToolDefinition{Name: "deferred_tool", Description: "deferred"}, dummyHandler, RegisterOptions{
		Tier: htools.TierDeferred,
	}); err != nil {
		t.Fatalf("RegisterWithOptions deferred failed: %v", err)
	}

	// Definitions() should return ALL tools regardless of tier
	defs := r.Definitions()
	if len(defs) != 2 {
		t.Fatalf("expected 2 definitions from Definitions(), got %d", len(defs))
	}
	// Sorted by name
	if defs[0].Name != "core_tool" {
		t.Errorf("expected first %q, got %q", "core_tool", defs[0].Name)
	}
	if defs[1].Name != "deferred_tool" {
		t.Errorf("expected second %q, got %q", "deferred_tool", defs[1].Name)
	}
}

func TestRegistry_Execute_DeferredTool(t *testing.T) {
	r := NewRegistry()

	called := false
	handler := func(_ context.Context, _ json.RawMessage) (string, error) {
		called = true
		return "deferred result", nil
	}

	if err := r.RegisterWithOptions(ToolDefinition{Name: "deferred_tool", Description: "deferred"}, handler, RegisterOptions{
		Tier: htools.TierDeferred,
	}); err != nil {
		t.Fatalf("RegisterWithOptions failed: %v", err)
	}

	// Execute should work even without activation -- deferred tools can be called,
	// they just aren't listed for the LLM unless activated.
	result, err := r.Execute(context.Background(), "deferred_tool", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !called {
		t.Error("expected handler to be called")
	}
	if result != "deferred result" {
		t.Errorf("expected %q, got %q", "deferred result", result)
	}
}
