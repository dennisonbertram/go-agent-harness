# Issue #495: Enrich Tool Descriptions with Behavioral Specs

**Date:** March 31, 2026  
**Status:** Planning Phase  
**Priority:** Medium  

## Executive Summary

Current tool descriptions in `/internal/harness/tools/descriptions/` are purely API documentation — they explain parameters and return values but lack behavioral specifications. This plan proposes a structured system for enriching these descriptions with:

- **Preconditions** — what must be true before calling the tool
- **Side effects** — what state changes result from the tool
- **Error modes** — how the tool fails and recovery strategies  
- **Performance implications** — cost, speed, resource usage
- **Interaction patterns** — when to call in sequence, conflicts
- **Agent decision guidance** — when/why agents should choose this tool

This enables smarter agent decision-making and better user transparency.

---

## Current State Assessment

### Tool Description Inventory

**Total tool description files:** 72 markdown files  
**Total lines of description content:** 969 lines  
**Average description length:** ~13 lines per tool

#### Description Format Categories

**1. Minimal API Docs (35+ tools)**  
Pure parameter/return documentation with no behavioral context.

Examples:
- `cron_create.md` — 1 line: "Create a RECURRING scheduled cron job..."
- `web_search.md` — 11 lines: parameter docs + return format
- `fetch.md` — 20 lines: parameter docs + return format

**2. Extended API Docs with Usage Guidance (25+ tools)**  
Includes "when to use" patterns, examples, and gotchas.

Examples:
- `bash.md` — 34 lines: includes test output interpretation, safety warnings
- `edit.md` — 13 lines: when to use vs write tool
- `read.md` — 15 lines: single-file access pattern guidance
- `agent.md` — 24 lines: delegation use cases (multi-step vs atomic)
- `apply_patch.md` — 70 lines: three patch modes with concrete examples
- `skill.md` — 19 lines: lists built-in actions (list, verify)

**3. Comprehensive Domain Descriptions (12+ tools)**  
Behavioral context mixed with API docs.

Examples:
- `write.md` — JSON validation, file creation semantics
- `git_status.md` — describes git state mapping
- `git_diff.md` — explains staging vs unstaged, revision ranges
- `ls.md` — directory traversal, filtering, depth parameters

#### Tools Without Behavioral Context

**High priority for enrichment (core + frequently used):**
- `bash` — execution model, stdout/stderr handling, background job lifecycle
- `read` — truncation behavior, hash-based addressing, offset/limit semantics
- `write` — atomic writes, truncation vs append, concurrency handling
- `edit` — hash-based line anchors, multiline matching, replacement semantics
- `glob` — single-level matching (*/* vs **/*), recursion depth
- `grep` — regex flavors, case sensitivity defaults, multiline mode
- `agent` — step budget consumption, sub-agent isolation, model selection

**Medium priority (specialized/agentic):**
- `cron_create` — job persistence, execution model, failure handling
- `web_search` — rate limiting, result ordering, quality scores
- `fetch` — timeouts, SSL validation, redirect handling
- `apply_patch` — atomicity guarantees, conflict detection, rollback semantics
- `skill` — version resolution, scope isolation, allowed-tools enforcement
- `AskUserQuestion` — timeout behavior, concurrent requests, answer verification

**Lower priority (infrastructure/admin):**
- `compact_history` — summarization strategy, token accounting
- `context_status` — state consistency guarantees
- `observational_memory` — persistence layer, scope isolation
- `lsp_*` — language server lifecycle, error recovery

---

## Proposed Behavioral Spec Format

Each tool description file will be extended to include optional structural sections:

