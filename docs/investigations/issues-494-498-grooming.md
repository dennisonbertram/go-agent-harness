# GitHub Issues 494–498 Grooming Assessment

**Analysis Date:** 2026-03-31  
**Analyst:** Claude Code (Haiku 4.5)  
**Codebase:** go-agent-harness  

---

## Issue 494: P0 — Implement Static/Dynamic System Prompt Cache Boundary

**Title:** `P0: Implement static/dynamic system prompt cache boundary`

### Already Resolved?
**NO** — Evidence:
- `internal/systemprompt/engine.go` currently returns a single `StaticPrompt` string with all sections concatenated (line 101)
- No `SYSTEM_PROMPT_DYNAMIC_BOUNDARY` marker exists in the codebase
- Tool/agent listings are not separated into message attachments; they'd need to be added to extension system
- `ResolvedPrompt` struct (types.go) does not distinguish static vs. dynamic portions
- `RuntimeContext()` is separate but not labeled as "dynamic" in the API

### Well-Specified?
**YES** — Problem statement is crystal clear:
- **Boundary marker:** Everything before "globally cacheable," everything after "session-specific"
- **Static sections:** identity, task philosophy, tool behavior rules, tone/style
- **Dynamic sections:** memory, env, tool catalog, plugin instructions
- **Cache implication:** Only static portion hash should be cache key
- **Implementation hint:** Move volatile tool/agent listings to message attachments instead of system prompt body

### Acceptance Criteria
**Explicit but needs test definition:**
- ✓ `SYSTEM_PROMPT_DYNAMIC_BOUNDARY` marker added to `internal/systemprompt/`
- ✓ `ResolvedPrompt` struct extended with `.StaticPrompt` and `.DynamicPrompt` fields (or `.StaticHash`)
- ✓ Tool/MCP listings moved to attachment messages (not in prompt body)
- ✓ Cache key generation updated to hash only static portion
- ✗ **Missing:** Measurable test showing ~10.2% reduction in cache_creation tokens (fleet-level validation)

### Scope Assessment
**MEDIUM** — Not fully atomic. Depends on:
1. System prompt refactoring (moderate effort)
2. Message attachment plumbing (architectural change)
3. Cache key generation update (new logic)
4. Tool catalog serialization changes (medium effort)

### Dependencies
- **Hard:** Requires understanding of Anthropic prompt caching protocol (cache control headers, cache_creation_tokens reporting)
- **Soft:** Should integrate with #497 (no-tools preamble) since both affect prompt structure
- **Soft:** Works well with #498 (word-count anchors) since both go in dynamic section

### Blockers
- Message attachment format needs design (JSON vs. XML?)
- Tool listing serialization format for attachments needs specification
- Decision: should static prompt hash be exposed in API, or remain internal?

### Effort Estimate
**LARGE** (8–12 story points)
- 3–4h: Design attachment format and refactor `ResolvedPrompt` struct
- 2–3h: Extract tool/MCP listings from prompt sections
- 2–3h: Update cache key generation and validate hash-only behavior
- 1–2h: Add tests for cache hit scenarios (synthetic)
- 1–2h: Integration testing with message builder

### Key Files That Will Need Change
```
internal/systemprompt/
  ├── types.go                  [ResolvedPrompt struct]
  ├── engine.go                 [Resolve() to split static/dynamic]
  ├── catalog.go                [May need cache hash annotation]
  └── *_test.go                 [Add boundary marker tests]

internal/harness/
  ├── runner_step_engine.go     [Message building, attachment injection]
  ├── context_builder.go        [Cache key generation]
  └── tools/                    [Tool listing extraction]

internal/provider/anthropic/
  └── client.go                 [Cache control header handling]
```

### TOML Config Fields Needed
```toml
[systemprompt]
# If making cache behavior configurable:
cache_enabled = true
cache_static_only = true              # Future: allow disabling
cache_hash_algorithm = "sha256"

# Tool listing delivery method:
tool_listing_transport = "attachment" # "inline" | "attachment"
```

