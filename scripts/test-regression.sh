#!/usr/bin/env bash
set -euo pipefail

MIN_TOTAL_COVERAGE="${MIN_TOTAL_COVERAGE:-80.0}"
COVERPROFILE_PATH="${COVERPROFILE_PATH:-coverage.out}"


echo "[regression] go test ./..."
go test ./...

echo "[regression] go test ./... -race"
go test ./... -race

echo "[regression] go test ./... -coverprofile=${COVERPROFILE_PATH}"
go test ./... -coverprofile="${COVERPROFILE_PATH}"

echo "[regression] coverage gate: min total ${MIN_TOTAL_COVERAGE}% + no zero-coverage functions"
go run ./cmd/coveragegate -coverprofile="${COVERPROFILE_PATH}" -min-total="${MIN_TOTAL_COVERAGE}"

echo "[regression] PASS"
