#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

info() {
  printf '[dev] %s\n' "$*"
}

die() {
  printf '[dev] ERROR: %s\n' "$*" >&2
  exit 1
}

on_error() {
  local line="$1"
  local command="$2"
  printf '[dev] ERROR: command failed at line %s: %s\n' "$line" "$command" >&2
  exit 1
}

trap 'on_error "$LINENO" "$BASH_COMMAND"' ERR

# ---------------------------------------------------------------------------
# 1. Find and source .env (worktree-aware)
# ---------------------------------------------------------------------------

ENV_FILE="$APP_DIR/.env"

if [ ! -f "$ENV_FILE" ]; then
  # Worktree support: look for .env in the main worktree
  MAIN_WORKTREE="$(git worktree list --porcelain | head -1 | awk '{print $2}')"
  MAIN_ENV="$MAIN_WORKTREE/apps/socialagent/.env"
  if [ -f "$MAIN_ENV" ]; then
    info "Using .env from main worktree: $MAIN_ENV"
    ENV_FILE="$MAIN_ENV"
  else
    die "No .env file found at $APP_DIR/.env or $MAIN_ENV. Run setup.sh first."
  fi
fi

set -a  # auto-export all variables
# shellcheck source=/dev/null
source "$ENV_FILE"
set +a

# ---------------------------------------------------------------------------
# 2. Validate required env vars
# ---------------------------------------------------------------------------

MISSING=()
[ -z "${TELEGRAM_BOT_TOKEN:-}" ] && MISSING+=("TELEGRAM_BOT_TOKEN")
[ -z "${OPENAI_API_KEY:-}" ] && MISSING+=("OPENAI_API_KEY")
[ -z "${TELEGRAM_WEBHOOK_SECRET:-}" ] && MISSING+=("TELEGRAM_WEBHOOK_SECRET")
[ -z "${DATABASE_URL:-}" ] && MISSING+=("DATABASE_URL")

