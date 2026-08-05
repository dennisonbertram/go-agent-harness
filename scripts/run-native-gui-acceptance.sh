#!/bin/zsh
# Runs a repository-owned real rendered-App driver, then fail-closes on the
# single proof pack it produced. This launcher creates the collection boundary;
# it deliberately never discovers, kills, or reuses GoCode/harnessd processes.
set -euo pipefail

: "${NATIVE_GUI_DRIVER:?set a repository-tracked driver that performs actual AX/OCR/screenshot interaction}"
: "${HARNESS_BASE_URL:?set the fresh isolated loopback daemon URL}"

repo_root="$(git rev-parse --show-toplevel)"
driver_dir="$(cd "$(dirname "$NATIVE_GUI_DRIVER")" && pwd -P)"
driver_path="$driver_dir/$(basename "$NATIVE_GUI_DRIVER")"
case "$driver_path" in
  "$repo_root"/*) ;;
  *) print -u2 'native GUI driver must be a repository-owned path'; exit 2 ;;
esac
git -C "$repo_root" ls-files --error-unmatch -- "${driver_path#$repo_root/}" >/dev/null
[[ -x "$driver_path" ]] || { print -u2 'native GUI driver must be executable'; exit 2; }

collection_root="$(mktemp -d "${TMPDIR:-/private/tmp}/native-gui-acceptance.XXXXXXXX")"
artifact_root="$collection_root/artifacts"
mkdir -p "$artifact_root"
nonce="$(openssl rand -hex 16)"
manifest_path="$artifact_root/manifest.json"

export NATIVE_GUI_COLLECTION_LAUNCHER='scripts/run-native-gui-acceptance.sh'
export NATIVE_GUI_COLLECTION_NONCE="$nonce"
export NATIVE_GUI_COLLECTION_TEMP_ROOT="$collection_root"
export NATIVE_GUI_COLLECTION_ARTIFACT_ROOT="$artifact_root"
export NATIVE_GUI_COLLECTION_REPOSITORY_ROOT="$repo_root"
export NATIVE_GUI_COLLECTION_DRIVER_PATH="$driver_path"
export NATIVE_GUI_COLLECTION_DRIVER_DIGEST="sha256:$(shasum -a 256 "$driver_path" | awk '{print $1}')"
export NATIVE_GUI_MANIFEST="$manifest_path"
export NATIVE_GUI_ARTIFACT_ROOT="$artifact_root"

"$driver_path"
go run ./cmd/native-gui-acceptance -harness-url "$HARNESS_BASE_URL" -manifest "$manifest_path" -artifact-root "$artifact_root"
