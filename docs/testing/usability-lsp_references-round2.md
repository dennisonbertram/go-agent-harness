# Usability Test: lsp_references -- Round 2

**Date:** 2026-03-09
**Purpose:** Retest lsp_references discoverability after system prompt changes that instruct the LLM to use `find_tool` before falling back to bash/grep.
**Scoring:** P = used find_tool then lsp_references, A = extra steps but acceptable, F = used grep instead

---

## Summary

| Test | Prompt | Score | Tools Used | Turns |
|------|--------|-------|------------|-------|
| 1 | "Find all references to the RunStep function in this codebase" | **F** | grep | 2 |
| 2 | "Where is the Definition type used across the project?" | **F** | grep (x3) | 4 |
| 3 | "Find all callers of BuildCatalog" | **F** | grep, read (x3), grep | 6 |
| 4 | "What functions call MarshalToolResult?" | **F** | grep | 2 |
| 5 | "Show me everywhere ActivationTracker is referenced" | **F** | grep | 2 |

**Overall: 0/5 P, 0/5 A, 5/5 F**

The system prompt changes did NOT improve lsp_references discoverability. The agent used grep exclusively across all 5 test cases and never invoked `find_tool` to discover the LSP tools.

---

## Detailed Results

### Test 1: "Find all references to the RunStep function in this codebase"

- **Run ID:** run_9
- **Status:** completed
- **Score:** F
- **Turns:** 2
- **Tool calls:**
  1. `grep` -- query: "RunStep", max_matches: 2000
- **Result:** Correctly reported no matches (RunStep does not exist in the codebase). However, an LSP-aware tool would have been the semantically correct approach for "find all references."
- **Notes:** Agent went straight to grep without considering find_tool or LSP.

### Test 2: "Where is the Definition type used across the project?"

- **Run ID:** run_11
- **Status:** completed
- **Score:** F
- **Turns:** 4
- **Tool calls:**
  1. `grep` -- query: "Definition", max_matches: 100
  2. `grep` -- query: "Definition", path: "internal/harness/tools/agent.go", max_matches: 20
  3. `grep` -- query: "Definition", path: "internal/harness/registry.go", max_matches: 20
- **Result:** Found references via text search. Multiple grep calls needed to narrow results.
- **Notes:** LSP references would have been more precise (type-aware, not just text matching). Agent never attempted find_tool.

### Test 3: "Find all callers of BuildCatalog"

- **Run ID:** run_14
- **Status:** completed
- **Score:** F
- **Turns:** 6
- **Tool calls:**
  1. `grep` -- query: "BuildCatalog", regex: true
  2. `read` -- path: "internal/harness/tools/catalog.go", offset: 0, limit: 40
  3. `read` -- path: "internal/harness/tools/catalog.go", offset: 40, limit: 40
  4. `read` -- path: "internal/harness/tools/catalog.go", offset: 80, limit: 40
  5. `grep` -- query: "BuildCatalog", path: "internal/harness/tools/catalog_test.go"
- **Result:** Found callers but required 6 turns and 5 tool calls. LSP references would have returned this in 1 call.
- **Notes:** This is the worst case -- 6 turns for something lsp_references handles in 1. Still no find_tool attempt.

### Test 4: "What functions call MarshalToolResult?"

- **Run ID:** run_19
- **Status:** completed
- **Score:** F
- **Turns:** 2
- **Tool calls:**
  1. `grep` -- query: "MarshalToolResult", max_matches: 2000
- **Result:** Found references via text search in a single grep call.
- **Notes:** Grep was efficient here, but LSP would provide semantically accurate call sites. No find_tool attempt.

### Test 5: "Show me everywhere ActivationTracker is referenced"

- **Run ID:** run_23
- **Status:** completed
- **Score:** F
- **Turns:** 2
- **Tool calls:**
  1. `grep` -- query: "ActivationTracker"
- **Result:** Found references correctly.
- **Notes:** Word "referenced" in prompt maps directly to LSP references, yet agent still used grep. No find_tool attempt.

---

## Analysis

### Why find_tool is not being invoked

1. **grep is "good enough"** -- The agent has grep in its active toolset and it works for simple text searches. There is no strong signal that grep is insufficient before using it.
2. **System prompt hint is passive** -- The `find_tool` description says "LSP diagnostics, references, or restart -> search lsp" but this hint is buried inside the tool description. The LLM reads tool descriptions only when considering which tool to use, and grep already seems to match "find references."
3. **No explicit routing rule** -- The system prompt tells the LLM to use find_tool "before falling back to bash/grep" but the LLM treats grep as a first-class code-search tool, not a "fallback." The instruction framing does not override the LLM's preference for familiar tools.
4. **LSP is an unfamiliar concept to the LLM context** -- The model may not associate user phrases like "references," "callers," or "used across" with the LSP protocol concept of "references." It sees them as text-search tasks.

### Recommendations for Round 3

1. **Explicit routing in system prompt** -- Add a rule: "When the user asks for references, callers, usages, or definitions of a symbol, ALWAYS use find_tool to search for LSP tools first. Do NOT use grep for symbol-level queries."
2. **Keyword triggers** -- Enumerate trigger words in the system prompt: "references", "callers", "usages", "definition", "where is X used", "who calls X" should all route to find_tool -> lsp_references.
3. **Remove ambiguity** -- Currently the find_tool hint says "search lsp" but the user's prompt says "find references." The LLM needs to be told that "find references" == "LSP references" explicitly.
4. **Consider promoting lsp_references to the active toolset** -- If it is used frequently enough, making it non-deferred eliminates the discovery problem entirely.

---

## Comparison with Round 1

No Round 1 results file found for comparison. This serves as the baseline.
