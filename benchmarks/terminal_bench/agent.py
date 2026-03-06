from __future__ import annotations

import os
import platform
import shlex
import subprocess
import tarfile
import tempfile
from pathlib import Path

from terminal_bench.agents.base_agent import AgentResult, BaseAgent
from terminal_bench.agents.failure_mode import FailureMode
from terminal_bench.terminal.tmux_session import TmuxSession

REPO_ROOT = Path(__file__).resolve().parents[2]
CONTAINER_REPO_ROOT = "/opt/go-agent-harness"
CONTAINER_BIN_DIR = "/tmp/go-agent-harness-bin"
HARNESS_BASE_URL = "http://127.0.0.1:8080"


class GoAgentHarnessAgent(BaseAgent):
    def __init__(self, **kwargs):
        super().__init__(**kwargs)
        self._model = kwargs.get("model", os.getenv("HARNESS_BENCH_MODEL", "gpt-5-mini"))
        self._api_key = kwargs.get("openai_api_key", os.getenv("OPENAI_API_KEY", ""))
        self._base_url = kwargs.get("openai_base_url", os.getenv("OPENAI_BASE_URL", ""))
        self._max_steps = kwargs.get("harness_max_steps", os.getenv("HARNESS_BENCH_MAX_STEPS", "20"))
        self._memory_mode = kwargs.get("harness_memory_mode", os.getenv("HARNESS_BENCH_MEMORY_MODE", "off"))
        self._target_arch = kwargs.get("target_arch", os.getenv("HARNESS_BENCH_TARGET_ARCH", self._default_target_arch()))

    @staticmethod
    def name() -> str:
        return "go-agent-harness"

    def perform_task(
        self,
        instruction: str,
        session: TmuxSession,
        logging_dir: Path | None = None,
    ) -> AgentResult:
        if not self._api_key:
            return AgentResult(failure_mode=FailureMode.AGENT_INSTALLATION_FAILED)

        archive_path = self._package_repo()
        binary_dir = self._build_binaries()
        session.copy_to_container(paths=[archive_path], container_dir="/tmp")
        session.copy_to_container(
            paths=[binary_dir / "harnessd", binary_dir / "harnesscli"],
            container_dir="/tmp",
        )

        try:
            install_script = self._build_install_script(archive_path.name)
            session.send_keys([f"bash -lc {shlex.quote(install_script)}", "Enter"], block=True, max_timeout_sec=1200)

            run_script = self._build_run_script(self._render_instruction(instruction))
            session.clear_history()
            session.send_keys([f"bash -lc {shlex.quote(run_script)}", "Enter"], block=True, max_timeout_sec=1800)

            terminal_output = session.capture_pane(capture_entire=True)
            if "terminal_event=run.completed" in terminal_output:
                return AgentResult(failure_mode=FailureMode.NONE)
            return AgentResult(failure_mode=FailureMode.UNKNOWN_AGENT_ERROR)
        finally:
            archive_path.unlink(missing_ok=True)
            for binary_path in binary_dir.glob("*"):
                binary_path.unlink(missing_ok=True)
            binary_dir.rmdir()

    def _build_install_script(self, archive_name: str) -> str:
        server_command = self._shell_join(
            {
                "OPENAI_API_KEY": self._api_key,
                "OPENAI_BASE_URL": self._base_url,
                "HARNESS_ADDR": ":8080",
                "HARNESS_MODEL": self._model,
                "HARNESS_MAX_STEPS": str(self._max_steps),
                "HARNESS_MEMORY_MODE": self._memory_mode,
                "HARNESS_PROMPTS_DIR": f"{CONTAINER_REPO_ROOT}/prompts",
            },
            f"{CONTAINER_BIN_DIR}/harnessd >/tmp/harnessd.log 2>&1",
        )
        tmux_command = f'cd "$TASK_ROOT" && HARNESS_WORKSPACE="$TASK_ROOT" {server_command}'
        return f"""
set -euo pipefail
TASK_ROOT="$(pwd)"
mkdir -p {CONTAINER_BIN_DIR}
rm -rf {CONTAINER_REPO_ROOT}
mkdir -p {CONTAINER_REPO_ROOT}
tar -xf /tmp/{archive_name} -C {CONTAINER_REPO_ROOT} --strip-components=1
mv /tmp/harnessd {CONTAINER_BIN_DIR}/harnessd
mv /tmp/harnesscli {CONTAINER_BIN_DIR}/harnesscli
chmod +x {CONTAINER_BIN_DIR}/harnessd {CONTAINER_BIN_DIR}/harnesscli
cd {CONTAINER_REPO_ROOT}
tmux kill-session -t harnessd >/dev/null 2>&1 || true
tmux new-session -d -s harnessd "{tmux_command}"
for attempt in $(seq 1 90); do
  if curl -fsS {HARNESS_BASE_URL}/healthz >/dev/null 2>&1; then
    exit 0
  fi
  sleep 1
done
echo "harness server did not become healthy" >&2
tail -n 200 /tmp/harnessd.log >&2 || true
exit 1
"""

    def _build_run_script(self, instruction: str) -> str:
        cli_command = self._shell_join(
            {},
            (
                f'{CONTAINER_BIN_DIR}/harnesscli '
                f'-base-url={HARNESS_BASE_URL} '
                f'-model={shlex.quote(self._model)} '
                f'-agent-intent=general '
                f'-task-context={shlex.quote("Terminal Bench private smoke suite")} '
                f'-prompt={shlex.quote(instruction)}'
            ),
        )
        return f"""
set -euo pipefail
TASK_ROOT="$(pwd)"
cd "$TASK_ROOT"
{cli_command}
"""

    def _shell_join(self, env_map: dict[str, str], command: str) -> str:
        env_parts = []
        for key, value in env_map.items():
            if value == "":
                continue
            env_parts.append(f"{key}={shlex.quote(value)}")
        if env_parts:
            return " ".join(env_parts) + " " + command
        return command

    def _package_repo(self) -> Path:
        fd, temp_path = tempfile.mkstemp(prefix="go-agent-harness-", suffix=".tar")
        os.close(fd)
        archive_path = Path(temp_path)
        with tarfile.open(archive_path, "w") as archive:
            archive.add(REPO_ROOT, arcname="go-agent-harness", filter=self._tar_filter)
        return archive_path

    def _tar_filter(self, tarinfo: tarfile.TarInfo) -> tarfile.TarInfo | None:
        path = Path(tarinfo.name)
        parts = path.parts[1:]
        if parts:
            root_entry = parts[0]
            if root_entry in {".git", ".tmp", "node_modules"}:
                return None
        return tarinfo

    def _build_binaries(self) -> Path:
        temp_dir = Path(tempfile.mkdtemp(prefix="go-agent-harness-bin-"))
        build_env = os.environ.copy()
        build_env.update(
            {
                "GOOS": "linux",
                "GOARCH": self._target_arch,
                "CGO_ENABLED": "0",
            }
        )
        subprocess.run(
            ["go", "build", "-o", str(temp_dir / "harnessd"), "./cmd/harnessd"],
            cwd=REPO_ROOT,
            env=build_env,
            check=True,
        )
        subprocess.run(
            ["go", "build", "-o", str(temp_dir / "harnesscli"), "./cmd/harnesscli"],
            cwd=REPO_ROOT,
            env=build_env,
            check=True,
        )
        return temp_dir

    def _default_target_arch(self) -> str:
        machine = platform.machine().lower()
        if machine in {"arm64", "aarch64"}:
            return "arm64"
        return "amd64"
