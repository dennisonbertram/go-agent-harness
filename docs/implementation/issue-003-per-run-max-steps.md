# Issue #3: Make max_steps Tunable Per-Run, Default to Unlimited

## Summary

Implemented per-run `max_steps` override for `POST /v1/runs` requests, and changed the runner default from a hard-coded 8-step cap to unlimited (0 = no limit).

## Changes

### `internal/harness/types.go`
- Added `MaxSteps int` field to `RunRequest` with JSON tag `max_steps,omitempty`
- Value semantics: `0` = use runner config default; positive integer = per-run cap; negative = rejected at `StartRun`

### `internal/harness/runner.go`
- `NewRunner`: Removed the `if config.MaxSteps <= 0 { config.MaxSteps = 8 }` default. `MaxSteps == 0` in config now means unlimited.
- `StartRun`: Added validation — returns error if `req.MaxSteps < 0`
- `execute`: Computes `effectiveMaxSteps` by combining per-run and config values:
  - If `req.MaxSteps > 0`: use per-run limit
  - Otherwise: use `config.MaxSteps` (which may be 0 = unlimited)
  - Loop condition: `effectiveMaxSteps == 0 || step <= effectiveMaxSteps`
- Error message now uses `effectiveMaxSteps` so callers see the actual limit that was applied

## Behavior

| `config.MaxSteps` | `req.MaxSteps` | Effective limit |
|---|---|---|
| 0 | 0 | unlimited |
| 8 | 0 | 8 |
| 8 | 20 | 20 |
| 0 | 5 | 5 |
| 10 | 3 | 3 |

## Tests Added (`internal/harness/runner_test.go`)

- `TestPerRunMaxSteps_OverridesConfig` — per-run limit of 3 takes precedence over config limit of 10
- `TestPerRunMaxSteps_ZeroFallsBackToConfig` — per-run MaxSteps=0 uses config limit (2)
- `TestConfigMaxSteps_ZeroMeansUnlimited` — config MaxSteps=0 allows run to complete naturally
- `TestPerRunMaxSteps_NegativeIsInvalid` — negative MaxSteps rejected at StartRun with descriptive error

## Test Results

All tests pass (excluding pre-existing `demo-cli` build failure):

```
ok  go-agent-harness/internal/harness   2.406s
ok  go-agent-harness/internal/server    1.986s
... (all other packages pass)
```

Race detector: clean.

## HTTP API

Callers can now pass `max_steps` in the run request body:

```json
{
  "prompt": "Refactor the authentication module",
  "max_steps": 50
}
```

Omitting `max_steps` (or setting it to `0`) uses the runner's configured default (which defaults to unlimited when `HARNESS_MAX_STEPS` is not set or is `0`).
