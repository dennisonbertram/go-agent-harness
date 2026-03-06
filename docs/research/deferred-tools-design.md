# Deferred Tools (Lazy-Loaded Tools) Design for go-agent-harness

## Problem Statement

The harness currently sends all registered tools (30+) in every LLM completion request via `r.tools.Definitions()` on line 338 of `runner.go`. Each tool definition averages ~200 tokens (name + description + JSON Schema parameters), totaling ~6,000 tokens per turn. Over an 8-step run, that is ~48,000 wasted prompt tokens for tools the model never calls.

## How Claude Code Does It (Observed Behavior)

Claude Code implements a `ToolSearch` meta-tool. The pattern:

1. **Tool categorization**: Tools are split into "always available" (read, write, edit, bash, grep, glob) and "deferred" (MCP tools, specialized tools like LSP, Sourcegraph, Notion, Linear, etc.).
2. **ToolSearch description as catalog**: The `ToolSearch` tool's description contains a complete list of all deferred tool names. This is how the model knows they exist -- it reads the ToolSearch description and sees `mcp__claude_ai_Linear__save_issue`, etc.
3. **Two query modes**: `select:tool_name` for direct selection when the model knows the exact name, or keyword search for fuzzy discovery.
4. **Session-scoped loading**: Once a tool is loaded via ToolSearch, it remains available for the rest of the conversation. The tool definitions are injected into subsequent completion requests.
5. **No re-loading**: If a keyword search already returned a tool, a follow-up `select:` for the same tool is unnecessary -- the search already loaded it.

## Current Architecture (Relevant Code Paths)

```
internal/harness/tools/catalog.go    -- BuildCatalog() creates all Tool instances
internal/harness/tools/types.go      -- Tool{Definition, Handler} struct
internal/harness/registry.go         -- Registry stores tools, Definitions() returns all
internal/harness/runner.go:338       -- r.tools.Definitions() passed to every CompletionRequest
internal/harness/types.go            -- CompletionRequest.Tools []ToolDefinition
internal/provider/openai/client.go   -- mapTools() sends all definitions to OpenAI API
```

Key observation: `Registry.Definitions()` returns ALL registered tools. There is no concept of visibility tiers. The runner calls this on every step of the loop.

## Proposed Design

### 1. Tool Categorization

Add a `Tier` field to `tools.Definition`:

```go
type ToolTier string

const (
    TierCore     ToolTier = "core"     // Always sent to LLM
    TierDeferred ToolTier = "deferred" // Only sent after ToolSearch loads them
)

type Definition struct {
    Name         string         `json:"name"`
    Description  string         `json:"description"`
    Parameters   map[string]any `json:"parameters"`
    Tier         ToolTier       `json:"-"` // NEW
    Action       Action         `json:"-"`
    Mutating     bool           `json:"-"`
    ParallelSafe bool           `json:"-"`
}
```

**Core tools** (always loaded): `read`, `write`, `edit`, `bash`, `job_output`, `job_kill`, `ls`, `glob`, `grep`, `apply_patch`, `git_status`, `git_diff`, `ask_user_question`, `observational_memory`, `tool_search` (the meta-tool itself).

**Deferred tools**: `fetch`, `download`, `lsp_diagnostics`, `lsp_references`, `lsp_restart`, `sourcegraph`, `list_mcp_resources`, `read_mcp_resource`, all dynamic MCP tools, `agent`, `agentic_fetch`, `web_search`, `web_fetch`, `todos`.

### 2. Registry Changes

```go
type Registry struct {
    mu       sync.RWMutex
    tools    map[string]registeredTool
    loaded   map[string]bool // tracks which deferred tools are "active" this session
}

// CoreDefinitions returns only Tier=core tools + any deferred tools that have been loaded.
func (r *Registry) CoreDefinitions() []ToolDefinition { ... }

// DeferredCatalog returns name+description pairs for all deferred tools (for ToolSearch description).
func (r *Registry) DeferredCatalog() []DeferredToolEntry { ... }

// LoadTool marks a deferred tool as active for the session.
func (r *Registry) LoadTool(name string) (ToolDefinition, bool) { ... }

// SearchTools fuzzy-matches deferred tools by query, loads and returns top N.
func (r *Registry) SearchTools(query string, maxResults int) []ToolDefinition { ... }
```

