#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${HARNESS_AUTORESEARCH_BASE_URL:-http://localhost:8080}"
PROFILE="${HARNESS_AUTORESEARCH_PROFILE:-full}"
PROMPT_PROFILE="${HARNESS_AUTORESEARCH_PROMPT_PROFILE:-autoresearch}"
MODEL="${HARNESS_AUTORESEARCH_MODEL:-}"
TEST_CMD="${HARNESS_AUTORESEARCH_TEST_CMD:-./scripts/test-regression.sh}"
REPORT_DIR="${HARNESS_AUTORESEARCH_REPORT_DIR:-.tmp/autoresearch}"
POLL_INTERVAL="${HARNESS_AUTORESEARCH_POLL_INTERVAL:-2}"
MAX_POLLS="${HARNESS_AUTORESEARCH_MAX_POLLS:-300}"
TARGET=""

usage() {
  cat <<'EOF'
Usage:
  scripts/autoresearch-run.sh --target "package.or.func" [--test-cmd "..."] [--report-dir DIR]

Required:
  --target        The seam to focus on during this run.

Optional:
  --test-cmd      Validation command to run after the agent finishes.
  --base-url      Harness API base URL. Default: http://localhost:8080
  --profile       Run profile sent with POST /v1/runs. Default: full
  --prompt-profile Prompt routing profile. Default: autoresearch
  --model         Optional model override for the run.
  --report-dir    Output directory for logs and the markdown report.

Environment overrides:
  HARNESS_AUTORESEARCH_BASE_URL
  HARNESS_AUTORESEARCH_PROFILE
  HARNESS_AUTORESEARCH_PROMPT_PROFILE
  HARNESS_AUTORESEARCH_MODEL
  HARNESS_AUTORESEARCH_TEST_CMD
  HARNESS_AUTORESEARCH_REPORT_DIR
  HARNESS_AUTORESEARCH_POLL_INTERVAL
  HARNESS_AUTORESEARCH_MAX_POLLS
EOF
}

slugify() {
  printf '%s' "$1" \
    | tr '[:upper:]' '[:lower:]' \
    | tr '/: ' '---' \
    | tr -cd 'a-z0-9._-'
}

json_field() {
  local field="$1"
  local json="$2"
  if command -v jq >/dev/null 2>&1; then
    printf '%s' "$json" | jq -r ".${field} // empty"
  else
    JSON_FIELD="$field" python3 -c 'import json, os, sys; data = json.load(sys.stdin); value = data.get(os.environ["JSON_FIELD"], ""); print("" if value is None else value)' <<<"$json"
  fi
}

build_payload() {
  if command -v jq >/dev/null 2>&1; then
    jq -n \
      --arg prompt "$PROMPT" \
      --arg profile "$PROFILE" \
      --arg prompt_profile "$PROMPT_PROFILE" \
      --arg model "$MODEL" \
      '{prompt: $prompt, profile: $profile, prompt_profile: $prompt_profile} + (if $model != "" then {model: $model} else {} end)'
  else
    AUTORESEARCH_PROMPT="$PROMPT" \
    AUTORESEARCH_PROFILE="$PROFILE" \
    AUTORESEARCH_PROMPT_PROFILE="$PROMPT_PROFILE" \
    AUTORESEARCH_MODEL="$MODEL" \
    python3 - <<'PY'
import json
import os

payload = {
    "prompt": os.environ["AUTORESEARCH_PROMPT"],
    "profile": os.environ["AUTORESEARCH_PROFILE"],
    "prompt_profile": os.environ["AUTORESEARCH_PROMPT_PROFILE"],
}
model = os.environ.get("AUTORESEARCH_MODEL", "")
if model:
    payload["model"] = model
print(json.dumps(payload))
PY
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target)
      TARGET="${2:-}"
      shift 2
      ;;
    --test-cmd)
      TEST_CMD="${2:-}"
      shift 2
      ;;
    --base-url)
      BASE_URL="${2:-}"
      shift 2
      ;;
    --profile)
      PROFILE="${2:-}"
      shift 2
      ;;
    --prompt-profile)
      PROMPT_PROFILE="${2:-}"
      shift 2
      ;;
    --model)
      MODEL="${2:-}"
      shift 2
      ;;
    --report-dir)
      REPORT_DIR="${2:-}"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "autoresearch-run: unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "$TARGET" ]]; then
  echo "autoresearch-run: --target is required" >&2
  usage >&2
  exit 1
fi

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

RUN_DIR="$REPORT_DIR/$(date +%Y%m%d-%H%M%S)-$(slugify "$TARGET")"
mkdir -p "$RUN_DIR"

RUN_LOG="$RUN_DIR/run.log"
TEST_LOG="$RUN_DIR/test.log"
REPORT="$RUN_DIR/report.md"

