package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	htools "go-agent-harness/internal/harness/tools"
)

// errorMCPRegistry is an htools.MCPRegistry that always returns an error from
// ListTools. It is used to verify non-fatal behaviour in NewDefaultRegistryWithOptions.
type errorMCPRegistry struct{}

func (e *errorMCPRegistry) ListTools(_ context.Context) (map[string][]htools.MCPToolDefinition, error) {
	return nil, fmt.Errorf("simulated MCP ListTools failure")
}

func (e *errorMCPRegistry) CallTool(_ context.Context, _, _ string, _ json.RawMessage) (string, error) {
	return "", fmt.Errorf("simulated MCP CallTool failure")
}

func (e *errorMCPRegistry) ListResources(_ context.Context, _ string) ([]htools.MCPResource, error) {
	return nil, nil
}

func (e *errorMCPRegistry) ReadResource(_ context.Context, server, _ string) (string, error) {
	return "", fmt.Errorf("resources not supported for %q", server)
}

// TestNewDefaultRegistryWithOptions_MCPRegistryError_NonFatal verifies that
// when the MCPRegistry.ListTools call fails during DynamicMCPTools discovery,
// NewDefaultRegistryWithOptions does NOT panic and returns a valid registry.
//
// Prior to the fix, this code path called panic(err). After the fix it calls
// log.Printf and continues, so we verify the registry is non-nil and usable.
func TestNewDefaultRegistryWithOptions_MCPRegistryError_NonFatal(t *testing.T) {
	t.Parallel()

	// This must not panic.
	registry := NewDefaultRegistryWithOptions("", DefaultRegistryOptions{
		MCPRegistry: &errorMCPRegistry{},
	})

	if registry == nil {
		t.Fatal("expected non-nil registry even when MCPRegistry.ListTools fails")
	}

	// The registry should have at least the core tools registered (e.g., read, write, bash).
	defs := registry.Definitions()
	if len(defs) == 0 {
		t.Error("expected core tools to be registered in the returned registry")
	}
}
