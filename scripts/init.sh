#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DEFAULT_WORKTREE_ROOT="${REPO_ROOT}/.codex-worktrees"
DEFAULT_BASE_REF="main"
DEFAULT_BRANCH_PREFIX="${INIT_BRANCH_PREFIX:-codex}"
SCRIPT_NAME="scripts/init.sh"

# scripts/init.sh owns a fresh checkout and must never let ambient Git
# environment variables redirect that checkout's creation or its build
# provenance. In particular, Go 1.26 does not reliably discover the intended
# worktree through its .git indirection file, and can otherwise stamp a binary
# with dirty metadata from the parent checkout.
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_COMMON_DIR GIT_OBJECT_DIRECTORY

usage() {
  cat <<'EOF'
Usage:
  scripts/init.sh [options] <task-slug>

Options:
  --base-ref <ref>       Base ref used when creating a new worktree (default: main)
  --branch <name>        Git branch name for the worktree (default: codex/<task-slug>)
  --worktree-root <dir>  Directory that stores worktrees (default: .codex-worktrees)
  --session <name>       Start harnessd in tmux with this session name
  --start-server         Start harnessd in tmux after bootstrapping
  --skip-build           Skip the local go build step
  --skip-download        Skip go mod download
  --check                Verify prerequisites and exit without creating a worktree
  -h, --help             Show this help text

Examples:
  scripts/init.sh issue-361
  scripts/init.sh --base-ref main --start-server issue-361
  scripts/init.sh --check
EOF
}

info() {
  printf '[init] %s\n' "$*"
}

warn() {
  printf '[init] WARN: %s\n' "$*" >&2
}

die() {
  printf '[init] ERROR: %s\n' "$*" >&2
  exit 1
}

on_error() {
  local line="$1"
  local command="$2"
  printf '[init] ERROR: command failed at line %s\n' "$line" >&2
  printf '[init] ERROR: %s\n' "$command" >&2
  printf '[init] ERROR: rerun with --help to review options, or use bash -x for a trace.\n' >&2
  exit 1
}

trap 'on_error "$LINENO" "$BASH_COMMAND"' ERR

bootstrap_staging_root=""

cleanup_bootstrap_staging() {
  if [[ -n "${bootstrap_staging_root}" && -d "${bootstrap_staging_root}" ]]; then
    rm -rf -- "${bootstrap_staging_root}"
  fi
}

trap cleanup_bootstrap_staging EXIT

require_command() {
  local command_name="$1"
  local hint="${2:-}"
  if ! command -v "$command_name" >/dev/null 2>&1; then
    if [[ -n "${hint}" ]]; then
      die "required command not found: ${command_name}. ${hint}"
    fi
    die "required command not found: ${command_name}"
  fi
}

