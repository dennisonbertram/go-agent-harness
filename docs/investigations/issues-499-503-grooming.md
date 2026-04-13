# Issue Grooming Assessment: #499–503

**Assessment date:** 2026-03-31  
**Assessor notes:** Codebase analysis via grep + read of existing type definitions, runner loop, config schema, and system prompt infrastructure.

---

## Issue #499: P1 Implement MaxTurns budget for background and forked agents

### Summary
Add a hard turn budget to ForkConfig/profile system. Background agents should have explicit turn limits (e.g., 2 turns for memory extraction).

### Already Resolved?
**NO** — Not implemented. Evidence:
- `ForkConfig.MaxSteps` exists (line 217–219 in `internal/harness/tools/types.go`) but this is **already-in-use for general step budgets**
- `ProfileRunner.MaxSteps` exists (line 71 in `internal/profiles/profile.go`) — this is what `run_agent` uses
- Step enforcement is in runner_step_engine.go:106 with `for step := 1; effectiveMaxSteps == 0 || step <= effectiveMaxSteps; step++`
- **No MaxTurns field** exists; no separate turn counter distinct from step counter

### Well-Specified?
**YES** — The issue clearly states:
- Defaults: unlimited (interactive), 5 (forked), 2 (background extraction)
- Where it goes: `ForkConfig` AND profile schema (`max_turns` TOML field)
- Enforcement point: step loop in `runner.go`
- Observability: forensic event on exhaustion

### Acceptance Criteria Present?
**Partially** — Implicit but not explicit. Must add:
- [x] `MaxTurns` field to `ForkConfig` (go struct)
- [x] `max_turns` field to `ProfileRunner` (TOML schema)
- [x] Enforcement in step loop (stop after N assistant turns)
- [x] Forensic event emission on turn budget exhaustion
- [x] Documentation of default behaviors by agent type

### Scope
**ATOMIC** — All changes are contained:
- Tools layer: `ForkConfig` struct
- Profiles layer: `ProfileRunner` struct
- Harness layer: step loop enforcement + event emission
- Config parsing: TOML schema updates
- No cascading dependencies

### Dependencies
**NONE** — This is independent. Not blocking #500–503.

### Effort Estimate
**MEDIUM** (2–3 days)

**Rationale:**
- Field additions: trivial (30 mins)
- Step loop enforcement: moderate (need to distinguish assistant turns from total steps; ~4 hours)
- Profile schema + defaults: simple (1 hour)
- Testing: medium (edge cases around turn exhaustion, profile inheritance; ~4 hours)
- Documentation: 1 hour

### Key Files to Change
```
internal/harness/tools/types.go               # Add MaxTurns to ForkConfig
internal/profiles/profile.go                  # Add max_turns to ProfileRunner
internal/harness/runner.go                    # Apply max_turns in profile application logic
internal/harness/runner_step_engine.go        # Count assistant turns; stop at budget
internal/config/config.go                     # Add MaxTurns parsing if needed
internal/harness/tools/core/skill.go          # Pass MaxTurns to ForkConfig construction
internal/harness/tools/deferred/spawn_agent.go # Pass MaxTurns from agent request
internal/harness/events.go                    # Add EventMaxTurnsExhausted event type
```

### TOML Config Fields Needed
**Profile level** (`~/.harness/profiles/<name>.toml`):
```toml
[runner]
max_turns = 5  # per-profile default
```

**ForkConfig propagation** (internal only, no user-facing TOML):
- Already has `MaxSteps` field
- Will add `MaxTurns` (mirrors max_turns from profile)

### Implementation Notes
- **Critical:** Turn counting must only count **assistant messages**, not user messages or tool results
- `runner_step_engine.go` has `step` counter (currently 1-indexed); will need parallel `assistantTurnCount` to track actual assistant turns
- Profile inheritance: if child profile doesn't set `max_turns`, inherit from parent (already supported by profile resolution logic)
- Event emission should include: runID, step number, max_turns limit, message about exhaustion

---

## Issue #500: P1 Add named anti-patterns and evidence requirements to verification

### Summary
Enhance verification tool descriptions to explicitly name anti-patterns and require executable evidence. Every PASS must include a `Command run:` block with actual output.

