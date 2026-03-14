package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"

	htools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/mcp"
)

// ScopedMCPRegistry combines a global MCPRegistry with a per-run set of MCP
// servers. Per-run servers shadow global ones with the same name. The per-run
// ClientManager is closed when Close is called; the global registry is left
// untouched.
type ScopedMCPRegistry struct {
	global      htools.MCPRegistry
	perRun      *mcp.ClientManager
	perRunNames map[string]struct{}
	mu          sync.RWMutex
	closed      bool
}

// NewScopedMCPRegistry creates a ScopedMCPRegistry wrapping the given global
// registry (may be nil) and a per-run ClientManager with the given server names.
func NewScopedMCPRegistry(global htools.MCPRegistry, perRun *mcp.ClientManager, perRunNames []string) *ScopedMCPRegistry {
	names := make(map[string]struct{}, len(perRunNames))
	for _, n := range perRunNames {
		names[n] = struct{}{}
	}
	return &ScopedMCPRegistry{
		global:      global,
		perRun:      perRun,
		perRunNames: names,
	}
}

// isPerRun reports whether the given server name belongs to the per-run set.
func (s *ScopedMCPRegistry) isPerRun(server string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.perRunNames[server]
	return ok
}

// ListTools returns the union of global and per-run tools. Per-run servers
// shadow global servers with the same name.
func (s *ScopedMCPRegistry) ListTools(ctx context.Context) (map[string][]htools.MCPToolDefinition, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, fmt.Errorf("scoped MCP registry is closed")
	}
	s.mu.RUnlock()

	result := make(map[string][]htools.MCPToolDefinition)

	// Start with global tools (if available).
	if s.global != nil {
		globalTools, err := s.global.ListTools(ctx)
		if err != nil {
			return nil, fmt.Errorf("list global MCP tools: %w", err)
		}
		for k, v := range globalTools {
			result[k] = v
		}
	}

	// Overlay per-run tools (shadow global servers with same name).
	servers := s.perRun.ListServers()
	for _, serverName := range servers {
		defs, err := s.perRun.DiscoverTools(ctx, serverName)
		if err != nil {
			return nil, fmt.Errorf("list per-run MCP tools for %q: %w", serverName, err)
		}
		toolDefs := make([]htools.MCPToolDefinition, 0, len(defs))
		for _, d := range defs {
			params := make(map[string]any)
			if d.InputSchema != nil {
				_ = json.Unmarshal(d.InputSchema, &params)
			}
			toolDefs = append(toolDefs, htools.MCPToolDefinition{
				Name:        d.Name,
				Description: d.Description,
				Parameters:  params,
			})
		}
		result[serverName] = toolDefs
	}

	return result, nil
}

// CallTool routes to the per-run ClientManager if the server belongs to the
// per-run set, otherwise delegates to the global registry.
func (s *ScopedMCPRegistry) CallTool(ctx context.Context, server, tool string, args json.RawMessage) (string, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return "", fmt.Errorf("scoped MCP registry is closed")
	}
	s.mu.RUnlock()

	if s.isPerRun(server) {
		return s.perRun.ExecuteTool(ctx, server, tool, args)
	}
	if s.global != nil {
		return s.global.CallTool(ctx, server, tool, args)
	}
	return "", fmt.Errorf("MCP server %q not found", server)
}

// ListResources routes to the per-run ClientManager if the server belongs to
// the per-run set, otherwise delegates to the global registry.
func (s *ScopedMCPRegistry) ListResources(ctx context.Context, server string) ([]htools.MCPResource, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, fmt.Errorf("scoped MCP registry is closed")
	}
	s.mu.RUnlock()

	if s.isPerRun(server) {
		// ClientManager does not expose a ListResources method — return empty.
		return nil, nil
	}
	if s.global != nil {
		return s.global.ListResources(ctx, server)
	}
	return nil, fmt.Errorf("MCP server %q not found", server)
}

