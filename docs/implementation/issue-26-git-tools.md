# Issue #26: Phase 1 Deep Git Tools

**Date**: 2026-03-18
**Issue**: [#26 — Research: Deep git tools for repo-wide historical understanding](https://github.com/dennisonbertram/go-agent-harness/issues/26)
**Status**: COMPLETE (Phase 1)

---

## What Was Built

Five new `TierDeferred` git history tools implemented in `internal/harness/tools/deferred/git_deep.go`:

### 1. `git_log_search`
Searches commit history by keyword in commit messages (`git log --grep`) and/or diff content (`git log -S` pickaxe). Supports `mode` parameter (`message`, `pickaxe`, `both`), optional `path` scoping, `max_results`, and `since` date filter. Returns structured JSON with commit metadata.

### 2. `git_file_history`
Shows the commit timeline for a specific file or directory. Supports rename following (`--follow`), optional diff inclusion per commit, `max_commits` limit, and `since` filter. Returns commit array with hash, author, date, subject, and optional diff.

### 3. `git_blame_context`
Per-line blame with full commit context. Uses `git blame --porcelain` to parse blame output, then runs `git show --format="%s\x1F%b"` for each unique commit hash to enrich lines with commit messages. Supports line ranges (`start_line`, `end_line`) and revision selection.

### 4. `git_diff_range`
Diff between two arbitrary refs. Runs `git diff --stat` (always) and `git diff` (unless `stat_only=true`). Parses file count, insertion, and deletion counts from the stat summary line. Supports `max_bytes` truncation and optional `path` scoping.

### 5. `git_contributor_context`
Top contributors for a file, directory, or entire repo. Runs `git log --pretty=format:"%aN\x1F%aE"`, groups by email address, counts commits, sorts descending. Supports `max_authors` and `since` filter.

---

## Implementation Details

### Files Created

| File | Description |
|------|-------------|
| `internal/harness/tools/deferred/git_deep.go` | All five tool constructors + helper functions |
| `internal/harness/tools/deferred/git_deep_test.go` | 32 tests using real temp git repos |
| `internal/harness/tools/descriptions/git_log_search.md` | Tool description for LLM |
| `internal/harness/tools/descriptions/git_file_history.md` | Tool description for LLM |
| `internal/harness/tools/descriptions/git_blame_context.md` | Tool description for LLM |
| `internal/harness/tools/descriptions/git_diff_range.md` | Tool description for LLM |
| `internal/harness/tools/descriptions/git_contributor_context.md` | Tool description for LLM |
| `docs/implementation/issue-26-git-tools.md` | This file |

### Files Modified

| File | Change |
|------|--------|
| `internal/harness/tools/types.go` | Added `EnableDeepGit` to `BuildOptions`; added `ForkDepthFromContext`, `WithForkDepth`, `DefaultMaxForkDepth` (needed for co-existing `spawn_agent.go` untracked file) |
| `internal/harness/tools_default.go` | Wired all five tools into `NewDefaultRegistryWithOptions` as always-registered deferred tools |
| `internal/harness/tools/descriptions/embed_test.go` | Added new description names to `TestLoadAllKnownDescriptions` slice and `TestEmbeddedFSAndKnownListAreInSync` map |

### Key Implementation Patterns

- **Shell exec only**: All tools use `tools.RunCommand(ctx, timeout, "git", args...)` — no go-git dependency
- **Record separator parsing**: git log output uses `--pretty=format:%x1E%H%x1F%h%x1F...%x1E` (record separator U+001E, field separator U+001F) for reliable multi-field parsing
- **Porcelain blame format**: `git blame --porcelain` provides stable, machine-readable output; parsed line-by-line with commit metadata accumulation
- **Commit enrichment**: `git_blame_context` runs `git show --format="%s\x1F%b" --no-patch <hash>` per unique commit to add commit messages to blame output
- **Token efficiency**: Body fields truncated at 500 bytes; diffs truncated at `max_bytes` or 4096 bytes per commit for `show_diffs=true`
- **Path validation**: All path parameters go through `tools.ResolveWorkspacePath` + `tools.NormalizeRelPath` for workspace confinement

### Wiring

All five tools are always registered in `NewDefaultRegistryWithOptions` as `TierDeferred`. Since git is already required by the existing `git_status` and `git_diff` core tools, there is no need for an additional `EnableDeepGit` guard in production. The `EnableDeepGit` field was added to `BuildOptions` for completeness/future use by the legacy `BuildCatalog` path.

Agents discover and activate these tools via `find_tool` with queries like `find_tool("git history")` or `find_tool("select:git_blame_context")`.

---

## Test Coverage

32 tests across all five tools:

- Definition tests (name, tier, tags)
- Required parameter validation
- Success path tests with real temp git repos
- Mode/option variant tests (pickaxe, message, both; stat_only; show_diffs; line ranges)
- max_results/max_commits/max_authors truncation
- InvalidJSON error handling
- Edge cases (no results, path-scoped results, default parameter values)

Test fixture: `initTestRepo()` creates a 4-commit repo with 2 authors (Alice: 3 commits, Bob: 1 commit) and a subdirectory, suitable for exercising all tools.

---

## What Was NOT Built (Phase 2+)

- `git_grep_history` — `git grep` across historical refs (lower priority for Phase 1)
- `git_change_patterns` — co-change analysis (high complexity, deferred)
- `git_regression_detect` — semantic git bisect (requires multi-step agentic operation)
- Semantic/embedding-based search (requires embedding API + index infrastructure)
- GitHub API enrichment for `git_blame_context` (optional `github_token` enrichment)
- `git_historian` persona configuration (depends on repo-level memory scope from issue #17)

---

## Performance Notes

- All tools use 20–30 second timeouts via `tools.RunCommand`
- `git_log_search` pickaxe mode (`git log -S`) can be slow on large repos; default `max_results=20` bounds output
- `git_blame_context` fetches commit messages per unique hash — up to N+1 git invocations for N unique commits in a range; bounded by line range selection
- `git_diff_range` always runs `--stat` first, then full diff unless `stat_only=true`
