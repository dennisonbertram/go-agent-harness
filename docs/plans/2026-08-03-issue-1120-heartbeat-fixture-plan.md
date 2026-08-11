# Plan: deterministic blocked-heartbeat callback fixture (Issue #1120)

## Context and Scope

- Governing issue: #1120. This is a test-and-documentation-only follow-up to
  #1117/#1106, stacked on #1119 head `869d26ad`.
- Sol diagnosis: the old 90 ms lease could expire before its first heartbeat
  entered the deliberately blocking store under hosted race load. That is a
  fixture scheduling gap, not evidence that production fencing lost ownership.
- In scope: make the blocked-heartbeat fixture establish the intended causal
  ordering and assert the complete durable handoff contract.
- Out of scope: callback manager/store/runtime semantics, API/TUI/native GUI,
  schema, configuration, and merging either parent PR.

## Test-First Contract

1. Preserve the historical red evidence: hosted race scheduling could prevent
   the old 90 ms fixture from entering its blocking heartbeat; the old local
   race x200 characterization passed and therefore is not claimed as a
   reproduction of a production defect.
2. Give the fixture a one-second lease, wait for the original starter and then
   `blocking.entered`, and only then ask a second manager to recover.
3. Require process-fence rejection, deadline cancellation of the original,
   exact claimed-token release into `retry_wait` with cleared fence fields,
   attempt one and the reserved run ID unchanged, and no replacement admission.

## Impact and Rollout

- Test fixture: `TestCallbackManagerBlockingHeartbeatCancelsBeforeTakeover` and
  its test-only `releaseObservingStore` gain ordering and exact-token evidence.
- Production/data/API/clients/security/deployment: none; production source is
  intentionally unchanged. Rollback is a revert of the isolated test/docs PR.
- Required validation: focused normal/race x100 plus stress, complete tools
  normal/race, and isolated foreground `./scripts/test-regression.sh`.

## Checklist

- [x] Verify #1120's Sol diagnosis and stack on #1119.
- [x] Capture pre-change focused race x200 characterization.
- [x] Strengthen only the fixture and its test helper.
- [x] Run the required stress/package/full gates on the final tree.
- [x] Push one stacked PR with `Closes #1120` and stop for review.

## Evidence

- Pre-change characterization: focused old-fixture race x200 passed in
  22.373s; this confirms the hosted red is load-sensitive and is not treated as
  an unwaived baseline.
- Final fixture: normal x100 passed in 100.791s; race x100 passed in 103.045s.
  Complete `./internal/harness/tools` passed normal in 13.562s and race in
  14.836s.
- Isolated `./scripts/test-regression.sh`: PASS, including normal and race;
  coverage 85.5% against the 80% minimum and zero uncovered functions.
