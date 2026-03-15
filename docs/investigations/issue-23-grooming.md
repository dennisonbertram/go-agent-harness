# Issue #23 Grooming: Research - OS-Level Sandboxing

## Summary
Research OS-level process sandboxing (Apple Seatbelt on macOS, Landlock/seccomp on Linux, AppContainer on Windows) as implemented by Codex CLI. Contrasts with current application-level security (tool blocklist + approval mode).

## Evaluation
- **Clarity**: Clear — Problem (running untrusted code needs kernel-level isolation), example implementations (Seatbelt, Landlock, AppContainer), and integration hint (two-axis permission model #15) provided.
- **Acceptance Criteria**: Present — Four explicit criteria: document mechanisms + Go compatibility, assess overhead, design proposal, identify MVP.
- **Scope**: Atomic — Research only, no implementation required yet.
- **Blockers**: Related to #15 (two-axis permission model) but not explicitly blocked.
- **Effort**: Medium — Requires platform-specific research (3 OSes), kernel API study, Go feasibility assessment. Could involve proof-of-concept. 6-10 hours.

## Recommended Labels
research, well-specified, medium

## Missing Clarifications
1. For Go compatibility assessment: acceptable to use cgo for platform-specific code, or native Go only?
2. Performance overhead — what's acceptable? Per-tool overhead or per-run setup amortized?
3. Is MVP single-platform (Linux via Landlock) or multi-platform?
4. Interaction with MCP tools (#19) — should sandboxing apply to MCP-invoked executables?
5. Deprecated Seatbelt on Apple — should research target current/future Apple APIs instead?

## Notes
- Labeled as "research" in GitHub
- Competitive context: Codex's sandbox is noted as enterprise requirement
- Integration assumption: compose with existing permission model (#15), not replace
- Security: Significant hardening opportunity for multi-tenant scenarios (1000 agent instances)
- Complexity: Landlock requires kernel 5.13+; Seatbelt replacement path unclear; Windows AppContainer is well-documented
- Risk: High implementation complexity; Go ecosystem may lack good Landlock bindings

## Related Issues
- #15 (two-axis permission model — approval_policy + sandbox_scope)
- #19 (MCP — sandboxing MCP-invoked tools)
