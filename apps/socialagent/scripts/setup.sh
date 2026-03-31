#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(git -C "$APP_DIR" rev-parse --show-toplevel)"

info() {
  printf '[setup] %s\n' "$*"
}

die() {
  printf '[setup] ERROR: %s\n' "$*" >&2
  exit 1
}

on_error() {
  local line="$1"
  local command="$2"
  printf '[setup] ERROR: command failed at line %s: %s\n' "$line" "$command" >&2
  exit 1
}

trap 'on_error "$LINENO" "$BASH_COMMAND"' ERR

# ---------------------------------------------------------------------------
# 1. Check prerequisites
# ---------------------------------------------------------------------------

require_command() {
  local cmd="$1"
  local hint="${2:-}"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    if [ -n "$hint" ]; then
      die "required command not found: $cmd. $hint"
    fi
    die "required command not found: $cmd"
  fi
}

require_command docker "Install Docker Desktop from https://www.docker.com/products/docker-desktop"
require_command go "Install Go from https://go.dev/dl/"
require_command openssl "Install openssl (macOS: brew install openssl)"

# Verify Docker daemon is running
if ! docker info >/dev/null 2>&1; then
  die "Docker daemon is not running. Start Docker Desktop and try again."
fi

# ---------------------------------------------------------------------------
# 2. Start Postgres via Docker (idempotent)
# ---------------------------------------------------------------------------

CONTAINER_NAME="socialagent-postgres"
PG_PORT=5433
PG_USER=socialagent
PG_PASS=socialagent
PG_DB=socialagent

if docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
  info "Postgres container already running."
else
  # Check if container exists but is stopped
  if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    info "Starting existing Postgres container..."
    docker start "$CONTAINER_NAME"
  else
    info "Creating and starting Postgres container..."
    docker run -d \
      --name "$CONTAINER_NAME" \
      -e POSTGRES_USER="$PG_USER" \
      -e POSTGRES_PASSWORD="$PG_PASS" \
      -e POSTGRES_DB="$PG_DB" \
      -p "${PG_PORT}:5432" \
      --restart unless-stopped \
      postgres:16-alpine
  fi

  info "Waiting for Postgres to be ready..."
  for i in $(seq 1 30); do
    if docker exec "$CONTAINER_NAME" pg_isready -U "$PG_USER" >/dev/null 2>&1; then
      info "Postgres is ready."
      break
    fi
    if [ "$i" -eq 30 ]; then
      die "Postgres did not become ready within 30 seconds."
    fi
    sleep 1
  done
fi

# ---------------------------------------------------------------------------
# 3. Create .env file (idempotent)
# ---------------------------------------------------------------------------

ENV_FILE="$APP_DIR/.env"

if [ ! -f "$ENV_FILE" ]; then
  cp "$APP_DIR/.env.example" "$ENV_FILE"

  # Generate a random webhook secret
  WEBHOOK_SECRET=$(openssl rand -hex 32)
  sed -i.bak "s/^TELEGRAM_WEBHOOK_SECRET=$/TELEGRAM_WEBHOOK_SECRET=${WEBHOOK_SECRET}/" "$ENV_FILE"
  rm -f "$ENV_FILE.bak"

  info "Created $ENV_FILE"
else
  info ".env file already exists, skipping creation."
fi

# ---------------------------------------------------------------------------
# 4. Auto-fill OPENAI_API_KEY from environment if blank in .env
# ---------------------------------------------------------------------------

if grep -q "^OPENAI_API_KEY=$" "$ENV_FILE" 2>/dev/null; then
  if [ -n "${OPENAI_API_KEY:-}" ]; then
    sed -i.bak "s/^OPENAI_API_KEY=$/OPENAI_API_KEY=${OPENAI_API_KEY}/" "$ENV_FILE"
    rm -f "$ENV_FILE.bak"
    info "Filled OPENAI_API_KEY from environment."
  fi
fi

# ---------------------------------------------------------------------------
# 5. Build binaries
# ---------------------------------------------------------------------------

BIN_DIR="$APP_DIR/.tmp/bin"
mkdir -p "$BIN_DIR"

info "Building harnessd..."
(cd "$REPO_ROOT" && go build -o "$BIN_DIR/harnessd" ./cmd/harnessd/)

info "Building socialagent..."
(cd "$REPO_ROOT" && go build -o "$BIN_DIR/socialagent" ./apps/socialagent/)

# ---------------------------------------------------------------------------
# 6. Print summary
# ---------------------------------------------------------------------------

cat <<EOF

Setup complete!

  Postgres:  localhost:${PG_PORT}  (container: ${CONTAINER_NAME})
  Binaries:  apps/socialagent/.tmp/bin/
  Config:    apps/socialagent/.env

NOTE: apps/socialagent/.env and apps/socialagent/.tmp/ are gitignored.
      Never commit the .env file — it contains secrets.

EOF

# Check which required fields are still empty
NEEDS_FILL=()
if grep -q "^TELEGRAM_BOT_TOKEN=$" "$ENV_FILE" 2>/dev/null; then
  NEEDS_FILL+=("TELEGRAM_BOT_TOKEN  (from @BotFather on Telegram)")
fi
if grep -q "^OPENAI_API_KEY=$" "$ENV_FILE" 2>/dev/null; then
  NEEDS_FILL+=("OPENAI_API_KEY      (your OpenAI API key)")
fi

if [ ${#NEEDS_FILL[@]} -gt 0 ]; then
  echo "Next steps:"
  echo "  1. Fill in the following in apps/socialagent/.env:"
  for field in "${NEEDS_FILL[@]}"; do
    echo "       - $field"
  done
  echo "  2. Run: apps/socialagent/scripts/dev.sh"
else
  echo "Next steps:"
  echo "  Run: apps/socialagent/scripts/dev.sh"
fi
