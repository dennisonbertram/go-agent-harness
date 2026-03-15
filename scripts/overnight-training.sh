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

echo "=== Overnight Training Loop ===" | tee "$LOG"
echo "Date: $DATE" | tee -a "$LOG"
echo "Rollout dir: $ROLLOUT_DIR" | tee -a "$LOG"
echo "Report: $REPORT" | tee -a "$LOG"
echo "" | tee -a "$LOG"

# Verify required env vars
if [[ -z "${OPENAI_API_KEY:-}" ]]; then
  echo "ERROR: OPENAI_API_KEY not set" | tee -a "$LOG"
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

# Build trainerd once
echo "Building trainerd..." | tee -a "$LOG"
go build -o /tmp/trainerd-overnight ./cmd/trainerd/

# Start harnessd in background
echo "Starting harnessd..." | tee -a "$LOG"
export HARNESS_ROLLOUT_DIR="$ROLLOUT_DIR"
go run ./cmd/harnessd >> "$REPORT_DIR/$DATE-harnessd.log" 2>&1 &
HARNESS_PID=$!
trap 'kill $HARNESS_PID 2>/dev/null; echo "harnessd stopped"' EXIT

# Wait for harnessd ready
echo "Waiting for harnessd..." | tee -a "$LOG"
for i in $(seq 1 30); do
  if curl -sf http://localhost:8080/healthz >/dev/null 2>&1; then
    echo "harnessd ready" | tee -a "$LOG"
    break
  fi
  if [[ $i -eq 30 ]]; then
    echo "ERROR: harnessd did not become ready after 60s" | tee -a "$LOG"
    exit 1
  fi
  sleep 2
done

run_task() {
  local name="$1"
  local prompt="$2"
  local difficulty="$3"

  echo "  [$(date +%H:%M:%S)] Task: $name ($difficulty)" | tee -a "$LOG"

  # Submit task, capture run_id from first line of output
  local output
  output=$(timeout 600 go run ./cmd/harnesscli/ \
    -base-url="$BASE_URL" \
    -prompt="$prompt" \
    -model="$MODEL" 2>&1) || true

  local run_id
  run_id=$(echo "$output" | grep '^run_id=' | cut -d= -f2 | head -1)

  if [[ -n "$run_id" ]]; then
    echo "    run_id=$run_id" | tee -a "$LOG"
    echo "$run_id"  # return value
  else
    echo "    WARN: no run_id captured" | tee -a "$LOG"
    echo ""
  fi
}

analyze_batch() {
  local batch_name="$1"
  shift
  local run_ids=("$@")

  # Filter empty
  local valid_ids=()
  for id in "${run_ids[@]}"; do
    [[ -n "$id" ]] && valid_ids+=("$id")
  done

  if [[ ${#valid_ids[@]} -eq 0 ]]; then
    echo "  No valid run IDs to analyze" | tee -a "$LOG"
    return
  fi

  echo "  Analyzing ${#valid_ids[@]} runs..." | tee -a "$LOG"

  if [[ -z "${ANTHROPIC_API_KEY:-}" ]]; then
    # Score only (no Claude)
    echo "  (ANTHROPIC_API_KEY not set — running structural score only)" | tee -a "$LOG"
    for id in "${valid_ids[@]}"; do
      /tmp/trainerd-overnight --db-path "$DB_PATH" score \
        --run-id "$id" \
        --rollout-dir "$ROLLOUT_DIR" 2>&1 | tee -a "$LOG" || true
    done
  else
    # Full Claude analysis
    local ids_joined
    ids_joined=$(IFS=,; echo "${valid_ids[*]}")
    /tmp/trainerd-overnight --db-path "$DB_PATH" analyze \
      --run-ids "$ids_joined" \
      --rollout-dir "$ROLLOUT_DIR" 2>&1 | tee -a "$LOG" || true
  fi

  # Append to report
  cat >> "$REPORT" << EOF

## Batch: $batch_name

**Timestamp:** $(date)
**Run IDs:** ${valid_ids[*]:-none}

\`\`\`
$(tail -50 "$LOG")
\`\`\`

EOF
}

echo "" | tee -a "$LOG"
echo "=== Starting task batches ===" | tee -a "$LOG"
BATCH_NUM=0

# Infinite loop — runs until killed or morning
while true; do
  BATCH_NUM=$((BATCH_NUM + 1))
  echo "" | tee -a "$LOG"
  echo "--- Batch $BATCH_NUM ($(date)) ---" | tee -a "$LOG"

  # Pick difficulty based on batch number
  if   [[ $BATCH_NUM -le 2 ]]; then TIER="easy"
  elif [[ $BATCH_NUM -le 4 ]]; then TIER="medium"
  elif [[ $BATCH_NUM -le 6 ]]; then TIER="hard"
  else                               TIER="expert"
  fi

  echo "  Difficulty tier: $TIER" | tee -a "$LOG"

  # Source tasks for this tier
  TASK_FILE="./benchmarks/overnight-tasks/${TIER}.sh"
  if [[ ! -f "$TASK_FILE" ]]; then
    echo "  ERROR: task file not found: $TASK_FILE" | tee -a "$LOG"
    sleep 60
    continue
  fi

  # Run tasks from file, collect run IDs
  declare -a BATCH_RUN_IDS=()
  while IFS='|' read -r task_name prompt; do
    [[ "$task_name" =~ ^# ]] && continue  # skip comments
    [[ -z "$task_name" ]] && continue
    id=$(run_task "$task_name" "$prompt" "$TIER")
    BATCH_RUN_IDS+=("$id")
    sleep 5  # brief pause between tasks
  done < "$TASK_FILE"

  analyze_batch "Batch-$BATCH_NUM-$TIER" "${BATCH_RUN_IDS[@]}"

  echo "  Batch $BATCH_NUM complete. Sleeping ${BATCH_PAUSE}s..." | tee -a "$LOG"
  sleep "$BATCH_PAUSE"
done
