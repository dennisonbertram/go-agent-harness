# Cross-Surface Impact Map: Issue #1273

## Task

- Task / issue: #1273 native GUI proof artifact-root path identity.
- Plan link: `2026-08-07-issue-1273-nativegui-canonical-root-plan.md`.
- Owner: `internal/acceptance/nativegui` proof sealing and validation.
- Status: implemented locally; review/merge pending.

## Current Ownership, Callers, and Data Flow

- Entry points: `CoreProof.SealArtifacts`, `ValidateCoreProof`, and native
  scenario acceptance fixtures.
- Source of truth: `canonicalDirectory` validates the final root and resolves
  safe parent aliases; `canonicalCoreArtifact` validates regular non-symlink
  contained artifact files.
- Consumers: `WriteCoreProof` serializes the sealed proof; native acceptance
  suite validates it. Search evidence: `rg -n "SealArtifacts|ArtifactRoot|canonicalDirectory" internal/acceptance/nativegui`.
- Conclusion: normalize at the existing root boundary, not in individual
  artifact checks or production native-app paths.

## Config, API, CLI, and Tools

- No config, API, CLI, endpoint, tool, or wire-format changes.
- Persisted `ArtifactRoot` retains its existing field but is normalized to the
  already accepted canonical directory spelling during sealing.

## Persistence and Compatibility

- No schema migration. A proof generated through a safe parent alias now
  serializes the canonical root and remains acceptable to the existing reader.
- Unsafe final-root symlink paths remain incompatible by design.

## Lifecycle, Security, and Reliability

- No concurrency/retry/process change.
- Security boundary remains fail-closed: final root must not be a symlink;
  artifacts must be regular non-symlink files contained by canonical root.
- Failure recovery is unchanged: sealing returns an error before proof write.

## Product and Integration Surfaces

- Harness/API/TUI/web/macOS application behavior: none. This affects only
  acceptance proof construction/validation.
- External systems, provider/model catalog, and routing: none.

## Deployment and Operations

- No deployment or feature flag. Full regression resumes on macOS hosts where
  `/var` aliases `/private/var`.
- Rollback is reverting this isolated proof-infrastructure PR.

## Regression Tests

- First red: `go test ./internal/acceptance/nativegui -run
  TestCoreProofSealArtifactsCanonicalizesArtifactRootThroughParentAlias -count=1`
  rejected owned artifact zero before the fix.
- Coverage: portable parent-alias positive path, final-root symlink rejection,
  existing artifact-symlink/escape/hash/type/correlation negative cases.
- Commands: `go test ./internal/acceptance/nativegui -count=1`, `go test -race
  ./internal/acceptance/nativegui -count=1`, and `./scripts/test-regression.sh`.

## Documentation and Handoff

- Update plan/map, plans index, active plan, engineering/long-term logs, and
  logs index. PR must state `Closes #1273`; no merge in this slice.
