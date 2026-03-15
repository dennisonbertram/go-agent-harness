# Applier & Regression Guard Implementation

**Date**: 2026-03-14
**Package**: `internal/training/`

## Files Created

| File | Purpose |
|------|---------|
| `applier.go` | Applies high-confidence findings to system prompts / tool descriptions via git branches |
| `applier_test.go` | 14 tests: canAutoApply logic (6 cases), dry-run, all-skipped, system_prompt apply, tool_description apply, missing tool desc, git branch integration, store integration, defaults |
| `regression.go` | Runs benchmarks and decides merge/revert/flag based on configurable thresholds |
| `regression_test.go` | 9 tests: LoadBaseline (3 cases), decision logic (9 table-driven sub-tests), RunBenchmark placeholder, Revert, Merge, Check mock, config defaults |

## Key Design Decisions

1. **Strict `<` for accuracy drop threshold** -- AccuracyDelta must be strictly less than the threshold to trigger revert. Exactly at threshold is not a revert.

2. **Relative deltas for cost/step** -- Cost and step changes are computed as relative changes `(candidate - baseline) / baseline`, not absolute. This means a 15% cost rise threshold works proportionally regardless of baseline cost.

3. **Decision priority** -- Revert (accuracy drop) takes priority over flag (cost/step rise). If both conditions are met, revert wins.

4. **Placeholder benchmark mode** -- When `BenchmarkCmd` is empty and `HARNESS_BENCHMARK_CMD` env var is not set, RunBenchmark returns a zero-value result. This allows the system to work without a real benchmark suite.

5. **Branch naming** -- Training branches use format `training/auto-YYYY-MM-DD-{8-char-hash}` for uniqueness and traceability.

6. **Auto-apply rules** (canAutoApply):
   - Confidence must be CERTAIN
   - EvidenceCount >= MinEvidenceCount (default 3)
   - Priority must NOT be "critical" (requires human review)
   - Type must be "system_prompt" or "tool_description" (not "behavior")

7. **File operations**:
   - System prompt findings: creates new file at `prompts/behaviors/training-{target}-{timestamp}.md`
   - Tool description findings: appends to `internal/harness/tools/descriptions/{target}.md` with `<!-- training: {timestamp} -->` marker
   - Tool description target must already exist (error if missing)

8. **Store integration** -- SaveAppliedChange is best-effort (doesn't fail the operation). FindingID is 0 since we don't have row-level IDs from the batch.

## Test Results

```
59 tests total (36 existing + 23 new), all passing
Race detector: clean
```

## Baseline.json Format

The regression guard loads `benchmarks/terminal_bench/baseline.json` and computes:
- PassRate: fraction of tasks with `expected_pass: true`
- AvgCostUSD: mean of all tasks' `avg_cost_usd`
- AvgSteps: mean of all tasks' `avg_steps`
- TaskResults: map of task_id -> expected_pass