### Suggested Labels
- `enhancement`
- `performance`
- `system-prompt`
- `prompt-caching`

---

## Issue 495: P0 — Enrich Tool Descriptions with Behavioral Specifications

**Title:** `P0: Enrich tool descriptions with behavioral specifications`

### Already Resolved?
**PARTIAL** — Mixed evidence:
- ✓ `agent.md` has "WHEN TO USE" and "WHEN NOT TO USE" sections (exemplar)
- ✓ `bash.md` has 7 git safety rules and anti-patterns (exemplar)
- ✓ `bash.md` has "INTERPRETING Go TEST OUTPUT" behavioral guide
- ✗ Most other tool descriptions are bare API docs (write.md, read.md, glob.md, grep.md)
- ✗ `find_tool.md`, `apply_patch.md` lack behavioral sections
- ✗ No "Common mistakes" or named anti-pattern sections in most tools

### Well-Specified?
**YES** — Clear priority list and format:
- **Format required:** "When to use," "When NOT to use," "Behavioral rules," "Common mistakes"
- **Priority:** bash, file edit/write, run_agent, find_tool
- **Examples required:** WRONG vs RIGHT concrete examples (like coordinator synthesis doctrine)
- **Inspiration:** Full doctrine from Claude Code learnings doc

### Acceptance Criteria
**Explicit:**
- ✓ All high-priority tools (5–6 core tools) have "When NOT to use" section
- ✓ All high-priority tools have 1–2 "Common mistakes" named anti-patterns
- ✓ All high-priority tools have concrete WRONG/RIGHT examples
- ✓ Tool descriptions embedded in system prompt show behavioral rules
- ✗ **Missing:** Measurable quality gate (e.g., "tool misuse rate < 2%")

### Scope Assessment
**SMALL-MEDIUM** — Largely isolated content additions; no code changes required.
- High-priority tools: bash, edit, write, agent, find_tool (~6 tools)
- Optional medium-priority: grep, glob, git, web_fetch (~4 tools)
- Each tool gets 3–5 new paragraphs added

### Dependencies
- **Soft:** Works with #494 (dynamic boundary) since tool descriptions live in dynamic section if not embedded in base
- **Soft:** Could reference #496 (analysis pattern) if tool descriptions mention scratchpad usage

### Blockers
- **Decision:** Are behavioral rules part of the embedded tool definition (JSON schema), or separate Markdown? Current structure uses embedded `.md` files loaded via `descriptions.Load()` — this is fine.
- **Decision:** Should all tools get the full treatment, or just priority ones? Issue says "prioritize" but doesn't say "only."

### Effort Estimate
**SMALL** (3–5 story points)
- 1–2h: Research and write behavioral specs for 6 priority tools
- 1h: Review and refine examples (WRONG/RIGHT pairs)
- 1h: Integrate into tool description files (markdown editing)
- 30m: Review consistency across all descriptions

### Key Files That Will Need Change
```
internal/harness/tools/descriptions/
  ├── bash.md                     [Already exemplary; may expand]
  ├── edit.md                     [Add behavioral rules]
  ├── write.md                    [Add behavioral rules]
  ├── agent.md                    [Already exemplary; may refine]
  ├── find_tool.md                [Add behavioral rules]
  ├── grep.md                     [Medium priority]
  ├── glob.md                     [Medium priority]
  ├── git_*.md                    [Medium priority]
  └── web_fetch.md                [Medium priority; security implications]
```

### TOML Config Fields Needed
**NONE** — Pure content addition, no config needed.

### Suggested Labels
- `enhancement`
- `documentation`
- `tool-specs`
- `behavioral-rules`

---

## Issue 496: P1 — Add `<analysis>` Scratchpad Pattern to Context Compaction

**Title:** `P1: Add <analysis> scratchpad pattern to context compaction`

