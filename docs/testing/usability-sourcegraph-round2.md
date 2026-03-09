# Usability Test: Sourcegraph Tool (Round 2)

**Date**: 2026-03-09
**Server**: `http://localhost:8080` (harnessd, gpt-4.1-mini)
**Goal**: Verify that the system prompt hint "Code search across repositories -> search sourcegraph" in find_tool's description causes the LLM to discover and use the `sourcegraph` deferred tool.

## Environment Note

**Critical finding**: The `sourcegraph` deferred tool is **not registered** in the running server. In `internal/harness/tools_default.go:122`, the tool is only added when `buildOpts.Sourcegraph.Endpoint != ""`. The `DefaultRegistryOptions` struct does not wire a Sourcegraph endpoint, and `cmd/harnessd/main.go` does not read any `HARNESS_SOURCEGRAPH_*` env var. As a result, `find_tool` cannot discover `sourcegraph` regardless of what query the LLM sends.

This means **all tests below are expected to fail** -- the tool physically does not exist in the deferred catalog.

---

## Scoring Key

| Grade | Meaning |
|-------|---------|
| **P** | Used `find_tool` -> discovered `sourcegraph` -> used it |
| **A** | Used `find_tool` but sourcegraph was not found; fell back gracefully |
| **F** | Skipped `find_tool` entirely, went straight to bash/grep |

---

## Test 1: "Use Sourcegraph to search for all implementations of the Runner interface"

| Field | Value |
|-------|-------|
| **Run ID** | `run_43` |
| **Status** | completed |
| **Score** | **A** (used find_tool, but selected wrong tool) |
| **Turns** | 3 |

### Tool Call Sequence

1. `find_tool` with `{"query": "select:lsp_references"}` -- activated `lsp_references`
2. `lsp_references` with `{"symbol": "Runner"}` -- gopls not available

### Analysis

The LLM attempted to use `find_tool` with `select:lsp_references` instead of searching for "sourcegraph". It used the `select:` syntax to directly activate an LSP tool rather than searching by keyword. The system prompt hint says to search "sourcegraph" but the LLM interpreted "implementations of an interface" as an LSP task. The find_tool description hint was not followed for the Sourcegraph-specific keyword.

---

## Test 2: "Search the codebase for error handling patterns using Sourcegraph"

| Field | Value |
|-------|-------|
| **Run ID** | `run_47` |
| **Status** | completed |
| **Score** | **A** (used find_tool, but wrong query) |
| **Turns** | 3 |

### Tool Call Sequence

1. `find_tool` with `{"query": "search error handling patterns"}` -- activated lsp_references, lsp_diagnostics, skill, cancel_delayed_callback (none is sourcegraph)
2. `grep` with `{"query": "error", "max_matches": 20}`

### Analysis

The LLM used `find_tool` with a generic natural-language query instead of the keyword "sourcegraph" that the description explicitly recommends. The fuzzy search returned 4 unrelated tools. The LLM then fell back to `grep`. Even if sourcegraph were registered, the query "search error handling patterns" might not have matched it (tags are `code-search`, `sourcegraph`). The LLM did not try the literal keyword "sourcegraph" despite it being mentioned in the user prompt.

---

## Test 3: "Find all TODO comments across all repositories"

| Field | Value |
|-------|-------|
| **Run ID** | `run_50` |
| **Status** | completed |
| **Score** | **F** (did not use find_tool at all) |
| **Turns** | 2 |

### Tool Call Sequence

1. `grep` with `{"query": "TODO", "path": "."}`

### Analysis

The LLM went straight to `grep` without consulting `find_tool` first. The phrase "across all repositories" is a strong signal for a cross-repo search tool like Sourcegraph, but the LLM treated it as a simple text search. No attempt was made to discover the sourcegraph tool.

---

## Test 4: "Search for how other projects implement tool activation"

| Field | Value |
|-------|-------|
| **Run ID** | `run_51` |
| **Status** | completed |
| **Score** | **F** (did not use find_tool at all) |
| **Turns** | 7 |

### Tool Call Sequence

1. `grep` with `{"query": "tool activation", "max_matches": 20}`
2. `read` (runner.go, offset 740, limit 20)
3. `read` (runner.go, offset 740, limit 40)
4. `grep` with `{"query": "activate tool", "max_matches": 3}`
5. `grep` with `{"query": "activate", "max_matches": 15}`
6. `read` (activation.go)

### Analysis

The LLM interpreted "other projects" as the current codebase and used grep + read to explore locally. No `find_tool` call was made. The phrase "other projects" is exactly the kind of cross-repo search that Sourcegraph enables, but the LLM did not recognize this.

---

## Summary Table

| # | Prompt | Score | find_tool used? | sourcegraph found? | sourcegraph used? | Turns |
|---|--------|-------|-----------------|--------------------|--------------------|-------|
| 1 | Runner interface implementations | **A** | Yes (select:lsp_references) | No | No | 3 |
| 2 | Error handling patterns using Sourcegraph | **A** | Yes (generic query) | No | No | 3 |
| 3 | TODO comments across all repositories | **F** | No | No | No | 2 |
| 4 | How other projects implement tool activation | **F** | No | No | No | 7 |

**Overall: 0P / 2A / 2F**

---

## Root Causes

### 1. Sourcegraph tool not registered (blocking)
The sourcegraph deferred tool is gated behind `Sourcegraph.Endpoint != ""` in `tools_default.go:122`. Since `DefaultRegistryOptions` does not expose a `Sourcegraph` field and `main.go` does not read a `HARNESS_SOURCEGRAPH_ENDPOINT` env var, the tool is never added to the deferred catalog. Even perfect LLM behavior cannot activate a tool that does not exist.

**Fix**: Either:
- Add `HARNESS_SOURCEGRAPH_ENDPOINT` / `HARNESS_SOURCEGRAPH_TOKEN` env var reading in `cmd/harnessd/main.go` and wire it into `DefaultRegistryOptions`
- Or add a `Sourcegraph` field to `DefaultRegistryOptions` and set it from `main.go`

### 2. LLM does not use the literal "sourcegraph" keyword (behavioral)
Even when `find_tool` is called, the LLM uses natural-language queries or `select:` for other tools instead of the keyword "sourcegraph" that the description explicitly recommends. In Test 2, the user literally says "using Sourcegraph" and the LLM still does not query `find_tool` with "sourcegraph".

**Fix**: Strengthen the system prompt to say something like: "When the user mentions Sourcegraph by name, always call find_tool with query 'sourcegraph' first."

### 3. "across all repositories" does not trigger cross-repo tool discovery (behavioral)
Tests 3 and 4 include phrases that imply cross-repo search but the LLM defaults to local grep. The system prompt does not teach the LLM that grep only searches the local workspace.

**Fix**: Add a note in the system prompt: "grep and glob only search the current workspace. For cross-repository search, use find_tool to discover the sourcegraph tool."

---

## Recommendations

1. **P0**: Wire `HARNESS_SOURCEGRAPH_ENDPOINT` in `main.go` so the deferred tool actually registers
2. **P1**: Add system prompt guidance distinguishing local search (grep) from cross-repo search (sourcegraph)
3. **P2**: Consider making the LLM always call `find_tool("sourcegraph")` when the user mentions "Sourcegraph" by name
4. **P2**: Re-run this test suite after fixing P0 to get meaningful behavioral signal
