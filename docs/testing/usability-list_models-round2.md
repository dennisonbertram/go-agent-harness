# list_models Tool Usability -- Round 2

**Date**: 2026-03-09
**System prompt changes tested**: `find_tool` description now includes "Listing available LLM models -> search models" hint; system prompt tells LLM to use `find_tool` before falling back to bash.

## Summary

| Test | Prompt | find_tool used? | list_models used? | bash fallback? | Score | Turns |
|------|--------|-----------------|-------------------|----------------|-------|-------|
| 1 | What models are available? | Yes | No | No | **F** | 4 |
| 2 | List all the LLM models I can use | Yes | No | No | **F** | 2 |
| 3 | Show me the GPT models available in this system | Yes | No | No | **F** | 3 |
| 4 | Which model am I currently using? | No | No | No | **F** | 1 |
| 5 | What's the cheapest model available? | Yes | No | No | **F** | 3 |

**Pass (P): 0 / 5 | Acceptable (A): 0 / 5 | Fail (F): 5 / 5**

## Scoring Criteria

- **P (Pass)**: Used `find_tool` to discover `list_models`, then called `list_models`.
- **A (Acceptable)**: Eventually found and used `list_models` but with extra unnecessary steps.
- **F (Fail)**: Did not use `list_models`; answered from general knowledge, used bash, or gave up.

## Root Cause: `list_models` is NOT registered

**CRITICAL FINDING**: The `list_models` tool is not registered as a deferred tool in the running server. All 5 test failures stem from a single wiring bug, not from prompt/discoverability issues.

### Evidence

1. `find_tool` with query `"models"` (Test 2) returned: `"No matching tools found."`
2. `find_tool` with query `"list models"` (Tests 1, 3, 5) returned 5 unrelated tools: `cron_list`, `list_delayed_callbacks`, `todos`, `skill`, `cancel_delayed_callback` -- no `list_models`.
3. `find_tool` with query `"select:models"` (Tests 1, 3) returned: `"Tool 'models' not found in available deferred tools."`

### The Bug

In `internal/harness/tools_default.go`, the `DefaultRegistryOptions` struct does **not** include a `ModelCatalog` field:

```go
type DefaultRegistryOptions struct {
    ApprovalMode    ToolApprovalMode
    Policy          ToolPolicy
    AskUserBroker   htools.AskUserQuestionBroker
    AskUserTimeout  time.Duration
    MemoryManager   om.Manager
    AgentRunner     htools.AgentRunner
    SkillLister     htools.SkillLister
    CronClient      htools.CronClient
    CallbackManager *htools.CallbackManager
    Activations     *ActivationTracker
}
```

Even though `cmd/harnessd/main.go` loads the catalog from `HARNESS_MODEL_CATALOG_PATH` (line 183), it never passes it through to the tool builder. The `buildOpts` in `NewDefaultRegistryWithOptions` has `ModelCatalog` always `nil`, so the guard `if buildOpts.ModelCatalog != nil` (line 136) is never satisfied.

### Fix Required

1. Add `ModelCatalog *catalog.Catalog` to `DefaultRegistryOptions`.
2. Wire it through in `NewDefaultRegistryWithOptions` to `buildOpts.ModelCatalog`.
3. In `cmd/harnessd/main.go`, pass the loaded catalog (from `providerRegistry`) into `DefaultRegistryOptions`.

## Detailed Results

### Test 1: "What models are available?"

**Score: F (Fail)** -- 4 turns, run_29

Tool call sequence:
1. `find_tool` query=`"select:models"` -- tried direct select, got "Tool 'models' not found"
2. `find_tool` query=`"list models"` -- keyword search returned 5 unrelated tools (cron_list, list_delayed_callbacks, etc.)
3. `skill` action=`"list"` -- tried skills listing, got empty list

**Final output**: "There are currently no explicitly registered or specialized models available in this environment."

