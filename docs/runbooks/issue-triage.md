# Issue Triage Runbook

## Policy

Every repository change starts from a current structured GitHub Issue Form
before a branch, plan implementation, or code edit begins. Blank issues are
disabled. The issue remains the source of truth for acceptance, scope, impact,
tests, and rollout; update it before continuing when those change.

Confidential vulnerabilities are the only public-issue exception: use GitHub
private vulnerability reporting and do not paste exploit or credential details
into a public issue.

## Select the Contract

| Work | Form | Shipping rule |
| --- | --- | --- |
| Feature, refactor, maintenance, dependency, CI, process, API, architecture, performance, or reliability change | Engineering change / feature slice | PR closes this issue |
| Incorrect or regressed behavior | Bug / regression | Failing regression test first; PR closes this issue |
| Multi-PR outcome | Epic | Decompose into shippable child issues; implementation PRs do not close the epic |
| Evidence-gathering decision | Research spike | Output PR is documentation-only; product code uses a follow-on issue |
| Tiny non-behavioral doc correction | Minor documentation-only change | Documentation-only paths, at most 5 files and 150 changed lines |

If uncertain, use the full engineering-change or bug form. Never downgrade work
to minor to avoid analysis.

## Ready-for-Implementation Gate

Do not implement until the issue contains:

1. Concrete externally observable acceptance behavior.
2. Explicit in-scope and out-of-scope boundaries.
3. Repository search evidence naming the current owners, callers, consumers,
   similar abstractions, and sources of truth.
4. An impact analysis covering all fields in the selected form. `None` is valid
   only with a searched rationale.
5. The first meaningful test, exact red command, and expected failure.
6. Targeted, integration, full-regression, and real-path verification plans.
7. Compatibility, deployment, observability, rollback, and documentation plans.
8. Dependencies and ordering, including parent/child issues.

## Bug-Only Artifacts

Every bug additionally requires:

- a deterministic reproduction;
- a permanent failing regression test before the fix;
- an engineering-log entry in the same PR;
- blast-radius verification of adjacent callers and product surfaces.

If a second bug is discovered while implementing the first, create a separate
bug issue and regression test. Do not hide it inside the current scope.

## PR Linkage

The PR body must use a GitHub closing keyword for a same-repository issue:

```text
Closes #123
```

`Related #123` is useful for epics and adjacent work but does not satisfy the
shipping contract. During the process-guided pilot, agents and reviewers must
reject a PR that does not follow this linkage; GitHub does not yet enforce it
through branch protection.
