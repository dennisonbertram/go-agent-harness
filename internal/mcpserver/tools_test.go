package mcpserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/mcpserver"
)

// BT-001: When BuildCatalog returns N tools, MCP server registers exactly N
// tools with matching names and descriptions.
func TestBridgeToolsCountAndNames(t *testing.T) {
	fakeCatalog := []tools.Tool{
		{
			Definition: tools.Definition{
				Name:        "tool_alpha",
				Description: "Alpha does things",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"input": map[string]any{"type": "string"},
					},
				},
			},
			Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
				return "alpha result", nil
			},
		},
		{
			Definition: tools.Definition{
				Name:        "tool_beta",
				Description: "Beta does other things",
				Parameters:  map[string]any{"type": "object"},
			},
			Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
				return "beta result", nil
			},
		},
	}

	serverTools := mcpserver.BridgeTools(fakeCatalog)

	require.Len(t, serverTools, 2, "expected exactly 2 MCP tools for 2 harness tools")

	names := make(map[string]string)
	for _, st := range serverTools {
		names[st.Tool.Name] = st.Tool.Description
	}

	assert.Equal(t, "Alpha does things", names["tool_alpha"], "tool_alpha description mismatch")
	assert.Equal(t, "Beta does other things", names["tool_beta"], "tool_beta description mismatch")
}

// BT-002: When an MCP client calls a tool via CallToolRequest, the bridge invokes
// the corresponding harness Handler with correct JSON arguments and returns the
// result as ToolResultText.
func TestBridgeHandlerInvocation(t *testing.T) {
	var capturedArgs json.RawMessage
	fakeCatalog := []tools.Tool{
		{
			Definition: tools.Definition{
				Name:        "echo_tool",
				Description: "Echoes input",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"message": map[string]any{"type": "string"},
					},
				},
			},
			Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
				capturedArgs = args
				var params map[string]string
				if err := json.Unmarshal(args, &params); err != nil {
					return "", err
				}
				return "echo: " + params["message"], nil
			},
		},
	}

	serverTools := mcpserver.BridgeTools(fakeCatalog)
	require.Len(t, serverTools, 1)

	req := mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name: "echo_tool",
			Arguments: map[string]any{
				"message": "hello world",
			},
		},
	}

	result, err := serverTools[0].Handler(context.Background(), req)
	require.NoError(t, err, "bridge handler should not return a Go error")
	require.NotNil(t, result)
	assert.False(t, result.IsError, "result should not be an MCP error")

	// Verify content
	require.Len(t, result.Content, 1)
	textContent, ok := mcplib.AsTextContent(result.Content[0])
	require.True(t, ok, "result content should be TextContent")
	assert.Equal(t, "echo: hello world", textContent.Text)

	// Verify args were passed correctly
	require.NotNil(t, capturedArgs)
	var parsedArgs map[string]string
	require.NoError(t, json.Unmarshal(capturedArgs, &parsedArgs))
	assert.Equal(t, "hello world", parsedArgs["message"])
}

// BT-003: When a harness tool handler returns an error, the MCP bridge returns
// an MCP error response (not a crash).
func TestBridgeHandlerErrorReturnsIsError(t *testing.T) {
	fakeCatalog := []tools.Tool{
		{
			Definition: tools.Definition{
				Name:        "failing_tool",
				Description: "Always fails",
				Parameters:  map[string]any{"type": "object"},
			},
			Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
				return "", errors.New("intentional failure")
			},
		},
	}

	serverTools := mcpserver.BridgeTools(fakeCatalog)
	require.Len(t, serverTools, 1)

	req := mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name:      "failing_tool",
			Arguments: map[string]any{},
		},
	}

	result, err := serverTools[0].Handler(context.Background(), req)
	// The bridge must NOT return a Go error — it must encode it in result.IsError
	require.NoError(t, err, "bridge must not return a Go error for handler failures")
	require.NotNil(t, result)
	assert.True(t, result.IsError, "result.IsError should be true when harness handler fails")

	require.Len(t, result.Content, 1)
	textContent, ok := mcplib.AsTextContent(result.Content[0])
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "intentional failure")
}

