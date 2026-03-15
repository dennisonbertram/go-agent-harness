# MCP Client: Config-Driven Server Startup

## Status

Draft — not yet scheduled.

## Problem

MCP servers can only be registered in two ways today:

1. **In code** — calling `ClientManager.AddServer` directly during server
   initialisation in `cmd/harnessd/main.go` or equivalent wiring.
2. **At runtime** — via the `connect_mcp` deferred tool, which requires the LLM to
   spend a turn connecting the server.

Neither mechanism allows an operator to configure MCP servers without modifying Go
source code and rebuilding the binary. There is no config file support and no
environment variable support. This means:

- Deploying harnessd with a new MCP server integration requires a code change and
  redeploy.
- Environment-specific server sets (staging vs. production, per-tenant configs) are
  not possible without custom builds.
- Operators running harnessd as a managed service cannot wire MCP servers without
  Go build access.

## Solution

Support config-driven MCP server registration via two mechanisms:

1. **`HARNESS_MCP_SERVERS` environment variable** — a JSON array of server
   configurations parsed at startup.
2. **`mcp_servers` key in the YAML/JSON config file** — if harnessd supports a
   config file (determined by reading existing config file support; if absent, only
   the env var path is implemented).

Both mechanisms feed into the same startup path: the parsed `[]MCPServerConfig`
slice is passed to `ClientManager.AddServer` for each entry. Connections are lazy
(not established until first use), so an unavailable server at startup does not
prevent harnessd from starting.

Invalid or unparseable entries log a warning and are skipped; they do not crash
startup.

## Config Format

### Environment Variable: `HARNESS_MCP_SERVERS`

Value is a JSON array. Each element is a flat object:

```json
[
  {
    "name": "filesystem",
    "transport": "stdio",
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  },
  {
    "name": "github",
    "transport": "http",
    "url": "https://api.github.com/mcp"
  },
  {
    "name": "memory",
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-memory"]
  }
]
```

Field semantics:

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | Yes | Unique server identifier |
| `transport` | string | No | `"stdio"` or `"http"`. Inferred from presence of `command` vs `url` if omitted. |
| `command` | string | Conditional | Executable path for stdio transport |
| `args` | []string | No | Arguments for stdio executable |
| `url` | string | Conditional | HTTP endpoint for http transport |

Transport inference rule (when `transport` field is absent):
- If `command` is set → `"stdio"`.
- If `url` is set → `"http"`.
- If both or neither → validation error (entry is skipped with a warning).

This is a convenience for operators who find the `transport` field redundant when
`command` and `url` are mutually exclusive indicators.

### Config File: `mcp_servers` Key

If harnessd supports a YAML config file (look for existing config file loading in
`cmd/harnessd/`), add an `mcp_servers` array at the top level:

```yaml
mcp_servers:
  - name: filesystem
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  - name: github
    transport: http
    url: https://api.github.com/mcp
```

The YAML schema mirrors the JSON env var schema. The same validation and inference
rules apply.

**Merge behaviour**: Config file entries and env var entries are merged. If the same
server name appears in both, the env var entry takes precedence (env vars override
config file). Duplicate names within the same source produce a warning and only the
first occurrence is used.

## New Code

### `internal/mcp/config.go` (new file)

