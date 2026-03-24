# Grooming: Issue #391 — feat(cli): add profile selection and inspection to harness CLI and TUI

## Already Addressed?

No — Profile selection and inspection are absent from both the CLI and TUI.

CLI (`cmd/harnesscli/main.go`):
- The `runCreateRequest` struct (line 63-71) already has a `PromptProfile string` field and the `-prompt-profile` flag (line 130) is wired. This means the CLI can already send a `prompt_profile` value in the run request.
- However, there is no profile discovery capability: no way to list available profiles, no `-list-profiles` flag, no profile inspection before submitting a run.
- The `PromptProfile` field is undocumented in the CLI help and its semantics (what profiles exist, what they do) are opaque to the user.

TUI (`cmd/harnesscli/tui/`):
- No profile-related components exist anywhere in the TUI directory. A search for "profile" across all TUI Go files returns no matches.
- The existing `configpanel` component shows key/value config entries (model, base URL, etc.) but has no profile selection or inspection.
- The `sessionpicker` component exists for session selection but no equivalent `profilepicker` component exists.
- The TUI sends run requests through the bridge but has no affordance for choosing a profile before submitting.

The `PromptProfile` field in the CLI run request is a prerequisite that is already present (from earlier work). What is missing is the user-facing discovery and selection layer.

## Clarity

Clear — The issue scope is well-defined: read-only profile UX, covering CLI flag-level and TUI affordances. The "start with read-only" constraint is explicit and appropriate. Related files are correct.

## Acceptance Criteria

Partial — The issue mentions "profile selection and inspection affordances" and "start with read-only UX" but does not specify:
- What "inspection" looks like (e.g., a `/profiles` slash command in TUI, a `--list-profiles` CLI flag, or a profile detail view)
- What the TUI affordance for selecting a profile is (a picker overlay like `sessionpicker`? a configpanel entry?)
- Whether "selection" means setting the profile for the next run, or setting it as a persistent default
- Whether the CLI needs a `-list-profiles` flag that calls a server endpoint, or just better documentation of existing `-prompt-profile`

Suggested acceptance criteria for clarification:
1. `harnesscli --list-profiles` (or equivalent) prints available profiles from the server
2. TUI has a profile picker overlay (invoked via slash command or keybind) showing available profiles with descriptions
3. Selecting a profile in TUI sets it for the next run submission
4. Both surfaces are read-only (no profile creation/editing)

## Scope

Potentially too broad — The issue combines CLI changes and TUI changes. These are separable surfaces. However, both are read-only UX (no server-side changes), and the "start with read-only" constraint keeps it from expanding. The scope is medium but coherent.

This issue is blocked on #377 (add `list_profiles` and `get_profile` server surfaces) since there is no endpoint to discover profiles from. The CLI `-prompt-profile` flag already works for known names, but profile listing requires the server endpoint.

## Blockers

Blocked on #377 — `list_profiles` HTTP endpoint must exist before the CLI or TUI can discover profiles. Without this endpoint:
- The CLI can still document the existing `-prompt-profile` flag and its semantics
- The TUI cannot show a profile picker (nothing to list)

Partially blocked on #381 (tool manifest introspection) if the inspection affordance includes showing what tools a profile enables.

## Recommended Labels

needs-clarification, blocked, medium

## Effort

Medium — Estimated 3-5 days. Requires:
- A new `profilepicker` TUI component (modeled on `sessionpicker`)
- CLI flag/command for profile listing
- Integration with the server's profile list endpoint (once #377 lands)
- Tests for both CLI request shape and TUI interaction

## Recommendation

needs-clarification — The issue is directionally clear but lacks specific acceptance criteria for what "inspection" and "selection" look like in each surface. Also blocked on #377 for profile discovery. The existing `PromptProfile` CLI flag is already present, so the actual work is the discovery/listing layer and TUI picker. Should be clarified to specify: (1) exactly what CLI commands/flags are added, (2) what the TUI affordance looks like (keybind? slash command? profile picker overlay), and (3) whether TUI profile selection persists across runs in a session.

## Notes

- `cmd/harnesscli/main.go` line 69: `PromptProfile string` field already exists in `runCreateRequest`
- `cmd/harnesscli/main.go` line 130: `-prompt-profile` flag already wired — so sending a known profile name already works
- No profile-related TUI components exist in `cmd/harnesscli/tui/components/`
- The `sessionpicker` component (`components/sessionpicker/`) is the closest architectural analog to what a `profilepicker` would look like
- The `configpanel` component could potentially host a profile selector as a read-only entry, but a dedicated picker is more discoverable
- The plan document (`docs/plans/2026-03-20-profile-subagent-backlog-plan.md`) places #391 after #377 (profile discovery) and #381 (tool manifest) in the delivery order
