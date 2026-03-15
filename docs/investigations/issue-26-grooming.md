# Issue #26 Grooming: Research - Deep Git Tools

## Summary
Design deep git toolset (git_log_search, git_blame_context, git_evolution, git_regression_detect, git_contributor_context, git_change_patterns) for repo-wide historical understanding and semantic analysis. Integrate with observational memory to build persistent architectural knowledge.

## Evaluation
- **Clarity**: Clear — Problem statement (git treated as shallow utility), proposed tools (6 concrete tools with input/output), integration (memory flow), persona example, and competitive differentiation well-explained.
- **Acceptance Criteria**: Present — Five explicit criteria: tool interfaces + schemas, prototypes of 2 tools, benchmarks on varying repo sizes, memory integration design, persona package definition.
- **Scope**: Too broad — This is really 3 issues: (1) Research/design 6 git tools, (2) Implement git_log_search + git_evolution prototypes, (3) Design memory integration. Should split after research.
- **Blockers**: Related to memory system but not explicitly blocked.
- **Effort**: Large — Requires: tool design (input/output schemas), performance analysis on large repos, memory integration design, implementation of prototypes, benchmarking. 12-20 hours.

## Recommended Labels
research, needs-clarification, large

## Missing Clarifications
1. git_log_search: what's the search mechanism? Regex on commit messages? Semantic embedding search? Heuristic keywords?
2. Performance baseline: should research target performance on repos like Linux kernel (1M commits) or typical projects (10k)?
3. Memory integration: should git_evolution automatically trigger when file is touched, or only on-demand?
4. Should git_change_patterns pre-compute coupling matrix on first run, or compute on-demand?
5. For git_regression_detect: what does "semantic git bisect" mean? Comparing LLM behavior predictions?
6. Persona package scope: just "git_historian" or broader "repository_archaeology"?

## Notes
- Labeled as "research" in GitHub
- Positioning: "biggest differentiation opportunity" — significant strategic importance
- Competitive context: other harnesses (Codex, Pi) treat git shallowly; this is novel
- Tool interdependencies: git_evolution informs architectural knowledge, git_change_patterns informs coupling patterns
- Integration assumption: tools are deferred (#4) — loaded on-demand
- Scaling concern: git_change_patterns has N² potential comparisons; must address
- Memory interaction: how to avoid "forgotten history" when memory compacts?
- MSR (Mining Software Repositories) is well-researched academic field; many techniques available

## Performance Risks
1. Large repos: semantic search on 100k+ commits may be slow
2. Blame context: fetching PR/issue context requires GitHub API integration
3. Change patterns: computing correlation matrix is expensive; caching required
4. Regression detect: bisecting semantically may require dozens of LLM calls

## Related Issues
- #17 (conversation compaction — memory coordination)
- #4 (deferred tools — git tools should be deferred by default)
- Observational memory system (integration point)

## Implementation Dependencies
- #7 (persistence layer — store git analysis results)
- GitHub API integration (for PR/issue linking in blame_context)
