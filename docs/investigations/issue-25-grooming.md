# Issue #25 Grooming: Research - Role-Based Model Routing

## Summary
Research role-based model routing from omp: use different LLMs for different tasks within a session (cheap models for summarization/memory, strong models for reasoning). Potential 3-5x cost reduction at scale.

## Evaluation
- **Clarity**: Clear — Problem (all tasks don't need the strongest model), examples (default/smol/slow/plan/commit roles), cost impact (3-5x reduction), integration hint (via persona YAML) well-explained.
- **Acceptance Criteria**: Present — Four explicit criteria: document omp's mechanism, design proposal, cost modeling, YAML schema.
- **Scope**: Atomic — Research only, no implementation required yet.
- **Blockers**: Depends on #11 (multi-provider) for full implementation, but research can proceed.
- **Effort**: Small-Medium — Primarily code reading (omp), cost modeling, YAML schema design. Proof-of-concept of cost calculator might add time. 4-6 hours.

## Recommended Labels
research, well-specified, small

## Missing Clarifications
1. How does omp decide which role to use for a task? Automatic (heuristic) or LLM self-selection or explicit routing?
2. Should roles be global (project-level) or per-intent (git_historian uses different roles than code_writer)?
3. "Shared context or separate windows" — should research recommend one model?
4. Token overhead of switching models: is there context redundancy when role changes?
5. For cost modeling: which models should research use for each role? (e.g., GPT-5-nano for smol, Claude Opus for slow?)

## Notes
- Labeled as "research" in GitHub
- Related issues: #11 (multi-provider), #10 (cost ceiling), #4 (deferred tools philosophy)
- Integration point: YAML intent schema already supports persona config; roles could extend naturally
- Competitive advantage: cost-optimized multi-model orchestration is relatively novel; most harnesses use one model per run
- Existing precedent: observational memory already uses cheap model (gpt-5-nano) — extends that idea
- Risk: Role selection heuristics may be noisy; careful tuning required to avoid quality degradation on critical tasks
- Scaling assumption: needs multi-provider support (#11) fully working first

## Implementation Dependencies
- #11 (multi-provider — must support switching models)
- #20 (config layer — roles could be configured per-profile or per-intent)
