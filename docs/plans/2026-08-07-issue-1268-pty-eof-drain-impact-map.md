# Cross-Surface Impact Map: Issue #1268 PTY EOF Drain

## Task

- Task / issue: #1268 acceptance-runner PTY tail preservation.
- Plan link: `2026-08-07-issue-1268-pty-eof-drain-plan.md`.
- Owner: acceptance `ptyrunner`.
- Status: implemented in the closing PR, pending merge.

## Current Ownership, Callers, and Data Flow

- Entry points: `RunNonMutatingCommandBatch` and `RunFreshConversation` in `internal/acceptance/ptyrunner/runner.go`.
- Owning source of truth: one `freshFrameCollector` reads the master PTY into raw/frame artifacts; `waitEOF` owns terminal-read completion; `sealFinal` owns immutable final screen evidence.
- Callers/consumers: acceptance tests retain artifacts and correlate them to API/SSE data; no product request path calls this package.
- Similar abstractions searched: both paths share the same `ptyCmd.Wait -> master.Close -> collector.waitEOF` tail sequence; `TestLinuxPTYSlaveCloseDrainsAsCleanEOF` mirrors it.
- Search evidence: `rg -n -C 8 "waitEOF|master\\.Close|cmd\\.Wait|LinuxPTYSlaveCloseDrains" internal/acceptance/ptyrunner`.
- Conclusion: success must drain while the master remains open, seal the complete prefix, then let cleanup close it.

## Config, API, CLI, and Tools

- User-facing config/defaults/environment/endpoints/CLI wire formats: None. Existing configured context and test timeout remain unchanged.
- Error states: collector's existing EOF/EIO normalization and unexpected-read error propagation remain authoritative.

## Persistence and Compatibility

- Schemas/migrations/caches: None.
- Compatibility: generated artifact completeness improves; artifact names and formats remain compatible.
- Mixed rollout: None; acceptance-only Go package.

## Lifecycle, Security, and Reliability

- Lifecycle: success becomes `Wait -> waitEOF -> sealFinal -> deferred Close`; the shared deferred cleanup explicitly closes the master before child-process cleanup on early abort/error returns.
- Security/privacy: no credentials, authorization, or file-boundary changes; artifacts remain inside caller-built `ArtifactRoot`.
- Failure/recovery: bounded caller context still ends a stuck drain; unexpected read errors still fail rather than being normalized.

## Product and Integration Surfaces

- Server/runtime: None; harnessd remains unchanged.
- TUI/web/macOS: no client code change; retained real-TUI evidence can include the final bytes reliably.
- Provider/model/tool catalog/external automation: None.
- UX/accessibility: None; this affects acceptance evidence, not rendered product behavior.

## Deployment and Operations

- Deployment/flags: normal Go test/CI delivery, no runtime rollout.
- Logs/metrics: durable engineering logs record artifact-tail ownership and exact verification commands.
- Rollback: revert the isolated acceptance-runner/fixture commit; no persisted state requires repair.
- Runbooks: existing regression gate remains sufficient.

## Regression Tests

- Characterization/red: the cleanup-order unit test is undefined before the helper exists; Linux real PTY fixture establishes child exit, collector EOF, and final tail before cleanup without a timeout.
- Acceptance: both acceptance paths retain their final `sealFinal` behavior after drain.
- Edge/negative/lifecycle: preserve injected `n > 0` plus `EIO`, unexpected read error, context bounded EOF wait, and early abort close paths.
- Real-path proof: Linux PTY child writes a final marker and closes its slave; collector must retain it as a clean EOF.
- Commands: `go test ./internal/acceptance/ptyrunner -run 'TestLinuxPTYSlaveCloseDrainsAsCleanEOF|TestFreshCollectorRetainsBytesWhenReadReturnsDataAndEIO' -count=20`; same with `-race`; package normal/race; `TMPDIR=/private/tmp GOCACHE=/private/tmp/gocode-1268-cache ./scripts/test-regression.sh`.

## Documentation and Handoff

- No public/spec change beyond plan/impact. Add durable logs and `docs/plans/INDEX.md` / `docs/logs/INDEX.md` entries after code/test evidence.

## Warning Check

- No blank surface: unaffected product interfaces are explicitly searched and documented as none.
