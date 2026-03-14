package main

import (
	"go-agent-harness/internal/config"
	"go-agent-harness/internal/mcp"
)

// registerMCPServersFromConfig registers MCP servers from TOML config and env
// var sources into the given ClientManager.
//
// Registration order:
//  1. TOML servers are registered first (they take precedence).
//  2. Env var servers are registered next; if a name collision occurs, the env
//     var entry is skipped and a log message is emitted via logf.
//
// Transport inference: if a TOML server's Transport field is empty, it is
// inferred from the other fields — "http" when URL is non-empty, "stdio"
// otherwise.
func registerMCPServersFromConfig(
	manager *mcp.ClientManager,
	tomlServers map[string]config.MCPServerConfig,
	envServers []mcp.ServerConfig,
	logf func(string, ...any),
) {
	// Register TOML servers first.
	for name, srv := range tomlServers {
		transport := srv.Transport
		if transport == "" {
			if srv.URL != "" {
				transport = "http"
			} else {
				transport = "stdio"
			}
		}
		sc := mcp.ServerConfig{
			Name:      name,
			Transport: transport,
			Command:   srv.Command,
			Args:      srv.Args,
			URL:       srv.URL,
		}
		if addErr := manager.AddServer(sc); addErr != nil {
			logf("warning: failed to register TOML MCP server %q: %v", name, addErr)
		} else {
			logf("registered MCP server %q from config (transport=%s)", name, transport)
		}
	}

	// Register env var servers, skipping any names already registered from TOML.
	registered := make(map[string]struct{}, len(tomlServers))
	for name := range tomlServers {
		registered[name] = struct{}{}
	}
	for _, sc := range envServers {
		if _, exists := registered[sc.Name]; exists {
			logf("mcp: skipping env var server %q: already registered from TOML config", sc.Name)
			continue
		}
		if addErr := manager.AddServer(sc); addErr != nil {
			logf("warning: failed to register MCP server %q: %v", sc.Name, addErr)
		} else {
			logf("registered MCP server %q (transport=%s)", sc.Name, sc.Transport)
		}
	}
}
