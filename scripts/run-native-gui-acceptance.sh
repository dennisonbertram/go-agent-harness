#!/bin/zsh
# Runs one owner-created core rendered scenario after non-prompting TCC
# preflight. It accepts no caller daemon, driver, manifest, PID, or artifact root.
set -euo pipefail

if (( $# != 0 )); then
  print -u2 'run-native-gui-acceptance accepts no URL, driver, manifest, or positional input'
  exit 2
fi

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"
exec go run ./cmd/native-gui-acceptance -foreground-opt-in
