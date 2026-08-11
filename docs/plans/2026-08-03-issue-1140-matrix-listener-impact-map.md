# Cross-Surface Impact Map: Issue #1140 matrix listener identity

## Task

- Task / issue: #1140, bind matrix readiness to the listener actually acquired.
- Plan link: `2026-08-03-issue-1140-matrix-listener-plan.md`.
- Owner: harnessd maintainers.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: `runWithSignals` → `runWithSignalsWithDeps` → listener acquisition; test-only `runMatrixTest`.
- Owning source: `cmd/harnessd/main.go` owns daemon startup; `cmd/harnessd/main_test.go` owns parallel matrix startup fixtures.
- Callers/consumers: production calls `runWithSignals`; direct dependency tests call `runWithSignalsWithDeps`; matrix tests use `/healthz` then selected endpoints.
- Similar abstractions searched: existing `runDeps.newConversationCleaner`; `net.Listen` at the sole HTTP startup point; `freeLocalAddr` matrix fixture helper.
- Search evidence: `rg -n "runDeps|runMatrixTest|freeLocalAddr|net.Listen" cmd/harnessd`.
- Conclusion: listener is already locally owned by the startup function, so an optional dependency preserves one ownership boundary.

## Config, API, CLI, and Tools

- User-facing config: unchanged; matrix fixture uses existing `HARNESS_ADDR=127.0.0.1:0`.
- Defaults/fallbacks: nil injected listener defaults to `net.Listen`.
- API/CLI/tools: `/healthz` and `/v1/skills` remain unchanged; the test helper probes only the acquired address.
- Errors: early `runWithSignals` failure is observed before readiness timeout.

## Persistence and Compatibility

- Schemas/migrations/caches: None.
- Compatibility: production startup behavior and address resolution remain compatible.
- Mixed rollout: None; a single internal dependency default.

## Lifecycle, Security, and Reliability

- Concurrency: removes TOCTOU port reservation in parallel tests; listener lifecycle remains owned by `httpServer.Serve`/shutdown.
- Security/privacy: no new input, authority, secrets, or network exposure.
- Failure/recovery: preserves listen failure cleanup and callback recovery ordering; helper reports early startup errors deterministically.

## Product and Integration Surfaces

- Server/runtime: startup binding seam only.
- TUI/web/macOS: None; no behavior, wire, or transcript change.
- Provider/model/tool catalogs: no production loading change; custom skill fixture remains the acceptance endpoint.
- External systems/UX: None.

## Deployment and Operations

- Deployment/migration: none.
- Observability: existing `harness server listening on` log continues to emit actual listener address.
- Rollback: revert internal injected dependency/helper change; no persistent state.
- Runbooks: testing evidence in issue/PR and durable logs.

## Regression Tests

- First red: listener-identity matrix test must fail against the old helper because it probes literal `127.0.0.1:0`.
- Acceptance: wrapper captures the actual listener address, custom global skill response is obtained from that server.
- Edge/failure: early startup/listen failure is selected before readiness; no arbitrary sleep or test serialization.
- Real path: `runMatrixTest` launches actual `runWithSignalsWithDeps`, then makes real `/healthz` and `/v1/skills` requests.
- Commands: focused normal/race stress, full `TestMatrix_` normal/race, `go test ./cmd/harnessd` normal/race, `./scripts/test-regression.sh`.

## Documentation and Handoff

- Public docs: None.
- Internal notes: plan/impact, engineering/observational/system/long-term logs and indexes.
- Training/release notes: None; test-harness-only reliability repair.
