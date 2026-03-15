# Issue #22: TTSR — Time Traveling Streamed Rules for Zero-Cost Context Injection

**Date**: 2026-03-14
**Status**: Research
**Blockers**: None — pure research, implementable independently

---

## 1. What Is TTSR?

**TTSR** (Time Traveling Streamed Rules) is a context injection technique from
the `oh-my-pi` fork of Pi agent. From the Pi research doc:

> "Pattern-triggered system reminders that inject zero context tokens until a
> matching pattern fires."

The grooming doc clarifies the corrected interpretation:

> "Pattern-triggered context injection that keeps rules at zero token cost
> until matching pattern appears, then injects for that turn only."

**Core idea**: You have a library of rules (e.g., "when you are about to use
`os.Remove`, check the path is inside the workspace"). Instead of including all
rules in every system prompt (wasting tokens), each rule has a trigger pattern.
The rule is only injected into the prompt for the turn in which the pattern
fires.

The name is slightly misleading:
- **"Time traveling"**: the rule is injected retroactively into the current
  turn's prompt rather than having been present from the start (it appears as
  if it was always there)
- **"Streamed"**: rules are evaluated incrementally as the conversation
  progresses (not pre-loaded)
- **"Rules"**: this is specifically about rule/instruction content, not
  arbitrary context

---

## 2. Technical Mechanism

### Pattern-Trigger Model

At each step, before building the `CompletionRequest`, the system:

1. Scans the current context (recent messages, tool calls, etc.) for trigger patterns
2. For each rule whose pattern fires, appends its content to the system prompt
3. Sends the augmented prompt to the LLM
4. For the next step, evaluates again — rules that don't fire are not present

**Pattern matching approaches** (from least to most expensive):

| Granularity | What Is Scanned | Examples |
|-------------|-----------------|---------|
| Per-message | Recent user/assistant messages | "user mentions 'delete file'" |
| Per-tool | Tool name in pending calls | "tool == 'bash'" |
| Per-tool-output | Content of tool results | "tool output contains 'error'" |
| Per-content | All messages in window | Full regex scan of context |

The `oh-my-pi` implementation reportedly uses per-line scanning, which is a
middle ground — more precise than per-message, less expensive than full-window
scan.

### Fire-Once vs Per-Turn Scopes

Rules can have different injection lifetimes:
- **fire-once**: inject the first time the pattern fires, never again in the
  same session
- **per-turn**: inject every turn the pattern fires
- **persistent**: once the pattern fires, inject for all remaining turns

### "Zero Token Cost Until Pattern Fires"

The cost model is straightforward:
- Rules not firing = **zero prompt tokens** for those rules
- Rules firing = tokens for **that turn only**

For a 50-rule system where 48 rules are irrelevant on any given turn, this
is a substantial savings compared to including all 50 rules in every system
prompt.

---

## 3. Cost Implications

### Token Savings Estimate

Current system: rules are in behaviors/talents loaded at `ResolvePrompt` time
and included in **every turn's** system prompt.

Assuming:
- 10 rules, 200 tokens each = 2,000 tokens per turn in system prompt
- 100-turn session
- Only 3 rules fire (fire-once), rest never relevant

**Current approach**: 2,000 × 100 = 200,000 prompt tokens for rules
**TTSR approach**: 3 × 200 (fire-once) + 0 × 97 = 600 prompt tokens for rules
**Savings**: 199,400 tokens, ~99.7% reduction

At gpt-4.1 pricing ($2/1M input tokens), that's a saving of ~$0.40 per session
— meaningful for long agentic sessions.

More conservatively, if rules are short (50 tokens each) and fire 10% of turns:
- Current: 500 × 100 = 50,000 tokens
- TTSR: 10 rules × 50 tokens × 10 turns = 5,000 tokens
- Savings: ~90%

### Interaction With Prefix Caching

OpenAI prefix caching (and Anthropic extended thinking context caching) works
by caching a fixed prefix of the prompt. TTSR actually **interferes with
prefix caching**: if the system prompt changes turn-by-turn (as rules are
injected or removed), the cached prefix is invalidated more frequently.

**Trade-off analysis:**