### Already Resolved?
**NO** — Evidence:
- `internal/harness/tools/compact_history.go` calls a `MessageSummarizer` (line 13)
- No special handling of `<analysis>` tags in summarization prompts
- No XML tag parsing to strip analysis blocks before injection
- Compaction code does not mention scratchpad patterns

### Well-Specified?
**YES** — Implementation path is clear:
- **Prompt change:** Add instruction to reason in `<analysis>` tags, output summary in `<summary>` tags
- **Parsing:** Strip `<analysis>...</analysis>` blocks before re-injecting summary
- **Benefit:** Chain-of-thought quality without context pollution
- **Location:** compaction prompt in `internal/harness/` (auto-compaction trigger)

### Acceptance Criteria
**Explicit:**
- ✓ Compaction prompt includes `<analysis>` instruction
- ✓ XML parser extracts both `<analysis>` and `<summary>` blocks
- ✓ Only `<summary>` content injected back into conversation
- ✓ Compacted messages still contain summary (no data loss)
- ✗ **Missing:** Before/after quality comparison (token efficiency, summary accuracy)

### Scope Assessment
**SMALL** — Localized to compaction pipeline:
1. Modify compaction prompt template
2. Add XML parser for summary extraction
3. Update message replacement logic to use stripped summary

### Dependencies
- **Soft:** Benefits from #497 (no-tools preamble) since both affect compaction behavior
- **Soft:** Complements #498 (word-count anchors) since analysis pattern encourages thorough reasoning

### Blockers
- **Design decision:** Should `<summary>` tags be mandatory, or optional with fallback to full output?
- **Design decision:** How to handle nested/malformed XML? Graceful fallback or strict validation?

### Effort Estimate
**SMALL** (2–3 story points)
- 1h: Write compaction prompt template with `<analysis>` + `<summary>` instruction
- 1h: Implement XML parser (can reuse `encoding/xml` or regex)
- 30m: Integrate parser into message replacement pipeline
- 30m: Add tests for both well-formed and malformed cases

### Key Files That Will Need Change
```
internal/harness/
  ├── tools/compact_history.go         [Prompt injection]
  ├── tools/descriptions/
  │   └── compact_history.md           [Update documentation]
  ├── context_builder.go               [Summary extraction logic]
  └── *_test.go                        [Parser tests]
```

### TOML Config Fields Needed
```toml
[auto_compact]
# Optional: control scratchpad behavior
scratchpad_enabled = true              # Enable <analysis> block stripping
scratchpad_analysis_kept = false       # Discard analysis or keep it
```

### Suggested Labels
- `enhancement`
- `compaction`
- `chain-of-thought`
- `context-management`

---

## Issue 497: P1 — Add Adversarial No-Tools Preamble for Compaction/Summarization

**Title:** `P1: Add adversarial no-tools preamble for compaction and summarization`

### Already Resolved?
**NO** — Evidence:
- No `NO_TOOLS_PREAMBLE` constant exists in codebase
- Compaction/summarization prompts do not contain aggressive prohibition
- Tool inheritance in compaction context is not addressed

### Well-Specified?
**YES** — Specific framing required:
- **Tone:** Adversarial, not gentle ("you will fail the task" not "please don't use tools")
- **Metric:** Reduced tool-call-during-compact rate from 2.79% (Sonnet 4.6) to near-zero
- **Application scope:** auto-compaction, memory extraction, any non-tool inference path
- **Exact framing:** Suggest "Tool calls will be REJECTED and will waste your only turn"

### Acceptance Criteria
**Explicit:**
- ✓ `NO_TOOLS_PREAMBLE` constant added to system prompt builder
- ✓ Preamble is first content in any compaction/summary request
- ✓ Framing is adversarial (high-stakes language)
- ✓ Applied to auto-compaction, memory extraction, non-tool inference
- ✗ **Missing:** Telemetry to measure tool-call-during-compact rate pre/post