The key change in `runner.go`:

```go
// Before (line 338):
completionReq := CompletionRequest{
    Tools: r.tools.Definitions(),
}

// After:
completionReq := CompletionRequest{
    Tools: r.tools.CoreDefinitions(), // only core + session-loaded tools
}
```

### 3. The `tool_search` Meta-Tool

```go
func toolSearchTool(registry *Registry) Tool {
    catalog := registry.DeferredCatalog()

    // Build the description with all deferred tool names listed
    var sb strings.Builder
    sb.WriteString("Search for or select deferred tools to make them available.\n\n")
    sb.WriteString("Query modes:\n")
    sb.WriteString("1. Keyword search: \"lsp diagnostics\" -> fuzzy match\n")
    sb.WriteString("2. Direct select: \"select:sourcegraph\" -> exact load\n\n")
    sb.WriteString("Available deferred tools (must be loaded before use):\n")
    for _, entry := range catalog {
        sb.WriteString(entry.Name)
        sb.WriteString("\n")
    }

    return Tool{
        Definition: Definition{
            Name:        "tool_search",
            Description: sb.String(),
            Parameters: map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "query": map[string]any{
                        "type":        "string",
                        "description": "Keyword search or \"select:<tool_name>\" for direct selection",
                    },
                    "max_results": map[string]any{
                        "type":        "number",
                        "description": "Max results to return (default 5)",
                        "default":     5,
                    },
                },
                "required": []string{"query"},
            },
            Tier: TierCore, // tool_search itself is always available
        },
        Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
            var input struct {
                Query      string `json:"query"`
                MaxResults int    `json:"max_results"`
            }
            json.Unmarshal(args, &input)
            if input.MaxResults <= 0 {
                input.MaxResults = 5
            }

            if strings.HasPrefix(input.Query, "select:") {
                name := strings.TrimPrefix(input.Query, "select:")
                def, ok := registry.LoadTool(name)
                if !ok {
                    return mustJSON(map[string]any{"error": "tool not found: " + name}), nil
                }
                return mustJSON([]ToolDefinition{def}), nil
            }

            results := registry.SearchTools(input.Query, input.MaxResults)
            return mustJSON(results), nil
        },
    }
}
```

### 4. Search/Matching Strategy

For fuzzy matching on deferred tool names and descriptions:

```go
func (r *Registry) SearchTools(query string, maxResults int) []ToolDefinition {
    r.mu.Lock()
    defer r.mu.Unlock()

    query = strings.ToLower(query)
    keywords := strings.Fields(query)

    // Handle required keyword prefix (+keyword)
    var requiredPrefix string
    filtered := keywords[:0]
    for _, kw := range keywords {
        if strings.HasPrefix(kw, "+") {
            requiredPrefix = strings.TrimPrefix(kw, "+")
        } else {
            filtered = append(filtered, kw)
        }
    }
    keywords = filtered

    type scored struct {
        name  string
        score float64
    }
    var candidates []scored

    for name, tool := range r.tools {
        if tool.def.Tier != TierDeferred {
            continue
        }
        if requiredPrefix != "" && !strings.Contains(strings.ToLower(name), requiredPrefix) {
            continue
        }

        haystack := strings.ToLower(name + " " + tool.def.Description)
        score := 0.0
        for _, kw := range keywords {
            if strings.Contains(haystack, kw) {
                score += 1.0
            }
            // Bonus for name match vs description match
            if strings.Contains(strings.ToLower(name), kw) {
                score += 0.5
            }
        }
        if score > 0 || (requiredPrefix != "" && len(keywords) == 0) {
            candidates = append(candidates, scored{name, score})
        }
    }

    sort.Slice(candidates, func(i, j int) bool {
        return candidates[i].score > candidates[j].score
    })

    results := make([]ToolDefinition, 0, maxResults)
    for i, c := range candidates {
        if i >= maxResults {
            break
        }
        r.loaded[c.name] = true // Auto-load on search
        results = append(results, mapToToolDefinition(r.tools[c.name].def))
    }
    return results
}
```

