# MCP Server Configuration: Design

## Current State

The TOML config system (`internal/config/config.go`) already has `MCPServers map[string]MCPServerConfig` defined across all 6 layers. The schema works. The gap is:

1. `cfg.MCPServers` is never read in `cmd/harnessd/main.go` — only `HARNESS_MCP_SERVERS` env var is used
2. Profile-level MCP servers have no activation path
3. `mcpManager.Close()` is a bare `defer`, not wired into the graceful shutdown sequence
4. Subagents don't inherit profile MCP servers

---

## TOML Schema (already implemented)

### Global / project config

```toml
# ~/.harness/config.toml  or  .harness/config.toml
[mcp_servers.filesystem]
transport = "stdio"
command   = "npx"
args      = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]

[mcp_servers.fetch]
transport = "http"
url       = "http://localhost:3001/mcp"
```

### Profile config

```toml
# ~/.harness/profiles/coding.toml
[mcp_servers.filesystem]      # shadows the global "filesystem"
transport = "stdio"
command   = "npx"
args      = ["-y", "@modelcontextprotocol/server-filesystem", "/workspace"]

[mcp_servers.git-tools]       # adds a new server
transport = "stdio"
command   = "uvx"
args      = ["mcp-server-git"]
```

The TOML schema is **already valid and parseable** today. No schema changes needed.

---

## Design Decisions

### 1. Global vs profile servers

The core problem: the global `ClientManager` is created once at startup. Profiles are per-run. We can't register profile servers globally at startup because different runs use different profiles.

**Decision: two-layer split at startup**

- **Layers 1–3** (user global + project): load without a profile, register servers into the global `ClientManager`
- **Layer 4** (profile): loaded per-run when a run specifies a profile; registered into `ScopedMCPRegistry` as per-run servers

This maps perfectly onto the existing `ScopedMCPRegistry` semantics: global servers are in the global `ClientManager`, profile servers shadow them per-run.

**Implementation in `cmd/harnessd/main.go`:**

```go
// Load global config (layers 1-3, no profile)
globalCfg, err := config.Load(config.LoadOptions{
    UserConfigPath:    userConfigPath,
    ProjectConfigPath: projectConfigPath,
    // ProfileName intentionally omitted
})

// Register global MCP servers (TOML layers 1-3)
for name, srv := range globalCfg.MCPServers {
    mcpManager.AddServer(mcp.ServerConfig{
        Name:      name,
        Transport: srv.Transport,
        Command:   srv.Command,
        Args:      srv.Args,
        URL:       srv.URL,
    })
}

// Also register env var servers (additive; skip on name collision)
for _, cfg := range mcpEnvConfigs {
    if _, exists := globalCfg.MCPServers[cfg.Name]; exists {
        log.Printf("mcp: skipping env var server %q: already registered from config", cfg.Name)
        continue
    }
    mcpManager.AddServer(cfg)
}
```

### 2. Profile MCP servers → per-run ScopedMCPRegistry

When a run is started with a profile:

```go
// In runner.go StartRun (or a helper called from it):
if req.ProfileName != "" {
    profileCfg, err := config.LoadProfile(profilesDir, req.ProfileName)
    // profileCfg.MCPServers = only the servers this profile defines
    // Pass these to buildPerRunMCPRegistry alongside req.MCPServers
}
```

