# Issue #495: Quick Reference Guide

## Current Tool Description System

**Location:** `/internal/harness/tools/descriptions/`  
**Count:** 72 .md files  
**Total content:** 969 lines  

### Embedded Loading
- Uses Go `embed.FS` in `/internal/harness/tools/descriptions/embed.go`
- Loaded via `descriptions.Load(toolName)` at tool registration time
- Descriptions injected into `Definition.Description` field in `types.go`

### Tool Organization
- **TierCore (54 tools):** Always visible to LLM
- **TierDeferred (18 tools):** Hidden until activated

---

## Proposed Behavioral Specs Structure

```
/internal/harness/tools/
├── descriptions/          [EXISTING]
│   ├── *.md              (API-focused tool descriptions)
│   └── embed.go
├── behavioral_specs/      [NEW DIRECTORY]
│   ├── INDEX.md
│   ├── bash.md
│   ├── read.md
│   ├── write.md
│   ├── edit.md
│   ├── apply_patch.md
│   └── ...
└── behavioral_specs_test.go [NEW]
```

---

## What Needs to Change

### 1. Type System (`/internal/harness/tools/types.go`)

```go
// Line ~71 in Definition struct, add:
type Definition struct {
    Name             string         `json:"name"`
    Description      string         `json:"description"`
    BehavioralSpec   string         `json:"behavioral_spec,omitempty"`  // ← NEW
    Parameters       map[string]any `json:"parameters"`
    // ... rest unchanged
}
```

### 2. Embed & Loading (`/internal/harness/tools/descriptions/embed.go`)

```go
package descriptions

import "embed"

//go:embed *.md
var FS embed.FS

//go:embed behavioral_specs/*.md
var BehavioralSpecFS embed.FS  // ← NEW

func LoadBehavioralSpec(name string) (string, error) {  // ← NEW
    data, err := BehavioralSpecFS.ReadFile("behavioral_specs/" + name + ".md")
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(string(data)), nil
}
```

### 3. Tool Catalog (`/internal/harness/tools/catalog.go`)

In `BuildCatalog()` function, when creating tool definitions:

```go
// After creating Definition struct:
def := Definition{
    Name: "bash",
    Description: descriptions.Load("bash"),
    // ...
}

// Add behavioral spec (if available):
if spec, err := descriptions.LoadBehavioralSpec("bash"); err == nil {
    def.BehavioralSpec = spec
}
```

### 4. Configuration (`/internal/config/config.go`)

Add new struct around line 170:

```go
type BehavioralSpecsConfig struct {
    Enabled                  bool   `toml:"enabled"`
    PreconditionChecking     bool   `toml:"precondition_checking"`
    SideEffectTracing        bool   `toml:"side_effect_tracing"`
    ErrorModeDocumentation   bool   `toml:"error_mode_documentation"`
    SpecInjection            string `toml:"spec_injection"`  // "none" | "minimal" | "full"
}

// In Config struct:
type Config struct {
    // ... existing fields
    BehavioralSpecs BehavioralSpecsConfig `toml:"behavioral_specs"`
}

// In Defaults():
BehavioralSpecs: BehavioralSpecsConfig{
    Enabled: false,  // opt-in for backward compatibility
    SpecInjection: "minimal",
}
```

### 5. Testing (`/internal/harness/tools/behavioral_specs_test.go`)

```go
package tools

import "testing"

func TestBehavioralSpecsLoading(t *testing.T) {
    // List of Phase 1 tools
    phase1Tools := []string{
        "bash", "read", "write", "edit", "apply_patch",
        "glob", "grep", "agent", "skill", "web_search",
        "fetch", "cron_create", "git_status", "git_diff",
        "AskUserQuestion",
    }
    
    for _, tool := range phase1Tools {
        spec, err := descriptions.LoadBehavioralSpec(tool)
        if err != nil {
            t.Errorf("Failed to load spec for %s: %v", tool, err)
        }
        if spec == "" {
            t.Errorf("Spec for %s is empty", tool)
        }
        // Check required sections
        requiredSections := []string{
            "## Preconditions",
            "## Side Effects",
            "## Error Modes",
        }
        for _, section := range requiredSections {
            if !strings.Contains(spec, section) {
                t.Errorf("Spec for %s missing section: %s", tool, section)
            }
        }
    }
}
```

---

## Phase 1 Tools (Priority Implementation Order)

1. **bash** — Complex: execution model, timeouts, background jobs
2. **read** — Complex: truncation, hashing, offset/limit
3. **write** — Medium: atomicity, append vs replace
4. **edit** — Complex: line hashes, multiline matching
5. **apply_patch** — Complex: atomicity, multi-file semantics

Then expand to:
6. glob, grep, agent, skill, web_search, fetch, cron_create, git_status, git_diff, AskUserQuestion

---

## Behavioral Spec Template

```markdown
# <TOOL_NAME> — <SHORT BEHAVIOR DESCRIPTION>

[Keep existing description from descriptions/*.md]

## Behavioral Specification

### Preconditions
- What must be true before calling
- File exists, git repo initialized, etc.

### Side Effects
- What state changes occur
- Workspace modifications, process state, external calls

### Error Modes
- Error codes and meanings
- Recovery strategies

### Atomicity & Consistency
- Single vs multi-operation semantics
- Conflict detection (version hashes, etc.)

### Performance Characteristics
- Time complexity
- Token/cost implications
- Resource usage and limits

### Interaction Patterns
- Which tools pair well
- Known conflicts
- Parallel safety

### Agent Decision Guidance
- When to choose vs alternatives
- Common pitfalls
- Cost/benefit tradeoffs
```

---

## Key Configuration Points

### Tool Description Loading

Current: `descriptions.Load("bash")` → returns markdown string  
Future: `descriptions.Load("bash")` + `descriptions.LoadBehavioralSpec("bash")`

### Embedding Strategy

- Descriptions: Embedded in current FS
- Behavioral specs: New FS with `//go:embed behavioral_specs/*.md`
- Keeps separation of concerns (API docs vs behavioral context)

### Backward Compatibility

- Config feature is opt-in (defaults disabled)
- Behavioral spec field in Definition is optional (omitempty)
- Existing tools continue working unchanged
- Specs can be added gradually (Phase 1 → Phase 3)

---

## Metrics & Success Measures

**Phase 1 Completion Criteria:**
- 15 tools with behavioral specs
- Specs have all required sections
- No contradictions with descriptions
- Zero test failures
- < 5% increase in tool catalog build time

**Long-term Goals:**
- 72 tools with behavioral specs (100% coverage)
- Agent decision-making improvements measurable
- Automated consistency checks in CI

---

## Related Code Locations

| File | Purpose | Changes Needed |
|------|---------|-----------------|
| `/internal/harness/tools/types.go` | Tool type system | Add BehavioralSpec field |
| `/internal/harness/tools/catalog.go` | Tool registration | Inject behavioral specs |
| `/internal/harness/tools/descriptions/embed.go` | Asset embedding | New embedded FS + loader |
| `/internal/config/config.go` | Configuration | New behavioral specs config section |
| `/internal/harness/tools/*_test.go` | Tests | Add behavioral spec tests |

---

## Next Actions

1. Create `/internal/harness/tools/behavioral_specs/` directory
2. Review this plan with team
3. Write bash.md behavioral spec (start with Phase 1 highest-priority)
4. Implement types.go + embed.go changes
5. Add basic tests for loading/validation
6. Iterate based on feedback

