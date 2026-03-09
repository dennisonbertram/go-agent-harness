# Usability Test: lsp_restart -- Round 2

**Date:** 2026-03-09
**Purpose:** Retest lsp_restart discoverability after system prompt changes that instruct the LLM to use `find_tool` before falling back to bash. The `find_tool` description includes "LSP diagnostics, references, or restart -> search lsp" as a hint.
**Scoring:** P = used find_tool then lsp_restart, A = extra steps but eventually used lsp_restart, F = used bash to kill process

---

## Summary

| Test | Prompt | Score | Tools Used | Turns |
|------|--------|-------|------------|-------|
| 1 | "Restart the Go language server" | **A** | ls, find_tool, lsp_restart | 4 |
| 2 | "The LSP seems stuck, can you restart it?" | **F** | ls (x6), read (x3), bash | 12 (max steps) |
| 3 | "Restart gopls" | **F** | ls (x3), bash | 5 |
| 4 | "My code intelligence stopped working, fix it" | **P** | find_tool, lsp_restart | 3 |

**Overall: 1/4 P, 1/4 A, 2/4 F**

The system prompt changes improved discoverability for some prompts but results are inconsistent. The most abstract prompt ("code intelligence stopped working") achieved a perfect score, while the most direct prompts ("Restart gopls", "LSP seems stuck") failed completely -- the agent browsed the codebase or used bash to kill the process instead.

---

## Detailed Results

### Test 1: "Restart the Go language server"

- **Run ID:** run_12
- **Status:** completed
- **Score:** A (acceptable -- used find_tool but with unnecessary ls first)
- **Turns:** 4
- **Tool calls:**
  1. `ls` -- path: "." (unnecessary exploration)
  2. `find_tool` -- query: "lsp_restart" (correct, but too specific -- already knew the tool name)
  3. `lsp_restart` -- {} (correct)
- **Output:** "The Go language server (gopls) has been restarted successfully."
- **Notes:** The agent used `find_tool` with the exact tool name "lsp_restart", suggesting it already knew the tool existed. The initial `ls` was unnecessary. Scored A instead of P because of the extra ls step. Arguably close to P since the key path (find_tool -> lsp_restart) was followed.

### Test 2: "The LSP seems stuck, can you restart it?"

- **Run ID:** run_16
- **Status:** failed (max steps reached: 12)
- **Score:** F
- **Turns:** 12
- **Tool calls:**
  1. `ls` -- path: ".", depth: 1
  2. `ls` -- path: "internal", depth: 1
  3. `ls` -- path: "internal/harness", depth: 1
  4. `ls` -- path: "internal/harness/tools", depth: 1
  5. `ls` -- path: "internal/harness/tools/core", depth: 1
  6. `ls` -- path: "internal/harness/tools/core", depth: 2
  7. `read` -- path: "internal/harness/tools/lsp.go", offset 0-40
  8. `read` -- path: "internal/harness/tools/lsp.go", offset 40-120
  9. `read` -- path: "internal/harness/tools/lsp.go", offset 120-160
  10. `ls` -- path: "internal/harness/tools/lsp.go" (invalid ls on a file)
  11. `ls` -- path: "internal/harness/tools", depth: 1
  12. `bash` -- "pkill -f gopls || true"
- **Output:** N/A (failed due to max steps)
- **Notes:** Complete failure. The agent browsed the source code of the harness itself, trying to understand how LSP restart works internally, rather than using `find_tool` to discover the available tool. It spent 11 steps exploring the codebase before resorting to `bash pkill`. The phrasing "seems stuck" may have triggered diagnostic/investigation behavior instead of tool discovery. Never invoked `find_tool`.

### Test 3: "Restart gopls"

- **Run ID:** run_25
- **Status:** completed
- **Score:** F
- **Turns:** 5
- **Tool calls:**
  1. `ls` -- path: "."
  2. `ls` -- path: "cmd"
  3. `ls` -- path: "cmd/harnesscli"
  4. `bash` -- "pkill -HUP gopls || echo 'gopls not running'"
- **Output:** "The gopls language server was not running, so it was not restarted."
- **Notes:** Agent went straight to browsing the project structure and then used `bash` with `pkill -HUP` to signal gopls. Never used `find_tool`. The specific tool name "gopls" may have encouraged the agent to directly interact with the process rather than search for a dedicated tool. This is exactly the failure mode the system prompt change was meant to prevent.

### Test 4: "My code intelligence stopped working, fix it"

- **Run ID:** run_32
- **Status:** completed
- **Score:** P (perfect)
- **Turns:** 3
- **Tool calls:**
  1. `find_tool` -- query: "code intelligence" (correct semantic search)
  2. `lsp_restart` -- {} (correct)
- **Output:** "I have restarted the language server process (gopls) that powers code intelligence."
- **Notes:** Perfect execution. The abstract phrasing "code intelligence" triggered the agent to use `find_tool` with a semantic query. `find_tool` returned 6 tools (lsp_diagnostics, lsp_references, lsp_restart, fetch, download, skill) with lsp_restart scoring 3. The agent correctly chose lsp_restart from the results. This is the ideal flow.

---

## Analysis

### What worked
- **Abstract/problem-oriented prompts** trigger `find_tool`. When the user describes a problem ("code intelligence stopped working") rather than naming a specific action, the agent uses `find_tool` for semantic discovery.
- **The `find_tool` hint** ("LSP diagnostics, references, or restart -> search lsp") may be helping when the agent does think to search. Test 1 used `find_tool` with the exact query "lsp_restart".

### What failed
- **Direct action prompts** bypass `find_tool`. When the user says "restart gopls" or "restart the LSP", the agent interprets this as a direct command and reaches for `bash` or codebase exploration instead.
- **Diagnostic framing** ("seems stuck") triggers investigation behavior -- browsing source code to understand the system rather than using it.
- **The `ls` reflex** is still strong. 3 out of 4 tests started with `ls` before any other action.

### Comparison with Round 1 (if applicable)
This is the first lsp_restart usability test. Baseline established: 1P / 1A / 2F.

### Recommendations
1. **Strengthen the system prompt** to explicitly say: "When asked to restart, fix, or interact with a language server, use `find_tool` first -- do NOT use bash to kill processes."
2. **Add `lsp_restart` to the always-visible tool set** -- it is a simple, safe, frequently-needed tool that doesn't benefit from being deferred.
3. **Consider tool aliases** -- make "restart gopls" a recognized pattern in the system prompt hints.
4. **Reduce the `ls` reflex** -- the system prompt could state "Do not explore the project structure when the user asks for a specific action."

---

## Raw Data

Event files stored at:
- Test 1: `/tmp/sse-run_12.txt`
- Test 2: `/tmp/sse-run_16.txt`
- Test 3: `/tmp/sse-run_25.txt`
- Test 4: `/tmp/sse-run_32.txt`
