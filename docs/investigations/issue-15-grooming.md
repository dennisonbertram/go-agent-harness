# Issue #15 Grooming: Implement two-axis permission model: sandbox scope × approval policy

## Summary
Current harness has flat binary permission model. Proposal introduces two independent axes: (1) Sandbox axis — what CAN the agent do (filesystem scope, network access), and (2) Approval axis — what SHOULD the agent ask about (everything, just destructive ops, nothing).

## Already Addressed?
**NOT ADDRESSED** — Current model is binary (`ApprovalModeFullAuto` / `ApprovalModePermissions`). No sandbox layer exists. No path-scoping for bash/write tools. No network origin whitelist for fetch tool.

## Clarity Assessment
Excellent — concrete design with enumerated sandbox levels (`none`, `workspace-only`, `read-only`, `network-deny`) and approval levels (`all`, `mutating`, `destructive`, `none`). Codex CLI cited as reference.

## Acceptance Criteria
- `SandboxPolicy` defines what agent CAN do (filesystem, network scope)
- `ApprovalPolicy` defines what agent MUST ask about
- Policies compose independently (3×3 matrix)
- Two-attempt execution: try sandboxed, escalate on denial
- Approval caching prevents repeated questions
- macOS Seatbelt or Linux Bubblewrap enforcement
- `RunRequest` accepts `sandbox` and `approval_policy` config
- SSE events for approval requests/grants

## Scope
Large — architectural change touching runner, all tools, HTTP API, and OS-level sandboxing.

## Blockers
None hard. Requires Seatbelt/Bubblewrap availability per platform.

## Effort
**Large** (20-40h) — suggest phased: Phase 1 (ApprovalPolicy enum), Phase 2 (SandboxPolicy enforcement), Phase 3 (two-attempt pattern + caching).

## Label Recommendations
Current: none. Recommended: `enhancement`, `security`, `architecture`

## Recommendation
**well-specified** — Excellent design. Recommend breaking into sub-issues for phased rollout.