**Key constraint change**: currently `buildPerRunMCPRegistry` rejects per-run servers that collide with global server names. Profile servers must be allowed to shadow globals (that's the whole point). Add a `allowShadow bool` parameter:

```go
func buildPerRunMCPRegistry(
    global htools.MCPRegistry,
    globalNames []string,
    profileServers []MCPServerConfig,  // from TOML profile — shadow allowed
    runServers []MCPServerConfig,       // from RunRequest — shadow NOT allowed
) (*ScopedMCPRegistry, error)
```

Profile servers go into the ScopedMCPRegistry and shadow global ones with the same name. Run-level servers from `RunRequest.MCPServers` still cannot collide with globals or profile servers (error path unchanged for those).

Ordering within ScopedMCPRegistry: global < profile < per-run (each layer can shadow the one below).

### 3. Env var relationship

`HARNESS_MCP_SERVERS` is additive on top of TOML. Name collision resolution: **TOML wins** (env var entry is skipped with a warning). This matches the principle that TOML is explicit configuration, env var is for quick overrides.

If env var needs to override TOML, the user should put it in a profile.

### 4. Shutdown

**Current problem**: `defer func() { _ = mcpManager.Close() }()` runs last, potentially after the process is already exiting, not coordinated with in-flight tool calls completing.

**Fix**: Add `mcpManager.Close()` to the explicit graceful shutdown sequence, after the HTTP server drains (so in-flight requests finish) but before process exit:

```go
// In the shutdown sequence (after httpServer.Shutdown):
shutdownCallbacks = append(shutdownCallbacks, func() {
    if err := mcpManager.Close(); err != nil {
        log.Printf("mcp: close error: %v", err)
    }
})
```

Per-run ScopedMCPRegistry is already handled correctly: `closeScopedMCP` is called in `completeRun`, `failRun`, and `failRunMaxSteps` in `runner.go`. This tears down per-run servers when each run ends, independent of process shutdown.

**One gap**: if harnessd is killed while a run is active, the per-run ScopedMCPRegistry's subprocess might be orphaned. This is acceptable (the subprocess will die when its stdin pipe closes anyway for stdio transport; HTTP servers are remote so no issue).

### 5. Subagent propagation

Two subagent paths:

#### Path A: Workspace subagents (container, VM, worktree, Hetzner)

Issue #236 already passes `ConfigTOML` to workspace provisioners which write it to `.harness/config.toml` in the workspace. The subagent starts its own `harnessd` which loads that config, picking up global MCP servers from layers 1-3.

**What's missing**: profile MCP servers. If the parent run used a profile, the profile's servers aren't in the `ConfigTOML`.

**Fix**: when building `ConfigTOML` for a workspace subagent, merge the parent run's profile servers into the config as layer-3 (project) servers. This "bakes in" the profile for the subagent.

```go
// Before writing ConfigTOML for workspace:
if profileName != "" {
    profileCfg := loadProfile(profileName)
    for name, srv := range profileCfg.MCPServers {
        workspaceCfg.MCPServers[name] = srv  // profile wins over global for subagent
    }
}
```

#### Path B: In-process forks (RunForkedSkill)

The fork creates a new `RunRequest` for the child run. The child run goes through the same `StartRun` path and builds its own `ScopedMCPRegistry`.

**Fix**: copy `ProfileName` from parent run into child `RunRequest`. The child run will then load the same profile and activate the same profile MCP servers.

```go
// In runner.go RunForkedSkill or similar:
childReq.ProfileName = parentRun.ProfileName
```

This requires `Run` struct and `RunRequest` to carry `ProfileName`. `RunRequest` already has `PromptProfile` for the prompt system — we either reuse it or add a dedicated `MCPProfile` field. Simplest: one `Profile string` field that covers all profile-level config (both prompt and MCP).

---

## Implementation Plan

Three issues to create:

### Issue A: Wire TOML mcp_servers into ClientManager at startup
- Read `globalCfg.MCPServers` in `main.go` and register each server
- Env var is additive (TOML wins on name collision)
- Tests: server registered from TOML is discoverable via `mcpManager.DiscoverTools`
- Files: `cmd/harnessd/main.go`

### Issue B: Profile MCP server activation
- Add `ProfileName` to `RunRequest` (reuse existing `PromptProfile` or add new field)
- In `runner.go StartRun`: load profile config, extract profile-only MCP servers, pass to `buildPerRunMCPRegistry` with shadow-allowed flag
- Update `buildPerRunMCPRegistry` to accept two server lists: profile (shadow ok) + run-level (no shadow)
- Files: `internal/harness/runner.go`, `internal/harness/types.go`, `internal/harness/scoped_mcp.go`

### Issue C: Subagent propagation
- Path A: when building `ConfigTOML` for workspace subagent, merge parent run's profile servers into it
- Path B: copy `ProfileName` from parent run to fork `RunRequest`
- Files: wherever `ConfigTOML` is constructed for workspaces (likely `internal/symphd/`), `internal/harness/runner.go`

### Issue D: Shutdown wiring
- Move `mcpManager.Close()` into explicit shutdown callback
- Files: `cmd/harnessd/main.go`

Issues A and D are small and can be done together. B and C are larger.

---

## What We're NOT Changing

- TOML schema: already correct
- `MCPServerConfig` struct: already correct
- `ScopedMCPRegistry` core logic: already correct, just need the shadow-allowed flag
- Per-run `RunRequest.MCPServers`: still works for dynamic per-request servers
- `HARNESS_MCP_SERVERS` env var: still works, now additive on top of TOML
