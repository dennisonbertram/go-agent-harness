# Overnight Task Benchmarks

Task files used by `scripts/overnight-training.sh` to drive the overnight training loop.

## Format

Each file is a plain-text table of tasks. Lines beginning with `#` are comments and are skipped.
Each task line uses pipe (`|`) as a separator:

```
task_name|prompt text
```

- **task_name**: Short identifier, no spaces or pipes. Used in logging and rollout filenames.
- **prompt**: The full prompt sent to the harness agent. No newlines. Keep under 500 chars.
  Every prompt must include: working directory, `go mod init`, implementation requirements,
  test file requirements, and the command `go test ./...` as the acceptance criterion.

## Difficulty Tiers

| File | Tier | Description |
|------|------|-------------|
| `easy.sh` | Easy | Single-function problems, basic data structures, simple bug fixes |
| `medium.sh` | Medium | Concurrent data structures, algorithm problems, HTTP middleware |
| `hard.sh` | Hard | Lock-based concurrent containers, state machines, performance-sensitive code |
| `expert.sh` | Expert | Multi-component systems, async primitives, parsers, optimization passes |

## Progression

The overnight script uses batch number to select difficulty:

- Batches 1-2: easy
- Batches 3-4: medium
- Batches 5-6: hard
- Batches 7+: expert (loops expert indefinitely overnight)

## Adding Tasks

1. Add a line to the appropriate tier file.
2. Ensure the prompt includes the standard template:
   - `Work in /tmp/training-TASKNAME/.`
   - `Initialize a Go module (go mod init training/TASKNAME).`
   - Implementation requirements.
   - `Write TASKNAME_test.go with ...` (test requirements).
   - `Run go test ./... and report results.`
   - `Acceptance: <specific criterion>.`
3. Keep the prompt under 500 characters to avoid tokenization edge cases.
4. Test the prompt manually once before adding it to the rotation.

## Output

- Rollout JSONL files: `$HARNESS_ROLLOUT_DIR/<YYYY-MM-DD>/<run_id>.jsonl`
- Training log: `./training-reports/<DATE>-overnight.log`
- Markdown report: `./training-reports/<DATE>-overnight.md`
- harnessd log: `./training-reports/<DATE>-harnessd.log`
