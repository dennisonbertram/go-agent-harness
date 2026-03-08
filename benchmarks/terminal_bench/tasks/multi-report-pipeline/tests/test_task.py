from __future__ import annotations

import json
import subprocess
from pathlib import Path


def _run_script() -> None:
    """Helper to run the generate script."""
    result = subprocess.run(
        ["bash", "scripts/generate_reports.sh"],
        cwd="/app",
        capture_output=True,
        text=True,
        timeout=30,
    )
    assert result.returncode == 0, (
        f"script failed:\nstdout: {result.stdout}\nstderr: {result.stderr}"
    )


# --- summary.md tests ---

def test_summary_file_exists() -> None:
    _run_script()
    assert Path("/app/build/summary.md").exists(), "build/summary.md not found"


def test_summary_is_markdown_table() -> None:
    _run_script()
    content = Path("/app/build/summary.md").read_text()
    has_table_header = "| Server" in content or "|Server" in content
    assert has_table_header, "summary.md must contain a markdown table header with 'Server' column"
    assert "---" in content, "summary.md must contain table separator"


def test_summary_has_all_servers() -> None:
    _run_script()
    content = Path("/app/build/summary.md").read_text()
    for server in ["api-01", "db-01", "web-01", "web-02"]:
        assert server in content, f"summary.md missing server {server}"


def test_summary_servers_sorted() -> None:
    _run_script()
    content = Path("/app/build/summary.md").read_text()
    lines = content.strip().split("\n")
    # Find data rows (skip header and separator)
    data_rows = [l for l in lines if l.startswith("|") and "---" not in l and "Server" not in l]
    servers = [r.split("|")[1].strip() for r in data_rows]
    assert servers == sorted(servers), f"servers not sorted: {servers}"


def test_summary_avg_cpu_values() -> None:
    """Check computed average CPU values."""
    _run_script()
    content = Path("/app/build/summary.md").read_text()
    # api-01: (92.1 + 88.4 + 95.3) / 3 = 91.9
    assert "91.9" in content, "api-01 avg CPU should be 91.9"
    # db-01: (35.0 + 33.1) / 2 = 34.1 (or 34.0 depending on rounding)
    assert "34.1" in content or "34.0" in content, "db-01 avg CPU should be ~34.1"
    # web-01: (45.2 + 52.3 + 48.9) / 3 = 48.8
    assert "48.8" in content, "web-01 avg CPU should be 48.8"


def test_summary_avg_memory_values() -> None:
    """Check computed average memory values."""
    _run_script()
    content = Path("/app/build/summary.md").read_text()
    # api-01: (7500 + 7800 + 8100) / 3 = 7800.0
    assert "7800.0" in content or "7800" in content, "api-01 avg memory should be 7800.0"
    # web-02: (6800 + 7200 + 6900) / 3 = 6966.7
    assert "6966.7" in content, "web-02 avg memory should be 6966.7"


# --- alerts.md tests ---

def test_alerts_file_exists() -> None:
    _run_script()
    assert Path("/app/build/alerts.md").exists(), "build/alerts.md not found"


def test_alerts_contains_violations() -> None:
    """Should flag rows where CPU > 80 OR memory > 7000."""
    _run_script()
    content = Path("/app/build/alerts.md").read_text()
    # api-01 appears in all 3 timestamps (CPU > 80 and mem > 7000)
    assert content.count("api-01") == 3, "api-01 should appear 3 times in alerts"
    # web-02 at 10:05 (CPU=85.7, mem=7200) and 10:10 (CPU=81.2, mem=6900)
    assert content.count("web-02") >= 2, "web-02 should appear at least 2 times"


def test_alerts_sorted_by_timestamp() -> None:
    _run_script()
    content = Path("/app/build/alerts.md").read_text()
    lines = [l for l in content.strip().split("\n") if l.startswith("- ")]
    timestamps = []
    for line in lines:
        # Extract timestamp from "- <server> at <timestamp>: ..."
        if " at " in line:
            ts = line.split(" at ")[1].split(":")[0:3]
            timestamps.append(":".join(ts))
    assert timestamps == sorted(timestamps), f"alerts not sorted by timestamp: {timestamps}"


def test_alerts_format() -> None:
    _run_script()
    content = Path("/app/build/alerts.md").read_text()
    lines = [l for l in content.strip().split("\n") if l.startswith("- ")]
    for line in lines:
        assert "CPU=" in line, f"alert line missing CPU= : {line}"
        assert "Mem=" in line, f"alert line missing Mem= : {line}"


# --- status-counts.json tests ---

def test_status_counts_file_exists() -> None:
    _run_script()
    assert Path("/app/build/status-counts.json").exists(), "build/status-counts.json not found"


def test_status_counts_values() -> None:
    _run_script()
    data = json.loads(Path("/app/build/status-counts.json").read_text())
    assert data.get("healthy") == 6, f"expected 6 healthy, got {data.get('healthy')}"
    assert data.get("degraded") == 2, f"expected 2 degraded, got {data.get('degraded')}"
    assert data.get("critical") == 3, f"expected 3 critical, got {data.get('critical')}"
