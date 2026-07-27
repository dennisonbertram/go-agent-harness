#!/usr/bin/env bash
# live-harnessd.sh — start a key-free harnessd for the native app's live tests.
#
#   macapp/scripts/live-harnessd.sh [port]
#   HARNESS_TEST_BASE_URL=http://127.0.0.1:8899 swift test
#
# Why the odd provider setup: HARNESS_PROVIDER=fake installs the fake provider
# as the runner's *default*, but per-run resolution prefers whatever catalog
# provider serves the run's model. Pointing HARNESS_MODEL at a name no catalog
# provider serves forces resolution to miss, so runs fall back to the default
# (fake) provider — which is why live runs must send allow_fallback:true.
# See github.com/dennisonbertram/go-code issue for the underlying smoke bug.
set -euo pipefail

PORT="${1:-8899}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORKDIR="$(mktemp -d)"
BINARY="${WORKDIR}/harnessd"

cleanup() {
    [ -n "${SERVER_PID:-}" ] && kill "${SERVER_PID}" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

echo "[live-harnessd] building harnessd..."
(cd "${REPO_ROOT}" && go build -o "${BINARY}" ./cmd/harnessd)

# Enough scripted turns for several runs: the fake provider advances a single
# global cursor across runs rather than resetting per run.
python3 - "${WORKDIR}/turns.json" <<'PY'
import json, sys
turn_pair = [
    {"content": "", "tool_calls": [{"id": "c1", "name": "ls", "arguments": "{\"path\":\".\"}"}],
     "usage": {"prompt": 120, "completion": 10}},
    {"content": "I listed the workspace and it contains the project files.",
     "usage": {"prompt": 140, "completion": 12}, "cost_usd": 0.0025, "cost_status": "available"},
]
json.dump(turn_pair * 25, open(sys.argv[1], "w"))
PY

echo "[live-harnessd] starting on 127.0.0.1:${PORT} (workspace: ${WORKDIR})"
HARNESS_PROVIDER=fake \
HARNESS_FAKE_TURNS="${WORKDIR}/turns.json" \
HARNESS_MODEL=fake-model \
HARNESS_WORKSPACE="${WORKDIR}" \
HARNESS_CONVERSATION_DB="${WORKDIR}/conversations.db" \
HARNESS_ADDR="127.0.0.1:${PORT}" \
HARNESS_AUTH_DISABLED=true \
    "${BINARY}" >"${WORKDIR}/harnessd.log" 2>&1 &
SERVER_PID=$!

for _ in $(seq 1 30); do
    if curl -sf "http://127.0.0.1:${PORT}/healthz" >/dev/null 2>&1; then
        echo "[live-harnessd] ready — HARNESS_TEST_BASE_URL=http://127.0.0.1:${PORT}"
        echo "[live-harnessd] log: ${WORKDIR}/harnessd.log"
        wait "${SERVER_PID}"
        exit 0
    fi
    sleep 0.5
done

echo "[live-harnessd] FATAL: server did not become healthy" >&2
cat "${WORKDIR}/harnessd.log" >&2
exit 1
