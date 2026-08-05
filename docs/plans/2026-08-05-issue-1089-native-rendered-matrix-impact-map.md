# Issue #1089 Impact Map

## Task

- Issue: #1089; plan: `2026-08-05-issue-1089-native-rendered-matrix-plan.md`; status: implementation.

## Current Ownership, Callers, and Data Flow

- Entry: `scripts/run-native-gui-acceptance.sh` -> real driver -> `cmd/native-gui-acceptance` -> running `/v1/tools` -> #1086 compiler -> proof validator.
- Source of truth: resolved registry/TUI metadata and #1086 `SuiteContract`; no static tool list.
- Evidence: every native pass retains ordered actions, UI evidence, SSE/API/store probes, IDs, cleanup, and environment identity.

## Config, API, CLI, and Tools

- Required env: repository-tracked driver and fresh loopback `HARNESS_BASE_URL`;
  the launcher creates nonce/temp/artifact/manifest paths and exports their
  provenance. No production endpoint/config change.
- The CLI rejects missing resolver records and does not start, stop, or attach
  to arbitrary processes. A driver outside the repository or a non-loopback
  URL cannot qualify.

## Persistence and Compatibility

- No product schema/migration. Artifacts are isolated driver-owned files and are digest-checked before a pass.

## Lifecycle, Security, and Reliability

- Serialized driver owns app/daemon cleanup. The qualifying manifest has one
  final PASS per applicable case, while generic reports keep their history.
  Canonical root and regular-file checks prevent traversal and symlink escapes;
  proof contract requires redaction declaration and binds each row to the
  launcher nonce, exact app build, and child daemon identity.
- Existing GoCode/harnessd processes are neither killed nor reused.

## Product and Integration Surfaces

- Native GUI: actual installed bundle evidence is mandatory; headless ToolWalk cannot satisfy it.
- TUI slash commands: mapping must be explicit N/A with source/UX rationale; native composer exposes controls/navigation instead.
- API/SSE/store: corroborating probes only, never a substitute for rendered evidence.

## Operations and Tests

- Failure preserves manifest/artifacts and fails closed for drift, missing native mappings/cases, invalid bundle metadata, digest mismatch, or missing cleanup.
- Focused red/green validates tamper detection. Full Swift/Go gates remain required before PR.