### Already Resolved?
**NO** — Partially addressed. Evidence:
- `verify_skill` tool exists (internal/harness/tools/verify_skill.go + description at internal/harness/tools/descriptions/verify_skill.md)
- Current description (5 lines) lists checks but **makes no mention of anti-patterns**
- **No VERDICT field** in verify_skill output format; no command-run requirement
- No system prompt guidance about "verification avoidance" or "first-80% seduction"

### Well-Specified?
**YES** — The issue is explicit:
- Named anti-pattern 1: "verification avoidance" (reading code instead of running it)
- Named anti-pattern 2: "first-80% seduction" (declaring success when easy parts work)
- Evidence requirement: Every PASS must have `Command run:` block with output
- Output format: `VERDICT: PASS|FAIL|PARTIAL`

### Acceptance Criteria Present?
**Partially** — Implicit structure but no acceptance tests defined. Must add:
- [x] Update all verification-related tool descriptions
- [x] Add anti-pattern naming (2 specific patterns)
- [x] Add structured VERDICT output format
- [x] Add evidence requirement clause
- [x] Apply to `verify_skill` at minimum; consider other validation tools

### Scope
**NOT FULLY ATOMIC** — Scope creep risk:
- **Minimal scope:** Update verify_skill.md + handler output format
- **Expanded scope:** Apply anti-pattern guidance to ALL validation tools (code_review, test_runner, etc.)
- **Recommendation:** Start with verify_skill; make pattern applicable to others

### Dependencies
**NONE** — Can be done independently of #499, #501–503.

### Effort Estimate
**SMALL-MEDIUM** (1–2 days)

**Rationale:**
- Description updates: 1 hour
- Handler output format change (add VERDICT): 2 hours
- Testing + validation: 3 hours
- (If expanding to other tools: +6 hours per tool)

### Key Files to Change
```
internal/harness/tools/descriptions/verify_skill.md
internal/harness/tools/verify_skill.go        # Add VERDICT field to output, add anti-pattern warnings
internal/harness/tools/core/skill.go          # May need anti-pattern guidance in skill runner
internal/harness/tools/descriptions/skill.md  # If skill creation needs anti-pattern guidance
```

### TOML Config Fields Needed
**None** — This is tool description + handler logic, not config.

### Implementation Notes
- VERDICT output should be **exact string match** (per issue spec): `VERDICT: PASS`, `VERDICT: FAIL`, `VERDICT: PARTIAL`
- Anti-pattern text should be in **tool description** (visible to LLM) so model sees it before using tool
- Command-run evidence: Consider whether `Command run:` should be a **structured field** in JSON output, or just prose requirement
- Test case: Verify that a tool call returning `VERDICT: PASS` without `Command run:` block is flagged as incomplete

---

## Issue #501: P2 Redesign memory system with typed taxonomy, capped index, and on-demand retrieval

### Summary
Implement structured memory system with 4 memory types, capped 200-line index, and on-demand relevance retrieval.

### Already Resolved?
**PARTIALLY** — Significant infrastructure exists. Evidence:
- Memory system exists: `internal/observationalmemory/` directory + `types.go`
- Current types: `Record`, `Status`, `Operation`, `Marker`, `TranscriptMessage` (no typed taxonomy)
- Index file: No evidence of 200-line cap or separate index file structure
- Relevance selector: Not present; no secondary LLM call for picking top-5 files
- **Drift protection:** No evidence in system prompt or runner of "verify memory claims against current file state"
- Feedback type: No distinction between corrections and validations

### Well-Specified?
**YES** — The issue provides detailed implementation guidance:
- 4 types: user, feedback, project, reference (closed taxonomy)
- Index cap: 200 lines / 25KB
- Retrieval: On-demand via relevance selector (secondary LLM call, top-5 files)
- Drift protection: Must verify claims against current file state
- **Critical detail:** Drift protection section MUST be a section header (`##`), not a bullet (0/3 vs 3/3 pass rate)
- Feedback savings: Both corrections AND validations

### Acceptance Criteria Present?
**YES** — Explicit but complex:
- [x] Design memory schema with typed frontmatter
- [x] Implement index cap enforcement (200 lines / 25KB)
- [x] Implement relevance selector (secondary LLM call)
- [x] Add memory drift protection (section header, not bullet)
- [x] Add feedback type with corrections + validations
- [x] Integrate into system prompt

