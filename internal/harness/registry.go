package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	htools "go-agent-harness/internal/harness/tools"
)

type registeredTool struct {
	def          ToolDefinition
	handler      ToolHandler
	tier         htools.ToolTier // "core" or "deferred"
	tags         []string
	parallelSafe bool
	mutating     bool
	action       htools.Action
	inflight     *sync.WaitGroup
	mcpServer    string
	owner        string
	condition    string
}

type ToolMetadata struct {
	Definition ToolDefinition
	Tier       htools.ToolTier
	Tags       []string
	Owner      string
	Condition  string
}

// ToolsetResolverProvenance identifies the runtime resolver that determined a
// configured dynamic toolset was unavailable. It intentionally contains no
// credentials, endpoints, or raw upstream errors.
type ToolsetResolverProvenance struct {
	Source               string `json:"source"`
	Provider             string `json:"provider"`
	IndividualNamesKnown bool   `json:"individual_names_known"`
}

type ConfiguredUnavailableToolset struct {
	Name       string                    `json:"name"`
	Owner      string                    `json:"owner"`
	Condition  string                    `json:"condition"`
	Provenance ToolsetResolverProvenance `json:"provenance"`
}

type UnavailableToolsetObservation struct {
	Kind       string                    `json:"kind"`
	Name       string                    `json:"name"`
	Owner      string                    `json:"owner"`
	Condition  string                    `json:"condition"`
	Reason     string                    `json:"reason"`
	Provenance ToolsetResolverProvenance `json:"provenance"`
}

// ToolsetResolutionSnapshot carries the paired configured/observed records
// required to prove that a dynamic provider was not silently omitted.
type ToolsetResolutionSnapshot struct {
	ConfiguredUnavailable []ConfiguredUnavailableToolset  `json:"configured_unavailable_toolsets"`
	Unavailable           []UnavailableToolsetObservation `json:"unavailable"`
	Incomplete            bool                            `json:"-"`
	IncompleteReason      string                          `json:"-"`
}

type toolsetResolutionReporter interface {
	ToolsetResolutionSnapshot() ToolsetResolutionSnapshot
}

// ToolsetResolutionError binds a partial dynamic catalog failure to the exact
// per-call resolver snapshot, avoiding a mutable global "last result" race
// when several run registries are constructed concurrently.
type ToolsetResolutionError struct {
	Snapshot ToolsetResolutionSnapshot
	Err      error
}

func (e *ToolsetResolutionError) Error() string { return e.Err.Error() }
func (e *ToolsetResolutionError) Unwrap() error { return e.Err }
func (e *ToolsetResolutionError) ToolsetResolutionSnapshot() ToolsetResolutionSnapshot {
	return cloneToolsetResolutionSnapshot(e.Snapshot)
}

// RegisterOptions provides optional metadata when registering a tool.
type RegisterOptions struct {
	Tier      htools.ToolTier
	Tags      []string
	Owner     string
	Condition string
	// Action records the capability category used by selected-profile policy.
	// Empty preserves direct custom-registration behavior (no category policy).
	Action htools.Action
}

type Registry struct {
	mu                sync.RWMutex
	tools             map[string]registeredTool
	mcpServers        map[string]struct{} // tracks registered MCP server names to prevent duplicates
	mcpServerTools    map[string][]string // maps server name → tool names registered for that server
	shutdownHooks     []func(context.Context) error
	toolsetResolution ToolsetResolutionSnapshot
	// jobManager owns this registry's background bash jobs. Retained so a
	// cancelled run can terminate the jobs it started — those jobs are
	// detached from the run's context on purpose, so cancellation has to be
	// said explicitly rather than propagated.
	jobManager *htools.JobManager
}

// SetToolsetResolutionSnapshot records the condition resolver's redacted,
// immutable unavailable-toolset result for operator inventory consumers.
func (r *Registry) SetToolsetResolutionSnapshot(snapshot ToolsetResolutionSnapshot) {
	r.mu.Lock()
	r.toolsetResolution = cloneToolsetResolutionSnapshot(snapshot)
	r.mu.Unlock()
}

// ToolsetResolutionSnapshot returns an isolated copy of the resolver result.
func (r *Registry) ToolsetResolutionSnapshot() ToolsetResolutionSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return cloneToolsetResolutionSnapshot(r.toolsetResolution)
}

func cloneToolsetResolutionSnapshot(snapshot ToolsetResolutionSnapshot) ToolsetResolutionSnapshot {
	return ToolsetResolutionSnapshot{
		ConfiguredUnavailable: append([]ConfiguredUnavailableToolset(nil), snapshot.ConfiguredUnavailable...),
		Unavailable:           append([]UnavailableToolsetObservation(nil), snapshot.Unavailable...),
		Incomplete:            snapshot.Incomplete,
		IncompleteReason:      snapshot.IncompleteReason,
	}
}