```go
package mcp

import (
    "encoding/json"
    "fmt"
    "log"
    "os"
    "strings"
)

// EnvVarMCPServers is the environment variable name for JSON-encoded MCP server configs.
const EnvVarMCPServers = "HARNESS_MCP_SERVERS"

// ParseMCPServersEnv reads HARNESS_MCP_SERVERS from the environment and returns
// the parsed configs. Returns an empty slice (not an error) if the variable is
// unset or empty. Invalid entries are logged and skipped; valid entries are
// returned. Never returns a non-nil error for missing or empty env var.
func ParseMCPServersEnv() ([]ServerConfig, error) {
    raw := strings.TrimSpace(os.Getenv(EnvVarMCPServers))
    if raw == "" {
        return nil, nil
    }
    return parseMCPServersJSON(raw)
}

// parseMCPServersJSON parses a JSON array of server configs.
// Invalid individual entries are logged and skipped. The function returns an
// error only if the outer JSON is not a valid array (unparseable at the top level).
func parseMCPServersJSON(raw string) ([]ServerConfig, error) {
    var raw_entries []json.RawMessage
    if err := json.Unmarshal([]byte(raw), &raw_entries); err != nil {
        return nil, fmt.Errorf("mcp: HARNESS_MCP_SERVERS is not a valid JSON array: %w", err)
    }
    var out []ServerConfig
    for i, entry := range raw_entries {
        cfg, err := parseSingleServerConfig(entry)
        if err != nil {
            log.Printf("mcp: skipping HARNESS_MCP_SERVERS[%d]: %v", i, err)
            continue
        }
        out = append(out, cfg)
    }
    return out, nil
}

// parseSingleServerConfig parses and validates a single server config entry.
func parseSingleServerConfig(raw json.RawMessage) (ServerConfig, error) {
    var m struct {
        Name      string   `json:"name"`
        Transport string   `json:"transport"`
        Command   string   `json:"command"`
        Args      []string `json:"args"`
        URL       string   `json:"url"`
    }
    if err := json.Unmarshal(raw, &m); err != nil {
        return ServerConfig{}, fmt.Errorf("invalid JSON: %w", err)
    }
    if strings.TrimSpace(m.Name) == "" {
        return ServerConfig{}, fmt.Errorf("name is required")
    }
    // Infer transport if not explicitly set.
    transport := strings.TrimSpace(m.Transport)
    if transport == "" {
        if m.Command != "" && m.URL == "" {
            transport = "stdio"
        } else if m.URL != "" && m.Command == "" {
            transport = "http"
        } else if m.Command != "" && m.URL != "" {
            return ServerConfig{}, fmt.Errorf("server %q: command and url are mutually exclusive", m.Name)
        } else {
            return ServerConfig{}, fmt.Errorf("server %q: one of command or url is required", m.Name)
        }
    }
    cfg := ServerConfig{
        Name:      m.Name,
        Transport: transport,
        Command:   m.Command,
        Args:      m.Args,
        URL:       m.URL,
    }
    // Delegate structural validation to AddServer's existing checks.
    return cfg, validateServerConfig(cfg)
}

// validateServerConfig checks that the config is internally consistent.
// This mirrors the checks in ClientManager.AddServer but is callable without a manager.
func validateServerConfig(cfg ServerConfig) error {
    switch cfg.Transport {
    case "stdio":
        if cfg.Command == "" {
            return fmt.Errorf("server %q: stdio transport requires command", cfg.Name)
        }
    case "http":
        if cfg.URL == "" {
            return fmt.Errorf("server %q: http transport requires url", cfg.Name)
        }
    default:
        return fmt.Errorf("server %q: unsupported transport %q", cfg.Name, cfg.Transport)
    }
    return nil
}
```

### `internal/mcp/config_yaml.go` (new file, conditional on config file support)

If harnessd already has a YAML config struct (check `cmd/harnessd/` for an existing
config loader), add a `MCPServers []ServerConfigYAML` field to that struct. If no
config file support exists yet, this file is deferred and only the env var path is
implemented in this issue.

```go
package mcp

// ParseMCPServersYAML parses mcp_servers entries from a YAML config.
// entries is the raw decoded slice from the YAML parser ([]map[string]interface{}).
// Invalid entries are logged and skipped. Returns empty slice (not error) if entries is nil.
func ParseMCPServersYAML(entries []map[string]interface{}) ([]ServerConfig, error)
```

### `cmd/harnessd/main.go` (or server wiring file)