### Scope
**LARGE** — This is architectural:
- New memory file format (with frontmatter)
- New index manager (cap enforcement)
- New relevance selector (LLM call)
- System prompt changes (drift protection section)
- Existing observationalmemory/ integration

### Dependencies
**#502** (system-reminder tag injection) would help with drift protection section injection, but not required.

### Effort Estimate
**LARGE** (5–7 days)

**Rationale:**
- Schema design + file format: 8 hours
- Index cap enforcement + rollover: 6 hours
- Relevance selector (secondary LLM call): 8 hours
- Drift protection section integration: 4 hours
- Feedback type redesign: 3 hours
- Integration testing: 8 hours
- Edge cases (large index shrinking, concurrent writes): 4 hours

### Key Files to Change
```
internal/observationalmemory/types.go         # Add memory type enum, taxonomy
internal/observationalmemory/manager.go       # Implement index cap + on-demand loading
internal/observationalmemory/index.go         # New file: index cap enforcement (200 lines/25KB)
internal/observationalmemory/relevance.go     # New file: secondary LLM call for top-5 selection
internal/observationalmemory/drift_protection.go # New file or integrated: verify claims
internal/harness/runner.go                    # Integrate drift protection into system prompt
internal/systemprompt/system_prompt.go        # Add drift protection section (must be ##)
```

### TOML Config Fields Needed
**Memory config level:**
```toml
[memory]
enabled = true
index_max_lines = 200
index_max_bytes = 25600  # 25KB
relevance_selector_enabled = true
drift_protection_enabled = true
drift_protection_model = "claude-opus-4.6"  # or inherit from main model
```

### Implementation Notes
- **Memory file format** should include frontmatter:
  ```yaml
  ---
  name: "Memory Name"
  description: "What this memory is about"
  type: "user|feedback|project|reference"
  created_at: "2026-03-31T12:00:00Z"
  ---
  
  # Actual memory content
  ```
- **Index file** (MEMORY.md or similar): Listed entries with line count and byte count tracking
- **Drift protection section:** Must be in system prompt as:
  ```markdown
  ## Before recommending from memory
  
  Verify memory claims against current file state by reading the files mentioned. 
  Do not cite memory that contradicts current reality.
  ```
- **Feedback type** should store:
  - Corrections: "User corrected me on X, the right approach is Y"
  - Validations: "User validated that approach Z works well for task W"
- **Relevance selector:** Call Claude mini (or same model) with query + memory index to pick top 5 files per query

---

## Issue #502: P2 Implement <system-reminder> tag injection for mid-conversation dynamic content

### Summary
Add `<system-reminder>` tag mechanism for injecting dynamic system content without busting prompt cache.

### Already Resolved?
**NO** — Not implemented. Evidence:
- No `<system-reminder>` mechanism exists in codebase
- Current system prompt is static + cached
- Tool catalog updates would require full prompt cache invalidation (not done)
- No injection pipeline for volatile content (tool listings, plugin state, etc.)

### Well-Specified?
**PARTIALLY** — The issue explains WHAT but not HOW:
- **What:** `<system-reminder>` tags injected into conversation messages
- **Where:** Between system prompt and user messages (per Claude Code architecture)
- **Why:** Volatile content (tool catalog, plugin updates) doesn't invalidate prompt cache
- **What NOT:** These tags bear no relation to tool results or user messages (system tells model this)

### Acceptance Criteria Present?
**Implicit** — Must define:
- [x] Define `<system-reminder>` tag format (XML-like? Markdown?)
- [x] Add explanation to system prompt about these tags
- [x] Implement injection mechanism in message pipeline
- [x] Use for tool catalog changes, plugin state, dynamic hints
- [x] Ensure injected content doesn't affect cache key

### Scope
**MEDIUM** — Localized to message pipeline:
- Message building: tools layer
- System prompt update: systemprompt package
- Cache key calculation: runner or cache layer
- Tests: message pipeline tests

### Dependencies
**NONE** — But would **enhance #501** (memory system) by allowing drift protection section to be injected without cache bust.

### Effort Estimate
**SMALL-MEDIUM** (2–3 days)