| Scenario | TTSR Wins | Prefix Cache Wins |
|----------|-----------|-------------------|
| Long rule sets, low fire rate | Yes (rules not sent) | No (no cache benefit on dynamic prompts) |
| Short rule sets, high fire rate | No (minimal savings) | Yes (stable prefix gets cached) |
| Rules that fire every turn | Neither helps much | Neither helps much |

**Conclusion**: TTSR and prefix caching are somewhat in tension. For
long-running sessions with large rule libraries (the primary target), TTSR
wins because the savings from not sending unneeded rules outweigh the loss of
cache hits on the system prompt.

For short sessions or small rule libraries, a stable system prompt with prefix
caching may be more cost-effective.

---

## 4. Applicability to This Harness

### Current Prompt Architecture

From `internal/systemprompt/engine.go`, prompt composition:

```
[SECTION BASE]       ← base prompt (always included)
[SECTION INTENT]     ← intent-specific (always included for this run)
[SECTION MODEL_PROFILE] ← model-specific (always included)
[SECTION BEHAVIOR:id] ← per-behavior content (always included if behavior active)
[SECTION TALENT:id]  ← per-talent content (always included if talent active)
[SECTION SKILL:id]   ← per-skill content (always included if skill active)
[SECTION CUSTOM]     ← custom content (always included if set)
```

This is a static composition: the system prompt is resolved once at `StartRun`
time and reused for every LLM call in `execute()`.

```go
// In execute():
systemPrompt, resolvedPrompt, runStartedAt := r.promptContext(runID)
// ...
// Per-step:
if systemPrompt != "" {
    turnMessages = append(turnMessages, Message{Role: "system", Content: systemPrompt})
}
```

TTSR would add a **dynamic layer**: per-turn pattern matching that adds
additional system messages for specific turns only.

### What Content Benefits Most From TTSR

**High value (large, rarely relevant):**
- Tool-specific rules: "when using bash, always quote paths with spaces" (fires
  only when bash is called)
- Error recovery guidance: "when you see a compilation error, check the import
  list first" (fires only when errors appear in tool output)
- Safety constraints: "do not remove directories without explicit confirmation"
  (fires when `rm -rf` pattern appears)
- Long memory context: observational memory snippets (currently injected every
  turn — TTSR could inject them only when relevant patterns appear)

**Low value (short, always relevant):**
- Core behavioral instructions ("be concise")
- Identity/persona content
- Format instructions

