# Plan: Anytime Contextual Feedback Intake

## Context

- Governing GitHub issue: #1023
- Problem: `/feedback` creates a local-only zip from the TUI, accepts no user
  request, omits the active transcript/session and daemon logs, cannot include
  an intentional screenshot, and cannot prepare or create a structured issue.
- User impact: defects lose the exact evidence needed to reproduce them, so the
  user must stop dogfooding go-code and manually reconstruct context.
- Constraints:
  - preserve local-only behavior unless issue submission is explicit;
  - never claim that screenshot pixels are redacted;
  - use only supported GitHub issue behavior;
  - keep bundle creation useful when GitHub, `gh`, or the browser is unavailable;
  - do not interrupt or mutate an active run.

## Scope

- In scope:
  - accept a free-form request in `/feedback`;
  - snapshot current run, conversation, workspace, transcript, selected model,
    bounded rollout files, and bounded daemon logs into the canonical zip;
  - include an explicitly selected PNG or JPEG after validating type, size,
    regular-file ownership, and symlink safety;
  - generate a sanitized structured issue body;
  - open a prefilled GitHub browser draft when explicitly requested, because
    the full context bundle is binary evidence and GitHub exposes its supported
    attachment uploader in the issue composer;
  - retain the local bundle and report partial success on GitHub failures;
  - add a top-level `go-code feedback` entry point only if it can reuse the same
    bundle contract without duplicating TUI state ownership.
- Out of scope:
  - undocumented GitHub attachment APIs;
  - automatic background issue creation;
  - hosted telemetry or artifact storage;
  - autonomous triage, repair, merge, or release;
  - OCR or pixel redaction;
  - a separate macOS UI.

## Documentation Contract

- Feature status: `implemented and merged in PR #1025`
- Public docs affected:
  - `website/docs/reference/cli-flags.md`
  - TUI slash-command/operator documentation that describes `/feedback`
- Spec docs to update before code:
  - this plan;
  - `2026-07-30-issue-1023-feedback-intake-impact-map.md`.
- Implementation notes to add after code:
  - `docs/logs/engineering-log.md`;
  - affected documentation indexes.

## Test Plan (TDD)

- New failing tests to add first:
  - `/feedback <request>` writes `request.md`, `context.json`, and a redacted
    transcript while preserving active-run state;
  - valid PNG/JPEG input lands in `attachments/` with checksum/provenance;
  - directories, symlinks, unsupported types, malformed headers, and oversized
    images fail without a misleading bundle success;
  - request/transcript/log/rollout canary secrets do not survive into text
    members or the generated issue body;
  - GitHub command failure retains the bundle and reports partial success;
  - binary evidence selects a supported browser draft rather than an
    undocumented upload request.
- Existing tests to update:
  - feedback bundle membership and command registration/description;
  - slash-command parser/command-path tests;
  - wrapper routing tests only if `go-code feedback` is added.
- Regression tests required:
  - no-argument `/feedback` remains local-only and succeeds without rollouts;
  - active run/conversation state remains unchanged.

## Cross-Surface Impact Map

- See `2026-07-30-issue-1023-feedback-intake-impact-map.md`.

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Link a contract-complete structured GitHub issue before implementation.
- [x] Record current architecture, callers, consumers, and source-of-truth search evidence.
- [x] Document feature status and exact contract before code.
- [x] Complete and reconcile the cross-surface impact map before implementation.
- [x] Add characterization coverage before structural refactors.
- [x] Write failing tests first.
- [x] Review ownership/copy semantics for exported or state-storing types when mutable fields cross boundaries.
- [x] Implement minimal code changes.
- [x] Refactor while tests remain green.
- [x] Update docs, status ledgers, and indexes.
- [x] Update engineering/system/observational logs as needed.
- [x] Run full test suite.
- [x] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: a screenshot or transcript leaks sensitive data.
  - Mitigation: screenshots require an explicit file path and carry an
    unredacted-pixels warning; text is redacted before local persistence or
    issue generation; issue submission is explicit.
- Risk: “all context” grows without bound.
  - Mitigation: cap transcript, logs, rollout count, member size, and total
    bundle size; record truncation notes.
- Risk: issue creation succeeds but binary evidence is absent.
  - Mitigation: when binary evidence exists, use a prefilled web draft and
    clearly identify the files that must be attached before submission.
- Risk: feedback capture disrupts an active run.
  - Mitigation: copy current values synchronously and do not cancel, steer,
    subscribe, or mutate run state.