// BT-006: Tool parameter schemas with nested objects are faithfully converted
// (not lossy).
func TestBridgePreservesNestedParameterSchema(t *testing.T) {
	nestedSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"config": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"timeout": map[string]any{"type": "integer", "description": "timeout in seconds"},
					"retries": map[string]any{"type": "integer", "description": "retry count"},
				},
				"required": []any{"timeout"},
			},
		},
		"required": []any{"config"},
	}

	fakeCatalog := []tools.Tool{
		{
			Definition: tools.Definition{
				Name:        "nested_tool",
				Description: "Tool with nested params",
				Parameters:  nestedSchema,
			},
			Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
				return "ok", nil
			},
		},
	}

	serverTools := mcpserver.BridgeTools(fakeCatalog)
	require.Len(t, serverTools, 1)

	// Serialize the MCP tool to JSON and deserialize to check schema is preserved
	toolJSON, err := json.Marshal(serverTools[0].Tool)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(toolJSON, &decoded))

	// The inputSchema must contain the nested structure
	inputSchema, ok := decoded["inputSchema"]
	require.True(t, ok, "inputSchema field must be present")

	schemaBytes, err := json.Marshal(inputSchema)
	require.NoError(t, err)
	var schemaMap map[string]any
	require.NoError(t, json.Unmarshal(schemaBytes, &schemaMap))

	props, ok := schemaMap["properties"].(map[string]any)
	require.True(t, ok, "properties must be a map")
	configProp, ok := props["config"].(map[string]any)
	require.True(t, ok, "config property must be present")
	assert.Equal(t, "object", configProp["type"], "nested config property type must be preserved")

	nestedProps, ok := configProp["properties"].(map[string]any)
	require.True(t, ok, "nested properties must be present")
	assert.Contains(t, nestedProps, "timeout", "nested timeout field must be preserved")
	assert.Contains(t, nestedProps, "retries", "nested retries field must be preserved")
}

// Regression: Both TierCore and TierDeferred tools are included in the bridge
// (MCP clients manage their own context).
func TestBridgeIncludesBothTiers(t *testing.T) {
	fakeCatalog := []tools.Tool{
		{
			Definition: tools.Definition{
				Name:        "core_tool",
				Description: "Always visible",
				Parameters:  map[string]any{"type": "object"},
				Tier:        tools.TierCore,
			},
			Handler: func(ctx context.Context, args json.RawMessage) (string, error) { return "", nil },
		},
		{
			Definition: tools.Definition{
				Name:        "deferred_tool",
				Description: "Normally hidden",
				Parameters:  map[string]any{"type": "object"},
				Tier:        tools.TierDeferred,
			},
			Handler: func(ctx context.Context, args json.RawMessage) (string, error) { return "", nil },
		},
	}

	serverTools := mcpserver.BridgeTools(fakeCatalog)
	assert.Len(t, serverTools, 2, "both core and deferred tools must be bridged")
}

// BT-Fix3: When a tool has nil Handler and is called via MCP, the bridge returns
// an MCP error result (IsError:true) instead of panicking.
func TestBridgeNilHandlerReturnsErrorNotPanic(t *testing.T) {
	fakeCatalog := []tools.Tool{
		{
			Definition: tools.Definition{
				Name:        "nil_handler_tool",
				Description: "Tool with no handler configured",
				Parameters:  map[string]any{"type": "object"},
			},
			Handler: nil, // explicitly nil
		},
	}

	serverTools := mcpserver.BridgeTools(fakeCatalog)
	require.Len(t, serverTools, 1)

	req := mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name:      "nil_handler_tool",
			Arguments: map[string]any{},
		},
	}

	// Must not panic, must return an error result.
	assert.NotPanics(t, func() {
		result, err := serverTools[0].Handler(context.Background(), req)
		require.NoError(t, err, "bridge must not return a Go error")
		require.NotNil(t, result)
		assert.True(t, result.IsError, "result.IsError must be true when handler is nil")

		require.Len(t, result.Content, 1)
		textContent, ok := mcplib.AsTextContent(result.Content[0])
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "not configured")
	})
}

// Regression: nil Parameters map does not panic.
func TestBridgeNilParametersDoesNotPanic(t *testing.T) {
	fakeCatalog := []tools.Tool{
		{
			Definition: tools.Definition{
				Name:        "no_params_tool",
				Description: "No parameters",
				Parameters:  nil,
			},
			Handler: func(ctx context.Context, args json.RawMessage) (string, error) { return "ok", nil },
		},
	}

	assert.NotPanics(t, func() {
		serverTools := mcpserver.BridgeTools(fakeCatalog)
		require.Len(t, serverTools, 1)
	})
}
