# Issue #495 Planning Documentation

Complete analysis and implementation plan for enriching tool descriptions with behavioral specifications.

## Documents Included

### 1. **issue-495-plan.md** (18KB, 546 lines)
Comprehensive planning document covering:
- Current state assessment of all 72 tool descriptions
- Proposed behavioral spec format with examples
- Files requiring changes (6 categories)
- Tool tier mapping (TierCore vs TierDeferred)
- Complete testing strategy
- 5-week implementation roadmap (Phase 1-4)
- Priority matrix for all tools
- Configuration requirements
- Risk analysis and mitigations
- Templates and appendices

**Best for:** Strategic planning, team review, implementation timeline

### 2. **ISSUE-495-SUMMARY.txt** (6.4KB, 184 lines)
Executive summary structured for quick scanning:
- 10-section overview
- Current tool description statistics
- Behavioral spec format overview
- Files requiring changes (organized by phase)
- Tool tier system explanation
- Testing strategy (unit, integration, documentation)
- Implementation roadmap (4 phases, week-by-week)
- Configuration needs
- Success criteria
- Key risks and mitigations
- Priority matrix

**Best for:** Quick reference, management overview, decision-making

### 3. **QUICK-REFERENCE-495.md** (7.2KB, 276 lines)
Implementation reference guide:
- Current tool description system overview
- Proposed directory structure
- Code changes needed (5 specific files with examples)
  - types.go: Add BehavioralSpec field
  - embed.go: New embedded FS + loader function
  - catalog.go: Inject specs into definitions
  - config.go: New configuration section
  - behavioral_specs_test.go: Test skeleton
- Phase 1 tool priority list
- Behavioral spec template (copy-paste ready)
- Configuration points explained
- Backward compatibility notes
- Success metrics
- Code location reference table
- Next actions checklist

**Best for:** Developers, implementation, code review

### 4. **TOOL-DESCRIPTIONS-INVENTORY.txt** (7.2KB, 246 lines)
Complete inventory and analysis:
- All 72 tools listed by phase
- Tier classification (TierCore 54, TierDeferred 18)
- Current content quality metrics
- Line count distribution
- Coverage gaps (35% with guidance, 11% with error modes, 8% with performance notes)
- Richest descriptions (apply_patch, bash, read, edit)
- Sparsest descriptions (cron tools, web tools)
- Key observations about gaps
- Enrichment impact assessment (high/medium/low)
- Estimated time per phase (40 hours Phase 1, 30 hours Phase 2, 25 hours Phase 3)

**Best for:** Understanding current state, prioritization, resource planning

---

## Quick Start

1. **Need to understand the issue?** Start with ISSUE-495-SUMMARY.txt
2. **Starting implementation?** Read QUICK-REFERENCE-495.md
3. **Reviewing the plan?** Check issue-495-plan.md sections 1-3
4. **Planning resources?** Use TOOL-DESCRIPTIONS-INVENTORY.txt for time estimates
5. **Technical deep dive?** Section 3-6 of issue-495-plan.md

---

## Key Findings

### Current State
- **72 tool descriptions** in `/internal/harness/tools/descriptions/`
- **969 total lines** of content (avg 13 lines/tool)
- **Only 35%** have "when to use" guidance
- **Only 11%** document error modes
- **Only 8%** include performance notes
- **Only 6%** explain interaction patterns

### Coverage Gaps
High-priority tools needing enrichment:
1. **bash** — execution model, timeouts, job lifecycle
2. **read** — truncation, hashing, offset/limit semantics
3. **write** — atomicity, append vs replace
4. **edit** — line-hash semantics, multiline matching
5. **apply_patch** — atomicity, multi-file guarantees

### Proposed Solution
New `/internal/harness/tools/behavioral_specs/` directory containing markdown files with:
- Preconditions
- Side effects
- Error modes and recovery
- Atomicity & consistency guarantees
- Performance characteristics
- Interaction patterns
- Agent decision guidance

