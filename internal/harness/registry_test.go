package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
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

func TestRegistry_DefinitionsIsolationForToolParameters(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	original := ToolDefinition{
		Name:        "schema_tool",
		Description: "schema",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string"},
			},
		},
	}
	if err := r.Register(original, dummyHandler); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Mutating the caller-owned definition after Register must not corrupt the
	// registry's stored schema.
	original.Parameters["type"] = "array"
	original.Parameters["properties"].(map[string]any)["path"] = map[string]any{"type": "integer"}

	defs1 := r.Definitions()
	if got := defs1[0].Parameters["type"]; got != "object" {
		t.Fatalf("stored parameters mutated via caller alias: got %v, want object", got)
	}

	props1, ok := defs1[0].Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties type = %T, want map[string]any", defs1[0].Parameters["properties"])
	}
	pathSchema1, ok := props1["path"].(map[string]any)
	if !ok {
		t.Fatalf("path schema type = %T, want map[string]any", props1["path"])
	}
	if got := pathSchema1["type"]; got != "string" {
		t.Fatalf("stored nested schema mutated via caller alias: got %v, want string", got)
	}

	// Mutating a returned definition must not affect subsequent reads.
	defs1[0].Parameters["type"] = "number"
	props1["path"] = map[string]any{"type": "boolean"}

	defs2 := r.Definitions()
	if got := defs2[0].Parameters["type"]; got != "object" {
		t.Fatalf("registry returned aliased parameters: got %v, want object", got)
	}

	props2, ok := defs2[0].Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatalf("second properties type = %T, want map[string]any", defs2[0].Parameters["properties"])
	}
	pathSchema2, ok := props2["path"].(map[string]any)
	if !ok {
		t.Fatalf("second path schema type = %T, want map[string]any", props2["path"])
	}
	if got := pathSchema2["type"]; got != "string" {
		t.Fatalf("nested schema corrupted across Definitions calls: got %v, want string", got)
	}
}

func TestRegistry_DefinitionsWithMetadataIsolation(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	if err := r.RegisterWithOptions(ToolDefinition{
		Name:        "meta_tool",
		Description: "metadata",
		Parameters: map[string]any{
			"type": "object",
		},
	}, dummyHandler, RegisterOptions{
		Tier: htools.TierDeferred,
		Tags: []string{"profiles", "manifest"},
	}); err != nil {
		t.Fatalf("RegisterWithOptions failed: %v", err)
	}

	defs1 := r.DefinitionsWithMetadata()
	if len(defs1) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(defs1))
	}
	if defs1[0].Tier != htools.TierDeferred {
		t.Fatalf("tier = %q, want %q", defs1[0].Tier, htools.TierDeferred)
	}
	defs1[0].Tags[0] = "mutated"
	defs1[0].Definition.Parameters["type"] = "array"

	defs2 := r.DefinitionsWithMetadata()
	if defs2[0].Tags[0] != "profiles" {
		t.Fatalf("tags mutated across calls: %v", defs2[0].Tags)
	}
	if got := defs2[0].Definition.Parameters["type"]; got != "object" {
		t.Fatalf("parameters mutated across calls: got %v want object", got)
	}
}

func TestRegistry_DefinitionsForRunIsolationForToolParameters(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	if err := r.RegisterWithOptions(ToolDefinition{
		Name:        "deferred_schema_tool",
		Description: "schema",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
			},
		},
	}, dummyHandler, RegisterOptions{Tier: htools.TierDeferred}); err != nil {
		t.Fatalf("RegisterWithOptions failed: %v", err)
	}

	tracker := NewActivationTracker()
	tracker.Activate("run-1", "deferred_schema_tool")

	defs1 := r.DefinitionsForRun("run-1", tracker)
	if len(defs1) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(defs1))
	}
	defs1[0].Parameters["type"] = "corrupted"

	defs2 := r.DefinitionsForRun("run-1", tracker)
	if got := defs2[0].Parameters["type"]; got != "object" {
		t.Fatalf("DefinitionsForRun returned aliased parameters: got %v, want object", got)
	}
}

