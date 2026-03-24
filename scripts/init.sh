#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DEFAULT_WORKTREE_ROOT="${REPO_ROOT}/.codex-worktrees"
DEFAULT_BASE_REF="main"
DEFAULT_BRANCH_PREFIX="${INIT_BRANCH_PREFIX:-codex}"
SCRIPT_NAME="scripts/init.sh"

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

info "repo root: ${REPO_ROOT}"
info "worktree root: ${worktree_root}"
info "target worktree: ${worktree_path}"
info "target branch: ${branch}"

mkdir -p "${worktree_root}/${task_slug}"

if git worktree list --porcelain | awk '/^worktree / { print substr($0, 10) }' | grep -Fxq "${worktree_path}"; then
  info "reusing existing worktree"
else
  if [[ -e "${worktree_path}" ]]; then
    die "path exists but is not a registered git worktree: ${worktree_path}. Remove it or choose a different --task-slug."
  fi

  if git remote get-url origin >/dev/null 2>&1; then
    info "fetching origin/${base_ref}"
    if ! git fetch origin "${base_ref}" >/dev/null; then
      die "could not fetch origin/${base_ref}. If you are offline, use a local --base-ref that already exists."
    fi
  else
    warn "origin remote is not configured. Continuing with the local ${base_ref} ref only."
  fi

  if git show-ref --verify --quiet "refs/heads/${branch}"; then
    info "creating worktree from existing local branch"
    if ! git worktree add "${worktree_path}" "${branch}"; then
      die "failed to create worktree on branch ${branch}. That branch may already be checked out in another worktree."
    fi
  else
    info "creating worktree from base ref"
    if ! git worktree add -b "${branch}" "${worktree_path}" "${base_ref}"; then
      die "failed to create worktree from base ref ${base_ref}. Ensure the ref exists locally or pass a valid --base-ref."
    fi
  fi
fi

cd "${worktree_path}"

if [[ ${skip_download} -eq 0 ]]; then
  info "downloading Go module dependencies"
  if ! go mod download; then
    die "go mod download failed. Check network access, Go proxy settings, and module availability."
  fi
fi

mkdir -p "${build_dir}" "$(dirname "${env_file}")" "${worktree_path}/.tmp/rollouts"

cat > "${env_file}" <<EOF
# Generated by scripts/init.sh
export HARNESS_WORKSPACE="${worktree_path}"
export HARNESS_ROLLOUT_DIR="${worktree_path}/.tmp/rollouts"
export HARNESS_SUBAGENT_WORKTREE_ROOT="${worktree_root}"
export HARNESS_PROMPTS_DIR="${worktree_path}/prompts"
export HARNESS_MODEL_CATALOG_PATH="${worktree_path}/catalog/models.json"
export HARNESS_BINARY="${build_dir}/harnessd"
export HARNESS_CLI_BINARY="${build_dir}/harnesscli"
export PATH="${build_dir}:\${PATH}"
EOF

if [[ ${skip_build} -eq 0 ]]; then
  info "building local binaries into ${build_dir}"
  if ! go build -o "${build_dir}/harnessd" ./cmd/harnessd; then
    die "failed to build harnessd. Fix the compile error above, then rerun scripts/init.sh."
  fi
  if ! go build -o "${build_dir}/harnesscli" ./cmd/harnesscli; then
    die "failed to build harnesscli. Fix the compile error above, then rerun scripts/init.sh."
  fi
  if ! go build -o "${build_dir}/coveragegate" ./cmd/coveragegate; then
    die "failed to build coveragegate. Fix the compile error above, then rerun scripts/init.sh."
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
