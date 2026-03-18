# Issue #42: S3 JSONL Backup Streaming on Run Completion

## Summary

Adds event-driven JSONL backup to S3 for conversation/run persistence. When a run completes (success or failure), all run events are serialized to JSONL and uploaded to an S3 bucket via a background goroutine. The upload is non-blocking and non-fatal — S3 errors are logged but never impact run execution.

## What Was Built

### New Package: `internal/store/s3backup/`

**File:** `/Users/dennisonbertram/Develop/go-agent-harness/.claude/worktrees/agent-a92b6c02/internal/store/s3backup/s3backup.go`

Key exports:

- `Config` — holds bucket, prefix, region, credentials, optional endpoint override
- `ConfigFromEnv(getenv)` — reads config from env vars; returns `(Config, bool)` (false = absent = silent skip)
- `RunUploader` — interface with `UploadRun(ctx, store, convID, runID) error`
- `Uploader` — concrete S3 implementation using AWS Signature Version 4 (no SDK dependency)
- `NoOpUploader` — no-op implementation returned when config is absent
- `BuildJSONL(ctx, store, runID)` — assembles the JSONL payload from the store

**JSONL format:** first line is a `"type":"run"` header record with run metadata; subsequent lines are `"type":"event"` records in ascending seq order.

**S3 key format:** `{prefix}/{conversation_id}/{run_id}.jsonl`

### Runner Changes

**File:** `/Users/dennisonbertram/Develop/go-agent-harness/.claude/worktrees/agent-a92b6c02/internal/harness/types.go`

Added field to `RunnerConfig`:
```go
S3Uploader s3backup.RunUploader `json:"-"`
```

**File:** `/Users/dennisonbertram/Develop/go-agent-harness/.claude/worktrees/agent-a92b6c02/internal/harness/runner.go`

Added `backupRunToS3(runID)` helper that:
1. Reads `conversationID` from run state under the lock
2. Fires a goroutine with a 60-second timeout to call `S3Uploader.UploadRun`
3. Logs errors non-fatally (never blocks or panics)

Called from:
- `completeRun()` — after `run.completed` event is emitted
- `failRun()` — after `run.failed` event is emitted
- `failRunMaxSteps()` — after `run.failed` (max steps) event is emitted

### Production Wiring

**File:** `/Users/dennisonbertram/Develop/go-agent-harness/.claude/worktrees/agent-a92b6c02/cmd/harnessd/main.go`

1. **Run state store** (`HARNESS_RUN_DB`): creates a `store.SQLiteStore` and wires it to both `RunnerConfig.Store` and `ServerOptions.Store`. Previously the runner Store was nil in production.

2. **S3 uploader**: calls `s3backup.ConfigFromEnv(getenv)` and wires either a real `Uploader` or `NoOpUploader` depending on env var presence.

## Configuration

| Env Var | Required | Description |
|---------|----------|-------------|
| `AWS_ACCESS_KEY_ID` | Yes (for S3) | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | Yes (for S3) | AWS secret key |
| `AWS_REGION` | Yes (for S3) | AWS region (e.g. `us-east-1`) |
| `S3_BUCKET` | Yes (for S3) | Target S3 bucket name |
| `S3_KEY_PREFIX` | No | Key prefix (default: empty) |
| `HARNESS_RUN_DB` | No | SQLite path for run/event persistence; required for S3 backup to have data to upload |

If any of the required S3 env vars are absent, the uploader is a silent no-op.

## TDD Process

Tests were written first; implementation followed until all tests passed.

### Unit tests: `internal/store/s3backup/s3backup_test.go`

17 tests covering:
- `ConfigFromEnv` with all vars present, missing bucket, missing credentials, empty prefix
- `ObjectKey` with and without prefix
- `BuildJSONL` correctness, run-not-found error, event ordering, run header fields
- `Uploader.UploadRun` success with fake S3 server, S3 error propagation, run-not-found
- `NoOpUploader` returns nil without calling anything
- Content-Type header is set
- Concurrent upload safety
- Interface compliance compile-time check

### Integration tests: `internal/harness/runner_s3backup_test.go`

5 tests covering:
- S3 PUT fires on run completion with valid JSONL body
- No-op when `S3Uploader` is nil in `RunnerConfig`
- No-op when `NewNoOpUploader()` is explicitly set
- S3 PUT fires on run failure
- Bucket name appears in PUT URL

## Test Results

- All 17 unit tests in `internal/store/s3backup/` pass
- All 5 runner integration tests pass
- Full regression suite: **PASS** (85.2% coverage, 0 zero-coverage functions)

## Design Decisions

1. **No AWS SDK**: uses `crypto/hmac` + `net/http` to implement AWS Sig V4 manually. This avoids adding a large dependency to `go.mod`.

2. **Event-driven, not periodic**: upload fires exactly once per terminal event (`run.completed`, `run.failed`, `run.failed` max steps). The issue spec confirms this.

3. **Background goroutine**: upload is off the hot path — the terminal event emitter returns immediately; a goroutine handles the S3 PUT with a 60s timeout.

4. **Non-fatal**: S3 errors are logged via `RunnerConfig.Logger` but never surface as run failures. Operators can monitor log output for backup health.

5. **Idempotent key**: re-uploading the same run overwrites the existing S3 object (S3 PutObject is idempotent by key). No deduplication logic needed.

6. **Store dependency**: `BuildJSONL` reads from `store.Store` (events + run record). The store must be populated before the backup fires — this is ensured by wiring `RunnerConfig.Store` so the runner persists events incrementally during execution.