// ReadResource routes to the per-run ClientManager if the server belongs to
// the per-run set, otherwise delegates to the global registry.
func (s *ScopedMCPRegistry) ReadResource(ctx context.Context, server, uri string) (string, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return "", fmt.Errorf("scoped MCP registry is closed")
	}
	s.mu.RUnlock()

	if s.isPerRun(server) {
		// ClientManager does not expose a ReadResource method — return error.
		return "", fmt.Errorf("per-run MCP server %q does not support resources", server)
	}
	if s.global != nil {
		return s.global.ReadResource(ctx, server, uri)
	}
	return "", fmt.Errorf("MCP server %q not found", server)
}

// Close tears down the per-run ClientManager. It is idempotent and does not
// affect the global registry.
func (s *ScopedMCPRegistry) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.perRun.Close()
}

// validateMCPServerConfigs validates a slice of per-run MCP server configs.
// Each entry must have a non-empty name, exactly one of command or url set,
// and URL entries must use http or https scheme.
func validateMCPServerConfigs(servers []MCPServerConfig) error {
	seen := make(map[string]struct{}, len(servers))
	for i, s := range servers {
		name := strings.TrimSpace(s.Name)
		if name == "" {
			return fmt.Errorf("mcp_servers[%d]: name is required", i)
		}
		if _, dup := seen[name]; dup {
			return fmt.Errorf("mcp_servers[%d]: duplicate name %q", i, name)
		}
		seen[name] = struct{}{}

		hasCommand := strings.TrimSpace(s.Command) != ""
		hasURL := strings.TrimSpace(s.URL) != ""

		if !hasCommand && !hasURL {
			return fmt.Errorf("mcp_servers[%d] (%q): must specify either command or url", i, name)
		}
		if hasCommand && hasURL {
			return fmt.Errorf("mcp_servers[%d] (%q): cannot specify both command and url", i, name)
		}
		if hasURL {
			u, err := url.Parse(strings.TrimSpace(s.URL))
			if err != nil {
				return fmt.Errorf("mcp_servers[%d] (%q): invalid url: %w", i, name, err)
			}
			if u.Scheme != "http" && u.Scheme != "https" {
				return fmt.Errorf("mcp_servers[%d] (%q): url scheme must be http or https, got %q", i, name, u.Scheme)
			}
		}
	}
	return nil
}

// buildPerRunMCPRegistry creates a ScopedMCPRegistry from per-run configs.
// It creates a new ClientManager, registers each server, and returns the
// scoped registry. The caller must call Close() on the returned registry
// when the run completes.
//
// globalServerNames is the set of server names already registered globally.
// If a per-run name collides with a global name, an error is returned.
func buildPerRunMCPRegistry(global htools.MCPRegistry, configs []MCPServerConfig, globalServerNames []string) (*ScopedMCPRegistry, error) {
	globalSet := make(map[string]struct{}, len(globalServerNames))
	for _, n := range globalServerNames {
		globalSet[n] = struct{}{}
	}

	cm := mcp.NewClientManager()
	var names []string

	for _, cfg := range configs {
		name := strings.TrimSpace(cfg.Name)

		// Check for collision with global servers.
		if _, exists := globalSet[name]; exists {
			_ = cm.Close()
			return nil, fmt.Errorf("per-run MCP server %q collides with globally registered server", name)
		}

		// Determine transport.
		transport := "stdio"
		if strings.TrimSpace(cfg.URL) != "" {
			transport = "http"
		}

		serverCfg := mcp.ServerConfig{
			Name:      name,
			Transport: transport,
			Command:   strings.TrimSpace(cfg.Command),
			Args:      cfg.Args,
			URL:       strings.TrimSpace(cfg.URL),
		}
		if err := cm.AddServer(serverCfg); err != nil {
			_ = cm.Close()
			return nil, fmt.Errorf("register per-run MCP server %q: %w", name, err)
		}
		names = append(names, name)
	}

	return NewScopedMCPRegistry(global, cm, names), nil
}
