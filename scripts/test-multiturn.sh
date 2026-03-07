#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
FAILED=0

# Check jq is installed
if ! command -v jq &>/dev/null; then
  echo "ERROR: jq is required but not installed." >&2
  exit 1
fi

# Check server health
echo "Checking server health at ${BASE_URL}/healthz ..."
health_status=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/healthz")
if [[ "${health_status}" != "200" ]]; then
  echo "ERROR: Server health check failed (HTTP ${health_status}). Is the server running?" >&2
  exit 1
fi
echo "Server is healthy."
echo ""

# Poll for run completion. Args: run_id
# Returns the final status string or exits non-zero on timeout.
poll_run() {
  local run_id="$1"
  local max_attempts=60
  local attempt=0
  while [[ ${attempt} -lt ${max_attempts} ]]; do
    local response
    response=$(curl -s "${BASE_URL}/v1/runs/${run_id}")
    local status
    status=$(echo "${response}" | jq -r '.status // empty')
    case "${status}" in
      completed|failed|error)
        echo "${status}"
        return 0
        ;;
      "")
        echo "ERROR: Could not parse status from response: ${response}" >&2
        echo "error"
        return 0
        ;;
    esac
    sleep 1
    attempt=$((attempt + 1))
  done
  echo "timeout"
}

# Run a single test case. Args: label prompt [conversation_id]
# Prints PASS or FAIL and updates FAILED.
run_test() {
  local label="$1"
  local prompt="$2"
  local conversation_id="${3:-}"

  local payload
  if [[ -n "${conversation_id}" ]]; then
    payload=$(jq -n --arg p "${prompt}" --arg cid "${conversation_id}" \
      '{"prompt": $p, "conversation_id": $cid}')
  else
    payload=$(jq -n --arg p "${prompt}" \
      '{"prompt": $p}')
  fi

  local response
  response=$(curl -s -X POST "${BASE_URL}/v1/runs" \
    -H "Content-Type: application/json" \
    -d "${payload}")

  local run_id
  run_id=$(echo "${response}" | jq -r '.id // empty')

  if [[ -z "${run_id}" ]]; then
    echo "FAIL [${label}]: Could not get run ID from response: ${response}"
    FAILED=$((FAILED + 1))
    return
  fi

  local final_status
  final_status=$(poll_run "${run_id}")

  if [[ "${final_status}" == "completed" ]]; then
    echo "PASS [${label}]: run_id=${run_id} status=${final_status}"
  else
    echo "FAIL [${label}]: run_id=${run_id} status=${final_status}"
    FAILED=$((FAILED + 1))
  fi
}

# Returns the conversation_id from a newly created run. Args: prompt
# Prints the conversation_id or exits non-zero.
create_run_get_conversation_id() {
  local prompt="$1"
  local payload
  payload=$(jq -n --arg p "${prompt}" '{"prompt": $p}')

  local response
  response=$(curl -s -X POST "${BASE_URL}/v1/runs" \
    -H "Content-Type: application/json" \
    -d "${payload}")

  local run_id
  run_id=$(echo "${response}" | jq -r '.id // empty')

  if [[ -z "${run_id}" ]]; then
    echo "ERROR: Could not get run ID for multi-turn setup: ${response}" >&2
    return 1
  fi

  # Poll to completion
  local final_status
  final_status=$(poll_run "${run_id}")
  if [[ "${final_status}" != "completed" ]]; then
    echo "ERROR: Setup run did not complete (status=${final_status})" >&2
    return 1
  fi

  # Fetch the run details to get conversation_id
  local run_detail
  run_detail=$(curl -s "${BASE_URL}/v1/runs/${run_id}")
  local conversation_id
  conversation_id=$(echo "${run_detail}" | jq -r '.conversation_id // empty')

  if [[ -z "${conversation_id}" ]]; then
    echo "ERROR: Could not extract conversation_id from run: ${run_detail}" >&2
    return 1
  fi

  echo "${conversation_id}"
}

echo "=== Test 1: Simple greeting ==="
run_test "simple greeting" "Hello! How are you?"

echo ""
echo "=== Test 2: Mixed quotes ==="
run_test "mixed quotes" "It's a \"test\" with 'quotes'"

echo ""
echo "=== Test 3: Shell metacharacters ==="
run_test "shell metacharacters" 'echo $HOME && ls | grep foo'

echo ""
echo "=== Test 4: Unicode / emoji ==="
run_test "unicode emoji" "Hello 🌍 world"

echo ""
echo "=== Test 5: Embedded JSON ==="
run_test "embedded JSON" 'Parse this: {"key": "value"}'

echo ""
echo "=== Test 6: Multi-turn conversation ==="
echo "  Step 1: Creating initial run to obtain conversation_id ..."
conversation_id=$(create_run_get_conversation_id "Start a conversation. Reply with the word READY." 2>&1) || {
  echo "FAIL [multi-turn setup]: ${conversation_id}"
  FAILED=$((FAILED + 1))
  conversation_id=""
}

if [[ -n "${conversation_id}" ]]; then
  echo "  Step 1 PASS: conversation_id=${conversation_id}"
  echo "  Step 2: Continuing conversation ..."
  run_test "multi-turn continuation" "Continue the conversation. What did you say before?" "${conversation_id}"
fi

echo ""
if [[ ${FAILED} -eq 0 ]]; then
  echo "All tests PASSED."
  exit 0
else
  echo "${FAILED} test(s) FAILED."
  exit 1
fi
