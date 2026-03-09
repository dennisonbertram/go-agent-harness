package deferred

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/descriptions"
)

// sanitizeToolNamePart normalizes a string for use in MCP tool names.
func sanitizeToolNamePart(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, ".", "_")
	if s == "" {
		return "x"
	}
	return s
}

// ListMCPResourcesTool returns a deferred tool for listing MCP resources.
func ListMCPResourcesTool(reg tools.MCPRegistry) tools.Tool {
	def := tools.Definition{
		Name:         "list_mcp_resources",
		Description:  descriptions.Load("list_mcp_resources"),
		Action:       tools.ActionList,
		ParallelSafe: true,
		Tier:         tools.TierDeferred,
		Tags:         []string{"mcp", "integration", "external"},
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"mcp_name": map[string]any{"type": "string"}},
			"required":   []string{"mcp_name"},
		},
	}
	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		args := struct {
			Name string `json:"mcp_name"`
		}{}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse list_mcp_resources args: %w", err)
		}
		if strings.TrimSpace(args.Name) == "" {
			return "", fmt.Errorf("mcp_name is required")
		}
		items, err := reg.ListResources(ctx, args.Name)
		if err != nil {
			return "", err
		}
		return tools.MarshalToolResult(map[string]any{"mcp_name": args.Name, "resources": items})
	}
	return tools.Tool{Definition: def, Handler: handler}
}

// ReadMCPResourceTool returns a deferred tool for reading an MCP resource.
func ReadMCPResourceTool(reg tools.MCPRegistry) tools.Tool {
	def := tools.Definition{
		Name:         "read_mcp_resource",
		Description:  descriptions.Load("read_mcp_resource"),
		Action:       tools.ActionRead,
		ParallelSafe: true,
		Tier:         tools.TierDeferred,
		Tags:         []string{"mcp", "integration", "external"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mcp_name": map[string]any{"type": "string"},
				"uri":      map[string]any{"type": "string"},
			},
			"required": []string{"mcp_name", "uri"},
		},
	}
	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		args := struct {
			Name string `json:"mcp_name"`
			URI  string `json:"uri"`
		}{}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse read_mcp_resource args: %w", err)
		}
		if strings.TrimSpace(args.Name) == "" || strings.TrimSpace(args.URI) == "" {
			return "", fmt.Errorf("mcp_name and uri are required")
		}
		content, err := reg.ReadResource(ctx, args.Name, args.URI)
		if err != nil {
			return "", err
		}
		return tools.MarshalToolResult(map[string]any{"mcp_name": args.Name, "uri": args.URI, "content": content})
	}
	return tools.Tool{Definition: def, Handler: handler}
}

// DynamicMCPTools generates deferred tools dynamically from MCP server tool listings.
func DynamicMCPTools(ctx context.Context, reg tools.MCPRegistry) ([]tools.Tool, error) {
	byServer, err := reg.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]tools.Tool, 0)
	for server, defs := range byServer {
		for _, d := range defs {
			safeServer := sanitizeToolNamePart(server)
			safeName := sanitizeToolNamePart(d.Name)
			toolName := "mcp_" + safeServer + "_" + safeName
			server := server
			origName := d.Name
			definition := tools.Definition{
				Name:         toolName,
				Description:  d.Description,
				Action:       tools.ActionExecute,
				Mutating:     true,
				ParallelSafe: false,
				Tier:         tools.TierDeferred,
				Tags:         []string{"mcp", "integration", "external"},
				Parameters:   d.Parameters,
			}
			handler := func(ctx context.Context, args json.RawMessage) (string, error) {
				return reg.CallTool(ctx, server, origName, args)
			}
			result = append(result, tools.Tool{Definition: definition, Handler: handler})
		}
	}
	return result, nil
}
