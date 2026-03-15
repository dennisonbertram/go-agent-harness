# MCP Client: Per-Run MCP Server Configuration

## Status

Draft — not yet scheduled.

## Problem

`ClientManager` is instantiated once at server startup and shared across all runs.
MCP server connections registered at startup are global: every run sees the same tool
set derived from the same servers. There is no mechanism to:

- Provide a run with a set of MCP servers that differ from the global set.
- Scope an MCP server connection's lifetime to a single run.
- Prevent one tenant's MCP tools from appearing in another tenant's run.
- Give per-customer or per-request tool sets to an agent.

The only runtime connection path is the `connect_mcp` deferred tool, which registers
into the global `Registry` — cross-contaminating all concurrent runs with the same
server name.

## Solution

Add an `MCPServers` field to `RunRequest`. When present, the runner creates a
`ScopedMCPRegistry` that merges the global `ClientManager` with a per-run
`ClientManager` containing only the run-specific servers. The `ScopedMCPRegistry` is
closed (and all per-run connections torn down) when the run completes.

The `connect_mcp` deferred tool is updated to register into the run-scoped registry
rather than the global one, so runtime-connected servers are also scoped to the run.

## New Types

### `MCPServerConfig` in `internal/harness/types.go`

```go
// MCPServerConfig describes a single MCP server to connect for a run.
// Exactly one of Command (stdio) or URL (http) must be set.
type MCPServerConfig struct {
    // Name is the logical identifier for this server. Required, must be unique
    // within the run (and must not collide with globally registered servers).
    Name string `json:"name"`

    // Command is the executable to launch for stdio transport.
    // Required when Transport is "stdio".
    Command string `json:"command,omitempty"`

    // Args are the arguments to pass to Command.
    Args []string `json:"args,omitempty"`

    // URL is the HTTP endpoint for http transport.
    // Required when Transport is "http".
    URL string `json:"url,omitempty"`
}
```

`MCPServerConfig` deliberately mirrors `mcp.ServerConfig` but is defined in the
`harness` package (where `RunRequest` lives) to avoid a circular import. A conversion
function `toMCPServerConfig(c MCPServerConfig) mcp.ServerConfig` bridges the two.

### `RunRequest` addition in `internal/harness/types.go`

```go
// MCPServers lists MCP servers to connect for this run only.
// Each server is connected on first use and torn down when the run completes.
// Server names must not collide with globally registered MCP server names;
// a collision causes StartRun to return an error.
// When nil or empty, only globally registered servers are available.
MCPServers []MCPServerConfig `json:"mcp_servers,omitempty"`
```

### `ScopedMCPRegistry` in `internal/mcp/scoped.go` (new file)

```go
// ScopedMCPRegistry wraps a global MCPRegistry and an optional per-run
// ClientManager. ListTools returns the union of both; CallTool routes to
// whichever registry owns the named server. Close tears down only the
// per-run manager.
type ScopedMCPRegistry struct {
    global  tools.MCPRegistry       // global ClientManager adapter — never closed by this type
    perRun  *mcp.ClientManager      // per-run connections; may be nil if no run-specific servers
    perRunNames map[string]struct{}  // set of server names owned by perRun
    mu      sync.RWMutex
    closed  bool
}

func NewScopedMCPRegistry(global tools.MCPRegistry, perRun *mcp.ClientManager, perRunNames []string) *ScopedMCPRegistry

// ListTools returns the union of global and per-run tool lists.
// Per-run tools shadow global tools with the same server name.
func (s *ScopedMCPRegistry) ListTools(ctx context.Context) (map[string][]tools.MCPToolDefinition, error)

// CallTool routes to per-run manager if server is in perRunNames, else to global.
func (s *ScopedMCPRegistry) CallTool(ctx context.Context, server, tool string, args json.RawMessage) (string, error)

// ListResources routes by server name.
func (s *ScopedMCPRegistry) ListResources(ctx context.Context, server string) ([]tools.MCPResource, error)

// ReadResource routes by server name.
func (s *ScopedMCPRegistry) ReadResource(ctx context.Context, server, uri string) (string, error)

// Close tears down the per-run ClientManager (all per-run connections).
// The global registry is NOT closed. Safe to call multiple times.
func (s *ScopedMCPRegistry) Close() error
```

`ScopedMCPRegistry` satisfies `tools.MCPRegistry`.

## Wiring in the Runner

### `StartRun` / run initialisation in `internal/harness/runner.go`

When `RunRequest.MCPServers` is non-empty:

