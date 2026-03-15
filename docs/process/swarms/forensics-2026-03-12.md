# Swarm: Forensics Traceability Foundation

**Started**: 2026-03-12
**Scope**: `internal/harness/**,internal/rollout/**,internal/provider/**`
**Spec**: `docs/investigations/forensics-impl-spec.md`
**GitHub Issues**: #207, #208, #209, #210, #213, #214, #215, #217, #218, #219, #220

## Wave Plan

### Wave 1 — Foundation (sequential: both touch runner.go)
- Task 1: **#217** — Event schema versioning + correlation IDs
- Task 2: **#220** — Timing/duration metrics (blocked by #217)

### Wave 2 — Core Tracing (parallel after Wave 1 merges)
- Task 3: **#208** — Tool selection + hook mutation tracing
- Task 4: **#218** — LLM request envelope capture
- Task 5: **#219** — PII/secret redaction pipeline

### Wave 3 — Analysis Layer (parallel after Wave 2 merges)
- Task 6: **#207** — Reasoning/thinking block capture
- Task 7: **#209** — Context window snapshots (best-effort)
- Task 8: **#210** — Error chain tracing

---

## Wave 1 — Foundation

**Teammates**: TBD
**Files changed**:
**Codex tasks**:
**Review**:
**Findings fixed**:
**Commit**:

## Wave 2 — Core Tracing

**Teammates**: TBD
**Files changed**:
**Review**:
**Commit**:

## Wave 3 — Analysis Layer

**Teammates**: TBD
**Files changed**:
**Review**:
**Commit**:

---

## Ralph Loop

**Pass 1 (Adversarial)**:
**Pass 2 (Skeptical User)**:
**Pass 3 (Correctness)**:
**Result**:

## Final Status

- [ ] 3 consecutive clean passes
- [ ] All tests passing (`./scripts/test-regression.sh`)
- [ ] Lint + build clean
- [ ] Committed and pushed