### 5. Runner Integration

The change in `runner.go` is minimal. Replace line 338:

```go
// runner.go execute() loop
completionReq := CompletionRequest{
    Model:    model,
    Messages: turnMessages,
    Tools:    r.tools.CoreDefinitions(), // Changed from Definitions()
}
```

No other runner changes needed. When `tool_search` is called, it modifies the registry's `loaded` set. On the next iteration of the step loop, `CoreDefinitions()` will include the newly loaded tools.

This works because the runner already re-calls `r.tools.Definitions()` (now `CoreDefinitions()`) on each step iteration. There is no need to "inject" tools mid-turn -- they appear naturally on the next turn after the tool_search call returns.

### 6. Token Savings Analysis

Current state (all 30 tools, every turn):
- ~200 tokens/tool x 30 tools = **6,000 tokens/turn**
- 8-step run = **48,000 tokens** on tool definitions alone

With deferred tools (15 core, 15+ deferred):
- Core: ~200 tokens x 15 = 3,000 tokens/turn
- ToolSearch description (deferred names listed): ~500 tokens
- Total per turn: **3,500 tokens/turn** (42% reduction)
- If 2 deferred tools loaded mid-run: 3,500 + 400 = 3,900 tokens on later turns
- 8-step run: ~**28,000-30,000 tokens** (37-42% savings)

With aggressive deferral (8 core, 22+ deferred):
- Core: ~200 x 8 = 1,600 tokens/turn
- ToolSearch description: ~800 tokens (more names listed)
- Total per turn: **2,400 tokens/turn** (60% reduction)
- 8-step run: ~**19,000-22,000 tokens** (54-60% savings)

### 7. Impact on Model Accuracy

**Will the model reliably search for tools it cannot see?**

Yes, with caveats:

1. **The ToolSearch description is the catalog.** The model sees the full list of deferred tool names in the ToolSearch description. It does not need to "guess" that tools exist -- it can read the names.

2. **Name quality matters.** Deferred tools need descriptive names. `lsp_diagnostics` is self-explanatory; `mcp_tool_7` is not. The model matches its intent to tool names.

3. **Two-step overhead.** Every deferred tool use costs an extra turn (search then use). For frequently-needed tools, this overhead may not be worth it. This is why high-frequency tools stay in the core tier.

4. **Claude Code's evidence.** Claude Code ships this pattern in production with 100+ deferred tools (MCP integrations). The model reliably calls ToolSearch first. GPT-4.1 and Claude both handle this pattern well.

5. **Failure mode is graceful.** If the model tries to call a deferred tool without loading it first, `Registry.Execute()` still finds the tool and executes it. The only issue is that the model might hallucinate the tool's parameter schema. Mitigation: return a clear error message saying "Tool X is deferred. Call tool_search first to load it."

## Alternative Approaches Compared

### A. Intent-Based Filtering

Detect the task type from the prompt and pre-load relevant tool subsets.

| Aspect | Deferred (ToolSearch) | Intent-Based |
|--------|----------------------|--------------|
| Accuracy | Model chooses tools it needs | Heuristic may miss edge cases |
| Flexibility | Works for any combination | Need predefined intent-to-tool mappings |
| Implementation | Simple (meta-tool + tier flag) | Complex (NLP classification or keyword matching) |
| Token cost | Extra turn for search | No extra turns, but may over-include |
| Adaptability | Self-service (model adapts) | Needs maintenance as tools change |

**Verdict**: Intent-based filtering is brittle and requires ongoing maintenance. ToolSearch is self-service.

### B. Tiered Loading (Core + Domain Packs)