### Scope Assessment
**SMALL** — Pure prompt engineering:
1. Define constant with aggressive text
2. Inject into compaction/memory/summary prompts
3. Test with various models to verify effectiveness

### Dependencies
- **Soft:** Works with #496 (analysis pattern) since both affect compaction prompts
- **Hard:** Requires #494 (cache boundary) to be meaningful if tools are in cached system prompt

### Blockers
- **Question:** Should preamble be conditional based on model? (Issue implies Sonnet 4.6+ effect; 4.5 had 0.01%)

### Effort Estimate
**SMALL** (2–3 story points)
- 30m: Write `NO_TOOLS_PREAMBLE` constant and variations
- 1h: Identify all compaction/summarization prompt injection points
- 1h: Integrate preamble into each code path
- 30m: Test with sample payloads to verify message structure

### Key Files That Will Need Change
```
internal/systemprompt/
  ├── engine.go                        [Add NO_TOOLS_PREAMBLE constant]
  └── *_test.go                        [Preamble injection tests]

internal/harness/
  ├── tools/compact_history.go         [Inject into compaction prompt]
  ├── tools/observational_memory.go    [Inject into memory extraction]
  ├── runner_step_engine.go            [Auto-compaction path]
  └── *_test.go                        [Integration tests]
```

### TOML Config Fields Needed
```toml
[systemprompt]
no_tools_preamble_enabled = true       # Can disable for testing
no_tools_aggressive_mode = true        # Severity level control
```

### Suggested Labels
- `enhancement`
- `compaction`
- `safety`
- `system-prompt`

---

## Issue 498: P1 — Add Numeric Word-Count Anchors for Output Efficiency

**Title:** `P1: Add numeric word-count anchors for output efficiency`

### Already Resolved?
**NO** — Evidence:
- `RuntimeContext()` function (runtime_context.go) does not include word-count budgets
- System prompt does not mention numeric anchors for output limits
- Profile system (TOML) has no `output_budget` or `reasoning_effort` fields
- No references to "≤25 words" or "≤100 words" constraints in code

### Well-Specified?
**YES** — Clear numeric targets and applicability:
- **Default limits:** ≤25 words between tool calls, ≤100 words final responses
- **Configurable by:** Profile system (`reasoning_effort` or new `output_budget` field)
- **Measurable benefit:** Reduced token waste vs. vague "be concise" instructions
- **Placement:** Dynamic section of system prompt (per-profile customizable)

### Acceptance Criteria
**Explicit:**
- ✓ Word-count anchors added to system prompt (configurable)
- ✓ Default: 25 words between tool calls
- ✓ Default: 100 words final response
- ✓ Overridable via profile (TOML field)
- ✓ Applied to both regular and compaction inference paths
- ✗ **Missing:** Benchmarks showing token reduction (e.g., "10% fewer completion tokens")

### Scope Assessment
**SMALL-MEDIUM** — Config-driven, minimal code changes:
1. Add fields to profile/config TOML schema
2. Inject word-count anchors into system prompt builder
3. Make injections respect profile settings
4. Update built-in profiles with sensible defaults

### Dependencies
- **Soft:** Works with #497 (no-tools preamble) in compaction prompt
- **Soft:** Complements #496 (analysis pattern) — analysis can be verbose, final summary constrained

### Blockers
- **Question:** What counts as a "word"? (Whitespace-separated? Or tokenizer-based?)
- **Question:** Should limits be hard constraints (prompt refusal) or soft guidance?
- **Question:** Different models may have different efficiency; should profiles vary by model?

### Effort Estimate
**SMALL** (3–4 story points)
- 1h: Add `output_budget` fields to config schema (Config struct, TOML parsing)
- 1h: Modify `RuntimeContext()` and system prompt builder to inject word counts
- 1h: Update built-in profiles with sensible defaults
- 30m: Add tests for config override behavior

