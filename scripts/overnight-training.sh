#!/usr/bin/env bash
set -euo pipefail

ROLLOUT_DIR="${HARNESS_ROLLOUT_DIR:-$HOME/.trainerd/rollouts}"
DB_PATH="${TRAINERD_DB:-$HOME/.trainerd/training.db}"
REPORT_DIR="./training-reports"
DATE=$(date +%Y-%m-%d)
LOG="$REPORT_DIR/$DATE-overnight.log"
REPORT="$REPORT_DIR/$DATE-overnight.md"
MODEL="${HARNESS_MODEL:-gpt-4.1}"
BASE_URL="http://localhost:8080"
BATCH_PAUSE=60  # seconds between batches

mkdir -p "$ROLLOUT_DIR" "$REPORT_DIR"

log() { echo "$*" | tee -a "$LOG"; }

log "=== Overnight Training Loop ==="
log "Date: $DATE"
log "Rollout dir: $ROLLOUT_DIR"
log "Report: $REPORT"
log ""

# Verify required env vars
if [[ -z "${OPENAI_API_KEY:-}" ]]; then
  log "ERROR: OPENAI_API_KEY not set"
  exit 1
fi

# Initialize report
cat > "$REPORT" << EOF
# Overnight Training Report — $DATE

**Started:** $(date)
**Model:** $MODEL
**Rollout dir:** $ROLLOUT_DIR

---
EOF

# Pre-build all binaries once (avoids recompile per task)
log "Building binaries..."
go build -o /tmp/trainerd-overnight    ./cmd/trainerd/   2>&1 | tee -a "$LOG"
go build -o /tmp/harnesscli-overnight  ./cmd/harnesscli/ 2>&1 | tee -a "$LOG"
go build -o /tmp/harnessd-overnight    ./cmd/harnessd/   2>&1 | tee -a "$LOG"
log "Binaries ready."

# Start harnessd in background with generous step limit
log "Starting harnessd..."
export HARNESS_ROLLOUT_DIR="$ROLLOUT_DIR"
export HARNESS_MAX_STEPS="${HARNESS_MAX_STEPS:-1000}"
/tmp/harnessd-overnight >> "$REPORT_DIR/$DATE-harnessd.log" 2>&1 &
HARNESS_PID=$!
trap 'kill $HARNESS_PID 2>/dev/null; log "harnessd stopped"' EXIT

# Wait for harnessd ready
log "Waiting for harnessd..."
for i in $(seq 1 30); do
  if curl -sf http://localhost:8080/healthz >/dev/null 2>&1; then
    log "harnessd ready"
    break
  fi
  if [[ $i -eq 30 ]]; then
    log "ERROR: harnessd did not become ready after 60s"
    exit 1
  fi
  sleep 2
done

# run_task: submits a single task via harnesscli.
# Logs go ONLY to $LOG (not stdout) so that id=$(run_task ...) captures only the run_id.
run_task() {
  local name="$1"
  local prompt="$2"
  local difficulty="$3"

  echo "  [$(date +%H:%M:%S)] Task: $name ($difficulty)" >> "$LOG"

  local output
  # macOS has no 'timeout' built-in; rely on harness's own HARNESS_MAX_STEPS limit
  # Redirect SSE stream to a separate verbose log to keep main log readable
  output=$(/tmp/harnesscli-overnight \
    -base-url="$BASE_URL" \
    -prompt="$prompt" \
    -model="$MODEL" 2>&1) || true

  # Log only the terminal event line (not full SSE stream) to keep log readable
  echo "$output" | grep -E '^(run_id=|terminal_event=|ERROR)' >> "$LOG" || true
  # Full SSE stream goes to verbose log for deep analysis
  echo "=== $name ===" >> "$REPORT_DIR/$DATE-verbose.log"
  echo "$output"      >> "$REPORT_DIR/$DATE-verbose.log"

  local run_id
  run_id=$(echo "$output" | grep '^run_id=' | cut -d= -f2 | head -1)

  if [[ -n "$run_id" ]]; then
    echo "    run_id=$run_id" >> "$LOG"
    echo "$run_id"   # ← only this goes to stdout; captured by id=$(run_task ...)
  else
    echo "    WARN: no run_id captured" >> "$LOG"
    echo ""
  fi
}

