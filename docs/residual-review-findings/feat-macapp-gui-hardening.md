# Residual review findings — feat/macapp-gui-hardening (epic #991)

Plan: `docs/plans/2026-07-30-001-feat-macapp-gui-hardening-plan.md`

## Adversarial review summary

An adversarial review pass ran across 4 dimensions (correctness, concurrency/races, safety-guard
completeness, accessibility) against the branch's 8 implemented slices. It confirmed 15 findings;
14 were fixed on-branch. Fix commits:

- `dd0ae517` — test(red)(macapp): failing tests for review fixes F1-F8
- `4250d9f2` — fix(macapp): implement review fixes F1-F8
- `c4a0b5ea` — test(regression)(macapp): regression coverage for F1a/F4 fixes in `4250d9f2`
- `e8e06514` — test(red)(macapp): failing test for stale runControlTask write race
- `cb8ab7db` — fix(macapp): guard runControlTask against stale post-reset writes
- `83d3a29e` — test(macapp): steer failure coverage and try?-absence scan for steer
- `c540f3e1` — refactor(macapp): rewire duplicate source-scan helpers to ReachabilitySource
- `158a26b4` — fix(macapp): ReachabilitySource.wholeModule scans Sources/GoCodeUI recursively

## Residuals (not fixed on this branch)

1. **`setCost` status-erasure gap.** `ModelSettingsModel.setCost` (`macapp/Sources/GoCodeUI/ModelSettingsView.swift:132-140`)
   was not brought in line with the `load(clearingStatus: false)` pattern applied to its five
   siblings (`fetch`/`setExposed`/`setAllVisible`/`saveProvider`/`delete`) for the same class of
   finding. Deliberately left out of U8 scope to keep that slice's diff scoped to the findings it
   was written against. Tracked in follow-up issue:
   https://github.com/dennisonbertram/go-code/issues/1020

2. **Deferred manual smokes.** The plan's Verification Contract lists six smokes that require a
   live app/daemon and were not run in this pass: transcript autoscroll scroll-up, failed-load
   retry with the daemon killed, delete/undo confirmation cancel paths, force-rewind "Restore
   Anyway", prompt-history Up/Down with a half-typed draft, and VoiceOver + keyboard-only passes
   on Sessions/Models. Tracked in the same follow-up issue above.

3. **Caret-aware prompt history (D1).** Literal caret-position-aware history recall (`#998`
   wording) is not implementable on the package's `.macOS(.v14)` floor (`macapp/Package.swift`);
   `TextSelection`/caret bindings are a macOS 15 API. The shipped approximation (recall only when
   the draft is empty/unchanged) is documented as KTD-7 in the plan. Tracked in the same follow-up
   issue above.

## Process deviation

The epic's acceptance criteria state each child issue (#992–#999) merges independently. This
branch instead delivers all 8 slices in a single PR. Reason: every unit shares at least one file
with another (`ChatView.swift` across U1/U3/U7; `ProjectSession.swift` across U2/U4/U5/U6;
`SessionsView.swift` across U2/U5/U6/U8), forcing serial execution rather than independent
fan-out — documented in the plan's Assumptions (A1) and Deviations section. Flagged for
maintainer judgment in the PR.