### Key Files That Will Need Change
```
internal/config/
  ├── config.go                        [Add OutputBudget struct]
  └── *_test.go                        [Config parsing tests]

internal/systemprompt/
  ├── runtime_context.go               [Add word-count injection]
  ├── engine.go                        [Profile-aware injection]
  └── *_test.go                        [Prompt composition tests]

internal/profiles/builtins/
  ├── full.toml                        [Add output_budget section]
  ├── file-writer.toml                [Vary for high-output profile]
  ├── reviewer.toml                   [Vary for analysis-heavy profile]
  └── *.toml                           [All other profiles]
```

### TOML Config Fields Needed
```toml
[runner]
model = "gpt-4.1-mini"
max_steps = 30
max_cost_usd = 2.0

[output_budget]
# Word count limits (configurable per profile)
words_between_tool_calls = 25          # Conciseness between tool invocations
words_final_response = 100             # Conciseness in final user-facing response
words_analysis = null                  # null = no limit for analysis/reasoning
words_summary = 50                     # Summary-specific limit
```

### Suggested Labels
- `enhancement`
- `performance`
- `efficiency`
- `system-prompt`

---

## Summary Table

| Issue | Title | Resolved? | Well-Specified? | Effort | Blockers | Key Dependencies |
|-------|-------|-----------|-----------------|--------|----------|------------------|
| **494** | Static/Dynamic Cache Boundary | NO | YES | LARGE | Cache protocol design | 497, 498 (soft) |
| **495** | Tool Behavioral Specs | PARTIAL | YES | SMALL | None | 494 (soft) |
| **496** | Analysis Scratchpad | NO | YES | SMALL | XML parsing fallback | 497 (soft) |
| **497** | No-Tools Preamble | NO | YES | SMALL | Model-specific testing | 494 (hard), 496 (soft) |
| **498** | Word-Count Anchors | NO | YES | SMALL-MEDIUM | Definition of "word" | 497 (soft), 496 (soft) |

---

## Implementation Roadmap Recommendation

### Phase 1 (Week 1): Low-Risk Foundation
1. **#495** (Behavioral specs) — Pure content, no code risk
2. **#498** (Word-count anchors) — Config-driven, small surface area
3. **#496** (Scratchpad) — Isolated to compaction, easy rollback

**Rationale:** Build confidence with low-risk changes; validate approach with small PRs.

### Phase 2 (Week 2): Compaction Hardening
1. **#497** (No-tools preamble) — Builds on #496 insights
2. Integrate #496 + #497 into single compaction prompt upgrade

### Phase 3 (Week 3): Cache Architecture
1. **#494** (Static/dynamic boundary) — Largest effort, most risk
2. Depends on clarity from Phase 1 work (where do behavioral specs live?)
3. Integration testing with fleet telemetry

---

## Open Design Questions

1. **#494 — Tool listing transport:** Should MCP server instructions go in message attachments as JSON, or stay in prompt as Markdown? What format?

2. **#495 — Scope:** Should ONLY priority tools (6) get behavioral specs, or all 30+ tools? Issue says "prioritize" but doesn't exclude others.

3. **#496 — Fallback:** How strict should XML parsing be? Reject malformed `<summary>` or gracefully use full output?

4. **#497 — Model-aware:** Should preamble vary by model (aggressive for 4.6+, gentle for 4.5)? Or one-size-fits-all?

5. **#498 — Token-aware:** Should word counts be hints, or should the system validate and reject over-length responses?

---

## Cross-Issue Integration Notes

- **#494, #497, #498 all affect system prompt composition.** Consider unified system prompt builder refactor rather than three separate PRs. Reduces conflicts.
  
- **#495 tool descriptions become more valuable if delivered via #494's attachment system.** Tool descriptions could move to message attachments in future, reducing static prompt size.

- **#496 + #497 should be merged into single compaction prompt template PR.** They address the same code path and should be tested together.

- **#498 word counts might need tuning per model.** Consider linking profile system to model-specific defaults.

