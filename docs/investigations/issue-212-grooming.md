# Issue #212 Grooming: Forensics — Run Replay and Fork from Any Step (Phased)

## Summary
Phased forensics feature: rollout loader, offline replay, fork-from-step-N, and HTTP endpoint.

## Already Addressed?
**Partially — Phases 1–3 are complete and landed.**

Evidence:
- `internal/forensics/replay/replayer.go` — Phase 2 offline simulation (26 tests, 98.3% coverage)
- `internal/forensics/replay/forker.go` — Phase 3 fork from step N reconstruction
- `internal/forensics/rollout/` — Phase 1 rollout loader + canonicalization
- `docs/implementation/forensics-211-212-215-spec-2026-03-12.md` — full design doc
- Owner comment dated 2026-03-12 confirms Phases 1–3 are done; Phase 4 (HTTP endpoint) remains

**Only Phase 4 (HTTP endpoint wiring in server/http.go) is incomplete.**

## Clarity
**5/5** — Extremely detailed spec with wave dependency order.

## Acceptance Criteria
**Partial** — 3 of 4 phases done. Phase 4 noted as "in progress."

## Scope
**Needs splitting** — Phase 4 (HTTP endpoint) should be a separate implementation issue from the completed phases.

## Blockers
None (#217 event schema versioning resolved).

## Recommended Labels
`needs-clarification` (update status), `small` (for Phase 4 only)

## Effort
**Small** — Only Phase 4 remains: wire HTTP endpoint in `server/http.go`.

## Recommendation
**already-resolved (Phases 1–3) / needs-clarification** — Update issue body to reflect completion of Phases 1–3. Either close and open a new issue for Phase 4 HTTP endpoint, or update this issue's scope.
