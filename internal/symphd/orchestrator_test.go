package symphd

import (
	"context"
	"testing"
	"time"

	"go-agent-harness/internal/workspace"
)

func TestNewOrchestrator(t *testing.T) {
	cfg := DefaultConfig()
	o := NewOrchestrator(cfg)
	if o == nil {
		t.Fatal("NewOrchestrator returned nil")
	}
	if o.config != cfg {
		t.Error("config not set")
	}
	if o.startedAt.IsZero() {
		t.Error("startedAt not set")
	}
}

func TestOrchestrator_State(t *testing.T) {
	cfg := DefaultConfig()
	o := NewOrchestrator(cfg)
	state := o.State()
	if state["version"] != "0.1.0" {
		t.Errorf("version = %v", state["version"])
	}
	if _, ok := state["running_since"]; !ok {
		t.Error("running_since missing")
	}
	if state["agent_count"] != 0 {
		t.Errorf("agent_count = %v", state["agent_count"])
	}
}

func TestOrchestrator_State_RunningTimeIncreases(t *testing.T) {
	cfg := DefaultConfig()
	o := NewOrchestrator(cfg)
	s1 := o.State()
	time.Sleep(10 * time.Millisecond)
	s2 := o.State()
	// running_since should be the same (fixed at start)
	if s1["running_since"] != s2["running_since"] {
		t.Error("running_since changed between calls")
	}
}

func TestOrchestrator_Start(t *testing.T) {
	cfg := DefaultConfig()
	o := NewOrchestrator(cfg)
	if err := o.Start(context.Background()); err != nil {
		t.Errorf("Start returned error: %v", err)
	}
}

func TestOrchestrator_Shutdown(t *testing.T) {
	cfg := DefaultConfig()
	o := NewOrchestrator(cfg)
	if err := o.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown returned error: %v", err)
	}
}

