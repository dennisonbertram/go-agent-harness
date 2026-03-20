# Issue #26 Research: Deep Git Tools for Repo-Wide Historical Understanding

**Date**: 2026-03-18
**Issue**: [#26 — Research: Deep git tools for repo-wide historical understanding](https://github.com/dennisonbertram/go-agent-harness/issues/26)
**Status**: OPEN (labels: large, needs-clarification, research)

---

## Executive Summary

The harness currently exposes only two git tools: `git_status` (working-tree state) and `git_diff` (diff content). Both are shallow — they answer "what is different right now?" but provide no access to commit history, authorship reasoning, or co-change patterns. Issue #26 calls for a "deep git toolset" enabling agents to build repo-wide historical understanding. This report inventories what exists, proposes a concrete minimum useful tool set with signatures, evaluates implementation approaches, estimates per-tool complexity, and recommends a phased rollout.

---

## 1. What Already Exists

### Core Git Tools (always loaded — `TierCore`)

Both tools live in `internal/harness/tools/core/git.go` (canonical `core/` layer) and are also duplicated in the legacy `internal/harness/tools/git_diff.go` / `git_status.go`. Both variants are wired into `BuildCatalog` in `internal/harness/tools/catalog.go`.

| Tool | File | What it does |
|------|------|--------------|
| `git_status` | `core/git.go`, `git_status.go` | Runs `git status --porcelain=v1`. Returns `clean`, `output`, `exit_code`, `timed_out`. |
| `git_diff` | `core/git.go`, `git_diff.go` | Runs `git diff [--staged] [target] [-- path]`. Returns unified diff, truncated at 256 KB by default. Supports revision ranges (`HEAD~3`, `main...feature`). |

### What the existing tools cannot do

- Access commit history (messages, authors, dates, hashes)
- Perform keyword or semantic search over commits
- Show blame for a file or line range
- Trace how a file/function changed across multiple commits
- Show diff between two arbitrary refs beyond what `git diff target` allows
- Search git's object store for strings in historical content
- Show commit co-occurrence patterns across files

### No deferred git tools exist

A search of `internal/harness/tools/deferred/` and all `*.go` files in `internal/harness/tools/` confirms zero deferred git tools. The issue notes from the prior grooming session (`docs/investigations/issue-26-grooming.md`) confirm: "None of the six proposed tools exist anywhere in the codebase."

---

## 2. What Do Competitors Do?

From `docs/research/harness-comparison-synthesis.md` and `docs/research/pi-review.md`:

- **Codex, OpenCode, Crush**: No deep git tools. Git is accessible only via the bash tool.
- **oh-my-pi (omp) fork**: Three tools — `git-overview` (repo summary), `git-file-diff` (diff for a specific commit/file), `git-hunk` (patch-level commit generation). These are focused on intelligent commit generation rather than historical archaeology.
- **None** of the competitors build a persistent, queryable model of why the repository evolved.

This confirms the differentiation opportunity stated in the issue.

---

## 3. go-git vs Shell Exec — The Implementation Tradeoff

### go-git (`github.com/go-git/go-git/v5`)

**Pros:**
- Pure Go, no external binary dependency
- Programmatic access to git objects (commits, trees, blobs, refs)
- Can read pack files, walk commit DAG, parse blame results, read file history — all in-process
- Testable without a real git installation
- Structured data access (commit metadata as typed Go structs)

**Cons:**
- Not in `go.mod` today — adding it pulls a moderate dependency tree
- Notably **does not support `git worktree` commands** (confirmed by the workspace implementation notes in MEMORY.md — we already hit this and fell back to shell exec)
- Performance on large repos is adequate but not always faster than CLI (go-git's pack reading is slower than C git for cold paths)
- Some advanced operations (e.g., `git log -S`, `git log --follow`, `git log --all`) have no go-git equivalent or are significantly more complex to implement than wrapping the CLI flag

### Shell Exec (`git` CLI via `runCommand`)

**Pros:**
- Already used by all existing git tools — the pattern is established and tested
- Gives access to every git feature without reimplementation overhead
- The `git log` porcelain (`--format="%H%n%ae%n%s%n%b"`) and `git log -S` (pickaxe) are mature, fast, and predictable
- `git blame --porcelain` is the canonical blame output format
- `git log --follow` for file renames is handled by git itself
- No new dependency
- `git log --all -S <string>` (pickaxe keyword search) is the fastest possible approach for keyword commit search without an index

**Cons:**
- Output parsing requires careful format strings and sanitization
- Git must be installed in the runtime environment (already required for existing tools)
- Less testable in pure unit tests (tests need a real git repo fixture)

### Verdict: Shell Exec for All Six Tools

The existing codebase uses shell exec everywhere for git. Adding go-git would introduce a new dependency solely for deep git tools, where the shell exec approach already works well. The one place go-git might add value is for `git_change_patterns` (co-change analysis), where iterating the full commit DAG in Go is cleaner than parsing thousands of lines of `git log` output — but even there, `git log --name-only --pretty=format:"%H"` piped through parsing is manageable.

**Decision**: Implement all new tools as shell exec wrappers using the existing `runCommand` pattern. Go-git can be reconsidered in a Phase 2 if structured DAG traversal becomes a bottleneck.

---

## 4. Keyword Search vs Embeddings for git_log_search

This is the most architecturally consequential decision.

### Option A: Keyword/Regex Search (git pickaxe + message grep)

**Mechanism**: `git log --all -S <string>` (finds commits where the string was added/removed in diff content), combined with `git log --all --grep=<pattern>` (searches commit messages). Both are built into git, fast, and require no external services.

- `git log -S "auth flow" --pretty=format:"%H|%ae|%ai|%s"` — finds all commits where "auth flow" appeared/disappeared in diffs
- `git log --grep="authentication" --pretty=format:"%H|%ae|%ai|%s"` — finds commits with "authentication" in message

**Pros**: Zero additional dependencies. Works offline. Fast on repos up to 50k commits (under 2 seconds on typical hardware). Deterministic and reproducible.

**Cons**: Literal string matching only. A query like "when did we change the auth flow?" requires the caller (or the LLM using the tool) to extract the keyword. Cannot rank by semantic relevance — all matches are equally ranked (only sorted by date).

**Feasibility**: High. Implementable in ~2 hours. No external deps.

### Option B: Embedding-based Semantic Search

**Mechanism**: Embed all commit messages (and optionally diffs) into a vector space. At query time, embed the query, find nearest neighbors.

- Requires: embedding model (OpenAI `text-embedding-3-small`, or local Ollama model)
- Requires: vector store (in-memory HNSW, or SQLite with vector extension, or a sidecar like ChromaDB)
- Requires: indexing pipeline run before first search (or incrementally on new commits)

**Pros**: True semantic matching. "when did we change the auth flow?" would surface commits about JWT refresh, login redirects, session management without needing the exact words.

**Cons**:
- Requires embedding API calls or a local model — adds latency and cost
- Requires an indexing step before first use (could be seconds to hours depending on repo size)
- Vector store is a new dependency not in `go.mod`
- Increases operational complexity: agents must wait for index build, index must be invalidated on new commits
- On a 10k-commit repo, embedding all messages costs roughly $0.01 (OpenAI pricing), but this is a recurring cost whenever the index is stale
- The harness already uses `modernc.org/sqlite` — a sqlite-vec extension could store embeddings, but this adds significant implementation surface

**Feasibility**: Medium. Full implementation is a sprint (3–5 days). Prototype is achievable in a day if OpenAI embeddings are used, but requires an API key and adds per-use cost.

### Recommendation: Start with Keyword, Design for Semantic Upgrade

The keyword/pickaxe approach provides **immediate, practical value** for 80% of real-world queries (developers searching for "when did we add X" or "why was Y changed"). The tool description can explicitly note that it accepts literal strings and instruct the LLM to extract key terms from natural language queries. The tool signature can accept a `mode` field (`"pickaxe"` for diff search, `"message"` for commit message grep, `"both"` as default) to avoid two separate tools.

Semantic search can be layered on in Phase 2 as a separate `git_log_semantic_search` tool, gated on `OPENAI_EMBEDDING_KEY` being set and an index being built.

---

## 5. GitHub API: Needed or Optional for git_blame_context?

The issue proposes that `git_blame_context` link commits to PRs and issues. This requires the GitHub API.

**What local git provides without GitHub API:**
- Blame output: which commit touched each line, when, by whom
- The commit message for each blame commit
- The commit hash (which can be cross-referenced manually against PR merge commits)

**What the GitHub API adds:**
- Which PR number merged this commit
- PR description, labels, review discussion
- Linked issue numbers from commit/PR body
- CI status at the time of merge

**Practical assessment**: For the stated goal of "explaining why changes were made," the commit message alone is often sufficient. A well-written commit message on a mature project contains the "why." The GitHub API link adds significant value when commit messages are poor (common in practice), but requires:
- `GITHUB_TOKEN` env var
- Parsing the remote URL to extract `owner/repo`
- Rate limiting handling (60 req/hr unauthenticated, 5000/hr authenticated)
- GitHub-specific coupling (breaks on GitLab, Bitbucket, self-hosted)

**Recommendation**: Implement `git_blame_context` using local git only for the initial version. The tool should surface commit messages and author/date metadata alongside blame output. Add an optional `github_token` parameter that, when provided, attempts GitHub API enrichment. Document clearly that this is optional enrichment, not required for core function. This keeps the tool useful in all environments while allowing richer output when GitHub access is available.

---

## 6. Recommended Tool Set with Signatures

All proposed tools should be `TierDeferred` — loaded on demand via `find_tool`. They are specialized archaeology tools, not every-run utilities. The `Tags` field enables discovery.

### 6.1 `git_log_search` — Keyword search over commit history

**Purpose**: Find commits by searching messages and/or diff content. The LLM asks "when did we change X" or "why was Y removed" and the tool finds matching commits.

```go
Parameters: map[string]any{
    "type": "object",
    "properties": map[string]any{
        "query":      map[string]any{"type": "string", "description": "Search term (literal string)"},
        "mode":       map[string]any{"type": "string", "enum": []string{"message", "pickaxe", "both"}, "default": "both"},
        "path":       map[string]any{"type": "string", "description": "Limit search to this file or directory (optional)"},
        "max_results": map[string]any{"type": "integer", "minimum": 1, "maximum": 100, "default": 20},
        "since":      map[string]any{"type": "string", "description": "Limit to commits after this date (YYYY-MM-DD) or ref"},
        "until":      map[string]any{"type": "string", "description": "Limit to commits before this date or ref"},
        "author":     map[string]any{"type": "string", "description": "Filter by author email or name pattern"},
    },
    "required": []string{"query"},
}

// Output:
{
    "commits": [
        {
            "hash": "abc123",
            "short_hash": "abc1234",
            "author_name": "Alice",
            "author_email": "alice@example.com",
            "date": "2025-11-15T14:23:00Z",
            "subject": "fix: handle auth token expiry",
            "body": "...",
            "match_type": "pickaxe"  // "message" or "pickaxe"
        }
    ],
    "total_found": 7,
    "truncated": false,
    "query": "auth token",
    "mode": "both"
}
```

**Implementation**: Two `git log` invocations (unless mode is specified):
1. `git -C <root> log --all --grep=<query> --pretty=format:"<format>" -- [path]`
2. `git -C <root> log --all -S <query> --pretty=format:"<format>" -- [path]`

Merge and deduplicate by hash. Sort by date descending. Truncate to `max_results`.

**Complexity**: Low. 1–2 days including description writing and tests.

---

### 6.2 `git_blame_context` — Per-line authorship with commit context

**Purpose**: For a file and line range, show who changed each line and why (commit message + optional PR context). Primary use case: "why does this function do X? who wrote it?"

```go
Parameters: map[string]any{
    "type": "object",
    "properties": map[string]any{
        "path":          map[string]any{"type": "string", "description": "File path relative to workspace root"},
        "start_line":    map[string]any{"type": "integer", "minimum": 1},
        "end_line":      map[string]any{"type": "integer", "minimum": 1},
        "rev":           map[string]any{"type": "string", "description": "Revision to blame at (default: HEAD)"},
        "github_token":  map[string]any{"type": "string", "description": "Optional: GitHub token for PR/issue enrichment"},
    },
    "required": []string{"path"},
}

// Output:
{
    "file": "internal/auth/handler.go",
    "rev": "HEAD",
    "lines": [
        {
            "line_number": 42,
            "content": "    return token.Validate(ctx)",
            "commit_hash": "def456",
            "author_name": "Bob",
            "author_email": "bob@example.com",
            "date": "2025-09-03T09:11:00Z",
            "commit_subject": "refactor: extract token validation to shared helper",
            "commit_body": "Moved validation logic to avoid duplication with..."
        }
    ],
    "unique_commits": 3,
    "github_enrichment": false
}
```

**Implementation**: `git -C <root> blame --porcelain [-L <start>,<end>] [<rev>] -- <path>`. Parse the porcelain blame format (well-documented, stable). For each unique commit hash seen, run `git show --format="%H|%aN|%aE|%aI|%s|%b" --no-patch <hash>` to get full commit context.

If `github_token` is provided, attempt to call `GET /repos/{owner}/{repo}/commits/{sha}/pulls` to find the associated PR. Parse remote URL from `git remote get-url origin`.

**Complexity**: Medium. 2–3 days. Porcelain blame parsing is straightforward but slightly fiddly; the GitHub enrichment path adds another day if included.

---

### 6.3 `git_file_history` — Timeline of changes to a file or function

**Purpose**: Show how a file evolved over time as a commit timeline. More focused than `git_log_search` — specifically scoped to a path, with rename following and optional diff summaries.

```go
Parameters: map[string]any{
    "type": "object",
    "properties": map[string]any{
        "path":         map[string]any{"type": "string", "description": "File path (or directory) to trace"},
        "max_commits":  map[string]any{"type": "integer", "minimum": 1, "maximum": 200, "default": 50},
        "follow":       map[string]any{"type": "boolean", "default": true, "description": "Follow file renames"},
        "show_diffs":   map[string]any{"type": "boolean", "default": false, "description": "Include diff for each commit (increases output size)"},
        "diff_max_bytes": map[string]any{"type": "integer", "default": 2048, "description": "Max bytes per diff when show_diffs is true"},
        "since":        map[string]any{"type": "string"},
        "until":        map[string]any{"type": "string"},
    },
    "required": []string{"path"},
}

// Output:
{
    "file": "internal/auth/handler.go",
    "follow": true,
    "commits": [
        {
            "hash": "abc123",
            "date": "2025-11-15T14:23:00Z",
            "author_name": "Alice",
            "subject": "fix: handle auth token expiry",
            "diff": "..."   // only if show_diffs=true
        }
    ],
    "total_commits": 23,
    "truncated": false
}
```

**Implementation**: `git -C <root> log [--follow] [--since] [--until] -n <max_commits> --pretty=format:"<format>" -- <path>`. When `show_diffs=true`, add `-p` and `--unified=3`, then truncate each diff hunk at `diff_max_bytes`. The `--follow` flag handles rename tracking (a key feature — without it, history before a rename is invisible).

**Complexity**: Low-Medium. 1–2 days. The diff-per-commit path requires careful truncation logic but the basic form is simple.

---

### 6.4 `git_diff_range` — Diff between two arbitrary refs

**Purpose**: Show the diff between two commits, branches, or tags. Essentially a more ergonomic version of `git_diff`'s `target` parameter, with explicit `from` and `to`, stat summary, and per-file filtering.

This is a companion to the existing `git_diff` rather than a replacement. `git_diff` focuses on the working tree; `git_diff_range` focuses on historical commit-to-commit comparison.

```go
Parameters: map[string]any{
    "type": "object",
    "properties": map[string]any{
        "from":          map[string]any{"type": "string", "description": "Base ref (commit hash, branch, tag)"},
        "to":            map[string]any{"type": "string", "description": "Target ref (default: HEAD)"},
        "path":          map[string]any{"type": "string", "description": "Limit to this file or directory"},
        "stat_only":     map[string]any{"type": "boolean", "default": false, "description": "Return only file change counts, not full diff"},
        "max_bytes":     map[string]any{"type": "integer", "default": 262144},
        "context_lines": map[string]any{"type": "integer", "default": 3, "minimum": 0, "maximum": 20},
    },
    "required": []string{"from"},
}

// Output:
{
    "from": "abc123",
    "to": "HEAD",
    "diff": "...",           // empty if stat_only=true
    "stat": "...",           // always present: files changed, insertions, deletions
    "files_changed": 5,
    "insertions": 120,
    "deletions": 45,
    "truncated": false
}
```

**Implementation**: Two `git` calls:
1. `git -C <root> diff --stat <from>..<to> [-- path]` — always run for stat
2. `git -C <root> diff --unified=<context_lines> <from>..<to> [-- path]` — run unless `stat_only`

**Complexity**: Low. 1 day. Very similar to the existing `git_diff` tool.

---

### 6.5 `git_grep_history` — Search file content across all commits

**Purpose**: Find when a specific string (function name, variable, constant, error message) first appeared, last appeared, or appears in a specific historical commit. This is the "code archaeology" version of grep — search the entire history, not just HEAD.

This is distinct from `git_log_search`: `git_log_search` finds commits by their message or diff metadata; `git_grep_history` finds content within the state of the repo at each commit.

```go
Parameters: map[string]any{
    "type": "object",
    "properties": map[string]any{
        "pattern":    map[string]any{"type": "string", "description": "Regex or literal pattern to search for"},
        "rev":        map[string]any{"type": "string", "description": "Specific ref to search at (default: HEAD)"},
        "all_refs":   map[string]any{"type": "boolean", "default": false, "description": "Search across all branches/tags (slower)"},
        "path":       map[string]any{"type": "string", "description": "Limit to this file or directory"},
        "ignore_case": map[string]any{"type": "boolean", "default": false},
        "max_results": map[string]any{"type": "integer", "default": 50, "maximum": 500},
    },
    "required": []string{"pattern"},
}

// Output:
{
    "pattern": "ErrTokenExpired",
    "rev": "HEAD",
    "matches": [
        {
            "ref": "HEAD",
            "file": "internal/auth/errors.go",
            "line_number": 14,
            "content": "var ErrTokenExpired = errors.New(\"token expired\")"
        }
    ],
    "total_matches": 3,
    "truncated": false
}
```

**Implementation**: `git -C <root> grep [-i] [--line-number] <pattern> <rev> [-- path]`. When `all_refs=true`, enumerate refs with `git for-each-ref --format="%(refname:short)"` and run grep against each (deduplicating). Enforce max_results and a time limit (60 seconds) since `all_refs=true` on a large repo can be slow.

**Complexity**: Low. 1 day. `git grep` is purpose-built for this.

---

### 6.6 `git_contributor_context` — Who knows what in the codebase

**Purpose**: For a file or directory, identify the primary contributors (by commit count and lines changed) and show their recent activity. Answers "who should I ask about this code?"

```go
Parameters: map[string]any{
    "type": "object",
    "properties": map[string]any{
        "path":        map[string]any{"type": "string", "description": "File or directory path"},
        "max_authors": map[string]any{"type": "integer", "default": 5, "maximum": 20},
        "since":       map[string]any{"type": "string", "description": "Limit to commits since this date"},
        "show_recent": map[string]any{"type": "boolean", "default": true, "description": "Include 3 most recent commits per author"},
    },
    "required": []string{"path"},
}

// Output:
{
    "path": "internal/auth/",
    "authors": [
        {
            "name": "Alice",
            "email": "alice@example.com",
            "commit_count": 34,
            "first_commit": "2025-01-10T...",
            "last_commit": "2025-11-15T...",
            "recent_commits": [
                {"hash": "abc123", "date": "2025-11-15T...", "subject": "fix: handle auth token expiry"}
            ]
        }
    ]
}
```

**Implementation**: `git -C <root> log [--since] --pretty=format:"%aN|%aE|%H|%aI|%s" -- <path>`. Parse lines, group by author email (not name — names drift), count commits per author, sort by count descending. The `show_recent` field just keeps the top 3 most recent commits per author from the same parsed output.

**Complexity**: Low. 1 day. Pure log parsing — no special git commands.

---

### What Is NOT Recommended for Phase 1

**`git_change_patterns` (co-change analysis)**: Identifying which files change together requires either:
- Iterating every commit and building a co-occurrence matrix (O(C × F²) where C=commits and F=files per commit), or
- Using `git log --name-only` and doing the co-occurrence calculation in Go

On a 10k-commit repo with 500 files, this could produce a 500×500 matrix with 10k iterations. It is feasible (go-git would shine here for structured DAG traversal), but it is a significant chunk of work and the output is hard to token-budget (which pairs do you show?). This belongs in Phase 2.

**`git_regression_detect` (semantic git bisect)**: The issue describes "semantic git bisect" but does not specify how "semantics" are determined. Any meaningful implementation requires LLM calls at each bisect step, making it a multi-step agentic operation, not a single tool call. This is a skill or a recipe, not a tool. Deferred indefinitely.

---

## 7. Implementation Approach

### File Structure

New deep git tools should live in `internal/harness/tools/deferred/` as a new file `git_deep.go` (following the convention of grouping related deferred tools in one file). Descriptions go in `internal/harness/tools/descriptions/` with one `.md` per tool.

```
internal/harness/tools/deferred/git_deep.go        # all 5 tools
internal/harness/tools/descriptions/git_log_search.md
internal/harness/tools/descriptions/git_blame_context.md
internal/harness/tools/descriptions/git_file_history.md
internal/harness/tools/descriptions/git_diff_range.md
internal/harness/tools/descriptions/git_grep_history.md
internal/harness/tools/descriptions/git_contributor_context.md
```

### Wiring into Catalog

Each tool function returns a `tools.Tool` with `Tier: tools.TierDeferred`. They need an `EnableDeepGit bool` field in `BuildOptions` and a corresponding block in `BuildCatalog`:

```go
if opts.EnableDeepGit {
    tools = append(tools,
        deferred.GitLogSearchTool(opts),
        deferred.GitBlameContextTool(opts),
        deferred.GitFileHistoryTool(opts),
        deferred.GitDiffRangeTool(opts),
        deferred.GitGrepHistoryTool(opts),
        deferred.GitContributorContextTool(opts),
    )
}
```

### Output Formatting

All tools should return structured JSON (via `tools.MarshalToolResult`). Token efficiency is critical: the output fields for commit history should include only the fields an LLM needs to reason about changes (hash, short_hash, date, author_name, subject, body). Full diffs should be opt-in and bounded.

### Timeouts

| Tool | Suggested Timeout |
|------|-------------------|
| `git_log_search` | 30s |
| `git_blame_context` | 20s |
| `git_file_history` | 30s |
| `git_diff_range` | 30s |
| `git_grep_history` | 60s (all_refs can be slow) |
| `git_contributor_context` | 30s |

### Path Validation

All path parameters must go through `tools.ResolveWorkspacePath` + `tools.NormalizeRelPath` (the established pattern in `git_diff` and `git_status`) to enforce workspace confinement.

---

## 8. Performance Analysis

### Realistic Repo Sizes

| Repo Size | Commits | `git_log_search` | `git_blame_context` | `git_file_history` | `git_grep_history` |
|-----------|---------|-----------------|--------------------|--------------------|-------------------|
| Small (<1k commits) | ~500 | <100ms | <200ms | <100ms | <100ms |
| Medium (1k–10k) | ~5k | 200–500ms | 300ms | 300ms | 500ms |
| Large (10k–100k) | ~50k | 1–3s | 500ms | 500ms | 2–5s |
| Very large (100k+) | ~500k (Linux) | 5–15s | 1s | 1s | 10–30s (all_refs) |

Key insight: `git blame` and `git file history --no-diffs` are fast at any size because they operate on a specific path. `git log -S <string>` (pickaxe) and `git grep --all-refs` are the slow paths for large repos.

**Mitigations:**
- Enforce 30–60s timeouts (already done via `runCommand`)
- Default `max_results` to 20–50 (limits output even if git runs fast)
- `git_grep_history` should default `all_refs=false` with a clear warning that `true` is slow on large repos
- Consider a `repo_size_hint` parameter in Phase 2 to auto-tune behavior

### Token Efficiency

The main risk is dumping too much log output into the LLM context. Mitigations:
- Default `show_diffs=false` for `git_file_history` (users opt in to diffs)
- Return structured JSON so LLMs parse structured fields, not freeform text
- `body` field in commit objects should be truncated at 500 bytes by default
- `stat_only` option in `git_diff_range` for overview-before-deep-dive workflow

---

## 9. Memory Integration

The issue proposes that git tool outputs feed into the observational memory system. The recommended approach:

**Short term (Phase 1)**: Tools return structured output. The LLM agent decides what to write to `observational_memory`. No automatic injection. This is consistent with how the harness currently treats all tool results — the LLM observes and decides what to remember.

**Long term (Phase 2)**: A `git_historian` persona (via YAML intent config) equipped with these tools runs proactively when an agent touches a file for the first time, stores a "file history summary" observation, and incrementally updates it. This requires:
- A `repo_level` memory scope in the observational memory system (currently scoped per conversation/agent — see issue #17)
- A structured memory format for git observations (architectural decisions, coupling patterns, key authors)
- A proactive trigger mechanism (hook into the `read` tool to fire git history fetch on first encounter of a file — or a scheduled cron-based historian run)

This memory integration work should be tracked as a separate issue after the tool implementations.

---

## 10. Complexity Estimates

| Tool | Complexity | Estimate | Notes |
|------|-----------|----------|-------|
| `git_log_search` | Low | 1–2 days | Two `git log` calls + dedup + format |
| `git_blame_context` | Medium | 2–3 days | Porcelain blame parsing is fiddly; GitHub enrichment +1 day optional |
| `git_file_history` | Low | 1–2 days | Straightforward `git log --follow` wrapper |
| `git_diff_range` | Low | 1 day | Nearly identical to existing `git_diff` |
| `git_grep_history` | Low | 1 day | `git grep` wrapper with ref enumeration |
| `git_contributor_context` | Low | 1 day | Log parsing + author grouping |
| Descriptions (6 .md files) | Low | 0.5 day | |
| Tests (fixtures + unit) | Medium | 1–2 days | Needs git repo test fixtures |
| Wiring + catalog integration | Low | 0.5 day | |
| **Total Phase 1** | | **9–13 days** | |

---

## 11. Recommended Phasing

### Phase 1 (Issue #26 scope): Core Historical Tools

Implement the five high-value, low-complexity tools:
1. `git_log_search` — keyword commit search (pickaxe + message grep)
2. `git_file_history` — commit timeline for a file with rename following
3. `git_blame_context` — per-line authorship with commit messages (local only)
4. `git_diff_range` — diff between two arbitrary refs
5. `git_contributor_context` — top authors for a path

All as `TierDeferred`. Gate with `EnableDeepGit` in `BuildOptions`.

Skip for Phase 1: `git_grep_history` (useful but lower priority), `git_change_patterns` (high complexity), `git_regression_detect` (not a single-tool problem).

### Phase 2: Advanced Tools + Memory Integration

6. `git_grep_history` — search content across historical refs
7. `git_change_patterns` — co-change coupling matrix (requires go-git for efficiency)
8. Memory integration: `repo_level` scope + `git_historian` persona config
9. Optional: semantic embedding search for `git_log_search` (gated on embedding key)

### Phase 3: Persona Package

- `git_historian` YAML intent definition
- Talent definition: `code_archaeology`
- Proactive context loading (hook into `read` tool to auto-surface file history)
- Cross-conversation memory consolidation for git insights

---

## 12. Tool Naming Convention Clarification

The issue uses `git_evolution` and `git_regression_detect` as names. This research recommends renaming for clarity:

| Issue #26 Name | Recommended Name | Reason |
|----------------|-----------------|--------|
| `git_log_search` | `git_log_search` | Keep — clear |
| `git_blame_context` | `git_blame_context` | Keep — clear |
| `git_evolution` | `git_file_history` | "Evolution" is vague; "file history" is explicit about what it does |
| `git_regression_detect` | (Phase 2 skill, not tool) | Not implementable as a single tool call |
| `git_contributor_context` | `git_contributor_context` | Keep |
| `git_change_patterns` | (Phase 2) | High complexity, defer |

A new tool not in the original issue: `git_diff_range` — fills a clear gap between working-tree `git_diff` and historical commits.

---

## Summary of Recommendations

1. **Use shell exec for all tools** — no go-git dependency needed. The existing `runCommand` pattern handles everything.

2. **Keyword search only for Phase 1** — `git log -S` (pickaxe) + `git log --grep` covers 80% of real use cases without embedding infrastructure.

3. **GitHub API is optional enrichment, not required** — `git_blame_context` works locally; GitHub token enables PR/issue linking.

4. **Five tools in Phase 1** — `git_log_search`, `git_file_history`, `git_blame_context`, `git_diff_range`, `git_contributor_context`. Skip complex tools (`git_change_patterns`, `git_regression_detect`) until Phase 2.

5. **All tools as `TierDeferred`** — gate with `EnableDeepGit` in `BuildOptions`. Agents load them via `find_tool` when doing historical analysis.

6. **Memory integration is a separate issue** — the tools themselves are useful standalone. Proactive git historian persona requires repo-level memory scope (issue #17 work) as a prerequisite.

7. **Split the original issue** — as the grooming note recommended, this should become 3 GitHub issues: (a) implement 5 core historical tools [Phase 1], (b) implement co-change patterns + semantic search [Phase 2], (c) design and implement git historian persona + memory integration [Phase 3].