1. Create a new `mcp.ClientManager` (`perRunCM`).
2. For each entry in `RunRequest.MCPServers`:
   a. Convert to `mcp.ServerConfig`.
   b. Call `perRunCM.AddServer(cfg)`.
   c. If `AddServer` returns an error (e.g. duplicate name), return the error from
      `StartRun` immediately (do not start the run).
3. Verify that none of the per-run server names collide with globally registered
   server names. If a collision is detected, return an error:
   `fmt.Errorf("mcp: per-run server name %q conflicts with a globally registered server", name)`.
4. Wrap global registry + `perRunCM` in `ScopedMCPRegistry`.
5. Pass the `ScopedMCPRegistry` as `BuildOptions.MCPRegistry` when building the tool
   catalog for this run.
6. Store the `ScopedMCPRegistry` on the run's internal state so it can be closed at
   run completion.

When `RunRequest.MCPServers` is nil or empty:

- Use the global `MCPRegistry` directly (existing behaviour, no allocation).

### Run teardown

At run completion (success, failure, or cancellation), call `ScopedMCPRegistry.Close()`
if one was created. This closes all per-run server connections. The global registry
is unaffected.

### `connect_mcp` deferred tool

The `connect_mcp` tool currently receives a `DynamicToolRegistrar` and an
`MCPConnector`. The `MCPConnector.Connect` method creates an in-process
`ClientManager` and returns a scoped registry. This does not change.

However, the `DynamicToolRegistrar.RegisterMCPTools` call currently registers into
the global `Registry`. When a `ScopedMCPRegistry` is in use, `RegisterMCPTools` must
register into the per-run scope. The mechanism:

- The per-run `ScopedMCPRegistry` implements `DynamicToolRegistrar` (or wraps a
  registrar that writes into the per-run `ClientManager`).
- The runner passes the scoped registrar to the tool builder.

This ensures that a server connected at runtime via `connect_mcp` is also
run-scoped: it is torn down when the run ends.

## Validation

`validateMCPServerConfig` (new function, `internal/harness/runner.go` or
`internal/harness/validate.go`):

```go
func validateMCPServerConfig(c MCPServerConfig) error {
    if strings.TrimSpace(c.Name) == "" {
        return fmt.Errorf("mcp_servers: name is required")
    }
    hasCommand := strings.TrimSpace(c.Command) != ""
    hasURL := strings.TrimSpace(c.URL) != ""
    if !hasCommand && !hasURL {
        return fmt.Errorf("mcp_servers[%q]: one of command or url is required", c.Name)
    }
    if hasCommand && hasURL {
        return fmt.Errorf("mcp_servers[%q]: command and url are mutually exclusive", c.Name)
    }
    if hasURL {
        u, err := url.Parse(c.URL)
        if err != nil {
            return fmt.Errorf("mcp_servers[%q]: invalid url: %w", c.Name, err)
        }
        if u.Scheme != "http" && u.Scheme != "https" {
            return fmt.Errorf("mcp_servers[%q]: url scheme must be http or https", c.Name)
        }
    }
    return nil
}
```

Called from `StartRun` before creating the per-run `ClientManager`.

## Files Changed

- `internal/harness/types.go` — add `MCPServerConfig`, add `MCPServers` to `RunRequest`.
- `internal/mcp/scoped.go` — new file; `ScopedMCPRegistry`.
- `internal/harness/runner.go` — per-run MCP setup in `StartRun`, teardown at run end.
- `internal/harness/validate.go` (or inline) — `validateMCPServerConfig`.
- `internal/mcp/scoped_test.go` — unit tests for `ScopedMCPRegistry`.
- `internal/harness/runner_test.go` — integration tests for per-run MCP.

## Test Requirements

All tests must pass with `-race`.

### Unit Tests — `ScopedMCPRegistry`

**`TestScopedMCPRegistry_ListTools_UnionOfGlobalAndPerRun`**
Create a `ScopedMCPRegistry` with a global registry exposing `{server_a: [tool_a]}`
and a per-run registry exposing `{server_b: [tool_b]}`. Call `ListTools`. Assert the
result contains both `server_a` and `server_b` with their respective tools.

**`TestScopedMCPRegistry_ListTools_PerRunServerShadowsGlobal`**
Global exposes `{overlap: [global_tool]}`. Per-run exposes `{overlap: [perrun_tool]}`.
Call `ListTools`. Assert result has `overlap` with only `[perrun_tool]` (per-run wins).

**`TestScopedMCPRegistry_CallTool_RoutesToPerRun`**
Call `CallTool` with server name registered in per-run. Assert the per-run
`ClientManager` received the call (global was not called).

