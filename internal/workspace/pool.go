package workspace

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// poolEntry holds a pre-provisioned workspace and its availability state.
type poolEntry struct {
	ws    Workspace
	id    string
	inUse bool
}

// Pool maintains a set of pre-provisioned workspaces ready for immediate use.
// The background goroutine keeps the pool at target size, replacing destroyed
// or returned entries as needed.
//
// Pool is safe for concurrent use by multiple goroutines.
type Pool struct {
	mu         sync.Mutex
	entries    []*poolEntry
	factory    Factory  // creates new Workspace instances
	baseOpts   Options  // base Options used for provisioning; ID is overridden per entry
	targetSize int
	idCounter  int
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	ready      chan struct{} // closed once pool reaches target size for the first time
	readyOnce  sync.Once
}

// NewPool creates a Pool that maintains targetSize pre-provisioned workspaces.
// factory is called to create new Workspace instances.
// baseOpts provides BaseDir and other config; ID is auto-generated per entry.
//
// The pool starts a background goroutine to maintain the target size. Call
// Close to stop the background goroutine and destroy all workspaces.
func NewPool(factory Factory, baseOpts Options, targetSize int) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		factory:    factory,
		baseOpts:   baseOpts,
		targetSize: targetSize,
		ctx:        ctx,
		cancel:     cancel,
		ready:      make(chan struct{}),
	}
	p.wg.Add(1)
	go p.maintainLoop()
	return p
}

// Get leases an available workspace from the pool, blocking until one is
// available or ctx is done. Returns the leased Workspace and its pool ID.
// The caller must call Return(id) when done to release the workspace back to
// the pool.
func (p *Pool) Get(ctx context.Context) (Workspace, string, error) {
	// Wait for the pool to reach target size at least once.
	select {
	case <-p.ready:
	case <-ctx.Done():
		return nil, "", ctx.Err()
	case <-p.ctx.Done():
		return nil, "", fmt.Errorf("workspace: pool closed")
	}

	for {
		p.mu.Lock()
		for _, e := range p.entries {
			if !e.inUse && e.ws != nil {
				e.inUse = true
				ws := e.ws
				id := e.id
				p.mu.Unlock()
				return ws, id, nil
			}
		}
		p.mu.Unlock()

		// No available entry; wait a short interval and retry.
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-p.ctx.Done():
			return nil, "", fmt.Errorf("workspace: pool closed")
		case <-time.After(100 * time.Millisecond):
			// retry
		}
	}
}

// Return releases a leased workspace back to the pool.
// The workspace is marked for reset (Destroy + re-Provision) by setting its ws
// field to nil; the background goroutine will reprovision it.
// Calling Return with an unknown or already-returned id is a no-op.
func (p *Pool) Return(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, e := range p.entries {
		if e.id == id && e.inUse {
			// Destroy the underlying workspace to reset its state.
			// We do this under the lock with a background context because
			// Destroy is typically fast for local workspaces and avoids
			// holding the workspace in a dirty state.
			if e.ws != nil {
				ws := e.ws
				// Destroy asynchronously to avoid holding the lock.
				go func() {
					_ = ws.Destroy(context.Background())
				}()
			}
			e.ws = nil
			e.inUse = false
			return
		}
	}
}

// Close shuts down the pool, stopping the background goroutine and destroying
// all workspaces (both available and in-use).
func (p *Pool) Close() {
	p.cancel()
	p.wg.Wait()

	p.mu.Lock()
	entries := p.entries
	p.entries = nil
	p.mu.Unlock()

	for _, e := range entries {
		if e.ws != nil {
			_ = e.ws.Destroy(context.Background())
		}
	}
}

// Len returns the number of available (not in-use) entries currently in the pool.
func (p *Pool) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, e := range p.entries {
		if !e.inUse && e.ws != nil {
			n++
		}
	}
	return n
}

// Ready returns a channel that is closed once the pool has reached its target
// size for the first time.
func (p *Pool) Ready() <-chan struct{} {
	return p.ready
}

func (p *Pool) maintainLoop() {
	defer p.wg.Done()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	// Attempt an immediate fill before waiting for the first tick.
	p.fillPool()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.fillPool()
		}
	}
}

// fillPool provisions workspaces until the pool reaches targetSize.
// Entries with a nil ws (returned or failed) are reprovisioned.
func (p *Pool) fillPool() {
	p.mu.Lock()
	// Ensure we have enough entry slots.
	for len(p.entries) < p.targetSize {
		p.idCounter++
		p.entries = append(p.entries, &poolEntry{id: fmt.Sprintf("pool-%d", p.idCounter)})
	}

	// Collect entries that need provisioning.
	var toProvision []*poolEntry
	for _, e := range p.entries {
		if e.ws == nil && !e.inUse {
			toProvision = append(toProvision, e)
		}
	}
	p.mu.Unlock()

	provisioned := 0
	for _, e := range toProvision {
		// Stop provisioning if pool has been closed.
		if p.ctx.Err() != nil {
			break
		}

		ws := p.factory()
		opts := p.baseOpts
		opts.ID = e.id
		if err := ws.Provision(p.ctx, opts); err != nil {
			// Provisioning failed; maintainLoop will retry on next tick.
			continue
		}

		p.mu.Lock()
		// Only assign if the entry is still unprovisioned (not taken during provisioning).
		if e.ws == nil && !e.inUse {
			e.ws = ws
			provisioned++
		} else {
			// Entry was claimed or reprovisioned elsewhere; discard this one.
			p.mu.Unlock()
			_ = ws.Destroy(context.Background())
			continue
		}
		p.mu.Unlock()
	}

	// Signal readiness once we have at least targetSize live workspaces.
	p.mu.Lock()
	live := 0
	for _, e := range p.entries {
		if e.ws != nil {
			live++
		}
	}
	p.mu.Unlock()

	if live >= p.targetSize {
		p.readyOnce.Do(func() { close(p.ready) })
	}
}