**High value for harness specifically:**
- Tool descriptions — currently the harness sends all visible tool definitions
  on every turn. This is not TTSR, but deferred tools (#4) already address this
  for the TierDeferred case. TTSR could complement this by injecting tool-
  specific usage rules only when the tool is called.
- Observational memory — the memory snippet is injected every turn. TTSR could
  selectively inject only memory entries relevant to the current context
  (pattern: "if current messages mention X, inject memory about X").

### Integration Point: Talents Layer

Talents (`prompts/extensions/talents/`) are the natural home for TTSR-style
rules. Currently talents are resolved statically at run start. A TTSR-aware
talent would have:

```yaml
name: bash-safety
trigger: "bash"       # fires when 'bash' appears in tool calls
scope: per-turn
content: |
  When executing bash commands, always use absolute paths...
```

The engine would evaluate trigger patterns at each step and include only
fired talents.

---

## 5. Implementation Proposal

### New Concept: `DynamicTalent`

```go
type DynamicTalentRule struct {
    ID      string
    Trigger TriggerPattern
    Scope   TriggerScope
    Content string
}

type TriggerPattern struct {
    // ToolName: fires when a specific tool is used this turn
    ToolName string
    // MessageContent: regex on recent message content
    MessageContent string
    // ToolOutput: regex on tool result content
    ToolOutput string
}

type TriggerScope string
const (
    TriggerScopePerTurn  TriggerScope = "per_turn"   // every turn that fires
    TriggerScopeFireOnce TriggerScope = "fire_once"  // first fire only
    TriggerScopePersist  TriggerScope = "persist"    // all turns after first fire
)
```

### Integration in execute()

The key hook is just before building `completionReq.Messages`:

```go
// Existing: inject static system prompt
if systemPrompt != "" {
    turnMessages = append(turnMessages, Message{Role: "system", Content: systemPrompt})
}

// New: evaluate TTSR rules against current context
if r.config.DynamicRules != nil {
    fired := r.config.DynamicRules.Evaluate(runID, step, messages, result.ToolCalls)
    for _, rule := range fired {
        turnMessages = append(turnMessages, Message{
            Role:    "system",
            Content: rule.Content,
            IsMeta:  true, // TTSR rules are meta, not part of conversation history
        })
        r.emit(runID, EventDynamicRuleInjected, map[string]any{
            "rule_id": rule.ID,
            "step":    step,
            "trigger": rule.Trigger,
        })
    }
}
```

### Minimal Viable Implementation

The MVP is pattern matching on tool names — the cheapest and most precise
trigger:

1. At each step, collect the tool names from `result.ToolCalls`
2. For each rule with a `ToolName` trigger matching one of the called tools,
   inject the rule content as a system message
3. Track fire-once rules in per-run state

This requires:
- A `DynamicRuleEvaluator` interface in `RunnerConfig`
- A YAML-driven `FileRuleEvaluator` that reads rules from `prompts/rules/`
- Rule injection in the per-step system prompt assembly
- A `rule.injected` event type for observability

**Estimated effort**: 2-3 days for tool-name trigger only; add 1-2 days for
regex content triggers.

---

## 6. Feasibility Assessment

### Is This Implementable With Current APIs?

Yes. TTSR does not require any special model API features. It is purely a
**client-side prompt manipulation** technique. The model sees the additional
rules as part of the system prompt for that turn — it has no visibility into
whether the rule was "always there" or "just injected."

TTSR does **not** involve:
- KV cache manipulation (that's a server-side model feature)
- Prompt caching (useful but not required)
- Any streaming API changes

### Minimum Viable Version

The absolute minimum is:
1. A `[]DynamicRule` config field on `RunnerConfig`
2. In `execute()`, before each LLM call: loop over rules, check tool-name
   triggers against `result.ToolCalls` (or an empty list for step 1), inject
   fired rules as system messages

This is ~50 lines of code excluding tests. The YAML loader and catalog integration
are optional for the MVP.

### Risks

1. **Pattern matching cost**: regex on large tool outputs could be slow. Bound
   scanning to the last N bytes of output. For tool-name triggers (the MVP),
   this is a non-issue.

2. **LLM instruction following**: if a rule fires per-turn, the LLM sees
   repeated injections. Some models may deprioritize repeated instructions.
   Fire-once scope mitigates this.

3. **Interaction with compaction**: if messages are compacted, tool-output
   patterns may not fire for compacted turns. This is acceptable behavior.

4. **Observability**: without `rule.injected` events, it is hard to debug why
   the agent behaved a certain way. The event emission is important.

---

## 7. Token Savings Estimation for This Codebase

The current harness system prompt includes:

- Base prompt (~500 tokens)
- Intent section (~300 tokens)
- Model profile (~200 tokens)
- Behavior sections: 0-3 behaviors × ~200 tokens = 0-600 tokens
- Talent sections: 0-2 talents × ~300 tokens = 0-600 tokens
- Skill sections: 0-1 skills × ~500 tokens = 0-500 tokens

Total system prompt range: ~1,000 to ~2,700 tokens per turn.

If 2 talents and 1 behavior are only relevant for specific tool calls:
- TTSR savings: ~700 tokens × (turns they don't fire / total turns)
- For a 20-step run where they fire 5 times: 700 × 15 = 10,500 tokens saved
- At gpt-4.1 pricing: ~$0.02 saved per run

For high-volume deployments or long runs with large rule libraries, the
savings compound significantly.

---

## 8. Related Issues

- **#4 (Deferred Tools)**: analogous — tools are hidden until needed. TTSR
  applies the same principle to rules/instructions.
- **#17 (Compaction)**: interaction noted — compacted history may cause rules
  to not fire for older context.
- **Talents layer** (`prompts/extensions/talents/`): the natural integration
  point for TTSR-backed rules.
