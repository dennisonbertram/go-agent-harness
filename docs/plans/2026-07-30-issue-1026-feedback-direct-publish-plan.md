# Plan: Direct GitHub Publication for Attached Feedback

## Context

- Governing GitHub issue: #1026
- Problem: `/feedback` can capture rich context, but a screenshot must be named
  with `--screenshot`; publishing requires `--issue`, a browser composer, and
  manual attachment of both the image and bundle.
- User impact: filing feedback still interrupts dogfooding. The desired path is
  to paste an image into the message, type a request, and have a complete GitHub
  issue created without leaving the TUI.
- Constraints:
  - extend the existing input attachment and feedback sources of truth;
  - use supported GitHub APIs/CLI behavior, not the undocumented web attachment
    endpoint;
  - preserve a recoverable local bundle and image copies on external failure;
  - preserve ordinary image prompt submission and active-run state;
  - upload screenshot pixels as supplied under the current single-user policy.

## Scope

- In scope:
  - consume pending TUI image chips as `/feedback` evidence;
  - capture multiple validated images in attach order;
  - copy images beside the local bundle so async publication does not depend on
    clipboard temp-file lifetime;
  - publish by default through a dedicated GitHub release-asset bucket;
  - create the issue directly with inline images and a bundle download link;
  - report the issue URL and selectively consume captured chips after success;
  - retain `--issue` and `--screenshot` compatibility and add `--local`.
- Out of scope:
  - undocumented GitHub issue-upload endpoints;
  - general-purpose artifact hosting;
  - provider/model image-flow changes;
  - web or macOS feedback UI;
  - autonomous issue triage or repair.

## Documentation Contract

- Feature status: `implemented and verified; promotion pending`
- Public docs affected:
  - `website/docs/cli/tui.md`
  - `website/docs/reference/cli-flags.md`
- Spec docs to update before code:
  - this plan;
  - `2026-07-30-issue-1026-feedback-direct-publish-impact-map.md`;
  - `docs/logs/long-term-thinking-log.md`.
- Implementation notes to add after code:
  - `docs/logs/engineering-log.md`;
  - `docs/logs/system-log.md`;
  - `docs/logs/observational-log.md`;
  - affected documentation indexes and snapshots.

## Test Plan (TDD)

- New failing tests to add first:
  - an attached PNG chip plus `/feedback <request>` lands in the bundle and
    starts direct publication without `--issue`;
  - `--local` writes the same evidence without invoking GitHub;
  - multiple images preserve order and produce distinct bundle members and
    sidecars;
  - the publisher provisions or reuses the asset release, uploads the zip and
    images, writes inline/link Markdown, creates the issue without `--web`, and
    returns its URL;
  - upload/issue failures preserve paths and attachment chips;
  - successful publication removes only captured chips, preserving chips added
    after publication began.
- Existing tests to update:
  - feedback parser, issue-body, command help, status, and snapshots;
  - one-screenshot membership assertions where compatibility remains.
- Regression tests required:
  - ordinary image prompt submission still consumes and sends chips;
  - active feedback capture still leaves run state unchanged;
  - explicit `--screenshot` and compatibility `--issue` still parse.

## Cross-Surface Impact Map

- See `2026-07-30-issue-1026-feedback-direct-publish-impact-map.md`.

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
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: GitHub has no supported issue-attachment flag.
  - Mitigation: use documented release assets as the binary store and stable
    Markdown URLs in the created issue.
- Risk: the clipboard temp file disappears while async publication runs.
  - Mitigation: synchronously validate and copy every image to the durable local
    feedback directory before starting GitHub work.
- Risk: async success clears a new attachment the user added meanwhile.
  - Mitigation: remove only the exact captured attachment paths on success.
- Risk: asset release provisioning races or partially succeeds.
  - Mitigation: make release view/create idempotent, use unique filenames, and
    preserve local artifacts with stage-specific errors.
- Risk: raw screenshot content is published unintentionally.
  - Mitigation: this is the explicit current single-user contract; keep the
    upload-as-is fact visible in docs and issue provenance without blocking.
