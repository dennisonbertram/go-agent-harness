# GitHub Issues Created: Workspace & Symphd Tracks

Created on 2026-03-11.

## Labels Created

| Label | Color | Description |
|-------|-------|-------------|
| `workspace` | #0075ca | Workspace abstraction and implementations |
| `symphd` | #e4e669 | Symphony-style orchestrator daemon |
| `deferred` | #d93f0b | Planned but not current priority |

## Issues Summary

### Workspace Issues (active, label: workspace)

| Issue # | Title | URL |
|---------|-------|-----|
| #181 | `workspace: define Workspace interface and package scaffold` | https://github.com/dennisonbertram/go-agent-harness/issues/181 |
| #182 | `workspace: LocalWorkspace implementation (directory-based)` | https://github.com/dennisonbertram/go-agent-harness/issues/182 |
| #183 | `workspace: WorktreeWorkspace implementation (git worktree-based)` | https://github.com/dennisonbertram/go-agent-harness/issues/183 |
| #184 | `workspace: ContainerWorkspace implementation (Docker-based)` | https://github.com/dennisonbertram/go-agent-harness/issues/184 |
| #185 | `workspace: VMWorkspace implementation (cloud VM-based)` | https://github.com/dennisonbertram/go-agent-harness/issues/185 |
| #186 | `workspace: PoolWorkspace implementation (pre-provisioned pool)` | https://github.com/dennisonbertram/go-agent-harness/issues/186 |

### Symphd Issues (deferred, labels: symphd, deferred)

| Issue # | Title | URL |
|---------|-------|-----|
| #187 | `symphd: daemon scaffold and CLI` | https://github.com/dennisonbertram/go-agent-harness/issues/187 |
| #188 | `symphd: issue tracker client (GitHub Issues polling)` | https://github.com/dennisonbertram/go-agent-harness/issues/188 |
| #189 | `symphd: dispatcher (workspace provision + harness dispatch)` | https://github.com/dennisonbertram/go-agent-harness/issues/189 |
| #190 | `symphd: retry logic and exponential backoff` | https://github.com/dennisonbertram/go-agent-harness/issues/190 |
| #191 | `symphd: WORKFLOW.md configuration format` | https://github.com/dennisonbertram/go-agent-harness/issues/191 |

## Dependency Notes

- Workspace issues (#181-#186) should be implemented in order — #181 (interface) is a prerequisite for all others.
- Symphd issues (#187-#191) are blocked on the workspace abstraction (#181) and are deferred.
- The recommended implementation sequence for workspace: #181 → #182 → #183 → #184 → #185 → #186.
