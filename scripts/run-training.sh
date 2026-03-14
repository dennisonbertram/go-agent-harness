#!/usr/bin/env bash
# scripts/run-training.sh — convenience script to run the training loop
set -euo pipefail

ROLLOUT_DIR="${HARNESS_ROLLOUT_DIR:-$HOME/.trainerd/rollouts}"
DB_PATH="${TRAINERD_DB:-$HOME/.trainerd/training.db}"

echo "=== Go Agent Harness — Training Mode ==="
echo "Rollout dir: $ROLLOUT_DIR"
echo "DB: $DB_PATH"

go run ./cmd/trainerd/... \
  --db-path "$DB_PATH" \
  loop \
  --rollout-dir "$ROLLOUT_DIR" \
  "$@"