// SetJobManager records the background-job manager backing this registry.
func (r *Registry) SetJobManager(m *htools.JobManager) {
	r.mu.Lock()
	r.jobManager = m
	r.mu.Unlock()
}

// JobManager returns this registry's background-job manager, or nil.
func (r *Registry) JobManager() *htools.JobManager {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.jobManager
}

func NewRegistry() *Registry {
	return &Registry{
		tools:          make(map[string]registeredTool),
		mcpServers:     make(map[string]struct{}),
		mcpServerTools: make(map[string][]string),
	}
}

func (r *Registry) RegisterShutdownHook(hook func(context.Context) error) {
	if hook == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.shutdownHooks = append(r.shutdownHooks, hook)
}

func (r *Registry) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	hooks := append([]func(context.Context) error(nil), r.shutdownHooks...)
	r.mu.RUnlock()

	var joined error
	for _, hook := range hooks {
		if err := hook(ctx); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	return joined
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
	r.tools[def.Name] = registeredTool{
		def:          def.Clone(),
		handler:      handler,
		tier:         htools.TierCore,
		parallelSafe: def.ParallelSafe,
		mutating:     def.Mutating,
		inflight:     &sync.WaitGroup{},
		owner:        "harness.registry",
		condition:    "direct Register call",
	}
	return nil
}

// ActionFor returns the registered capability category for name. Direct
// custom registrations that do not declare one intentionally return false.
func (r *Registry) ActionFor(name string) (htools.Action, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	if !ok || tool.action == "" {
		return "", false
	}
	return tool.action, true
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
		defs = append(defs, r.tools[name].def.Clone())
	}
	return defs
}

// DefinitionsWithMetadata returns tool definitions together with their tier,
// tags, and registration provenance. Returned values are deeply cloned so
// callers can mutate them safely.
func (r *Registry) DefinitionsWithMetadata() []ToolMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	defs := make([]ToolMetadata, 0, len(names))
	for _, name := range names {
		rt := r.tools[name]
		defs = append(defs, ToolMetadata{
			Definition: rt.def.Clone(),
			Tier:       rt.tier,
			Tags:       copyStrings(rt.tags),
			Owner:      rt.owner,
			Condition:  rt.condition,
		})
	}
	return defs
}

// CatalogTools returns every registered tool as a flat []htools.Tool, sorted by
// name, with the tier and tags the tool was registered under.
//
// It exists so consumers that want the flat catalog rather than the registry
// API — currently the stdio MCP server — are served from the SAME registry the
// HTTP runner uses. The alternative, a second catalog builder assembling its
// own copies of every tool, is exactly what this replaced: it drifted from the
// registry it was supposed to mirror, and security fixes had to be applied
// twice to stay in sync.
func (r *Registry) CatalogTools() []htools.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	catalog := make([]htools.Tool, 0, len(names))
	for _, name := range names {
		rt := r.tools[name]
		def := rt.def.Clone()
		handler := rt.handler
		catalog = append(catalog, htools.Tool{
			Definition: htools.Definition{
				Name:         def.Name,
				Description:  def.Description,
				Parameters:   def.Parameters,
				ParallelSafe: rt.parallelSafe,
				Mutating:     rt.mutating,
				Tier:         rt.tier,
				Tags:         copyStrings(rt.tags),
			},
			Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
				return handler(ctx, args)
			},
		})
	}
	return catalog
}

func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage) (string, error) {
	r.mu.RLock()
	tool, exists := r.tools[name]
	if exists && tool.inflight != nil {
		tool.inflight.Add(1)
	}
	r.mu.RUnlock()
	if !exists {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	if tool.inflight != nil {
		defer tool.inflight.Done()
	}
	return tool.handler(ctx, args)
}

// RegisterWithOptions registers a tool with tier, tag, and provenance metadata.
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
	owner := strings.TrimSpace(opts.Owner)
	if owner == "" {
		owner = "harness.registry"
	}
	condition := strings.TrimSpace(opts.Condition)
	if condition == "" {
		condition = "direct RegisterWithOptions call"
	}
	r.tools[def.Name] = registeredTool{
		def:          def.Clone(),
		handler:      handler,
		tier:         tier,
		tags:         copyStrings(opts.Tags),
		parallelSafe: def.ParallelSafe,
		mutating:     def.Mutating,
		action:       opts.Action,
		inflight:     &sync.WaitGroup{},
		mcpServer:    mcpServerFromTags(opts.Tags),
		owner:        owner,
		condition:    condition,
	}
	return nil
}

// IsParallelSafe reports whether the named tool is safe to execute concurrently
// with other parallel-safe tool calls within the same runner step. Returns
// false for unknown tool names.
func (r *Registry) IsParallelSafe(name string) bool {
	r.mu.RLock()
	rt, ok := r.tools[name]
	r.mu.RUnlock()
	return ok && rt.parallelSafe
}

