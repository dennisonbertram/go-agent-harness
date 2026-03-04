#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 || $# -gt 3 ]]; then
  echo "Usage: $0 <feature-branch> <test-command> [main-branch]"
  echo "Example: $0 feature/login \"go test ./...\" main"
  exit 1
fi

FEATURE_BRANCH="$1"
TEST_COMMAND="$2"
MAIN_BRANCH="${3:-main}"
HAS_ORIGIN_REMOTE=0

if git remote get-url origin >/dev/null 2>&1; then
  HAS_ORIGIN_REMOTE=1
fi

CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [[ "$CURRENT_BRANCH" != "$FEATURE_BRANCH" ]]; then
  echo "Error: current branch is '$CURRENT_BRANCH', expected '$FEATURE_BRANCH'."
  echo "Switch to your feature branch before running this script."
  exit 1
fi

echo "Running tests on '$FEATURE_BRANCH': $TEST_COMMAND"
bash -lc "$TEST_COMMAND"

echo "Tests passed. Syncing and merging into '$MAIN_BRANCH'."
if [[ "$HAS_ORIGIN_REMOTE" -eq 1 ]]; then
  git fetch origin "$MAIN_BRANCH" || true
fi

if [[ "$HAS_ORIGIN_REMOTE" -eq 1 ]] && git show-ref --verify --quiet "refs/remotes/origin/$MAIN_BRANCH"; then
  if git show-ref --verify --quiet "refs/heads/$MAIN_BRANCH"; then
    git checkout "$MAIN_BRANCH"
    git pull --ff-only origin "$MAIN_BRANCH"
  else
    git checkout -b "$MAIN_BRANCH" "origin/$MAIN_BRANCH"
  fi
else
  git checkout "$MAIN_BRANCH"
fi

if git merge --ff-only "$FEATURE_BRANCH"; then
  echo "Fast-forward merge completed."
else
  echo "Fast-forward not possible; creating merge commit."
  git merge --no-ff "$FEATURE_BRANCH" -m "Merge $FEATURE_BRANCH into $MAIN_BRANCH after passing tests"
fi

echo "Running post-merge tests on '$MAIN_BRANCH': $TEST_COMMAND"
bash -lc "$TEST_COMMAND"

if [[ "$HAS_ORIGIN_REMOTE" -eq 1 ]]; then
  echo "Pushing '$MAIN_BRANCH' to origin."
  git push origin "$MAIN_BRANCH"
else
  echo "No origin remote configured. Skipping push."
fi

echo "Merged '$FEATURE_BRANCH' -> '$MAIN_BRANCH'."
echo "Workflow complete."
