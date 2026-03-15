# Plan: Issue #187 — symphd Daemon Scaffold

## Overview
Create `cmd/symphd/` binary + `internal/symphd/` package. HTTP API stubs, graceful shutdown, YAML config. Foundation for #188-#191.

## Files

### `internal/symphd/config.go`
```go
type Config struct {
    Addr                string `yaml:"addr"`                  // default: ":8888"
    WorkspaceType       string `yaml:"workspace_type"`        // default: "local"
    MaxConcurrentAgents int    `yaml:"max_concurrent_agents"` // default: 10
    PollIntervalMs      int    `yaml:"poll_interval_ms"`      // default: 5000
    HarnessURL          string `yaml:"harness_url"`           // default: "http://localhost:8080"
    BaseDir             string `yaml:"base_dir"`              // default: os.TempDir()/symphd
}
func Load(path string) (*Config, error)
func (c *Config) applyDefaults()
```

### `internal/symphd/orchestrator.go`
Stub — real logic in #188-#190.
```go
type Orchestrator struct {
    config    *Config
    startedAt time.Time
    mu        sync.RWMutex
    agents    int
}
func NewOrchestrator(cfg *Config) *Orchestrator
func (o *Orchestrator) State() map[string]any
func (o *Orchestrator) Start(ctx context.Context) error  // stub: return nil
func (o *Orchestrator) Shutdown(ctx context.Context) error // stub: return nil
```

### `internal/symphd/http.go`
Three endpoints:
- `GET /api/v1/state` → `{"status":"ok","version":"0.1.0","running_since":"...","agent_count":0}`
- `GET /api/v1/issues` → `{"status":"ok","issues":[]}`
- `POST /api/v1/refresh` → `{"status":"ok","message":"refresh queued"}`

### `cmd/symphd/main.go`
- Flag: `--config` (path to YAML), `--addr` (override)
- Load config → create Orchestrator → start HTTP server
- signal.Notify(SIGINT, SIGTERM) → graceful shutdown (10s timeout)
- Follow same pattern as cmd/harnessd/main.go

## Tests
- `internal/symphd/config_test.go` — Load valid/invalid/missing YAML, defaults
- `internal/symphd/orchestrator_test.go` — NewOrchestrator, State, Start/Shutdown stubs
- `internal/symphd/http_test.go` — all 3 endpoints with httptest.NewRecorder

## Dependencies
- gopkg.in/yaml.v3 (already in go.mod)
- stdlib only

## Commit
`feat(#187): add symphd daemon scaffold with HTTP API and config`
