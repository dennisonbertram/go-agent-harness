# Issue #21 Grooming: Research - Hashline Edits

## Summary
Research hashline edits (content-hash anchored line editing) from Pi/omp, which claims 6.7% → 68.3% reliability improvement by anchoring edit targets with content hashes instead of exact string matching.

## Evaluation
- **Clarity**: Clear — Research topic is well-framed with specific source (oh-my-pi repo), reported benchmark (6.7% → 68.3%), and context (string matching fails on whitespace/hallucinations).
- **Acceptance Criteria**: Present — Three explicit criteria: document algorithm + token cost, prototype plan, benchmark comparison vs current edit tool.
- **Scope**: Atomic — Pure research, no implementation required yet.
- **Blockers**: None — Research can proceed independently.
- **Effort**: Small — Primarily code reading + documentation. Benchmarking could expand scope but is optional (in criteria). Estimated 4-6 hours.

## Recommended Labels
research, well-specified, small

## Missing Clarifications
1. What "representative edit tasks" should benchmark compare? (e.g., existing test suite, or new synthetic tests?)
2. Should token cost analysis be per-file, per-edit, or aggregate for typical session?
3. Acceptable token overhead for the 68% reliability gain? (e.g., 15% overhead acceptable, 30% not?)
4. Does "compare to apply_patch" (#13) mean we should evaluate apply_patch as alternative, or hashline only?

## Notes
- Labeled as "research" in GitHub
- Source material available: oh-my-pi repo (public), Pi blog post cited
- Competitive context clear: identified as #1 adoption priority in harness-comparison-synthesis.md
- Integration assumption: "enhance existing edit tool" suggests backwards-compatible implementation
- Risk: If token overhead is high (>25%), may only be worth it for high-stakes edits
