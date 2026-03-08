#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DATASET_PATH="${TERMINAL_BENCH_DATASET_PATH:-${REPO_ROOT}/benchmarks/terminal_bench/tasks}"
TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
OUTPUT_DIR="${TERMINAL_BENCH_OUTPUT_DIR:-${REPO_ROOT}/.tmp/terminal-bench/${TIMESTAMP}}"
AGENT_IMPORT_PATH="${TERMINAL_BENCH_AGENT_IMPORT_PATH:-benchmarks.terminal_bench.agent:GoAgentHarnessAgent}"
PYTHON_VERSION="${TERMINAL_BENCH_PYTHON_VERSION:-3.12}"
N_CONCURRENT="${TERMINAL_BENCH_N_CONCURRENT:-1}"
SKIP_BUILD="${TERMINAL_BENCH_SKIP_BUILD:-false}"
BENCH_MIN_ACCURACY="${BENCH_MIN_ACCURACY:-70}"

# Parse flags
for arg in "$@"; do
  case "${arg}" in
    --skip-build)
      SKIP_BUILD=true
      shift
      ;;
    --build-base-only)
      "${SCRIPT_DIR}/build-bench-images.sh"
      exit 0
      ;;
  esac
done

if [[ -z "${OPENAI_API_KEY:-}" ]]; then
  echo "OPENAI_API_KEY is required" >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required" >&2
  exit 1
fi

mkdir -p "${OUTPUT_DIR}"

# Build images unless --skip-build was passed
if [[ "${SKIP_BUILD}" != "true" ]]; then
  "${SCRIPT_DIR}/build-bench-images.sh"
fi

if command -v tb >/dev/null 2>&1; then
  TB_CMD=(tb)
elif command -v uv >/dev/null 2>&1; then
  TB_CMD=(uv tool run --python "${PYTHON_VERSION}" terminal-bench)
else
  echo "terminal-bench is required (install 'tb' or make 'uv' available)" >&2
  exit 1
fi

export PYTHONPATH="${REPO_ROOT}${PYTHONPATH:+:${PYTHONPATH}}"
"${TB_CMD[@]}" run \
  --dataset-path "${DATASET_PATH}" \
  --agent-import-path "${AGENT_IMPORT_PATH}" \
  --output-path "${OUTPUT_DIR}" \
  --n-concurrent "${N_CONCURRENT}" \
  "$@"

echo "[terminal-bench] results written to ${OUTPUT_DIR}"

# Generate analysis report
REPORT_PATH="${OUTPUT_DIR}/report.md"
if python3 "${SCRIPT_DIR}/analyze-bench-results.py" "${OUTPUT_DIR}" -o "${REPORT_PATH}"; then
  echo "[terminal-bench] report written to ${REPORT_PATH}"
else
  echo "[terminal-bench] warning: report generation failed" >&2
fi

# --- Summary table and accuracy gate ---
# Find results.json (may be in a timestamped subdirectory)
RESULTS_JSON=""
if [[ -f "${OUTPUT_DIR}/results.json" ]]; then
  RESULTS_JSON="${OUTPUT_DIR}/results.json"
else
  for d in "${OUTPUT_DIR}"/*/; do
    if [[ -f "${d}results.json" ]]; then
      RESULTS_JSON="${d}results.json"
      break
    fi
  done
fi

if [[ -n "${RESULTS_JSON}" ]]; then
  echo ""
  echo "=============================="
  echo "  Terminal Bench Summary"
  echo "=============================="
  echo ""

  # Parse results.json with python for portability
  python3 - "${RESULTS_JSON}" "${BENCH_MIN_ACCURACY}" <<'PYEOF'
import json, sys

results_file = sys.argv[1]
min_accuracy = int(sys.argv[2])

data = json.load(open(results_file))
results = data.get("results", [])
n_resolved = data.get("n_resolved", 0)
n_total = len(results)
accuracy = data.get("accuracy", 0)

print(f"  {'Task':<35} {'Result':>8}")
print(f"  {'-'*35} {'-'*8}")
for r in sorted(results, key=lambda x: x["task_id"]):
    status = "PASS" if r.get("is_resolved") else "FAIL"
    print(f"  {r['task_id']:<35} {status:>8}")
print(f"  {'-'*35} {'-'*8}")
print(f"  {'TOTAL':<35} {n_resolved}/{n_total}")
print(f"  Accuracy: {accuracy:.1%}")
print()

sys.exit(0 if accuracy * 100 >= min_accuracy else 1)
PYEOF
  ACCURACY_OK=$?

  if [[ ${ACCURACY_OK} -ne 0 ]]; then
    echo "[terminal-bench] FAIL: accuracy below threshold (${BENCH_MIN_ACCURACY}%)" >&2
    exit 1
  else
    echo "[terminal-bench] accuracy meets threshold (>= ${BENCH_MIN_ACCURACY}%)"
  fi
else
  echo "[terminal-bench] warning: could not find results.json for summary" >&2
fi