PROMPT=$(cat <<EOF
You are the autoresearch testing agent for go-agent-harness.

Goal:
- Find the smallest meaningful regression or characterization test for the target seam.
- If the seam is too broad, narrow it and explain why.
- Prefer tests first; only make production changes after a failing or missing test is identified.
- Keep edits surgical and evidence-backed.

Required reading before editing:
- docs/context/critical-context.md
- docs/runbooks/testing.md
- docs/investigations/test-coverage-gaps.md

Target seam:
${TARGET}

Suggested validation command:
${TEST_CMD}

Output format:
1. Target seam
2. Tests added or tightened
3. Commands run
4. Result and remaining risk
5. Next target
EOF
)

if ! curl -fsS "${BASE_URL}/healthz" >/dev/null; then
  {
    echo "# Autoresearch Run"
    echo
    echo "- target: \`${TARGET}\`"
    echo "- result: server unreachable at \`${BASE_URL}\`"
  } >"$REPORT"
  echo "autoresearch-run: harnessd is not reachable at ${BASE_URL}" >&2
  exit 1
fi

PAYLOAD="$(build_payload)"

set +e
START_RESPONSE="$(curl -sS -X POST "${BASE_URL}/v1/runs" -H "Content-Type: application/json" -d "${PAYLOAD}")"
START_STATUS=$?
set -e

printf '%s\n' "$START_RESPONSE" >"$RUN_LOG"
if [[ $START_STATUS -ne 0 ]]; then
  {
    echo "# Autoresearch Run"
    echo
    echo "- target: \`${TARGET}\`"
    echo "- result: failed to create run"
    echo
    echo "## Run Log"
    echo
    echo '```'
    cat "$RUN_LOG"
    echo '```'
  } >"$REPORT"
  echo "autoresearch-run: failed to create run" >&2
  exit 1
fi

RUN_ID="$(json_field id "$START_RESPONSE")"
if [[ -z "$RUN_ID" ]]; then
  {
    echo "# Autoresearch Run"
    echo
    echo "- target: \`${TARGET}\`"
    echo "- result: run creation response missing id"
    echo
    echo "## Run Log"
    echo
    echo '```'
    cat "$RUN_LOG"
    echo '```'
  } >"$REPORT"
  echo "autoresearch-run: missing run id in response" >&2
  exit 1
fi

FINAL_RESPONSE=""
FINAL_STATUS=""
FINAL_OUTPUT=""
FINAL_ERROR=""

for _ in $(seq 1 "$MAX_POLLS"); do
  set +e
  STATUS_RESPONSE="$(curl -sS "${BASE_URL}/v1/runs/${RUN_ID}")"
  POLL_STATUS=$?
  set -e
  if [[ $POLL_STATUS -ne 0 ]]; then
    sleep "$POLL_INTERVAL"
    continue
  fi

  FINAL_RESPONSE="$STATUS_RESPONSE"
  FINAL_STATUS="$(json_field status "$STATUS_RESPONSE")"
  FINAL_OUTPUT="$(json_field output "$STATUS_RESPONSE")"
  FINAL_ERROR="$(json_field error "$STATUS_RESPONSE")"

  case "$FINAL_STATUS" in
    completed|failed|error|cancelled)
      break
      ;;
  esac

  sleep "$POLL_INTERVAL"
done

if [[ -z "$FINAL_STATUS" ]]; then
  FINAL_STATUS="timeout"
fi

if [[ -n "$TEST_CMD" ]]; then
  set +e
  bash -lc "$TEST_CMD" >"$TEST_LOG" 2>&1
  TEST_EXIT=$?
  set -e
else
  TEST_EXIT=0
  : >"$TEST_LOG"
fi

{
  echo "# Autoresearch Run"
  echo
  echo "- target: \`${TARGET}\`"
  echo "- run profile: \`${PROFILE}\`"
  echo "- prompt profile: \`${PROMPT_PROFILE}\`"
  echo "- run id: \`${RUN_ID}\`"
  echo "- final status: \`${FINAL_STATUS}\`"
  echo "- validation command: \`${TEST_CMD}\`"
  echo "- validation exit: \`${TEST_EXIT}\`"
  echo
  echo "## Prompt"
  echo
  echo '```'
  printf '%s\n' "$PROMPT"
  echo '```'
  echo
  echo "## Run Log"
  echo
  echo '```'
  cat "$RUN_LOG"
  echo '```'
  echo
  echo "## Run Status"
  echo
  echo '```json'
  if [[ -n "$FINAL_RESPONSE" ]]; then
    printf '%s\n' "$FINAL_RESPONSE"
  fi
  echo '```'
  echo
  echo "## Validation Log"
  echo
  echo '```'
  cat "$TEST_LOG"
  echo '```'
  echo
  echo "## Git Status"
  echo
  echo '```'
  git status --short
  echo '```'
  echo
  echo "## Git Diff Stat"
  echo
  echo '```'
  git diff --stat
  echo '```'
} >"$REPORT"

if [[ "$FINAL_STATUS" != "completed" || "$TEST_EXIT" -ne 0 ]]; then
  echo "autoresearch-run: completed with status=${FINAL_STATUS}, validation_exit=${TEST_EXIT}" >&2
  exit 1
fi

echo "autoresearch-run: report written to ${RUN_DIR}"
