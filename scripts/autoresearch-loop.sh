#!/usr/bin/env bash
set -euo pipefail

ITERATIONS="${HARNESS_AUTORESEARCH_ITERATIONS:-1}"
PAUSE_SECONDS="${HARNESS_AUTORESEARCH_PAUSE_SECONDS:-0}"
REPORT_DIR="${HARNESS_AUTORESEARCH_REPORT_DIR:-.tmp/autoresearch}"
BASE_URL="${HARNESS_AUTORESEARCH_BASE_URL:-http://localhost:8080}"
PROFILE="${HARNESS_AUTORESEARCH_PROFILE:-full}"
PROMPT_PROFILE="${HARNESS_AUTORESEARCH_PROMPT_PROFILE:-autoresearch}"
MODEL="${HARNESS_AUTORESEARCH_MODEL:-}"
TARGETS=()

usage() {
  cat <<'EOF'
Usage:
  scripts/autoresearch-loop.sh [--iterations N] [--pause SECONDS] [--target "seam"] ...

Optional flags:
  --iterations    Number of passes over the target list. Default: 1
  --pause         Seconds to sleep between runs. Default: 0
  --target        Add a target seam. May be repeated. When omitted, a default
                  coverage-gap-driven target list is used.
  --report-dir    Output directory for run artifacts. Default: .tmp/autoresearch
EOF
}

default_targets() {
  cat <<'EOF'
internal/workspace.ContainerWorkspace.Provision
internal/harness.tools.core.GitDiffTool
internal/harness.tools.core.ObservationalMemoryTool
internal/harness.Runner.SubmitInput
internal/harness.parseTurnsHTTP
internal/server.handleRunEvents
EOF
}

pick_test_cmd() {
  case "$1" in
    internal/workspace.ContainerWorkspace.Provision)
      echo "go test ./internal/workspace ./internal/harness ./cmd/harnessd"
      ;;
    internal/harness.tools.core.GitDiffTool|internal/harness.tools.core.ObservationalMemoryTool)
      echo "go test ./internal/harness/tools/core ./internal/harness"
      ;;
    internal/harness.Runner.SubmitInput|internal/harness.parseTurnsHTTP)
      echo "go test ./internal/harness ./internal/server"
      ;;
    internal/server.handleRunEvents)
      echo "go test ./internal/server ./internal/harness"
      ;;
    *)
      echo "${HARNESS_AUTORESEARCH_DEFAULT_TEST_CMD:-./scripts/test-regression.sh}"
      ;;
  esac
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --iterations)
      ITERATIONS="${2:-1}"
      shift 2
      ;;
    --pause)
      PAUSE_SECONDS="${2:-0}"
      shift 2
      ;;
    --target)
      TARGETS+=("${2:-}")
      shift 2
      ;;
    --report-dir)
      REPORT_DIR="${2:-}"
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
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "autoresearch-loop: unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ ${#TARGETS[@]} -eq 0 ]]; then
  while IFS= read -r target; do
    TARGETS+=("$target")
  done < <(default_targets)
fi

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"
mkdir -p "$REPORT_DIR"

LOOP_LOG="$REPORT_DIR/loop.log"
SUMMARY="$REPORT_DIR/loop-summary.md"
OVERALL_FAIL=0

{
  echo "# Autoresearch Loop"
  echo
  echo "- started: $(date)"
  echo "- iterations: \`${ITERATIONS}\`"
  echo "- targets: \`${TARGETS[*]}\`"
  echo "- report dir: \`${REPORT_DIR}\`"
  echo
} >"$SUMMARY"

for iteration in $(seq 1 "$ITERATIONS"); do
  {
    echo "[loop] iteration ${iteration} / ${ITERATIONS}"
    echo "[loop] target count: ${#TARGETS[@]}"
  } | tee -a "$LOOP_LOG"

  for target in "${TARGETS[@]}"; do
    if [[ -z "$target" ]]; then
      continue
    fi

    test_cmd="$(pick_test_cmd "$target")"
    {
      echo "[loop] target=${target}"
      echo "[loop] test_cmd=${test_cmd}"
    } | tee -a "$LOOP_LOG"

    set +e
    HARNESS_AUTORESEARCH_BASE_URL="$BASE_URL" \
    HARNESS_AUTORESEARCH_PROFILE="$PROFILE" \
    HARNESS_AUTORESEARCH_PROMPT_PROFILE="$PROMPT_PROFILE" \
    HARNESS_AUTORESEARCH_MODEL="$MODEL" \
    HARNESS_AUTORESEARCH_REPORT_DIR="$REPORT_DIR" \
    HARNESS_AUTORESEARCH_TEST_CMD="$test_cmd" \
    ./scripts/autoresearch-run.sh --target "$target"
    run_exit=$?
    set -e

    {
      echo "[loop] target=${target} exit=${run_exit}"
      echo
    } | tee -a "$LOOP_LOG"

    if [[ $run_exit -ne 0 ]]; then
      OVERALL_FAIL=1
    fi

    if [[ "$PAUSE_SECONDS" -gt 0 ]]; then
      sleep "$PAUSE_SECONDS"
    fi
  done
done

{
  echo "- finished: $(date)"
  echo "- overall status: $(if [[ $OVERALL_FAIL -eq 0 ]]; then echo DONE; else echo NOT_DONE; fi)"
} >>"$SUMMARY"

if [[ $OVERALL_FAIL -ne 0 ]]; then
  echo "autoresearch-loop: one or more runs failed; see ${SUMMARY}" >&2
  exit 1
fi

echo "autoresearch-loop: summary written to ${SUMMARY}"
