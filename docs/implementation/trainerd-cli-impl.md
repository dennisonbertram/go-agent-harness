# trainerd CLI Implementation

**Date**: 2026-03-14
**Package**: `cmd/trainerd/`

## Files Created

| File | Purpose |
|------|---------|
| `cmd/trainerd/main.go` | Cobra root command with persistent flags (--db-path, --log-level) |
| `cmd/trainerd/commands.go` | All 5 subcommand implementations: score, analyze, loop, status, history |
| `cmd/trainerd/commands_test.go` | 23 tests covering all commands, helpers, and edge cases |

## Files Modified

| File | Change |
|------|--------|
| `internal/training/storage.go` | Added `AppliedChange` type, `CountTraces()`, `CountFindings()`, `CountAppliedChanges()`, `QueryHistory()` methods |
| `go.mod` / `go.sum` | Added `github.com/spf13/cobra` v1.10.2 dependency |

## Command Structure

```
trainerd
├── score       --run-id <id> --rollout-dir <dir>
├── analyze     --run-ids r1,r2 --rollout-dir <dir> --output-format text|json
├── loop        --task-set all|go|python --trainer claude-opus --dry-run
├── status      (shows trace/finding/change counts)
└── history     --since 2006-01-02 (shows applied changes)
```

## Test Results

```
23 tests, all passing
Race detector: clean
Build: clean
```

## Test Coverage

- score: basic run, missing file
- status: populated DB, empty DB
- history: no changes, with changes, invalid date format
- loop: dry-run, no files, save to store, task-set filter
- helpers: findRolloutFile (dated dir, flat dir, not found), findAllRolloutFiles (populated, empty)
- printFindings: text, json, empty
- root: help, score help
- Store methods: counts (empty + populated), QueryHistory (empty, populated, future date filter)

## Design Decisions

1. **Cobra for CLI** — Standard Go CLI framework, matches typical Go project patterns.

2. **Rollout file discovery** — Two-strategy lookup: first `<dir>/*/<runID>.jsonl` (dated subdirectory pattern used by rollout.Recorder), then flat `<dir>/<runID>.jsonl`.

3. **Store API extension** — Added count and query methods to `Store` rather than opening a second DB connection, keeping the single-writer pattern.

4. **Graceful degradation** — `loop` command works without ANTHROPIC_API_KEY (scores only, skips Claude analysis). `analyze` requires the key.

5. **Output formats** — Text (default) uses aligned columns; JSON uses indented encoder for pipe-friendly output.
