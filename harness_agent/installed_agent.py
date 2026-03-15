"""
HarnessInstalledAgent — Harbor BaseAgent adapter that runs harnessd inside
the Harbor container.

Tests our full harness: all 40+ tools, our system prompt, our agent loop.
The model is just the backend — harnessd orchestrates everything.
"""

from __future__ import annotations

import asyncio
import os
import shlex
from pathlib import Path

from harbor.agents.base import BaseAgent
from harbor.environments.base import BaseEnvironment
from harbor.models.agent.context import AgentContext


class HarnessInstalledAgent(BaseAgent):
    """
    Runs harnessd inside the Harbor container.
    Tests our full harness: all 40+ tools, our system prompt, our agent loop.
    The model is just the backend — harnessd orchestrates everything.
    """

    SUPPORTS_ATIF = False
    BINARY_DIR = Path(__file__).parent / "bin"

    @staticmethod
    def name() -> str:
        return "harness-installed"

    def version(self) -> str | None:
        return "0.1.0"

    async def setup(self, environment: BaseEnvironment) -> None:
        harnessd = self.BINARY_DIR / "harnessd-linux-amd64"
        harnesscli = self.BINARY_DIR / "harnesscli-linux-amd64"
        if not harnessd.exists() or not harnesscli.exists():
            raise FileNotFoundError(
                "Run ./harness_agent/build_binaries.sh first to cross-compile binaries."
            )
        self.logger.info("Uploading harnessd and harnesscli to container...")
        await environment.exec("mkdir -p /harness-agent/rollouts")
        await environment.upload_file(harnessd, "/harness-agent/harnessd")
        await environment.upload_file(harnesscli, "/harness-agent/harnesscli")
        await environment.exec("chmod +x /harness-agent/harnessd /harness-agent/harnesscli")
        self.logger.info("Harness binaries installed in container.")

    async def run(
        self,
        instruction: str,
        environment: BaseEnvironment,
        context: AgentContext,
    ) -> None:
        # Parse provider/model from harbor's "provider/model" format
        raw = self.model_name or "openai/gpt-4.1-mini"
        model = raw.split("/", 1)[-1] if "/" in raw else raw

        api_key = os.environ.get("OPENAI_API_KEY", "")
        anthropic_key = os.environ.get("ANTHROPIC_API_KEY", "")

        env = {
            "OPENAI_API_KEY": api_key,
            "ANTHROPIC_API_KEY": anthropic_key,
            "HARNESS_MODEL": model,
            "HARNESS_MAX_STEPS": "150",
            "HARNESS_ROLLOUT_DIR": "/harness-agent/rollouts",
            "HARNESS_ADDR": ":8080",
        }

        # Single shell script: start harnessd, wait for ready, run task, collect result
        script = f"""
set -eo pipefail
cd /harness-agent

# Start harnessd in background
./harnessd >> /harness-agent/harnessd.log 2>&1 &
HARNESS_PID=$!
trap 'kill $HARNESS_PID 2>/dev/null || true' EXIT

# Wait up to 60s for harnessd to be ready
echo "[harness] waiting for harnessd..."
for i in $(seq 1 30); do
    if (echo > /dev/tcp/127.0.0.1/8080) 2>/dev/null; then
        echo "[harness] harnessd ready after $((i*2))s"
        break
    fi
    sleep 2
    if [ $i -eq 30 ]; then
        echo "[harness] ERROR: harnessd did not start"
        cat /harness-agent/harnessd.log
        exit 1
    fi
done

# Run the task — harnesscli blocks until done, outputs SSE stream
echo "[harness] submitting task..."
./harnesscli \\
    -base-url=http://localhost:8080 \\
    -model="{model}" \\
    -prompt={shlex.quote(instruction)} \\
    2>&1

echo "[harness] task complete"
"""

        self.logger.info("HarnessInstalledAgent running: model=%s", model)
        result = await environment.exec(command=script, env=env, timeout_sec=1800)

        stdout = result.stdout or ""
        self.logger.info(
            "Exit code: %d, output length: %d chars",
            result.return_code,
            len(stdout),
        )

        # Parse terminal_event from harnesscli output
        for line in stdout.splitlines():
            if line.startswith("terminal_event="):
                self.logger.info("Result: %s", line)
            elif line.startswith("run_id="):
                self.logger.info("Run ID: %s", line)
