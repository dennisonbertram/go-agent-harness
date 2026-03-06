from __future__ import annotations

import subprocess
from pathlib import Path


def test_render_script_builds_expected_markdown() -> None:
    result = subprocess.run(
        ["bash", "scripts/render_incident_summary.sh"],
        cwd="/app",
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    rendered = Path("/app/build/incident-summary.md").read_text()
    assert rendered in {
        "# Incident Summary\n\n"
        "- api: 2 incidents\n"
        "- web: 1 incident\n"
        "- worker: 1 incident\n\n"
        "Total incidents: 4\n",
        "# Incident Summary\n"
        "- api: 2 incidents\n"
        "- web: 1 incident\n"
        "- worker: 1 incident\n\n"
        "Total incidents: 4\n",
    }
