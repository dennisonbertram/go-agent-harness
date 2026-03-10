package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
	mu         sync.RWMutex
	tools      map[string]registeredTool
	mcpServers map[string]struct{} // tracks registered MCP server names to prevent duplicates
}

func NewRegistry() *Registry {
	return &Registry{
		tools:      make(map[string]registeredTool),
		mcpServers: make(map[string]struct{}),
	}
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

// RegisterMCPTools dynamically registers tools discovered from a new MCP server.
// serverName is the logical name for the server (used as part of tool name prefix).
// toolDefs contains the tool definitions returned by the MCP server.
// caller is the MCPRegistry used to invoke the tools via CallTool.
//
// Each tool is registered as "mcp_<serverName>_<toolName>" at TierDeferred tier
// so it is immediately available for activation.
//
// Returns the list of tool names that were registered.
// Returns an error if the server name was already registered or if required args are missing.
func (r *Registry) RegisterMCPTools(serverName string, toolDefs []htools.MCPToolDefinition, caller htools.MCPRegistry) ([]string, error) {
	if serverName == "" {
		return nil, fmt.Errorf("server name is required")
	}
	if caller == nil {
		return nil, fmt.Errorf("MCPRegistry caller is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.mcpServers[serverName]; exists {
		return nil, fmt.Errorf("MCP server %q is already connected", serverName)
	}

	safeServer := sanitizeMCPNamePart(serverName)
	var registered []string

	for _, td := range toolDefs {
		safeName := sanitizeMCPNamePart(td.Name)
		toolName := "mcp_" + safeServer + "_" + safeName

		if _, exists := r.tools[toolName]; exists {
			// Skip duplicates silently — prefer first registration.
			continue
		}

		origName := td.Name
		regServer := serverName
		mcpReg := caller

		def := ToolDefinition{
			Name:        toolName,
			Description: td.Description,
			Parameters:  td.Parameters,
		}
		handler := ToolHandler(func(ctx context.Context, args json.RawMessage) (string, error) {
			return mcpReg.CallTool(ctx, regServer, origName, args)
		})
		r.tools[toolName] = registeredTool{
			def:     def,
			handler: handler,
			tier:    htools.TierDeferred,
			tags:    []string{"mcp", "integration", "external", "dynamic"},
		}
		registered = append(registered, toolName)
	}

	r.mcpServers[serverName] = struct{}{}
	return registered, nil
}

// sanitizeMCPNamePart normalizes a string for use as part of an MCP tool name.
// Mirrors the logic in the deferred package to keep naming consistent.
func sanitizeMCPNamePart(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, ".", "_")
	if s == "" {
		return "x"
	}
	return s
}

// ReplaceByTag atomically replaces all tools that have the given source tag
// with the new set of tools. Tools that do not carry the tag are left
// untouched. This is intended for hot-reload scenarios where a particular
// source (e.g. "skills" or "scripts") is reloaded from disk.
//
// Each tool in newTools must supply a non-empty Name. All new tools are
// tagged with sourceTag so they can be replaced again in future hot-reload
// cycles.
//
// ReplaceByTag is safe for concurrent use.
func (r *Registry) ReplaceByTag(sourceTag string, newTools []htools.Tool) error {
	if sourceTag == "" {
		return fmt.Errorf("sourceTag must not be empty")
	}
	for _, t := range newTools {
		if t.Definition.Name == "" {
			return fmt.Errorf("tool name is required")
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove all currently registered tools that carry the source tag.
	for name, rt := range r.tools {
		for _, tag := range rt.tags {
			if tag == sourceTag {
				delete(r.tools, name)
				break
			}
		}
	}

	// Register the new tools, tagging each with sourceTag.
	for _, t := range newTools {
		tags := make([]string, 0, len(t.Definition.Tags)+1)
		tags = append(tags, t.Definition.Tags...)
		// Ensure the source tag is always present so future reloads can find it.
		hasSrc := false
		for _, tg := range tags {
			if tg == sourceTag {
				hasSrc = true
				break
			}
		}
		if !hasSrc {
			tags = append(tags, sourceTag)
		}

		tier := t.Definition.Tier
		if tier == "" {
			tier = htools.TierCore
		}

		r.tools[t.Definition.Name] = registeredTool{
			def: ToolDefinition{
				Name:        t.Definition.Name,
				Description: t.Definition.Description,
				Parameters:  t.Definition.Parameters,
			},
			handler: ToolHandler(t.Handler),
			tier:    tier,
			tags:    tags,
		}
	}
	return nil
}