```markdown
# Tool Name

[Existing API documentation]

## Behavioral Specification

### Preconditions
- Workspace must be a git repository (for git_* tools)
- File must already exist (for edit tool)
- User must have activated the skill (for skill tool)

### Side Effects
- Creates/modifies file at path X
- Stages changes in git index
- Updates cron daemon state
- Sends HTTP request (with side-effect implications)

### Error Modes
- `file_not_found` — requested file does not exist
- `permission_denied` — insufficient access to path
- `conflict` — concurrent modification detected (via version hash)
- Recovery: retry with updated version hash, or use different path

### Atomicity & Consistency
- Single-file operations: atomic (all-or-nothing)
- Patch operations: atomic across multiple files (or fails cleanly)
- Git operations: atomic via git's own transactional semantics

### Performance Characteristics
- Time: linear in file size (for read/write)
- Tokens: ~100 per tool call overhead
- Cost: network tools (web_search, fetch) incur per-request costs
- Limits: max 1MB per write, 256KB per fetch response

### Interaction Patterns
- **Read → Write sequence:** Use version hashes for concurrency control
- **Bash + job_output:** Foreground cmds < 300s; background jobs unlimited
- **Glob + Grep:** Glob for discovery, grep for content search
- **Parallel safety:** read/glob/grep/ls are parallel-safe; write/edit/bash may conflict

### Agent Decision Guidance
- **Choose read over bash cat:** Always prefer read for workspace files
- **Choose edit over write:** Use edit when changing small parts; write for full replacement
- **Choose glob over bash ls:** Glob for name patterns; ls for full directory listing with filtering
- **Choose skill over agent:** Skills have constraints; agents are fully autonomous
- **Cost consideration:** Web tools (search, fetch) are ~100x more expensive than workspace tools
```

### Annotation Strategy

To avoid overwhelming the existing description files, use inline **optional** markers:

```
[BEHAVIORAL SPEC AVAILABLE] for detailed semantics, see behavioral_specs/ directory
```

Then store full specs in a separate indexed directory: `/internal/harness/tools/behavioral_specs/`

---

## Files Requiring Changes

### 1. Tool Description Files (72 files in `/internal/harness/tools/descriptions/`)

**Phase 1 (Priority: High)** — 15 core tools with usage complexity:
- `bash.md` — Add execution model, job lifecycle, timeout semantics
- `read.md` — Add truncation, hashing, offset/limit semantics
- `write.md` — Add atomicity, append vs replace, version handling
- `edit.md` — Add line-hash semantics, multiline matching
- `apply_patch.md` — Add atomicity, rollback semantics
- `glob.md` — Add recursion depth, pattern scoping
- `grep.md` — Add regex flavor, multiline mode
- `agent.md` — Add step budget, isolation, model selection
- `skill.md` — Add scope isolation, allowed-tools enforcement
- `web_search.md` — Add rate limiting, result ranking
- `fetch.md` — Add timeout, SSL, redirect handling
- `cron_create.md` — Add execution model, failure recovery
- `git_status.md` — Add state consistency
- `git_diff.md` — Add revision semantics
- `AskUserQuestion.md` — Add timeout, concurrency handling

**Phase 2 (Priority: Medium)** — 20 specialized/integration tools
**Phase 3 (Priority: Low)** — 37 administrative/infrastructure tools

### 2. New File: Behavioral Spec Index

Create `/internal/harness/tools/behavioral_specs/INDEX.md`:

```markdown
# Behavioral Specifications Index

Companion reference to tool descriptions in /descriptions/.
Maps tool names to behavioral semantics, preconditions, side effects, error modes.

[Auto-generated index with links to spec files]
```

### 3. New Files: Individual Behavioral Specs (Phase 1)

Create `/internal/harness/tools/behavioral_specs/<tool_name>.md` for each Phase 1 tool.

Example: `/internal/harness/tools/behavioral_specs/bash.md`

```markdown
# bash — Execution Model & Job Lifecycle

## Preconditions
- Command must be shell-safe (no dangerous rm -rf / rejected by policy)
- Foreground commands timeout at 300 seconds
- Background commands allow up to 3600 seconds

## Side Effects
- Modifies workspace state (via shell command)
- Creates background job if run_in_background=true
- May affect workspace root or working_dir subdirectory

## Error Modes
- `timeout` — command exceeded timeout_seconds
- `exit_code` — command returned non-zero
- `safety_rejected` — command blocked by policy

## Atomicity
- Single foreground command: "fires and forgets" — no rollback
- Background jobs: managed via job manager with kill support
- Multiple sequential commands: agent is responsible for orchestration

## Performance Characteristics
- Foreground: returns immediately when command completes
- Background: returns shell_id immediately; use job_output to poll
- No timeout → 30s default for foreground
- Cost: no direct LLM cost (unlike web_* tools)

## Interaction Patterns
- run_in_background=true for commands > 30s
- Use job_output(shell_id) to retrieve results
- Combine with job_kill for cleanup
- Pair with glob/grep for file discovery before shell operations
```

### 4. Configuration: TOML Extension (Optional)

Extend `/internal/config/config.go` with optional behavioral-spec fields:

