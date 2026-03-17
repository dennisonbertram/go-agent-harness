#!/usr/bin/env bash
set -e

PORT=${HARNESS_ADDR:-:8080}
PORT_NUM=${PORT#:}

# Kill whatever is on the port
if lsof -ti ":$PORT_NUM" &>/dev/null; then
    echo "Killing process on port $PORT_NUM..."
    lsof -ti ":$PORT_NUM" | xargs kill -9
    sleep 0.3
fi

exec go run ./cmd/harnessd "$@"
