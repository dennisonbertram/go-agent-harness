# Usability Test: lsp_diagnostics -- Round 2

**Date:** 2026-03-09
**Model:** gpt-4.1-mini
**Purpose:** Retest lsp_diagnostics discoverability after system prompt changes that instruct the LLM to use `find_tool` before falling back to bash.
**Scoring:** P = used find_tool then lsp_diagnostics, A = extra steps but acceptable, F = used bash (go build/go vet) or manual read instead

---

## Summary

| Test | Prompt | Score | Tools Used | Turns |
|------|--------|-------|------------|-------|
| 1 | "Check if there are any compilation errors in internal/harness/runner.go" | **F** | bash (`go build`) | 2 |
| 2 | "Run diagnostics on the Go files in internal/server/" | **F** | ls (x14), read (x3), bash (`gofmt`, `golangci-lint`) | 11 |
| 3 | "Are there any type errors in cmd/harnessd/main.go?" | **F** | read, bash (`go build`) | 3 |
| 4 | "Check this file for problems: internal/harness/tools/find_tool.go" | **F** | read (manual review only) | 2 |
| 5 | "I just edited internal/harness/runner.go, can you verify it compiles correctly?" | **F** | bash (`go build`) | 2 |

**Overall: 0/5 P, 0/5 A, 5/5 F**

The system prompt changes did NOT improve lsp_diagnostics discoverability. The agent used `bash` with `go build` in 3 out of 5 cases, manual `read` inspection in 1 case, and a chaotic multi-step combination of `ls`/`read`/`bash` (gofmt, golangci-lint) in 1 case. `find_tool` was never invoked. `lsp_diagnostics` was never discovered or used.

---

## Detailed Results

### Test 1: "Check if there are any compilation errors in internal/harness/runner.go"

- **Run ID:** run_8
- **Status:** completed
- **Score:** F
- **Turns:** 2
- **Tool calls:**
  1. `bash` -- command: `go build internal/harness/runner.go`
- **Notes:** Agent went straight to `bash go build` without considering find_tool or LSP diagnostics. This is the most direct bash-fallback case.

### Test 2: "Run diagnostics on the Go files in internal/server/"

- **Run ID:** run_13
- **Status:** completed
- **Score:** F
- **Turns:** 11
- **Tool calls:**
  1. `ls` -- path: "internal/server" (repeated 14 times with varying params)
  2. `read` -- path: "internal/server/http.go"
  3. `read` -- path: "internal/server/http_prompt_test.go"
  4. `read` -- path: "internal/server/http_test.go"
  5. `bash` -- command: `gofmt -l internal/server/*.go`
  6. `bash` -- command: `golangci-lint run internal/server/*.go`
- **Notes:** The worst result. The agent spiraled through 14 `ls` calls (most identical or near-identical), then read 3 files manually, and finally ran `gofmt` and `golangci-lint` via bash. The word "diagnostics" in the prompt should have been a strong signal to search for LSP tools, but the agent never attempted `find_tool`. The excessive `ls` calls also suggest the model struggled with the `ls` tool's parameters. 11 turns consumed the entire step budget with no structured diagnostic output.

### Test 3: "Are there any type errors in cmd/harnessd/main.go?"

- **Run ID:** run_22
- **Status:** completed
- **Score:** F
- **Turns:** 3
- **Tool calls:**
  1. `read` -- path: "cmd/harnessd/main.go"
  2. `bash` -- command: `go build cmd/harnessd/main.go`
- **Notes:** Agent read the file first, then ran `go build` via bash. The phrase "type errors" is a strong LSP-diagnostics signal that was ignored. No find_tool attempt.

### Test 4: "Check this file for problems: internal/harness/tools/find_tool.go"

- **Run ID:** run_30
- **Status:** completed
- **Score:** F
- **Turns:** 2
- **Tool calls:**
  1. `read` -- path: "internal/harness/tools/find_tool.go"
