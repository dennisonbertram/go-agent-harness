package workspace_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/workspace"
)

// Compile-time interface compliance check.
var _ workspace.Workspace = (*workspace.PoolWorkspace)(nil)

// makeLocalFactory returns a Factory that creates LocalWorkspace instances
// backed by the given base directory.
func makeLocalFactory(harnessURL, baseDir string) workspace.Factory {
	return func() workspace.Workspace {
		return workspace.NewLocal(harnessURL, baseDir)
	}
}

// waitReady blocks until the pool's Ready channel is closed or ctx is done.
func waitReady(t *testing.T, pool *workspace.Pool, ctx context.Context) {
	t.Helper()
	select {
	case <-pool.Ready():
	case <-ctx.Done():
		t.Fatal("pool did not reach target size in time")
	}
}

// --------------------------------------------------------------------------
// TestPool_GetAndReturn
// --------------------------------------------------------------------------

func TestPool_GetAndReturn(t *testing.T) {
	dir := t.TempDir()
	factory := makeLocalFactory("http://localhost:8080", dir)
	pool := workspace.NewPool(factory, workspace.Options{BaseDir: dir}, 2)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	waitReady(t, pool, ctx)

	ws, id, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if ws == nil {
		t.Fatal("Get: returned nil workspace")
	}
	if id == "" {
		t.Fatal("Get: returned empty id")
	}

	pool.Return(id)
}

// --------------------------------------------------------------------------
// TestPool_TargetSize
// --------------------------------------------------------------------------

func TestPool_TargetSize(t *testing.T) {
	dir := t.TempDir()
	factory := makeLocalFactory("http://localhost:8080", dir)
	pool := workspace.NewPool(factory, workspace.Options{BaseDir: dir}, 3)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	waitReady(t, pool, ctx)

	if got := pool.Len(); got != 3 {
		t.Errorf("pool.Len() = %d, want 3", got)
	}
}

// --------------------------------------------------------------------------
// TestPool_Concurrent
// --------------------------------------------------------------------------

func TestPool_Concurrent(t *testing.T) {
	dir := t.TempDir()
	factory := makeLocalFactory("http://localhost:8080", dir)
	pool := workspace.NewPool(factory, workspace.Options{BaseDir: dir}, 5)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	waitReady(t, pool, ctx)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ws, id, err := pool.Get(ctx)
			if err != nil {
				// Context might have expired; tolerate this.
				return
			}
			_ = ws
			// Simulate some work.
			time.Sleep(10 * time.Millisecond)
			pool.Return(id)
		}()
	}
	wg.Wait()
}

// --------------------------------------------------------------------------
// TestPool_GetContextCancelled
// --------------------------------------------------------------------------

func TestPool_GetContextCancelled(t *testing.T) {
	dir := t.TempDir()
	// Use targetSize=0 to ensure no entries are ever available.
	factory := makeLocalFactory("http://localhost:8080", dir)
	pool := workspace.NewPool(factory, workspace.Options{BaseDir: dir}, 0)
	defer pool.Close()

	// pool with size 0 never closes its ready channel, so we bypass that wait
	// by using a context that is already cancelled.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already done

	_, _, err := pool.Get(ctx)
	if err == nil {
		t.Error("Get: expected error for cancelled context, got nil")
	}
}

// --------------------------------------------------------------------------
// TestPool_GetAfterClose
// --------------------------------------------------------------------------

func TestPool_GetAfterClose(t *testing.T) {
	dir := t.TempDir()
	factory := makeLocalFactory("http://localhost:8080", dir)
	pool := workspace.NewPool(factory, workspace.Options{BaseDir: dir}, 2)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	waitReady(t, pool, ctx)

	pool.Close()

	_, _, err := pool.Get(context.Background())
	if err == nil {
		t.Error("Get after Close: expected error, got nil")
	}
}

// --------------------------------------------------------------------------
// TestPool_ReturnAndReplenish
// --------------------------------------------------------------------------

// TestPool_ReturnAndReplenish verifies that after returning a workspace the
// pool's background goroutine eventually replenishes the slot.
func TestPool_ReturnAndReplenish(t *testing.T) {
	dir := t.TempDir()
	factory := makeLocalFactory("http://localhost:8080", dir)
	pool := workspace.NewPool(factory, workspace.Options{BaseDir: dir}, 2)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	waitReady(t, pool, ctx)

	// Drain all entries.
	_, id1, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Get 1: %v", err)
	}
	_, id2, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Get 2: %v", err)
	}

	if pool.Len() != 0 {
		t.Errorf("expected 0 available after draining, got %d", pool.Len())
	}

	// Return one; the pool should eventually replenish back to 2.
	pool.Return(id1)
	pool.Return(id2)

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if pool.Len() == 2 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if pool.Len() != 2 {
		t.Errorf("pool did not replenish to target size; Len() = %d", pool.Len())
	}
}

// --------------------------------------------------------------------------
// TestPool_ReturnUnknownID
// --------------------------------------------------------------------------

func TestPool_ReturnUnknownID(t *testing.T) {
	dir := t.TempDir()
	factory := makeLocalFactory("http://localhost:8080", dir)
	pool := workspace.NewPool(factory, workspace.Options{BaseDir: dir}, 1)
	defer pool.Close()

	// Returning an unknown ID must not panic.
	pool.Return("nonexistent-id")
}

