#!/usr/bin/env bash
set -euo pipefail

cat <<'EOF' > build/incident-summary.md
# Incident Summary

- worker: 1 incident
- api: 2 incidents
EOF