func TestToolDefinitionClonePreservesNilSemantics(t *testing.T) {
	t.Parallel()

	nilDef := ToolDefinition{Name: "nil_params"}
	if cloned := nilDef.Clone(); cloned.Parameters != nil {
		t.Fatalf("nil parameters should remain nil, got %#v", cloned.Parameters)
	}

	emptyDef := ToolDefinition{Name: "empty_params", Parameters: map[string]any{}}
	cloned := emptyDef.Clone()
	if cloned.Parameters == nil {
		t.Fatal("empty parameters should remain non-nil")
	}
	if !reflect.DeepEqual(cloned.Parameters, emptyDef.Parameters) {
		t.Fatalf("clone mismatch: got %#v want %#v", cloned.Parameters, emptyDef.Parameters)
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

// TestRegistry_ReplaceByTag_Basic verifies that ReplaceByTag removes old
// tools with the given tag and inserts the new set.
func TestRegistry_ReplaceByTag_Basic(t *testing.T) {
	r := NewRegistry()

	// Register an untagged core tool that must not be affected.
	_ = r.Register(ToolDefinition{Name: "permanent_tool", Description: "untagged"}, dummyHandler)

	// Register two tools with the "skills" source tag via RegisterWithOptions.
	_ = r.RegisterWithOptions(ToolDefinition{Name: "skill_a", Description: "skill a"}, dummyHandler, RegisterOptions{
		Tags: []string{"skills"},
	})
	_ = r.RegisterWithOptions(ToolDefinition{Name: "skill_b", Description: "skill b"}, dummyHandler, RegisterOptions{
		Tags: []string{"skills"},
	})

	// Confirm initial state: 3 tools.
	if n := len(r.Definitions()); n != 3 {
		t.Fatalf("expected 3 tools before ReplaceByTag, got %d", n)
	}

	// Hot-reload: replace the "skills" set with a single new skill.
	newSkillTool := htools.Tool{
		Definition: htools.Definition{
			Name:        "skill_c",
			Description: "skill c",
		},
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "c", nil
		},
	}
	if err := r.ReplaceByTag("skills", []htools.Tool{newSkillTool}); err != nil {
		t.Fatalf("ReplaceByTag failed: %v", err)
	}

	defs := r.Definitions()
	if len(defs) != 2 {
		t.Fatalf("expected 2 tools after ReplaceByTag, got %d: %v", len(defs), defs)
	}

	names := map[string]bool{}
	for _, d := range defs {
		names[d.Name] = true
	}
	if !names["permanent_tool"] {
		t.Error("permanent_tool should still be registered")
	}
	if !names["skill_c"] {
		t.Error("skill_c should have been registered")
	}
	if names["skill_a"] || names["skill_b"] {
		t.Error("skill_a and skill_b should have been removed")
	}
}

// TestRegistry_ReplaceByTag_EmptyNewTools verifies that passing an empty
// slice removes all tools with the tag and adds nothing.
func TestRegistry_ReplaceByTag_EmptyNewTools(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(ToolDefinition{Name: "perm", Description: "perm"}, dummyHandler)
	_ = r.RegisterWithOptions(ToolDefinition{Name: "tagged_tool", Description: "tagged"}, dummyHandler, RegisterOptions{
		Tags: []string{"mytag"},
	})

	if err := r.ReplaceByTag("mytag", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defs := r.Definitions()
	if len(defs) != 1 || defs[0].Name != "perm" {
		t.Errorf("expected only 'perm' to remain, got %v", defs)
	}
}

// TestRegistry_ReplaceByTag_EmptySourceTag verifies that an empty tag returns an error.
func TestRegistry_ReplaceByTag_EmptySourceTag(t *testing.T) {
	r := NewRegistry()
	err := r.ReplaceByTag("", nil)
	if err == nil {
		t.Fatal("expected error for empty sourceTag")
	}
}

// TestRegistry_ReplaceByTag_EmptyToolName verifies that a tool with an empty name fails.
func TestRegistry_ReplaceByTag_EmptyToolName(t *testing.T) {
	r := NewRegistry()
	badTool := htools.Tool{
		Definition: htools.Definition{Name: ""},
		Handler:    dummyHandler,
	}
	err := r.ReplaceByTag("tag", []htools.Tool{badTool})
	if err == nil {
		t.Fatal("expected error for tool with empty name")
	}
}

// TestRegistry_ReplaceByTag_Concurrent verifies race-free behaviour when
// ReplaceByTag, Definitions, and Execute are called concurrently.
func TestRegistry_ReplaceByTag_Concurrent(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(ToolDefinition{Name: "core", Description: "core"}, dummyHandler)
	_ = r.RegisterWithOptions(ToolDefinition{Name: "dyn_0", Description: "dyn"}, dummyHandler, RegisterOptions{
		Tags: []string{"dynamic"},
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 20; i++ {
			name := fmt.Sprintf("dyn_%d", i)
			tool := htools.Tool{
				Definition: htools.Definition{Name: name, Description: "dynamic tool"},
				Handler:    dummyHandler,
			}
			_ = r.ReplaceByTag("dynamic", []htools.Tool{tool})
		}
	}()

	// Concurrent readers
	var wg sync.WaitGroup
	for j := 0; j < 4; j++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for k := 0; k < 20; k++ {
				_ = r.Definitions()
			}
		}()
	}

	<-done
	wg.Wait()
}

// TestRegistry_ReplaceByTag_ExecuteAfterReload verifies that the new handler
// is callable after a hot-reload.
func TestRegistry_ReplaceByTag_ExecuteAfterReload(t *testing.T) {
	r := NewRegistry()
	_ = r.RegisterWithOptions(ToolDefinition{Name: "skill_x", Description: "x"}, dummyHandler, RegisterOptions{
		Tags: []string{"skills"},
	})

	// Replace with a new handler
	newHandler := func(_ context.Context, _ json.RawMessage) (string, error) {
		return "new-result", nil
	}
	newTool := htools.Tool{
		Definition: htools.Definition{Name: "skill_x", Description: "updated x"},
		Handler:    newHandler,
	}
	if err := r.ReplaceByTag("skills", []htools.Tool{newTool}); err != nil {
		t.Fatalf("ReplaceByTag failed: %v", err)
	}

	result, err := r.Execute(context.Background(), "skill_x", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result != "new-result" {
		t.Errorf("expected %q, got %q", "new-result", result)
	}
}
