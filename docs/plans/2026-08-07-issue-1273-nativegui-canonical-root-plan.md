# Plan: Issue #1273 native GUI artifact-root canonicalization

## Context

- Governing GitHub issue: #1273.
- Problem: native GUI proof sealing compared a lexical artifact root with
  canonical artifact paths. macOS parent aliases such as `/var` and
  `/private/var` falsely rejected owned regular fixture files.
- User impact: the required repository regression gate failed nine native GUI
  proof tests, blocking unrelated delivery.
- Constraints: do not weaken containment, symlink, digest, typed-signal, or
  correlation validation.

## Scope

- In scope: canonicalize the artifact root once in `SealArtifacts`, use that
  value for containment/relative-path conversion, and persist it in the proof.
- Out of scope: rendered app behavior, native lifecycle, proof schema changes,
  accepting artifact or final-root symlinks, and broad fixture rewrites.

## Documentation Contract

- Feature status: `implemented` locally, pending review/merge.
- Public docs affected: none; this is acceptance-proof infrastructure.
- Spec/docs: this plan, impact map, durable logs, and indexes.

## Test Plan (TDD)

- New expected red: owned artifacts addressed through a symlinked parent must
  seal and validate, with the root persisted in canonical form.
- Negative regression: a final artifact-root symlink remains rejected; existing
  artifact symlink, escape, digest, kind, and correlation tests remain.
- Commands: focused nativegui normal/race and full regression in tmux.

## Cross-Surface Impact Map

- See `2026-08-07-issue-1273-nativegui-canonical-root-impact-map.md`.

## Implementation Checklist

- [x] Confirm #1273 and reproduce baseline failure.
- [x] Add portable alias-root red regression.
- [x] Canonicalize only the root boundary and retain final-symlink rejection.
- [x] Run focused normal/race checks.
- [ ] Run full regression; commit, push, and open closing PR.

## Risks and Mitigations

- Risk: canonicalization admits attacker-controlled final symlink roots.
  Mitigation: reuse `canonicalDirectory`, which lstat-rejects final symlinks.
- Risk: task broadens into native UI behavior. Mitigation: package-only proof
  sealing/test repair with no application or lifecycle edits.
