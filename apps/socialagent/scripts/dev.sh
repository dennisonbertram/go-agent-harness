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
# 3. Check Postgres is running
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
# 4. Build binaries if missing
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
# 5. Start harnessd in background
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
# 6. Start socialagent in foreground (trap kills harnessd on exit)
# ---------------------------------------------------------------------------

cleanup() {
  info "Stopping harnessd (PID: $HARNESSD_PID)..."
  kill "$HARNESSD_PID" 2>/dev/null || true
  wait "$HARNESSD_PID" 2>/dev/null || true
}
trap cleanup EXIT

info "Starting socialagent on ${LISTEN_ADDR:-:8081}..."
"$BIN_DIR/socialagent"
