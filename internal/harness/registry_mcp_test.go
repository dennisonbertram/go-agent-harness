package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	htools "go-agent-harness/internal/harness/tools"
)

// mockMCPReg is a minimal MCPRegistry implementation for testing RegisterMCPTools.
type mockMCPReg struct{}

func (m *mockMCPReg) ListResources(_ context.Context, server string) ([]htools.MCPResource, error) {
	return nil, nil
}
func (m *mockMCPReg) ReadResource(_ context.Context, server, uri string) (string, error) {
	return "", nil
}
func (m *mockMCPReg) ListTools(_ context.Context) (map[string][]htools.MCPToolDefinition, error) {
	return nil, nil
}
func (m *mockMCPReg) CallTool(_ context.Context, server, tool string, args json.RawMessage) (string, error) {
	return `{"result":"ok"}`, nil
}

// TestRegistry_RegisterMCPTools_Success verifies RegisterMCPTools registers tools correctly.
func TestRegistry_RegisterMCPTools_Success(t *testing.T) {
	r := NewRegistry()
	caller := &mockMCPReg{}

	toolDefs := []htools.MCPToolDefinition{
		{Name: "search", Description: "Search", Parameters: map[string]any{}},
		{Name: "fetch", Description: "Fetch", Parameters: map[string]any{}},
	}

	registered, err := r.RegisterMCPTools("my-server", toolDefs, caller)
	if err != nil {
		t.Fatalf("RegisterMCPTools failed: %v", err)
	}
	if len(registered) != 2 {
		t.Fatalf("expected 2 registered tools, got %d", len(registered))
	}

	// Verify tools appear in registry at deferred tier.
	defs := r.DeferredDefinitions()
	if len(defs) != 2 {
		t.Fatalf("expected 2 deferred definitions, got %d", len(defs))
	}

	// Verify tool names follow the mcp_<server>_<tool> convention.
	for _, def := range defs {
		if def.Name != "mcp_my_server_search" && def.Name != "mcp_my_server_fetch" {
			t.Errorf("unexpected tool name %q", def.Name)
		}
	}
}

// TestRegistry_RegisterMCPTools_DuplicateServer verifies RegisterMCPTools rejects duplicate server names.
func TestRegistry_RegisterMCPTools_DuplicateServer(t *testing.T) {
	r := NewRegistry()
	caller := &mockMCPReg{}

	toolDefs := []htools.MCPToolDefinition{
		{Name: "tool1", Description: "t1", Parameters: map[string]any{}},
	}

	if _, err := r.RegisterMCPTools("dup-server", toolDefs, caller); err != nil {
		t.Fatalf("first RegisterMCPTools failed: %v", err)
	}

	_, err := r.RegisterMCPTools("dup-server", toolDefs, caller)
	if err == nil {
		t.Fatal("expected error for duplicate server name")
	}
}

// TestRegistry_RegisterMCPTools_EmptyServerName verifies RegisterMCPTools rejects empty server name.
func TestRegistry_RegisterMCPTools_EmptyServerName(t *testing.T) {
	r := NewRegistry()
	caller := &mockMCPReg{}

	_, err := r.RegisterMCPTools("", []htools.MCPToolDefinition{}, caller)
	if err == nil {
		t.Fatal("expected error for empty server name")
	}
}

// TestRegistry_RegisterMCPTools_NilCaller verifies RegisterMCPTools rejects nil caller.
func TestRegistry_RegisterMCPTools_NilCaller(t *testing.T) {
	r := NewRegistry()

	_, err := r.RegisterMCPTools("my-server", []htools.MCPToolDefinition{}, nil)
	if err == nil {
		t.Fatal("expected error for nil caller")
	}
}

// TestRegistry_RegisterMCPTools_ExecuteTool verifies registered MCP tools are callable.
func TestRegistry_RegisterMCPTools_ExecuteTool(t *testing.T) {
	r := NewRegistry()
	caller := &mockMCPReg{}

	toolDefs := []htools.MCPToolDefinition{
		{Name: "search", Description: "Search", Parameters: map[string]any{}},
	}

	if _, err := r.RegisterMCPTools("exec-server", toolDefs, caller); err != nil {
		t.Fatalf("RegisterMCPTools failed: %v", err)
	}

	result, err := r.Execute(context.Background(), "mcp_exec_server_search", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result from MCP tool")
	}
}

// TestRegistry_RegisterMCPTools_NameSanitization verifies tool names are properly sanitized.
func TestRegistry_RegisterMCPTools_NameSanitization(t *testing.T) {
	r := NewRegistry()
	caller := &mockMCPReg{}

	toolDefs := []htools.MCPToolDefinition{
		{Name: "my-fancy.tool", Description: "Fancy", Parameters: map[string]any{}},
	}

	registered, err := r.RegisterMCPTools("my-server.v2", toolDefs, caller)
	if err != nil {
		t.Fatalf("RegisterMCPTools failed: %v", err)
	}
	if len(registered) != 1 {
		t.Fatalf("expected 1 registered tool, got %d", len(registered))
	}
	if registered[0] != "mcp_my_server_v2_my_fancy_tool" {
		t.Errorf("unexpected tool name %q", registered[0])
	}
}

// TestRegistry_RegisterMCPTools_EmptyToolList verifies RegisterMCPTools handles empty tool lists.
func TestRegistry_RegisterMCPTools_EmptyToolList(t *testing.T) {
	r := NewRegistry()
	caller := &mockMCPReg{}

	registered, err := r.RegisterMCPTools("empty-server", []htools.MCPToolDefinition{}, caller)
	if err != nil {
		t.Fatalf("RegisterMCPTools failed: %v", err)
	}
	if len(registered) != 0 {
		t.Errorf("expected 0 registered tools, got %d", len(registered))
	}
}

// TestRegistry_RegisterMCPTools_Concurrency verifies RegisterMCPTools is safe under concurrent access.
func TestRegistry_RegisterMCPTools_Concurrency(t *testing.T) {
	r := NewRegistry()
	caller := &mockMCPReg{}

	const nGoroutines = 20
	var wg sync.WaitGroup
	errs := make(chan error, nGoroutines)

	for i := 0; i < nGoroutines; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			serverName := fmt.Sprintf("concurrent-server-%d", i)
			toolDefs := []htools.MCPToolDefinition{
				{Name: "tool", Description: "t", Parameters: map[string]any{}},
			}
			if _, err := r.RegisterMCPTools(serverName, toolDefs, caller); err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent RegisterMCPTools failed: %v", err)
	}

	// Verify all servers were registered.
	defs := r.DeferredDefinitions()
	if len(defs) != nGoroutines {
		t.Errorf("expected %d deferred tools, got %d", nGoroutines, len(defs))
	}
}