// IsMutating reports whether the named tool modifies external state (writes
// files, executes commands, etc.). Returns false for unknown tool names.
// Used by the approval broker to decide whether a tool call requires approval
// under ApprovalPolicyDestructive.
func (r *Registry) IsMutating(name string) bool {
	r.mu.RLock()
	rt, ok := r.tools[name]
	r.mu.RUnlock()
	return ok && rt.mutating
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
		defs = append(defs, rt.def.Clone())
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
			defs = append(defs, rt.def.Clone())
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
			Parameters:  deepClonePayload(td.Parameters),
			// MCP servers are external, untrusted code; treat every tool as
			// mutating unless explicitly proven otherwise.
			Mutating: true,
		}
		handler := ToolHandler(func(ctx context.Context, args json.RawMessage) (string, error) {
			return mcpReg.CallTool(ctx, regServer, origName, args)
		})
		r.tools[toolName] = registeredTool{
			def:       def,
			handler:   handler,
			tier:      htools.TierDeferred,
			tags:      []string{"mcp", "integration", "external", "dynamic", "mcp_server:" + serverName},
			inflight:  &sync.WaitGroup{},
			owner:     "harness.mcp",
			condition: fmt.Sprintf("MCP server %q connected at runtime", serverName),
			// Conservative default: every MCP tool is mutating.
			mutating: true,
			// An MCP invocation crosses an external RPC boundary. The harness
			// cannot safely infer the remote server's implementation, so selected
			// profiles denying network access must fail closed before dispatch.
			action:    htools.ActionFetch,
			mcpServer: serverName,
		}
		registered = append(registered, toolName)
	}

	r.mcpServers[serverName] = struct{}{}
	r.mcpServerTools[serverName] = registered
	return registered, nil
}

// UnregisterMCPServer removes a previously registered MCP server and all of
// its tools from the registry. It is a no-op when the server is not registered.
// This is called during per-run cleanup so that the same server name can be
// re-registered on subsequent runs without hitting the "already connected" error.
func (r *Registry) UnregisterMCPServer(serverName string) {
	if serverName == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.mcpServers[serverName]; !exists {
		return
	}

	// Remove each tool that was registered for this server.
	for _, toolName := range r.mcpServerTools[serverName] {
		delete(r.tools, toolName)
	}

	// Remove the server tracking entries.
	delete(r.mcpServers, serverName)
	delete(r.mcpServerTools, serverName)
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

	var retiring []*sync.WaitGroup
	var removeNames []string
	for name, rt := range r.tools {
		if hasToolTag(rt.tags, sourceTag) {
			if rt.inflight != nil {
				retiring = append(retiring, rt.inflight)
			}
			removeNames = append(removeNames, name)
		}
	}
	for _, wg := range retiring {
		wg.Wait()
	}

	// Remove all currently registered tools that carry the source tag.
	for _, name := range removeNames {
		delete(r.tools, name)
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
		owner := "harness.dynamic"
		condition := fmt.Sprintf("dynamic registry source %q resolved", sourceTag)
		if server := mcpServerFromTags(tags); server != "" {
			owner = "harness.mcp"
			condition = fmt.Sprintf("MCP server %q resolved from dynamic source %q", server, sourceTag)
		}

		r.tools[t.Definition.Name] = registeredTool{
			def: ToolDefinition{
				Name:         t.Definition.Name,
				Description:  t.Definition.Description,
				Parameters:   t.Definition.Parameters,
				ParallelSafe: t.Definition.ParallelSafe,
				Mutating:     t.Definition.Mutating,
			},
			handler:      ToolHandler(t.Handler),
			tier:         tier,
			tags:         tags,
			parallelSafe: t.Definition.ParallelSafe,
			mutating:     t.Definition.Mutating,
			action:       t.Definition.Action,
			inflight:     &sync.WaitGroup{},
			mcpServer:    mcpServerFromTags(tags),
			owner:        owner,
			condition:    condition,
		}
	}
	r.rebuildMCPServerToolsLocked()
	return nil
}

func hasToolTag(tags []string, want string) bool {
	for _, tag := range tags {
		if tag == want {
			return true
		}
	}
	return false
}

func mcpServerFromTags(tags []string) string {
	for _, tag := range tags {
		if strings.HasPrefix(tag, "mcp_server:") {
			return strings.TrimSpace(strings.TrimPrefix(tag, "mcp_server:"))
		}
	}
	return ""
}

func (r *Registry) rebuildMCPServerToolsLocked() {
	rebuilt := make(map[string][]string, len(r.mcpServers))
	for server := range r.mcpServers {
		rebuilt[server] = nil
	}
	for name, rt := range r.tools {
		server := rt.mcpServer
		if server == "" {
			server = mcpServerFromTags(rt.tags)
		}
		if server == "" {
			continue
		}
		if _, known := r.mcpServers[server]; !known {
			continue
		}
		rebuilt[server] = append(rebuilt[server], name)
	}
	for server := range rebuilt {
		sort.Strings(rebuilt[server])
	}
	r.mcpServerTools = rebuilt
}