### Implementation Plan
**4-phase rollout over 5 weeks:**
- **Phase 1 (Week 1):** Foundation + 5 highest-priority tools
- **Phase 2 (Weeks 2-3):** 10 more core tools
- **Phase 3 (Week 4):** 20 extended tools
- **Phase 4 (Week 5):** Validation, polish, feedback

**Estimated effort:** 40 hours (Phase 1) + 30 hours (Phase 2) + 25 hours (Phase 3)

---

## Code Changes Required

### Files to Modify
1. `/internal/harness/tools/types.go` — Add `BehavioralSpec` field to `Definition`
2. `/internal/harness/tools/descriptions/embed.go` — New embedded FS + loader
3. `/internal/harness/tools/catalog.go` — Inject specs into definitions
4. `/internal/config/config.go` — New behavioral specs config section

### Files to Create
1. `/internal/harness/tools/behavioral_specs/` — New directory for spec files
2. `/internal/harness/tools/behavioral_specs/INDEX.md` — Spec index/organization
3. `/internal/harness/tools/behavioral_specs_test.go` — Tests for loading/validation

### Phase 1 Spec Files (15 new files)
bash.md, read.md, write.md, edit.md, apply_patch.md, glob.md, grep.md, agent.md, skill.md, web_search.md, fetch.md, cron_create.md, git_status.md, git_diff.md, AskUserQuestion.md

---

## Configuration Needed

New optional TOML section:
```toml
[behavioral_specs]
enabled = true                          # default: false
precondition_checking = false           # validate inputs before tool use
side_effect_tracing = false             # log state changes
error_mode_documentation = true         # inject error modes into descriptions
spec_injection = "full"                 # "none" | "minimal" | "full"
```

Backward compatible — all defaults disabled.

---

## Success Criteria

1. ✓ 100% of Phase 1 tools (15) have behavioral specs
2. ✓ No contradictions between descriptions and specs
3. ✓ ≥90% test coverage for new code
4. ✓ Tool catalog build time increases by <5%
5. ✓ Agents demonstrate improved decision-making

---

## Related Issues & Prior Work

- **Issue #41:** Tool description migration to embedded .md files (completed)
- **Issue #94:** Bash tool test output guidance (completed)
- **Deferred tools design:** Research on tool visibility and discovery

---

## Next Steps

1. **Team review** of this plan (documents prepared for feedback)
2. **Approve scope** (Phase 1 vs phased approach)
3. **Create directory:** `/internal/harness/tools/behavioral_specs/`
4. **Start Phase 1:** Highest-priority 5 tools (bash, read, write, edit, apply_patch)
5. **Implement infrastructure:** types.go, embed.go, catalog.go changes
6. **Add tests:** behavioral_specs_test.go with loading/validation
7. **Iterate:** Gather feedback on spec format, adjust templates
8. **Expand:** Phases 2 and 3 based on initial success

---

## Document Access

All documents stored in: `/Users/dennisonbertram/Develop/go-agent-harness/docs/plans/`

```
issue-495-plan.md                    (main planning doc)
ISSUE-495-SUMMARY.txt                (executive summary)
QUICK-REFERENCE-495.md               (implementation guide)
TOOL-DESCRIPTIONS-INVENTORY.txt      (detailed inventory)
README-ISSUE-495.md                  (this file)
```

---

## Questions & Clarifications

**Q: Why a separate directory for behavioral specs?**
A: Separation of concerns. API docs (descriptions/*.md) stay focused on parameters/returns. Behavioral specs are rich, multi-section documents better suited to a dedicated directory.

**Q: Is this backward compatible?**
A: Yes. BehavioralSpec field in Definition is optional (omitempty). Config is opt-in (disabled by default). Existing tools work unchanged.

**Q: How much effort is this?**
A: Phase 1 requires ~40 hours. Phase 2 adds 30 hours. Phase 3 adds 25 hours. Can be spread over weeks or compressed.

**Q: Can we do this incrementally?**
A: Yes, by design. Specs can be added tool-by-tool. CI checks validate completeness per phase.

**Q: What's the user benefit?**
A: Agents make better tool-choosing decisions. Users understand tool semantics better. Errors are more recoverable with clear error mode docs.

---

**Document generated:** March 31, 2026  
**Status:** Ready for team review
