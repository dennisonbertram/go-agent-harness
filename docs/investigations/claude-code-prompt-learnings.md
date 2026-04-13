# Claude Code Internals — Prompt Engineering Learnings

**Source:** Leaked Claude Code CLI source code (`tengu` codebase), analyzed 2026-03-31
**Purpose:** Actionable patterns to apply to go-agent-harness

---

## 1. Prompt Cache Architecture

**What they do:** The system prompt is split at a `SYSTEM_PROMPT_DYNAMIC_BOUNDARY` marker:
- Before the boundary → globally cacheable across all users/sessions (`scope: 'global'`)
- After the boundary → session-specific (memory, env, MCP instructions)

MCP server instructions (which change when tools connect/disconnect) were responsible for **~10.2% of fleet cache_creation tokens** when placed inline. They moved volatile content to attachment messages instead.

**Applicability:** Our system prompt builder should explicitly separate static vs. dynamic sections. Any tool/plugin listing that can change mid-session should NOT be in the main system prompt body. Tool catalogs and agent listings should be injected as separate message attachments.

---

## 2. Analysis Scratchpad Pattern

**What they do:** For compaction/summarization prompts:
```
Think inside <analysis> tags first, then output in <summary> tags.
```
The `<analysis>` block is **stripped before it enters conversation context** via `formatCompactSummary()`. This gives chain-of-thought quality without polluting the context window.

**Applicability:** Our compaction pipeline should let the model reason inside scratchpad tags, then strip the reasoning before injecting the summary back into context.

---

## 3. Adversarial No-Tools Preamble

**What they do:** When running compaction (which inherits the full toolset due to cache sharing), the first thing the model sees is:

> "CRITICAL: Respond with TEXT ONLY. Do NOT call any tools... Tool calls will be REJECTED and will waste your only turn — you will fail the task."

Sonnet 4.6+ had a **2.79% tool-call-during-compact rate** vs 0.01% on 4.5. The aggressive framing was deliberately adversarial.

**Applicability:** When we need tool-free inference from a model that has tools loaded, explicit aggressive prohibition works better than gentle instructions. Apply to our compaction and summary generation steps.

---

## 4. Memory System Design

**What they do:**
- **4 types only**: `user`, `feedback`, `project`, `reference` — closed taxonomy
- **Feedback type saves both corrections AND validations** — "if you only save corrections, you drift away from validated approaches"
- **Memory drift protection**: Before recommending from memory, the model must verify claims against current file state. This only worked with its **own section header** (0/3 pass rate as a bullet, 3/3 as a `## Before recommending from memory` section)
- **MEMORY.md** is a 200-line / 25KB-capped index; individual memories are separate `.md` files loaded on-demand via a Sonnet relevance selector (picks top 5 per query)
- **Long-lived sessions (KAIROS)**: Uses append-only daily logs → nightly `/dream` skill distills into topic files

**Applicability:** Our memory/learning system should: (a) cap the index aggressively, (b) use on-demand retrieval for details, (c) save validations not just corrections, (d) use section headers not bullets for behavioral instructions.

---

## 5. Speculative Pre-Execution

**What they do:** After each response, a forked agent predicts "what the user would naturally type next" and pre-executes it in a copy-on-write overlay directory. If the user accepts, the speculated work is injected. If they type something else, it's discarded.

- Max 20 turns / 100 messages per speculation
- Stops at non-read-only operations (file edits, risky bash)

**Applicability:** Speculative pre-execution is a powerful UX pattern. We could implement this for common follow-up patterns (e.g., after a plan is approved, speculatively start the first task).

---

## 6. Tool Descriptions as Behavioral Specs

**What they do:** Tool prompts aren't just descriptions — they're behavioral contracts:
- **BashTool**: 7 git safety rules, sleep avoidance, parallel execution guidance
- **FileEditTool**: "Use the smallest old_string — usually 2-4 adjacent lines"
- **AgentTool**: Full doctrine on when to fork vs. spawn, "Don't peek" / "Don't race" rules
- **TodoWriteTool**: Requires two forms per task (`content` = imperative, `activeForm` = present continuous), exactly one `in_progress` at a time

**Applicability:** Our tool descriptions in `internal/harness/tools/descriptions/*.md` should encode behavioral rules, not just API docs. Every tool should have a "when NOT to use" section.

---

## 7. Verification Agent Anti-Patterns

