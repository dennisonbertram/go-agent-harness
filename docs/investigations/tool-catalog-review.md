# Tool Catalog Review

**Date**: 2026-03-09
**Scope**: Full review of all tools registered in `internal/harness/tools_default.go`
**Method**: Source code analysis of every tool handler + usability test results from `docs/testing/`

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Core Tools (Always Visible)](#core-tools-always-visible)
3. [Deferred Tools (Hidden Behind find_tool)](#deferred-tools-hidden-behind-find_tool)
4. [Meta Tool](#meta-tool)
5. [Cut / Keep / Enhance Summary](#cut--keep--enhance-summary)
6. [Gaps -- Missing Capabilities](#gaps----missing-capabilities)
7. [Tier Reclassification Recommendations](#tier-reclassification-recommendations)

---

## Executive Summary

The harness registers **15 core tools** and up to **24 deferred tools** (depending on configuration). Usability testing reveals a consistent pattern: **the deferred-tool discovery mechanism (find_tool) fails for gpt-4.1-mini in the majority of cases**. Tools that are deferred but should be commonly used (todos, lsp_diagnostics, lsp_references) have 0-25% discovery rates. Promoting `todos` to core tier took it from 0/5 pass to 4/5 pass (Round 3 test). This strongly suggests the deferred tier should be reserved for truly rare/specialized tools, and several current deferred tools need promotion.

### Quick Verdict

| Category | Count | Keep | Cut | Enhance |
|----------|-------|------|-----|---------|
| Core | 15 | 10 | 2 | 3 |
| Deferred | 24 | 10 | 5 | 9 |
| **Total** | **39** | **20** | **7** | **12** |

---

## Core Tools (Always Visible)

### 1. `read`

**Source**: `internal/harness/tools/core/read.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Reads file content from the workspace with optional line offset/limit and returns versioned output. |
| **Bash alternative?** | `cat`, `head`, `tail` via bash -- but loses version tracking, structured JSON output, and workspace-path sandboxing. |
| **LLM usage** | Extremely high -- used in nearly every test run across all usability tests. |
| **Unique value** | File versioning (SHA256 hash enables optimistic concurrency in edit/write), workspace-path resolution and sandboxing, structured line-number output for offset reads. |
| **Recommendation** | **KEEP** -- foundational tool, no changes needed. |

### 2. `write`

**Source**: `internal/harness/tools/core/write.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Writes or appends content to a workspace file with optimistic concurrency via `expected_version`. |
| **Bash alternative?** | `echo > file` or `tee` -- but loses version checking, diff reporting, and parent-directory auto-creation. |
| **LLM usage** | High -- frequently used, sometimes incorrectly (e.g., fabricating downloaded content instead of using fetch/download). |
| **Unique value** | Optimistic concurrency control (stale_write detection), automatic parent directory creation, diff metadata (before/after bytes). |
| **Recommendation** | **KEEP** -- foundational. The overuse problem (LLM writing fabricated content) is a system prompt issue, not a tool issue. |

### 3. `edit`

**Source**: `internal/harness/tools/core/edit.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Performs find-and-replace text editing on a workspace file with version checking. |
| **Bash alternative?** | `sed` via bash -- but sed is error-prone for multi-line replacements, lacks version tracking, and requires escaping. |
| **LLM usage** | Moderate -- used in test 4 of todos round 2 (incorrectly, to edit markdown checkboxes). |
| **Unique value** | Safer than sed for LLM use (no regex escaping issues), version tracking, replace_all option. |
| **Recommendation** | **KEEP** -- core editing primitive. Overlaps with `apply_patch` but serves different use cases (single replacement vs. multi-hunk patches). |

### 4. `apply_patch`

**Source**: `internal/harness/tools/core/apply_patch.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Applies find/replace patches, multi-edit arrays, or unified diff patches to workspace files. |
| **Bash alternative?** | `patch` or `sed` -- but the unified patch format here is custom (not standard unified diff), so bash `patch` would not work directly. |
| **LLM usage** | Unclear from test data -- no usability tests specifically target apply_patch. |
| **Unique value** | Multi-edit support (batch multiple replacements in one call), custom unified patch format for multi-file changes, version checking. Overlaps significantly with `edit`. |
| **Recommendation** | **ENHANCE** -- The tool handles three distinct input formats (find/replace, edits array, unified patch). This makes the parameter schema complex and may confuse the LLM. Consider whether `edit` + `apply_patch` should be consolidated, or whether `apply_patch` should be specialized to only handle the unified patch format and the edits-array format (removing the simple find/replace which duplicates `edit`). |

### 5. `bash`

**Source**: `internal/harness/tools/core/bash.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Executes bash commands in the workspace with timeout, background job support, and dangerous-command filtering. |
| **Bash alternative?** | N/A -- this IS the bash tool. |
| **LLM usage** | Extremely high -- used as the fallback for everything the LLM does not have a specialized tool for. Usability tests show it is the LLM's first choice for compilation checks, process management, and file downloads. |
| **Unique value** | Dangerous command filtering (rm -rf /, sudo, shutdown, fork bombs), timeout enforcement, background job management, workspace-scoped execution. |
| **Recommendation** | **KEEP** -- essential. The dangerous-command patterns are solid. Consider adding `curl` and `wget` to the caution list (not blocked, but flagged) to encourage use of `download`/`fetch` tools instead. |

### 6. `job_output`

**Source**: `internal/harness/tools/core/job.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Reads stdout/stderr from a background bash job by shell_id. |
| **Bash alternative?** | No direct equivalent -- background jobs are managed internally by the JobManager, not via shell job control. |
| **LLM usage** | Low -- only relevant when `run_in_background: true` is used in bash. |
| **Unique value** | Required companion to bash's background mode. Without it, background job output is inaccessible. |
| **Recommendation** | **KEEP** -- necessary for the background job workflow. |

### 7. `job_kill`

**Source**: `internal/harness/tools/core/job.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Terminates a background bash job by shell_id. |
| **Bash alternative?** | `kill <pid>` -- but the LLM does not have the PID, only the shell_id from JobManager. |
| **LLM usage** | Very low -- rarely needed. |
| **Unique value** | Required companion to bash's background mode for cleanup. |
| **Recommendation** | **CUT (move to deferred)** -- This tool is rarely needed and occupies a core slot. It should be deferred and discoverable via find_tool when the LLM needs to kill a background job. The LLM already has the shell_id from the bash background response, so it can use find_tool to locate `job_kill` when needed. |

### 8. `ls`

**Source**: `internal/harness/tools/core/ls.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Lists files and directories in the workspace with recursive traversal, depth limits, and hidden file filtering. |
| **Bash alternative?** | `ls -la` or `find` via bash -- comparable functionality but loses workspace sandboxing and structured JSON output. |
| **LLM usage** | Very high -- used frequently, sometimes excessively (usability tests show 14 repeated ls calls in one session). The "ls reflex" is a documented problem where the LLM starts by listing directories before doing anything else. |
| **Unique value** | Workspace-path sandboxing, structured output, hidden-file filtering, depth control. |
| **Recommendation** | **KEEP** -- but the excessive-use problem should be addressed in the system prompt, not the tool. |

### 9. `glob`

**Source**: `internal/harness/tools/core/glob.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Matches files in the workspace by glob pattern (e.g., `**/*.go`). |
| **Bash alternative?** | `find . -name "*.go"` or shell globbing -- comparable but uses Go's `filepath.Glob` which does not support `**` recursive patterns (unlike bash's `globstar`). |
| **LLM usage** | Moderate -- used for file discovery before reading/editing. |
| **Unique value** | Workspace sandboxing, structured output. However, Go's `filepath.Glob` does NOT support `**` recursive patterns, which is a significant limitation compared to what the LLM expects from glob. |
| **Recommendation** | **ENHANCE** -- The tool uses `filepath.Glob` which does not support `**` (double-star) recursive matching. This is a critical gap: the LLM will try `**/*.go` patterns and get no results. Either switch to a library that supports `**` (e.g., `doublestar`) or document the limitation clearly in the tool description. |

### 10. `grep`

**Source**: `internal/harness/tools/core/grep.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Searches file contents in the workspace for text or regex matches, walking directories recursively. |
| **Bash alternative?** | `grep -r` or `rg` (ripgrep) via bash -- bash grep is faster for large codebases since this implementation reads entire files into memory and uses Go's regexp. |
| **LLM usage** | Extremely high -- the LLM's go-to tool for code search, used in preference to lsp_references in 100% of usability tests. |
| **Unique value** | Workspace sandboxing, structured JSON output with file/line/content, automatic binary-file skipping, .git directory exclusion. |
| **Recommendation** | **KEEP** -- heavily used. Performance could be improved for large codebases (currently reads entire files into memory), but functionally solid. |

### 11. `git_status`

**Source**: `internal/harness/tools/core/git.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Runs `git status --porcelain=v1` and returns structured output indicating whether the workspace is clean. |
| **Bash alternative?** | `git status` via bash -- identical underlying command but loses the structured `clean` boolean field. |
| **LLM usage** | Moderate -- used at the start of some sessions to assess workspace state. |
| **Unique value** | The `clean` boolean field is marginally useful for conditional logic. Otherwise this is a thin wrapper around `git status`. |
| **Recommendation** | **CUT (merge into bash guidance)** -- This is a very thin wrapper. The LLM could run `git status` via bash and get the same information. The `clean` boolean is the only added value, and the LLM can determine that from empty output. Consider removing this tool and adding system prompt guidance to use `bash` for git operations. Alternatively, if git tools are kept, they should be enhanced with more git operations (commit, add, log, etc.) to justify their existence as a category. |

### 12. `git_diff`

**Source**: `internal/harness/tools/core/git.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Runs `git diff` with optional `--staged`, target revision, and path filtering, with output truncation. |
| **Bash alternative?** | `git diff` via bash -- identical underlying command but loses the structured truncation handling and max_bytes limit. |
| **LLM usage** | Low to moderate -- used when reviewing changes. |
| **Unique value** | Output truncation at `max_bytes` prevents context window overflow from large diffs. This is genuinely useful and hard to replicate cleanly via bash. |
| **Recommendation** | **KEEP** -- the output truncation feature justifies its existence. Without it, a large `git diff` via bash could blow up the context window. |

### 13. `AskUserQuestion`

**Source**: `internal/harness/tools/core/ask_user_question.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Presents structured multiple-choice questions to the user and waits for answers, with configurable timeout. |
| **Bash alternative?** | No -- requires the SSE/broker infrastructure for real-time user interaction. |
| **LLM usage** | Moderate but sometimes inappropriate -- usability tests show the LLM using it to ask unnecessary clarifying questions instead of just doing the task (todos tests 2 and 5). |
| **Unique value** | The only tool that enables synchronous human-in-the-loop interaction. Required for approval modes and clarification. |
| **Recommendation** | **KEEP** -- unique capability. The overuse problem (asking clarifying questions instead of acting) is a system prompt issue. |

### 14. `observational_memory`

**Source**: `internal/harness/tools/core/observational_memory.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Manages per-conversation observational memory: enable/disable, configure, export, review (via sub-agent), and force reflection. |
| **Bash alternative?** | No -- tightly integrated with the MemoryManager and requires SQLite-backed conversation-scoped state. |
| **LLM usage** | Unknown -- no usability tests cover this tool. |
| **Unique value** | Persistent memory across conversation turns, reflection/consolidation of observations, export to markdown/JSON. This is a differentiating feature for long-running agent sessions. |
| **Recommendation** | **ENHANCE** -- The tool is complex (6 actions: enable, disable, status, export, review, reflect_now) which may overwhelm the LLM. Consider whether the review action (which spawns a sub-agent) should be separated into its own tool. Also needs usability testing. |

### 15. `todos`

**Source**: `internal/harness/tools/deferred/todos.go` (code is in deferred package but registered as core)

| Aspect | Assessment |
|--------|------------|
| **What it does** | Manages a run-scoped in-memory todo list (CRUD via full-list replacement). |
| **Bash alternative?** | Writing a text file via bash/write -- which is exactly what the LLM does when it cannot find this tool. |
| **LLM usage** | 0% when deferred (Round 2: 0/5), 80% when core (Round 3: 4/5 pass). Promotion to core tier was the single most impactful change in usability testing. |
| **Unique value** | Structured task tracking with status (pending/in_progress/completed), run-scoped state. Much better than ad-hoc text files. |
| **Recommendation** | **ENHANCE** -- Already promoted to core (good). Needs partial-update support (current API requires full-list replacement which forces a read-then-write pattern). Add `update` action to modify a single item by ID without sending the full list. |

---

## Deferred Tools (Hidden Behind find_tool)

### 16. `find_tool` (Meta Tool)

**Source**: `internal/harness/tools/find_tool.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Searches deferred tools by keyword or activates a specific tool by name (`select:<name>`). Acts as the gateway to all deferred capabilities. |
| **Bash alternative?** | No -- this is an internal meta-tool for tool discovery. |
| **LLM usage** | Mixed. When the LLM does use it, it works well (download Round 2: 3/5, MCP Round 2: 3/4, skill Round 2: 3/4). But for many categories, the LLM never thinks to use it (LSP diagnostics: 0/5, LSP references: 0/5, todos: 0/5). |
| **Unique value** | Enables the deferred-tool pattern which keeps the active tool count low. The keyword searcher works well when invoked. |
| **Recommendation** | **ENHANCE** -- The tool works but the LLM does not reliably invoke it. Key issues: (1) the LLM's existing tool preferences (grep for search, bash for compilation) override find_tool guidance, (2) gpt-4.1-mini may lack the reasoning capacity for the two-step discovery pattern. Consider stronger system prompt routing rules or promoting high-value deferred tools to core. |

### 17. `fetch`

**Source**: `internal/harness/tools/deferred/fetch.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Fetches URL content via HTTP GET with timeout and size limits, returning the body as text. |
| **Bash alternative?** | `curl` via bash -- nearly identical functionality, with curl being more familiar to the LLM. |
| **LLM usage** | No usability test data specific to fetch (download tests cover the download variant). |
| **Unique value** | Structured output (status_code, content_type, truncation flag), workspace-independent (does not write to disk). Compared to `curl` via bash, the main advantages are size limiting and structured JSON output. |
| **Recommendation** | **KEEP (deferred)** -- reasonable as a deferred tool. The overlap with `curl` via bash is real but the structured output and size limits add value. |

### 18. `download`

**Source**: `internal/harness/tools/deferred/download.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Downloads URL content and saves it to a workspace file with size limits and version tracking. |
| **Bash alternative?** | `curl -o file url` or `wget` via bash -- comparable but loses workspace sandboxing, version tracking, and size limits. |
| **LLM usage** | 60% discovery rate when deferred (Round 2: 3/5 pass). When discovered, it works correctly. Failures are from content fabrication (LLM writes fake content instead of discovering the tool). |
| **Unique value** | Combines fetch + write in one atomic operation with workspace sandboxing. Prevents the LLM from fabricating file contents. |
| **Recommendation** | **ENHANCE (promote to core)** -- Usability tests show content fabrication as the failure mode when this tool is not discovered. Promoting to core would eliminate the discovery barrier. The download test report itself recommends this: "Consider making `download` a core tool." |

### 19-21. `lsp_diagnostics`, `lsp_references`, `lsp_restart`

**Source**: `internal/harness/tools/deferred/lsp.go`

| Tool | What it does |
|------|-------------|
| `lsp_diagnostics` | Runs `gopls check` on a file or package to get compilation diagnostics. |
| `lsp_references` | Runs `gopls workspace_symbol` to find symbol references across the workspace. |
| `lsp_restart` | No-op placeholder that returns `{"restarted": true}` -- does not actually restart anything. |

| Aspect | Assessment |
|--------|------------|
| **Bash alternative?** | `gopls check` and `gopls workspace_symbol` via bash -- identical functionality since these tools are thin wrappers around gopls commands. |
| **LLM usage** | **0% discovery rate** across all tests. lsp_diagnostics: 0/5, lsp_references: 0/5, lsp_restart: 1/4 (only when the prompt was abstract: "code intelligence stopped working"). The LLM always uses `go build` via bash for diagnostics and `grep` for references. |
| **Unique value for diagnostics** | Minimal -- `gopls check` via bash produces the same output. The tool adds workspace-path resolution but that is it. |
| **Unique value for references** | Minimal -- `gopls workspace_symbol` via bash produces the same output. LSP references WOULD be valuable if they provided semantic (type-aware) results, but `workspace_symbol` is just a fuzzy name search, not true reference finding. |
| **Unique value for restart** | **None** -- the handler is a no-op. It does not actually restart any process. |
| **Recommendation** | **CUT all three.** These tools are thin wrappers around shell commands with zero discovery rate. The LLM will never use them because `bash go build` and `grep` are stronger priors. The restart tool is a no-op. If LSP integration is desired, it needs to be a real LSP client (not shelling out to gopls), and the tools would need to be core-tier to overcome the discovery barrier. Current implementation adds no value over bash. |

### 22. `sourcegraph`

**Source**: `internal/harness/tools/deferred/sourcegraph.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Sends a search query to a Sourcegraph instance via HTTP POST and returns results. |
| **Bash alternative?** | `curl` to the Sourcegraph API via bash -- but requires knowing the API format and auth token. |
| **LLM usage** | 0% -- not registered (gated behind `Sourcegraph.Endpoint != ""` which is never set). Usability tests confirm: 0/4 pass, 2/4 acceptable (used find_tool but sourcegraph was absent). |
| **Unique value** | Encapsulates the Sourcegraph API (endpoint, auth token, query format) so the LLM does not need to know API details. Enables cross-repo search. |
| **Recommendation** | **KEEP (deferred)** -- but fix the wiring bug. The tool is not registered because `DefaultRegistryOptions` does not expose the Sourcegraph config. Add `HARNESS_SOURCEGRAPH_ENDPOINT` and `HARNESS_SOURCEGRAPH_TOKEN` env var support in `cmd/harnessd/main.go`. When properly configured, this tool adds genuine value for cross-repo search that grep cannot replicate. |

### 23-24. `list_mcp_resources`, `read_mcp_resource`

**Source**: `internal/harness/tools/deferred/mcp.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Lists and reads resources from configured MCP (Model Context Protocol) servers. |
| **Bash alternative?** | No -- MCP is a custom protocol that requires the registry infrastructure. |
| **LLM usage** | When MCP is configured: unknown (no tests with active MCP servers). Discovery rate via find_tool: 75% for list, 25% for read. |
| **Unique value** | Required for MCP integration -- no alternative exists. |
| **Recommendation** | **KEEP (deferred)** -- appropriate as deferred tools since they require MCP server configuration. The dynamic MCP tools (auto-generated from server tool listings) are also well-designed. |

### 25. Dynamic MCP Tools

**Source**: `internal/harness/tools/deferred/mcp.go` (`DynamicMCPTools`)

| Aspect | Assessment |
|--------|------------|
| **What it does** | Auto-generates deferred tools from MCP server tool listings, proxying calls through the MCP registry. |
| **Bash alternative?** | No -- requires the MCP protocol infrastructure. |
| **LLM usage** | Depends on configuration. |
| **Unique value** | Enables arbitrary tool extensibility via MCP servers. Well-architected: auto-namespacing (`mcp_<server>_<tool>`), proper tag propagation. |
| **Recommendation** | **KEEP (deferred)** -- good design, appropriate tier. |

### 26. `list_models`

**Source**: `internal/harness/tools/deferred/list_models.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Lists, filters, and inspects available LLM models from the provider catalog with rich filtering (tool_calling, speed_tier, cost_tier, modality, reasoning). |
| **Bash alternative?** | No -- requires the catalog.Catalog infrastructure. |
| **LLM usage** | 0% -- **not registered due to wiring bug**. `ModelCatalog` is never passed through `DefaultRegistryOptions`. All 5 usability tests failed because the tool does not exist in the deferred catalog, not because of discovery issues. When the LLM did use find_tool to search for it, the search worked correctly but returned no results. |
| **Unique value** | Rich model filtering (by provider, capabilities, cost, speed). Enables the LLM to make informed model selection decisions. |
| **Recommendation** | **ENHANCE (fix wiring bug)** -- Add `ModelCatalog *catalog.Catalog` to `DefaultRegistryOptions`, wire it from `cmd/harnessd/main.go`. The tool design is solid; the only issue is that it is never registered. Keep as deferred once fixed. |

### 27. `skill`

**Source**: `internal/harness/tools/deferred/skill.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Lists available skills and applies (resolves) a named skill, returning instructions and allowed tools. |
| **Bash alternative?** | No -- requires the SkillLister interface for skill resolution. |
| **LLM usage** | 50% discovery rate (Round 2: 2/4 pass, 1/4 acceptable). Works well with explicit action verbs ("apply the code review skill") but fails with conversational phrasing ("what skills do you have?"). |
| **Unique value** | Enables the skills system -- specialized instruction sets that modify agent behavior. No bash alternative. |
| **Recommendation** | **KEEP (deferred)** -- appropriate tier. The discovery rate is acceptable for a specialized capability. System prompt improvements would help with conversational phrasing. |

### 28. `agent`

**Source**: `internal/harness/tools/deferred/agent.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Runs a delegated sub-agent prompt, returning the sub-agent's output. |
| **Bash alternative?** | No -- requires the AgentRunner interface for nested LLM calls. |
| **LLM usage** | Unknown -- no specific usability tests. Used internally by observational_memory's review action. |
| **Unique value** | Enables sub-agent delegation for complex tasks, research, and analysis. |
| **Recommendation** | **KEEP (deferred)** -- appropriate for a powerful but specialized capability. |

### 29. `agentic_fetch`

**Source**: `internal/harness/tools/deferred/agent.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | Fetches web content and analyzes it with a delegated sub-agent in one call. |
| **Bash alternative?** | `curl` + paste into prompt -- but loses the atomic fetch-then-analyze pattern. |
| **LLM usage** | Unknown -- no specific usability tests. |
| **Unique value** | Combines web fetching with LLM analysis in a single tool call, avoiding the need to pass fetched content through the conversation. |
| **Recommendation** | **CUT** -- This tool's value proposition is thin. The LLM can fetch content (via `fetch` or `curl`) and then analyze it in the same conversation turn. The "agentic" wrapper adds a sub-agent call, but the main agent could do the analysis itself. The tool exists as a convenience but adds complexity without clear benefit. If retained, it overlaps with `web_fetch` + agent reasoning. |

### 30-31. `web_search`, `web_fetch`

**Source**: `internal/harness/tools/deferred/web.go`

| Aspect | Assessment |
|--------|------------|
| **What it does** | `web_search` performs a web search query returning results. `web_fetch` fetches a webpage's content. Both delegate to the WebFetcher interface. |
| **Bash alternative?** | No direct bash equivalent for web_search (requires a search API). `web_fetch` overlaps with `fetch` and `curl`. |
| **LLM usage** | Unknown -- no specific usability tests. Gated behind `WebFetcher != nil` which may not be configured. |
| **Unique value** | `web_search` provides search capability that has no bash equivalent. `web_fetch` is largely redundant with `fetch`. |
| **Recommendation** | **KEEP web_search (deferred)**, **CUT web_fetch** -- `web_fetch` duplicates `fetch` with no additional value. The only difference is that `web_fetch` uses the WebFetcher interface while `fetch` uses a raw HTTP client, but both return page content. Consolidate into one. |

### 32-37. Cron Tools (`cron_create`, `cron_list`, `cron_get`, `cron_delete`, `cron_pause`, `cron_resume`)

**Source**: `internal/harness/tools/deferred/cron.go`

| Aspect | Assessment |
|--------|------------|
| **What they do** | Full CRUD + pause/resume for cron jobs via the CronClient interface. |
| **Bash alternative?** | `crontab -e` via bash -- but loses the structured API, execution history tracking, and pause/resume semantics. |
| **LLM usage** | Unknown -- no specific usability tests. Gated behind `CronClient != nil`. |
| **Unique value** | Managed cron with execution history, pause/resume, and structured job definitions. Significantly richer than raw crontab. |
| **Recommendation** | **KEEP all six (deferred)** -- appropriate tier for a specialized scheduling subsystem. The six-tool CRUD pattern is well-structured. Consider whether `cron_pause` and `cron_resume` could be merged into a single `cron_update` tool to reduce tool count. |

### 38-40. Delayed Callback Tools (`set_delayed_callback`, `cancel_delayed_callback`, `list_delayed_callbacks`)

**Source**: `internal/harness/tools/deferred/delayed_callback.go`

| Aspect | Assessment |
|--------|------------|
| **What they do** | Schedule, cancel, and list one-shot delayed callbacks that trigger new agent runs after a specified delay. |
| **Bash alternative?** | `sleep <n> && command` via bash background job -- but loses the structured callback management and new-run triggering. |
| **LLM usage** | Unknown -- no specific usability tests. Gated behind `CallbackManager != nil`. |
| **Unique value** | Enables "check back later" patterns (e.g., "run tests in 5 minutes and report results"). The callback triggers a new conversation run, which bash sleep cannot do. |
| **Recommendation** | **KEEP all three (deferred)** -- unique capability for deferred execution. Appropriate tier. |

---

## Cut / Keep / Enhance Summary

### KEEP (No Changes Needed) -- 20 tools

| Tool | Tier | Rationale |
|------|------|-----------|
| `read` | Core | Foundational, heavily used |
| `write` | Core | Foundational, version tracking |
| `edit` | Core | Safer than sed for LLM use |
| `bash` | Core | Essential, good safety filtering |
| `job_output` | Core | Required for background jobs |
| `ls` | Core | Heavily used, structured output |
| `grep` | Core | Most-used search tool |
| `git_diff` | Core | Output truncation prevents context overflow |
| `AskUserQuestion` | Core | Unique human-in-the-loop capability |
| `todos` | Core | 80% pass rate when core (vs 0% deferred) |
| `fetch` | Deferred | Structured HTTP fetch |
| `sourcegraph` | Deferred | Cross-repo search (fix wiring bug) |
| `list_mcp_resources` | Deferred | Required for MCP integration |
| `read_mcp_resource` | Deferred | Required for MCP integration |
| `skill` | Deferred | Enables skills system |
| `agent` | Deferred | Sub-agent delegation |
| `web_search` | Deferred | Search capability, no bash equivalent |
| `cron_*` (6 tools) | Deferred | Managed scheduling |
| `delayed_callback_*` (3 tools) | Deferred | Deferred execution |

### CUT (Remove or Merge) -- 7 tools

| Tool | Current Tier | Reason |
|------|-------------|--------|
| `job_kill` | Core -> Deferred | Rarely needed, does not justify core slot |
| `git_status` | Core | Thin wrapper around `git status` via bash; the `clean` boolean is the only added value |
| `lsp_diagnostics` | Deferred | 0% usage, thin wrapper around `gopls check` (just use bash) |
| `lsp_references` | Deferred | 0% usage, thin wrapper around `gopls workspace_symbol` (just use bash/grep) |
| `lsp_restart` | Deferred | No-op placeholder -- does not actually restart anything |
| `agentic_fetch` | Deferred | Thin convenience wrapper, overlaps with fetch + agent reasoning |
| `web_fetch` | Deferred | Duplicates `fetch` tool |

### ENHANCE (Has Value But Needs Work) -- 12 tools

| Tool | Change Needed |
|------|--------------|
| `apply_patch` | Simplify: remove simple find/replace mode (duplicates `edit`), keep only multi-edit and unified patch formats |
| `glob` | Fix `**` recursive pattern support (switch to doublestar library) |
| `observational_memory` | Split complex 6-action tool; add usability testing |
| `todos` | Add partial-update support (update single item by ID) |
| `find_tool` | Strengthen system prompt routing; consider renaming to `discover_tools` |
| `download` | **Promote to core tier** -- 60% discovery rate as deferred, content fabrication is the failure mode |
| `list_models` | **Fix wiring bug** -- add `ModelCatalog` to `DefaultRegistryOptions` |
| `sourcegraph` | **Fix wiring bug** -- add env var support for endpoint/token |
| `cron_pause` + `cron_resume` | Consider merging into `cron_update` |

---

## Gaps -- Missing Capabilities

### High Priority

| Capability | Rationale | Suggested Tool |
|-----------|-----------|----------------|
| **Git commit/add/log** | The LLM frequently needs to commit, stage, and view history. Currently requires bash. `git_status` and `git_diff` exist but without commit/add/log, the git tooling is incomplete. | `git_commit` (core) with message, files params; `git_log` (core) with count, path params |
| **File rename/move** | No tool for moving/renaming files. The LLM must use `bash mv` which bypasses workspace sandboxing. | `mv` or `rename` (core) |
| **File delete** | No tool for deleting files. The LLM must use `bash rm` which bypasses the dangerous-command filter for targeted deletes. | `rm` or `delete` (core) |

### Medium Priority

| Capability | Rationale | Suggested Tool |
|-----------|-----------|----------------|
| **Diff two files** | The LLM can diff staged/unstaged changes via `git_diff`, but cannot diff two arbitrary files in the workspace. | `diff` (core) with path_a, path_b params |
| **JSON/YAML parse** | The LLM frequently needs to read structured config files. `read` returns raw text, requiring the LLM to parse mentally. | `parse_structured` (deferred) that reads a file and returns parsed JSON/YAML |
| **Test runner** | Running `go test` is the most common bash command. A dedicated tool could provide structured test results (pass/fail counts, failure details) instead of raw output. | `test` (deferred) with package, race, verbose params |
| **Environment info** | No tool to inspect the workspace environment (Go version, available commands, env vars). The LLM uses `bash` for this. | `environment` (deferred) reporting Go version, available tools, key env vars |

### Low Priority

| Capability | Rationale | Suggested Tool |
|-----------|-----------|----------------|
| **Image/binary file info** | `read` cannot handle binary files. The LLM has no way to inspect image dimensions, binary file sizes, etc. | `file_info` (deferred) returning MIME type, size, dimensions for images |
| **Clipboard/snippet store** | For multi-step operations, the LLM often needs to hold intermediate results. The todos tool is task-focused, not data-focused. | `scratchpad` (deferred) key-value store for intermediate data |

---

## Tier Reclassification Recommendations

### Promote to Core (from Deferred)

| Tool | Justification |
|------|--------------|
| `download` | Content fabrication is a dangerous failure mode when the LLM cannot find the download tool. 60% discovery rate is too low for a common operation. |

### Demote to Deferred (from Core)

| Tool | Justification |
|------|--------------|
| `job_kill` | Rarely needed. The LLM can use `find_tool` to discover it when a background job needs killing. |

### Remove (Cut Entirely)

| Tool | Justification |
|------|--------------|
| `lsp_diagnostics` | 0% usage, thin wrapper, `bash gopls check` is equivalent |
| `lsp_references` | 0% usage, thin wrapper, `bash gopls workspace_symbol` is equivalent |
| `lsp_restart` | No-op -- does not actually restart anything |
| `agentic_fetch` | Redundant with `fetch` + agent analysis |
| `web_fetch` | Redundant with `fetch` |
| `git_status` | Thin wrapper, `bash git status` is equivalent |

---

## Key Insight: The Deferred Tier Works For Specialized Tools, Fails For Common Ones

The usability data tells a clear story:

- **Deferred works**: `skill` (50% discovery), `list_mcp_resources` (75% discovery), `download` (60% discovery) -- these are specialized enough that users use explicit language that triggers find_tool.
- **Deferred fails**: `lsp_diagnostics` (0%), `lsp_references` (0%), `todos` (0%) -- these compete with strong LLM priors (bash for compilation, grep for search, write for todo files). The LLM never thinks to search for alternatives.
- **Core works**: `todos` went from 0/5 to 4/5 after promotion. This is the strongest evidence that tier placement matters more than system prompt tuning for commonly-needed tools.

**Rule of thumb**: If a tool competes with a bash command the LLM already knows, it must be core-tier to get used. Deferred tier is for capabilities that have no bash equivalent.
