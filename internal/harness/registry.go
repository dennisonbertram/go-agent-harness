package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	htools "go-agent-harness/internal/harness/tools"
)

type registeredTool struct {
	def     ToolDefinition
	handler ToolHandler
	tier    htools.ToolTier // "core" or "deferred"
	tags    []string
}

// RegisterOptions provides optional metadata when registering a tool.
type RegisterOptions struct {
	Tier htools.ToolTier
	Tags []string
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
	r.tools[def.Name] = registeredTool{def: def, handler: handler, tier: htools.TierCore}
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

// RegisterWithOptions registers a tool with tier and tag metadata.
func (r *Registry) RegisterWithOptions(def ToolDefinition, handler ToolHandler, opts RegisterOptions) error {
	if def.Name == "" {
		return fmt.Errorf("tool name is required")
	}
	if handler == nil {
		return fmt.Errorf("tool handler is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.tools[def.Name]; ok {
		return fmt.Errorf("tool %q already registered", def.Name)
	}
	tier := opts.Tier
	if tier == "" {
		tier = htools.TierCore
	}
	r.tools[def.Name] = registeredTool{def: def, handler: handler, tier: tier, tags: opts.Tags}
	return nil
}

// DefinitionsForRun returns core tools plus any deferred tools activated for the given run.
func (r *Registry) DefinitionsForRun(runID string, tracker htools.ActivationTrackerInterface) []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var defs []ToolDefinition
	for _, rt := range r.tools {
		if rt.tier == htools.TierDeferred {
			if tracker == nil || !tracker.IsActive(runID, rt.def.Name) {
				continue
			}
		}
		defs = append(defs, rt.def)
	}
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})
	return defs
}

// DeferredDefinitions returns definitions of all deferred-tier tools.
func (r *Registry) DeferredDefinitions() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var defs []ToolDefinition
	for _, rt := range r.tools {
		if rt.tier == htools.TierDeferred {
			defs = append(defs, rt.def)
		}
	}
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})
	return defs
}
