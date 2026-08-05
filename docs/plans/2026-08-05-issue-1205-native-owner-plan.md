# Plan: Issue #1205 owned native-acceptance foundation

## Context

- Governing issue: #1205 (parent #1089).
- Problem: the #1089 handoff proves an evidence validator, but its launcher accepts caller URLs, drivers, and manifest claims before trust is established.
- Constraints: zero-effect rejection; no discovery, attach, broad kill, foregrounding, installed-app replacement, or rendered scenario claim.

## Scope

- In scope: owner preflight, private root, repository-built fixed probe, fake-provider daemon/app child ownership, attestation, narrowly-owned cleanup, and sentinel tests.
- Out of scope: AX/OCR scenario execution, product UI changes, real providers, and a public inherited-listener daemon interface.

## Test Plan (TDD)

- First red: caller URL/driver/manifest, dirty/symlinked source, and missing opt-in reject before spawn or HTTP; sentinel app/daemon survive each injected failure.
- Green: only the owner creates a `0700` root, fixed built probe, isolated child daemon/app, and provenance; cleanup addresses only recorded handles.
- Full: focused Go, `cd macapp && swift test`, then `TMPDIR=/private/tmp ./scripts/test-regression.sh`.

## Listener Decision

`harnessd` has only a test-only injected listener seam. Exposing inherited FD startup is too broad for this slice, so the owner uses a short-lived loopback `:0` reservation, immediately releases it before child spawn, and verifies the exact recorded child PID plus endpoint. This is not external attachment or descriptor inheritance and is recorded on #1205 before code.

## Rollback

The command is additive and private-root-only. Revert it if it causes CI trouble; no product runtime, global process, or persistent state is mutated.
