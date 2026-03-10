package deferred

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/descriptions"
)

// DynamicToolRegistrar is implemented by the harness Registry.
// It allows connect_mcp to register new tools at runtime after session startup.
type DynamicToolRegistrar interface {
	// RegisterMCPTools registers a set of MCP-proxied tools under the given server name.
	// Returns the names of the tools that were registered.
	// Returns an error if the server name is already registered.
	RegisterMCPTools(serverName string, toolDefs []tools.MCPToolDefinition, caller tools.MCPRegistry) ([]string, error)
}

// MCPConnector opens a connection to an MCP server at the given URL and
// returns an MCPRegistry scoped to that single server.
// The returned registry must support ListTools and CallTool for the named server.
type MCPConnector interface {
	// Connect establishes a connection to the given HTTP/SSE MCP server URL.
	// serverName is the logical name to use when querying via the returned registry.
	Connect(ctx context.Context, serverURL, serverName string) (tools.MCPRegistry, error)
}

// ConnectMCPTool returns a deferred tool that connects to a new MCP server
// mid-session and registers its tools into the running registry.
func ConnectMCPTool(registrar DynamicToolRegistrar, connector MCPConnector) tools.Tool {
	def := tools.Definition{
		Name:         "connect_mcp",
		Description:  descriptions.Load("connect_mcp"),
		Action:       tools.ActionExecute,
		Mutating:     true,
		ParallelSafe: false,
		Tier:         tools.TierDeferred,
		Tags:         []string{"mcp", "integration", "external", "connect"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "HTTP/SSE MCP server URL (e.g. http://localhost:3000/mcp)",
				},
				"server_name": map[string]any{
					"type":        "string",
					"description": "Optional display name for this MCP server (derived from URL if omitted)",
				},
			},
			"required":             []string{"url"},
			"additionalProperties": false,
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			URL        string `json:"url"`
			ServerName string `json:"server_name"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse connect_mcp args: %w", err)
		}

		rawURL := strings.TrimSpace(args.URL)
		if rawURL == "" {
			return "", fmt.Errorf("url is required")
		}

		// Validate the URL scheme.
		parsed, err := url.Parse(rawURL)
		if err != nil {
			return "", fmt.Errorf("invalid url %q: %w", rawURL, err)
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return "", fmt.Errorf("unsupported scheme %q: only http and https are supported", parsed.Scheme)
		}

		// Derive a server name from the URL if not provided.
		serverName := strings.TrimSpace(args.ServerName)
		if serverName == "" {
			serverName = deriveServerName(parsed)
		}
		if err := validateServerName(serverName); err != nil {
			return "", err
		}

		// Connect to the MCP server.
		mcpReg, err := connector.Connect(ctx, rawURL, serverName)
		if err != nil {
			return "", fmt.Errorf("connect_mcp: failed to connect to %q: %w", rawURL, err)
		}

		// Discover the server's tools.
		byServer, err := mcpReg.ListTools(ctx)
		if err != nil {
			return "", fmt.Errorf("connect_mcp: failed to list tools from %q: %w", rawURL, err)
		}

		// Find tools for this server.
		var toolDefs []tools.MCPToolDefinition
		for name, defs := range byServer {
			if sanitizeToolNamePart(name) == sanitizeToolNamePart(serverName) || name == serverName {
				toolDefs = defs
				break
			}
		}
		// If no exact match, collect all tools (single-server registry).
		if len(toolDefs) == 0 {
			for _, defs := range byServer {
				toolDefs = append(toolDefs, defs...)
			}
		}

		if len(toolDefs) == 0 {
			return tools.MarshalToolResult(map[string]any{
				"server_name":      serverName,
				"tools_registered": []string{},
				"count":            0,
				"message":          "Connected successfully but no tools were discovered on the server.",
			})
		}

		// Register the tools into the dynamic registrar.
		registered, err := registrar.RegisterMCPTools(serverName, toolDefs, mcpReg)
		if err != nil {
			return "", fmt.Errorf("connect_mcp: failed to register tools from %q: %w", rawURL, err)
		}

		return tools.MarshalToolResult(map[string]any{
			"server_name":      serverName,
			"tools_registered": registered,
			"count":            len(registered),
		})
	}

	return tools.Tool{Definition: def, Handler: handler}
}

// deriveServerName creates a short server name from a URL.
// It uses the hostname, stripping port and replacing dots with underscores.
func deriveServerName(u *url.URL) string {
	host := u.Hostname() // strips port
	// Replace dots, hyphens with underscores; take first segment of path if helpful
	name := sanitizeToolNamePart(host)
	if name == "" || name == "x" {
		name = "mcp_server"
	}
	return name
}

// validateServerName checks that a server name is suitable for use as a tool name prefix.
func validateServerName(name string) error {
	if name == "" {
		return fmt.Errorf("server_name must not be empty")
	}
	for _, ch := range name {
		if !isAlphanumOrUnderscore(ch) && ch != '-' {
			return fmt.Errorf("server_name %q contains invalid character %q: only alphanumeric, hyphens, and underscores are allowed", name, ch)
		}
	}
	return nil
}

func isAlphanumOrUnderscore(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}