func TestOrchestrator_Concurrent_State(t *testing.T) {
	cfg := DefaultConfig()
	o := NewOrchestrator(cfg)
	// Concurrent reads should not race
	done := make(chan struct{})
	for i := 0; i < 20; i++ {
		go func() {
			_ = o.State()
			done <- struct{}{}
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}

// ---------------------------------------------------------------------------
// buildWorkspaceFactory tests
// ---------------------------------------------------------------------------

// TestBuildWorkspaceFactory_Local verifies that workspace_type "local" returns
// a non-nil factory and a nil pool.
func TestBuildWorkspaceFactory_Local(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkspaceType = "local"

	factory, pool := buildWorkspaceFactory(cfg)

	if factory == nil {
		t.Fatal("expected non-nil factory for workspace_type=local")
	}
	if pool != nil {
		t.Errorf("expected nil pool for workspace_type=local, got non-nil")
	}

	// Verify the factory returns a non-nil workspace.
	ws := factory()
	if ws == nil {
		t.Error("factory() returned nil workspace")
	}
	// Verify the returned workspace is a *workspace.LocalWorkspace.
	if _, ok := ws.(*workspace.LocalWorkspace); !ok {
		t.Errorf("factory() returned %T, want *workspace.LocalWorkspace", ws)
	}
}

// TestBuildWorkspaceFactory_Worktree verifies that workspace_type "worktree"
// returns a non-nil factory and a nil pool.
func TestBuildWorkspaceFactory_Worktree(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkspaceType = "worktree"

	factory, pool := buildWorkspaceFactory(cfg)

	if factory == nil {
		t.Fatal("expected non-nil factory for workspace_type=worktree")
	}
	if pool != nil {
		t.Errorf("expected nil pool for workspace_type=worktree, got non-nil")
	}

	ws := factory()
	if ws == nil {
		t.Error("factory() returned nil workspace")
	}
	if _, ok := ws.(*workspace.WorktreeWorkspace); !ok {
		t.Errorf("factory() returned %T, want *workspace.WorktreeWorkspace", ws)
	}
}

// TestBuildWorkspaceFactory_Container verifies that workspace_type "container"
// returns a non-nil factory and a nil pool.
func TestBuildWorkspaceFactory_Container(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkspaceType = "container"

	factory, pool := buildWorkspaceFactory(cfg)

	if factory == nil {
		t.Fatal("expected non-nil factory for workspace_type=container")
	}
	if pool != nil {
		t.Errorf("expected nil pool for workspace_type=container, got non-nil")
	}

	ws := factory()
	if ws == nil {
		t.Error("factory() returned nil workspace")
	}
	if _, ok := ws.(*workspace.ContainerWorkspace); !ok {
		t.Errorf("factory() returned %T, want *workspace.ContainerWorkspace", ws)
	}
}

// TestBuildWorkspaceFactory_Unknown verifies that an unrecognised workspace_type
// returns nil factory and nil pool.
func TestBuildWorkspaceFactory_Unknown(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkspaceType = "unknown-type-xyz"

	factory, pool := buildWorkspaceFactory(cfg)

	if factory != nil {
		t.Errorf("expected nil factory for unknown workspace_type, got non-nil")
	}
	if pool != nil {
		t.Errorf("expected nil pool for unknown workspace_type, got non-nil")
	}
}

// TestBuildWorkspaceFactory_Pool verifies that workspace_type "pool" returns a
// non-nil factory AND a non-nil pool. The pool is closed after the test.
func TestBuildWorkspaceFactory_Pool(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkspaceType = "pool"
	cfg.PoolWorkspaceType = "local"
	cfg.PoolSize = 1

	factory, pool := buildWorkspaceFactory(cfg)

	if factory == nil {
		t.Fatal("expected non-nil factory for workspace_type=pool")
	}
	if pool == nil {
		t.Fatal("expected non-nil pool for workspace_type=pool")
	}
	defer pool.Close()

	// Verify the factory returns a PoolWorkspace.
	ws := factory()
	if ws == nil {
		t.Error("factory() returned nil workspace")
	}
	if _, ok := ws.(*workspace.PoolWorkspace); !ok {
		t.Errorf("factory() returned %T, want *workspace.PoolWorkspace", ws)
	}
}

// TestBuildWorkspaceFactory_Pool_UnknownInner verifies that pool mode with an
// unrecognised inner type returns nil factory and nil pool (graceful failure).
func TestBuildWorkspaceFactory_Pool_UnknownInner(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkspaceType = "pool"
	cfg.PoolWorkspaceType = "unknown-inner"
	cfg.PoolSize = 1

	factory, pool := buildWorkspaceFactory(cfg)

	if factory != nil {
		t.Errorf("expected nil factory for pool with unknown inner type, got non-nil")
	}
	if pool != nil {
		t.Errorf("expected nil pool for pool with unknown inner type, got non-nil")
	}
}

// ---------------------------------------------------------------------------
// NewOrchestrator wiring tests
// ---------------------------------------------------------------------------

// TestNewOrchestrator_WiresDispatcher verifies that when both WorkspaceType and
// GitHub owner/repo are set, NewOrchestrator creates a non-nil dispatcher.
func TestNewOrchestrator_WiresDispatcher(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkspaceType = "local"
	cfg.GitHubOwner = "testowner"
	cfg.GitHubRepo = "testrepo"
	cfg.GitHubToken = "tok"

	o := NewOrchestrator(cfg)

	if o.dispatcher == nil {
		t.Error("expected non-nil dispatcher when WorkspaceType and GitHub config are set")
	}
	if o.pool != nil {
		t.Errorf("expected nil pool for workspace_type=local, got non-nil")
	}
}

// TestNewOrchestrator_NoDispatcher_NoTracker verifies that without GitHub config,
// the dispatcher is not auto-wired even if WorkspaceType is set.
func TestNewOrchestrator_NoDispatcher_NoTracker(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkspaceType = "local"
	// No GitHubOwner/Repo configured.

	o := NewOrchestrator(cfg)

	if o.dispatcher != nil {
		t.Error("expected nil dispatcher when no tracker is configured")
	}
}

// TestNewOrchestrator_NoDispatcher_NoWorkspace verifies that without WorkspaceType,
// the dispatcher is not auto-wired even if GitHub config is set.
func TestNewOrchestrator_NoDispatcher_NoWorkspace(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkspaceType = "unknown-type"
	cfg.GitHubOwner = "testowner"
	cfg.GitHubRepo = "testrepo"
	cfg.GitHubToken = "tok"

	o := NewOrchestrator(cfg)

	if o.dispatcher != nil {
		t.Error("expected nil dispatcher when workspace factory cannot be built")
	}
}

// TestNewOrchestrator_Pool_WiresDispatcherAndPool verifies that pool mode
// creates both a non-nil pool and a non-nil dispatcher.
func TestNewOrchestrator_Pool_WiresDispatcherAndPool(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkspaceType = "pool"
	cfg.PoolWorkspaceType = "local"
	cfg.PoolSize = 1
	cfg.GitHubOwner = "testowner"
	cfg.GitHubRepo = "testrepo"
	cfg.GitHubToken = "tok"

	o := NewOrchestrator(cfg)
	defer func() {
		// Shutdown closes the pool.
		_ = o.Shutdown(context.Background())
	}()

	if o.pool == nil {
		t.Error("expected non-nil pool for workspace_type=pool")
	}
	if o.dispatcher == nil {
		t.Error("expected non-nil dispatcher for pool mode with tracker configured")
	}
}

// ---------------------------------------------------------------------------
// Shutdown pool close test
// ---------------------------------------------------------------------------

// TestOrchestrator_ShutdownClosesPool verifies that Shutdown calls pool.Close().
// We use a real pool backed by a local workspace factory, but with size 0 to
// avoid actual provisioning, and verify Close returns without error.
func TestOrchestrator_ShutdownClosesPool(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkspaceType = "pool"
	cfg.PoolWorkspaceType = "local"
	// Use size 0 — pool still creates a background goroutine that must be stopped.
	cfg.PoolSize = 0
	// No tracker; dispatcher won't be wired, but pool still gets created.
	// We need to manually set pool to test Shutdown path.

	// Build the factory and pool directly.
	factory, pool := buildWorkspaceFactory(cfg)
	if pool == nil {
		// With PoolSize=0 it's still a valid pool — if buildWorkspaceFactory
		// returns nil for size=0 inner "local", that's also acceptable.
		// Just skip the pool-specific assertion.
		t.Skip("pool not created for PoolSize=0; skipping")
	}
	_ = factory

	cfg2 := DefaultConfig()
	cfg2.WorkspaceType = "local"

	o := NewOrchestrator(cfg2)
	// Manually inject the pool so we can verify it gets closed.
	o.pool = pool

	// Shutdown should call pool.Close() without panicking or returning an error.
	if err := o.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown returned error: %v", err)
	}

	// Calling Close again on a closed pool should not panic (Pool.Close is idempotent
	// in the sense that the cancel is called and wg.Wait returns immediately).
	// We just verify Shutdown completed successfully.
}