**Notes**: The LLM correctly tried `find_tool` with two reasonable queries. The tool simply doesn't exist in the deferred registry. The LLM then tried the `skill` tool as a fallback, which also found nothing. Reasonable behavior given the tool is missing.

### Test 2: "List all the LLM models I can use"

**Score: F (Fail)** -- 2 turns, run_40

Tool call sequence:
1. `find_tool` query=`"models"` -- keyword search returned "No matching tools found."

**Final output**: "I currently do not have a dedicated tool activated to list available LLM models."

**Notes**: Correct use of `find_tool` with a relevant query. The search for "models" returned zero results because `list_models` is not in the deferred definitions list. The LLM gave an honest answer about not having the capability.

### Test 3: "Show me the GPT models available in this system"

**Score: F (Fail)** -- 3 turns, run_41

Tool call sequence:
1. `find_tool` query=`"select:models"` -- direct select failed ("Tool 'models' not found")
2. `find_tool` query=`"list available models"` -- returned 5 unrelated tools

**Final output**: "I currently do not have direct access to a tool that lists all available GPT models in the system."

**Notes**: Same pattern as Test 1. The LLM tried both select and keyword modes. The keyword search matched irrelevant tools because the terms "list" and "available" partially match other tool names/descriptions. Without `list_models` in the registry, there's no correct tool to find.

### Test 4: "Which model am I currently using?"

**Score: F (Fail)** -- 1 turn, run_42

Tool call sequence:
(none -- answered without tool calls)

**Final output**: "You are currently using the GPT-4 model."

**Notes**: The LLM answered from its own knowledge without using any tools. The answer is also factually incorrect -- the server is configured with `gpt-4.1-mini`, not `GPT-4`. Even if `list_models` were available, this prompt is more about introspection than listing. A `list_models` tool with an "info" action for the current model would address this, but the LLM would need to know to look for it.

### Test 5: "What's the cheapest model available?"

**Score: F (Fail)** -- 3 turns, run_45

Tool call sequence:
1. `find_tool` query=`"list models"` -- returned 5 unrelated tools
2. `skill` action=`"list"` -- got empty list

**Final output**: "I currently do not have direct access to a list of available models or their prices in this environment."

**Notes**: The LLM correctly tried `find_tool` to look for model-listing capabilities. After finding no relevant tool, it tried `skill` as a fallback. Reasonable behavior. With `list_models` available, this could use the `cost_tier` filter parameter.

## Search Quality Analysis

Even though the root cause is the missing tool registration, the search results reveal a secondary issue:

| Query | Expected Top Result | Actual Results |
|-------|-------------------|----------------|
| `"models"` | `list_models` (name substring match, score ~5) | No results |
| `"list models"` | `list_models` (name contains both terms, score ~15) | `cron_list`, `list_delayed_callbacks`, etc. |

The fact that `"models"` returns zero results confirms `list_models` is simply absent from the deferred registry. The `"list models"` query matching `cron_list` (via "list" substring) is expected behavior -- the searcher correctly scores partial matches, but without `list_models` in the pool, the best matches are irrelevant.

**Once `list_models` is wired into the registry**, the searcher should work correctly:
- `"models"` would match `list_models` via name substring (score 5) + tag exact match on "models" (score 8) = 13
- `"list models"` would match via name substring for both terms (score 5+5) + tag (score 8) = 18

## Recommendations

1. **[BLOCKER] Wire ModelCatalog through DefaultRegistryOptions** -- This is the only fix needed. Add the field, pass it from `main.go`, and all 5 tests should improve dramatically.
2. **Set HARNESS_MODEL_CATALOG_PATH** -- Ensure the env var points to `catalog/models.json` so the catalog loads at startup.
3. **Re-run this test suite** after both fixes are applied to validate the `find_tool -> list_models` discovery path.
4. **Consider adding a "current model" query** to `list_models` for Test 4-style prompts asking about the active model.