Pre-define domain packs: "web_tools", "lsp_tools", "mcp_tools". Load a pack when any tool in it is needed.

This is a middle ground. It reduces per-tool search overhead (load a pack of 5 instead of 5 individual searches) but requires manual pack curation. Can be combined with ToolSearch: the search can load entire packs when one tool matches.

**Verdict**: Good enhancement to ToolSearch. Add a `Pack` field to deferred tools and load the whole pack when any member is searched.

### C. Tool Description Compression

Keep all tools always loaded but shorten descriptions aggressively.

| Aspect | Compression | Deferred |
|--------|------------|----------|
| Token savings | 30-50% per tool | 100% per deferred tool |
| Accuracy impact | Shorter descriptions hurt usage accuracy | Full descriptions when loaded |
| Implementation | Rewrite all descriptions | Add tier field + meta-tool |
| Scalability | Diminishing returns at 50+ tools | Scales to 100+ tools |

**Verdict**: Compression is complementary. Apply it to core tools for additional savings, but it cannot replace deferral for large tool sets.

### D. Dynamic Tool Pruning (Remove After N Unused Turns)

Track tool usage per run. After N turns without use, remove a tool from subsequent requests.

This is risky: the model might need a tool on turn 7 that it last used on turn 2. Pruning it on turn 5 would fail. The model has no way to "ask for it back" unless you also have ToolSearch.

**Verdict**: Unnecessary if ToolSearch exists. ToolSearch already provides on-demand loading. Pruning adds complexity with marginal benefit.

## Practical Considerations

### OpenAI vs Anthropic Differences

**OpenAI (current provider)**:
- Tools are sent in the `tools` array of the chat completion request.
- Tool definitions can change between turns within the same conversation. The API is stateless -- each request is independent.
- No special handling needed for mid-conversation tool injection. Just include the new tool definitions in the next request.

**Anthropic**:
- Same pattern: tools are in the request body and can change between messages.
- Anthropic supports tool `cache_control` hints for prompt caching. Deferred tools that get loaded mid-run would NOT benefit from prompt caching on their first appearance (cache miss). Core tools benefit from caching every turn.
- This makes the core/deferred split even more valuable for Anthropic: core tools get cached, deferred tools only add cache-miss cost when actually needed.

### How Tool Injection Mid-Conversation Works

Both OpenAI and Anthropic chat completion APIs are **stateless per request**. The server does not remember what tools were available on previous turns. This means:

1. You can add new tools to any request -- the model sees them immediately.
2. You can remove tools -- the model will not try to call them (unless it hallucinates from earlier context where it saw them).
3. The only state is in the message history. If a previous assistant message contains a tool call, the model knows that tool existed. If the tool is no longer in the `tools` array, the model may still reference it in text but should not try to call it.

For our design: once `tool_search` loads a tool, we add it to `CoreDefinitions()` for all subsequent turns. We never remove loaded tools within a run. This is safe and consistent.

### Persistence Across Runs

**Within a run**: Loaded tools persist for the rest of the run (session cache in the `loaded` map).

**Across runs**: Loaded tools should NOT persist by default. Each run starts fresh with only core tools. Reasons:
- Different runs may have different tasks and need different tools.
- Stale loaded sets waste tokens on irrelevant tools.
- The cost of re-searching is one extra turn, which is acceptable.

**Exception**: If the harness supports multi-run conversations (same `conversation_id`), consider persisting loaded tools across runs in the same conversation. This avoids re-searching for the same tools when continuing a task.

### Registry Thread Safety

The current `Registry` is already thread-safe with `sync.RWMutex`. The `loaded` map addition follows the same pattern. `LoadTool` and `SearchTools` acquire a write lock; `CoreDefinitions` acquires a read lock.

### Backward Compatibility

The change is backward-compatible:
- `Registry.Definitions()` continues to return ALL tools (for tests, CLI, etc.)
- `Registry.CoreDefinitions()` is the new method used by the runner
- Existing `BuildCatalog` just needs to set `Tier` on each tool
- No API changes to the HTTP server or run request format