analyze_batch() {
  local batch_name="$1"
  shift
  local -a run_ids=("$@")

  # Filter empty entries
  local -a valid_ids=()
  for id in "${run_ids[@]:-}"; do
    [[ -n "$id" ]] && valid_ids+=("$id")
  done

  if [[ ${#valid_ids[@]} -eq 0 ]]; then
    log "  No valid run IDs to analyze"
    return
  fi

  log "  Analyzing ${#valid_ids[@]} runs: ${valid_ids[*]}"

  if [[ -z "${ANTHROPIC_API_KEY:-}" ]]; then
    log "  (ANTHROPIC_API_KEY not set — structural score only)"
    for id in "${valid_ids[@]}"; do
      /tmp/trainerd-overnight --db-path "$DB_PATH" score \
        --run-id "$id" \
        --rollout-dir "$ROLLOUT_DIR" >> "$LOG" 2>&1 || true
    done
  else
    local ids_joined
    ids_joined=$(IFS=,; echo "${valid_ids[*]}")
    /tmp/trainerd-overnight --db-path "$DB_PATH" analyze \
      --run-ids "$ids_joined" \
      --rollout-dir "$ROLLOUT_DIR" >> "$LOG" 2>&1 || true
  fi

  # Append batch section to report
  {
    echo ""
    echo "## Batch: $batch_name"
    echo ""
    echo "**Timestamp:** $(date)"
    echo "**Run IDs:** ${valid_ids[*]}"
    echo ""
    echo '```'
    tail -80 "$LOG"
    echo '```'
    echo ""
  } >> "$REPORT"
}

log ""
log "=== Starting task batches ==="
BATCH_NUM=0

# Infinite loop — runs until killed
while true; do
  BATCH_NUM=$((BATCH_NUM + 1))
  log ""
  log "--- Batch $BATCH_NUM ($(date)) ---"

  # Escalate difficulty over time
  if   [[ $BATCH_NUM -le 1 ]]; then TIER="easy"
  elif [[ $BATCH_NUM -le 2 ]]; then TIER="terminal-easy"
  elif [[ $BATCH_NUM -le 3 ]]; then TIER="medium"
  elif [[ $BATCH_NUM -le 4 ]]; then TIER="terminal-hard"
  elif [[ $BATCH_NUM -le 5 ]]; then TIER="hard"
  elif [[ $BATCH_NUM -le 6 ]]; then TIER="expert"
  elif [[ $BATCH_NUM -le 7 ]]; then TIER="ultra"
  else
    # Cycle through terminal-hard and ultra for the rest of the night
    CYCLE=$(( (BATCH_NUM - 8) % 2 ))
    if [[ $CYCLE -eq 0 ]]; then TIER="terminal-hard"
    else                        TIER="ultra"
    fi
  fi

  log "  Difficulty tier: $TIER"

  TASK_FILE="./benchmarks/overnight-tasks/${TIER}.sh"
  if [[ ! -f "$TASK_FILE" ]]; then
    log "  ERROR: task file not found: $TASK_FILE"
    sleep 60
    continue
  fi

  # Collect run IDs for this batch
  local_ids=()
  while IFS='|' read -r task_name prompt; do
    [[ "$task_name" =~ ^[[:space:]]*# ]] && continue  # skip comments
    [[ -z "${task_name// }" ]]           && continue  # skip blank lines
    id=$(run_task "$task_name" "$prompt" "$TIER")
    local_ids+=("$id")
    sleep 5  # brief pause between tasks
  done < "$TASK_FILE"

  analyze_batch "Batch-$BATCH_NUM-$TIER" "${local_ids[@]:-}"

  log "  Batch $BATCH_NUM complete. Sleeping ${BATCH_PAUSE}s..."
  sleep "$BATCH_PAUSE"
done
