# Training Package Implementation

**Date**: 2026-03-14
**Package**: `internal/training/`

## Files Created

| File | Purpose |
|------|---------|
| `types.go` | Shared types: TraceBundle, Finding, Message, Trainer interface, etc. |
| `exporter.go` | Reads rollout JSONL files and produces TraceBundle with computed metrics |
| `exporter_test.go` | 11 tests: basic run, failed run, efficiency, first-try rate, truncation, context snapshots, anti-patterns, messages, file-not-found, empty file, malformed lines |
| `scorer.go` | Structural scoring (ToolQuality, Efficiency, MaxContextRatio) without LLM |
| `scorer_test.go` | 7 tests: perfect run, anti-patterns, max penalty, efficiency scaling, zero tool calls, summary, context ratio |
| `trainer.go` | MockTrainer test double implementing Trainer interface |
| `trainer_test.go` | 5 tests: analyze, analyze error, analyze batch, batch error, interface compliance |
| `claude_trainer.go` | Claude opus-4-6 backend via direct HTTP to Anthropic Messages API |
| `claude_trainer_test.go` | 6 tests: analyze, HTTP error, malformed response, batch, context canceled, interface compliance |
| `storage.go` | SQLite storage (traces, findings, applied_changes, patterns tables) |
| `storage_test.go` | 7 tests: new/close, save/get trace, not found, findings, applied change, duplicate, concurrent access |

## Key Design Decisions

1. **Direct HTTP for Anthropic API** — No external SDK dependency. Uses `net/http` + JSON marshaling. Supports `WithBaseURL` option for test mock servers.

2. **SQLite concurrency** — Follows existing codebase pattern: `SetMaxOpenConns(1)` + `_txlock=immediate` DSN + `busy_timeout=5000` + WAL mode. Prevents SQLITE_BUSY under concurrent writes.

3. **JSONL parsing** — Resilient to malformed lines (skips them). Sorts by sequence number for deterministic processing.

4. **Retry detection** — Tool calls with identical name+args JSON are marked as retries. FirstTryRate = non-retried / total.

5. **Truncation** — When TokenCount > 180,000, drops middle messages keeping first 20% and last 30%. Inserts a `[N messages truncated]` system message.

6. **Scoring formulas**:
   - ToolQuality = FirstTryRate * (1 - min(1, antiPatterns/5))
   - Efficiency = 1.0 / (1.0 + steps*0.1 + costUSD*10) capped [0,1]

7. **ClaudeTrainer prompt** — Structured prompt with task details, system prompt, anti-patterns, tool call trace summary. Requests JSON output matching TrainerReport/BatchReport schemas.

## Test Results

```
36 tests, all passing
Race detector: clean
```

## Known Limitations

- Exporter only reads single JSONL files (no directory scanning yet — CLI can iterate)
- ClaudeTrainer does not implement streaming or retries on rate limits
- Storage does not implement pattern upsert (patterns table exists but no write method yet)
- Truncation token count is approximate (uses rollout token count, not re-estimated)
