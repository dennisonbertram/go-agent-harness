# Issue #18 Grooming: Head-tail buffer for long-running process output

## Summary
Long bash commands (builds, tests) produce overwhelming output. Add a head-tail buffer that keeps the first N + last N lines to keep output useful but bounded.

## Already Addressed?
**NEARLY RESOLVED — NEEDS MERGE** — Complete implementation exists on branch `automation/issue-18-head-tail-buffer` (commit `0580b37`). ~10 files, 302 insertions. `HeadTailBuffer` struct reduces bash output to first 100 + last 100 lines. Tests included. Not yet merged to main.

## Clarity Assessment
Clear.

## Acceptance Criteria
All met in the branch implementation.

## Scope
Atomic.

## Blockers
None — just needs merge.

## Effort
Done. Only merge + verify needed.

## Label Recommendations
Recommended: `needs-merge`

## Recommendation
**already-resolved (pending merge)** — Merge branch `automation/issue-18-head-tail-buffer` to main, run tests, then close.