- **Notes:** The agent only read the file and provided a manual code review in its response. No compilation check, no diagnostics tool, no find_tool search. Ironically, it read the find_tool.go file itself (which contains the LSP hint in its own description) but never thought to use find_tool to discover lsp_diagnostics.

### Test 5: "I just edited internal/harness/runner.go, can you verify it compiles correctly?"

- **Run ID:** run_37
- **Status:** completed
- **Score:** F
- **Turns:** 2
- **Tool calls:**
  1. `bash` -- command: `go build ./internal/harness/runner.go`
- **Notes:** Straightforward bash fallback. The word "compiles" maps strongly to `go build` in the LLM's training data, making it unlikely to search for alternatives without explicit routing.

---

## Analysis

### Why find_tool is not being invoked for diagnostics

1. **`bash go build` is the obvious first choice** -- For "compilation errors," "type errors," and "verify it compiles," the LLM has extremely strong training signal that `go build` is the correct command. The `find_tool` hint about "LSP diagnostics" does not compete with this ingrained pattern.

2. **"Diagnostics" does not trigger LSP association** -- Test 2 used the word "diagnostics" explicitly, which is the exact term in the find_tool hint ("LSP diagnostics, references, or restart -> search lsp"). Despite this, the agent did not invoke find_tool. The hint is read when the LLM is already considering find_tool as a candidate; it does not cause the LLM to consider find_tool when it has already decided on bash or read.

3. **System prompt instruction is too weak** -- The prompt says "search for one before falling back to bash or general-purpose tools." But the LLM does not perceive `go build` as a "fallback" -- it sees it as the correct, primary tool for compilation checks. The instruction needs to be more explicit about what counts as a task that should route through find_tool.

4. **No negative signal from bash** -- Even when `go build` works perfectly (as it did in tests 1, 3, 5), there is no feedback telling the agent it should have used a different approach. The results are correct, just not via the preferred tool chain.

5. **read-only review (test 4) is worse than bash** -- In test 4, the agent did not even run `go build`. It just read the file and gave a manual review. This suggests that for vague prompts ("check for problems"), the agent's tool selection is inconsistent.

### Comparison with lsp_references Round 2

Both lsp_diagnostics and lsp_references show the same fundamental pattern: the LLM has strong existing tool preferences (grep for references, bash+go build for diagnostics) that the current find_tool hint and system prompt instruction fail to override. The deferred-tool discovery mechanism is not being triggered for either category.

### Recommendations for Round 3

1. **Explicit routing rules in system prompt** -- Add a direct instruction: "When the user asks to check for compilation errors, type errors, diagnostics, or problems in a file, ALWAYS use find_tool to search for 'lsp' first. Do NOT use `bash go build` or `bash go vet` directly."

2. **Negative examples** -- Add to system prompt: "Do NOT run `go build`, `go vet`, `gofmt`, or `golangci-lint` via bash for diagnostics. Use find_tool to discover the lsp_diagnostics tool instead."

3. **Promote lsp_diagnostics to active toolset** -- If the tool is expected to be used frequently, making it non-deferred eliminates the discovery problem entirely. The deferred-tool pattern works best for truly rare tools, not for tools that should be used in common workflows.

4. **Stronger find_tool-first instruction** -- The current system prompt says "If a task feels like it should have a dedicated tool, search for one." Change to: "ALWAYS use find_tool before using bash for code analysis, diagnostics, references, or any IDE-like operation."

5. **Consider tool aliasing** -- Register `go_diagnostics` or `check_errors` as aliases that route to lsp_diagnostics, using terminology the LLM naturally reaches for.

---

## Configuration at Test Time

- **System prompt** (`prompts/base/main.md`): Includes instruction to use find_tool "before falling back to bash or general-purpose tools" and lists LSP diagnostics as a discoverable capability.
- **find_tool description** (`descriptions/find_tool.md`): Includes hint "LSP diagnostics, references, or restart -> search lsp."
- **lsp_diagnostics status**: Deferred (not in active toolset; must be discovered via find_tool).
