#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

info() {
  printf '[teardown] %s\n' "$*"
}

die() {
  printf '[teardown] ERROR: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage: teardown.sh [--clean]

Options:
  --clean    Remove the Postgres container (deletes all DB data) and delete
             the .tmp/ directory (built binaries, conversation DB).
             Without this flag, only running processes are stopped and the
             Postgres container is stopped but not removed (data is kept).
  -h, --help Show this help text.
EOF
}

# ---------------------------------------------------------------------------
# Parse flags
# ---------------------------------------------------------------------------

CLEAN=0

while [ $# -gt 0 ]; do
  case "$1" in
    --clean)
      CLEAN=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "Unknown option: $1. Use --help for usage."
      ;;
  esac
done

CONTAINER_NAME="socialagent-postgres"

# ---------------------------------------------------------------------------
# 1. Kill any running harnessd / socialagent processes
# ---------------------------------------------------------------------------

info "Stopping any running harnessd processes..."
if pgrep -f "$APP_DIR/.tmp/bin/harnessd" >/dev/null 2>&1; then
  pkill -f "$APP_DIR/.tmp/bin/harnessd" || true
  info "harnessd stopped."
else
  info "No harnessd process found (already stopped)."
fi

info "Stopping any running socialagent processes..."
if pgrep -f "$APP_DIR/.tmp/bin/socialagent" >/dev/null 2>&1; then
  pkill -f "$APP_DIR/.tmp/bin/socialagent" || true
  info "socialagent stopped."
else
  info "No socialagent process found (already stopped)."
fi

# ---------------------------------------------------------------------------
# 2. Stop (and optionally remove) Postgres container
# ---------------------------------------------------------------------------

if docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$" 2>/dev/null; then
  info "Stopping Postgres container..."
  docker stop "$CONTAINER_NAME"
  info "Postgres container stopped."
else
  info "Postgres container not running (already stopped or not created)."
fi

if [ "$CLEAN" -eq 1 ]; then
  if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$" 2>/dev/null; then
    info "Removing Postgres container (--clean)..."
    docker rm "$CONTAINER_NAME"
    info "Postgres container removed. All database data has been deleted."
  fi

  if [ -d "$APP_DIR/.tmp" ]; then
    info "Removing .tmp/ directory (--clean)..."
    rm -rf "$APP_DIR/.tmp"
    info ".tmp/ removed (binaries and conversation DB deleted)."
  else
    info ".tmp/ directory not found, skipping."
  fi
fi

# ---------------------------------------------------------------------------
# 3. Summary
# ---------------------------------------------------------------------------

if [ "$CLEAN" -eq 1 ]; then
  cat <<EOF

Teardown complete (--clean).

  Postgres container removed. Re-run setup.sh to recreate.
  .tmp/ directory removed. Re-run setup.sh to rebuild binaries.

EOF
else
  cat <<EOF

Teardown complete.

  Postgres data is preserved. Container can be restarted with:
    docker start ${CONTAINER_NAME}
  Or run dev.sh which will restart it automatically.

  To do a full clean (remove DB data and binaries):
    apps/socialagent/scripts/teardown.sh --clean

EOF
fi
