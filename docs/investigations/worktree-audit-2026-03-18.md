# Worktree Audit — 2026-03-18

**Audit performed:** 2026-03-18
**Auditor:** top-level Claude Code session
**Repo:** `/Users/dennisonbertram/Develop/go-agent-harness`
**Remote:** `upstream` → `https://github.com/dennisonbertram/go-agent-harness.git`

---

## Starting State

`git worktree list` returned 70 entries before cleanup. Branches fell into four categories:

| Category | Count | Description |
|---|---|---|
| Detached HEAD (codex) | 6 | `.codex/worktrees` pointing at commit hashes with no branch |
| Named codex/automation branches | 9 | `.codex/worktrees` with named branches, all stale |
| `.claude/worktrees/agent-*` | ~50 | Automation artifacts from swarm sessions |
| Substantive named worktrees | 5 | Potentially real work to evaluate |

---

## Preliminary Action: Fast-Forward Local Main

Local `main` was behind `upstream/main` by 2 commits (PR #339 — "test: add direct context grid coverage (issue #316)" — had already been merged). Fast-forwarded:

```
git merge --ff-only upstream/main
# Updated bb028e7..bb028e7 (PR #339 merge commit)
```

---

## Branch-by-Branch Evaluation

### Branches with NO unmerged commits (fully merged or superseded)

| Branch | Tip Commit | Disposition |
|---|---|---|
| `automation/issue-335-tui-helper-coverage` | 45ab1a7 | Behind main — merged work. Removed. |
| `codex/resolve-one-backlog-issue` | 45ab1a7 | Behind main. Removed. |
| `automation/apply-patch-occurrence-targeting-v2` | badacaf | Far behind main, stale automation. Removed. |
| `automation/apply-patch-occurrence-targeting` | 4b54727 | Far behind main, stale automation. Removed. |
| `automation/issue-14-structured-write-hardening` | 764261d | Far behind main. Removed. |
| `automation/issue-3-per-run-max-steps` | 764261d | Far behind main. Removed. |
| `agent/issue-316` | 0abb46f | Was the subject of PR #339 (merged). Removed. |
| `dennisonbertram/update-docs` | 40a775e | Stale docs branch, behind main. Removed. |
| `agent/issue-327-1773858065-26059` | 80539d2 | Duplicate of #327 work already merged via PR #328. Removed. |

### Branches with UNMERGED commits — real work

#### `agent/issue-317` — `acb345b`: fix(issue-317): implement with TDD and regression tests

Files changed:
- `cmd/harnesscli/tui/components/thinkingbar/model.go` — added `defaultLabel` const + real `View()` logic
- `cmd/harnesscli/tui/components/thinkingbar/model_test.go` — 3 real behavioral tests replacing the stub

**Action:** Cherry-picked to main as `4ee9fa9`.

#### `agent/issue-332` — `bcae146`: Add runner orchestration regression coverage

Files changed:
- `internal/harness/runner.go` — refactored `RunPrompt`/`RunForkedSkill` shared logic into `waitForTerminalResult()`
- `internal/harness/runner_orchestration_test.go` (new) — 7 direct regression tests for orchestration paths

**Action:** Cherry-picked to main as `1c63ac5`. Minor conflicts in `docs/logs/engineering-log.md`, `docs/plans/INDEX.md`, and `docs/plans/active-plan.md` resolved by keeping both sides of the log entries.

#### `issue-316-contextgrid-coverage` — `d08f7a0`: Add context grid regression coverage and narrow-width handling

This branch duplicated work from `agent/issue-316` (which became PR #339, merged). Attempted cherry-pick conflicted on `model_test.go` (add/add conflict) and `model.go` since PR #339 already incorporated the canonical version. The `agent/issue-316` branch (81 lines, internal package) was the one merged by the PR. The `issue-316-contextgrid-coverage` branch (139 lines, `contextgrid_test` external package) is superseded.

**Action:** Abandoned. No cherry-pick. Branch deleted.

#### `automation/issue-18-head-tail-buffer` — branch far behind main

Had 20+ commits in symmetric diff — all older than current main. The head-tail buffer feature is already implemented and merged.

**Action:** Branch deleted, no cherry-pick needed.

---

## Worktree Removals

### Codex Worktrees (`.codex/worktrees/`)

All 15 tracked worktrees (6 detached + 9 named) removed with `git worktree remove --force`:

| Path | Branch/State |
|---|---|
| `.codex/worktrees/349e/go-agent-harness` | detached HEAD 45ab1a7 |
| `.codex/worktrees/3c2c/go-agent-harness` | detached HEAD e13cfad |
| `.codex/worktrees/761c/go-agent-harness` | detached HEAD 45ab1a7 |
| `.codex/worktrees/b9bb/go-agent-harness` | detached HEAD badacaf |
| `.codex/worktrees/d2fd/go-agent-harness` | detached HEAD 7fec813 |
| `.codex/worktrees/f174/go-agent-harness` | detached HEAD 1d48155 |
| `.codex/worktrees/086b/go-agent-harness` | `codex/resolve-one-backlog-issue` |
| `.codex/worktrees/5cb8/go-agent-harness` | `automation/issue-18-head-tail-buffer` |
| `.codex/worktrees/63ba/go-agent-harness` | `agent/issue-332` |
| `.codex/worktrees/675c/go-agent-harness` | `agent/issue-316` (PR #339 was merged) |
| `.codex/worktrees/7bfb/go-agent-harness` | `automation/apply-patch-occurrence-targeting-v2` |
| `.codex/worktrees/aa7d/go-agent-harness` | `issue-316-contextgrid-coverage` |
| `.codex/worktrees/cfcd/go-agent-harness` | `automation/apply-patch-occurrence-targeting` |
| `.codex/worktrees/d0ed/go-agent-harness` | `automation/issue-14-structured-write-hardening` |
| `.codex/worktrees/e395/go-agent-harness` | `automation/issue-3-per-run-max-steps` |

### Tmp Worktrees

| Path | Branch | Notes |
|---|---|---|
| `/private/tmp/go-agent-harness-issue-335` | `automation/issue-335-tui-helper-coverage` | Dirty: unstaged model.go changes + untracked test file. Work is stale vs main. Removed with `--force`. |

### Conductor Worktrees

| Path | Branch | Notes |
|---|---|---|
| `/Users/dennisonbertram/conductor/workspaces/go-agent-harness/vatican` | `dennisonbertram/update-docs` | Clean. Removed. |

### Develop/worktrees

| Path | Branch | Notes |
|---|---|---|
| `/Users/dennisonbertram/Develop/worktrees/317` | `agent/issue-317` | Dirty: untracked transcript exports only. No source changes. Removed with `--force`. |
| `/Users/dennisonbertram/Develop/worktrees/327-1773858065-26059` | `agent/issue-327-1773858065-26059` | Dirty: untracked transcript exports only. Removed with `--force`. |

### .codex-worktrees/94

| Path | Branch | Notes |
|---|---|---|
| `.codex-worktrees/94` | `agent/issue-94` | Dirty: 128-line uncommitted addition to `internal/provider/openai/client_test.go` (cost-clamping test). Issue #94 is CLOSED. Work appears to be unrelated to the closed issue. Discarded and removed with `--force`. |

### .claude/worktrees/agent-* (50+ entries)

All 50+ automation swarm worktrees removed. Dirty state found in many was confined to:
- `training-reports/2026-03-14-verbose.log` (staged artifact, not real work)
- `harness_agent/__pycache__/` (untracked Python bytecode)
- `.harness/cron.db` (untracked runtime database)
- `harnessd` binary (untracked build artifact)
- `code-reviews/` directory (untracked)

Two nested worktrees were removed first before parent directories:
- `.claude/worktrees/agent-ad5cdf9a/.claude/worktrees/agent-a97febef` (`issue-34-retention-policy-impl`)
- `.claude/worktrees/agent-aed9e51e/.claude/worktrees/agent-ae0fed6a` (`worktree-agent-ae0fed6a`)

### .claude/worktrees/ (named)

| Path | Branch |
|---|---|
| `.claude/worktrees/issue-137-models-slash-command` | `issue-137-models-slash-command` |
| `.claude/worktrees/issue-181` | `issue-181-workspace-interface-scaffold` |
| `.claude/worktrees/issue-codex-app-server-provider` | `issue-codex-app-server-provider` |
| `.claude/worktrees/tui-042` | `tui-042-autocomplete` |

---

## Branches Deleted

In addition to the worktree branches above, the following `worktree-agent-*` branches (64 total) were deleted — they are automation artifacts with no real commits on top of main:

```
worktree-agent-a0c30b27 through worktree-agent-afdbcc08 (64 branches)
```

---

## What Was Merged to Main

| Commit | Message | Source Branch |
|---|---|---|
| `4ee9fa9` | fix(issue-317): implement with TDD and regression tests | `agent/issue-317` |
| `1c63ac5` | Add runner orchestration regression coverage | `agent/issue-332` |

Both commits pushed to `upstream/main` (via `git push upstream HEAD:main`).

---

## Final State

```
$ git worktree list
/Users/dennisonbertram/Develop/go-agent-harness  1c63ac5 [main]
```

Only the primary working tree remains. All 69 non-main worktrees have been removed. Stale metadata cleared with `git worktree prune`.

---

## Notes on Remaining Branches (not cleaned)

The following local branches were intentionally left alone — they are historical feature/issue branches not associated with the worktrees being audited:

- `backup/local-artifacts-snapshot-20260317`, `backup/local-docs-snapshot-20260317`
- `codex/provider-model-impact-map-guardrail-20260318`, `codex/review-3`, `codex/review-item2`
- `issue-{1..n}-*` feature branches from prior milestones
- `issue-23-research-os-sandboxing-seatbelt-landlock` (has 1 unmerged research doc commit)
- `feat/forensics-*`, `cronsd-phase1`, `feature/force-non-streaming-gemini`

These total ~78 branches and represent historical work or in-progress features not yet PRed.
