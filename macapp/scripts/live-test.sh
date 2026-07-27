#!/usr/bin/env bash
# live-test.sh — run the full Swift suite against real harnessd processes.
#
#   macapp/scripts/live-test.sh [extra swift test args...]
#
# Covers two live paths:
#   1. HARNESS_TEST_BASE_URL — a long-lived server this script starts, used by
#      the client and RunSession suites.
#   2. HARNESS_BINARY — the real binary, which ProjectSession spawns itself so
#      process supervision is exercised end to end.
#
# The fake-provider env vars are exported here rather than passed by the
# supervisor: spawned children inherit this process's environment, which is what
# lets a supervised server run key-free. See issue #920 for why HARNESS_MODEL
# must name a model no catalog provider serves.
set -euo pipefail

MACAPP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_ROOT="$(cd "${MACAPP_DIR}/.." && pwd)"
WORKDIR="$(mktemp -d)"
# Build the binary inside the repo so the supervisor's own installation-root
# resolution (prompts + model catalog) is what gets exercised, rather than
# being bypassed by explicit env vars.
BIN_DIR="${REPO_ROOT}/.harnessd-bin"
PORT="${LIVE_TEST_PORT:-8899}"

cleanup() {
    [ -n "${SERVER_PID:-}" ] && kill "${SERVER_PID}" 2>/dev/null || true
    # Reap any server a test spawned but failed to stop.
    pkill -f "${BIN_DIR}/harnessd" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

echo "[live-test] building harnessd…"
mkdir -p "${BIN_DIR}"
(cd "${REPO_ROOT}" && go build -o "${BIN_DIR}/harnessd" ./cmd/harnessd)

python3 - "${WORKDIR}/turns.json" <<'PY'
import json, sys
pair = [
    {"content": "", "tool_calls": [{"id": "c1", "name": "ls", "arguments": "{\"path\":\".\"}"}],
     "usage": {"prompt": 120, "completion": 10}},
    {"content": "I listed the workspace and it contains the project files.",
     "usage": {"prompt": 140, "completion": 12}, "cost_usd": 0.0025, "cost_status": "available"},
]
# The fake provider advances one global cursor shared by every run on a server,
# so supply plenty of turns for a whole suite.
json.dump(pair * 200, open(sys.argv[1], "w"))
PY

export HARNESS_PROVIDER=fake
export HARNESS_FAKE_TURNS="${WORKDIR}/turns.json"
export HARNESS_MODEL=fake-model
export HARNESS_AUTH_DISABLED=true
export HARNESS_BINARY="${BIN_DIR}/harnessd"

echo "[live-test] starting shared server on 127.0.0.1:${PORT}…"
(cd "${REPO_ROOT}" && \
 HARNESS_ADDR="127.0.0.1:${PORT}" \
 HARNESS_WORKSPACE="${WORKDIR}" \
 HARNESS_CONVERSATION_DB="${WORKDIR}/conversations.db" \
    "${BIN_DIR}/harnessd" >"${WORKDIR}/harnessd.log" 2>&1) &
SERVER_PID=$!

for _ in $(seq 1 60); do
    if curl -sf "http://127.0.0.1:${PORT}/healthz" >/dev/null 2>&1; then break; fi
    sleep 0.5
done
if ! curl -sf "http://127.0.0.1:${PORT}/healthz" >/dev/null 2>&1; then
    echo "[live-test] FATAL: server never became healthy" >&2
    cat "${WORKDIR}/harnessd.log" >&2
    exit 1
fi

export HARNESS_TEST_BASE_URL="http://127.0.0.1:${PORT}"
echo "[live-test] running suite…"
cd "${MACAPP_DIR}"
swift test "$@"
