#!/usr/bin/env bash
# run_bench.sh — run terminal-bench 2.0 with HarnessAgent
#
# Usage (from project root):
#   ./harness_agent/run_bench.sh [model] [n_tasks]
#
# Examples:
#   ./harness_agent/run_bench.sh                              # defaults: sonnet-4-6, 5 tasks
#   ./harness_agent/run_bench.sh anthropic/claude-opus-4-6   # full model path
#   ./harness_agent/run_bench.sh anthropic/claude-sonnet-4-6 20
#
# NOTE: run from the project root so that harness_agent/ is on the Python path.

set -euo pipefail

MODEL="${1:-anthropic/claude-sonnet-4-6}"
N="${2:-5}"  # start small; the full leaderboard set can be 100+

echo "Running terminal-bench 2.0 with model=${MODEL} n_tasks=${N}"

harbor run \
  -d terminal-bench@2.0 \
  --agent-import-path harness_agent.agent:HarnessAgent \
  -m "${MODEL}" \
  -n "${N}"
