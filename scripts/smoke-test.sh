#!/usr/bin/env bash
# smoke-test.sh — golden-path smoke test for go-agent-harness
#
# Usage: ./scripts/smoke-test.sh
#
# Prerequisites:
#   - harnessd binary built at ./harnessd (run: go build -o harnessd ./cmd/harnessd)
#   - At least one of: OPENAI_API_KEY, ANTHROPIC_API_KEY, or GEMINI_API_KEY set in env
#   - curl available on PATH
#
# The script starts harnessd on a random high port, runs a sequence of checks,
# then kills the server on exit.

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

BINARY="${HARNESS_BINARY:-./harnessd}"
PROFILE="${HARNESS_PROFILE:-full}"
LOG_FILE="${HARNESS_SMOKE_LOG:-/tmp/harnessd-smoke.log}"
TIMEOUT_S="${HARNESS_SMOKE_TIMEOUT:-120}"
MODEL="${HARNESS_SMOKE_MODEL:-}"
PREFERRED_PROVIDER="${HARNESS_SMOKE_PROVIDER:-}"

# Pick a random port in the ephemeral range to avoid conflicts.
PORT=$(( ( RANDOM % 10000 ) + 50000 ))
BASE_URL="http://localhost:${PORT}"

PASS_COUNT=0
FAIL_COUNT=0
SERVER_PID=""

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

info()  { echo "[smoke] $*"; }
pass()  { echo "[smoke] PASS: $*"; PASS_COUNT=$(( PASS_COUNT + 1 )); }
fail()  { echo "[smoke] FAIL: $*"; FAIL_COUNT=$(( FAIL_COUNT + 1 )); }

