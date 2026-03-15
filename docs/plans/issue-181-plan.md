# Plan: Issue #181 — Workspace Interface + Package Scaffold

## Summary
Create `internal/workspace/` package with the core `Workspace` interface, `Options` struct, a thread-safe `Registry`, and sentinel error types. No concrete implementations — just the contract.

## Files to Create

### `internal/workspace/workspace.go`
- `Workspace` interface with 4 methods
- `Options` struct
- `Factory` type alias
- Sentinel error vars

### `internal/workspace/registry.go`
- `Registry` struct with `sync.RWMutex`
- `Register(name string, f Factory)` — add implementation
- `New(ctx context.Context, name string, opts Options) (Workspace, error)` — create by name
- `List() []string` — sorted implementation names
- Package-level default registry + top-level `Register`/`New`/`List` functions

### `internal/workspace/doc.go`
- Package-level documentation

### `internal/workspace/workspace_test.go`
- Mock workspace for interface compliance
- Registry unit tests (register, new, list, not-found, concurrent access)
- Error type tests

## Interface Design

```go
type Workspace interface {
    Provision(ctx context.Context, opts Options) error
    HarnessURL() string
    WorkspacePath() string
    Destroy(ctx context.Context) error
}

type Options struct {
    ID      string
    RepoURL string
    BaseDir string
    Env     map[string]string
}

type Factory func() Workspace
```

## Errors
```go
var (
    ErrNotFound         = errors.New("workspace: implementation not found")
    ErrAlreadyExists    = errors.New("workspace: implementation already registered")
    ErrInvalidID        = errors.New("workspace: ID must not be empty")
    ErrInvalidName      = errors.New("workspace: name must not be empty")
)
```

## Testing Strategy
- Mock `Workspace` implementation for all interface tests
- Registry: register, new (found), new (not found), list (sorted), concurrent register+new under -race
- Boundary: empty name, empty ID, nil factory

## Commit Strategy
1. `feat(#181): add internal/workspace package with Workspace interface and registry`

## Risk Areas
- Concurrent map access in registry — use sync.RWMutex
- Factory returning nil — document contract, don't guard against it at registry level
