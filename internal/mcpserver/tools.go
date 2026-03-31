package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"go-agent-harness/internal/harness/tools"
)

// BridgeTools converts a slice of harness tools into MCP ServerTools.
// Both TierCore and TierDeferred tools are included — MCP clients manage
// their own context window, so all tools are exposed.
func BridgeTools(catalog []tools.Tool) []mcpserver.ServerTool {
	result := make([]mcpserver.ServerTool, 0, len(catalog))

	for _, t := range catalog {
		mcpTool := buildMCPTool(t.Definition)
		handler := buildMCPHandler(t.Handler)

		result = append(result, mcpserver.ServerTool{
			Tool:    mcpTool,
			Handler: handler,
		})
	}

	return result
}

// buildMCPTool converts a harness tool Definition into an mcp.Tool.
// The parameters JSON schema is serialized and used as the raw input schema
// so that nested objects and all schema keywords are faithfully preserved.
func buildMCPTool(def tools.Definition) mcplib.Tool {
	if def.Parameters == nil {
		return mcplib.NewToolWithRawSchema(def.Name, def.Description, json.RawMessage(`{"type":"object"}`))
	}

	schemaBytes, err := json.Marshal(def.Parameters)
	if err != nil {
		// Fallback to empty object schema on marshal failure.
		schemaBytes = json.RawMessage(`{"type":"object"}`)
	}

	return mcplib.NewToolWithRawSchema(def.Name, def.Description, schemaBytes)
}

// buildMCPHandler wraps a harness tool Handler in an MCP ToolHandlerFunc.
// Handler errors are encoded as MCP error results (IsError: true) rather than
// returned as Go errors, so the MCP client can observe and react to them.
// A nil Handler is guarded: calling it returns an MCP error result instead of
// panicking.
func buildMCPHandler(h tools.Handler) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		if h == nil {
			return mcplib.NewToolResultError("tool handler not configured"), nil
		}

		// Marshal the MCP arguments back to JSON so the harness handler
		// receives a json.RawMessage as it expects.
		var argsJSON json.RawMessage
		if req.Params.Arguments != nil {
			data, err := json.Marshal(req.Params.Arguments)
			if err != nil {
				return mcplib.NewToolResultError(
					fmt.Sprintf("failed to marshal tool arguments: %v", err),
				), nil
			}
			argsJSON = data
		} else {
			argsJSON = json.RawMessage(`{}`)
		}

		result, err := h(ctx, argsJSON)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		return mcplib.NewToolResultText(result), nil
	}
}