## Implementation Plan

### Phase 1: Core Infrastructure (Low Risk)

1. Add `Tier` field to `tools.Definition`
2. Set `Tier` values in `BuildCatalog()` for each tool
3. Add `loaded` map to `Registry`
4. Implement `CoreDefinitions()`, `DeferredCatalog()`, `LoadTool()`, `SearchTools()`
5. Implement `tool_search` tool in `internal/harness/tools/tool_search.go`

### Phase 2: Runner Integration (Medium Risk)

6. Change `runner.go` line 338 to use `CoreDefinitions()`
7. Pass registry reference to `tool_search` tool (requires `BuildCatalog` to accept registry or return tool_search separately)
8. Add "tool not loaded" error handling in `Registry.Execute()` for deferred tools

### Phase 3: Observability

9. Emit events: `tool.search.query`, `tool.loaded`, `tool.deferred_catalog_size`
10. Add `loaded_tools` to run metadata for debugging

### Phase 4: Optimization (Optional)

11. Tool packs (group related deferred tools)
12. Description compression for core tools
13. Per-conversation tool persistence for multi-run conversations
14. Usage analytics to tune core vs deferred classification

## Circular Dependency: tool_search Needs Registry

The `tool_search` tool needs a reference to the `Registry` to call `LoadTool` and `SearchTools`. But the `Registry` is built from `BuildCatalog`, and `tool_search` is one of the tools in the catalog.

**Solution**: Two-phase initialization.

```go
// In catalog.go or a new file:
func BuildCatalogWithDeferral(opts BuildOptions) ([]Tool, *DeferredToolManager, error) {
    // 1. Build all tools as before
    allTools := buildAllTools(opts)

    // 2. Separate into core and deferred
    manager := NewDeferredToolManager()
    var coreTools []Tool
    for _, t := range allTools {
        if t.Definition.Tier == TierDeferred {
            manager.RegisterDeferred(t)
        } else {
            coreTools = append(coreTools, t)
        }
    }

    // 3. Create tool_search with reference to manager
    searchTool := toolSearchTool(manager)
    coreTools = append(coreTools, searchTool)

    return coreTools, manager, nil
}

// DeferredToolManager is separate from Registry to avoid circular deps
type DeferredToolManager struct {
    mu       sync.RWMutex
    deferred map[string]Tool
    loaded   map[string]bool
}
```

Then in `tools_default.go`, inject the manager into the registry:

```go
func NewDefaultRegistryWithOptions(workspaceRoot string, opts DefaultRegistryOptions) *Registry {
    coreTools, deferredMgr, err := htools.BuildCatalogWithDeferral(opts)

    registry := NewRegistry()
    registry.deferredManager = deferredMgr

    // Register ALL tools (core + deferred) for execution
    // But only expose core tools in Definitions
    for _, t := range allTools {
        registry.Register(t.Definition, t.Handler)
    }
    return registry
}
```

## Open Questions

1. **Should tool_search return full definitions or just names+descriptions?** Claude Code returns full definitions (including parameter schemas) so the model can immediately call the tool. This is the right approach -- returning only names would require a second search.

2. **What if the deferred tool list is very large (100+)?** The ToolSearch description grows linearly. At ~10 tokens per tool name, 100 tools = 1,000 tokens just for the catalog in the description. This is still less than sending 100 full definitions (~20,000 tokens). For very large sets, consider grouping by server/domain prefix.

3. **Should the model be able to call deferred tools without searching?** Currently `Registry.Execute()` can execute any registered tool. If the model hallucinates a deferred tool call (having seen the name in ToolSearch's description), it would succeed. This is arguably fine -- the tool works correctly, we just miss the "loading" signal. The only downside is the model may guess wrong parameter schemas. Mitigation: return an error with the correct schema.

4. **How to handle MCP tools that appear/disappear at runtime?** Dynamic MCP tools are already loaded at catalog build time. If MCP servers change during a run, the deferred catalog would be stale. This is a pre-existing issue unrelated to deferral.
