#!/bin/zsh
# Starts only the native acceptance owner's private lifecycle. This is not a
# rendered scenario runner: it accepts no caller daemon, driver, or manifest.
set -euo pipefail

if (( $# != 0 )); then
  print -u2 'run-native-gui-acceptance accepts no URL, driver, manifest, or positional input'
  exit 2
fi

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"
exec go run ./cmd/native-gui-acceptance -foreground-opt-in
