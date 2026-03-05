package observationalmemory

import (
	"context"
	"sync"
)

type Coordinator interface {
	WithinScope(ctx context.Context, key ScopeKey, fn func(context.Context) error) error
}

type lockEntry struct {
	mu   sync.Mutex
	refs int
}

type LocalCoordinator struct {
	mu    sync.Mutex
	locks map[string]*lockEntry
}

func NewLocalCoordinator() *LocalCoordinator {
	return &LocalCoordinator{locks: make(map[string]*lockEntry)}
}

func (c *LocalCoordinator) WithinScope(ctx context.Context, key ScopeKey, fn func(context.Context) error) error {
	unlock := c.lock(key.MemoryID())
	defer unlock()
	return fn(ctx)
}

func (c *LocalCoordinator) lock(scope string) func() {
	c.mu.Lock()
	entry, ok := c.locks[scope]
	if !ok {
		entry = &lockEntry{}
		c.locks[scope] = entry
	}
	entry.refs++
	c.mu.Unlock()

	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		c.mu.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(c.locks, scope)
		}
		c.mu.Unlock()
	}
}
