package workspace

import (
	"context"
	"sort"
	"sync"
)

// Registry maintains a mapping from implementation names to Factory functions.
// All methods on Registry are safe for concurrent use.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewRegistry returns an empty, ready-to-use Registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]Factory),
	}
}

// Register adds a workspace Factory under the given name.
// It returns ErrInvalidName if name is empty, or ErrAlreadyExists if a
// factory is already registered under that name.
func (r *Registry) Register(name string, f Factory) error {
	if name == "" {
		return ErrInvalidName
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.factories[name]; exists {
		return ErrAlreadyExists
	}
	r.factories[name] = f
	return nil
}

// New creates a workspace by name, provisions it with opts, and returns it.
// It returns ErrInvalidName if name is empty, ErrInvalidID if opts.ID is
// empty, or ErrNotFound if no factory is registered under name.
// If Provision returns an error, New returns that error.
func (r *Registry) New(ctx context.Context, name string, opts Options) (Workspace, error) {
	if name == "" {
		return nil, ErrInvalidName
	}
	if opts.ID == "" {
		return nil, ErrInvalidID
	}

	r.mu.RLock()
	f, exists := r.factories[name]
	r.mu.RUnlock()

	if !exists {
		return nil, ErrNotFound
	}

	ws := f()
	if err := ws.Provision(ctx, opts); err != nil {
		return nil, err
	}
	return ws, nil
}

// List returns a sorted slice of all registered implementation names.
// It returns an empty (non-nil) slice when no factories are registered.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// defaultRegistry is the package-level registry used by the top-level functions.
var defaultRegistry = NewRegistry()

// Register adds a Factory to the default package-level registry.
// See Registry.Register for error semantics.
func Register(name string, f Factory) error {
	return defaultRegistry.Register(name, f)
}

// New creates a Workspace using the default package-level registry.
// See Registry.New for error semantics.
func New(ctx context.Context, name string, opts Options) (Workspace, error) {
	return defaultRegistry.New(ctx, name, opts)
}

// List returns sorted implementation names from the default package-level registry.
func List() []string {
	return defaultRegistry.List()
}