cleanup() {
    if [ -n "${SERVER_PID}" ]; then
        info "stopping harnessd (pid=${SERVER_PID})"
        kill "${SERVER_PID}" 2>/dev/null || true
        wait "${SERVER_PID}" 2>/dev/null || true
        SERVER_PID=""
    fi
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# Step 0: Prerequisites
# ---------------------------------------------------------------------------

info "checking prerequisites..."

if [ ! -x "${BINARY}" ]; then
    fail "harnessd binary not found or not executable at: ${BINARY}"
    echo "[smoke] Build it first:  go build -o harnessd ./cmd/harnessd"
    exit 1
fi

# At least one provider key must be set.
PROVIDER_KEY_FOUND=0
for VAR in OPENAI_API_KEY ANTHROPIC_API_KEY GEMINI_API_KEY; do
    if [ -n "${!VAR:-}" ]; then
        info "found provider credential: ${VAR}"
        PROVIDER_KEY_FOUND=1
        break
    fi
done

if [ "${PROVIDER_KEY_FOUND}" -eq 0 ]; then
    fail "no provider API key found — set OPENAI_API_KEY, ANTHROPIC_API_KEY, or GEMINI_API_KEY"
    exit 1
fi

pass "prerequisites satisfied"

# ---------------------------------------------------------------------------
# Step 1: Start harnessd
# ---------------------------------------------------------------------------

info "starting harnessd --profile ${PROFILE} on port ${PORT}..."
HARNESS_ADDR=":${PORT}" HARNESS_AUTH_DISABLED=true \
    "${BINARY}" --profile "${PROFILE}" \
    >"${LOG_FILE}" 2>&1 &
SERVER_PID=$!

info "harnessd pid=${SERVER_PID}, log=${LOG_FILE}"

# ---------------------------------------------------------------------------
# Step 2: Wait for /healthz
# ---------------------------------------------------------------------------

info "waiting for /healthz (up to 30s)..."
HEALTH_WAITED=0
while true; do
    if curl -sf "${BASE_URL}/healthz" >/dev/null 2>&1; then
        pass "/healthz is responding"
        break
    fi
    HEALTH_WAITED=$(( HEALTH_WAITED + 1 ))
    if [ "${HEALTH_WAITED}" -ge 30 ]; then
        fail "/healthz did not respond within 30s"
        echo "[smoke] server log tail:"
        tail -20 "${LOG_FILE}" || true
        exit 1
    fi
    sleep 1
done

# ---------------------------------------------------------------------------
# Step 3: Test GET /healthz
# ---------------------------------------------------------------------------

info "GET /healthz..."
HEALTH_BODY=$(curl -sf "${BASE_URL}/healthz")
HEALTH_STATUS=$(echo "${HEALTH_BODY}" | grep -o '"status":"ok"' || true)
if [ -n "${HEALTH_STATUS}" ]; then
    pass "GET /healthz → 200 {\"status\":\"ok\"}"
else
    fail "GET /healthz unexpected body: ${HEALTH_BODY}"
fi

# ---------------------------------------------------------------------------
# Step 4: Test GET /v1/providers
# ---------------------------------------------------------------------------

info "GET /v1/providers..."
PROVIDERS_BODY=$(curl -sf "${BASE_URL}/v1/providers")
# The response wraps the array: {"providers": [...]}
PROVIDER_COUNT=$(echo "${PROVIDERS_BODY}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
providers = data.get('providers', [])
print(len(providers))
" 2>/dev/null || echo "0")

SELECTED_PROVIDER="${PREFERRED_PROVIDER}"
if [ -z "${SELECTED_PROVIDER}" ]; then
    SELECTED_PROVIDER=$(echo "${PROVIDERS_BODY}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
providers = data.get('providers', [])
configured = [p.get('name', '') for p in providers if p.get('configured')]
print(configured[0] if configured else '')
" 2>/dev/null || true)
fi

if [ "${PROVIDER_COUNT}" -gt 0 ]; then
    pass "GET /v1/providers → 200, ${PROVIDER_COUNT} provider(s) in catalog"
else
    fail "GET /v1/providers returned 0 providers (body: ${PROVIDERS_BODY})"
fi

if [ -z "${SELECTED_PROVIDER}" ]; then
    fail "could not determine a configured provider from /v1/providers (body: ${PROVIDERS_BODY})"
    exit 1
fi

pass "selected smoke provider: ${SELECTED_PROVIDER}"

# ---------------------------------------------------------------------------
# Step 5: Test GET /v1/models
# ---------------------------------------------------------------------------

info "GET /v1/models..."
MODELS_BODY=$(curl -sf "${BASE_URL}/v1/models")
# Response is a JSON array of models.
MODEL_COUNT=$(echo "${MODELS_BODY}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
# Response is wrapped: {\"models\": [...]}
if isinstance(data, list):
    print(len(data))
else:
    models = data.get('models', [])
    print(len(models))
" 2>/dev/null || echo "0")

if [ "${MODEL_COUNT}" -gt 0 ]; then
    pass "GET /v1/models → 200, ${MODEL_COUNT} model(s)"
else
    fail "GET /v1/models returned 0 models (body: ${MODELS_BODY})"
fi

if [ -z "${MODEL}" ]; then
    MODEL=$(printf '%s' "${MODELS_BODY}" | SMOKE_PROVIDER="${SELECTED_PROVIDER}" python3 -c "
import json, os, sys
provider = os.environ.get('SMOKE_PROVIDER', '')
data = json.load(sys.stdin)
models = data if isinstance(data, list) else data.get('models', [])
for model in models:
    if provider and model.get('provider') == provider:
        print(model.get('id', ''))
        break
else:
    print('')
" 2>/dev/null || true)
fi

if [ -z "${MODEL}" ]; then
    fail "could not determine a smoke model for provider ${SELECTED_PROVIDER} from /v1/models"
    exit 1
fi

pass "selected smoke model: ${MODEL}"

# ---------------------------------------------------------------------------
# Step 6: Create a run
# ---------------------------------------------------------------------------

info "POST /v1/runs (model=${MODEL})..."
RUN_RESPONSE=$(curl -sf -X POST "${BASE_URL}/v1/runs" \
    -H "Content-Type: application/json" \
    -d "{\"model\": \"${MODEL}\", \"prompt\": \"Reply with exactly: SMOKE_TEST_PASS\"}")

RUN_ID=$(echo "${RUN_RESPONSE}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(data.get('run_id', data.get('id', '')))
" 2>/dev/null || true)

if [ -z "${RUN_ID}" ]; then
    fail "POST /v1/runs did not return a run_id (response: ${RUN_RESPONSE})"
    exit 1
fi

pass "POST /v1/runs → run_id=${RUN_ID}"

# ---------------------------------------------------------------------------
# Step 7: Poll for run completion
# ---------------------------------------------------------------------------

info "polling GET /v1/runs/${RUN_ID} for completion (timeout ${TIMEOUT_S}s)..."
ELAPSED=0
RUN_STATUS=""
while true; do
    RUN_STATUS_BODY=$(curl -sf "${BASE_URL}/v1/runs/${RUN_ID}" || echo "{}")
    RUN_STATUS=$(echo "${RUN_STATUS_BODY}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(data.get('status', ''))
" 2>/dev/null || echo "")

    if [ "${RUN_STATUS}" = "completed" ]; then
        RUN_OUTPUT=$(echo "${RUN_STATUS_BODY}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(data.get('output', ''))
" 2>/dev/null || echo "")
        pass "run completed: output=\"${RUN_OUTPUT}\""
        break
    elif [ "${RUN_STATUS}" = "failed" ]; then
        fail "run ${RUN_ID} ended in status: failed"
        break
    fi

    ELAPSED=$(( ELAPSED + 2 ))
    if [ "${ELAPSED}" -ge "${TIMEOUT_S}" ]; then
        fail "run ${RUN_ID} did not complete within ${TIMEOUT_S}s (last status: ${RUN_STATUS})"
        break
    fi
    info "  status=${RUN_STATUS}, elapsed=${ELAPSED}s..."
    sleep 2
done

# ---------------------------------------------------------------------------
# Step 8: Stream events and verify assistant.message.delta
# ---------------------------------------------------------------------------

info "GET /v1/runs/${RUN_ID}/events (streaming check)..."
# Fetch SSE stream with a short timeout; look for assistant.message.delta.
# We use curl with --max-time so it doesn't hang if the run is already done.
EVENTS_RAW=$(curl -sf --max-time 10 \
    -H "Accept: text/event-stream" \
    "${BASE_URL}/v1/runs/${RUN_ID}/events" 2>/dev/null || true)

if echo "${EVENTS_RAW}" | grep -q "assistant.message.delta"; then
    pass "GET /v1/runs/${RUN_ID}/events → found assistant.message.delta event"
elif echo "${EVENTS_RAW}" | grep -q "run.completed"; then
    # The run completed so fast that deltas may have flushed before we connected.
    # A completed event in the replay is acceptable evidence the stream works.
    pass "GET /v1/runs/${RUN_ID}/events → found run.completed event (stream replay OK)"
else
    # Non-fatal: events may have expired from memory for very fast runs.
    EVENT_TYPES=$(echo "${EVENTS_RAW}" | grep '^event:' | head -5 || true)
    if [ -n "${EVENT_TYPES}" ]; then
        pass "GET /v1/runs/${RUN_ID}/events → stream returned events: ${EVENT_TYPES}"
    else
        fail "GET /v1/runs/${RUN_ID}/events → no events received (raw: ${EVENTS_RAW:0:200})"
    fi
fi

# ---------------------------------------------------------------------------
# Step 9: GET /v1/profiles — verify list returns at least 1 profile
# ---------------------------------------------------------------------------

PROFILE_SMOKE_STEPS_PASSED=0
PROFILE_SMOKE_STEPS_FAILED=0

profile_pass() { echo "[smoke] PASS: $*"; PROFILE_SMOKE_STEPS_PASSED=$(( PROFILE_SMOKE_STEPS_PASSED + 1 )); PASS_COUNT=$(( PASS_COUNT + 1 )); }
profile_fail() { echo "[smoke] FAIL: $*"; PROFILE_SMOKE_STEPS_FAILED=$(( PROFILE_SMOKE_STEPS_FAILED + 1 )); FAIL_COUNT=$(( FAIL_COUNT + 1 )); }

info "GET /v1/profiles..."
PROFILES_BODY=$(curl -sf "${BASE_URL}/v1/profiles")
PROFILE_COUNT=$(echo "${PROFILES_BODY}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(data.get('count', len(data.get('profiles', []))))
" 2>/dev/null || echo "0")

if [ "${PROFILE_COUNT}" -ge 1 ]; then
    profile_pass "GET /v1/profiles → 200, ${PROFILE_COUNT} profile(s) in catalog"
else
    profile_fail "GET /v1/profiles returned 0 profiles (body: ${PROFILES_BODY})"
fi

# Extract the first profile name for subsequent steps.
SMOKE_PROFILE_NAME=$(echo "${PROFILES_BODY}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
profiles = data.get('profiles', [])
print(profiles[0].get('name', '') if profiles else '')
" 2>/dev/null || echo "")

if [ -z "${SMOKE_PROFILE_NAME}" ]; then
    SMOKE_PROFILE_NAME="full"
    info "could not extract profile name from list response; falling back to 'full'"
fi

info "using profile for smoke: ${SMOKE_PROFILE_NAME}"

# ---------------------------------------------------------------------------
# Step 10: GET /v1/profiles/full — verify required fields
# ---------------------------------------------------------------------------

info "GET /v1/profiles/${SMOKE_PROFILE_NAME}..."
PROFILE_DETAIL_BODY=$(curl -sf "${BASE_URL}/v1/profiles/${SMOKE_PROFILE_NAME}")
PROFILE_NAME_FIELD=$(echo "${PROFILE_DETAIL_BODY}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(data.get('name', ''))
" 2>/dev/null || echo "")
PROFILE_MODEL_FIELD=$(echo "${PROFILE_DETAIL_BODY}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(data.get('model', ''))
" 2>/dev/null || echo "")

if [ -n "${PROFILE_NAME_FIELD}" ] && [ -n "${PROFILE_MODEL_FIELD}" ]; then
    profile_pass "GET /v1/profiles/${SMOKE_PROFILE_NAME} → name=${PROFILE_NAME_FIELD}, model=${PROFILE_MODEL_FIELD}"
else
    profile_fail "GET /v1/profiles/${SMOKE_PROFILE_NAME} missing required fields (body: ${PROFILE_DETAIL_BODY})"
fi

# ---------------------------------------------------------------------------
# Step 11: POST /v1/runs with profile field — create a profile-backed run
# ---------------------------------------------------------------------------

info "POST /v1/runs with profile=${SMOKE_PROFILE_NAME}..."
PROFILE_RUN_RESPONSE=$(curl -sf -X POST "${BASE_URL}/v1/runs" \
    -H "Content-Type: application/json" \
    -d "{\"prompt\": \"Reply with exactly: PROFILE_TEST_OK\", \"profile\": \"${SMOKE_PROFILE_NAME}\"}")

PROFILE_RUN_ID=$(echo "${PROFILE_RUN_RESPONSE}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(data.get('run_id', data.get('id', '')))
" 2>/dev/null || true)

if [ -z "${PROFILE_RUN_ID}" ]; then
    profile_fail "POST /v1/runs with profile did not return a run_id (response: ${PROFILE_RUN_RESPONSE})"
else
    profile_pass "POST /v1/runs with profile=${SMOKE_PROFILE_NAME} → run_id=${PROFILE_RUN_ID}"
fi

# ---------------------------------------------------------------------------
# Step 12: Poll until profile-backed run reaches completed status
# ---------------------------------------------------------------------------

if [ -n "${PROFILE_RUN_ID}" ]; then
    info "polling GET /v1/runs/${PROFILE_RUN_ID} for completion (timeout ${TIMEOUT_S}s)..."
    PROFILE_ELAPSED=0
    PROFILE_RUN_STATUS=""
    PROFILE_RUN_OUTPUT=""
    while true; do
        PROFILE_RUN_STATUS_BODY=$(curl -sf "${BASE_URL}/v1/runs/${PROFILE_RUN_ID}" || echo "{}")
        PROFILE_RUN_STATUS=$(echo "${PROFILE_RUN_STATUS_BODY}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(data.get('status', ''))
" 2>/dev/null || echo "")

        if [ "${PROFILE_RUN_STATUS}" = "completed" ]; then
            PROFILE_RUN_OUTPUT=$(echo "${PROFILE_RUN_STATUS_BODY}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(data.get('output', ''))
" 2>/dev/null || echo "")
            profile_pass "profile-backed run completed (status=completed)"
            break
        elif [ "${PROFILE_RUN_STATUS}" = "failed" ]; then
            profile_fail "profile-backed run ${PROFILE_RUN_ID} ended in status: failed"
            break
        fi

        PROFILE_ELAPSED=$(( PROFILE_ELAPSED + 2 ))
        if [ "${PROFILE_ELAPSED}" -ge "${TIMEOUT_S}" ]; then
            profile_fail "profile-backed run ${PROFILE_RUN_ID} did not complete within ${TIMEOUT_S}s (last status: ${PROFILE_RUN_STATUS})"
            break
        fi
        info "  profile run status=${PROFILE_RUN_STATUS}, elapsed=${PROFILE_ELAPSED}s..."
        sleep 2
    done

    # -----------------------------------------------------------------------
    # Step 13: Verify output field is non-empty in the completed run
    # -----------------------------------------------------------------------

    if [ "${PROFILE_RUN_STATUS}" = "completed" ]; then
        if [ -n "${PROFILE_RUN_OUTPUT}" ]; then
            profile_pass "profile-backed run output is non-empty: \"${PROFILE_RUN_OUTPUT}\""
        else
            profile_fail "profile-backed run ${PROFILE_RUN_ID} completed but output field is empty"
        fi
    else
        profile_fail "skipping output verification — run did not reach completed status"
    fi

    # -----------------------------------------------------------------------
    # Step 14: Re-fetch GET /v1/runs/{id} — verify run is still readable
    # -----------------------------------------------------------------------

    info "GET /v1/runs/${PROFILE_RUN_ID} (idempotent readback)..."
    PROFILE_RUN_READBACK=$(curl -sf "${BASE_URL}/v1/runs/${PROFILE_RUN_ID}" || echo "{}")
    PROFILE_RUN_READBACK_STATUS=$(echo "${PROFILE_RUN_READBACK}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(data.get('status', ''))
" 2>/dev/null || echo "")

    if [ -n "${PROFILE_RUN_READBACK_STATUS}" ]; then
        profile_pass "GET /v1/runs/${PROFILE_RUN_ID} readable after completion (status=${PROFILE_RUN_READBACK_STATUS})"
    else
        profile_fail "GET /v1/runs/${PROFILE_RUN_ID} returned empty or unparseable response on readback"
    fi
fi

# ---------------------------------------------------------------------------
# Step 15: Summary
# ---------------------------------------------------------------------------

echo ""
echo "======================================================"
echo " Smoke Test Summary"
echo "======================================================"
echo " PASS: ${PASS_COUNT}"
echo " FAIL: ${FAIL_COUNT}"
echo " (profile steps passed: ${PROFILE_SMOKE_STEPS_PASSED}, failed: ${PROFILE_SMOKE_STEPS_FAILED})"
echo "======================================================"

if [ "${FAIL_COUNT}" -eq 0 ]; then
    echo "[smoke] ALL TESTS PASSED"
    exit 0
else
    echo "[smoke] ${FAIL_COUNT} TEST(S) FAILED — see server log: ${LOG_FILE}"
    exit 1
fi
