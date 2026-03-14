# Issue #236 Implementation Plan: Deterministic Config Propagation to Subagent Workspaces

## Current State

### What `internal/config/config.go` covers (TOML-serializable):
- `model` (string)
- `max_steps` (int)
- `addr` (string)
- `[cost].max_per_run_usd` (float64)
- `[memory].enabled` (bool)
- `[mcp_servers.*]` (map)

### What RunnerConfig has that is NOT in TOML:
- `AutoCompactEnabled` (bool)
- `AutoCompactMode` (string)
- `AutoCompactThreshold` (float64)
- `AutoCompactKeepLast` (int)
- `ModelContextWindow` (int)
- `TraceToolDecisions` (bool)
- `DetectAntiPatterns` (bool)
- `TraceHookMutations` (bool)
- `CaptureRequestEnvelope` (bool)
- `SnapshotMemorySnippet` (bool)
- `ErrorChainEnabled` (bool)
- `ErrorContextDepth` (int)
- `CaptureReasoning` (bool)
- `CostAnomalyDetectionEnabled` (bool)
- `CostAnomalyStepMultiplier` (float64)
- `AuditTrailEnabled` (bool)
- `ContextWindowSnapshotEnabled` (bool)
- `ContextWindowWarningThreshold` (float64)
- `CausalGraphEnabled` (bool)
- `RolloutDir` (string — filesystem path, appropriate for TOML)

### What workspace.Options has:
- `ID` (string)
- `RepoURL` (string)
- `BaseDir` (string)
- `Env` (map[string]string)

Missing: `ConfigTOML string` — the TOML config for the subagent

### What symphd.Dispatcher.runIssue() does:
- Creates `workspace.Options{ID: ..., BaseDir: ...}` — no config injection
- Never populates `opts.Env` with API keys or other config
- Never writes a config file to the workspace

## Design Decision: B+A Hybrid

**Option B (TOML file)** for feature flags and non-secret settings:
- All RunnerConfig bool/int/float/string fields that are safe to write to disk
- Written to `harness.toml` in the workspace root via `opts.ConfigTOML`
- Each workspace type writes it during `Provision()` if `opts.ConfigTOML != ""`

**Option A (env vars)** for secrets:
- `OPENAI_API_KEY` → goes to `opts.Env`, never written to disk
- Container workspace already iterates `opts.Env` and passes them to the container
- Local/worktree workspaces don't need env injection (they inherit the parent process env)

## WorkspaceRunnerConfig Typed Struct

New type in `internal/config/workspace_config.go`:

```go
// WorkspaceRunnerConfig is the serializable subset of RunnerConfig that
// is propagated to subagent workspaces via a TOML config file.
// Fields that involve interface types (hooks, managers, etc.) are excluded.
// Secrets (API keys) are excluded and must be propagated via opts.Env instead.
type WorkspaceRunnerConfig struct {
    Model    string `toml:"model"`
    MaxSteps int    `toml:"max_steps"`

    // Cost
    MaxCostPerRunUSD float64 `toml:"max_cost_per_run_usd"`

    // Memory
    MemoryEnabled bool `toml:"memory_enabled"`

    // Auto-compaction
    AutoCompactEnabled    bool    `toml:"auto_compact_enabled"`
    AutoCompactMode       string  `toml:"auto_compact_mode"`
    AutoCompactThreshold  float64 `toml:"auto_compact_threshold"`
    AutoCompactKeepLast   int     `toml:"auto_compact_keep_last"`
    ModelContextWindow    int     `toml:"model_context_window"`

    // Forensics: tool decisions
    TraceToolDecisions  bool `toml:"trace_tool_decisions"`
    DetectAntiPatterns  bool `toml:"detect_anti_patterns"`
    TraceHookMutations  bool `toml:"trace_hook_mutations"`

    // Forensics: request envelope
    CaptureRequestEnvelope bool `toml:"capture_request_envelope"`
    SnapshotMemorySnippet  bool `toml:"snapshot_memory_snippet"`

    // Forensics: error chain
    ErrorChainEnabled bool `toml:"error_chain_enabled"`
    ErrorContextDepth int  `toml:"error_context_depth"`

    // Reasoning
    CaptureReasoning bool `toml:"capture_reasoning"`

    // Cost anomaly
    CostAnomalyDetectionEnabled bool    `toml:"cost_anomaly_detection_enabled"`
    CostAnomalyStepMultiplier   float64 `toml:"cost_anomaly_step_multiplier"`

    // Audit trail
    AuditTrailEnabled bool `toml:"audit_trail_enabled"`

    // Context window
    ContextWindowSnapshotEnabled    bool    `toml:"context_window_snapshot_enabled"`
    ContextWindowWarningThreshold   float64 `toml:"context_window_warning_threshold"`

    // Causal graph
    CausalGraphEnabled bool `toml:"causal_graph_enabled"`

    // Rollout
    RolloutDir string `toml:"rollout_dir"`
}

// ToTOML serializes the struct to a TOML string.
func (w WorkspaceRunnerConfig) ToTOML() (string, error) { ... }

// FromConfig creates a WorkspaceRunnerConfig from the harness config.Config
// and RunnerConfig, selecting only the propagatable fields.
func FromConfig(cfg Config) WorkspaceRunnerConfig { ... }
```

