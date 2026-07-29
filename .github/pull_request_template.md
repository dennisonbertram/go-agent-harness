Closes #<!-- required same-repository issue number -->

## Summary

<!-- Required: what changed and which externally observable outcome it provides. -->

## Scope and issue reconciliation

<!-- Required: compare the final diff with the linked issue's in-scope, out-of-scope, acceptance, and dependencies. Explain every deviation and update the issue before review. -->

## Impact analysis reconciliation

<!-- Required: revisit every issue impact-map surface. Name changed files/symbols and explain why all unaffected surfaces remain unaffected. -->

## Architecture and duplication check

<!-- Required: list existing abstractions/callers searched, the ownership boundary used, and evidence that this does not create a parallel source of truth or spaghetti wiring. -->

## Test-first evidence

<!-- Required:
Red command:
Observed failure:
Why the failure proved the missing/incorrect behavior:
Green command:
Refactor/characterization evidence:
For a documentation-only minor PR, explain the non-code verification substituted for TDD.
-->

## Verification evidence

<!-- Required: exact targeted, integration/e2e, race, full regression, and real user-path commands with PASS/FAIL results. Keep implemented, tested, and proven claims distinct. -->

## Rollout and rollback

<!-- Required: migration/deployment order, observability, rollback trigger, recovery command/path, and data repair. Write `None — reason` only after checking. -->

## Documentation

<!-- Required: specs, public docs, runbooks, indexes, logs, comments, release notes, and operator guidance updated or explicitly unaffected with rationale. -->

## Contract checklist

- [ ] Linked issue follows the current structured contract and this PR closes it
- [ ] Issue acceptance criteria, impact map, and scope were updated when the design changed
- [ ] All callers, consumers, sources of truth, and similar abstractions were searched
- [ ] No unrelated cleanup, hidden scope growth, duplicated wiring, or parallel abstraction was introduced
- [ ] Tests were written first and the expected red failure was observed, or this is a strictly docs-only minor PR
- [ ] Targeted checks and the repository-required full regression are green
- [ ] Security, compatibility, lifecycle, deployment, observability, documentation, and rollback were reconciled
- [ ] Real mouse/keyboard/API/operator behavior was exercised when the change is interaction- or integration-heavy
