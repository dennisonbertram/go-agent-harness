# Issue #8 Grooming: Workspace abstraction: transparent local/remote file and tool execution

## Summary
All tools are local-only. Proposes a workspace interface so tools can execute transparently against local filesystems, containers, or remote machines via SSH/proxy.

## Evaluation
- **Clarity**: Clear — the problem (local-only tools) and proposed solution (workspace interface) are well-explained
- **Acceptance Criteria**: Partial — specifies interface and implementations, but lacks acceptance criteria for completeness
- **Scope**: Very broad — involves workspace interface design, LocalWorkspace extraction, refactoring 30+ tools, container/SSH/proxy implementations, and security model
- **Blockers**: None
- **Effort**: large — multi-phase; Stage 1 (interface + local): medium; Stage 2 (remote implementations): large

## Recommended Labels
needs-clarification, large

## Missing Clarifications
1. Which tools need refactoring? (all 30+, or subset first?)
2. What's the MVP scope: local + container, or include SSH/proxy?
3. How to handle tool permissions (which directories/commands can remote agents access)?
4. Migration strategy: big-bang refactor all tools, or gradual?

## Notes
- WorkspaceProxy pattern is elegant and recommended
- Issue needs a phased implementation plan
- Security model (path traversal, auth, resource limits) is mentioned but needs detail
- Consider starting with LocalWorkspace extraction + ContainerWorkspace before tackling proxy pattern
