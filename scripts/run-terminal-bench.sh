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

if [[ -z "${OPENAI_API_KEY:-}" ]]; then
  echo "OPENAI_API_KEY is required" >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required" >&2
  exit 1
fi

mkdir -p "${OUTPUT_DIR}"

DOCKER_BUILDKIT=0 docker build --pull=false -t go-agent-harness-tb-go-retry:latest "${DATASET_PATH}/go-retry-schedule-fix" >/dev/null
DOCKER_BUILDKIT=0 docker build --pull=false -t go-agent-harness-tb-staging-docs:latest "${DATASET_PATH}/staging-deploy-docs" >/dev/null
DOCKER_BUILDKIT=0 docker build --pull=false -t go-agent-harness-tb-incident-shell:latest "${DATASET_PATH}/incident-summary-shell" >/dev/null

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
