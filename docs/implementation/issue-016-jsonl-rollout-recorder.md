# Issue #16: JSONL Rollout Recorder

## Summary

Implemented a JSONL rollout recorder that captures the complete event timeline of every run to a date-partitioned file. Enables session replay, audit, and future fork support.

## Files Changed

- `internal/rollout/recorder.go` — new package: `Recorder` type, `RecorderConfig`, `RecordableEvent`
- `internal/rollout/recorder_test.go` — unit tests for the recorder
- `internal/rollout/integration_test.go` — integration tests verifying runner produces JSONL files
- `internal/harness/runner.go` — imports `rollout` package; creates recorder per run in `StartRun`/`ContinueRun`; hooks `Record` into `emit`; closes recorder after terminal events
- `internal/harness/types.go` — added `RolloutDir string` field to `RunnerConfig`
- `cmd/harnessd/main.go` — reads `HARNESS_ROLLOUT_DIR` env var; passes `RolloutDir` to `RunnerConfig`

## Design

### Storage layout

```
<HARNESS_ROLLOUT_DIR>/
└── <YYYY-MM-DD>/
    ├── run_1.jsonl
    ├── run_2.jsonl
    └── ...
```

Each line in a `.jsonl` file is a JSON object:

```jsonl
{"ts":"2026-03-09T14:30:00Z","seq":0,"type":"run.started","data":{"prompt":"hello"}}
{"ts":"2026-03-09T14:30:01Z","seq":1,"type":"llm.turn.requested","data":{"step":1}}
{"ts":"2026-03-09T14:30:02Z","seq":2,"type":"run.completed","data":{"output":"done"}}
```

### Key design decisions

1. **Local type `RecordableEvent`** — avoids a circular import between `rollout` and `harness`. The runner converts `harness.Event` to `rollout.RecordableEvent` in the `emit` method.

2. **Opt-in via `RolloutDir`** — leaving `RolloutDir` empty disables recording entirely with zero overhead.

3. **Close on terminal event** — the recorder is closed immediately after `run.completed` or `run.failed` is recorded, flushing the file without waiting for GC.

4. **Errors silently dropped** — recorder failures never crash or block the run loop. Creation errors are logged; per-event encoding errors are silently ignored.

5. **Sequence counter managed by recorder** — the `seq` field is assigned by the `Recorder` (not copied from the event ID) to ensure a clean 0-based monotonic sequence per file.

## Environment Variable

| Variable | Default | Description |
|---|---|---|
| `HARNESS_ROLLOUT_DIR` | (empty, disabled) | Root directory for rollout JSONL files |

## Tests

- `TestRecorder_BasicWrite` — verifies correct JSONL format and field values
- `TestRecorder_FileLayout` — verifies `<dir>/<YYYY-MM-DD>/<run_id>.jsonl` path
- `TestRecorder_Seq` — verifies 0-based monotonic seq numbering
- `TestRecorder_Concurrent` — 20 goroutines × 50 events with `-race`
- `TestRecorder_NilPayload` — graceful handling of nil payload
- `TestRecorder_CloseIdempotent` — double-close is safe
- `TestNewRecorder_EmptyDir` — error on empty Dir
- `TestNewRecorder_EmptyRunID` — error on empty RunID
- `TestRunnerRollout_RunProducesJSONL` — integration: runner with `RolloutDir` set produces a parseable JSONL file with `run.started` and terminal events, correct `seq`, valid `ts`
- `TestRunnerRollout_Disabled` — integration: runner without `RolloutDir` completes without error

All 10 tests pass with `-race`.

## Status: DONE