```go
type ToolConfig struct {
    Name                    string `toml:"name"`
    BehavioralSpecsEnabled  bool   `toml:"behavioral_specs_enabled"`
    PreconditionChecking    bool   `toml:"precondition_checking"`
    SideEffectTracing       bool   `toml:"side_effect_tracing"`
    ErrorModeDocumentation  bool   `toml:"error_mode_documentation"`
}
```

Example TOML:
```toml
[tools.bash]
behavioral_specs_enabled = true
precondition_checking = true
side_effect_tracing = true
```

**Rationale:** Allow profiles to opt-in to behavioral guidance without breaking existing tools.

### 5. Embedding & Loading

Extend `/internal/harness/tools/descriptions/embed.go`:

```go
//go:embed *.md
var DescriptionFS embed.FS

//go:embed behavioral_specs/*.md
var BehavioralSpecFS embed.FS

func LoadBehavioralSpec(name string) (string, error) {
    // ...
}
```

### 6. Type System: Add Behavioral Spec Field

Extend `/internal/harness/tools/types.go`:

```go
type Definition struct {
    Name              string         `json:"name"`
    Description       string         `json:"description"`
    BehavioralSpec    string         `json:"behavioral_spec,omitempty"`  // NEW
    Parameters        map[string]any `json:"parameters"`
    // ... existing fields
}
```

---

## Tool Tier Mapping

Current tier classifications (from `types.go`):

```go
const (
    TierCore      ToolTier = "core"        // Always sent to LLM (54 tools)
    TierDeferred  ToolTier = "deferred"    // Hidden until activated (18 tools)
)
```

**Enrichment priority by tier:**

| Tier | Count | Rationale |
|------|-------|-----------|
| Core | 54 | Higher impact on agent behavior; users interact frequently |
| Deferred | 18 | Specialized use cases; behavioral specs enable discovery |

---

## Testing Strategy

### Unit Tests

**New test file:** `/internal/harness/tools/behavioral_specs_test.go`

```go
func TestBehavioralSpecsLoading(t *testing.T) {
    // Verify all Phase 1 tools have behavioral specs
    // Check for required sections (preconditions, side effects, error modes)
    // Validate markdown format and links
}

func TestBehavioralSpecConsistency(t *testing.T) {
    // Verify descriptions match behavioral specs
    // Check for contradictions between description and spec
}
```

### Integration Tests

**Test:** Tool calls with behavioral spec enforcement

```go
func TestBashTimeoutEnforcement(t *testing.T) {
    // Verify foreground timeout at 300s
    // Verify background jobs don't timeout
}

func TestEditVersionHashValidation(t *testing.T) {
    // Verify version hash conflicts rejected
    // Test hash-addressed edits
}

func TestAgentStepBudgetTracking(t *testing.T) {
    // Verify agent tool consumes steps correctly
}
```

### Documentation Tests

**Test:** Behavioral spec examples are executable

```go
func TestBehavioralSpecExamples(t *testing.T) {
    // Parse example code from specs
    // Verify syntax (Go code blocks, shell commands)
    // Test example tool calls succeed
}
```

### Coverage Metrics

- **Phase 1 tools:** 100% coverage (15 tools)
- **Phase 2 tools:** 80% coverage (20 tools)
- **Phase 3 tools:** 50% coverage (37 tools)

---

## Implementation Roadmap

### Phase 1: Foundation (Week 1)
- [ ] Create `/internal/harness/tools/behavioral_specs/` directory
- [ ] Create INDEX.md with structure and guidelines
- [ ] Write behavioral specs for 5 highest-priority core tools:
  - bash, read, write, edit, apply_patch
- [ ] Extend types.go with BehavioralSpec field
- [ ] Extend embed.go to load behavioral specs
- [ ] Add unit tests for loading/validation

### Phase 2: Core Tools (Week 2-3)
- [ ] Write specs for 10 more core tools:
  - glob, grep, agent, skill, web_search, fetch, git_status, git_diff, cron_create, AskUserQuestion
- [ ] Update catalog.go to inject behavioral specs into Definition objects
- [ ] Add integration tests for selected tools
- [ ] Document in /docs/reference/behavioral-specs.md

### Phase 3: Extended Tools (Week 4)
- [ ] Write specs for 20 Phase 2 tools
- [ ] Optional: TOML configuration for behavioral spec features
- [ ] Update profiles with tool-specific behavioral flags
- [ ] Add forensics/tracing support if enabled in config