if [ ${#MISSING[@]} -gt 0 ]; then
  die "Missing required env vars: ${MISSING[*]}. Edit $ENV_FILE to fill them in."
fi

# ---------------------------------------------------------------------------
# 3. Check prerequisites
# ---------------------------------------------------------------------------

if ! command -v ngrok &>/dev/null; then
    echo "ERROR: ngrok is not installed. Install from https://ngrok.com/download"
    exit 1
fi

# ---------------------------------------------------------------------------
# 4. Check Postgres is running
# ---------------------------------------------------------------------------

CONTAINER_NAME="socialagent-postgres"

if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
  info "Postgres not running. Attempting to start..."
  if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    docker start "$CONTAINER_NAME"
    info "Waiting for Postgres to be ready..."
    for i in $(seq 1 15); do
      if docker exec "$CONTAINER_NAME" pg_isready -U socialagent >/dev/null 2>&1; then
        info "Postgres is ready."
        break
      fi
      if [ "$i" -eq 15 ]; then
        die "Postgres did not become ready in time. Check container logs: docker logs $CONTAINER_NAME"
      fi
      sleep 1
    done
  else
    die "Postgres container not found. Run setup.sh first."
  fi
fi

# ---------------------------------------------------------------------------
# 5. Build binaries if missing
# ---------------------------------------------------------------------------

REPO_ROOT="$(git -C "$APP_DIR" rev-parse --show-toplevel)"
BIN_DIR="$APP_DIR/.tmp/bin"
mkdir -p "$BIN_DIR"

if [ ! -f "$BIN_DIR/harnessd" ] || [ ! -f "$BIN_DIR/socialagent" ]; then
  info "One or more binaries missing. Building..."
  (cd "$REPO_ROOT" && go build -o "$BIN_DIR/harnessd" ./cmd/harnessd/)
  (cd "$REPO_ROOT" && go build -o "$BIN_DIR/socialagent" ./apps/socialagent/)
  info "Build complete."
fi

# ---------------------------------------------------------------------------
# 6. Start harnessd in background
# ---------------------------------------------------------------------------

export HARNESS_CONVERSATION_DB="${HARNESS_CONVERSATION_DB:-$APP_DIR/.tmp/conversations.db}"
export HARNESS_AUTH_DISABLED="${HARNESS_AUTH_DISABLED:-true}"
export HARNESS_ADDR="${HARNESS_ADDR:-:8080}"

info "Starting harnessd on ${HARNESS_ADDR}..."
"$BIN_DIR/harnessd" &
HARNESSD_PID=$!

# Wait for harnessd to be ready
HARNESS_READY=0
for i in $(seq 1 30); do
  if curl -s "http://localhost:8080/healthz" >/dev/null 2>&1; then
    HARNESS_READY=1
    info "harnessd is ready (PID: $HARNESSD_PID)"
    break
  fi
  sleep 1
done

if [ "$HARNESS_READY" -eq 0 ]; then
  kill "$HARNESSD_PID" 2>/dev/null || true
  die "harnessd did not become ready within 30 seconds. Check logs above."
fi

# ---------------------------------------------------------------------------
# 7. Start ngrok tunnel
# ---------------------------------------------------------------------------

# Extract port number from ":8081" style LISTEN_ADDR
SOCIALAGENT_PORT="${LISTEN_ADDR#:}"
SOCIALAGENT_PORT="${SOCIALAGENT_PORT:-8081}"

info "Starting ngrok tunnel on port ${SOCIALAGENT_PORT}..."
ngrok http "$SOCIALAGENT_PORT" --log=stdout --log-level=warn > /dev/null &
NGROK_PID=$!
sleep 2  # give ngrok time to establish tunnel

# Get the public URL from ngrok's local API
NGROK_URL=""
for i in $(seq 1 10); do
    NGROK_URL=$(curl -s http://localhost:4040/api/tunnels | grep -o '"public_url":"https://[^"]*' | grep -o 'https://[^"]*' | head -1)
    if [ -n "$NGROK_URL" ]; then
        break
    fi
    sleep 1
done

if [ -z "$NGROK_URL" ]; then
    echo "ERROR: Could not get ngrok public URL. Is ngrok running?"
    kill $HARNESSD_PID 2>/dev/null
    exit 1
fi

info "ngrok tunnel: $NGROK_URL"

# ---------------------------------------------------------------------------
# 8. Register Telegram webhook
# ---------------------------------------------------------------------------

WEBHOOK_URL="${NGROK_URL}/webhook/telegram"
info "Registering Telegram webhook: $WEBHOOK_URL"

WEBHOOK_RESPONSE=$(curl -s -X POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/setWebhook" \
    -H "Content-Type: application/json" \
    -d "{\"url\": \"${WEBHOOK_URL}\", \"secret_token\": \"${TELEGRAM_WEBHOOK_SECRET}\"}")

if echo "$WEBHOOK_RESPONSE" | grep -q '"ok":true'; then
    echo "✅ Telegram webhook registered successfully"
else
    echo "⚠️  Webhook registration response: $WEBHOOK_RESPONSE"
fi

# ---------------------------------------------------------------------------
# 9. Start socialagent in foreground (trap kills harnessd + ngrok on exit)
# ---------------------------------------------------------------------------

cleanup() {
    echo ""
    echo "Shutting down..."

    # Deregister Telegram webhook
    echo "Removing Telegram webhook..."
    curl -s -X POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/setWebhook" \
        -H "Content-Type: application/json" \
        -d '{"url": ""}' > /dev/null 2>&1

    # Kill processes
    echo "Stopping socialagent..."
    echo "Stopping ngrok..."
    kill $NGROK_PID 2>/dev/null
    echo "Stopping harnessd..."
    kill $HARNESSD_PID 2>/dev/null
    wait $NGROK_PID 2>/dev/null
    wait $HARNESSD_PID 2>/dev/null
    echo "Done."
}
trap cleanup EXIT INT TERM

echo ""
echo "========================================="
echo "  Social Agent is starting"
echo "========================================="
echo "  Harness:    http://localhost:8080"
echo "  Agent:      http://localhost:${SOCIALAGENT_PORT}"
echo "  Tunnel:     ${NGROK_URL}"
echo "  Webhook:    ${WEBHOOK_URL}"
echo "  Bot:        https://t.me/goAgentHarnessBot"
echo "========================================="
echo "  Send a message to @goAgentHarnessBot on Telegram!"
echo "  Press Ctrl+C to stop."
echo "========================================="
echo ""

info "Starting socialagent on ${LISTEN_ADDR:-:8081}..."
"$BIN_DIR/socialagent"