# Resolve the source before inspecting or creating a worktree.  A fetched
# remote-tracking ref (rather than FETCH_HEAD) is stable when another Git
# process fetches concurrently, while a validated object ID lets callers pin
# a bootstrap to one exact commit without attempting to fetch a branch named
# after that SHA.
resolve_base_ref() {
  if [[ "${base_ref}" =~ ^[0-9a-fA-F]{7,64}$ ]]; then
    if ! resolved_base_ref="$(git rev-parse --verify "${base_ref}^{commit}")"; then
      die "could not resolve commit SHA ${base_ref}. Ensure the object exists locally or fetch it before bootstrapping."
    fi
    return
  fi

  if [[ "${base_ref}" == refs/heads/* ]]; then
    if ! resolved_base_ref="$(git rev-parse --verify "${base_ref}^{commit}")"; then
      die "could not resolve explicit local base ref ${base_ref} to a commit"
    fi
    return
  fi

  if git remote get-url origin >/dev/null 2>&1; then
    remote_base_ref="${base_ref#origin/}"
    remote_base_ref="${remote_base_ref#refs/remotes/origin/}"
    [[ -n "${remote_base_ref}" ]] || die "--base-ref must name a branch or commit, not origin/"
    info "fetching origin/${remote_base_ref}"
    if ! git fetch origin "${remote_base_ref}" >/dev/null; then
      die "could not fetch origin/${remote_base_ref}. If you are offline, use a local commit SHA that already exists."
    fi
    if ! resolved_base_ref="$(git rev-parse --verify "refs/remotes/origin/${remote_base_ref}^{commit}")"; then
      die "could not resolve fetched origin/${remote_base_ref} to a remote-tracking commit"
    fi
    return
  fi

  warn "origin remote is not configured. Continuing with the local ${base_ref} ref only."
  if ! resolved_base_ref="$(git rev-parse --verify "${base_ref}^{commit}")"; then
    die "could not resolve local base ref ${base_ref} to a commit"
  fi
}

bootstrap_build_binary() {
  local output_path="$1"
  local package_path="$2"
  local candidate_path="${output_path}.candidate.$$"
  local build_info revision modified vcs

  rm -f "${candidate_path}"
  # Go 1.26 does not discover VCS metadata through a linked worktree's .git
  # indirection file. Build from the isolated clone below, whose .git is a
  # real directory, rather than trusting ambient Git environment overrides.
  if ! (
    cd "${bootstrap_build_worktree}"
    env -u GIT_DIR -u GIT_WORK_TREE -u GIT_INDEX_FILE -u GIT_COMMON_DIR -u GIT_OBJECT_DIRECTORY \
      go build -buildvcs=true -o "${candidate_path}" "${package_path}"
  ); then
    rm -f "${candidate_path}" "${output_path}"
    printf '[init] ERROR: bootstrap build failed for %s\n' "${package_path}" >&2
    return 1
  fi

  if ! build_info="$(go version -m "${candidate_path}")"; then
    rm -f "${candidate_path}" "${output_path}"
    printf '[init] ERROR: bootstrap provenance rejected: could not read build metadata for %s\n' "${candidate_path}" >&2
    return 1
  fi
  vcs="$(printf '%s\n' "${build_info}" | awk '$1 == "build" && $2 == "vcs=git" { print "git"; exit }')"
  revision="$(printf '%s\n' "${build_info}" | awk '$1 == "build" && $2 ~ /^vcs.revision=/ { sub(/^vcs.revision=/, "", $2); print $2; exit }')"
  modified="$(printf '%s\n' "${build_info}" | awk '$1 == "build" && $2 ~ /^vcs.modified=/ { sub(/^vcs.modified=/, "", $2); print $2; exit }')"
  if [[ "${vcs}" != "git" || "${revision}" != "${bootstrap_revision}" || "${modified}" != "false" ]]; then
    rm -f "${candidate_path}" "${output_path}"
    printf '[init] ERROR: bootstrap provenance rejected: expected clean git revision %s; got revision=%s modified=%s\n' \
      "${bootstrap_revision}" "${revision:-missing}" "${modified:-missing}" >&2
    return 1
  fi
  if ! mv -f "${candidate_path}" "${output_path}"; then
    rm -f "${candidate_path}" "${output_path}"
    printf '[init] ERROR: bootstrap provenance rejected: could not publish verified binary %s\n' "${output_path}" >&2
    return 1
  fi
}

task_slug=""
base_ref="${DEFAULT_BASE_REF}"
branch=""
branch_explicit=0
worktree_root="${DEFAULT_WORKTREE_ROOT}"
session_name=""
start_server=0
skip_build=0
skip_download=0
check_only=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --base-ref)
      [[ $# -ge 2 ]] || die "--base-ref requires a value"
      base_ref="$2"
      shift 2
      ;;
    --branch)
      [[ $# -ge 2 ]] || die "--branch requires a value"
      branch="$2"
      branch_explicit=1
      shift 2
      ;;
    --worktree-root)
      [[ $# -ge 2 ]] || die "--worktree-root requires a value"
      worktree_root="$2"
      shift 2
      ;;
    --session)
      [[ $# -ge 2 ]] || die "--session requires a value"
      session_name="$2"
      start_server=1
      shift 2
      ;;
    --start-server)
      start_server=1
      shift
      ;;
    --skip-build)
      skip_build=1
      shift
      ;;
    --skip-download)
      skip_download=1
      shift
      ;;
    --check)
      check_only=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      break
      ;;
    -*)
      die "unknown option: $1"
      ;;
    *)
      if [[ -z "${task_slug}" ]]; then
        task_slug="$1"
      else
        die "unexpected extra argument: $1"
      fi
      shift
      ;;
  esac
done

if [[ ${check_only} -eq 0 && -z "${task_slug}" ]]; then
  die "task slug is required unless --check is used"
fi

require_command git "Install Git and rerun this script."
require_command go "Install Go and rerun this script."

if [[ ${start_server} -eq 1 ]]; then
  require_command tmux "Install tmux if you want the script to launch harnessd in the background."
  require_command lsof "Install lsof so scripts/start.sh can clear the configured port."
fi

if [[ ${check_only} -eq 1 ]]; then
  info "prerequisites satisfied"
  info "git: $(command -v git)"
  info "go: $(command -v go)"
  if [[ ${start_server} -eq 1 ]]; then
    info "tmux: $(command -v tmux)"
    info "lsof: $(command -v lsof)"
  fi
  exit 0
fi

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  die "run this from inside a git repository checkout"
fi

if [[ -z "${branch}" ]]; then
  branch="${DEFAULT_BRANCH_PREFIX}/${task_slug}"
fi

if [[ ${branch_explicit} -eq 0 && "${branch}" != codex/* && "${branch}" != */* ]]; then
  branch="codex/${branch}"
fi

worktree_path="${worktree_root}/${task_slug}/go-agent-harness"
build_dir="${worktree_path}/.tmp/bootstrap/bin"
env_file="${worktree_path}/.tmp/bootstrap/dev.env"
resolved_base_ref=""
created_from_resolved_base=0

info "repo root: ${REPO_ROOT}"
info "worktree root: ${worktree_root}"
info "target worktree: ${worktree_path}"
info "target branch: ${branch}"

mkdir -p "${worktree_root}/${task_slug}"

resolve_base_ref
info "resolved bootstrap source: ${base_ref} -> ${resolved_base_ref}"

if git worktree list --porcelain | awk '/^worktree / { print substr($0, 10) }' | grep -Fxq "${worktree_path}"; then
  info "reusing existing worktree"
else
  if [[ -e "${worktree_path}" ]]; then
    die "path exists but is not a registered git worktree: ${worktree_path}. Remove it or choose a different --task-slug."
  fi

  if git show-ref --verify --quiet "refs/heads/${branch}"; then
    info "creating worktree from existing local branch"
    if ! git worktree add "${worktree_path}" "${branch}"; then
      die "failed to create worktree on branch ${branch}. That branch may already be checked out in another worktree."
    fi
  else
    info "creating worktree from resolved base ref ${resolved_base_ref}"
    if ! git worktree add -b "${branch}" "${worktree_path}" "${resolved_base_ref}"; then
      die "failed to create worktree from base ref ${resolved_base_ref}. Ensure the ref exists locally or pass a valid --base-ref."
    fi
    created_from_resolved_base=1
  fi
fi

cd "${worktree_path}"

bootstrap_worktree="$(git rev-parse --show-toplevel)"
bootstrap_revision="$(git rev-parse HEAD)"
if [[ -z "${bootstrap_worktree}" || -z "${bootstrap_revision}" ]]; then
  die "could not resolve clean Git metadata for bootstrap worktree"
fi
if [[ "${bootstrap_worktree}" != "${worktree_path}" ]]; then
  die "bootstrap Git worktree mismatch: expected ${worktree_path}, got ${bootstrap_worktree}"
fi
if [[ ${created_from_resolved_base} -eq 1 && "${bootstrap_revision}" != "${resolved_base_ref}" ]]; then
  die "bootstrap source provenance rejected: newly created worktree HEAD ${bootstrap_revision} does not match resolved source ${resolved_base_ref}"
fi
info "bootstrap provenance: source=${resolved_base_ref} worktree-head=${bootstrap_revision}"
if [[ -n "$(git status --porcelain)" ]]; then
  die "bootstrap worktree is dirty; refusing to build an unverifiable runtime"
fi

if [[ ${skip_download} -eq 0 ]]; then
  info "downloading Go module dependencies"
  if ! go mod download; then
    die "go mod download failed. Check network access, Go proxy settings, and module availability."
  fi
fi

mkdir -p "${build_dir}" "$(dirname "${env_file}")" "${worktree_path}/.tmp/rollouts"

# Keep the target worktree as the authority for the selected revision and
# cleanliness check above. The staging clone exists only because Go's buildvcs
# discovery requires a directory-form .git; it is checked out at the exact
# target revision, verified clean, and removed on every exit path.
bootstrap_staging_root="$(mktemp -d "${worktree_path}/.tmp/bootstrap/buildvcs.XXXXXX")" || \
  die "could not create isolated bootstrap VCS staging directory"
bootstrap_build_worktree="${bootstrap_staging_root}/source"
if ! git clone --no-local --no-checkout "${bootstrap_worktree}" "${bootstrap_build_worktree}" >/dev/null; then
  die "could not create isolated bootstrap VCS staging clone"
fi
if ! git -C "${bootstrap_build_worktree}" checkout --detach --force "${bootstrap_revision}" >/dev/null; then
  die "could not check out bootstrap revision ${bootstrap_revision} in isolated VCS staging clone"
fi
staging_revision="$(git -C "${bootstrap_build_worktree}" rev-parse HEAD)"
if [[ ! -d "${bootstrap_build_worktree}/.git" || "${staging_revision}" != "${bootstrap_revision}" || -n "$(git -C "${bootstrap_build_worktree}" status --porcelain)" ]]; then
  die "isolated bootstrap VCS staging clone is not a clean checkout of ${bootstrap_revision}"
fi

cat > "${env_file}" <<EOF
# Generated by scripts/init.sh
export HARNESS_WORKSPACE="${worktree_path}"
export HARNESS_ROLLOUT_DIR="${worktree_path}/.tmp/rollouts"
export HARNESS_SUBAGENT_WORKTREE_ROOT="${worktree_root}"
export HARNESS_PROMPTS_DIR="${worktree_path}/prompts"
export HARNESS_MODEL_CATALOG_PATH="${worktree_path}/catalog/models.json"
export HARNESS_BINARY="${build_dir}/harnessd"
export HARNESS_CLI_BINARY="${build_dir}/harnesscli"
export HARNESS_BOOTSTRAP_SOURCE_REVISION="${resolved_base_ref}"
export HARNESS_BOOTSTRAP_WORKTREE_REVISION="${bootstrap_revision}"
export PATH="${build_dir}:\${PATH}"
EOF

if [[ ${skip_build} -eq 0 ]]; then
  info "building local binaries into ${build_dir}"
  if ! bootstrap_build_binary "${build_dir}/harnessd" ./cmd/harnessd; then
    die "failed to build verified harnessd. Fix the provenance error above, then rerun scripts/init.sh."
  fi
  if ! bootstrap_build_binary "${build_dir}/harnesscli" ./cmd/harnesscli; then
    die "failed to build verified harnesscli. Fix the provenance error above, then rerun scripts/init.sh."
  fi
  if ! bootstrap_build_binary "${build_dir}/coveragegate" ./cmd/coveragegate; then
    die "failed to build verified coveragegate. Fix the provenance error above, then rerun scripts/init.sh."
  fi
else
  warn "skipping local builds because --skip-build was provided"
fi

info "bootstrap complete"
info "env file: ${env_file}"
info "binary directory: ${build_dir}"

cat <<EOF

Next steps:
  source "${env_file}"
  cd "${worktree_path}"
  ./scripts/test-regression.sh

EOF

if [[ ${start_server} -eq 1 ]]; then
  if [[ -z "${session_name}" ]]; then
    session_name="harness-${task_slug}"
  fi

  if tmux has-session -t "${session_name}" 2>/dev/null; then
    die "tmux session already exists: ${session_name}. Use --session with a different name or attach to the existing session."
  fi

  info "starting harnessd in tmux session: ${session_name}"
  if ! tmux new-session -d -s "${session_name}" "cd '${worktree_path}' && ./scripts/start.sh"; then
    die "failed to start tmux session ${session_name}. Verify tmux is installed and try again."
  fi

  cat <<EOF
tmux session started: ${session_name}
attach with: tmux attach-session -t ${session_name}
EOF
fi