**What they do:** Their verification agent explicitly names anti-patterns:
- **"Verification avoidance"**: reading code instead of running it
- **"Being seduced by the first 80%"**: declaring success when the easy parts work
- A PASS step without a `Command run:` block is a skip, not a pass
- Output must follow exact format: `VERDICT: PASS` / `VERDICT: FAIL` / `VERDICT: PARTIAL`

**Applicability:** Our test/verification tools should mandate executable evidence, not just code review.

---

## 8. Coordinator Synthesis Doctrine

**What they do:** The coordinator prompt contains specific anti-patterns:
> "Never write 'based on your findings'" — must include specific file paths, line numbers

Worked examples show WRONG vs RIGHT worker prompts. The coordinator must prove it understood research before delegating implementation.

**Applicability:** When our orchestrator delegates to workers, it should include concrete file paths and line references from the research phase — not vague summaries.

---

## 9. Output Efficiency Enforcement

**What they do:** Numeric anchors in system prompt: **≤25 words between tool calls, ≤100 words in final responses**. Measurably reduced token waste.

**Applicability:** Add similar numeric word-count limits to our system prompt builder for agent runs.

---

## 10. Two-Turn Budget for Background Agents

**What they do:** Memory extraction agent has an explicit 2-turn budget: turn 1 = all reads in parallel, turn 2 = all writes in parallel. Caps token cost for background operations.

**Applicability:** Background agents should have hard turn budgets, not just token budgets. Implement `MaxTurns` in ForkConfig/profile system.

---

## 11. Cron Thundering-Herd Prevention

**What they do:** Cron scheduler explicitly avoids `:00` and `:30` minute marks to prevent API fleet thundering herd.

**Applicability:** If we add scheduled tasks, jitter by default.

---

## 12. Undercover Mode

**What they do:** For public repo contributions, the model strips all agent attribution — no model IDs, no "Co-Authored-By" lines, writes commit messages "as a human developer would."

**Applicability:** Implement for our harness when agents contribute to external repos.

---

## 13. `<system-reminder>` Tag Injection

**What they do:** The system prompt tells the model that `<system-reminder>` tags in messages are system-injected and "bear no direct relation to the specific tool results or user messages in which they appear." This allows dynamic per-turn injections (tool listing updates, skill discovery) without busting the static prompt cache.

**Applicability:** We should adopt a similar tag-based injection mechanism for dynamic mid-conversation system content.

---

## 14. Feature Flag Architecture

**What they do:** Build-time `feature('FLAG_NAME')` for dead-code elimination + runtime GrowthBook flags for gradual rollout. Internal codename `tengu` prefixes all flags.

**Key flags:** `PROACTIVE`, `KAIROS`, `COORDINATOR_MODE`, `BUDDY`, `VERIFICATION_AGENT`, `FORK_SUBAGENT`, `CACHED_MICROCOMPACT`, `TEAMMEM`

**Applicability:** Our harness already has some config-driven behavior. Consider a more formal feature flag system for experimental capabilities.

---

## 15. Model Versions (Reference)

- Current frontier: **Claude Opus 4.6**, **Claude Sonnet 4.6**
- Knowledge cutoffs: Sonnet 4.6 = August 2025, Opus 4.6 = May 2025
- Haiku still on 4.5 (Feb 2025 cutoff)

---

## Implementation Priority

| Priority | Learning | Effort | Impact |
|----------|----------|--------|--------|
| P0 | Prompt cache split (static/dynamic boundary) | Medium | High — direct cost savings |
| P0 | Tool descriptions as behavioral specs | Medium | High — better tool use quality |
| P1 | Analysis scratchpad for compaction | Low | Medium — better summaries |
| P1 | Adversarial no-tools preamble | Low | Medium — prevents tool calls in compaction |
| P1 | Output efficiency numeric anchors | Low | Medium — reduces token waste |
| P1 | Two-turn budget for background agents | Low | Medium — caps background costs |
| P1 | Verification anti-pattern naming | Low | Medium — better verification |
| P2 | Memory system redesign (typed, capped, on-demand) | High | High — better long-term memory |
| P2 | System-reminder tag injection | Medium | Medium — cache-friendly dynamic content |
| P2 | Coordinator synthesis doctrine | Low | Medium — better delegation quality |
| P3 | Speculative pre-execution | High | High — UX improvement |
| P3 | Undercover mode | Medium | Low — niche use case |
| P3 | Cron jitter | Low | Low — only matters at scale |
| P3 | Feature flag system | Medium | Medium — better experimentation |
