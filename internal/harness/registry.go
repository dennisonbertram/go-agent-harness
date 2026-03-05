package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

type registeredTool struct {
	def     ToolDefinition
	handler ToolHandler
}

type Registry struct {
	mu    sync.RWMutex
	tools map[string]registeredTool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]registeredTool)}
}

func (r *Registry) Register(def ToolDefinition, handler ToolHandler) error {
	if def.Name == "" {
		return fmt.Errorf("tool name is required")
	}
	if handler == nil {
		return fmt.Errorf("tool handler is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[def.Name]; exists {
		return fmt.Errorf("tool %q already registered", def.Name)
	}
	r.tools[def.Name] = registeredTool{def: def, handler: handler}
	return nil
}

func (r *Registry) Definitions() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	defs := make([]ToolDefinition, 0, len(names))
	for _, name := range names {
		defs = append(defs, r.tools[name].def)
	}
	return defs
}

func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage) (string, error) {
	r.mu.RLock()
	tool, exists := r.tools[name]
	r.mu.RUnlock()
	if !exists {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	return tool.handler(ctx, args)
}