**Rationale:**
- Tag format design: 2 hours
- System prompt update: 1 hour
- Message injection pipeline: 4 hours
- Cache key audit (ensure tags don't affect it): 3 hours
- Testing: 3 hours
- Documentation: 1 hour

### Key Files to Change
```
internal/harness/tools/types.go               # Define system-reminder constants/format
internal/harness/tools/deferred/run_agent.go  # Inject system-reminder tags in message building
internal/harness/runner.go                    # Ensure cache key ignores system-reminder tags
internal/systemprompt/system_prompt.go        # Add section explaining system-reminder tags
internal/harness/runner_test.go               # Test that cache key is unaffected
```

### TOML Config Fields Needed
**None** — This is internal mechanism, no user-facing config.

### Implementation Notes
- **Tag format:** Recommend `<system-reminder>KEY: VALUE</system-reminder>` or similar XML-like format
- **Placement:** Insert after system prompt but before user/assistant messages
- **Content examples:**
  - `<system-reminder>tools_updated: skill_x, code_review</system-reminder>`
  - `<system-reminder>plugin_state: memory_enabled=true, caching_version=v2</system-reminder>`
- **System prompt text:** "System-reminder tags are internal system metadata injected by the harness. They bear no direct relation to specific tool results or user messages in which they appear. You may ignore them or use them as hints, at your discretion."
- **Cache key:** Explicitly strip `<system-reminder>` tags before computing cache key to prevent false invalidations

---

## Issue #503: P2 Add coordinator synthesis doctrine with anti-patterns to orchestrator

### Summary
Enhance orchestrator/coordinator system prompt with explicit synthesis requirements and anti-patterns.

### Already Resolved?
**NO** — Not implemented. Evidence:
- `internal/symphd/orchestrator.go` exists but is workspace/dispatch coordination
- Orchestrator system prompt: Not found in codebase; likely inherited from generic subagent prompt
- No evidence of anti-pattern guidance ("never write 'based on your findings'")
- No worked examples (WRONG vs RIGHT delegation patterns)
- No synthesis verification step before dispatching to workers

### Well-Specified?
**YES** — The issue provides clear anti-patterns and examples:
- Anti-pattern: "never write 'based on your findings'"
- Requirement: Worker prompts must include specific file paths and line references
- Include WRONG vs RIGHT worked examples
- Consider synthesis verification step pre-dispatch

### Acceptance Criteria Present?
**Implicit** — Must define:
- [x] Add anti-pattern guidance to orchestrator/coordinator system prompt
- [x] Add requirement about file paths + line numbers in worker prompts
- [x] Include worked examples (WRONG: "based on your findings"; RIGHT: "in /path/file.go line 42–58")
- [ ] Optional: Synthesis verification step (model confirms it understood research before dispatching)

### Scope
**SMALL** — Localized to system prompt:
- Orchestrator system prompt (one file or function)
- Coordinator system prompt (if separate)
- Optional: Verification step (new function in dispatcher/orchestrator)

### Dependencies
**#502** (system-reminder injection) could be used to inject updated anti-patterns without cache bust, but not required.

### Effort Estimate
**SMALL** (1–2 days)

**Rationale:**
- Identify current orchestrator prompt location: 1 hour
- Draft anti-patterns + worked examples: 2 hours
- Integrate into system prompt: 1 hour
- Test with sample coordinator prompts: 3 hours
- (Optional) Synthesis verification step: 4 hours

### Key Files to Change
```
internal/symphd/orchestrator.go               # If system prompt defined here
internal/subagents/system_prompt_test.go      # System prompt construction
internal/subagents/inline_manager.go          # Coordinator system prompt (if separate)
internal/symphd/dispatcher.go                 # Optional: synthesis verification before dispatch
internal/systemprompt/system_prompt.go        # If orchestrator prompt defined here
```

### TOML Config Fields Needed
**None** — This is system prompt guidance, not user-facing config.

### Implementation Notes
- **Anti-pattern section** should be early in coordinator prompt:
  ```markdown
  ## Synthesis Requirements
  
  ### Anti-patterns to avoid
  - WRONG: "Based on your findings, the issue is..."
  - WRONG: "From my research, I learned that..."
  
  ### Correct synthesis
  - RIGHT: "In /path/to/file.go lines 42–58, the function calculates X by..."
  - RIGHT: "The test in tests/cases/test_foo.go:112 validates that Y occurs when..."
  
  Every claim you make in worker prompts must be grounded in specific file locations.
  ```
- **Worked examples:** Include 3–4 pairs (WRONG vs RIGHT) showing proper file path citations
- **Optional synthesis verification:** Before dispatcher sends request to worker, run coordinator prompt through mini-model asking: "Does this prompt clearly cite specific file locations? Are all claims grounded? VERDICT: YES/NO"

---

## Summary Table

| Issue | Title | Resolved? | Well-Spec'd? | Scope | Effort | Dependencies | Blocker Risk |
|-------|-------|-----------|-------------|-------|--------|--------------|--------------|
| **499** | MaxTurns budget | NO | YES | ATOMIC | MEDIUM | None | LOW |
| **500** | Anti-patterns + VERDICT | NO | YES | ATOMIC | SMALL-MEDIUM | None | LOW |
| **501** | Typed memory system | PARTIAL | YES | LARGE | LARGE | #502 helpful | MEDIUM |
| **502** | system-reminder tags | NO | PARTIAL | MEDIUM | SMALL-MEDIUM | None | LOW |
| **503** | Coordinator anti-patterns | NO | YES | SMALL | SMALL | #502 helpful | LOW |

---

## Recommended Implementation Order

1. **#499 (MaxTurns)** — Independent, P1, medium effort. Start here.
2. **#500 (Anti-patterns/VERDICT)** — Independent, P1, small effort. Do in parallel with #499.
3. **#502 (system-reminder)** — Independent, P2, small effort. Enables better #501 + #503.
4. **#501 (Typed memory)** — P2, large, complex. After #502 for drift protection injection.
5. **#503 (Coordinator anti-patterns)** — P2, small. Can run in parallel with #501.

---

## Risks & Mitigation

### High-Risk Areas
- **#501 memory index capping:** Could lose data if rollover logic is wrong. Mitigation: Extensive testing of edge cases (full index, shrinking, concurrent writes).
- **#499 turn counting:** Distinguishing assistant turns from steps is subtle. Mitigation: Clear test matrix (e.g., "5 assistant messages = 5 turns regardless of step count").
- **#502 cache key:** If system-reminder tags affect cache, could cause cache thrashing. Mitigation: Explicit cache key test that verifies tags are stripped.

### Medium-Risk Areas
- **#501 drift protection section header:** Per issue, must be `##` section, not bullet. Mitigation: Acceptance test: "Drift protection section is in system prompt with `##` prefix?"
- **#500 VERDICT format:** Exact string matching is strict. Mitigation: Test verify_skill output format explicitly.

### Low-Risk Areas
- **#499, #502, #503:** Mostly additive, low surface area for regressions.

---

## Testing Strategy

### #499 (MaxTurns)
```
✓ Forked agent with max_turns=2 stops after 2 assistant turns
✓ Profile inheritance: child inherits max_turns from parent
✓ MaxSteps and MaxTurns work together (min applied)
✓ Forensic event emitted on turn exhaustion
✓ Interactive agent with max_turns=0 runs unlimited
✓ Default max_turns=5 for forked agents
✓ Background extraction max_turns=2 by default
```

### #500 (Anti-patterns/VERDICT)
```
✓ verify_skill returns VERDICT: PASS|FAIL|PARTIAL
✓ PASS without Command run: block is flagged incomplete
✓ Tool description includes "verification avoidance" and "first-80% seduction" text
✓ Model sees anti-pattern guidance before tool use
```

### #501 (Typed memory)
```
✓ Memory files have typed frontmatter (name, type, description)
✓ Index file capped at 200 lines / 25KB
✓ On-demand loading: relevance selector picks top-5 from index
✓ Drift protection section present as ## in system prompt
✓ Feedback type saves both corrections and validations
✓ Index rollover removes oldest entries when limit exceeded
✓ Concurrent writes don't corrupt index
```

### #502 (system-reminder)
```
✓ system-reminder tags injected into conversation
✓ Model receives guidance about tag format
✓ Cache key is unaffected by system-reminder content
✓ Tool catalog updates via system-reminder don't invalidate cache
```

### #503 (Coordinator anti-patterns)
```
✓ Orchestrator system prompt includes anti-pattern guidance
✓ Worked examples (WRONG vs RIGHT) are visible in prompt
✓ Coordinator generates worker prompts with file paths + line numbers
```

