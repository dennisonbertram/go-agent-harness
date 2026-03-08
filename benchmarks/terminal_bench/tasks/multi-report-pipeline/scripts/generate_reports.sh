#!/usr/bin/env bash
# generate_reports.sh — Generate reports from metrics CSV.
# BUG: Only generates a partial summary, missing alerts and status-counts.
set -euo pipefail

DATA="data/metrics.csv"

mkdir -p build

# Partial summary — just prints raw lines, no averages, no markdown table
echo "# Server Metrics Summary" > build/summary.md
echo "" >> build/summary.md
tail -n +2 "$DATA" | cut -d',' -f2 | sort -u | while read -r server; do
    echo "- $server" >> build/summary.md
done

# TODO: build/alerts.md — not implemented
# TODO: build/status-counts.json — not implemented