```go
// Load MCP servers from environment.
mcpCfgs, err := mcp.ParseMCPServersEnv()
if err != nil {
    // Outer-level parse error (malformed JSON array) — log and continue with empty list.
    log.Printf("warning: HARNESS_MCP_SERVERS parse error: %v; no MCP servers loaded from env", err)
    mcpCfgs = nil
}
for _, cfg := range mcpCfgs {
    if addErr := clientManager.AddServer(cfg); addErr != nil {
        // Duplicate name or other registration error — log and skip.
        log.Printf("warning: MCP server %q registration failed: %v; skipping", cfg.Name, addErr)
    }
}
```

The `ClientManager` is created before this block and shared across the application
as before.

## Error Handling Policy

| Situation | Behaviour |
|---|---|
| `HARNESS_MCP_SERVERS` unset or empty | No action; proceed with no env-configured servers |
| `HARNESS_MCP_SERVERS` is not valid JSON array | Log warning; proceed with no env-configured servers |
| Individual entry missing `name` | Log warning (include index); skip entry; continue |
| Individual entry missing `command` and `url` | Log warning; skip entry; continue |
| Individual entry has both `command` and `url` | Log warning; skip entry; continue |
| Individual entry has unsupported transport | Log warning; skip entry; continue |
| Duplicate `name` between two env entries | Log warning for duplicate; use first occurrence |
| Duplicate `name` collides with globally registered server | `AddServer` returns error; log warning; skip |
| Server unreachable at startup | Lazy connection; no startup error. Error surfaces on first `DiscoverTools` or `ExecuteTool`. |

In all cases above: harnessd starts normally. MCP server configuration errors are
non-fatal.

## Files Changed

- `internal/mcp/config.go` — `ParseMCPServersEnv`, `parseMCPServersJSON`,
  `parseSingleServerConfig`, `validateServerConfig`.
- `internal/mcp/config_yaml.go` — `ParseMCPServersYAML` (conditional).
- `internal/mcp/config_test.go` — all unit tests.
- `cmd/harnessd/main.go` (or equivalent server wiring) — call `ParseMCPServersEnv`
  and register resulting configs.

## Test Requirements

All tests must pass with `-race`.

### Unit Tests — `internal/mcp/config_test.go`

**`TestParseMCPServersEnv_EnvUnset_ReturnsEmpty`**
Unset `HARNESS_MCP_SERVERS`. Call `ParseMCPServersEnv`. Assert returns `(nil, nil)`.

**`TestParseMCPServersEnv_EnvEmpty_ReturnsEmpty`**
Set `HARNESS_MCP_SERVERS=""`. Call `ParseMCPServersEnv`. Assert returns `(nil, nil)`.

**`TestParseMCPServersEnv_ValidStdio`**
Set `HARNESS_MCP_SERVERS` to a JSON array with one stdio entry. Call
`ParseMCPServersEnv`. Assert returns one `ServerConfig` with `Transport="stdio"`,
correct `Command`, and correct `Args`.

**`TestParseMCPServersEnv_ValidHTTP`**
Set `HARNESS_MCP_SERVERS` to a JSON array with one http entry. Assert returns
`ServerConfig{Transport:"http", URL:"https://..."}`.

**`TestParseMCPServersEnv_TransportInferredFromCommand`**
Entry has `command` set, no `transport` field. Assert transport is inferred as
`"stdio"`.

**`TestParseMCPServersEnv_TransportInferredFromURL`**
Entry has `url` set, no `transport` field. Assert transport is inferred as `"http"`.

**`TestParseMCPServersEnv_InvalidJSON_OuterArray_ReturnsError`**
Set `HARNESS_MCP_SERVERS="{not an array}"`. Call `ParseMCPServersEnv`. Assert returns
a non-nil error and empty slice.

**`TestParseMCPServersEnv_MixedValidInvalid_ValidOnesLoaded`**
Set `HARNESS_MCP_SERVERS` to a 3-entry array: entry 0 valid, entry 1 missing `name`,
entry 2 valid. Assert `ParseMCPServersEnv` returns exactly 2 `ServerConfig` entries
(entries 0 and 2). Assert no error is returned (the invalid entry is only logged).

