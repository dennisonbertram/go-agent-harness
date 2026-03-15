#!/usr/bin/env bash
set -euo pipefail

MODEL="${1:-openai/gpt-4.1-mini}"
N="${2:-3}"

# Build binaries if not present
if [ ! -f "harness_agent/bin/harnessd-linux-amd64" ]; then
    echo "Building binaries..."
    ./harness_agent/build_binaries.sh
fi

harbor run \
  -d terminal-bench@2.0 \
  --agent-import-path harness_agent.installed_agent:HarnessInstalledAgent \
  -m "$MODEL" \
  -n "$N"