### Phase 4: Validation & Polish (Week 5)
- [ ] Full test coverage
- [ ] Performance benchmarks (no regression in tool load times)
- [ ] Documentation review
- [ ] User feedback cycle

---

## Priority Matrix

| Tool | Tier | Complexity | Impact | Priority |
|------|------|-----------|--------|----------|
| bash | Core | High | Critical | P0 |
| read | Core | Medium | Critical | P0 |
| write | Core | Medium | Critical | P0 |
| edit | Core | High | Critical | P0 |
| apply_patch | Core | High | High | P0 |
| agent | Core | High | High | P1 |
| skill | Core | High | High | P1 |
| glob | Core | Medium | High | P1 |
| grep | Core | Medium | High | P1 |
| web_search | Core | Medium | High | P1 |
| fetch | Core | Medium | High | P1 |
| cron_create | Deferred | Medium | Medium | P2 |
| git_status | Core | Low | Medium | P2 |
| git_diff | Core | Low | Medium | P2 |
| ls | Core | Low | Medium | P2 |

---

## Configuration Fields Needed

### In `/internal/config/config.go`

**New optional config section:**

```toml
[behavioral_specs]
enabled = true
precondition_checking = false    # Validate inputs before tool execution
side_effect_tracing = false      # Log state changes before/after
error_mode_documentation = true  # Inject error modes into descriptions
spec_injection = "full"          # "none" | "minimal" | "full"
```

**Default values:** All disabled (backward compatible)

---

## Success Criteria

1. **Specification coverage:** 100% of Phase 1 tools have behavioral specs
2. **Description consistency:** No contradictions between descriptions and specs
3. **Test coverage:** ≥90% of new spec code covered by tests
4. **Performance:** Tool catalog build time increases by <5%
5. **User feedback:** Agents demonstrate improved decision-making in behavioral choice tasks

---

## Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|-----------|
| Specs become stale | Medium | Medium | Automated consistency checks in CI |
| Overhead in tool initialization | Low | Medium | Lazy-load specs only when needed |
| Specs too verbose | Medium | Medium | Use tiered detail (summary → full) |
| Inconsistency with code | Medium | High | Link specs to source code line numbers |

---

## Related Issues

- Issue #41: Tool description migration to embedded .md files
- Issue #94: Bash tool test output guidance
- Issue #225: LSP tooling evaluation
- Deferred tools design research

---

## Appendix A: Current Tool Description Statistics

**File size distribution:**

| Lines | Count | Examples |
|-------|-------|----------|
| 1-5 | 12 | cron_create, cron_list, ls (minimal) |
| 6-15 | 28 | web_search, fetch, git_status |
| 16-30 | 18 | bash, edit, read, write |
| 31-70 | 10 | apply_patch (70 lines), agent (24 lines) |
| 70+ | 4 | (specialized tools) |

**Tools with "when to use" guidance:** 25/72 (35%)  
**Tools with error mode documentation:** 8/72 (11%)  
**Tools with performance notes:** 6/72 (8%)  
**Tools with interaction patterns:** 4/72 (6%)  

---

## Appendix B: Behavioral Spec Template

```markdown
# <TOOL_NAME> — <SHORT BEHAVIOR DESCRIPTION>

[Keep existing API documentation as-is]

## Behavioral Specification

### Preconditions
- List conditions that must be true before calling
- Examples: file exists, git repo initialized, user authenticated

### Side Effects
- What state changes result from execution
- Workspace modifications, process state, external calls

### Atomicity & Consistency
- Single operation: atomic or best-effort?
- Multi-operation: transaction support?
- Conflict detection mechanism?

### Error Modes
- Error codes and meanings
- Recovery strategies for each error type

### Performance Characteristics
- Time complexity (input size dependencies)
- Token/cost implications
- Resource usage (memory, network)
- Hard limits and quotas

### Interaction Patterns
- When to call before/after other tools
- Known conflicts or compatibility issues
- Parallel safety guarantees

### Agent Decision Guidance
- Why an agent should choose this tool over alternatives
- When NOT to use this tool
- Common pitfalls and how to avoid them

### Examples
```shell
# Example use case
<tool_invocation>
```
```

---

## Notes for Implementation

1. **Start small:** Deliver Phase 1 (5 tools) before expanding
2. **Validate early:** Get user feedback on spec format/usefulness
3. **Automate validation:** Use CI to check spec consistency vs code
4. **Iterate:** Specs should evolve as tools change
5. **Document assumptions:** Make implicit contract explicit