**`TestParseMCPServersEnv_MissingName_Skipped`**
Entry has no `name`. Assert skipped (not present in output).

**`TestParseMCPServersEnv_MissingCommandAndURL_Skipped`**
Entry has `name` but no `command` and no `url`. Assert skipped.

**`TestParseMCPServersEnv_BothCommandAndURL_Skipped`**
Entry has `name`, `command`, and `url`. Assert skipped (mutually exclusive).

**`TestParseMCPServersEnv_UnsupportedTransport_Skipped`**
Entry has `transport: "websocket"`. Assert skipped.

**`TestParseMCPServersEnv_ArgsPreserved`**
Stdio entry with `args: ["-y", "pkg", "/path"]`. Assert `ServerConfig.Args` equals
`["-y", "pkg", "/path"]` exactly.

**`TestParseMCPServersEnv_EmptyArray_ReturnsEmpty`**
Set `HARNESS_MCP_SERVERS=[]`. Assert returns `(nil, nil)` or `([], nil)`.

### Integration Test

**`TestHarnessStartup_MCPServersFromEnv`**
This test sets `HARNESS_MCP_SERVERS` to a JSON array pointing at a local
`httptest.Server`. Creates a `ClientManager`, calls `ParseMCPServersEnv`, registers
the result. Calls `DiscoverTools` on the registered server. Assert tools are returned.
This validates the full env-var → registration → discovery path end-to-end.

**`TestHarnessStartup_NoMCPServersEnv_StartsCleanly`**
Unset `HARNESS_MCP_SERVERS`. Create `ClientManager`, call `ParseMCPServersEnv`,
register result. Assert `ClientManager.ListServers()` is empty (no servers
registered). Assert no error. Validates backward compatibility.

### Regression Tests

**`TestProgrammaticRegistration_StillWorks`**
Call `ClientManager.AddServer(ServerConfig{...})` directly (existing code path).
Assert `DiscoverTools` works. The new config-loading code must not interfere with
programmatic registration.

**`TestExistingMCPTests_PassUnchanged`**
Run the full `internal/mcp/` test suite (`go test ./internal/mcp/... -race`). All
tests that existed before this feature must pass unchanged. This is the regression
guard.

### YAML Tests (if YAML config is in scope)

**`TestParseMCPServersYAML_ValidEntries`**
Pass a `[]map[string]interface{}` with valid entries. Assert parsed correctly.

**`TestParseMCPServersYAML_InvalidEntry_Skipped`**
Pass a mix of valid and invalid entries. Assert invalid entries are skipped.

**`TestParseMCPServersYAML_Nil_ReturnsEmpty`**
Pass `nil`. Assert `(nil, nil)`.

**`TestParseMCPServersYAML_EnvOverridesYAML`**
Register a server from YAML, then attempt to register the same name from env. Assert
env entry wins (first attempt with the name wins in `AddServer`; the YAML entry
would be skipped with a log message, or the env entry would shadow it depending on
merge order). The merge order is: env vars processed after config file; if
`AddServer` returns an error for duplicate, log and skip.

## Acceptance Criteria

1. `go test ./internal/mcp/... -race` passes with all new tests green.
2. `go test ./... -race` passes.
3. Coverage on `internal/mcp` does not drop below 80%; no 0%-covered functions.
4. Setting `HARNESS_MCP_SERVERS` to a valid JSON array causes harnessd to register
   those servers at startup (verified by integration test).
5. Invalid JSON in `HARNESS_MCP_SERVERS` logs a warning and does not crash startup.
6. Individual invalid entries in `HARNESS_MCP_SERVERS` are skipped with a warning;
   valid entries are still loaded.
7. Harnessd starts cleanly with `HARNESS_MCP_SERVERS` unset (existing behaviour
   unchanged).
8. Programmatically registered servers (current code path) still work after this change.
