# Plan: Autoresearch-Style Testing Loop for go-agent-harness

## Status Ledger

- `prompt-profile`: `planned`
- `one-shot-runner`: `planned`
- `targeted-loop`: `planned`
- `documentation`: `planned`

Allowed statuses:

- `planned`
- `in implementation`
- `implemented`
- `deferred`
- `rejected`

## Intent

- Command intent: apply the `autoresearch` idea to harness testing by building a narrow, repeatable loop that searches for useful regression and characterization tests, scores them with existing test commands, and keeps the workflow grounded in repo-native evidence.
- User intent: create a concrete plan plus first-pass prompt/profile and loop scripts that can be run against this repo without inventing a separate infrastructure stack.

## Core Idea

`autoresearch` works best when the loop is small, explicit, and scored by a stable oracle. For this repo the oracle is the existing test suite and smoke commands, not a synthetic benchmark.

The first pass should do four things:

1. Give the agent a narrow test-research prompt that favors one seam at a time.
2. Route that prompt through a dedicated prompt profile so the system prompt stays reusable.
3. Use a script wrapper to post the run request, wait for completion, and capture logs.
4. Use a loop wrapper to cycle through the highest-risk seams from the coverage-gap report.

## Scope

### In Scope

- A dedicated prompt-profile entry for `autoresearch`.
- A one-shot shell runner that sends an `autoresearch` prompt and records the result.
- A loop shell runner that iterates across a small target list and picks a validation command for each seam.
- A short runbook note explaining how to launch and score the loop.

### Out of Scope

- New product routes.
- New provider/model logic.
- Any autonomous fixer that rewrites the harness without a test-first workflow.
- Any benchmark or leaderboard framing.

## Target Seams For The First Loop

Start with seams that already appear in the coverage-gap report:

- `internal/workspace.ContainerWorkspace.Provision`
- `internal/harness/tools/core.GitDiffTool`
- `internal/harness/tools/core.ObservationalMemoryTool`
- `internal/harness.Runner.SubmitInput`
- `internal/harness.parseTurnsHTTP`
- `internal/server.handleRunEvents`

These targets are broad enough to matter, but small enough to map to narrow test commands.

## Prompt-Profile Contract

The `autoresearch` prompt profile should:

- bias toward regression tests, characterization tests, and failure-path checks
- insist on a single seam per run
- prefer tests before implementation
- require exact commands and results in the final response
- tell the agent to narrow scope if the requested target is too broad

## Loop Contract

The loop scripts should:

- post a run request with:
  - `profile = full`
  - `prompt_profile = autoresearch`
  - a target-specific prompt body
- wait for the run to finish
- run the target-specific validation command
- save a markdown report plus raw logs under `.tmp/autoresearch/`
- continue across the target list even if one target fails

## Validation Command Strategy

- Run the narrowest package test that exercises the target seam first.
- Escalate to `./scripts/test-regression.sh` only when the seam is broad or the change spans multiple packages.
- Keep the default test command visible in the report so the next iteration can compare it to later runs.

## Required Tests Before Code

- Add a prompt-profile resolution test proving `autoresearch` resolves correctly.
- Keep the prompt fixture aligned with the new catalog entry.
- Confirm the shell scripts are syntactically valid and executable.

## Acceptance Criteria

- `prompts/models/autoresearch.md` exists and is wired into `prompts/catalog.yaml`.
- `scripts/autoresearch-run.sh` can run one target and record a markdown report.
- `scripts/autoresearch-loop.sh` can iterate across multiple targets.
- `docs/runbooks/testing.md` documents how to use the loop.
- A test proves prompt-profile resolution for `autoresearch`.

## Stage Verification Workflow

1. Add the prompt-profile and loop scripts.
2. Add or tighten tests around prompt-profile resolution.
3. Update the testing runbook with the new workflow.
4. Run the focused test packages.
5. Run the regression gate if the slice touches broader behavior.

