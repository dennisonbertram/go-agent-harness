#!/usr/bin/env bash
# curl-run.sh — Send a prompt to the harness API and poll until completion.
#
# Usage:
#   ./scripts/curl-run.sh "prompt text" [conversation_id] [base_url]
#
# Arguments:
#   prompt          Required. The prompt text to send to the harness.
#   conversation_id Optional. An existing conversation ID to continue a run in.
#   base_url        Optional. Defaults to http://localhost:8080
#
# Why jq / python3 for JSON construction?
#   Shell escaping is notoriously fragile when prompt text contains quotes,
#   newlines, backslashes, or special characters. Building JSON by hand with
#   printf/echo leads to silent truncation or malformed payloads that are hard
#   to debug. jq and python3's json.dumps() handle all escaping correctly and
#   deterministically, so we delegate JSON construction to one of them rather
#   than relying on the shell.

set -euo pipefail

# ---------------------------------------------------------------------------
# Arguments
# ---------------------------------------------------------------------------
if [[ $# -lt 1 ]]; then
  echo "Usage: $0 \"prompt text\" [conversation_id] [base_url]" >&2
  exit 1
fi

PROMPT="${1}"
CONVERSATION_ID="${2:-}"
BASE_URL="${3:-http://localhost:8080}"

# ---------------------------------------------------------------------------
# Build the JSON payload
#
# Preference order:
#   1. jq  — purpose-built for JSON, widely available on developer machines
#   2. python3 — available on virtually every macOS/Linux system; json.dumps()
#                handles all edge cases (unicode, control chars, etc.)
#   3. fail loudly — rather than produce a silently broken payload
# ---------------------------------------------------------------------------
build_payload() {
  if command -v jq &>/dev/null; then
    # jq -n reads no input; --arg passes the shell variable as a JSON string,
    # handling all escaping automatically.
    if [[ -n "${CONVERSATION_ID}" ]]; then
      jq -n \
        --arg prompt          "${PROMPT}" \
        --arg conversation_id "${CONVERSATION_ID}" \
        '{prompt: $prompt, conversation_id: $conversation_id}'
    else
      jq -n \
        --arg prompt "${PROMPT}" \
        '{prompt: $prompt}'
    fi
  elif command -v python3 &>/dev/null; then
    # python3 json.dumps() is equally safe — it escapes everything that JSON
    # requires, including backslashes, quotes, and non-ASCII characters.
    if [[ -n "${CONVERSATION_ID}" ]]; then
      python3 -c "
import json, sys
payload = {'prompt': sys.argv[1], 'conversation_id': sys.argv[2]}
print(json.dumps(payload))
" "${PROMPT}" "${CONVERSATION_ID}"
    else
      python3 -c "
import json, sys
payload = {'prompt': sys.argv[1]}
print(json.dumps(payload))
" "${PROMPT}"
    fi
  else
    echo "Error: neither 'jq' nor 'python3' is available." >&2
    echo "Install one of them to safely construct the JSON payload." >&2
    exit 1
  fi
}

# ---------------------------------------------------------------------------
# POST /v1/runs — start a new run
# ---------------------------------------------------------------------------
PAYLOAD="$(build_payload)"

echo "Sending prompt to ${BASE_URL}/v1/runs ..."
if [[ -n "${CONVERSATION_ID}" ]]; then
  echo "  conversation_id: ${CONVERSATION_ID}"
fi

RESPONSE="$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -d "${PAYLOAD}" \
  "${BASE_URL}/v1/runs")"

# Extract the run ID from the response.
# Prefer jq; fall back to python3 for the same escaping-safety reasons.
if command -v jq &>/dev/null; then
  RUN_ID="$(echo "${RESPONSE}" | jq -r '.id // empty')"
else
  RUN_ID="$(echo "${RESPONSE}" | python3 -c "
import json, sys
data = json.load(sys.stdin)
print(data.get('id', ''))
")"
fi

if [[ -z "${RUN_ID}" ]]; then
  echo "Error: failed to obtain a run ID from the API response." >&2
  echo "Response was:" >&2
  echo "${RESPONSE}" >&2
  exit 1
fi

echo "Run started: ${RUN_ID}"

# ---------------------------------------------------------------------------
# Poll GET /v1/runs/{id} until the run reaches a terminal state
# ---------------------------------------------------------------------------
POLL_INTERVAL=2   # seconds between polls
MAX_POLLS=300     # give up after ~10 minutes (300 * 2s)
POLLS=0

echo "Polling for completion (interval: ${POLL_INTERVAL}s, max wait: $((MAX_POLLS * POLL_INTERVAL))s) ..."

while true; do
  POLLS=$(( POLLS + 1 ))
  if [[ ${POLLS} -gt ${MAX_POLLS} ]]; then
    echo "Error: timed out waiting for run ${RUN_ID} to complete." >&2
    exit 1
  fi

  STATUS_RESPONSE="$(curl -s "${BASE_URL}/v1/runs/${RUN_ID}")"

  if command -v jq &>/dev/null; then
    STATUS="$(echo "${STATUS_RESPONSE}" | jq -r '.status // empty')"
    RESULT="$(echo "${STATUS_RESPONSE}" | jq -r '.result // empty')"
    ERROR="$(echo  "${STATUS_RESPONSE}" | jq -r '.error  // empty')"
  else
    STATUS="$(echo "${STATUS_RESPONSE}" | python3 -c "
import json, sys
data = json.load(sys.stdin)
print(data.get('status', ''))
")"
    RESULT="$(echo "${STATUS_RESPONSE}" | python3 -c "
import json, sys
data = json.load(sys.stdin)
print(data.get('result', ''))
")"
    ERROR="$(echo "${STATUS_RESPONSE}" | python3 -c "
import json, sys
data = json.load(sys.stdin)
print(data.get('error', ''))
")"
  fi

  echo "  status: ${STATUS}"

  case "${STATUS}" in
    completed)
      echo ""
      echo "=== Run completed ==="
      if [[ -n "${RESULT}" ]]; then
        echo "${RESULT}"
      else
        # Print the full response so callers can inspect all fields.
        echo "${STATUS_RESPONSE}"
      fi
      exit 0
      ;;
    failed)
      echo "" >&2
      echo "=== Run failed ===" >&2
      if [[ -n "${ERROR}" ]]; then
        echo "Error: ${ERROR}" >&2
      else
        echo "${STATUS_RESPONSE}" >&2
      fi
      exit 1
      ;;
    "")
      echo "Warning: received empty status; raw response:" >&2
      echo "${STATUS_RESPONSE}" >&2
      ;;
  esac

  sleep "${POLL_INTERVAL}"
done
