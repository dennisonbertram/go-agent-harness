# Usability Test: list_mcp_resources (Round 2)

**Date**: 2026-03-09
**Purpose**: Retest MCP resource discovery after system prompt changes to improve deferred-tool discoverability via `find_tool`.
**Context**: System prompt now instructs the LLM to use `find_tool` before falling back. The `find_tool` description includes the hint "MCP server resources -> search mcp". No MCP servers are configured, so `list_mcp_resources` is not registered -- the test measures whether the LLM **attempts** discovery via `find_tool`.

## Scoring

| Grade | Meaning |
|-------|---------|
| **P** | Used `find_tool` to search for MCP-related tools |
| **A** | Extra steps but eventually tried `find_tool` |
| **F** | Did not attempt `find_tool` at all |

---

## Test 1: "List the MCP resources available"

| Field | Value |
|-------|-------|
| **Run ID** | `run_20` |
| **Status** | completed |
| **Turns** | 2 |
| **Grade** | **P** |

### Tool Calls
| Step | Tool | Args |
|------|------|------|
| 1 | `find_tool` | `{"query": "MCP"}` |

### Final Response
> There are no specific tools labeled as MCP resources available in the current environment. However, I can help with general code, files, searches, or tasks related to your project if you specify what you need regarding MCP resources. Let me know how you'd like to proceed!

### Analysis
Perfect behavior. The agent immediately used `find_tool` with the query "MCP", received no results (since no MCP servers are configured), and correctly reported that no MCP resources are available. Completed in 2 turns with minimal token usage.

---

## Test 2: "What MCP servers are connected?"

| Field | Value |
|-------|-------|
| **Run ID** | `run_21` |
| **Status** | **failed** (max steps reached) |
| **Turns** | 12 |
| **Grade** | **F** |

### Tool Calls
| Step | Tool | Args |
|------|------|------|
| 1 | `git_status` | `{}` |
| 2 | `ls` | `{"path": "internal/mcp", "recursive": true}` |
| 3 | `grep` | `{"query": "mcp"}` |
| 4 | `grep` | `{"path": "...README.md", "query": "mcp"}` |
| 5 | `grep` | `{"path": "...system-log.md", "query": "mcp"}` |
| 6 | `grep` | `{"path": "...engineering-log.md", "query": "mcp"}` |
| 7 | `read` | `{"path": "...engineering-log.md", "offset": 245}` |
| 8 | `grep` | `{"query": "mcp registry"}` |
| 9 | `read` | `{"path": "...tool-roadmap.md", "offset": 20}` |
| 10 | `read` | `{"path": "config/mcp-servers.yaml"}` |
| 11 | `grep` | `{"query": "mcp-server"}` |
| 12 | `read` | `{"path": "docs/research/codex-cli-review.md", "offset": 28}` |

### Final Response
> *(none -- hit max steps limit)*

### Analysis
Complete failure. The agent never attempted `find_tool`. Instead, it treated the question as a codebase investigation -- running `git_status`, browsing source code, grepping logs, and reading config files. It burned all 12 steps without discovering the tool or answering the question. The phrasing "What MCP servers are connected?" was interpreted as a question about the codebase rather than a request to use a tool.

---

## Test 3: "Show me the resources from the Model Context Protocol servers"

| Field | Value |
|-------|-------|
| **Run ID** | `run_31` |
| **Status** | completed |
| **Turns** | 2 |
| **Grade** | **P** |

### Tool Calls
| Step | Tool | Args |
|------|------|------|
| 1 | `find_tool` | `{"query": "mcp"}` |

### Final Response
> I don't have direct access to the Model Context Protocol (MCP) servers or their resources. However, if you have any files, logs, or data from the MCP servers that you want me to analyze or extract information from, please provide them and I can help. Let me know how you'd like to proceed.

### Analysis
Correct behavior. The agent used `find_tool` with "mcp" on the first step, found nothing, and reported accordingly. The full phrase "Model Context Protocol" was correctly mapped to the MCP acronym for the search. Completed efficiently in 2 turns.

---

## Test 4: "Are there any external tool servers I can use?"

| Field | Value |
|-------|-------|
| **Run ID** | `run_33` |
| **Status** | completed |
| **Turns** | 2 |
| **Grade** | **P** |

### Tool Calls
| Step | Tool | Args |
|------|------|------|
| 1 | `find_tool` | `{"query": "external tool server"}` |

### Final Response
> There are several external tool servers and services available that I can use to assist you, including:
> - Language Server Protocol (LSP) tools for code diagnostics, references, and restarting language servers (e.g., Go language server gopls).
> - Tools for fetching and downloading content from HTTP/HTTPS URLs.
> - Task and todo list management.
> - Skills registry access for applying specializations in code review, testing, refactoring, etc.
> - Scheduling recurring cron jobs.
> - One-shot delayed callbacks for scheduling single future actions.

### Analysis
Good behavior. The agent used `find_tool` with a semantically appropriate query "external tool server". While it didn't specifically search for "mcp", the broader query is reasonable given the prompt didn't mention MCP. It found deferred tools and reported them. Note: it did not specifically discover MCP resources (because none are configured), but the approach was correct.

---

## Summary

| Test | Prompt | Grade | Turns | Used find_tool? |
|------|--------|-------|-------|-----------------|
| 1 | "List the MCP resources available" | **P** | 2 | Yes -- `{"query": "MCP"}` |
| 2 | "What MCP servers are connected?" | **F** | 12 | No -- ran grep/ls/read on codebase |
| 3 | "Show me the resources from the Model Context Protocol servers" | **P** | 2 | Yes -- `{"query": "mcp"}` |
| 4 | "Are there any external tool servers I can use?" | **P** | 2 | Yes -- `{"query": "external tool server"}` |

### Overall Score: 3P / 0A / 1F (75% pass rate)

### Key Findings

1. **System prompt changes are working well for 3 out of 4 prompts.** Tests 1, 3, and 4 all immediately used `find_tool` as the first action, completing in just 2 turns each.

2. **Test 2 regression: "What MCP servers are connected?" failed completely.** The phrasing as a question ("What...are connected?") rather than an imperative ("List...") caused the agent to interpret it as a codebase investigation rather than a tool-discovery request. It used `git_status`, `ls`, `grep`, and `read` to search the source code for MCP server configurations.

3. **Compared to round 1 (if applicable):** The `find_tool` hint in the system prompt is effective for direct/imperative requests but does not yet handle interrogative phrasing reliably.

### Recommendations

1. **Strengthen the system prompt** for interrogative forms. Consider adding guidance like: "When a user asks about MCP, external tools, or connected servers, always use find_tool first."
2. **Add explicit keyword triggers** in the system prompt: if the user mentions "MCP", "external tool", or "connected server", the first action should be `find_tool`.
3. **Consider increasing max_steps** or adding an early-termination heuristic to avoid burning 12 steps on fruitless codebase searches.