**`TestScopedMCPRegistry_CallTool_RoutesToGlobal`**
Call `CallTool` with server name registered only in global. Assert global received
the call (per-run was not called).

**`TestScopedMCPRegistry_CallTool_UnknownServer_ReturnsError`**
Call `CallTool` with a server name registered in neither global nor per-run. Assert
a non-nil error is returned.

**`TestScopedMCPRegistry_Close_TeardownPerRunOnly`**
Create a `ScopedMCPRegistry`. Call `Close()`. Assert per-run `ClientManager` is
closed (subsequent calls to `ExecuteTool` on it return errors). Assert the global
registry is NOT closed (subsequent calls to it still work).

**`TestScopedMCPRegistry_Close_Idempotent`**
Call `Close()` twice. Assert no panic and second call returns nil.

**`TestScopedMCPRegistry_EmptyPerRun_DelegatesToGlobal`**
Create a `ScopedMCPRegistry` with `perRun = nil`. Assert `ListTools` returns
global's tools and `CallTool` routes to global.

### Unit Tests — `StartRun` validation

**`TestStartRun_MCPServers_InvalidConfig_NoCommand_NoURL`**
Submit a `RunRequest` with `MCPServers: [{Name:"x"}]`. Assert `StartRun` returns an
error (does not start the run).

**`TestStartRun_MCPServers_BothCommandAndURL`**
Submit a `RunRequest` with `MCPServers: [{Name:"x", Command:"cmd", URL:"http://h"}]`.
Assert `StartRun` returns an error.

**`TestStartRun_MCPServers_NameCollisionWithGlobal`**
Register server `"shared"` in the global `ClientManager`. Submit `RunRequest` with
`MCPServers: [{Name:"shared", URL:"http://h"}]`. Assert `StartRun` returns an error
containing `"conflicts with a globally registered server"`.

**`TestStartRun_MCPServers_Empty_NoAllocation`**
Submit a `RunRequest` with `MCPServers: nil`. Assert no `ScopedMCPRegistry` is
created (global registry is used directly). Validate by checking that the tool
catalog is built from the global registry unchanged.

**`TestStartRun_MCPServers_PerRunTeardownOnCompletion`**
Start a run with one per-run MCP server (backed by an `httptest.Server`). Let the run
complete. Assert the per-run server received an HTTP close / the connection is no
longer usable after run end.

### Integration Tests

**`TestConcurrentRuns_IndependentMCPServers`**
Start two concurrent runs, each with a different per-run MCP server (two distinct
`httptest.Server` instances). Each server returns different tool sets. Assert that
run A only sees server A's tools and run B only sees server B's tools, even while
both runs execute simultaneously. Verifies tenant isolation.

**`TestPerRunMCPTools_UnavailableAfterTeardown`**
After a run completes, attempt to call a per-run MCP tool. Assert the call fails
(connection is torn down).

### Race Test

**`TestConcurrentRuns_PerRunMCP_NoRaces`**
Spin up 10 concurrent runs, each with a distinct per-run MCP server. Call tools
from each run concurrently. Run with `-race`. Assert no data races in
`ScopedMCPRegistry`, the per-run `ClientManager`, or the global registry.

### Regression Tests

**`TestRunRequest_NoMCPServers_BackwardCompat`**
Submit a `RunRequest` with no `MCPServers` field. Assert the run behaves identically
to before this change: global MCP tools are available, no `ScopedMCPRegistry` is
allocated. Existing runner tests must pass unchanged.

**`TestGlobalMCPRegistry_StillWorks_AfterPerRunFeature`**
Register a server in the global `ClientManager` at startup. Start a run without
`MCPServers`. Assert global server's tools are present. Verifies the new code path
does not break the existing one.

**`TestConnectMCPDeferredTool_RegistersIntoPerRunScope`**
Start a run with `MCPServers` set (so a `ScopedMCPRegistry` is created). Invoke the
`connect_mcp` deferred tool from within the run. Assert the newly connected server
is registered into the per-run scope (not the global registry) and is torn down at
run end.

## Acceptance Criteria

1. `go test ./internal/mcp/... -race` passes with all new tests green.
2. `go test ./internal/harness/... -race` passes with all new tests green.
3. `go test ./... -race` passes.
4. Coverage on `internal/mcp` and `internal/harness` does not drop below 80%.
5. Two concurrent runs with different `MCPServers` do not share tools (verified by
   integration test).
6. Per-run servers are torn down after run completion (verified by test).
7. Runs without `MCPServers` are functionally identical to pre-feature behaviour.
8. `connect_mcp` deferred tool registers into the run-scoped registry when one exists.
