# Issue #236 Implementation: Deterministic Config Propagation to Subagent Workspaces

**Branch**: `issue-236-config-propagation`
**Status**: Complete — all tests pass with `-race`

## Problem

Subagent workspaces (container, VM, worktree) had no way to receive RunnerConfig settings from the parent orchestrator. Each subagent harnessd instance started with zero config — no model, no cost ceiling, no feature flags. The parent process's config was silently dropped at dispatch time.

## Design Decision: B+A Hybrid

Two options were considered:

- **Option A (env-only)**: Pass all config as environment variables. Simple, but env vars are strings; complex config (nested structs, float64) requires ad-hoc encoding and is fragile across harnessd versions.
- **Option B (TOML file)**: Write `harness.toml` to the workspace root, which harnessd already reads at startup. Typed, versioned, and self-documenting. Works for all workspace types.
- **Option B+A (chosen)**: TOML file for non-secret config flags; env vars for secrets. This keeps API keys out of files on disk while preserving strong typing for feature flags.

## Implementation Summary

### 1. `internal/config/config.go`

Added two new TOML-tagged config sections:

```go
type AutoCompactConfig struct {
    Enabled            bool    `toml:"enabled"`
    Mode               string  `toml:"mode"`
    Threshold          float64 `toml:"threshold"`
    KeepLast           int     `toml:"keep_last"`
    ModelContextWindow int     `toml:"model_context_window"`
}

type ForensicsConfig struct {
    TraceToolDecisions            bool    `toml:"trace_tool_decisions"`
    DetectAntiPatterns            bool    `toml:"detect_anti_patterns"`
    TraceHookMutations            bool    `toml:"trace_hook_mutations"`
    CaptureRequestEnvelope        bool    `toml:"capture_request_envelope"`
    SnapshotMemorySnippet         bool    `toml:"snapshot_memory_snippet"`
    ErrorChainEnabled             bool    `toml:"error_chain_enabled"`
    ErrorContextDepth             int     `toml:"error_context_depth"`
    CaptureReasoning              bool    `toml:"capture_reasoning"`
    CostAnomalyDetectionEnabled   bool    `toml:"cost_anomaly_detection_enabled"`
    CostAnomalyStepMultiplier     float64 `toml:"cost_anomaly_step_multiplier"`
    AuditTrailEnabled             bool    `toml:"audit_trail_enabled"`
    ContextWindowSnapshotEnabled  bool    `toml:"context_window_snapshot_enabled"`
    ContextWindowWarningThreshold float64 `toml:"context_window_warning_threshold"`
    CausalGraphEnabled            bool    `toml:"causal_graph_enabled"`
    RolloutDir                    string  `toml:"rollout_dir"`
}
```

Both are added to `Config` and `rawLayer` follows the existing pointer-for-nil pattern.

### 2. `internal/config/workspace_config.go` (new file)

`WorkspaceRunnerConfig` — flat struct with only value-typed fields (bool/int/float64/string). Interface-typed RunnerConfig fields (hooks, managers) are intentionally excluded because they cannot be serialized.

Key functions:
- `WorkspaceRunnerConfig.ToTOML() (string, error)` — BurntSushi TOML encoder
- `WorkspaceRunnerConfigFromConfig(c Config) WorkspaceRunnerConfig` — maps Config → WorkspaceRunnerConfig

### 3. `internal/workspace/workspace.go`

Added `ConfigTOML string` to `Options`:

```go
type Options struct {
    ID         string
    RepoURL    string
    BaseDir    string
    Env        map[string]string
    ConfigTOML string  // written to harness.toml in workspace root; NEVER put secrets here
}
```

### 4. `internal/workspace/{local,worktree,container}.go`

Each `Provision()` writes `harness.toml` with 0o600 permissions when `opts.ConfigTOML != ""`:

```go
if opts.ConfigTOML != "" {
    cfgPath := filepath.Join(w.path, "harness.toml")
    if err := os.WriteFile(cfgPath, []byte(opts.ConfigTOML), 0o600); err != nil {
        return fmt.Errorf("workspace: write harness.toml: %w", err)
    }
}
```

For container workspaces, the file is written to the host bind-mount path, which maps to `/workspace/harness.toml` inside the container.

### 5. `internal/symphd/dispatcher.go`

Added to `DispatchConfig`:

```go
SubagentConfigTOML string        // non-secret config written to harness.toml
SubagentEnv        map[string]string  // secrets passed to container env, never disk
```

`runIssue()` copies `SubagentEnv` into a fresh per-dispatch map (prevents mutation of shared config) and sets `opts.ConfigTOML`.

### 6. `internal/symphd/orchestrator.go`

New `buildSubagentEnv()` function captures API keys from the parent process environment:

```go
func buildSubagentEnv() map[string]string {
    knownKeys := []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "HARNESS_MODEL"}
    env := make(map[string]string)
    for _, key := range knownKeys {
        if v := os.Getenv(key); v != "" {
            env[key] = v
        }
    }
    if len(env) == 0 {
        return nil
    }
    return env
}
```

`NewOrchestrator` passes `SubagentEnv` in `DispatchConfig`.

## Secret/Config Split Invariant

| Data type         | Propagation path               | Written to disk? |
|-------------------|-------------------------------|-----------------|
| Feature flags     | SubagentConfigTOML → harness.toml | Yes (0o600)    |
| API keys / tokens | SubagentEnv → workspace.Options.Env → container env | No |

## Tests Added

| File | Tests |
|------|-------|
| `internal/config/workspace_config_test.go` | Round-trip, key presence, FromConfig mapping, zero-value safety, ForensicsConfig TOML, AutoCompactConfig TOML |
| `internal/workspace/config_injection_test.go` | LocalWorkspace writes ConfigTOML, no-op when empty, 0o600 permissions, Options struct field |
| `internal/symphd/dispatcher_config_test.go` | Dispatcher propagates ConfigTOML, propagates SubagentEnv, DispatchConfig fields, nil-safe empty env |

All tests pass under `-race`.

## Git Commits

```
a947f53  test(#236): add regression tests for config propagation round-trip
6fa1576  feat(#236): add TOML keys for all RunnerConfig feature flags
9880a25  feat(#236): add ConfigTOML to workspace.Options; write harness.toml in Provision
1a23442  feat(#236): wire symphd Dispatcher to propagate config at dispatch time
```

## Pre-existing Regression Gate Failures (not from this PR)

`./scripts/test-regression.sh` fails due to three pre-existing 0% coverage functions unrelated to this change:

- `cmd/forensics/main.go:main` — 0.0%
- `internal/forensics/redaction/redaction.go:deepTransformValue` — 0.0%
- `internal/provider/openai/client.go:injectAdditionalPropertiesFalse` — 0.0%

These were present before this work and are tracked as separate technical debt. All tests introduced by this PR pass cleanly.