## Config Changes: internal/config/config.go

Add new TOML sections to `Config`:

```toml
[auto_compact]
enabled = false
mode = "hybrid"
threshold = 0.80
keep_last = 8
model_context_window = 128000

[forensics]
trace_tool_decisions = false
detect_anti_patterns = false
trace_hook_mutations = false
capture_request_envelope = false
snapshot_memory_snippet = false
error_chain_enabled = false
error_context_depth = 10
capture_reasoning = false
cost_anomaly_detection_enabled = false
cost_anomaly_step_multiplier = 0.0
audit_trail_enabled = false
context_window_snapshot_enabled = false
context_window_warning_threshold = 0.0
causal_graph_enabled = false
rollout_dir = ""
```

## workspace.Options Change

```go
type Options struct {
    ID         string
    RepoURL    string
    BaseDir    string
    Env        map[string]string
    ConfigTOML string // serialized TOML config; if non-empty, written to harness.toml
}
```

## Per-Workspace Provision() Changes

### local.go
After `os.MkdirAll(w.path)`, if `opts.ConfigTOML != ""`:
```go
cfgPath := filepath.Join(w.path, "harness.toml")
os.WriteFile(cfgPath, []byte(opts.ConfigTOML), 0600)
```

### worktree.go
After `git worktree add`, if `opts.ConfigTOML != ""`:
```go
cfgPath := filepath.Join(w.path, "harness.toml")
os.WriteFile(cfgPath, []byte(opts.ConfigTOML), 0600)
```

### container.go
Container workspace: ConfigTOML written to bind-mounted workspace dir on the host (gets mounted at /workspace). API keys go via `opts.Env` which already passes them to the container env.

## symphd Orchestrator Changes

`buildWorkspaceFactory()` returns a plain `WorkspaceFactory`. The config injection happens in `Dispatcher.runIssue()` when building `workspace.Options`:

```go
opts := workspace.Options{
    ID:         fmt.Sprintf("issue-%d", issue.Number),
    BaseDir:    d.config.BaseDir,
    ConfigTOML: d.config.SubagentConfigTOML, // pre-serialized at Dispatcher creation time
    Env:        d.config.SubagentEnv,         // API keys injected here
}
```

Or alternatively, the `DispatchConfig` gets:
- `SubagentConfigTOML string`
- `SubagentEnv map[string]string`

And `NewOrchestrator` populates these from environment at startup time.

## Testing Strategy

### Unit tests in `internal/config/workspace_config_test.go`:
1. `TestWorkspaceRunnerConfigRoundTrip` — serialize to TOML, parse back, verify all fields match
2. `TestWorkspaceRunnerConfigFromConfig` — verify `FromConfig()` maps fields correctly
3. `TestWorkspaceRunnerConfigTOMLFieldNames` — verify TOML keys are correct (via reflection or direct parsing)

### Unit tests in `internal/workspace/workspace_config_test.go`:
1. `TestLocalWorkspaceWritesConfigTOML` — provision with ConfigTOML, verify harness.toml written
2. `TestLocalWorkspaceNoConfigTOML` — provision without ConfigTOML, verify no harness.toml written
3. `TestWorktreeWorkspaceWritesConfigTOML` — same for worktree (requires git)
4. `TestContainerWorkspaceEnvNotTOML` — verify API keys in Env, not written to disk (mock Docker)

### Unit tests in `internal/symphd/dispatcher_test.go`:
1. `TestDispatcherPopulatesConfigTOML` — verify opts.ConfigTOML is non-empty when DispatchConfig has SubagentConfigTOML
2. `TestDispatcherPopulatesEnv` — verify opts.Env has API keys from SubagentEnv

### Race detector:
All tests must pass with `go test ./... -race`

## File Changes Summary

1. `internal/config/config.go` — add AutoCompact, Forensics TOML sections to Config struct
2. `internal/config/workspace_config.go` (NEW) — WorkspaceRunnerConfig struct + ToTOML() + FromConfig()
3. `internal/workspace/workspace.go` — add ConfigTOML to Options
4. `internal/workspace/local.go` — write harness.toml in Provision()
5. `internal/workspace/worktree.go` — write harness.toml in Provision()
6. `internal/workspace/container.go` — write harness.toml to host path (bind-mounted)
7. `internal/symphd/dispatcher.go` — add SubagentConfigTOML + SubagentEnv to DispatchConfig
8. `internal/symphd/orchestrator.go` — populate SubagentConfigTOML + SubagentEnv in buildWorkspaceFactory
9. `internal/config/workspace_config_test.go` (NEW) — round-trip tests
10. `internal/workspace/local_test.go` — extend for ConfigTOML