// --------------------------------------------------------------------------
// TestPool_LenReflectsLeases
// --------------------------------------------------------------------------

func TestPool_LenReflectsLeases(t *testing.T) {
	dir := t.TempDir()
	factory := makeLocalFactory("http://localhost:8080", dir)
	pool := workspace.NewPool(factory, workspace.Options{BaseDir: dir}, 3)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	waitReady(t, pool, ctx)

	if pool.Len() != 3 {
		t.Fatalf("initial Len = %d, want 3", pool.Len())
	}

	_, id, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if pool.Len() != 2 {
		t.Errorf("Len after 1 lease = %d, want 2", pool.Len())
	}

	pool.Return(id)
}

// --------------------------------------------------------------------------
// TestPoolWorkspace_ImplementsWorkspace
// --------------------------------------------------------------------------

func TestPoolWorkspace_ImplementsWorkspace(t *testing.T) {
	var _ workspace.Workspace = (*workspace.PoolWorkspace)(nil)
}

// --------------------------------------------------------------------------
// TestPoolWorkspace_BeforeProvision
// --------------------------------------------------------------------------

func TestPoolWorkspace_BeforeProvision(t *testing.T) {
	dir := t.TempDir()
	factory := makeLocalFactory("http://localhost:8080", dir)
	pool := workspace.NewPool(factory, workspace.Options{BaseDir: dir}, 1)
	defer pool.Close()

	pw := workspace.NewPoolWorkspace(pool)
	if got := pw.HarnessURL(); got != "" {
		t.Errorf("HarnessURL before Provision = %q, want empty", got)
	}
	if got := pw.WorkspacePath(); got != "" {
		t.Errorf("WorkspacePath before Provision = %q, want empty", got)
	}
}

// --------------------------------------------------------------------------
// TestPoolWorkspace_FullLifecycle
// --------------------------------------------------------------------------

func TestPoolWorkspace_FullLifecycle(t *testing.T) {
	dir := t.TempDir()
	factory := makeLocalFactory("http://localhost:8080", dir)
	pool := workspace.NewPool(factory, workspace.Options{BaseDir: dir}, 2)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pw := workspace.NewPoolWorkspace(pool)
	// Provision leases from the pool (blocks until ready).
	if err := pw.Provision(ctx, workspace.Options{ID: "ignored"}); err != nil {
		t.Fatalf("Provision: %v", err)
	}

	if got := pw.HarnessURL(); got == "" {
		t.Error("expected non-empty HarnessURL after Provision")
	}
	if got := pw.WorkspacePath(); got == "" {
		t.Error("expected non-empty WorkspacePath after Provision")
	}

	// Destroy returns the workspace to the pool.
	if err := pw.Destroy(ctx); err != nil {
		t.Fatalf("Destroy: %v", err)
	}

	if got := pw.HarnessURL(); got != "" {
		t.Errorf("HarnessURL after Destroy = %q, want empty", got)
	}
	if got := pw.WorkspacePath(); got != "" {
		t.Errorf("WorkspacePath after Destroy = %q, want empty", got)
	}
}

// --------------------------------------------------------------------------
// TestPoolWorkspace_Destroy_NotProvisioned
// --------------------------------------------------------------------------

func TestPoolWorkspace_Destroy_NotProvisioned(t *testing.T) {
	dir := t.TempDir()
	factory := makeLocalFactory("http://localhost:8080", dir)
	pool := workspace.NewPool(factory, workspace.Options{BaseDir: dir}, 1)
	defer pool.Close()

	pw := workspace.NewPoolWorkspace(pool)
	if err := pw.Destroy(context.Background()); err != nil {
		t.Errorf("Destroy on unprovisioned PoolWorkspace: expected nil, got %v", err)
	}
}

// --------------------------------------------------------------------------
// TestPoolWorkspace_Destroy_NilPool
// --------------------------------------------------------------------------

func TestPoolWorkspace_Destroy_NilPool(t *testing.T) {
	// Calling Destroy with a nil pool must not panic.
	pw := workspace.NewPoolWorkspace(nil)
	if err := pw.Destroy(context.Background()); err != nil {
		t.Errorf("Destroy with nil pool: expected nil, got %v", err)
	}
}

// --------------------------------------------------------------------------
// TestPool_ConcurrentGetReturn_Race
// --------------------------------------------------------------------------

// TestPool_ConcurrentGetReturn_Race exercises concurrent Get/Return pairs
// under the race detector to catch data races.
func TestPool_ConcurrentGetReturn_Race(t *testing.T) {
	dir := t.TempDir()
	factory := makeLocalFactory("http://localhost:8080", dir)
	pool := workspace.NewPool(factory, workspace.Options{BaseDir: dir}, 4)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	waitReady(t, pool, ctx)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 3; j++ {
				ws, id, err := pool.Get(ctx)
				if err != nil {
					return
				}
				_ = ws
				time.Sleep(5 * time.Millisecond)
				pool.Return(id)
			}
		}()
	}
	wg.Wait()
}
