# Grooming Summary — 2026-03-18

## Overview

This document consolidates the findings from a grooming pass over open GitHub issues on 2026-03-18. Issues are grouped by disposition: already resolved, needs a PR, ready for implementation, and needs clarification.

**Token limitation note:** The GitHub personal access token in use returns 403 for write operations (`addComment`, `addLabelsToLabelable`, `closeIssue`). All label applications and issue closures listed below must be performed manually by the user.

---

## Phase 1: Already Resolved — Close These Issues

These issues have been fully implemented and all acceptance criteria are met. They should be closed manually.

### #281 — TUI-029: /clear command

- **Status:** Fully implemented in commit `2047b8c`.
- **Evidence:**
  - `cmd/harnesscli/tui/cmd_parser.go` lines 84-88: parses the `/clear` command.
  - `cmd/harnesscli/tui/model.go`: handles the command by resetting the viewport, clearing the transcript, and closing the slash-complete dropdown.
- **Action:** Close issue #281. All acceptance criteria met.

### #273 — TUI-021: User message bubble

- **Status:** Fully implemented in commit `cb890be`.
- **Evidence:**
  - `cmd/harnesscli/tui/components/messagebubble/user.go` implements the full spec: dark-gray background (color 237), `❯` prefix, 2-space continuation indent, trailing blank line, and empty message handling.
- **Action:** Close issue #273. All acceptance criteria met.

---

## Phase 2: Existing Implementation Branches — Open PRs

These issues have committed work on branches but no PR has been opened (or one is already open). No new implementation work is needed.

### #316 — contextgrid tests

- **PR status:** PR #339 already open.
- **Branches:**
  - `upstream/agent/issue-316` — commit `0abb46f`
  - `upstream/issue-316-contextgrid-coverage` — commit `d08f7a0`
- **Action:** Review and merge PR #339. The duplicate branch (`issue-316-contextgrid-coverage`) should be reconciled before merge.

### #317 — thinkingbar tests

- **Branch:** `upstream/agent/issue-317` — commit `acb345b`
- **PR status:** No PR open.
- **Action:** Open a PR from `upstream/agent/issue-317` targeting `main`.

### #332 — runner orchestration tests

- **Branch:** `upstream/agent/issue-332` — commit `bcae146`
- **PR status:** No PR open.
- **Action:** Open a PR from `upstream/agent/issue-332` targeting `main`.

---

## Phase 3: Well-Specified — Ready for Implementation

These issues are fully groomed and can be picked up immediately.

### New Test Coverage Issues (unlabeled, from this grooming pass)

These were identified during the grooming pass and have not yet been addressed (except where noted).

| Issue | Title | Size | Status |
|-------|-------|------|--------|
| #329 | tests(runner): execute lifecycle characterization | medium | Not started |
| #330 | tests(runner): terminal sealing, recorder drops, audit-writer | medium | Partially addressed — commit `95fec1b` added `runner_audittrail_test.go` for issue #327, but #330 has broader scope |
| #331 | tests(runner): compaction helper + auto-compact fallback | medium | Not started |
| #333 | tests(tui): Update reducer precedence matrix | medium | Not started |
| #334 | tests(tui): SSE tool-call/usage/error/drop branches | medium | Partially addressed |
| #335 | tests(tui): Init/status tick/helper coverage | small | Partially addressed |
| #336 | tests(harnessd): runWithSignals startup matrix | medium | Not started |
| #337 | tests(harnessd): failure paths | medium | Not started |
| #338 | tests(harnessd): shutdown order + cron flake | medium | Not started |

### Previously Labeled Well-Specified Issues

These issues were already labeled and groomed before this session. Listed here for completeness.

| Issue | Title | Size |
|-------|-------|------|
| #23 | Research: OS sandboxing | medium |
| #24 | Research: Session tree branching | medium |
| #136 | Research: mid-run model switching | unlabeled size |
| #225 | Research: Lightpanda integration | medium |
| #318 | security: derive effective tenant from auth | medium |
| #319 | security: enforce API key scopes | medium |
| #320 | feat(store): persist run state | large |
| #321 | feat(runs): run cancellation endpoint | medium |
| #322 | feat(runs): bounded scheduler/worker pool | large |
| #323 | feat(permissions): interactive approval workflow | large |
| #324 | feat(runs): workspace backends selectable | large |
| #325 | feat(runner): parallel-safe tool calls | medium |
| #326 | ux(tools): download + todo mutations | medium |

---

## Phase 4: Needs Clarification — Skip Implementation

These issues require design decisions or API contract definitions before work can begin. They should be labeled `needs-clarification` and left open.

### #313 — TUI model availability

- **Blocker:** The API contract for exposing available models to the TUI is undefined. Implementation is blocked until the server-side contract is specified.
- **Label:** `needs-clarification`, `medium`

### #314 — Codex MCP server

- **Blocker:** This is in the design phase. The interface between the harness and a Codex MCP server has not been specified.
- **Label:** `needs-clarification`, `large`

### #315 — TUI auth management

- **Blocker:** Future feature. Design work is required before implementation can begin. No current auth management surface exists in the TUI.
- **Label:** `needs-clarification`, `large`

---

## Phase 5: Previously Labeled — No Action Needed

These issues already carry `needs-clarification` or `blocked` labels from prior grooming passes. No new action is required.

- #237, #235, #42, #212, #152, #153, #26, #55

---

## Action Summary for User

The following actions require manual intervention due to the GitHub token write restriction:

1. **Close** issue #281 (TUI-029: /clear command — fully implemented).
2. **Close** issue #273 (TUI-021: user message bubble — fully implemented).
3. **Open PR** for issue #317 from branch `upstream/agent/issue-317`.
4. **Open PR** for issue #332 from branch `upstream/agent/issue-332`.
5. **Review and merge** PR #339 for issue #316 (contextgrid tests).
6. **Apply label** `needs-clarification` to issues #313, #314, #315.
7. **Apply size labels** to the new test coverage issues #329–#338 per the table in Phase 3.
