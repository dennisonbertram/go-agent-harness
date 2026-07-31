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

## 2026-07-31 independent production-review repair

PR #1021 was re-reviewed at exact head
`1f2444b2480b5832139318e4fa034f4240d92b8d` and integrated with
`origin/main` at `b3afc7ec487c60762a91a1219ceb92c523ef0e78`.
The repair branch closes the newly confirmed source/test gaps:

- generation ownership for run registration, answer acknowledgements, pending
  questions, catalog/conversation/activity/rewind loads, conversation opening,
  and durable conversation sync;
- one cancellable and generation-checked transcript autoscroll completion,
  Reduce Motion behavior, and an accessible Jump to Latest control;
- single-flight approve/deny/steer controls with exact failed-steer draft
  restoration that never overwrites a newer manual edit;
- lifecycle and rewind busy guards with shared disabled reasons exposed through
  mouse help and VoiceOver hints;
- stale collection rows preserved under a failed refresh;
- duplicate/declined prompt-history recall bookkeeping;
- the missing impact map, repair plan addendum, engineering/intent logs, and
  documentation indexes.

The merge explicitly preserves #1008 conversation replay deduplication and
#1028 failed/cancelled terminal reconciliation. External conversation-stream
activity contributes observable busy state to #995 guards, but actionable
scheduled-run identity remains #1007 and is not implemented by this PR.
Hosted live-harnessd verification additionally exposed and repaired terminal
usage/cost loss when durable replay immediately followed a terminal event.
Hosted Go race promotion remains blocked by the already-owned current-main
cleanup race in #1039 / PR #1041 rather than duplicating that fix here.
A subsequent Codex pass repaired three further ownership edges: pending-answer
view state now follows `callID`, a stale run-todo response no longer aborts
independent activity collections, and rewind refusals cannot cross or retry
against a different conversation.
The remaining accounting review threads are also repaired: cumulative
usage/cost and priced status reset at the new-run boundary, late prior-run
events cannot reclaim the active run's accounting or terminal state, and
sealed totals are applied consistently to completed, failed, and cancelled
runs.
A final ordering pass additionally binds force confirmation before alert
dismissal, releases the active run id on local force cancel, applies ready
tasks/runs without waiting for slow todos, and lets sealed terminal cost status
override earlier deltas while remaining immune to late duplicate status.
The final ownership pass additionally reserves accounting for the exact run id
returned by submission, rejects stale reserved refreshes before they set
loading, and resets transcript pin/autoscroll state when conversation identity
changes.

## Residuals (not fixed on this branch)

1. **`setCost` status-erasure gap / Settings investigation.** `ModelSettingsModel.setCost` (`macapp/Sources/GoCodeUI/ModelSettingsView.swift:132-140`)
   was not brought in line with the `load(clearingStatus: false)` pattern applied to its five
   siblings (`fetch`/`setExposed`/`setAllVisible`/`saveProvider`/`delete`) for the same class of
   finding. It remains intentionally pending the separate native Settings root-cause investigation
   and is not claimed fixed by the shared stale-row refresh repair. Tracked in follow-up issue:
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
