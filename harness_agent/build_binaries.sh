#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN_DIR="$SCRIPT_DIR/bin"
mkdir -p "$BIN_DIR"
cd "$PROJECT_ROOT"
echo "Building harnessd for linux/amd64..."
GOOS=linux GOARCH=amd64 go build -o "$BIN_DIR/harnessd-linux-amd64" ./cmd/harnessd/
echo "Building harnesscli for linux/amd64..."
GOOS=linux GOARCH=amd64 go build -o "$BIN_DIR/harnesscli-linux-amd64" ./cmd/harnesscli/
echo "Done. Binaries in $BIN_DIR/"
ls -lh "$BIN_DIR/"
