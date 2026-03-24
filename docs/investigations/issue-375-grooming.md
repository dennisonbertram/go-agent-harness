# Grooming: Issue #375 — fix(agents): make spawn_agent honor its declared profile/model contract

## Already Addressed?

Partial — `spawn_agent` declares `profile` and `model` parameters in its JSON schema and parses them from the request, but neither is forwarded to the child runner. Specifically:

- `internal/harness/tools/deferred/spawn_agent.go` lines 38–53 declare both `model` and `profile` as optional schema fields.
- Lines 62–66 parse them into the handler struct (`args.Model`, `args.Profile`).
- Lines 110–113 build the `ForkConfig` forwarded to `RunForkedSkill` — `Profile` and `Model` are not included. Only `AllowedTools` is forwarded.
- The `ForkConfig` type in `internal/harness/tools/types.go` has no `Profile` or `Model` field.
- `max_steps` is also accepted but the parsed value is embedded into the system prompt string only, not forwarded as a runner constraint.
- By contrast, `run_agent.go` fully honors all three parameters: it loads the named profile, applies its defaults, and allows per-call overrides for `model` and `max_steps` (lines 66–107).
- No existing tests assert that `profile` or `model` inputs affect child run behavior in `spawn_agent`.

The contract violation is clear and documented in the plan file. The fix work is not yet done.

## Clarity

Clear — The plan specifies two acceptable outcomes: (1) make `spawn_agent` a thin wrapper over the profile-aware delegation path, or (2) narrow `spawn_agent` to a lower-level primitive that removes or explicitly rejects the dead `profile` and `model` parameters and corrects the schema/docs. Both outcomes are well-understood. The implementer must choose one and pin it with regression tests.

## Acceptance Criteria

Present in the plan file. Explicit criteria:

1. A failing test is written first proving that `profile` and `model` inputs are currently ignored by `spawn_agent`.
2. A decision is made and documented: converge `spawn_agent` onto the profile path OR narrow its contract and remove the dead parameters.
3. Implementation is delivered in accordance with the chosen contract.
4. Regression tests pin the chosen contract so future changes cannot silently reintroduce dead parameters.
5. No existing delegation behavior changes beyond the targeted fix.

## Scope

Atomic — Changes are bounded to `spawn_agent.go`, the `ForkConfig` struct in `types.go` (if the widening path is chosen), the existing test file, and possibly `ForkedAgentRunner` in `runner.go`. No HTTP, TUI, or profile-loader changes are required. This is a single-concern fix.

## Blockers

None — explicitly listed as having no dependencies in the plan.

## Recommended Labels

well-specified, small

## Effort

Small — The code change is 30–60 lines. The primary cost is the design decision (converge vs. narrow), not the implementation itself.

## Recommendation

well-specified

## Notes

- `spawn_agent` uses `ForkedAgentRunner.RunForkedSkill(ctx, ForkConfig{...})`. If the profile path is chosen, either `ForkConfig` gains `Profile` and `Model` fields and the runner impl consumes them, or `spawn_agent` is refactored to also use the `SubagentManager` path (matching `run_agent`).
- The simpler outcome for now is the narrowing path: strip `profile` and `model` from the `spawn_agent` schema, update the description to say it is a lower-level forked primitive, and document that `run_agent` is the profile-backed tool. This avoids expanding `ForkConfig` and keeps the two tools with distinct roles.
- The `mockSpawnForkedRunner` in `spawn_agent_test.go` captures `lastConfig`, making it straightforward to add an assertion that `Profile` and `Model` fields are or are not present in the `ForkConfig`.
- The plan notes this ticket should be resolved before #383 (structured result contract), as both touch `spawn_agent` output shape.
