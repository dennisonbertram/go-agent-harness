package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func listMCPResourcesTool(reg MCPRegistry) Tool {
	def := Definition{
		Name:         "list_mcp_resources",
		Description:  "List resources for an MCP server",
		Action:       ActionList,
		ParallelSafe: true,
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
		return MarshalToolResult(map[string]any{"mcp_name": args.Name, "resources": items})
	}
	return Tool{Definition: def, Handler: handler}
}

func readMCPResourceTool(reg MCPRegistry) Tool {
	def := Definition{
		Name:         "read_mcp_resource",
		Description:  "Read a resource from an MCP server",
		Action:       ActionRead,
		ParallelSafe: true,
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
		return MarshalToolResult(map[string]any{"mcp_name": args.Name, "uri": args.URI, "content": content})
	}
	return Tool{Definition: def, Handler: handler}
}

func dynamicMCPTools(ctx context.Context, reg MCPRegistry) ([]Tool, error) {
	byServer, err := reg.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	tools := make([]Tool, 0)
	for server, defs := range byServer {
		for _, d := range defs {
			safeServer := sanitizeToolNamePart(server)
			safeName := sanitizeToolNamePart(d.Name)
			toolName := "mcp_" + safeServer + "_" + safeName
			server := server
			origName := d.Name
			definition := Definition{
				Name:         toolName,
				Description:  d.Description,
				Action:       ActionExecute,
				Mutating:     true,
				ParallelSafe: false,
				Parameters:   d.Parameters,
			}
			handler := func(ctx context.Context, args json.RawMessage) (string, error) {
				return reg.CallTool(ctx, server, origName, args)
			}
			tools = append(tools, Tool{Definition: definition, Handler: handler})
		}
	}
	return tools, nil
}

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
