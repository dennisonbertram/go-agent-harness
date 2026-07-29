# Issue-Driven Development Runbook

## Outcome

No change reaches `main` without an auditable chain:

```text
structured issue -> impact analysis -> isolated worktree -> tests red ->
minimal implementation -> tests green -> PR evidence -> review -> merge
```

This contract exists to stop local symptom patches, duplicate abstractions, and
unexamined cross-surface regressions before they become technical debt.

## 1. File the Issue Before Work Starts

Choose the form using [the triage table](issue-triage.md). Fill every required
field with concrete evidence. A form is not ready because every box contains
text; it is ready when another engineer can identify:

- what behavior changes and what must remain stable;
- where the current behavior is owned;
- which callers and downstream consumers exist;
- how every relevant integration surface was checked;
- which test will fail before implementation;
- how the result will be verified and rolled back.

Issue Forms are contracts, not immutable guesses. Edit the issue before code
when investigation changes the architecture, scope, dependencies, or acceptance
criteria.

## 2. Trace the Change Before Designing It

Search from both directions:

1. Entry points to the owning seam.
2. The owning seam to every caller, consumer, persisted value, event, client,
   and operational dependency.

Record file paths, symbols, and search commands in the issue. Prefer extending
the existing source of truth. If a second abstraction is genuinely necessary,
explain its separate ownership and lifecycle.

The impact map must reconcile:

- calls and data flow;
- config, defaults, environment, feature flags, APIs, CLI, and tools;
- persistence, schemas, migrations, caches, and generated data;
- concurrency, lifecycle, cancellation, retries, and cleanup;
- authentication, authorization, privacy, secrets, and trust boundaries;
- TUI, web, macOS, automation, integrations, and other clients;
- provider/model/tool catalogs and routing;
- deployment, monitoring, alerts, support, and rollback;
- compatibility, versioning, tests, fixtures, docs, and onboarding.

Write `None — <search evidence and reason>` for an unaffected surface. A blank
or unsupported `None` is incomplete.

## 3. Create the Plan and Worktree

Link the issue from the task plan. Bootstrap an isolated worktree from current
`origin/main` using `scripts/init.sh`. The plan may add implementation detail,
but cannot silently weaken the issue acceptance contract.

## 4. Execute Strict TDD

For behavior changes:

1. Characterize the current seam before structural refactoring.
2. Write the smallest meaningful acceptance/regression test.
3. Run the exact targeted command and record the expected red failure.
4. Implement only enough to make that behavior green.
5. Refactor while tests remain green.
6. Add edge, negative, failure, lifecycle, and integration coverage identified
   by the impact map.
7. Run the repository-required full regression before commit and merge.

Minor documentation changes replace TDD with exact link/example/format checks;
they may not change code, tests, automation, or behavior.

## 5. Reconcile the PR Against the Issue

Use `.github/pull_request_template.md` without deleting sections. The PR must:

- close a same-repository shippable issue;
- explain any scope or impact-map changes and update the issue first;
- show search evidence against duplicated ownership;
- record exact red, green, targeted, full, and real-path proof;
- distinguish implemented, tested, and proven;
- document rollout, observability, rollback, and docs;
- complete every contract checkbox.

Epics are referenced as `Related #N`; they are never the only closing issue.

## 6. Pilot Enforcement

The first rollout deliberately tests whether strong Issue Forms, `AGENTS.md`,
`CLAUDE.md`, repository drift tests, and the PR template are sufficient to keep
Claude and other agents disciplined. GitHub branch protection and a required
status check are not enabled in this pilot.

The absence of a technical merge block does not weaken the policy. Agents and
reviewers reject work when:

- no same-repository issue exists before implementation;
- a legacy, incomplete, or wrong issue type is used;
- an epic substitutes for a shippable child issue;
- the PR template is incomplete or its contract items are unchecked;
- research or minor work changes prohibited files/surfaces;
- scope, impact analysis, or test evidence changed without updating the issue.

The repository test in `internal/quality/repostructure` prevents the forms, PR
template, and agent rules from silently drifting or disappearing. It does not
inspect individual issue or PR contents.

## 7. What the Process Does Not Prove

Templates and policy prove neither compliance nor engineering truth. Reviewers
still verify that:

- search and impact evidence is accurate;
- tests cover behavior rather than implementation trivia;
- the chosen ownership boundary is coherent;
- the diff matches the issue and avoids opportunistic cleanup;
- real user/operator behavior was exercised where required;
- rollback is actually usable.

If the pilot shows repeated noncompliance, a trusted read-only PR validator and
protected-branch requirement can be proposed as a separate issue with evidence
from the failures. Do not preempt that decision inside ordinary feature work.
